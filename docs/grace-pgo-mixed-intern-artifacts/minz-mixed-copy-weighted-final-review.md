# Independent bounded mixed-copy/weighted review

Reviewed `/tmp/minz-forward-copy/minzc/pkg/hir/mixed_copy_weighted.go` and `pkg/pipeline/profile_mixed_copy_weighted.go` read-only. No compiler edits. Repro: `/tmp/minz-mixed-lineage-review.go`; output: `/tmp/minz-mixed-lineage-review.txt`.

## Confirmed public proof contract holes

All following inputs pass HIR Verify and currently return nil from ValidateMixedCopyWeightedProof:

1. Parse exact copy kernel plus ordinary `i=0; while i<n {i=i+1}; return i`, seed actual lineage, then move the ordinary WhileStmt's lineage marker onto its ReturnStmt. The real loop is unattributed; return impersonates expected loop. CurrentLoopLineage enumerates marked statements, not exclusively loops, and the validator checks multiplicities only.
2. Parse two ordinary loops in the same function and swap their original lineage values. HIR remains unchanged and all expected `(SiteID,function,role,rule,previous)` counts remain one, but attribution points at the wrong source site.
3. Delete one ordinary loop's SourceLocations entry in both source and actual before seeding. Source BuildLoopSiteMap reports UnmappedLoops=1; public validator nevertheless accepts because this loop never enters expected counts. The pipeline LoadLoopProfile already rejects this source, so this is a public proof boundary hole, not an identified measured-pipeline bypass.

These demonstrate invalid attribution acceptance, not semantic wrong-code. Unrelated-function structural equality now correctly blocks modifying/adding ordinary loops or their effects. The exact copy and weighted kernel proofs check actual loop structure and markers directly and prevent these manipulations for specialized loops.

Minimum hardening: reject unmapped source loops at public mixed API boundary; require markers only on actual While/ForRange/ForEach statements and require every actual loop to be marked; for unchanged nonspecial functions bind each source site to the same actual HIRPath, not solely aggregate counts. Keep specialized fast/fallback structural matching from existing validators.

## Other reviewed boundaries

Source-kind maps reject unknown and mismatched keys including false keys. Copy and weighted eligible IDs are disjoint; weighted scalar/pointer bits deliberately overlap only within weighted proof. Dimensions count copy+2*weighted, cap 1..4, at most 16 measured candidates. All legal sites are checked, including disabled/unobserved. All policies are owned and installed together before one lineage seed; constant-if and unrelated source-site transformations remain disabled. Current unrelated function and module definition comparisons close the previously noted unrequested rewrite hole.

Factory preflights all candidates plus source before rewriting and rechecks source identity/clean policy after all fresh calls. Controlled replay matches observations/counts/guards/completed calls. Each candidate uses independent structural proof before and after compilation, full raw oracle before and after compilation, and full caller target oracle. Module assembly digest is matched to target evidence digest; cost sums validate nonnegative values/overflow and retained exact binary is selected by strict improvement, preserving baseline ties. No additional material hole found in these bounded checks.

## Fixed-validator verification

Reran the same bounded reproducer after the isolated implementation fix. Saved pre-fix output separately as `/tmp/minz-mixed-lineage-review-before.txt`; fixed output is `/tmp/minz-mixed-lineage-review-after.txt`.

Clean original proof still succeeds. All three invalid cases now reject: misplaced return marker (`loop attribution attached to wrong node or missing`), unmapped source (`unmapped source loops`), and swapped ordinary loop IDs (`original loop path attribution changed`). Read-only inspection confirms actual loop/marker coverage plus unchanged ordinary-function source path binding, while specialized validators retain direct shape/marker matching. No further material hole identified within the requested scope. No compiler edits or broad test runs.
