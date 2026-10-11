# Finite arena + append composition

Design-only branch based on authoritative 57322d17. The append selector remains
pending integration; no parent compiler, source, test or API edits in this plan.
Actual SelfLanz source remains unchanged. Scope is two named exact transformations
and four measured whole-program candidates, not a general optimizer framework.

## Four choices, actual profitability

Introduce explicit `MemoryHostArenaAppendChoices{ArenaPeel bool, AppendHoist bool}`
and a separate measured selection schema, e.g.
`minz.memory-host-arena-append-selection/v1`. Candidate masks are fixed ordered:
0 original, 1 arena-only, 2 append-only, 3 both. Each mask must be constructed
privately from an independently fresh original graph and measured with the same
closed workload/compiler options. Preserve every result, including negative cost.

Speed minimizes actual complete-caller T-states; size minimizes actual binary
length. Stable mask order breaks primary ties, so original survives a tie with
an optimized candidate; no unrequested secondary metric or policy extrapolation.
A tie between two nonoriginal minimum masks selects the earlier recorded mask.
No result is inferred by adding separate candidate gains: allocation, codegen
and calling convention pressure can change when both bodies change.

Historical separate observations are original1221494T/2091B,
arena1218395T/2212B and append1150124T/2076B. These establish motivation only.
No combined cost, bytes or winner is known until all four actual candidates run.

## Independent aggregate proof

New public HIR contract `ValidateMemoryHostArenaAppendChoices(source, actual,
workload, choices)` validates unprepared independent immutable snapshots, not
transformed callback authority or a postcompile AST. It accepts only these two
explicit booleans; no site maps or general list of arbitrary transform actions.

Both original functions must satisfy the exact current source shapes and full
ABI metadata. For a false bit, its actual body must equal the complete original
body. For a true bit, independently check the exact known structural delta:

- arena region_match: original zero iterations path returns1 without resource
  access, peeled first read is arena+1 then source_buf+start, mismatch return0,
  length1 return1 after those reads, original i declaration changes only Init0
  to1, original tail body/control/return retained.
- append intern_region: original search/header unchanged; source and destination
  u16 bases hoisted after header poke, original j:u8=0/control/increment preserved,
  only append address expressions change, final bump and return exact original.

Existing standalone proofs mask only their own verified function body and
correctly reject the other changed function. They cannot simply be called in
sequence on the composed module. Preserve their public acceptance unchanged.
Extract internal exact-function checks if justified, then call them from both
standalone and aggregate contracts. The aggregate itself must independently own
all semantic module checks; never turn individual validators into permissive
multi-change APIs or manufacture intermediate transformed ASTs as proof.

Only after each enabled function is independently checked may the aggregate mask
those two bodies in wholemodule equality. Disabled bodies remain exact. Compare
all other functions/bodies, globals/initializers, function ABI/captures/IO/external
attributes, targets, SourceUnits completeness/hashes, strings/kinds, asserts,
sandboxes, warnings/stats and every other semantic field. No unrequested helper,
metadata, ordinary loop or policy changes allowed. All prior policy/LoopLineage
must remain nil, matching current private-only candidates.

Global generated/original binding collisions must be refused across both rules:
original mutable induction globals i,p,j and generated append names. Names are
reserved across globals/functions/params/captures/lexical declarations. Source
and actual verification/closed workload preparation must fail on malformed AST.

## Exact origin union, not lineage by totals

Check all source/actual SourceLocations keys are reachable and unambiguous;
source units and origin digests must match captured originals. Every origin is
bound to exact original function/logical path, not a search for a DeepEqual node.
For enabled arena, map region_match roots0/1/2 to3/4/5 and descendants. For enabled
append, map intern_region roots4/5/6/7 to6/7/8/9 and descendants. These mappings
are disjoint by function. All unenabled and ordinary roots remain unchanged.
New prefix/declarations have no invented coordinates; source cardinality is exact.
Reject detached origin keys, swapped samebody origins, duplicated/shared statement
identities, orphan markers and fake moved source paths.

Source sites are the exact original region_match/body/s1 and
intern_region/body/s5. Original ordinal is0 for each because ordinal is grouped
by source offset, not function sequence; bind captured full site instead of
hardcoded ordinal. Aggregate must reject unmapped source loops and missing sites.
Actual region_match loop is s4 only when ArenaPeel; otherwise s1. Actual append
loop is s7 only when AppendHoist; otherwise s5. Each retained loop must preserve
ID, Kind and complete Origin. Ordinary search loop s1 remains in intern_region.
No counts are attached to transformed sites or inferred from source SHA/IDs.

## Closed-memory effect prerequisite

The aggregate must independently establish source and actual raw VM module
storage exclusion, not rely on Prepare alone. Use verified raw MIR and actual
VM reserved-storage extent for both snapshots. Regions cannot overlap globals
or compiler storage, and native closed runner independently enforces its own
prepared globals/code/spills/stack/return protection. This stronger prerequisite
applies even to mask0 to keep the combined contract uniform and source-bound.

Closed peek/poke descriptors are exact, with no arbitrary external purity. Arena
read pair order and early denial prefixes remain unchanged; append header write
and subsequent read→write order remain unchanged for aliases, modular addresses
and partial stores. The call relation matters: intern_region search invokes
region_match before append, but verified arena preserves exactly the same return,
state and resource events. Therefore it chooses the same hit/miss and executes
the same append phase. Hoisted append bases remain invariant because closed host
writes cannot reach src_buf/str_bump globals. This explains composition beyond
mere disjoint function names. Internal global-load traffic and instruction gas
are not claimed equal.

## Source profile and private measured factory

Reuse the strict original loop-profile loader and complete fresh association
replay, full source/origin FunctionSiteMap and pre-callback input ownership. All
11 original site counts/observations must equal fresh closed run. Require positive
Entries on both exact chosen source sites for this bounded first version;
observed zero BodyEntries remains legitimate. If either source site is unobserved,
refuse the joint selector rather than invent its counts or silently alter mask
space. Existing standalone APIs remain available for single observed sites.

Use explicit retained graph ownership across every mask, before destructive
transform/compile and on error. All original snapshots/sentinels are rechecked;
only known prepared candidates are excluded. Forced GC must pass for independent
fresh sources and actual shared AST must remain rejected. Avoid copying an
implicit six-call index across a refactored engine without explicit lifecycle
validation; reuse existing private association lifecycle unchanged when possible.

Every mask invokes independent aggregate proof before compiler preparation.
Emit a distinct typed aggregate precompile proof containing choices, complete
original/actual baseline Compatibility, full original/actual FunctionSiteMaps,
both captured SourceSites and all ActualSites. Original context is the natural
closed baseline context, not instrumented profile context. Each private mask
factory invocation must produce identical proof. Finally bind proof identities
to that actual measured candidate baseline and original association baseline;
both captured sites must equal loaded original sites. Mask0 validates exact
unchanged source and receives an explicit proof or reuses its immutable original
record only under the same binding checks. Prepared MIR and native sidecar remain
binary/allocation diagnostics rather than postcompile HIR certificates.

Compare all four raw/native semantic observations to original, preserving full
state/events and typed first denial. Only Steps and diagnostic wrapper error text
may differ after completion/failure presence and typed denial equality. Native
infrastructure/stack/provenance/gas errors must yield nil evidence, not an
accepted failed-equivalence record.

## Required narrow tests

- All four exact parsed-source masks pass precompile proof; both mask fails each
  unchanged standalone public proof. Disabled function modification rejected.
- Forged boolean/mask mismatch, counter/ordering/header/read reversal, wide host
  effect, dropped suffix, unexpected helper, metadata/global/policy mutations
  refused. Malformed/nil source/actual and VM-storage overlap refused.
- Origins both shifted correctly; swapped samebody ordinary origins, missing
  unchosen origins, detached/duplicated keys and wrong actual path refused.
- Wrong source/search site, missing actual site, unmapped loops and one unobserved
  component refused; observed zero body remains legal.
- Self-consistent forged profile counts/observations rejected by full replay;
  stale source/workload/host/executable/proof identities refused.
- Delayed factory mutations across replay, mask boundaries and final sentinels;
  forcedGC positive and actual shared AST negative; repeated mask proofs drift
  refused. Check all surviving originals, not only last fresh source.
- Actual full unchanged original30 workload once per goal with four recorded
  candidates, independent host/resource/state oracle and typed baseline payload
  binding. Selected mask follows actual primary minimum/stable ties.
- Reuse separately proven raw/native boundary domains unless composition or
  source/backend changes justify targeted mixed boundary cases. Do not run full
  domains twice merely because the wrapper result schema differs.

## Implementation ownership and size

New HIR aggregate file/tests plus narrow internal exact-kernel helper extraction
may be required; existing standalone proof regressions retain strict semantics.
New pipeline finite selector/evidence/tests use the reviewed ownership/proof
pattern. CLI only if separately requested; new exclusive combined mode/schema,
no reuse of arena/append or ordinary ProfileDecision records. No new default
Grace rule, generalized callbacks, sticky policy, arbitrary helper count, generic
optimizer or cost-model framework is part of this stage.
