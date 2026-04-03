# B6 Pre-Split Prototype: Call Save/Restore Before Table Lookup

**Date:** 2026-04-03
**Track:** B6 (bounded prototype, /home/alice/dev/minz only)
**Predecessor:** 951b3487 (B6 enriched table IX/IY gap analysis)

---

## Mechanism

Added `PreSplitForTableLookup(ops)` in `regalloc_table.go` — a lightweight version
of the solver's `splitVRegsAtCalls` that:

1. Identifies vregs defined before a CALL and used after it
2. Inserts `OpMove` save (before call) and restore (after call) for each
3. Produces NO `DstHint` (unlike the solver version which hints IXH)
4. Creates fresh save vregs (v5, v6, ...) that do NOT cross the call boundary

After pre-split, no vreg's live range crosses a CALL, so the enriched table's
GPR-only `locSets8` can produce a correct assignment — save vregs are consumed
immediately, never need to survive a call.

Also added `AnalyzePreSplitEligibility(ops, desc, name)` which runs both the
original and pre-split ops through shape conversion and reports eligibility
changes.

## Hit-Rate Measurement Results

Tested 8 representative function profiles:

| Function | Orig vregs | Split vregs | Saves | Orig eligible | Split eligible | Blocker |
|----------|-----------|-------------|-------|---------------|----------------|---------|
| leaf_2v | 2 | 2 | 0 | yes | yes | - |
| leaf_3v | 3 | 3 | 0 | yes | yes | - |
| leaf_4v | 4 | 4 | 0 | yes | yes | - |
| call_1live_4v | 4 | 5 | 1 | yes | yes | - |
| call_1live_3v | 3 | 4 | 1 | yes | yes | - |
| call_2live_5v | 5 | 7 | 2 | yes | **no** | save_vregs_exceed_6v |
| call_3live_6v | 6 | 8 | 2 | yes | **no** | save_vregs_exceed_6v |
| call2_chain_4v | 4 | 4 | 0 | yes | yes | no_calls_to_split |

**Summary:**
- Original eligible: **8/8 (100%)**
- Pre-split eligible: **6/8 (75%)**
- Newly eligible: **0**
- Lost eligibility: **2** (5v and 6v functions with ≥2 call-live vregs)

## What Helped

- Functions with **≤4 original vregs and 1 call-live vreg** stay within the 6v
  table limit after splitting (4→5v, 3→4v). Pre-split produces a *correct* shape
  where the original shape would have placed a vreg in a call-clobbered GPR.

- The pre-split is cheap (no solver, no Z3, O(n) in ops) and conservative
  (no hints, no architectural changes).

## What Did Not Help / Regressed

- Functions with **≥5 vregs and ≥2 call-live vregs** lose eligibility because
  each save vreg adds +1 to the count, pushing past the 6v table limit.
  This is the fundamental tradeoff: pre-split trades call-safety for vreg count.

- The 6v table limit is the binding constraint. Without raising it (requires
  5v/6v/7v binary table enumeration — a z80-optimizer project), pre-split
  only helps the narrow band of functions with low vreg count AND few call-live vregs.

- **No net gain in the test cases** — all 8 were originally "eligible" by shape
  (the shape converter doesn't model call clobber, so it says "eligible" even
  when the assignment would be wrong). The real gain is *correctness*: pre-split
  makes the table assignment actually usable, not just shape-compatible.

## Key Insight

The original B6 analysis (951b3487) identified that "non-leaf functions miss
the table" — but actually they don't *miss* it (the shape converts fine).
They get *wrong* assignments from it (vreg in GPR across call = clobbered).
Pre-split fixes correctness but at the cost of vreg count expansion.

The trade-off:

| Original vregs | Call-live vregs | After split | Fits 6v? |
|---------------|----------------|-------------|----------|
| 2 | 1 | 3 | yes |
| 3 | 1 | 4 | yes |
| 4 | 1 | 5 | yes |
| 4 | 2 | 6 | yes |
| 5 | 1 | 6 | yes |
| 5 | 2 | 7 | **no** |
| 6 | 1 | 7 | **no** |

Functions with ≤4 original vregs (which is common for small utility functions)
and ≤2 call-live vregs benefit. Functions with ≥5 vregs lose out.

## Tests Run

```
$ go test ./pkg/vir/ -run 'TestPreSplit|TestEnrichedGap|TestLocSets8IX' -v
--- PASS: TestEnrichedGapAnalysis
--- PASS: TestPreSplitForTableLookup
--- PASS: TestPreSplitEligibility
--- PASS: TestPreSplitHitRateMeasurement
--- PASS: TestLocSets8IXDefinitions
PASS

$ go test ./pkg/vir/ -run 'TestLocSet$|TestZ80LocSets|TestPatternMatching|TestInsertSaveMoves|TestLivenessComputation|TestPIROpEmit'
PASS (all existing tests)
```

## Commits

- `951b3487` — B6 gap analysis (previous)
- *(this commit)* — Pre-split prototype + eligibility analysis + hit-rate measurement

Files changed:
- `minzc/pkg/vir/regalloc_table.go` — `PreSplitForTableLookup()`, `AnalyzePreSplitEligibility()`, `PreSplitResult`
- `minzc/pkg/vir/solver_test.go` — `TestPreSplitForTableLookup`, `TestPreSplitEligibility`, `TestPreSplitHitRateMeasurement`

## Recommendation: Continue or Stop?

**Stop for now.** The pre-split prototype works correctly and the diagnostic
infrastructure is useful, but the net value is limited by the 6v table ceiling:

1. **Pre-split alone** helps a narrow band (≤4v, ≤2 call-live). Useful but not
   transformative.

2. **Pre-split + IX-expanded tables** (from the previous B6 exploration) would be
   the real win — save vregs could stay in IXH without counting against the 6v
   GPR limit. But this requires z80-optimizer enumeration work.

3. **Pre-split + 7v/8v tables** would also help, but the binary table sizes grow
   exponentially (7v estimated ~8GB+).

The diagnostic functions (`AnalyzeEnrichedGap`, `AnalyzePreSplitEligibility`) are
the lasting value — they can measure real corpus hit rates when wired into
`VIR_DUMP_GPU_BATCH` or similar batch analysis.

**Do not wire pre-split into the production pipeline** until either (a) IX-expanded
tables exist or (b) corpus measurement on real Nanz/C89 functions shows net positive
hit rate.
