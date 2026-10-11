# Independent append-copy base-hoist review

Reviewed prototype source checkpoint a15cb422 / worktree58428373 and HIR proof
draft saved by exact file SHA. No parent or owner production source edits. A
bounded independent overlap probe was executed only; completed full30 actual
workload was not replayed.

Concrete finding: original public HIR proof called PrepareMemoryHostWorkload but
assumed compiler-global/storage exclusion. Prepare validates layout/regions but
only RunMemoryHostMIRBaseline establishes module heap extent. A declared writable
region at address0 passed the old proof while the closed runner refused overlap.
This made the proof's invariant-global precondition unstated. The fixed owner
validator now lowers/verifies both source and actual, obtains actual NewVM symbol/
global storage extent and refuses each region below that extent before execution.
Before probe: proofErrnil / runneroverlaperror. Fixed probe: proofoverlaperror and
runneroverlaperror, PASS0.070s. Native measured evidence was already safe because
its public runner refused overlap independently.

The exact transformation hoists src_buf+start and str_bump+1 AFTER header poke and
before j=0. Both sums and subsequent +j remain typedu16 modulo65536; same u8 j
increment/loop condition preserves0..255 semantics. It does not reorder host
operations: each source peek occurs before destination poke; previous iteration
writes precede following reads. Source==destination and forward overlap are
sequential copying, not memmove. A denied source read yields no current write;
a denied destination write retains the successful current source read. Equal-
value writes still occur. Header poke executes before source base is loaded,
retaining the original effect if header aliases source bytes. New local reads
on len0 introduce no resource events; VM instruction gas may differ and is not
claimed equal by the proof.

Global bases are invariant within the exact append body because it contains only
closed byte hosts, j-local assignment and no global write. Under newly established
resource exclusion, neither host can mutate src_buf/str_bump. Scan and region_match
execute BEFORE the bases are captured; arbitrary changes there do not undermine
within-loop invariance. The proof preserves the whole search/header prefix and
final bump assignment/return. Mutations of unrelated helpers/globals/function
metadata and exact origin paths are independently rejected. Reserved binding
collisions and original p/j global collisions refuse rather than relying on
lowerer name precedence.

Remaining gating is domain evidence, not another legality hole: owner is testing
all lengths, alias ±1, header/source aliases, wrap and each partial permission
prefix on raw/native runners. A future measured selector must bind the exact
intern_region/body/s5 source site to actualbody/s7; two loops in intern_region
make a function-name wildcard insufficient. No transformed counts or postcompiler
proof should be inherited. Candidate raw/native full observations must remain
compared against original on the same closed workload.

Independent saved-evidence audit (noCPU): all30 raw and30 target observations
match after Steps are ignored; global/region bytes, results, events and failure
fields remain exact. BinarySHA and invocationTStates sums verified directly:
original1221494T2091Bfed98; hoisted1150124T2076B7c244d. Improvement is71370T
(5.84%) and15B on this workload, not a general speed guarantee. TypedGo payload
SHA audits remain owner's separate evidence, not Python reserialization claims.

Additional original-name collision review: lowerExpr VarRef resolves local env
before globals. Exact intern_region parameters start/len and declared slen/new_p
are only read after binding and are never AssignStmt targets. A same-named global
therefore does not alter the transformed-vs-original computation. Only p/j are
local assignment targets, whose lowering can prefer globals; both are explicitly
refused. No additional unsafe collision was identified. Global src_buf/str_bump
are intended module bindings, protected by region exclusion. Prepare performs
HIR Verify before LowerModule; the exact accepted source shape contains no
unsupported dynamic/indirect control or undefined binding. Arbitrary malformed
HIR is not a newly admitted shape here; existing typed verifier/lowerer failures
are not presented as solved by this proof.

Final selector evidence audit at frozen7a67aa80: saved speed/size payloads each
contain two genuinely measured30-call baselines. Both goals choose hoisted;
original1221494T2091Bfed98 and hoisted1150124T2076B7c244d. Exacttypedprecompile
proof original/actual compatibility and FunctionSiteMaps bind those two baseline
evidences. Source site is intern_region/body/s5, actualtailbody/s7 with sameOrigin;
source13statemententries/286bodies match per-invocation sums and retained profile.
All30raw/native global/region/result/event observations agree after instruction
Steps only are ignored. No transformedbodycounts or postcompileproof claimed.

Each target carries90 exact global/spill descriptors. Every saved descriptor PC
opcode is present in its saved binary, direction/bytebudget fits its typed root,
and global symbol+offset resolves to the saved bounded address. These variants
emit no call-snapshot descriptors (emptylistSHA verified); ordinaryCALL/PUSH/POP/
RET remain checked by the existing finiteframe runtime, initializedbytes and
exactreturn/SP rules. This audit does not claim a dynamicstacktrace export absent
from payload. Source/runtime reviewed unchanged protectedrecipe semantics.

Four typedGo payload checks confirm exact sidecarSHA, allocation rawbytesSHA,
MIRdumpSHA, embedded/savedBIN and ASMidentity, invocationcostsums and30-success
boundaries. Python audits separately verify typedJSON content/state/proof without
claiming identical typedGo reserialization. Scripts support raw/gzip artifacts.
No measuredCPU replay was performed. Observedzero-body reversal (original731T,
hoisted895T) is owner test evidence outside the30call artifact audit; it retains
original, showing the selector follows measuredprofit rather than forcedrewrite.
