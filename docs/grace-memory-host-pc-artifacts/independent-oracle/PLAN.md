# Bounded native PC-cost attribution

Start with two saved full-30-call binaries: original and both arena/append
optimizations. They share the unchanged original SelfLanz source and workload.
This answers where the already measured total difference occurs without
introducing another source rewrite, permissions, profile, or loop certificate.
If short crossover attribution is needed later, compare original/arena/append
on the saved zero-body workload (the both variant adds no necessary first
question). Admit that separate three-run scope explicitly after the initial
accounting is reconciled. No native execution has occurred in this review.

Use the exact closed production native runner, emulator, memory/stack/sidecar
permissions and invocation ABI. A temporary overlay or test-only helper adds
only counters around the successful memoryHostNativeStep call: capture actual
instruction-start PC before the call, actual CPU T-state delta after it, and
one executed step. No marker, instrumented machine instruction, speculative
opcode cycle table, relaxed memory rule, or different caller bootstrap is needed.
Keep the full executed start-PC record map per invocation, including adapter and
unknown addresses. Reject instrumentation failures rather than dropping records.

For every invocation require equality with the saved evidence's results, ten
mutable/constant global byte values, full resource snapshots, ordered host
events, failure fields, executed instruction count and T-state count. Across all
calls, PC step/T sums must equal function-category sums and saved ExecutedSteps /
TotalTStates exactly. Assembly and machine binary must match byte-for-byte, with
saved SHA256; allocation, final MIR, sidecar, frame and adapter identities should
also match. The host helper executable/context changes under an overlay; declare
that diagnostic context difference explicitly instead of claiming old executable
compatibility. Source/workload/native payload identity remains fixed.

Define **exclusive physical address attribution** only. Resolve emitting
function labels against the actual assembled listing. Give every executed PC
exactly one disjoint byte-address range, with generated helpers retained as
separate physical functions. CALL/RET/tail-jump cost belongs to the address where
that instruction executes. The caller's inclusive duration is not computed by
summing exclusive functions, and recursion does not require an invented call
stack for this bounded first slice.

`node_tag EQU peek` is an alias-only source function with a later textual
function header. It must not create a second/overlapping range at peek's address.
Retain aliases as metadata; the physical `peek` and `poke` bodies are compiler
host adapters, counted in an explicit adapter category. Any shared/emitted alias
entry is canonicalized from actual byte emission, never alphabetical choice.
Function headers alone are not enough to establish address extents. Labels,
code origin, known executable instruction boundaries and final binary extent
must bind ranges. Unknown executable addresses get an explicit unmapped bucket,
never the nearest arbitrary source function.

The closed runner starts at each function entry after setup and writes a return
sentinel directly. It executes no external invocation bootstrap; setup/argument
writes are excluded from T-state timing. Internal calls, their save/restore code,
return instructions and adapters are included. Any runtime trampoline/helper
bytes not contained by a proven emitting function are reported separately.
Do not label physical ranges as HIR loop costs or infer loop lineage from PCs.

Artifacts: raw per-call PC/steps/T records, listing/symbol ranges and aliases,
exclusive per-call and cumulative category totals, source/workload and exact
native identity, saved-vs-replay observation comparison, instrumentation-only
overlay diff and tool context, portable summation auditor, command/log and SHA
inventory. Preserve gzip raw records losslessly and make paths configurable.

Independent negative accounting checks require rejection of missing PC rows,
duplicate per-call PC rows, overlapping function ranges, alias-only ranges,
out-of-binary/unmapped records silently discarded, step/T sum drift and changed
binary identity. Positive branch-sensitive cycle totals must come from actual
CPU deltas, not a fixed cost per mnemonic. These are accounting probes, not new
compiler or runtime semantics tests.
