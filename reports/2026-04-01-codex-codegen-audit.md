# Codegen Audit: Current Priorities, Visible Gaps, and Suggested Roadmap

Date: 2026-04-01
Author: Codex
Scope: `CLAUDE.md`, recent session context in `contexts/`, and current implementation in `minzc/pkg/vir` and `minzc/pkg/mir2`

## Summary

The current codegen story is no longer "we need a new backend". The repo already has the shape of the intended backend strategy:

- MIR2 legacy codegen still exists and still carries many local TODO/fallback edges.
- VIR is the real active path for quality work.
- The highest-value work is now concentrated in a narrow band:
  - make the VIR fast paths hit more often;
  - replace remaining generic runtime patterns with GPU-backed intrinsics;
  - turn EXX/IX architecture decisions into an actual allocation path, not just infrastructure and diagnostics;
  - reduce the need to fall back to PBQP/MIR2 on cases that are already conceptually solved.

The strongest conclusion from both notes and code is this:

1. The architecture is ahead of the implementation.
2. The implementation already contains most of the scaffolding for the next quality jump.
3. The most obvious improvements are not speculative research items. They are mostly "wire the thing that is already half-present".

## Overview

### What the project itself says is important

`CLAUDE.md` is explicit that current work is about codegen quality, register allocation correctness, and backend convergence, not frontend expansion.

- Register allocator and loop codegen bugs are still marked as known blockers, especially live-range corruption in loops and stale `HL` behavior in multi-expression contexts. See `CLAUDE.md:9-14`.
- Iterator fusion is semantically correct, but still bottlenecked by memory-backed allocation, with a stated gap of roughly `~207T actual vs ~43T ideal` per element. See `CLAUDE.md:16-23`.
- MIR2 still carries open production bugs, including the blocking arena/struct-method issue (`BUG-008`) and spill/adapter quality issues. See `CLAUDE.md:25-34`.
- LIR is described as an important architecture milestone, but its "remaining" list is exactly the same type of work now emphasized by the latest VIR notes: EXX/shadow usage, stronger combine rules, and default-path promotion. See `CLAUDE.md:36-52`.
- VIR is framed as the active quality backend: O(1) regalloc, PFCCO, CFG-aware solving, inline runtime, IX-half spill, OpAsmBlock integration. See `CLAUDE.md:54-99`.

The subtext is clear: the repo has already chosen its direction. The backlog is mostly about finishing the consequences of that choice.

### What the freshest contexts add

Recent contexts sharpen that picture and make the next steps even more concrete.

- `contexts/2026-04-01-18-reg-model.md` settles the 18-register architecture decisions:
  - IX halves should be cheap (`LocCost=1`, not `8`);
  - EXX should be modeled as zones, not as a flat 18-loc explosion;
  - IXH/IXL/IYH/IYL are not just spill slots but cross-zone bridge channels;
  - the pragmatic EXX path is `S1 + S2 + IX bridges`, not an immediate giant joint table.
- `contexts/2026-04-02-next-session-seed.md` is even more actionable:
  - arithmetic idioms are already committed but need testing/merging;
  - MIR2 DAG inliner exists and is expected to close a meaningful chunk of the Che gap;
  - P3/P4/P5 are explicitly listed: IYL loop counter, LICM/embedded globals, EXX-zone wiring into lookup;
  - 11-loc enriched tables are available now, not hypothetical.
- `contexts/2026-03-30-next-session-seed.md` shows a second, equally concrete backlog:
  - `intrinsics.go` still needs widening multiply, saturating arithmetic, abs/sign/min/max, and u32 operation wiring;
  - this is not design work anymore, just table-driven lowering work.

So the repo-level direction is stable:

- VIR is the production-quality path.
- GPU tables are the chosen mechanism for small/constant arithmetic and enriched O(1) allocation.
- EXX is the next structural step after IX-expanded enriched lookup.
- MIR2 should be treated as legacy debt except where it is still the fallback or the source of open correctness bugs.

## What Is Already Real in Code

### 1. VIR already has a layered O(1) allocation entry path

The actual code in `minzc/pkg/vir/pipeline.go` confirms that VIR first tries table-based allocation, then cut-vertex decomposition, and only afterwards leans on heavier solving.

- `CodegenFunc` first builds `allOps`, then calls `GetRegAllocTable().Lookup(...)`. See `minzc/pkg/vir/pipeline.go:526-545`.
- If direct table lookup misses, it immediately tries cut-vertex decomposition via `tryCutVertexDecompose(...)`. See `minzc/pkg/vir/pipeline.go:548-566`.
- Only after those fast paths does it move into timeout-adjusted solver territory. See `minzc/pkg/vir/pipeline.go:569-595`.

This is important because it means the repo is already structurally prepared for an "O(1) first, solver later" strategy. The missing work is mostly about coverage and successful emission, not architectural inversion.

### 2. Regalloc tables already support multiple lookup modes

`minzc/pkg/vir/regalloc_table.go` is more mature than the notes alone suggest.

- `Lookup(...)` already tries:
  - legacy signature;
  - exhaustive GPU signature;
  - enriched `shapeHash:opBagHash`;
  - binary Z80T table lookup by enumeration index.
  See `minzc/pkg/vir/regalloc_table.go:68-118`.
- The table loader already attempts to load:
  - 4v binary tables;
  - 5v merged IX-expanded tables;
  - 6v dense IX-expanded tables.
  See `minzc/pkg/vir/regalloc_table.go:260-298`.

That means the project is not blocked on "supporting IX-expanded tables". It already supports loading them. The real issue is whether the rest of the pipeline exploits that capability deeply enough and whether coverage-sensitive paths are routed through it.

### 3. Cut-vertex and bipartite graph analysis already exist

The EXX-related graph infrastructure is also already present.

- `FindCutVertices()` is implemented with Tarjan articulation-point logic. See `minzc/pkg/vir/regalloc_table.go:707-778`.
- `IsBipartite()` is implemented and documented explicitly as the basis for EXX dual-bank scheduling. See `minzc/pkg/vir/regalloc_table.go:780-822`.
- `DecomposeAtCutVertices()` also exists. See `minzc/pkg/vir/regalloc_table.go:824+`.

And `pipeline.go` uses this partially:

- `tryCutVertexDecompose()` is real and operational. See `minzc/pkg/vir/pipeline.go:1172-1308`.
- When decomposition fails, the code still computes bipartite structure and prints diagnostics, including partition sizes. See `minzc/pkg/vir/pipeline.go:1188-1219`.

This is one of the clearest "obvious improvements looking at you" in the repository:

- EXX is already modeled conceptually.
- Bipartite feasibility is already computed.
- Partition statistics are already emitted.
- But the result is still mostly diagnostic, not a first-class allocation/execution mode.

### 4. MIR2-level inliner is implemented

The MIR2 DAG inliner is not a placeholder. It is present and reasonably complete.

- `InlineMIR2(...)` identifies inlineable non-recursive DAG callees with bounded VIR op count. See `minzc/pkg/vir/inliner.go:21-94`.
- It clones callee CFG, remaps vregs, rewrites returns to jumps, and splices blocks into the caller. See `minzc/pkg/vir/inliner.go:96-243`.

Given the fresh session notes that specifically call out `--vir-inline` and its expected impact on the Che decoder, this is not speculative optimization territory. It is active code that likely needs stabilization, validation, and broader enablement.

### 5. Constant div/mod intrinsics are truly wired

The bridge path confirms that at least part of the intrinsic architecture is live:

- `bridge.go` explicitly routes constant 8-bit division through `tryConstDiv(...)`. See `minzc/pkg/vir/bridge.go:377-383`.
- It routes constant modulo through `tryConstMod(...)`. See `minzc/pkg/vir/bridge.go:408-414`.
- `intrinsics.go` implements `tryConstDiv(...)` as an `OpAsmBlock` with explicit clobbers and `A`-based ABI hints. See `minzc/pkg/vir/intrinsics.go:23-90`.
- `tryConstMod(...)` currently handles only the power-of-two fast case. See `minzc/pkg/vir/intrinsics.go:92-118`.

This proves the intended pattern is already accepted by the backend:

- GPU table lookup;
- `OpAsmBlock` emission;
- surrounding register allocation handled by VIR.

That pattern can be replicated across more arithmetic families with relatively low conceptual risk.

## What Still Looks Incomplete or Underexploited

### 1. `intrinsics.go` is conceptually ambitious but practically narrow

The file header says the intrinsic layer is about:

- `mul8`
- `mul16`
- `div8`
- `u32`

See `minzc/pkg/vir/intrinsics.go:1-14`.

But the actual implemented helpers are only:

- `tryConstDiv`
- `tryConstMod`

See `minzc/pkg/vir/intrinsics.go:23-118`.

And the stats struct only counts:

- `Mul8Inline`
- `Div8Inline`
- `Mod8Inline`

See `minzc/pkg/vir/intrinsics.go:120-125`.

This is a visible mismatch between architectural intention and actual lowering coverage.

Why this matters:

- the GPU table ecosystem in notes is much richer than what the intrinsic layer currently exposes;
- many of the remaining codegen quality gains are exactly in this category;
- the implementation pattern is already proven by `tryConstDiv`.

This is probably the single cleanest "obvious next implementation" in the repo.

### 2. EXX is still more "prepared" than "used"

The repository has all of the following:

- a decided EXX-zone model in context notes;
- `IsBipartite()` in code;
- diagnostics that compute partition sizes;
- a 5-level allocator story in docs that explicitly names EXX at L2.

But the actual fast path in `pipeline.go` still does:

- direct table lookup;
- cut-vertex decomposition;
- then fallback to solver logic.

The bipartite result is not yet used to produce a concrete banked allocation and corresponding emit mode. The code around `tryCutVertexDecompose()` treats bipartiteness as information, not as a completed path. See `minzc/pkg/vir/pipeline.go:1188-1219`.

This gap is especially notable because the context notes from 2026-04-01 and 2026-04-02 already define the preferred pragmatic solution:

- do not wait for a giant 18-loc table;
- split into two subproblems;
- use IX halves as bridges;
- charge explicit EXX/bridge costs.

In other words, the design debt is paid. The integration debt remains.

### 3. There is still too much "runtime inline after emit" instead of "intrinsic at lowering"

`pipeline.go` contains a substantial post-emit runtime inliner:

- tail-call `CALL+RET -> JP`;
- constant `div8` inline from tables;
- special `div10` and `mod10`;
- generic inline `__div8`, `__mod8`, `__mul8`;
- `mul16` constant specialization;
- generic inline `__div16`, `__mod16`, `__mul16`.

See `minzc/pkg/vir/pipeline.go:2790-3135`.

This is useful, but it also signals a layering smell:

- some arithmetic quality currently happens after assembly emission, by pattern-reading text lines;
- other arithmetic quality happens earlier, in `bridge.go` via `OpAsmBlock`.

The latter is strategically cleaner because:

- register allocation sees the clobbers and live ranges directly;
- ABI is explicit;
- there is less textual peephole fragility.

So one obvious improvement is to migrate more of the current post-emit arithmetic magic into earlier intrinsic lowering.

### 4. MIR2 legacy codegen is still a large correctness and maintenance sink

The grep of `minzc/pkg/mir2/z80codegen.go` shows many TODO-marked cases across arithmetic widths, comparisons, shifts, mul/div variants, extensions, and corner-case codegen paths.

Examples include:

- TODO-marked 8-bit and 16-bit binops;
- TODO-marked 32-bit arithmetic and shifts;
- variable-multiply TODOs;
- sign-fixup TODOs for division;
- compare and extension TODOs.

The exact list is long, but the pattern is stable: the MIR2 backend is still monolithic and incomplete in many local corners.

This aligns with `CLAUDE.md`, where MIR2 is still the home of several open bugs and allocator/loop issues. See `CLAUDE.md:25-34`.

The important strategic point is not "fix every TODO in MIR2". It is:

- identify which MIR2 issues still matter because VIR/PBQP fallback can hit them;
- stop investing in MIR2 quality where VIR should replace the path entirely;
- keep MIR2 effort constrained to correctness blockers and fallback safety.

### 5. ABI/fallback boundaries still deserve pressure

`pipeline.go` still has important PBQP fallback logic:

- if VIR codegen fails and PBQP allocation is available, the module can fall back to PBQP-generated assembly per function;
- this is explicitly used as a safety net.

See `minzc/pkg/vir/pipeline.go:169-190`.

This is good operationally, but it also means:

- every solver miss is still a quality cliff;
- every unsupported allocation/emission corner in VIR can route the user into an older backend path;
- residual MIR2/PBQP weaknesses still have real user impact.

So one of the best quality multipliers is not just "better assembly when VIR succeeds", but "fewer cases where VIR gives up".

## The Most Obvious Improvements Looking At the Repo Right Now

### A. Finish the arithmetic intrinsic expansion in VIR

This is the lowest-risk, highest-clarity next wave.

Why it is obvious:

- notes explicitly list the remaining families;
- `intrinsics.go` already establishes the pattern;
- `bridge.go` already calls that layer;
- GPU tables are already produced.

Most likely additions:

- constant `mul8` through intrinsic lowering, not only post-emit inline;
- constant `mul16` through intrinsic lowering;
- non-power-of-two `mod8` table-backed lowering where available;
- widening multiply;
- `sat_add8`, `sat_sub8`;
- `abs8`, `sign8`, `abs16`, `neg16`, `min/max`;
- table-backed `u32` operation lowering.

This work is attractive because it compounds:

- better code size;
- better T-state quality;
- less runtime helper dependence;
- fewer text-level peephole dependencies.

### B. Promote EXX from analysis to real allocation mode

This is the highest-leverage structural improvement.

Why it is obvious:

- context notes already made the architectural decision;
- graph infrastructure already exists;
- enriched table infrastructure already exists;
- current code already diagnoses candidate cases.

What "done enough" likely looks like:

- when direct table lookup fails and graph is bipartite, run a zone-aware allocation path;
- allocate each side independently against existing lookup machinery;
- reserve IX-half bridge channels for cross-zone live values;
- emit explicit EXX transitions at zone boundaries;
- use simple explicit cost model:
  - table cost left + table cost right + bridge penalty + EXX penalty.

This would close the current mismatch where EXX is a first-class concept in docs and notes but only a second-class concept in the actual pipeline.

### C. Shift more runtime specialization earlier into VIR lowering

The arithmetic post-pass in `pipeline.go` is effective, but it also means optimization happens after register choices have already been constrained by less-informative IR.

The more of that logic moves into:

- `bridge.go`
- `intrinsics.go`
- maybe ISLE combine rules

the better the solver can reason about it.

Concrete candidates:

- `mul16` constant sequences;
- 16-bit div/mod inline bodies;
- selected `u32` ops now handled late;
- `div10`/`mod10` special casing if representable as intrinsic templates.

### D. Stabilize and operationalize the MIR2 DAG inliner

This one is less fundamental than EXX but probably faster to convert into visible wins.

Why it is obvious:

- the code exists;
- the notes name it explicitly as a current gap closer;
- loop-heavy workloads are central to current performance concerns.

Most valuable next steps:

- verify safety on representative corpus cases;
- measure interaction with enriched-table hit rates;
- ensure inlining helps rather than worsening live-range pressure in unlucky cases;
- add reporting so it is visible when an inline transformed a function and what it cost/benefited.

### E. Be deliberate about MIR2 debt: correctness-only, not prestige work

The repo still has a lot of MIR2 codegen TODO surface area. That is not automatically a good place to spend time.

The best MIR2 work now is probably only:

- correctness blockers that still hit through fallback;
- known open bugs listed in `CLAUDE.md`;
- places where MIR2 is still required by a live workflow or test harness.

The wrong move would be to keep polishing MIR2 as if it were still the strategic backend.

## Inferred State of the Roadmap

This section is inferred from the combination of:

- `CLAUDE.md`
- 2026-03-30 through 2026-04-02 contexts
- current code structure

It is not a restatement of an explicit single project plan. It is the roadmap that the repository appears to already imply.

### Inferred short-term roadmap

1. Raise the fraction of functions solved by table-driven VIR paths.
2. Expand arithmetic intrinsic coverage using already-produced GPU tables.
3. Validate and merge currently prepared changes around arithmetic idioms and MIR2 inlining.
4. Reduce quality loss from fallback boundaries.
5. Close the "Che gap" with concrete, measurable T-state wins.

### Inferred medium-term roadmap

1. Implement pragmatic EXX zone allocation.
2. Exploit IX halves not merely as slow spill but as durable bridge channels.
3. Push more arithmetic and addressing combine rules into earlier VIR stages.
4. Reduce post-emit textual optimization dependence.
5. Make VIR the unquestioned default path not just semantically, but also operationally.

### Inferred long-term roadmap

1. Joint or near-joint instruction selection plus allocation quality beyond current split phases.
2. Better cross-call register preference tracking and EX/EXX elimination.
3. Wider use of GPU/offline exhaustive search as the source of real instruction templates, not only peephole fragments.
4. Further shrinking of the solver-only surface area.

## Suggested / Proposed Roadmap

This is the proposed version of the roadmap, based on what looks most pragmatic from the current repo state.

### Phase 1: Fast concrete wins

Goal: increase visible codegen quality without architectural churn.

- Extend `intrinsics.go` from `div8/mod8` into:
  - const `mul8`
  - const `mul16`
  - `sat_add8`, `sat_sub8`
  - `abs/sign/min/max`
  - selected `u32` ops
- Add basic instrumentation:
  - how often each intrinsic fires
  - bytes/T-state deltas where available
  - how often post-emit fallback patterns still trigger
- Run focused regression and corpus sampling around arithmetic-heavy demos.

Why first:

- the implementation pattern already exists;
- the table assets already exist;
- this work is easy to evaluate.

### Phase 2: Reduce fallback fragility

Goal: keep more functions inside the preferred VIR path.

- Audit all current VIR failure reasons that trigger PBQP fallback.
- Separate:
  - solver unsat due to real constraints;
  - emit-time limitations;
  - ABI validation failures;
  - avoidable missing move/order patterns.
- Fix the easiest emit-time and ABI cases first.

Why second:

- each resolved fallback gives compounding value;
- otherwise new VIR quality work benefits only the functions that already succeed.

### Phase 3: EXX pragmatic activation

Goal: cash in the architecture work already done in notes and scaffolding.

- Implement a true bipartite/zone-aware path after direct table miss.
- Use current `IsBipartite()` result as the trigger.
- Solve the two sides independently with existing table machinery.
- Reserve IX-half bridge channels explicitly.
- Emit EXX transition plan in a minimal, explicit manner.

Success metric:

- more 7-12 vreg functions handled without Z3/PBQP;
- better code than memory-backed fallback;
- measurable hit on loop-heavy kernels and Che-style workloads.

### Phase 4: Move quality left

Goal: perform more optimization before textual assembly exists.

- migrate arithmetic specialization from post-emit string rewriting into:
  - `bridge.go`
  - `intrinsics.go`
  - ISLE combine rules
- retain the current post-emit pass only as a safety net or for a shrinking residue.

Why:

- earlier passes are easier to reason about;
- solver sees the real clobbers and widths;
- code becomes less brittle.

### Phase 5: MIR2 triage, not expansion

Goal: control legacy debt while VIR takes over.

- fix only MIR2 correctness bugs that still affect real users or fallbacks;
- avoid broad new investment in MIR2 feature-quality work unless a path still depends on it;
- keep MIR2 readable and safe as fallback, not aspirational as the primary optimizer.

## Detailed Findings

### Finding 1: The repo already behaves like a "table-first" compiler

This is not just rhetoric in docs. The pipeline starts with table lookup and cut-vertex decomposition before solver-heavy work. See:

- `minzc/pkg/vir/pipeline.go:526-566`
- `minzc/pkg/vir/regalloc_table.go:68-118`

The obvious implication is that the next wins should improve table applicability, not bypass it.

### Finding 2: IX-expanded table support is already in production code

The loader already looks for merged 5v IX tables and 6v dense IX-expanded tables. See:

- `minzc/pkg/vir/regalloc_table.go:260-298`
- `minzc/pkg/vir/enriched_reader.go:1-18`

So any future statement like "we should support IX-expanded enriched tables" is outdated. The real question is: are we exploiting them fully enough in pipeline routing and cost modeling?

### Finding 3: EXX is the clearest missing integration

The codebase already contains:

- bipartite graph analysis;
- documentation that names EXX as allocator level 2;
- context notes defining the actual pragmatic implementation;
- diagnostic partition-size reporting in the live pipeline.

That combination almost always means the next engineering step is straightforwardly "finish the integration".

### Finding 4: Arithmetic intrinsic coverage is the cleanest near-term quality gap

The gap between:

- what `intrinsics.go` says the system is for, and
- what it actually lowers today

is probably the most actionable codegen discrepancy in the repository.

The bridge layer already trusts this mechanism. The next logical step is simply to widen the family set.

### Finding 5: MIR2 should be treated as fallback debt, not the center of gravity

The repository still carries serious MIR2 open bugs, and the file surface suggests ongoing local incompleteness. But the strategic energy in both notes and code has already moved to VIR.

This should inform prioritization:

- use MIR2 work to protect correctness;
- use VIR work to improve quality.

## Risks and Cautions

### 1. EXX can become a research sink if over-generalized too early

The context notes are wise here: avoid the flat 18-loc dream first. The zone split is the right pragmatic move.

### 2. More inlining can accidentally worsen reg pressure

The MIR2 inliner is promising, but it needs measurement. Some wins may turn into table misses or harder allocation unless bounded carefully.

### 3. Post-emit and pre-emit optimizations can diverge semantically

If the project grows both:

- textual inline/rewrite logic, and
- intrinsic-lowering logic

for the same arithmetic family, they can drift. The migration should reduce duplication, not create two permanent implementations.

### 4. PBQP fallback hides strategic pain

Fallback is operationally useful, but it can mask where VIR still leaks. That makes instrumentation important.

## Recommended Next Actions

If the goal is "best next engineering sequence", I would do this:

1. Expand `intrinsics.go` coverage first.
2. Add counters/reporting for intrinsic fires and fallback causes.
3. Validate/enable MIR2 DAG inlining on targeted cases.
4. Implement the pragmatic EXX bipartite path.
5. Shrink post-emit arithmetic specialization once earlier intrinsic lowering covers the same families.

## Final Take

The repository is in an unusually good position for codegen improvement.

Not because everything is finished, but because the hard conceptual decisions are mostly already made:

- table-first allocation;
- GPU-discovered arithmetic;
- VIR as the quality backend;
- IX halves as cheap, useful locations;
- EXX as a zone model, not a fantasy flat register file.

What remains is mostly execution:

- widen the intrinsic surface;
- activate EXX for real;
- keep more functions on the VIR happy path;
- spend MIR2 effort only where fallback correctness still demands it.

That is why the most obvious improvements "looking at me from the repo" are not vague:

- arithmetic intrinsic expansion,
- EXX path activation,
- inliner stabilization,
- fallback reduction.

These are the places where the code, the notes, and the architecture all point in the same direction.
