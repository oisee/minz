# LIR Backend Reference

> Quick reference for the MinZ LIR (Low-level IR) backend — architecture, components, DSLs, and data structures.

---

## Pipeline Overview

```
MIR2 Function
  |
  v
Bridge (bridge.go)          MIR2 → MIROps, call lowering, save-before-overwrite
  |
  v
ISLE Combine (combine.go)   Pattern rewriting: load16_le fusion, MUL→SHL/ADD
  |
  v
ISel (isel.go)              Pattern matching: MIROps → LIR Insts with LocSet constraints
  |
  v
CFG Rules (cfgrules.go)     Block-level transforms: loop rotation, DJNZ, empty block elim
  |
  v
WFC (wfc.go)                Constraint propagation + collapse: LocSet → physical registers
  |                         With PBQP hints (pickPreferred)
  v
Peephole + Emit             LD r,r elimination, tail call CALL+RET→JP, template expansion
  |
  v
Z80 Assembly
```

---

## Components

### Bridge (`bridge.go`)

Converts MIR2 instructions to LIR MIROps. Special handling:

| MIR2 Op | LIR Translation |
|---------|----------------|
| OpConst, OpAdd, OpSub, ... | Direct 1:1 MIROp |
| OpCall | Arg setup moves + OpCall (with DstAllowed per callee contract) |
| OpMul (non-const) | Setup moves + CALL `__mul8`/`__mul16` runtime |
| OpExt, OpSext, OpTrunc | OpMove (type conversion = register move on Z80) |

**Save-before-overwrite** (Z80 only): inserts copy moves before destructive ALU ops that would overwrite live values in the accumulator.

### ISLE Combine (`combine.go`)

S-expression rewrite rules, parsed by `isle/isle.go`:

```lisp
;; FatFS ld_word: two 8-bit loads → one 16-bit LE load
(rule 20 (or (shl (load8 (add ?base (const 1))) (const 8))
             (load8 ?base))
         (load16_le ?base))

;; Multiply strength reduction
(rule 15 (mul ?x (const 2))  (add ?x ?x))
(rule 15 (mul ?x (const 4))  (shl ?x (const 2)))

;; Identity elimination
(rule (add ?x (const 0))  ?x)
(rule (and ?x (const 0))  (const 0))
```

### ISel (`isel.go`)

Greedy pattern matching against the machine descriptor's pattern table:

```
MIROp{Op:OpAdd, Width:8, Dst:3, Src:[1,2]}
  → Pattern "add_a_r" {DstLocs:A, SrcLocs:[A, gpr8], Template:"ADD A, {src1}"}
  → Inst{Pat:add_a_r, Dst:{VReg:3, Allowed:A}, Srcs:[{VReg:1, Allowed:A}, {VReg:2, Allowed:gpr8}]}
```

Inserts setup moves when operand constraints require values in specific registers.

### CFG Rules (`cfgrules.go`)

Declarative block-level transformations:

| Rule | Priority | What |
|------|----------|------|
| `loop_rotate_djnz` | 35 | Absorb `sub(cnt,1)` into DJNZ terminator |
| `loop_rotate` | 30 | Header-tested → bottom-tested loop |
| `empty_block_elim` | 20 | Remove blocks with only a jump |
| `block_merge` | 15 | Merge sequential blocks |
| `branch_to_next` | 10 | Remove jump to layout successor |
| `branch_inversion` | 5 | Swap branch targets |

### WFC (`wfc.go`, `wfc_interblock.go`)

Wave Function Collapse register allocator. Each operand is a superposition of possible physical locations (LocSet bitfield). Constraints propagate until collapse.

**Propagation passes:**
1. **Forward:** def LocSet → use LocSet
2. **Backward:** use requirement → def constraint
3. **VReg consistency:** same vreg = same physical location everywhere
4. **Clobber:** CALL clobbers → narrow to callSafeLocs (IXH/IXL/spill)

**Collapse:** sequential, `pickPreferred(hints)` uses PBQP allocation as bias.

**Inter-block (Dimension 2):** `ProgWFC` propagates constraints across CFG edges via block params.

### Constraint DSL (`rules.go`)

Three-layer framework (partially implemented):

| Layer | Style | Purpose | Status |
|-------|-------|---------|--------|
| Facts | Datalog | Register metadata, aliases, forbidden combos | Implemented |
| Patterns | Cypher-like | Graph matching on IR | Declared |
| Rewrites | Actions | Transform matched patterns | Declared |

**Z80 Facts example:**
```
reg(A, 8, acc)          alias(HL, H)         alias(HL, L)
reg(HL, 16, ptr)        forbidden(IXH, ix_indirect)
acc_only(ADD8, A)       clobber(ADD8, F)
```

---

## Machine Descriptors

**Convergence spectrum** (same semantics, different constraints):

```
RISC32 ──→ RISC8 ──→ CISC ──→ Z80
(32 regs)   (8 regs)  (acc+pairs) (DD/FD prefix, IXH/IXL)
```

### Z80 Register File (22 locations)

| Index | Name | Width | Kind | Notes |
|-------|------|-------|------|-------|
| 0 | A | 8 | Acc | ALU destination |
| 1-6 | B,C,D,E,H,L | 8 | Reg | General purpose |
| 7-8 | BC, DE | 16 | Pair | |
| 9 | HL | 16 | Index | 16-bit ALU destination |
| 10 | SP | 16 | Index | Stack pointer |
| 11-12 | IX, IY | 16 | Index | DD/FD prefixed |
| 13 | F | 1 | Flag | CPU flags |
| 14-17 | IXH,IXL,IYH,IYL | 8 | Index | **Call-safe** (undocumented) |
| 18-21 | spill0-3 | 16 | Mem | Memory-backed |

### Z80 Patterns (41+)

Constants, moves, 8-bit ALU (acc-only), 16-bit ALU (HL-only), shifts, loads, stores, IX-indexed loads, spill moves, IX half moves, function calls, DJNZ.

### Resource Hierarchy (L0-L4)

| Layer | Storage | Cost | Survives CALL |
|-------|---------|------|---------------|
| L0 | Same register | 0T | No |
| L1 | GPR move (LD r,r') | 4T | No |
| L2 | IX/IY half (LD IXH,r) | 8T | **Yes** |
| L3 | Shadow (EXX) | 4T/batch | **Yes** (future) |
| L4 | Memory (LD (nn),A) | 13T | **Yes** |

---

## PBQP→WFC Hint Bridge

```
PBQP: global interference graph → vreg→PhysLoc{Name:"C"}
                                        |
                          pbqpToLIRHints: Name→LocByName→index
                                        |
                                        v
WFC:  Collapse → pickPreferred(available, vreg)
                   if hints[vreg] in available → use it
                   else → pickFirst (fallback)
```

**Result:** LIR output matches production codegen for leaf functions.

---

## Runtime Routines

Emitted once per module when referenced:

```asm
; __mul8: A = A * B (8-bit, ~80T, 8 iterations)
__mul8:
    LD C, A / XOR A / LD D, 8
    SRL C / JR NC, .sk / ADD A, B / .sk: SLA B / DEC D / JR NZ

; __mul16: HL = HL * DE (16-bit, ~200T, 16 iterations)
__mul16:
    PUSH BC / LD B,H / LD C,L / LD HL,0 / LD A,16
    SRL D / RR E / JR NC, .sk / ADD HL,BC / .sk: SLA C / RL B / DEC A / JR NZ
    POP BC / RET
```

---

## Key Files

| Component | File |
|-----------|------|
| Core types + LocSet | `pkg/lir/lir.go` |
| Machine descriptors | `pkg/lir/machines.go`, `pkg/lir/z80.go` |
| MIR2→LIR bridge | `pkg/lir/bridge.go` |
| ISLE parser + engine | `pkg/lir/isle/isle.go` |
| ISLE combining rules | `pkg/lir/combine.go` |
| Instruction selection | `pkg/lir/isel.go` |
| WFC (Dim 1) | `pkg/lir/wfc.go` |
| WFC (Dim 2, inter-block) | `pkg/lir/wfc_interblock.go` |
| CFG block rules | `pkg/lir/cfgrules.go` |
| Loop rotation | `pkg/lir/looprot.go` |
| Constraint DSL (facts) | `pkg/lir/rules.go` |
| Pipeline integration | `pkg/lir/pipeline.go` |
