# Measured closed-host append selector: bounded next slice

Authoritative parent base: 57322d1779300d2a29ddd5030a2351d64c9e4ced.
Original source remains examples/nanz/self_lanz_parser.nanz, unchanged.
Prototype append hoist and its independent stronger proof are separate pending
inputs; this branch contains design only and changes no compiler or API.

## Public boundary and evidence

Add a separate `MeasureMemoryHostAppendCopyHoist(factory, workload, serialized,
MemoryHostAppendSelectorOptions{Goal, MaxSites})` API. MaxSites must equal 1;
goal must be speed or size. Reuse the existing strict closed loop-profile loader
and its distinct schema, limits and full replay. New result schema:
`minz.memory-host-append-selection/v1`. Exactly two candidates are recorded:
original and privately constructed append-hoisted. Caller supplies originals
only; no transformed factory, external ASM or precomputed cost is accepted.
Neither arena composition nor a general multi-site selector is part of this slice.

Choose the exact original source site `intern_region/body/s5`, kind while,
ordinal 0, complete origin. The same function also has the unrelated search loop
at body/s1. A function-name wildcard is insufficient and would refuse a valid
source or select the wrong loop. Require precisely one matching mapped append
site and positive Entries. Zero BodyEntries is permitted for an observed zero
length loop. The original SelfLanz workload has append Entries=13,
BodyEntries=286; these values are historical observed data, never a selector
threshold or manufactured fixture expectation. Other sites can remain unobserved.

Use a new typed `MemoryHostAppendPrecompileProof` with schema
`minz.exact-memory-host-append-hoist-precompile/v1` containing function,
SourceSite, OriginalCompatibility, ActualCompatibility, OriginalFunctions,
ActualFunctions, ActualSites. Contexts are the natural closed baseline contexts,
not the instrumented profile context. SourceSite must equal the exact loaded
site. The retained actual loop must have the same source ID and complete Origin
at `intern_region/body/s7`; unrelated search remains body/s1. ActualSites is
structural attribution only and contains no inherited counters. The equality of
ID does not establish transformed invocation counts or post-lowering lineage.

## Ownership and replay protocol

1. Validate bounded options and profile size before copying or invoking factory.
   Own workload and serialized bytes before the first callback.
2. For every original graph, run the existing shared-pointer/map/slice freshness
   gate and retain all pre-destructive pointer/backing allocations through all
   success and failure returns. KeepAlive is deferred; forcing GC must not turn
   retained addresses into false shared-AST rejection or permit actual sharing.
3. Load against captured original source, then capture full FunctionSiteMap,
   including exact function/path source origins. Strict JSON decoding remains
   duplicate/unknown/omitted/case-alias/trailing-data refusing and uses UseNumber.
4. Re-observe through RunZ80MemoryHostLoopProfile on fresh originals. Compare
   the complete loaded profile to fresh closed observations and all 11 site
   counters, not just append totals. The existing association's exact six-call
   source lifecycle and sole prepared candidate exclusion remain explicit; do
   not waive identity for every graph which changes. Check all retained originals
   after callbacks and final sentinels; no source or origin drift may be hidden
   behind matching newer factory results.
5. Construct each candidate privately from a separate fresh original plus a
   fresh mutable candidate. Check both source identities before transformation,
   retain their original graphs, invoke only exact append hoist, then invoke
   `hir.ValidateMemoryHostAppendCopyHoist` before compiler preparation.
6. Build the typed proof from independently validated snapshots. Every private
   variant factory invocation must produce an identical record. Preserve known
   prepared graphs separately from untouched original sentinels.
7. Run the ordinary closed baseline on that private variant factory. Original
   measurement is association.Baseline. Both raw and native candidate evidence
   must equal original semantic observations: returns, all globals/regions,
   exact ordered successful byte events and typed first denial. Only instruction
   Steps and wrapper diagnostic error text may differ, after failure presence
   and typed failure equality. Infrastructure/gas failure returns nil evidence.
8. Bind proof OriginalCompatibility/OriginalFunctions to measured original
   baseline; ActualCompatibility/ActualFunctions to measured variant baseline.
   Bind SourceSite to loaded source and actual same-origin append site to s7.
   MIR/allocation/native sidecar dumps remain diagnostics/binary binding, not a
   fabricated postcompile HIR structural proof. Recheck all retained originals.

## Hoist legality prerequisite

The independent precompile validator establishes the exact whole function and
module delta. Header poke precedes both base declarations. Original j:u8=0,
loop control, final bump, search and return are preserved. Two u16 sums become
`src_buf+start` and `str_bump+1`; subsequent additions remain modulo 65536. No
nowrap guard or pointer induction is introduced. Reads continue to observe every
prior byte store for aliases and propagation; no snapshot/memmove semantics.

A material review found that PrepareMemoryHostWorkload alone does not establish
resource/global exclusion. The stronger append proof must independently verify
raw MIR and actual raw VM module allocation extent for both snapshots and refuse
all overlapping declared regions. Closed raw runtime and native runtime then
independently enforce their own actual prepared allocations, globals, spills,
code, stack and return protection. No arbitrary host purity is assumed. Hoisting
changes compiler-global read traffic; the contract covers ordered host-resource
byte events and final globals, not internal global-load count or gas equivalence.

## Profitability and scope

Compile both goals' candidates under identical compiler options; goal affects
selection only. Speed strictly compares actual total T-states; size strictly
compares actual binary lengths. Equal primary costs retain original without
secondary tie-breaking. No affine fit, counter extrapolation or predicted costs.
Retain all candidate evidence including slower/larger cases.

Existing standalone prototype actual original30 results are 1221494T/2091B
original versus 1150124T/2076B hoisted. Thus both objectives are promising, but
public API/CLI decisions must use their own freshly measured actual evidence and
not hardcode these numbers. Existing arena choice remains a separate API and
must continue refusing extra unrelated changes and policies.

## Required narrow negative gates

- Correct append selected while search body/s1 remains unrelated; missing,
  unmapped, duplicate or unobserved append refused. Observed zero-length allowed.
- Options/oversize rejected before factory; duplicate/unknown JSON and consistent
  forged counters refused by fresh whole-profile replay.
- Stale source SHA, ABI/body, workload/hosts/layout/executable context and swapped
  equal-body origin coordinates refused; delayed factory mutation at association,
  variant construction and final sentinel stages refused.
- Real shared AST including candidate append body refused. Forced GC on fresh
  originals passes while unchanged freshness checks continue rejecting sharing.
- Exact hoist validator refuses header/order change, wide/reversed peek/poke,
  counter/search/bump/return changes, metadata/global/unrelated function changes,
  detached origins, generated binding collisions and region-over-VM-storage.
- Typed proof field mutation, wrong source/search site, wrong actual append path,
  origin or original/actual baseline function/compatibility bindings refused.
- Exact partial denial prefixes retained, but infrastructure failure cannot be
  accepted merely because both errors are nonempty. Ties retain original.
- Full unchanged original30 actual cases under both objectives, independent
  source host oracle plus typed ASM/BIN/allocation/MIR/sidecar audit. Reuse saved
  raw/native domain proof; do not rerun massive boundary corpus without changes.

## Production change boundaries

New selector/result/proof construction and focused tests should be separate
pipeline files. Import the frozen new HIR append proof and exact private helper
from their reviewed branch. Reuse existing pure graph-retention, observation
comparison and primary-cost helpers with conservative internal names if useful;
rename/extract only when necessary without changing arena behavior.

Do not introduce a generic callback-driven measurement engine now: two narrow
wrappers have different exact source identities, transformations and proofs, and
fresh-source lifecycle authority is security-sensitive. Such an extraction would
expand review and risk without changing measured profitability. CLI, if requested
separately, needs its own exclusive append mode and result schema; it must not
reuse arena decisions or fabricate ordinary ProfileDecision records.
