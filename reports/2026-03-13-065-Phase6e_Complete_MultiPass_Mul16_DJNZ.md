# Report #065 — Phase 6e Complete: Multi-Pass Contracts + mul16/DJNZ Nudges

**Date:** 2026-03-13
**Status:** 23/23 showcase PASS · 23/23 pkg tests PASS

---

## Summary

Phase 6e PBQP affinity nudges are now fully implemented.  This session added:

| Change | File(s) | Impact |
|--------|---------|--------|
| **Multi-pass OptimizeContracts** | `contracts.go` | Pass 1+ include incoming edge cost; loop→ convergence |
| **`incomingEdgeCost`** | `contracts.go` | Sums what all callers pay for each contract candidate |
| **`choiceEqual`** | `contracts.go` | Convergence check between passes |
| **mul16 rhs→DE nudge** | `pbqp_affinity.go` | Biases multiplier toward DE (saves 8T setup) |
| **DJNZ counter→B nudge** | `pbqp_affinity.go` | Biases counter toward B (saves 4T `LD B,r`) |
| **Showcase snapshots updated** | `ex9b`, `ex14` | Multi-pass changed allocator decisions |

---

## Multi-Pass Contract Optimisation

### Problem

`OptimizeContracts` was a single bottom-up greedy DP: process callees before callers.
When optimising `swap`, no callers were in the contract set yet, so the optimizer
couldn't see what cost N callers would pay for a twisted ABI.

The classic example:
- Convention B (twisted: bare RET inside swap, EX DE,HL at each call site):
  body cost = 0T, per-caller cost = 4T (EX), total for N callers = 4N T
- Convention A (standard: EX inside swap, no setup at call sites):
  body cost = 4T, per-caller cost = 0T, total = 4T

For N≥2, Convention A is cheaper — but the optimizer couldn't see this.

### Solution

```go
// Pass 0: bottom-up as before (no incoming cost — callers not yet decided)
// Pass 1+: re-evaluate with incomingEdgeCost included
for pass := 0; pass < maxPasses; pass++ {
    changed := false
    for _, name := range cg.Order() {
        ...
        if pass > 0 {
            inc = incomingEdgeCost(f, choice, m, cs, ct)
        }
        ...
    }
    if !changed { break }
}
```

`incomingEdgeCost(f, choice, m, cs, ct)` iterates all callers of `f`, computes
the class move cost each caller pays for each arg, and sums them.

### Empirical result (N=3 callers of swap)

Pass 0: Conv B chosen (unary cost 0 < 8T for Conv A) → bare RET, EX × 3 at callers
Pass 1: Conv A total (8T) < Conv B total (0 + 24T = 24T) → optimizer switches to Conv A

However, the PBQP interference pattern in the swap body pushes physical allocation
back to Convention B regardless of the contract class. The optimizer correctly chooses
(a=ClassPointer, b=ClassPair) but PBQP resolves a=DE, b=HL due to liveness overlap.
**Root cause:** the body contains stale crossover-move instructions synthesized at
HIR lowering time; changing the contract class without re-lowering the body moves
has no physical effect. Fixing this requires either:
1. Function inlining + copy propagation (see TODO-NICE-TO-HAVE below), or
2. Re-synthesis of swap body after contract convergence.

Despite not fixing the swap ABI case, the multi-pass DOES improve other functions:

### Assembly changes (multi-pass)

**ex9b factorial_fold**: trampoline block eliminated:
```z80
; Before: 3-instruction trampoline
JRS NC, .factorial_fold_trmp0
...
.factorial_fold_trmp0:
    LD D, C
    JRS .factorial_fold_loop_exit4

; After: direct jump
JRS NC, .factorial_fold_loop_exit4
```

**ex14 fold_assert / sum_to**: 4-instruction trampoline eliminated:
```z80
; Before:
JRS NC, .sum_to_trmp0
...
.sum_to_trmp0:
    LD A, C  ;  ┐
    LD C, D  ;  │  4 instructions
    LD D, A  ;  │  eliminated
    JRS .sum_to_loop_exit4  ;  ┘

; After:
JRS NC, .sum_to_loop_exit4
```

---

## Phase 6e mul16 rhs→DE Nudge

### Z80 mul16 register layout

```z80
; genMul16 expects:
;   BC = multiplicand (lhs, copied from HL)
;   DE = multiplier   (rhs)
;   HL = result = 0
;   A  = iteration counter (16)
```

If rhs is already in DE, the 8T setup (`LD D,h; LD E,l`) is skipped.

### Implementation

```go
const Mul16DEReward = 8  // T-state savings: skip LD D,h + LD E,l

// Pattern: OpMul, 16-bit type, Src[1] = rhs
// Reward: lower rhs's cost for DE by 8T
```

---

## Phase 6e DJNZ Counter→B Nudge

### Intent

DJNZ implicitly uses B as the counter.  If the counter arrives in another register
(e.g. C from ClassGeneral), the codegen emits `LD B, counter` (4T).

```go
const DJNZCounterReward = 4  // saves LD B, counter_reg

// Pattern: TermDJNZ.Counter
// Reward: lower Counter's cost for B by 4T
```

### Current limitation

`TermDJNZ.Counter` is the body block's ClassCounter param — already costed at B=0.
The nudge has no effect on EXISTING showcase examples because the param is already
ClassCounter.

The `LD B, C` in `sum_chain` / `max_chain` comes from the **entry→fe_head parallel
copy**: function param `n` (ClassGeneral=C) being passed to the loop head's counter
block param (ClassCounter=B).  Eliminating that requires pre-coalescing the function
param with the block param — blocked by the same OpPtrAdd constraint as BUG-001.

The nudge is correct and will activate automatically once PreallocCoalesce is wired in.

---

## TODO: NICE TO HAVE — Swap Algebraic Simplification / Inlining

**Observation:** `let (a, b) = swap(a, b); print_val(b)` should simplify to
`print_val(a)` — the second return of swap IS the first argument unchanged.

**Root cause:** the compiler tracks register allocations through calling conventions
but doesn't do semantic inlining.  After inlining `swap`'s body:

```
; Inline swap(a, b) → (b_old, a_old):
let a' = b_old
let b' = a_old
print_val(b')   ; b' = a_old = original a → no swap needed!
```

Standard SSA + copy propagation + DCE resolves this to `JP print_val`.

**What's needed:**
- Function inliner for trivial functions (1-3 instructions)
- Copy propagation across multi-return destructuring (`TupleLetStmt`)
- The bare-RET / twisted-ABI optimization then becomes truly free:
  no EX at call site AND no register confusion at use site

**Not urgent:** the current code is CORRECT (produces right answers), just not
optimally terse.  Track as future optimization pass.

---

## Test Results

```
23/23 showcase examples:  PASS
23/23 pkg test packages:  PASS (go test ./pkg/... -vet=off)
Assembly regressions:      0
```

## Phase 6e Status

| Pattern | Status | Expected gain |
|---------|--------|----|
| LUT BC★/DE★ codegen | ✅ Done | 4T per access when index in C/E |
| PBQP affinity nudge (C/E) | ✅ Done | biases allocator toward BC★ path |
| mul16 rhs→DE nudge | ✅ Done | 8T when rhs lands in DE |
| DJNZ counter→B nudge | ✅ Done (pending PreallocCoalesce) | 4T when wired |
| Multi-pass OptimizeContracts | ✅ Done | improved ex9b + ex14 allocation |
| incomingEdgeCost | ✅ Done | enables N-caller balancing (swap needs inliner) |
