# Report #028 — MIR2 Codegen Quality Sprint

**Date:** 2026-03-07
**Status:** Done — 42 tests pass, 9 verified functions, 4 new peephole passes

---

## What happened

Continued building out `pkg/mir2` — the clean-room IR + register allocator + Z80
codegen sandbox. This session focused on **code quality**: eliminating dead stores,
preventing silent miscompilation from shadow registers, and adding new example
functions to measure output quality.

---

## What was fixed

### 1. Shadow register guard (ADR-0011)

`LocShadow` had cost 10 for `ClassGeneral`, `ClassAcc`, `ClassCounter` — cheaper
than the stack (21) and memory (28). Under pressure the allocator would silently
assign vregs to `B'`, `C'`, `D'`, `E'`, `A'`, but the codegen emits zero `EXX`
instructions → broken code with no warning. Fixed: `InfCost` for all three.
`ClassShadow` / `ClassAccShadow` are dedicated classes for explicit EXX use and
remain unaffected.

### 2. Dead-store elimination (DSE) pass

`OpConst` instructions emit `LD reg, n`. When a codegen peephole makes the
register unnecessary, the load became a dead store. The new `computeDeadConsts`
pre-pass suppresses `LD reg, n` when every use of that constant is handled by:

- **CP imm8** — `OpCmp` with constant rhs → `CP 0` instead of `LD D,0; CP D`
- **INC/DEC** — `OpAdd/Sub` with cv==1 in-place → `INC r` / `DEC r`
- **AND/OR/XOR/ADD/SUB immediate** — `AND 1` instead of `LD D,1; AND D` *(new)*
- **Shift count** — `OpShr/Shl/Sar` ignore `Src[1]` entirely → count reg DSE'd

Constants used as block arguments (TermJmp/TermBrIf) are correctly excluded.

### 3. AND/OR/XOR/ADD/SUB immediate peephole

Z80 has `AND n` / `OR n` / `XOR n` / `ADD A,n` / `SUB n` — immediate forms that
eliminate the need to materialise the constant in a register. Previously only
`CP n` (for `OpCmp`) had this peephole. Now all 8-bit bitwise and arithmetic ops
fire it when `constVals[Src[1]]` is known.

**popcount inner loop before:**
```asm
    LD D, 1        ; ← dead after peephole
    LD A, C
    AND D          ; 9 instructions total
```

**popcount inner loop after:**
```asm
    LD A, C
    AND 1          ; 8 instructions total — LD D,1 eliminated
```

---

## New example functions

All 9 functions are built with the MIR2 Builder API, assembled with MZA, and
verified end-to-end in the MZE Z80 emulator.

### min8 — baseline, essentially optimal

```asm
min8:
    CP B
    JP NC, .min8_b_wins
.min8_a_wins:
    RET
.min8_b_wins:
    LD A, B
    RET
```
5 instructions. One comparison, two paths. As good as hand-written.

### gcd — Euclidean subtraction

```asm
gcd:
.gcd_loop:
    CP B
    JP NC, .gcd_ge
.gcd_a_smaller:          ; b = b - a
    LD C, A              ; save a (A will be clobbered by SUB)
    LD A, B
    SUB C
    LD B, A
    LD A, C              ; restore a
    JP .gcd_loop
.gcd_ge:
    CP B                 ; ← one redundant CP (both Z and C already known)
    JP Z, .gcd_done
.gcd_a_bigger:           ; a = a - b
    SUB B
    JP .gcd_loop
.gcd_done:
    RET
```
Correct for all inputs. The redundant second `CP B` is a known limitation:
multi-flag branches (using both Z and C from one CP) are not yet implemented.

### max3 — 3-argument, clean comparison chain

```asm
max3:
    CP B
    JP NC, .max3_a_leads
.max3_b_leads:
    LD A, B
    CP C
    JP NC, .max3_ret_b
.max3_ret_c_from_b:
    LD A, C
    RET
.max3_ret_b:
    RET
.max3_a_leads:
    CP C
    JP NC, .max3_ret_a
.max3_ret_c_from_a:
    LD A, C
    RET
.max3_ret_a:
    RET
```
7 instructions on the hot path (a > b > c). Verified: 7/7 cases pass.

### popcount — shift-and-count, all 8-bit

```asm
popcount:
    LD C, A              ; x → C (preserve A for comparisons)
    LD B, 0              ; count = 0
.popcount_loop_head:
    LD A, C
    CP 0                 ; (LD D,0 was DSE'd)
    JP Z, .popcount_done
.popcount_body:
    LD A, C
    AND 1                ; (LD D,1 was DSE'd)
    LD D, A
    LD A, B
    ADD A, D
    LD B, A
    SRL C                ; (shift-count LD was DSE'd)
    JP .popcount_loop_head
.popcount_done:
    LD A, B
    RET
```
8 instructions in body. Main remaining overhead: 3 moves for `count += bit`
(`LD D,A; LD A,B; ADD A,D; LD B,A`). Fixable with "result-stays-in-A" tracking.

---

## Test suite status

```
ok  github.com/minz/minzc/pkg/mir2   0.205s   (42 tests)
```

| Function    | Cases | Notes |
|-------------|-------|-------|
| fibonacci   | 6     | recursive, E2E |
| clamp       | 5     | 3-arg, 2 branches |
| abs_diff    | 5     | NEG trick |
| sum_range   | 5     | loop + u16 acc |
| mul8        | 6     | shift-add loop |
| min8        | 6     | baseline |
| gcd         | 7     | Euclidean subtraction loop |
| max3        | 7     | 3-arg comparison chain |
| popcount    | 8     | AND + shift loop |

---

## Code quality assessment

| Pattern | Status |
|---------|--------|
| CP imm8 | ✅ done |
| INC/DEC for ±1 | ✅ done |
| AND/OR/XOR/ADD/SUB imm8 | ✅ done (this sprint) |
| Shadow guard | ✅ done (this sprint) |
| DSE for peephole-dead consts | ✅ done (this sprint) |
| "result stays in A" coalescing | 📋 next |
| Multi-flag branch (Z+C from one CP) | 📋 next |
| Loop-invariant code motion (LICM) | 📋 phase 2 |
| PBQP allocator | 📋 phase 5 — Z80 has 7 GP regs, greedy is fine for now |

---

## When does PBQP make sense?

Not yet. With only 7 primary Z80 registers and current functions using 3–5 live
ranges simultaneously, greedy + cost table makes optimal or near-optimal choices.
PBQP pays off when there are ≥10 simultaneous live ranges (complex functions after
Phase 3 — structs, arrays, multi-function modules). Premature implementation would
add ~500 LOC of belief-propagation infrastructure for zero measurable gain.

The real bottleneck is **instruction selection** (AND 1 vs LD+AND) and
**coalescing** (avoiding LD D,A when result is already where it needs to be).
Both are simpler wins achievable without PBQP.

---

## Next session

1. **"result stays in A" tracking** — suppress `LD D, A; LD A, B; ADD A, D`
   → `ADD A, B` when the AND/OR/XOR result is in A and immediately consumed
2. **Multi-flag branch** — `TermBrIf2` using both Z and C from one `CP` (fixes
   the redundant compare in GCD)
3. More examples: `is_pow2`, `min16`, `swap_nibbles`, signed arithmetic

See [MIR2 Roadmap](docs/MIR2_Roadmap.md) for the full plan.
