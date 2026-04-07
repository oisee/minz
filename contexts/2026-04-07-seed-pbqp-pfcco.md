# Next Session Seed — 2026-04-07 (evening)

**Previous:** QBE u8 truncation fix, PBQP-PFCCO groundwork, VIR bidirectional sync, Z3 parked.
**Wisdom:** [contexts/2026-04-07-wisdom-ixy-codegen-fixes.md](2026-04-07-wisdom-ixy-codegen-fixes.md) (still relevant)

---

## Context

MinZ Z80 compiler. This session made a strategic architecture decision: Z3 solver parked from critical path, PBQP is now the production register allocator. Three commits pushed.

## Commits This Session

1. `49ea9e46` — fix: QBE u8/u16 truncation masks (restores QBE as correctness oracle)
2. `8cb98773` — feat: PBQP-PFCCO solver + VIR sync + pass-through chain fix

## Current Architecture (post-session)

- **Production regalloc:** PBQP (`pkg/mir2/pbqp.go`)
- **Production PFCCO:** Greedy DP (`pkg/mir2/contracts.go` → `optimizeContractsGreedy`)
- **Experimental PFCCO:** PBQP solver (`pkg/mir2/contracts_pbqp.go` → `OptimizeContractsPBQP`)
- **Z3/VIR:** Parked. `--vir` flag still works but loop-header materialization bug remains open
- **QBE oracle:** Restored — u8 wrapping arithmetic correct via `defNarrow()` masks
- **Enriched tables:** O(1) regalloc for ≤6v (unchanged, 79-91% corpus)

## PBQP-PFCCO Status

- **Landed but NOT default** — `OptimizeContracts()` still delegates to greedy DP
- **Works:** Diamond call graphs, short AND long pass-through chains, single-function cost reduction (5/5 tests + guardrail)
- **Pass-through chain gap CLOSED:** `isPassThrough()` zeroes unary cost for simple forwarders (single call, no ALU on params), `candidateParamOnlyChoices()` eliminates return class noise. R1 folding now propagates correctly through arbitrary-length chains.
- **Remaining gap:** Multi-call functions (e.g. `double_sum` calling `double` twice) where `unaryCostWithMod` depends on original (pre-optimization) callee contracts via `inferNaturalClass`. PBQP picks different convention than greedy → wrong codegen in `TestContractOptimize_PreservesOutput`.
- **Next fix:** Decouple `unaryCostWithMod` from original callee contracts — either use candidate callee class, or zero unary for multi-call functions too (with edge costs driving).

## What To Do Next (in priority order)

### 1. Fix PBQP-PFCCO multi-call function gap
`unaryCostWithMod` calls `inferNaturalClass` which reads the **original** callee contract (ClassGeneral) instead of the optimized one. For multi-call functions like `double_sum(a,b) = double(a) + double(b)`, this biases toward ClassGeneral. Fix: decouple unary cost from original contracts — either pass candidate callee choices into inferNaturalClass, or zero unary cost for multi-call pass-through patterns too.
File: `pkg/mir2/contracts_pbqp.go` + `contracts.go` (inferNaturalClass). Tests: `TestContractOptimize_PreservesOutput` + `TestContractScale_Long` are the guardrails.

### 2. Joint caller+callee allocation via enriched tables
Alice's idea: if caller has 2 live-across-call + callee has 2-3v = 4-5v total → solve as ONE enriched table lookup. "Inlining allocation without inlining code." No existing Z80 compiler does this. Needs: call graph + live-across-call analysis + merged interference graph → existing GPU tables. **Research/design phase.**

### 3. FatFS blocker: `&local_var` address-taken (169 occurrences)
Medium effort. Needs address-taken marking in semantic analysis → force local to memory with emitted label → `OpAddrOf` resolves. See `reports/2026-04-06-Claude-FatFS-Precision-Blockers.md`.

### 4. SRL/SRA/SLA IX half-reg audit
Same class as BIT/SET/RES fix (ab362576). CB prefix encoding doesn't support IX halves. Not yet triggered but structurally vulnerable. Small fix in `z80codegen.go`.

## What To Avoid

- Don't switch PBQP-PFCCO to default until pass-through chain gap is closed
- Don't touch Z3/VIR solver code — it's parked, not broken
- Don't modify Tetris workarounds — wait for VIR loop-header fix (if ever needed with PBQP default)
- Don't run full FatFS compilation without checking cycle cost first

## Active Coordination

- `~/dev/minz-vir` — codex (pfi10zf3:main) handles VIR backend, roadmap, docs
- VIR files synced bidirectionally as of 8789f88a — don't re-sync without checking
- Use `dedelulu explore` before sending — session IDs change
- Always reply via dedelulu when completing cross-session tasks

## Corpus Status

- 14/14 contract tests pass (9 existing + 5 new PBQP-PFCCO)
- 13/13 QBE tests pass (11 existing + 2 new u8 wrap)
- MIR2: 1 pre-existing failure (TestStrLenZ80 — DB string format, unrelated)
- VIR loop-header: TestVIR_Assert_LookupLoopHeader FAIL (Z3 issue, parked)
