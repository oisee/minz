# Controlled ProfileWorkload: full-source dead extern admission

The controlled workload can now keep unreachable extern declarations in the full
input module without supplying runtime implementations. Source and candidate
call closures are computed conservatively across every block. Either closure
reaching an extern refuses the run, including a call subsequently erased by an
optimizer. Function addresses, indirect/undefined calls, ASM, ports and SMC remain
refused throughout the module. Every retained extern must preserve its full
source callable ABI, external address and declared clobbers; it cannot acquire a
body. Truly dead declarations can disappear only without residual call references.
Full source/context hashes and existing selector mutation checkpoints are retained.

Raw VM runs validate the input and privately generated instrumentation separately.
Only exact owned collector names/signatures are removed from the validation view;
user names with similar prefixes have no authority. Unbound original externs have
defensive error hooks under both VM name forms, with no effects or dummy result.
Native fetch authority intersects assembled instruction starts with the candidate
entry call closure. All instruction bytes remain protected from writes. Equal-PC
assembler aliases share authority; distinct adjacent dead function bodies do not.
An extern label with zero extent can share a reachable instruction address:
static callable admission, rather than impossible PC differentiation, closes it.

Validation: new ABI/body/call/address/clobber/indirect/undefined/ASM/ports/SMC
negative matrix; source direct/transitive/constant-false-call obligations; genuine
DCE; private-name collision; assembled injected jump into a real dead body; all
existing `TestZ80ProfileWorkload*` tests. Focused HIR/pipeline PASS, full HIR PASS
33.803 s. Final allocated MIR intentionally is not passed to an SSA verifier:
allocation legally reuses IDs; this guard validates admission, while the compiler
verifies its preceding SSA stages. Native execution retains independent code,
memory, instruction-bound and restored-SP checks.

The unchanged original `stdlib/fs/fat12.nanz` was executed through public raw VM
and native ProfileWorkload APIs in ordinary and Grace modes. Both modes completed
18 training and 36 frozen heldout calls. An independent packed-word model checks
all returned values, complete resource snapshots and preserved neighbouring
nibbles after each call. Both cost 5,572 T training and 11,199 T heldout.
Ordinary image SHA256: `60803503e50440190c40715fc3aa2c89ad23ba8ac83a301dd6beb91c64612b6c`;
Grace: `013edc316a17480452ec96fe6414289cafa0c26397bfdd4965cc22fdd24b541e`.
This is bounded computation, not disk mounting, CP/M filesystem runtime or a
whole-module behavioural claim.

The attempted four-mode prospective selector passed the previous extern refusal,
then refused a sentinel factory HIR ownership check. Eight freshly parsed,
retained uncompiled FAT modules had no shared AST/storage. Compilation replaces
AST nodes, so a uintptr-only historical ownership set can outlive its referents
and collide with reused allocations. Root is fixing operation-scoped referent
retention separately. No winner or four-mode/heldout PGO benefit is claimed here;
the existing prospective test and historical refusal artifact remain unchanged.
The third FAT flags correctness fix is prerequisite donor `a3e0fe29` (local
cherry-pick `db987221`), already integrated by root; it is not part of this patch.

Artifacts: logs before/after, final full-source ASM/BIN, exact unchanged source,
and frozen workloads are in `grace-profile-dead-extern-artifacts` and companion
logs, with SHA256 manifest. No full CLI or slow full-pipeline repeat was run.
