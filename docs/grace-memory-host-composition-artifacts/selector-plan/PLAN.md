# Finite combined selector lifecycle refinement

Design-only checkpoint based dc4a65dd, referring to composition prototype9d0
and independent aggregate HIR proof a61fbbb1 + test/doc17550149. Parent sources
remain unchanged. Native four-candidate measurement is separately owned; no
new CPU or tests are run for this plan.

## API and masks

Dedicated `MeasureMemoryHostArenaAppend(factory, workload, serializedProfile,
MemoryHostArenaAppendSelectorOptions{Goal,MaxSites})`. Exactly MaxSites=2 and
goal speed/size. Four masks fixed0 original,1 arena,2 append,3 both; mask bits
map directly to hir.MemoryHostArenaAppendChoices{ArenaPeel,AppendHoist}. A new
schema `minz.memory-host-arena-append-selection/v1` records every actual candidate,
choices, typed precompile proof, baseline evidence and selected mask. No arbitrary
choice maps, transformed callback, extra rules or general engine are exposed.

Both exact original sites must be observed: region_match/body/s1 and
intern_region/body/s5, Entries>0, BodyEntries may be0. Missing/unmapped/duplicate
site refused; search intern_region/body/s1 is not append. Loaded full source
sites are copied, not reconstructed from name/ordinal alone. Require both sites
for this first bounded API; standalone modes handle single observed components.

## Explicit factory lifetime

Own workload and profile bytes before callbacks; bounded options and8MiB limit
before copying. Use one seen-pointer set and one strong graph-retention root
collection across ALL masks and association, including error returns. Do not
weaken jointCheckFreshHIR or clear seen between candidates.

Expected default lifecycle (to be validated explicitly, not guessed from arbitrary
user input):

- Public call1: initial original source; capture loaded strict profile identity,
  fullFunctionSiteMap and complete loop-site map.
- Association consumes5 wrapped factory outputs: rawbaseline(call2), private
  compiled candidate(call3), baseline sentinel(call4), profile source(call5),
  profile sentinel(call6). Outer retained length6. Only known compiled candidate
  index2 is excluded from immutable ORIGINAL identity checks. Its precompile
  graph remains strongly retained despite replaced/freed AST nodes.
- Each optimized mask1,2,3 invokes the existing private baseline wrapper, which
  asks its variant factory3 times: rawsource, compilationcandidate, sentinel.
  Each variant invocation obtains TWO public originals: untouched proof source
  plus mutable candidate. Thus6 public calls per mask,18 additional,24 total.
  Exact lifecycle checks per mask should be expressed via counter snapshots,
  not global hardcoded indexes into unrelated arrays.

For each fresh module, run sharing gate immediately and retain its complete graph
before ANY destructive compile/hoist. All untouched proof originals join the
immutable-original list. Candidate source identity must equal captured original
before private construction; its pre-transform graph joins retention roots, but
it is kept separately from the immutable-original list because its exact changed
shape is intentional. No blanket exception based on 'module changed' is allowed.

After every factory/transform/proof phase check all retained originals; after each
mask baseline wrapper and final mask, recheck all originals and captured profile
source. The inner baseline controls its own prepared candidate and transformed
sentinel checks. Outside it, candidate graphs remain owned but only originals
must remain semantically identical. ForceGC tests must verify independent graph
success while actual sharing remains rejected across masks, not merely within one.

## Original reuse without redundant native measurement

Reuse association.Baseline as actual mask0 measurement. Original source counts
are freshly reobserved once via strict closed association and all11 counts and
semantic observations must equal serialized profile. Native instrumentation cost
is excluded as before. This association already internally measured original
raw/native under the exact owned workload; a redundant fourth original compile
would add cost and lifecycle review without independent evidence.

Mask0 still receives explicit independent aggregate false,false precompile proof.
Build it from initialsource and retained original rawbaseline(index1), captured
before any source drift; these are distinct fresh unchanged snapshots. The
record must bind to association.Baseline compatibility/functionmap and exact two
loaded source sites. No fabricated actual transform factory is needed.

For masks1..3, construct actual only inside private variantFactory; call the
independent aggregate validator BEFORE compile for every one of its3 invocations.
Private helper applies only finite requested arena/append bits. All original
proof snapshots stay unchanged. Repeated record for a given mask must DeepEqual;
record across different masks intentionally differs and must not be compared as
if all actual function maps were the same.

## Typed proof, identity and sites

New `MemoryHostArenaAppendPrecompileProof` schema
`minz.exact-memory-host-arena-append-precompile/v1` contains Choices,
OriginalCompatibility,ActualCompatibility,OriginalFunctions,ActualFunctions,
SourceSites with exact arena+append fields (prefer struct, not arbitrary map),
and ActualSites complete loop map. Contexts are NATURAL closed baseline contexts;
loaded profile context remains distinct instrumented semantics. No inherited
counts, postprepare HIR certificate or SourceSHA-only lineage assertion.

After independent aggregate validation and identity capture:
- source arena site is exactly loaded region_match/body/s1;
- source append site is exactly loaded intern_region/body/s5;
- actual arena site preserves ID/Kind/Origin at s4 ifarena,else s1;
- actual append preserves ID/Kind/Origin at s7 ifappend,else s5;
- unchanged search/ordinary loop/source origins remain exact via aggregateproof.

Final per-mask checks bind OriginalCompatibility/OriginalFunctions to the actual
association original baseline, ActualCompatibility/ActualFunctions to THAT mask's
actual private baseline. SourceSites equal captured loaded pair; choice bits equal
candidate mask. ActualSites must derive from precompile snapshot and retain those
actual physical HIR paths. MIR dump/allocation/binary/sidecar fields remain typed
native diagnostics bound by baseline hashes, not a manufactured post-HIR proof.

Identity rechecks include fullFunctionSiteMap origin digest and original profile
compatibility; profile JSON replay includes every counter/percall observation.
A matching fresh factory must not conceal earlier mutable source/global/origin
changes; all untouched originals remain rechecked through finalsentinel.

## Four actual objective values

Each mask uses the same compiler options. Goal affects selection only. Compare
actual target TotalTStates for speed and Binary length for size. Stable maskorder
minimization: initialize selected0; update only on strictly lower primary cost.
This retains original on its ties and earlier mask among equal nonoriginal minima.
Never add separate gains, fit counts, hardcode old costs or short-circuit measuring
mask3 because separate masks looked positive. Preserve negative candidate results.

Every raw/native observation set must equal original's full semantic state and
ordered resource effects. The existing comparator may normalize Steps and error
wrapper text only after failurepresence/typedFailure checks. Any infrastructure,
provenance, bounds on compilerstorage, unsupported opcode or gas failure must
return nil rather than being accepted as an equivalent semantic denial.

## Why explicit finite implementation

One small combined selector file with private helpers for proof construction and
mask selection is preferable to a generalized callback engine. Lifecycle,
sourceproof and controlled allocation authority are security-sensitive, and the
existing two standalone wrappers must retain strict acceptance. Reuse existing
pure ownership-retention, observation-equality and objective helpers if helpful;
share no public mutable 'expectedsource' or external baseline pointer authority.

If later reducing duplicate wrapper code, do so after this stage proves behavior
with exact regression coverage; do not refactor already verified standalone
selectors just to introduce one finite four-mask mode. A private per-mask loop is
sufficient; closures over immutable owned workload and captured source identity
are fine, arbitrary transformation callbacks are not.

## Focused gates before final actual E2E

- Aggregate fourmask proof valid, falsebits rejectdelta, bothstandalone validators
  continue refusing both; VM-storage/global invariant enforced uniformlymask0..3.
- Full selfconsistent forged profile rejected by replay, exacttwo sourcesites
  checked (including search misselection/unobserved component), observedzerobody.
- Late factory drift at association and masks1/2/3 sentinel boundaries, true AST
  sharing acrossmask boundaries, forcedGCfresh success, all untouchedoriginals
  checked even after previous candidate completed.
- Original mask0proof binds distinct initial/rawsource snapshots; repeated per-mask
  records and typed choice/source/actualcompat/functions/sitepath/origin mutations
  refused. Wrongmask proof cannot be attached to another candidate.
- Size/options/cap before callback, stale host/workload/execcontext, infraerror
  nil-evidence, stableprimary ties. Validate final selectedmask against all4costs.
- Original30 actual fourcandidate evidence bothgoals once after finalproofshape;
  independent host state/event audit plus typednativeSHA audits everycandidate.
  Reuse unchanged raw/native domains; only new actual composition effects or
  backend/source failures justify additional domain replay.

No compiler parent source edits, tests or native reruns are part of this plan.
