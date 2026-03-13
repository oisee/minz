# Bug Fixes (BUG-002, BUG-004, BUG-005) + Phase 6e LUT BC★/DE★

**Date:** 2026-03-12
**Status:** All fixes merged, 23/23 pkg tests pass

---

## Bugs Fixed

### BUG-005: `applySubSwapNeg` missing u16 guard (`condret.go`)

One-line fix: after hoisting a `neg(r_h)` for the swapped branch,
`inst.Cls` is now set conditionally:

```go
if h.Ty.Width() > 8 {
    inst.Cls = ClassPointer   // u16 result → HL
} else {
    inst.Cls = ClassAcc       // u8 result → A
}
```

Previously always set `ClassAcc`, which caused the 16-bit negation result
to be placed in A (8-bit) instead of HL.

---

### BUG-004: Non-zero-lo LUT pipeline ordering (`pipeline.go`)

`LUTGen` was running before `OptimizeContracts`, so the contract optimizer
would re-infer the parameter class after LUTGen had synthesized a fixed
IR that depended on `ClassAcc`.

**Fix:** In both `CompileHIRSteps` and `CompileHIRWithOptions`, moved
`mir2.LUTGen(m)` to run **after** `OptimizeContracts` + `ApplyContracts`:

```go
// Phase 5b: contract optimisation
ct := mir2.Z80CostTable{}
cs := mir2.OptimizeContracts(m, ct)
mir2.ApplyContracts(m, cs)

// Module-level: LUT synthesis — MUST run after contract opt (BUG-004).
mir2.LUTGen(m)
```

---

### BUG-002: forEach constant rematerialization (`z80codegen.go`)

**Problem:** When a loop entry block receives `(counter, accumulator=0)` as
parallel-copy args, the allocator emits `LD D,0` then further register moves
to get `0` into the expected slot.  The inline emission of `LD dst, imm`
clobbered live source registers that had not yet been moved.

**Fix:** Extended `parallelCopy` struct with:

```go
type parallelCopy struct {
    srcName string
    dstName string
    ty      Ty
    isImm   bool    // NEW
    immVal  int64   // NEW
}
```

In `buildBlockCopies`:
- Check `src == dst` first (pre-coalesced or same location → skip)
- If `constVals[arg]` hits, record `isImm=true` instead of emitting inline

In `emitParallelCopy`:
- All register-to-register moves resolved via existing cycle-safe algorithm
- Immediate emissions (`LD dst, imm`) follow after — guaranteed no aliasing conflict

---

## Phase 6e: LUT BC★/DE★ Optimization

### Background

The Z80 lookup table pattern historically used HL as the address register:

```z80
; HL path (18T):
LD H, sym^H    ; 7T — high byte of page-aligned table
LD L, idx      ; 4T — 8-bit index
LD A, (HL)     ; 7T — load from table
```

If the index byte is already in C or E, the BC/DE register pairs can be
exploited instead:

```z80
; BC★ path (14T, index in C):
LD B, sym^H    ; 7T — table high byte into B
LD A, (BC)     ; 7T — load via BC (no LD L needed!)

; DE★ path (14T, index in E):
LD D, sym^H    ; 7T
LD A, (DE)     ; 7T
```

**Savings: 4T per LUT access** when the index lands in C or E.

### Part 1: Codegen (`z80codegen.go` — OpLoad handler)

Added BC★/DE★ emission in `scanLUTPatterns` handler:

```go
if pat, ok := g.lutLoadPat[inst.Dst]; ok {
    src8 := g.loc(pat.src8Reg)
    sym := sanitizeIdent(pat.sym)
    switch src8 {
    case "C":
        g.emitf("    LD B, %s^H    ; BC★ LUT (14T)", sym)
        g.invalidate("B")
        g.emitf("    LD A, (BC)")
        g.invalidate("A")
    case "E":
        g.emitf("    LD D, %s^H    ; DE★ LUT (14T)", sym)
        g.invalidate("D")
        g.emitf("    LD A, (DE)")
        g.invalidate("A")
    default:  // HL path (18T)
        g.emitf("    LD H, %s^H", sym)
        g.invalidate("H")
        g.emitf("    LD L, %s", src8)
        g.invalidate("L")
        g.emitf("    LD A, (HL)")
        g.invalidate("A")
    }
    break
}
```

### Part 2: PBQP Affinity Nudge (`pbqp_affinity.go` — NEW FILE)

To bias the register allocator toward placing LUT index bytes in C or E,
a pre-solver cost nudge is applied:

**Pattern detection** (`collectLUTSrc8Regs`):
```
src8 → OpExt(u8→u16) → idx16 → OpPtrAdd(base, idx16) → OpLoad(u8)
```

**Cost reduction** (`applyLUTAffinityNudge`):
```
LUTBCStarReward = 4   (T-state savings)

For each src8 that feeds a LUT access:
    states[src8].costs[idxC] -= 4   (if finite)
    states[src8].costs[idxE] -= 4   (if finite)
```

Called from `PBQPAllocate` immediately after cost vectors are built, before
the R0/R1 reduction loop.

**Why this is safe:** The nudge is small (4 units ≪ spill cost ~100+).
A false positive (table not actually page-aligned) is harmless — C and E
are valid general-purpose registers, and the codegen falls back to the HL
path if the index isn't in C/E at emit time.

### `regState` promotion

`regState` was a local type inside `PBQPAllocate`.  Promoted to package level
in `pbqp.go` so `pbqp_affinity.go` can reference it without duplication.

---

## BUG-001 Status: Partial Progress

Pre-coalescing infrastructure (`PreallocCoalesce` in `precoalesce.go`) is
implemented and correct (Union-Find with path compression, safe class-conflict
guards, full IR remapper).  However, wiring it into `PBQPAllocate` caused
cascading regressions:

- `ptr` variable moved from BC → IX (valid per class, but breaks OpPtrAdd codegen)
- SSA violations: multiple instructions with same Dst after remapping

**Decision:** Defer full pre-coalescing to a future sprint when OpPtrAdd
codegen is register-agnostic.  The Phase 6e affinity nudge provides a partial
improvement for the common LUT index case (GCD's main bottleneck).

---

## Test Results

```
23/23 pkg test packages: PASS
All pre-existing failures unchanged (ex2/ex3/ex9b — pre-existing)
```
