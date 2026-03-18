# LIR Backend Architecture: How Three Solvers Cooperate to Generate Z80 Code

> From MIR2 to Z80 assembly in six stages, using ISLE term rewriting, Wave Function Collapse, and PBQP hints — with Z80-specific innovations no existing compiler has.

---

## The Problem

Z80 is one of the most constrained architectures still actively targeted:
- 7 general-purpose 8-bit registers (A, B, C, D, E, H, L)
- Accumulator-only ALU: `ADD A, r` — destination is always A
- Register pair asymmetry: 16-bit ADD only works on HL
- DD/FD prefix conflicts: IX-indexed ops can't use H or L
- No hardware multiply, no barrel shifter, no second accumulator

Traditional approaches (linear scan, graph coloring) struggle because Z80's constraints aren't just "these two vregs interfere" — they're **structural** (specific ops require specific registers) and **hierarchical** (IX halves survive calls but cost more than GPRs).

## The Solution: Three Cooperating Solvers

```
                    ┌─────────────────────────────────────┐
                    │          MIR2 Function               │
                    └──────────────┬──────────────────────┘
                                   │
                    ┌──────────────▼──────────────────────┐
                    │  ISLE (What to compute)              │
                    │  S-expression rewrite rules          │
                    │  "fuse two loads into load16_le"     │
                    │  "mul(x, 4) → shl(x, 2)"            │
                    └──────────────┬──────────────────────┘
                                   │
                    ┌──────────────▼──────────────────────┐
                    │  ISel (How to compute)               │
                    │  Pattern table → LIR instructions    │
                    │  "OpAdd width=8 → add_a_r"           │
                    │  Insert setup moves for constraints  │
                    └──────────────┬──────────────────────┘
                                   │
          ┌────────────────────────▼────────────────────────────┐
          │                                                     │
   ┌──────▼──────┐                                    ┌────────▼────────┐
   │ PBQP         │──── hints (preferred locs) ──────>│ WFC              │
   │ Global       │                                   │ Local            │
   │ Strategist   │                                   │ Tactician        │
   │              │                                   │                  │
   │ Sees: full   │                                   │ Sees: Z80-       │
   │ interference │                                   │ specific rules   │
   │ graph        │                                   │ DD/FD prefix     │
   │              │                                   │ IXH/IXL call-safe│
   │ Outputs:     │                                   │ clobber pass     │
   │ vreg→phys    │                                   │                  │
   └──────────────┘                                   │ Outputs:         │
                                                      │ physical regs    │
                                                      └────────┬────────┘
                                                               │
                                              ┌────────────────▼────────────────┐
                                              │ Peephole + Emit                  │
                                              │ LD r,r elimination               │
                                              │ Tail call: CALL+RET → JP         │
                                              │ Template expansion → Z80 asm     │
                                              └─────────────────────────────────┘
```

### ISLE: What to Compute

ISLE (Instruction Selection Lowering Expressions) is a term-rewriting engine inspired by Cranelift. It operates on MIROps as S-expression trees and applies prioritized rewrite rules:

```lisp
;; FatFS ld_word: fuse two 8-bit loads + shift + or into one 16-bit LE load
(rule 20 (or (shl (load8 (add ?base (const 1))) (const 8))
             (load8 ?base))
         (load16_le ?base))

;; Strength reduction: multiply by power of 2 → shift
(rule 15 (mul ?x (const 2))   (add ?x ?x))
(rule 15 (mul ?x (const 4))   (shl ?x (const 2)))

;; Identity elimination
(rule (add ?x (const 0))  ?x)
```

ISLE reduces instruction count *before* register allocation runs. For FatFS `ld_word`, it fuses 8 MIROps into 2.

### PBQP: Global Strategy

PBQP (Partitioned Boolean Quadratic Programming) builds an interference graph of all virtual registers in a function. For each vreg, it knows which physical registers are *possible* and which other vregs conflict. It solves for globally optimal assignment.

**Limitation:** PBQP doesn't know about Z80-specific encoding constraints (DD/FD prefix conflicts, accumulator-only ALU, IXH/IXL availability).

### WFC: Local Tactics

Wave Function Collapse treats register allocation as constraint satisfaction. Each instruction operand starts as a **superposition** of possible physical locations (LocSet bitfield). Constraints propagate until each operand collapses to exactly one location.

```
Before propagation:
  ADD dst={A,B,C,D,E,H,L} src0={A,B,C,D,E,H,L} src1={A,B,C,D,E,H,L}

After pattern constraint (add_a_r: dst=A, src0=A):
  ADD dst={A}              src0={A}              src1={A,B,C,D,E,H,L}

After vreg consistency (src1 vreg was defined as param in C):
  ADD dst={A}              src0={A}              src1={C}

Collapsed: ADD A, C
```

**Five propagation passes:**

| Pass | Direction | What it does |
|------|-----------|-------------|
| Forward | → | Def LocSet narrows use LocSet |
| Backward | ← | Use requirement narrows def LocSet |
| VReg consistency | ↔ | Same vreg = same physical everywhere |
| Clobber | ↓ | CALL clobbers → narrow to IXH/IXL/spill |
| Collapse | → | Sequential assignment with `pickPreferred(hints)` |

### The Bridge: PBQP Guides WFC

PBQP's allocation result is converted to **hints** — a map from vreg to preferred physical location. During WFC collapse, `pickPreferred()` checks if PBQP's choice is available in the current LocSet:

```go
func (st *WFCState) pickPreferred(available LocSet, vreg int) LocSet {
    if hinted, ok := st.Hints[vreg]; ok && available.Has(hinted) {
        return Singleton(hinted)  // use PBQP's choice
    }
    return pickFirst(available)   // fallback
}
```

**Result:** leaf functions produce output **identical to production codegen**.

---

## Z80-Specific Innovations

### IXH/IXL as Call-Safe Spill (L2)

Z80's undocumented index register halves (`LD IXH, r` via DD prefix) are **not clobbered by function calls**. This gives WFC 4 bytes of fast storage that survives CALL — cheaper than PUSH/POP (8T vs 11T) and doesn't touch the stack.

```
Resource hierarchy:
  L0: Same register        0T   (vreg consistency)
  L1: GPR move (LD r, r')  4T   (setup moves)
  L2: IX half (LD IXH, r)  8T   (call-safe!)      ← NEW
  L3: Shadow (EXX)          4T/batch  (future)
  L4: Memory (LD (nn), A)  13T  (always safe)
```

WFC's `clobberPass` detects vregs live across calls and narrows their LocSet to `callSafeLocs() = {IXH, IXL, IYH, IYL, spill0-3}`.

**No existing Z80 compiler uses IX/IY halves for register allocation.**

### Save-Before-Overwrite

Z80's accumulator ALU (`ADD A, r` overwrites A) creates a unique challenge: when multiple values need to be in A at different times, earlier values must be saved.

```
MIR2:  x = a + b;  y = a & b;  return x + y

Naive codegen (WRONG):
  ADD A, C    ; A = a+b = x
  AND C       ; A = x&b ← wrong, should be a&b

With save-before-overwrite:
  LD B, A     ; save a to B
  LD D, C     ; save b to D
  ADD A, D    ; A = a+b = x
  LD C, A     ; save x to C
  LD A, B     ; restore a
  AND D       ; A = a&b = y
  LD B, A     ; save y to B
  LD A, C     ; restore x
  ADD A, B    ; A = x+y (CORRECT)
```

### Tail Call Optimization

When the last instruction before `RET` is a `CALL`, replace with `JP`:

```
Before: LD A, 65; CALL putchar; RET   (17T + 10T = 27T)
After:  LD A, 65; JP putchar          (10T, saves 17T)
```

### Runtime Multiply

Z80 has no hardware multiply. Non-constant multiplies are lowered to shared runtime routines:

```asm
__mul8:   A = A * B    (~80T, 8 iterations, shift-and-add)
__mul16:  HL = HL * DE (~200T, 16 iterations)
```

Emitted once per module, shared across all call sites. Tail-call optimized when possible.

---

## CFG Block Rules (Layer 4)

Declarative block-level transformations applied after isel, before WFC:

```
┌──────────────────┐     loop_rotate      ┌──────────────────┐
│ head:            │    ─────────────>     │ guard:           │
│   branch cond    │                      │   branch cond    │
│   @body, @exit   │                      │   @body, @exit   │
├──────────────────┤                      ├──────────────────┤
│ body:            │                      │ body:            │
│   ...work...     │                      │   ...work...     │
│   jump @head     │                      │   branch cond    │
└──────────────────┘                      │   @body, @exit   │
                                          └──────────────────┘
   While loop                              Do-while (DJNZ-ready)
```

Six rules in priority order: DJNZ absorption (35) → loop rotation (30) → empty block elimination (20) → block merge (15) → branch-to-next (10) → branch inversion (5).

---

## Convergence Testing

Four machine descriptors verify correctness across the constraint spectrum:

```
RISC32    RISC8     CISC      Z80
32 regs   8 regs    acc+pairs  DD/FD prefix
symmetric symmetric asymmetric constrained
   |         |         |         |
   +----+----+---------+---------+
        |
   Same observable result?
   YES → lowering is correct
   NO  → bug in constraint handling
```

If a function passes on RISC32 but fails on Z80, the bug is in Z80-specific constraints. If it fails on all, the bug is in MIR2→LIR translation.

---

## Corpus Results

**948/948 = 100%** across all four frontends:

| Frontend | Functions | Rate | Notes |
|----------|-----------|------|-------|
| C89 | 720 (240 × 3 machines) | 100% | 36 source files |
| Nanz | 162 (54 × 3) | 100% | includes arena allocator |
| Lizp | 57 (19 × 3) | 100% | S-expression frontend |
| Lanz | 9 (3 × 3) | 100% | Lanz→HIR frontend |

---

## Future: EXX Shadow Stream (L3)

Z80's `EXX` instruction swaps BC↔BC', DE↔DE', HL↔HL' in **4 T-states**. For 3+ values live across a call:

```
EXX; CALL foo; EXX = 8T total
vs
PUSH BC; PUSH DE; PUSH HL; CALL foo; POP HL; POP DE; POP BC = 63T
```

WFC would model this as a group constraint: EXX regions are SESE (single-entry, single-exit), and the cost is amortized across all values saved.

**This would be a novel contribution — no existing Z80 compiler uses shadow registers for register allocation.**
