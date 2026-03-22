# Report 107: Z3 SMT Solver, LIR Improvements & Frill Frontend

**Date:** 2026-03-22
**Branch:** `feat/z3-optimal-codegen`
**Commits:** 16
**LOC:** +3552 lines across 22 files

---

## Executive Summary

Single-session sprint: integrated Z3 SMT solver for provably optimal register allocation, fixed 6 codegen bugs, added equality saturation framework, and created a new ML-inspired functional language frontend (Frill).

**E2E corpus: 68/79 (86%) → 74/81 (91.4%)**

---

## 1. Z3 SMT-Based Optimal Register Allocation

### What
Z3 theorem prover integrated as subprocess for register allocation. Encodes the full regalloc problem as SMT-LIB2 optimization formula and finds provably optimal assignment.

### Architecture
```
LIR Prog (after isel) → encodeSMT() → z3 subprocess → parseModel() → apply
```

### Key Design Decisions
- **Subprocess, not library** — Z3 called via `/home/alice/miniconda3/bin/z3 -in -smt2`. No CGo, no FFI, simple text protocol.
- **PBQP hints as soft constraints** — Z3 gets `+10` cost penalty for deviating from PBQP's suggestions, ensuring bootstrap compatibility.
- **Tied operands** — `ADD A,r` where dst and src0 both require A: `(assert (= v1 v3))` instead of interference.
- **Z3ValidateAndRepair** — post-WFC consistency checker, invokes Z3 only if WFC produces inconsistent vreg assignments.

### Files
| File | LOC | Purpose |
|------|-----|---------|
| `z3solve.go` | 350 | SMT encoding, Z3 subprocess, model parsing |
| `z3_joint.go` | 190 | Joint isel+regalloc via e-graph + Z3 |
| `z3validate.go` | 80 | Post-WFC consistency check + Z3 repair |
| `z3solve_test.go` | 330 | 5 tests: add, interference, pressure, spill, dump |
| `z3_e2e_test.go` | 200 | Z3 vs WFC comparison tests |
| `saturate.go` | 165 | Equality saturation loop + Z80 rewrite rules |
| `saturate_test.go` | 115 | 3 tests: CSE, LD_A_0→XOR, joint Z3 |

### CLI
```bash
mz program.nanz -b z80 --z3    # Z3 optimal regalloc (100ms/function)
mz program.nanz -b z80          # WFC greedy regalloc (1ms/function, default)
```

---

## 2. Codegen Bug Fixes (6 bugs)

### BUG: addr_of symbol propagation (CRITICAL)
**Before:** `LD BC, 0 / LD HL, 0 / LD A, C / LD (HL), A` — store to ROM at address 0!
**After:** `LD IY, p_name / LD A, L / LD (IY+0), A` — correct global store.
**Root cause:** Bridge dropped `inst.Sym` from `OpAddrOf`, emitting `Imm=0` instead of symbol name.

### BUG: fixInvalidZ80Template corrupting ALU ops (CRITICAL)
**Before:** `double16(x) → LD D,H / LD E,L / LD (HL),E / INC HL / LD (HL),D / DEC HL` (6 insts!)
**After:** `double16(x) → ADD HL, HL` (1 inst!)
**Root cause:** Template fixup checked `contains(tmpl, ", HL")` which matched `ADD HL, HL` as a store.
**Fix:** Guard with `isLD` — only apply store fixups to `LD` instructions.

### BUG: INC vs double disambiguation
**Before:** `add(x,x)` → `INC HL` (+1 instead of double)
**After:** `add(x,x)` → `ADD HL, HL` (correct double)
**Root cause:** isel `inc_rr` pattern matched `add(x,x)` when `src0 == src1`.

### BUG: Z80 register name collision
**Before:** Pascal variable `I` → `LD IY, I` (Z80 interrupt register!)
**After:** `LD IY, v_I` (sanitized label)

### BUG: 16-bit cond_ret
**Before:** `LD A, DE` (invalid Z80 — 16-bit pair into 8-bit register)
**After:** `EX DE, HL` (correct 16-bit register swap)

### BUG: Assert bootstrap PBQP mismatch
**Before:** Bootstrap loads arg into C (PBQP), function expects B (LIR WFC) → wrong result.
**After:** Z3 soft hints align register choices with PBQP bootstrap.

---

## 3. ASM Quality Assessment

### Leaf Functions — PERFECT Codegen
```z80
; let double (x : u8) : u8 = x + x
double:
    ADD A, A        ; 4T, 1 byte — OPTIMAL
    RET

; let add (a : u8) (b : u8) : u8 = a + b
add:
    ADD A, C        ; 4T, 1 byte — OPTIMAL
    RET

; let main () : u8 = add(3, 4)
main:
    LD A, 7         ; constant folded! — OPTIMAL
    RET

; fun double16(x : u16) : u16 = x + x
double16:
    ADD HL, HL      ; 11T, 1 byte — OPTIMAL (was 6 instructions!)
    RET

; let main () : u8 = 5 |> double |> inc
main:
    LD A, 11        ; pipe fully folded! — OPTIMAL
    RET
```

### Conditional Return — Correct, Room for Improvement
```z80
; fun max_byte(a: u8, b: u8) -> u8
max_byte:
    CP B
    JR NC, .max_byte_cond_ret_0_no
    LD A, B
    RET
.max_byte_cond_ret_0_no:
    JR NZ, .max_byte_cond_ret_0
    LD A, B
    RET
.max_byte_cond_ret_0:
    RET             ; 9 lines — correct but verbose (ideal: 4)
```

### ABAP (Complex Real-World)
```
hello_input.abap: 169 instructions
  LD:99  ALU:11  CALL:12  JP/JR:24  IY-store:24
  Known issues: duplicate LD IY loads, byte-by-byte stores
```

---

## 4. Frill Frontend — New Language

ML-inspired functional language for Z80 retro hardware.

### Syntax
```frill
-- Type-annotated let bindings
let double (x : u8) : u8 = x + x

-- Pipe operator (desugars to nested calls)
let main () : u8 = 5 |> double |> inc

-- If-then-else expressions
let max (a : u8) (b : u8) : u8 = if a > b then a else b

-- Comments: -- line, (* block *)
```

### Implementation
- `pkg/frill/frill.go` — 881 LOC: lexer, parser, AST, HIR lowering
- `pkg/frill/frill_test.go` — 4 tests (lex, parse, compile, pipe)
- Extension: `.frl`
- Compiles through full pipeline: Frill → HIR → MIR2 → LIR → Z80

### Codegen Quality
| Program | Frill Source | Z80 Output | Quality |
|---------|-------------|------------|---------|
| `double(21)` | `let main () : u8 = double(21)` | `LD A, 42 / RET` | PERFECT (const fold) |
| `5\|>double\|>inc` | pipe chain | `LD A, 11 / RET` | PERFECT (const fold) |
| `double` | `x + x` | `ADD A, A / RET` | PERFECT |
| `add` | `a + b` | `ADD A, C / RET` | PERFECT |

---

## 5. E2E Corpus Results

### Overall: 74/81 (91.4%)

| Frontend | Pass | Total | Rate |
|----------|------|-------|------|
| Nanz | ~17 | ~19 | ~89% |
| ABAP | ~8 | ~8 | ~100% |
| Pascal | ~4 | ~4 | ~100% |
| C89 | ~43 | ~48 | ~90% |
| Frill | 2 | 2 | 100% |

### Remaining 7 Failures
| File | Category | Root Cause |
|------|----------|------------|
| arena_allocator.nanz | WFC validate | SBC HL,mem (spill to memory loc) |
| nc.nanz | asm error | Deep LIR LD error in large program |
| assert_test.c | assert mismatch | 16-bit cmp.gt bootstrap issue |
| bench_extended.c | WFC validate | cond_ret variant for is_power_of_two |
| fatfs_lowlevel.c | duplicate label | PBQP splice duplicates globals |
| import_test.c | MIR2 VM bug | Cross-module function resolution |
| struct_promote.c | MIR2 VM bug | Outparam codegen mismatch |

---

## 6. Commit Log

```
07e48b33 feat(lir): Z3 SMT-based optimal register allocation
3704a653 fix(lir): addr_of symbol propagation — LD HL,0 → LD HL,symbol
da545741 feat(lir): equality saturation + Z3 joint isel+regalloc
9801811d feat(lir): global store/load fusion + OpStoreGlobal/OpLoadGlobal
02698130 fix(lir): emit string pool + fix assembly error on global symbols
2f347b76 feat(lir): Z3 --z3 flag + pipeline integration + string dedup
ef624b77 fix(lir): sanitize asm labels — avoid Z80 register name collisions
7045ec06 feat(lir): peephole — duplicate consecutive LD elimination
c086ed21 fix(lir): cond_ret emission + assert bootstrap contract classes
c4dcd3ce fix(lir): Z3 respects PBQP hints — fixes assert bootstrap mismatch
53e22a0b fix(lir): 16-bit cond_ret — EX DE,HL instead of invalid LD A, DE
78f25759 fix(lir): INC/DEC pattern filter — add(x,x) is double not increment
6f3e6381 fix(lir): fixInvalidZ80Template guard — ADD HL,HL is not a store!
068b8952 fix(lir): sanitize $ in symbols + suppress Z3 validate noise
504397e0 fix(lir): emit string pool in PBQP fallback path too
f6928457 feat: Frill frontend — ML-inspired functional language for Z80
```

---

## 7. Architecture: What Z3 Enables Next

```
Current:  MIR2 → Bridge → isel → WFC → emit
               ↓                   ↓
           e-graph           Z3 validate/repair

Future:   MIR2 → Bridge → e-graph → Z3 joint solve → emit
                             ↑              ↑
                     saturation rules   PBQP hints
                     (602K proven)      (soft constraints)
```

**Key insight from this sprint:** The bottleneck was never register allocation (Z3 and WFC produce identical results for simple functions). The real wins came from fixing **pattern selection** bugs (fixInvalidZ80Template, INC vs double, symbol propagation). Z3's greatest value is as a **validation oracle** — catching inconsistencies that WFC's greedy approach misses.

**Next steps:**
1. Z3 for joint isel+regalloc on e-graph variants (ADR-0037 Phase 2)
2. Equality saturation with 602K superoptimizer rules
3. Frill: pattern matching, algebraic data types, type inference
4. Fix remaining 7 failures (spill-to-memory, PBQP splice, 16-bit cmp)
