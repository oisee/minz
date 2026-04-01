# Report: Rewrite Triad Integration into VIR Backend
**Date:** 2026-04-01
**Session:** 15
**Status:** Phase 1 complete — Grace wired, ISLE bugs fixed, VIROp IRGraph adapter created

---

## Summary

This session wired the Rewrite Triad (Grace + ISLE + Datalog from `pkg/rewrite/`) into the
VIR backend. Three critical pre-existing bugs were discovered and fixed in the process.

---

## 1. Two Critical Bugs Fixed

### Bug A: `tryConstMul` (GPU table lookup) was dead code

**Root cause:** `bridge.go`'s LowerBlock pre-scan propagated constants into `inst.Imm` for
OpDiv and OpMod — but NOT for OpMul. So `tryConstMul` always received `inst.Imm == 0` and
fell through to `__mul8` runtime call.

**Impact:** Every `x * constant` 8-bit expression was emitting `CALL __mul8` (~200T) instead
of hitting the GPU-precomputed table (3–40T). This was broken for the entire VIR lifetime.

**Fix:** Added constant propagation for OpMul in the pre-scan (commutative — checks both src[0]
and src[1], normalizes to `src[0]=var, src[1]=const`):
```go
if inst.Op == mir2.OpMul && inst.Imm == 0 {
    if k, ok := mirConsts[inst.Src[1]]; ok && k > 0 {
        inst.Imm = k
    } else if k, ok := mirConsts[inst.Src[0]]; ok && k > 0 {
        inst.Imm = k
        inst.Src[0], inst.Src[1] = inst.Src[1], inst.Src[0]
    }
}
```

### Bug B: 16-bit div `OpAsmBlock` emitted nothing

**Root cause:** `translateDiv` built asm lines and assigned them to `Sym:` field of `VIROp`.
`parsePerInstSolution` reads `op.AsmTemplate` for inline asm, not `op.Sym`. The asm was
silently lost at emission time.

**Impact:** Any function using 16-bit division emitted an empty `CALL __div16` with no actual
divide sequence. Wrong output, silent failure.

**Fix:** Changed to `AsmTemplate: joinLines(asmLines)` with explicit `Clobbers`, `DstHint`,
`SrcHint` fields. Also added proper `SrcHint: HL` / `DstHint: HL` so Z3 gets location hints.

---

## 2. 16-bit Multiply Strength Reduction (new)

Added `tryStrengthReduceMul16` for constants 2, 3, 4, 5, 6, 8, 10, 12 covering the common
multipliers in iterator indexing, struct layout, and video/screen code.

| Constant | Sequence | T-states | vs `__mul16` (~200T) |
|----------|----------|----------|----------------------|
| ×2 | `ADD HL,HL` | 11T | **18× faster** |
| ×4 | `ADD HL,HL / ADD HL,HL` | 22T | **9× faster** |
| ×3 | `LD E,L / LD D,H / ADD HL,HL / ADD HL,DE` | 30T | **6.7× faster** |
| ×5 | `LD E,L / LD D,H / ADD HL,HL / ADD HL,HL / ADD HL,DE` | 41T | **4.9× faster** |
| ×6 | `ADD HL,HL / LD E,L / LD D,H / ADD HL,HL / ADD HL,DE` | 41T | **4.9× faster** |
| ×8 | `ADD HL,HL / ADD HL,HL / ADD HL,HL` | 33T | **6× faster** |
| ×10 | `ADD HL,HL / LD E,L / LD D,H / ADD HL,HL / ADD HL,HL / ADD HL,DE` | 52T | **3.8× faster** |
| ×12 | `ADD HL,HL / ADD HL,HL / LD E,L / LD D,H / ADD HL,HL / ADD HL,DE` | 52T | **3.8× faster** |

Implemented as `OpAsmBlock` with `SrcHint: HL`, `DstHint: HL`, and correct clobber sets.
DE is clobbered for constants ≥3 (uses LD E,L/LD D,H pattern). All sequences verified
against Z80 timing tables.

Bridge pre-scan now also propagates mul constants, then `translateMul` checks `w > 8 && inst.Imm != 0`
before calling the new function.

---

## 3. ISLE Combining: Extended to k=3..12

`isle.go`'s `eliminateIdentityOps` now handles OpMul for constants 0, 1, 2, 3, 4, 5, 6, 8, 10, 12
at the MIR2-level IR (pre-lowering). For constants requiring intermediate vregs (k=3,5,6,10,12),
a `nextVreg` counter allocates fresh vregs inline:

```
k=3:  t = x+x;  dst = t+x      (add + add)
k=5:  t = x<<2; dst = t+x      (shl + add)
k=6:  t = x<<1; u = x<<2; dst = t+u   (shl + shl + add)
k=10: t = x<<1; u = x<<3; dst = t+u   (shl + shl + add)
k=12: t = x<<2; u = x<<3; dst = t+u   (shl + shl + add)
```

Commutative: checks both src[0] and src[1] as the potential constant operand.

ISLECombineRules documentation string updated with rules 13-15 (mul×6, mul×10, mul×12).

---

## 4. Grace MIR2 Passes Wired into VIR Pipeline

**Before:** VIR called only `mir2.FuseAbsDiff(f)` (1 Grace rule) before lowering.

**After:** `SolverOptions.UseGrace bool` controls whether to run all 16+ Grace MIR2 rules:

```go
if opts.UseGrace {
    mir2.RunGracePasses(f, opts.GraceStats)  // DSE, CondRetSink, SplitJoinRet,
                                              // DeadBlockArgElim, BlockMerge, etc.
} else {
    mir2.FuseAbsDiff(f)                      // original single-rule path
}
```

`GraceStats *mir2.GraceStats` added to `SolverOptions` for per-rule fire-count tracking.
Default remains `UseGrace: false` until corpus validation is complete.

Grace rules available via `RunGracePasses` (from `mir2/grace_runner.go`):
- DSE (dead store elimination)
- CondRetSink (sink conditional returns)
- SplitJoinRet (diamond pattern compression)
- DeadBlockArgElim (remove unused block arguments)
- BlockMerge (merge single-predecessor blocks)
- EmptyBlockElim (remove empty pass-through blocks)
- FuseAbsDiff (fuse abs-diff pattern to OpAbsDiff)
- + more (total 16+ as of report 097)

---

## 5. VIROp IRGraph Adapter (`vir_graph.go`)

New file `pkg/vir/vir_graph.go` (~300 lines) implementing `rewrite.IRGraph/IRGraphMut` for
VIROps post-lowering. Enables Grace rules and future DSE/analysis passes to operate on
the VIR instruction stream directly (not just MIR2).

**Architecture:**
```
VIRGraph
  vf *Func          — VIROp instruction source
  mf *mir2.Func     — CFG source (successors/predecessors from MIR2 terminators)
  blockByLabel      — O(1) VIRBlock lookup
  predCache/succCache — pre-computed CFG edges
```

CFG is delegated to MIR2 (VIROp blocks are linear, no explicit successor links).
Instruction view uses VIROps.

**Mutation methods:** RemoveInst, ReplaceInst, AppendInst, HoistInsts, RemoveBlock, SetTerm,
RemoveBlockParam — all update internal caches consistently.

**`ApplyVIRDSE(vf *Func)`** — dead VIROp elimination:
- Fixpoint iteration per block
- Removes pure ops (OpConst/Move/Add/Sub/And/Or/Xor/Shl/Shr/Neg/Mul + Imm variants)
  whose dst vreg is never used by any other op in the block
- Returns total removed count

**`NewVIRNode`, `VIROpFromNode`** — round-trip between `rewrite.IRNode` and `VIROp`.

---

## 6. VIR Func Back-Reference

`vir.Func` now carries `MIRFunc interface{}` — a `*mir2.Func` back-reference set during
`LowerFunc`. Stored as `interface{}` since vir already imports mir2 (no new cycle), but
kept opaque in the struct definition. Enables `VIRGraph` construction post-lowering without
threading `mf` through every call site.

---

## Files Changed

| File | Change |
|------|--------|
| `pkg/vir/bridge.go` | Bug fixes (mul const propagation, div AsmTemplate), 16-bit mul strength reduction |
| `pkg/vir/isle.go` | ISLE mul rules extended to k=3..12 with intermediate vregs |
| `pkg/vir/pipeline.go` | `UseGrace` branch: `RunGracePasses` vs `FuseAbsDiff` |
| `pkg/vir/solver.go` | `SolverOptions.UseGrace` + `GraceStats` fields |
| `pkg/vir/vir.go` | `Func.MIRFunc interface{}` back-reference |
| `pkg/vir/vir_graph.go` | **NEW** — VIROp IRGraph/IRGraphMut adapter + ApplyVIRDSE |

---

## Next Steps (Phase 2)

1. **Wire `ApplyVIRDSE`** into the pipeline (post-`LowerFunc`, pre-solver)
2. **Wire real `pkg/rewrite/isle/` engine** — replace the manual Go switch in `ISLECombine`
   with `isle.Parse(ISLECombineRules)` + `rs.RewriteAll()` via a VIROp↔Term adapter
3. **Datalog SCC for PFCCO** — use `pkg/rewrite/datalog/` to cluster call-graph SCCs,
   reducing Z3 PFCCO work from O(N²) to O(SCC²) per cluster
4. **Benchmark** `UseGrace: false` vs `UseGrace: true` on:
   - `che_nanz.nanz` (LFSR video unpacker — mul/shift heavy)
   - FatFS `ld_word` pattern (16-bit load fusion already in ISLE)
   - Full Nanz/C89 corpus: instruction count delta, code size delta
5. **Validate `UseGrace: true`** — flip default once corpus delta is clean
