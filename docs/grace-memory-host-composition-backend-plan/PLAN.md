# Four-mask arena/append composition: native contract plan

Read-only plan based on reviewed append selector snapshotdc4a65dd. Parent source
is unchanged. No new compiler/runtime source, CPU execution, permission widening
or combined performance claim is introduced here.

## Existing native capability

Arena-only measurement previously emitted one finite call-result snapshot. Its
supported type is exactly one byte result SourceA Width8, Destination a saved
nonA/nonF byte, SnapshotPairs[AF]. The resolver binds every instruction marker
and exact opcode sequence: PUSHIX/PUSHAF, IX=SP anchor, readsnapshotAFhigh, patch
one saved destination byte, restoreAF/HL/IX and caller-saved pairs. Runtime checks
entry initialized role bytes, return-frame separation, stepPC/SP/IX, each pending
byte direction/address/value and final restore. Numeric stack addresses never
acquire permission solely by being inside stack range.

Append-only both-objective payloads emitted zero snapshot descriptors;90 ordinary
global/spill access descriptors bind exact finalPC opcode bytes and symbol offsets.
CALL/PUSH/POP/RET still use finite return/SP frames. Empty snapshot list does not
mean arbitrary stack load permission. Saved binary/proof evidence is separate
from a dynamic stack trace that is not exported.

Composition changes caller register pressure: append source/destination bases
replace repeated globals/spilled j address calculations; arena peel changes
region_match body/returns and ABI clobbers. A result destination or caller-save
set can change despite disjoint source functions. Therefore zero/one historical
snapshot counts cannot establish the combined shape. Existing resolver must
remain failclosed for word/multiple/flag results, different scratch/indexed slot
roles or noncontiguous sequence. No broad dynamic-taint/IX/SP whitelist is justified.

## Minimal measured prototype

Use exact fresh unmodified originalSelfLanz factory, same closed workload and
same sourceprofile source sites. Aggregate HIR proof validates two exact function
deltas and all union origins; existing standaloneproofs correctly reject both
changes and must not be weakened. Candidate construction remains private.

Construct fixed mask0/1/2/3 in independent owned modules, keep all graph allocations
alive before transform/compile and retain immutable source proof. Compile privately
with current baseline options and optional generated sidecar. Ask current native
runner to execute measured candidate. No caller assembly/host callbacks accepted.

For initial exploration it is sufficient to execute only NEWboth once after its
aggregate source proof is ready; oldthree measurements are explicitly historical
motivation, not fresh four-candidate evidence. Later finalselector run must measure
allfour under identical frozencontext and wholecallerworkload before selecting.
No arithmetic addition of separate gains predicts combinedprofitability.

If combined fails native snapshot capability, preserve exact finalMIR, allocation,
plain/annotatedASM and byte-identicalBIN, generated frame descriptors and failing
PC/SP/address. Classify: acceptedsemanticwrongcode, unprovedgeneratedform or stale
artifact. Do not fall back to source weakening or permit numeric protectedaccess.
Only a concrete newly emitted typed form may motivate a separate bounded capability
patch with exactgeometry/initializedslot/returnbyte regressions and unchanged
nil-option defaultbytes. Extend resolver/runtime narrowly and re-review; no blanket
statement that all codegen snapshot forms are supported.

## Actual audit acceptance

Allfour targets must match originalraw/native results, globals, fullregionbytes,
ordered hostevents and firsttypeddenial. Failedsource/partialcopy boundarydomains
remain necessary at composition boundary but shouldreuse prior domains when no
source/kernel/backend change invalidates them. Orderedarena reads preserve the
same searchhit/miss; append then performs same sequential peek→poke alias behavior.
Global/permission exclusion remains independently checked peractualmodule.

For each finaltarget verify typedsidecar/frames/allocation/MIRSHA usingGo, binary
SHA/savedASM byteidentity, invocationcostsums, complete30calls. Inspect generated
protectedPC opcodes and descriptor roots/width/direction; comparetypedprecompile
proof naturalbaselinecontexts/fullfunctionmaps to both measuredcandidateidentities.
Sourceprofile counters remain originalobservations, not transformedlineage.

Select speed by completecaller T-states and size by wholebinarybytes; stable mask
order resolves primaryties. Record allnegativecosts. No secondarycosttie preference
or unseenmaskprediction. CLI/defaultgate repeats only if actualexistingcompiler
files change; newoptinAPI-only stage can reusesaveddefaultbytes after exact source/
reference inventory, while requiredpackages/pipeline tests run once afterfreeze.
