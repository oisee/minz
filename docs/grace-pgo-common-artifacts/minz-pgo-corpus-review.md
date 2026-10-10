**The original joint API does not preserve the unoptimized HIR choice. Concurrent edits now give the CLI a separate baseline that does.** Findings below reflect the latest files read; reproductions are source-derived, not executed. No edits made.

1. **P1 — `MeasureProfileVariantSets` still forces common HIR optimizations into “original.”**  
   [profile_joint.go:60](/tmp/minz-grace-pgo-resume/minzc/pkg/pipeline/profile_joint.go:60) passes `commonChoices=false`, leaving common rules enabled for every candidate. [ApplyProfileDecisions:183](/tmp/minz-grace-pgo-resume/minzc/pkg/hir/profile_policy.go:183) runs Grace regardless of `Options.UseHIRGrace`. Consequently, all-original means indexed loops **plus constant folding/invariant hoisting**; the unchanged HIR alternative is absent.

   **Reproduction:** profile `grace_hir_constant_if.nanz`, then call this API with `UseHIRGrace:false`. With no eligible pointer sites, it produces one candidate whose HIR has already selected the constant branches. Using `graceRegionSource` similarly introduces `__grace_base_*` into the sole original candidate. Cost evidence cannot establish improvement over ordinary compilation with HIR Grace disabled.

   **Current mitigation:** the newly added `MeasureGraceVariantSets` disables common rules for candidate zero, and [the CLI now calls it](/tmp/minz-grace-pgo-resume/minzc/cmd/minzc/hir_profile_auto.go:122). That addresses the CLI baseline path; the existing API retains the defect.

2. **P2 — Evidence records requested rewrites without checking that they happened.**  
   Grace defaults to [100 rewrites](/tmp/minz-grace-pgo-resume/minzc/pkg/hir/grace.go:50), prioritizing constant-if over pointer-walk. Application discards the rule result, while [candidate evidence retains the requested decisions](/tmp/minz-grace-pgo-resume/minzc/pkg/pipeline/profile_joint.go:200).

   **Reproduction:** place 100 removable `if false {}` statements before an eligible sum function; measure with `UseHIRGrace:false`. Common-enabled candidates exhaust the budget before pointer conversion, yet evidence still labels selected sites `pointer-walk`. Identical binaries/costs then appear to demonstrate variant equivalence.

   Record actual applied rules/sites and reject or explicitly identify unrealized choices. A related current coverage gap: `TestMeasuredInvariantGraceRule` uses scalar `a+b`, but [the hoist recognizer requires a pointer operand](/tmp/minz-grace-pgo-resume/minzc/pkg/hir/grace_hoist.go:134); its result-only assertions can pass without measuring any invariant rewrite.

3. **P2 — Later calls can erase incorrect memory effects from earlier invocations.**  
   [The target harness compares globals/resources only after all calls](/tmp/minz-grace-pgo-resume/minzc/pkg/pipeline/profile_workload.go:438). The raw oracle and `jointSameState` likewise retain only final memory.

   **Reproduction:** source has `global g:u8=0`, void `set(){g=1}`, and void `reset(){g=0}`; workload calls `set`, then `reset`. Substitute target assembly that makes `set` store 99. Both void results and final `g=0` agree, so the checks accept incorrect externally visible state when `set` returns.

   Capture and compare declared memory at each invocation boundary.

4. **P2 — Joint acceptance is weaker than exact-sum access validation.**  
   [Joint access checks](/tmp/minz-grace-pgo-resume/minzc/pkg/pipeline/profile_workload.go:426) establish bounds, not whether source-authorized accesses occurred. [Exact-sum measurement](/tmp/minz-grace-pgo-resume/minzc/pkg/pipeline/profile_measure.go:343) additionally checks input read counts.

   **Reproduction:** call `sum_array(resource,0)` with a one-byte resource. Insert `LD C,(HL)` at entry, preserving the compiled return behavior. The joint harness admits the unnecessary read because it falls inside declared readable storage; exact-sum measurement rejects zero-trip data reads.

   Add the existing sum access contract to eligible joint sum cases, or explicitly limit joint evidence to return values and final memory equality. It cannot substitute for the stronger access evidence.

The current invocation cost totals consistently exclude bootstrap cycles. I found no arithmetic mismatch there; the defects concern which transformations and observable behaviors those costs actually certify.