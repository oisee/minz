# B6 Exploration: Enriched Table IX/IY-Expanded Shapes

**Date:** 2026-04-03
**Track:** B6 (exploratory, /home/alice/dev/minz only)
**Scope:** VIR regalloc enriched table — IX/IY half-register gap analysis

---

## Current State

The VIR enriched table provides O(1) register allocation for functions with 2-6 vregs.
Three lookup tiers exist:

| Tier | Format | Coverage |
|------|--------|----------|
| JSONL hash | `shapeHash:opBagHash` | 37.6M entries |
| Z80T binary (4v) | Enumeration index | 4MB, loaded by default |
| Z80T binary (5v/6v) | Enumeration index | 387MB/899MB, env opt-in |

The shape converter (`VIRToEnrichedShape` in `regalloc_table.go:870`) maps each vreg
to one of **4 constraint classes** (`locSets8` in `enriched_index.go:9`):

| Index | Name | Locations |
|-------|------|-----------|
| 0 | must_A | {A} |
| 1 | must_C | {C} |
| 2 | any_gpr8 | {A,B,C,D,E,H,L} |
| 3 | any_gpr8_except_A | {B,C,D,E,H,L} |

**None of these include IXH/IXL/IYH/IYL (indices 10-13).**

The binary tables CAN output assignments to IXH/IXL (the GPU enumerator considers
15 physical locations), but only within the locSet constraint space. Since no locSet
includes IX halves, the enumerator never considers placing a vreg there.

## Concrete Bottleneck

**Functions with calls where vregs survive across the call cannot get correct
table-based allocation.**

Here's why:

1. Call splitting (save/restore moves for call-surviving vregs) happens *inside*
   the Z3 solver (`solver.go:135-289`), **not** before table lookup.
2. Table lookup sees the raw ops including `OpCall` but without save moves.
3. `VIRToEnrichedShape` classifies call-surviving vregs as `any_gpr8` (locSet 2).
4. All 7 GPR8 registers are call-clobbered on Z80. The table assignment will put
   the surviving vreg in a GPR that gets destroyed by the call.
5. `verifyABICompat` at `pipeline.go:495` may catch some of these, but the
   fundamental shape doesn't model call clobber.
6. These functions always fall through to Z3, which handles it correctly via
   IXH/IXL (8T, call-safe) or PUSH/POP (21T).

**Impact:** Every non-leaf function with live-across-call vregs misses the table.
In the Nanz corpus, this is roughly 40-50% of functions (any function that calls
another function with a result used afterward).

## What Was Changed

### 1. `enriched_index.go` — Added `locSets8IX` definition

```go
var locSets8IX = [][]int{
    {0},                                              // 0: must be A
    {2},                                              // 1: must be C
    {0, 1, 2, 3, 4, 5, 6},                            // 2: any GPR8 (original)
    {1, 2, 3, 4, 5, 6},                               // 3: any GPR8 except A
    {10, 11, 12, 13},                                  // 4: IX/IY halves only (call-safe)
    {0, 1, 2, 3, 4, 5, 6, 10, 11, 12, 13},            // 5: any GPR8 + IX halves
}
```

This is a **diagnostic definition only** — it defines what an IX-expanded enumeration
would look like. The binary tables would need regeneration (z80-optimizer) to use it.

### 2. `regalloc_table.go` — Added `AnalyzeEnrichedGap()` diagnostic

New function `AnalyzeEnrichedGap(ops, desc, funcName)` returns an `EnrichedGapInfo`
struct that classifies WHY a function misses the enriched table and whether IX-expanded
locSets would help. Categories:

- `shape_ok` — function fits current table (leaf, no call pressure)
- `call_pressure` — has call-live vregs, would benefit from IX expansion
- `too_many_vregs` — >6 vregs, beyond table capacity
- `trivial` — <2 vregs, no allocation needed

### 3. `solver_test.go` — Added 2 test functions

- `TestEnrichedGapAnalysis` — verifies leaf/call/big-vreg classification
- `TestLocSets8IXDefinitions` — verifies IX-expanded locSet structure

## Tests Run

```
$ go test ./pkg/vir/ -run 'TestEnrichedGap|TestLocSets8IX' -v
=== RUN   TestEnrichedGapAnalysis
    leaf_add: {NVregs:3 HasCall:false CallLiveVregs:0 MissReason:shape_ok WouldBenefitIX:false}
    call_add: {NVregs:4 HasCall:true CallLiveVregs:2 MissReason:call_pressure WouldBenefitIX:true}
    too_big:  {NVregs:8 HasCall:false CallLiveVregs:0 MissReason:too_many_vregs WouldBenefitIX:false}
--- PASS: TestEnrichedGapAnalysis
=== RUN   TestLocSets8IXDefinitions
--- PASS: TestLocSets8IXDefinitions
PASS

$ go test ./pkg/vir/ -run 'TestLocSet$|TestZ80LocSets|TestPatternMatching|TestInsertSaveMoves'
--- PASS: TestLocSet
--- PASS: TestZ80LocSets
--- PASS: TestPatternMatching
--- PASS: TestInsertSaveMoves
PASS
```

All existing tests continue to pass.

## Commits

*(To be created after review)*

Files changed:
- `minzc/pkg/vir/enriched_index.go` — added `locSets8IX`
- `minzc/pkg/vir/regalloc_table.go` — added `AnalyzeEnrichedGap()` + `EnrichedGapInfo`
- `minzc/pkg/vir/solver_test.go` — added 2 test functions

## Recommendation: Is This Path Worth Continuing?

**Short answer: Yes, but the payoff requires work in z80-optimizer, not here.**

The enriched table currently provides O(1) allocation for **leaf functions only**
(in practice). The IX-expansion would unlock non-leaf functions (~40-50% of corpus),
but requires:

1. **z80-optimizer**: Extend `regalloc-enum` to enumerate with 6 locSets instead of 4.
   This multiplies the enumeration space by ~(6/4)^nVregs per width combo. For 4v:
   current = 4^4 = 256 locSet combos; expanded = 6^4 = 1296 (~5x more shapes).
   For 5v: 4^5=1024 → 6^5=7776 (~7.6x). Binary table sizes grow proportionally.

2. **VIR shape converter**: Update `VIRToEnrichedShape` to classify vregs as
   locSet 4 (IX-only) or 5 (GPR+IX) based on call-liveness analysis. This is
   straightforward once the binary tables exist.

3. **Alternative (cheaper)**: Pre-insert call save/restore moves BEFORE table lookup,
   so the table sees the expanded op sequence where save vregs are already separated.
   The save vregs would be classified as `any_gpr8` and could land in GPR (since
   they don't cross the call boundary after splitting). This avoids regenerating
   tables entirely — **recommend trying this first.**

**Priority:** Option 3 (pre-split before table lookup) is the lowest-cost highest-value
change. It could be done entirely in `pipeline.go` by calling `insertCallSaves()`
before the table lookup block. However, it would increase vreg count (potentially
pushing 5v functions to 7v, beyond table range), so it needs measurement first.
