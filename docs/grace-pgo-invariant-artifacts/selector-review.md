# Scalar invariant site selector handoff

Implemented exclusively in new `minzc/pkg/pipeline/profile_invariant_sites.go` and `profile_invariant_sites_test.go`. No existing files, CLI, commits, agents, full suite, or expensive budget tests touched/run.

## Production API

`MeasureInvariantSiteVariantSets(factory func() (*hir.Module, error), observations []byte, workload hir.ProfileWorkload, options Options, maxSites int) (*InvariantSiteVariantSetResult, error)`.

Separate result/evidence schema `minz.invariant-site-variant-sets/v1`; existing module-rule API untouched. Result retains `Steps`, all-original pointer `Decisions`, exact selected `Binary`, and evidence. Evidence includes compatibility, compilation ID/options, workload oracle, `eligible_invariant_sites`, candidate `requested_site_policy`, explicit constant-if=false/invariant-base=true module policy, before/after HIR and original-site lineage, raw and postcompile raw semantics, actual Z80 invocation costs/evidence, module bytes, exact binaries and binary digests, selection and full evidence SHA256. Strict improvements only; baseline wins ties with baseline.

Discover original legal scalar sites using HIR `EligibleInvariantBaseSites`; intersect source proof with Entries>0, including zero trips. No pointer guards required. Sort choices, cap 1..4 requested maximum (0 observed sites remains valid); at most 16 candidates. No pointer choices are crossed; every original profile site receives an original pointer decision. Constant-if disabled in both initial and compile Grace passes, per-site invariant policy stays attached throughout.

Borrow existing unedited `profile_joint.go` helpers for fresh mutable node/storage rejection, compatibility equality, sorted counts/guards, raw controlled workload execution, and semantic state matching. Own workload via JSON clone. Preflight all source/candidate modules and recheck every retained input before any compilation; refuse existing policies/lineage, shared nodes/storage, non-equivalence, mutation of prior inputs. Strict profile load and actual workload counts/guards/completed-call recheck refuse forged/stale observations before candidates compile. Every candidate must realize exactly requested site hoists, contain no pointer rewrites, and agree with raw oracle before/after compile and under actual Z80 execution. Also call `RemainingInvariantBaseSites` before/after compilation: a lineage marker alone cannot hide incomplete selected-site rewriting caused by budget exhaustion.

HIR dependencies recorded in `/tmp/minz-site-selector-needs.txt`. Parent has now implemented `EligibleInvariantBaseSites` and `RemainingInvariantBaseSites`; builds succeed.

## Tests

One unchanged source with two scalar functions and an idle function supplies 0/1/2 observed loops. Both speed and size, lengths 0/1/5/255, zero-trip-only workloads, all requested subset policies, actual independent modulo-65536 Z80 oracle, raw semantic evidence, measured invocation totals, minimum selection, exact binaries/digests and evidence digest. Retained Steps replay compares to selected measured evidence. Separate four-function zero-trip workload measures all 16 candidates; five observed legal loops refuse immediately after first source factory call, before candidate allocation/compile. Refusal tests cover invalid caps, forged counts, stale source, reused module, shared child nodes, mutation of previous input HIR/policy, and every dirty policy/lineage input.

Validation command (specified Go 1.26.8 and cache):
`GOCACHE=/tmp/minz-grace-gocache /home/alice/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.8.linux-amd64/bin/go test ./pkg/pipeline -run '^TestInvariantSiteVariantSets' -count=1 -timeout=180s`
Passed, 3.538s. Added shared-child/prior-HIR mutation refusal cases afterward; final focused refusal command outcome recorded below.

## Concrete backend finding outside assigned files

An earlier test fixture with two loops in one function and early return between them exposed existing backend failures for partial hoisting. Exact source shape:
`fun scalar(a:u16,b:u16,n:u8,second:u8)->u16 { var s:u16=0; var i:u8=0; while i<n {s=s+(a+b);i=i+1}; if second==0 {return s}; var j:u8=0; while j<n {s=s+(a+b);j=j+1}; return s }`
At a=65535,b=2,n=1,second=1, candidate 1 (later lexical site selected) actual Z80 result 514 versus raw expected 2. API correctly refused candidate with target semantic mismatch. A six-parameter variant using c+d in the second loop also failed a partial-hoist candidate (4721 vs 4465). This is not a selector legality failure; no backend change made. Final success fixture uses two independent scalar functions. Parent's same-block policy tests remain responsible for straight-line same-block selection. Bool workload literals are not supported by existing Z80 workload harness; final fixtures only use u8/u16.

Final focused refusal validation: `.../go test ./pkg/pipeline -run '^TestInvariantSiteVariantSetsRefusals$' -count=1 -timeout=60s` passed after adding the shared-child and prior-HIR mutation regressions. The shared-child fixture retains a fresh source-location map keyed by shared original nodes, so refusal exercises shared AST detection rather than missing origins.
