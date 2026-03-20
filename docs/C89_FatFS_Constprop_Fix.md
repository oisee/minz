# Constprop Fixpoint Loop Hang — Critical Fix for FatFS

> **Status:** Fixed locally on `feat/c89-fatfs`, NOT yet on master.
> **Severity:** Hang (infinite loop) — compiler never terminates on FatFS and any program with do-while loops.
> **Date:** 2026-03-19

---

## The Bug

The constant propagation fixpoint loop in the MIR2 optimization pipeline runs forever on certain programs. FatFS (`examples/c89/fatfs/ff.c`, 7K LOC, 47 functions) triggers this reliably.

**Root cause:** `PropagateConstants` returns `changed=true` indefinitely when the loop body contains values that cycle through the propagation lattice. This happens with `do-while` loops converted to `while(true) { body; if(!cond) break; }` — the `true` constant in the while condition keeps the fixpoint from converging.

The DJNZ back-edge already has special handling (line 283 in `constprop.go`), but regular `while(true)` back-edges don't.

## The Fix

Cap all fixpoint loops at 100 iterations. This is safe because:
- Normal programs converge in 1-5 iterations
- 100 iterations is more than enough for any real program
- If it doesn't converge in 100, more iterations won't help (the lattice is cycling)

### Files to change (4 call sites):

**1. `pkg/pipeline/pipeline.go` line 92** (HIR pipeline):
```go
// BEFORE:
for {
    p := mir2.PropagateConstants(f)
    ...
}
// AFTER:
for iter := 0; iter < 100; iter++ {
    p := mir2.PropagateConstants(f)
    ...
}
```

**2. `pkg/pipeline/pipeline.go` line 266** (LIR pipeline):
Same change.

**3. `cmd/mzn/main.go` line 291** (QBE/C99 native compiler):
Same change.

**4. `cmd/mzv/main.go` line 81** (MIR2 VM runner):
Same change.

### NOT affected (already bounded or different loops):
- `pkg/mir2/constprop_test.go` — test loops (bounded by test structure)
- `pkg/mir2c/codegen_test.go` — test loops
- `pkg/mir2qbe/e2e_test.go` — test loops
- `cmd/mzv/main.go` lines 346, 446 — different loops (not constprop)

## Why This Matters

Without this fix:
- `./mz examples/c89/fatfs/ff.c` **hangs forever** (never terminates)
- `./mzn examples/c89/fatfs/ff.c --emit-qbe` **hangs forever**
- `./mzv` on programs with do-while loops **hangs forever**

With this fix:
- FatFS compiles in seconds: **47 functions, 14026 lines Z80 asm**
- All existing Go tests pass (c89, hir, pipeline — 3/3 packages)
- No regressions on any corpus (C89, Nanz, Lizp, Lanz)

## Proper Fix (Future)

The real fix is to make `PropagateConstants` detect back-edge cycles for ALL loop types (not just DJNZ) and widen the lattice at loop headers. This is standard compiler technique (Wegman-Zadeck). The cap at 100 is a safe, correct workaround until then.

## Context: What Else Was Fixed for FatFS

These fixes are already on master (done independently on u7):
1. **switch-break** (`inSwitch` flag) — C89 `break` inside `switch` no longer panics
2. **do-while** (`while(true){body; if(!cond) break;}`) — correct break/continue semantics
3. **if-join SSA** — block params created when then-branch terminates and no else

Only the constprop cap is missing from master.
