# Mixed copy + weighted checksum: bounded contract review

Read-only review of `/tmp/minz-grace-pgo-resume`, 2026-10-11. No production files changed. Reviewed `forward_copy_profile.go`, `grace_weighted_composition.go`, `profile_invariant_sites.go`, `profile_forward_copy.go`, `profile_policy.go` and `/tmp/minz-forward-copy-next-plan.md`.

The proposed three dimensions / eight candidates are appropriate for one observed exact copy site and one observed exact weighted checksum site. The checksum contributes two independent bits (scalar hoist and pointer loop), despite having one source SiteID. Constant-if must remain false throughout this first mixed contract. Four total dimensions / sixteen candidates is the existing practical cap, not four unique IDs.

## Minimum safe application

Add a distinct mixed application helper accepting original compatibility plus three typed maps: copy, scalar invariant, and indexed pointer choices. Do not call the two existing application helpers sequentially. `ApplyForwardCopySiteChoices` requires fresh input and seeds lineage, then overwrites all policies. `ApplyProfileDecisions` seeds lineage and runs Grace immediately. Chaining fails freshness or applies the first choice before the other choices are installed. Choosing the two separate winners and compiling their combination also lacks caller-level measured evidence.

The mixed helper should:

1. Reject any existing lineage or any of the four policy maps. Verify original source/typed-HIR/dependency/context compatibility before mutation.
2. Discover all original legal copy sites, scalar sites, pointer opportunities and original exact weighted overlap sites from the same source map. Require every requested key to belong to its corresponding legal set. Copy must be disjoint from scalar and indexed-pointer sites; only exact weighted source is an allowed scalar/pointer intersection. Unknown false keys should be refused too.
3. Seed lineage once. Copy all caller-supplied maps into owned maps, including explicit empty maps. Set `ForwardCopyPolicy`, `InvariantBasePolicy`, `PointerWalkPolicy` together. Set `GraceRulePolicy` exactly to constant-if false, invariant-base true, guarded-pointer-walk true and guarded-forward-copy true. Empty per-site maps, not nil, disable unrequested sites.
4. Run `RunGraceWithOptions` only after all maps are installed. This is one application phase using its existing rewrite-to-fixpoint rounds, not literally one rewrite. Both weighted scalar and pointer need separate rule firings. Reject incomplete realization rather than pretending a budget-limited result is the requested candidate.

Original unobserved copy and weighted sites remain disabled and must still be structurally checked as original. Observations select dimensions; they do not establish legality. A zero-trip invocation has a positive loop entry and is observed even though body count is zero. Replayed original counts/guards must match the supplied observations before candidate execution.

## Validation and attribution pitfalls

Reuse `ValidateForwardCopyProof` and `ValidateWeightedComposition` directly on source versus candidate before and after compilation. They now check full function metadata, globals/target, seed/index/return, exact address guard, ordered reads/stores and updates, fresh names, `MutateInPlace=false`, exact fallback, and precise lineage.

The copy selector's existing local validator cannot be reused unchanged: its global lineage scan rejects every non-copy rule. The weighted normalizer cannot be reused blindly either: it ultimately delegates to `jointSourceSitesFullyRealized`, whose treatment of unknown rules does not establish the copy contract. A mixed validator must first require a copy proof for each original legal copy site and a weighted proof for each original weighted overlap group. Then it may filter the already-proved copy records out of the ordinary scalar/pointer realization input, normalize only independently proved weighted compound records, and run the ordinary remaining-additions check for other scalar sites.

For each enabled copy SiteID require exactly one fast and one fallback record with `ForwardCopyRule`, empty PreviousRule, and the original function/site attribution. Disabled copy IDs require the original loop record and no copy records. For the weighted both-enabled case, accept `PreviousRule=hir-invariant-base` only on the fast/fallback records already checked by the weighted proof; do not erase arbitrary PreviousRule values or synthesize a missing hoist marker. Preserve count uniqueness and reject unknown/unrequested rule records. LoopLineage retains stale map entries after replacements; enumerate reachable records via `CurrentLoopLineage`, not every key in the map.

Validate before compilation and again on the surviving HIR afterward: CompileHIRSteps can run Grace again and split functions. Preserve both before-compile proof records and postcompile records rather than treating an earlier marker as sufficient. If a split prevents exact structural validation, refuse the candidate until that transformation has an explicit proof mapping; do not weaken the checker.

## Measurement contract

Preflight all eight independent factory modules plus the untouched oracle module before any mutation. Recheck all retained modules for compatibility and all policies after factory construction. Own the workload/maps. Run the original complete caller once to establish persistent return/snapshot/exact-byte-access expectations. For every candidate compare raw VM, postcompile raw VM and actual assembled Z80 against the same oracle. Sum invocation T-states, not just the selected kernels; module bytes include every compiled function. Check overflow and retain the exact assembled binary and matching target binary SHA256. Winner ties stay at all-off baseline. Size mode must still measure both enabled guards rather than suppressing the requested candidate through the conservative general Osize policy.

Use a new typed mixed decision/evidence schema: distinguish forward-copy, scalar hoist and indexed pointer choices. Do not fabricate old `pointer-walk` ProfileDecision entries for copy. Store requested maps, all candidate proofs, costs, source compatibility, target/binary hashes, full caller evidence, selected index, exact winner binary and a Go-serialized self-excluding SHA256.

## Necessary bounded checks

Eight combinations on the persistent typed intern/checksum caller, not standalone winners; independent reference for hit/reuse/miss allocation and checksum. Include all-off baseline, zero-trip calls, lengths1/5/16/64/255, repeated hits without arena mutation, equal-length mismatch at first/middle/last byte, different lengths/prefixes, wrapped checksum weights and counters. Preserve exact per-call traffic and all declared resource snapshots. Keep the already-proved overlapping copy corpus as separate source/backend coverage.

Add direct refusal checks for policy preseed, shared/mutated factory data, unknown and mismatched-kind keys, copied SiteID forged onto a weighted loop, missing or forged PreviousRule, extra copy fast/fallback record, incomplete weighted hoist, and cap counting two weighted bits. Other rules must remain disabled even on unobserved functions. Existing helper checks should be reused without broadening their acceptance.

No new optimization proof is required for the interning search or equality loop in this first mixed slice; they execute as original code. Baseline wrong-code or target unsupported behavior must be fixed or reported before performance claims. The typed intern subsystem does not prove the unchanged extern tokenizer works.
