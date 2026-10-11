# Declared direct MIR call shape verification

Base `4f58692b`. Production: new `verify_calls.go` plus one Verify invocation.
Independent caller-buffer owner retains its metadata verifier; no buffer schema
or HIR callable inference edits are included here.

Before: all seven malformed declared direct call cases passed Verify: missing
argument, extra argument, NoReg argument, consumed void/missing result, extra
return beyond callee slots, untyped argument and incompatible aggregate-to-scalar.
Exact failing tests retained in `before.txt`.

The new verifier checks defined ordinary OpCall and OpCallCond against physical
Contract.Params. Arity includes any appended hidden parameters; public/source
argument count is not used. Unknown/unregistered calls and @mir intrinsics retain
existing separate authority; no blanket declaration requirement added.

Types use representation compatibility rather than exact equality: HIR permits
integer/pointer coercions and VM parameter binding truncates to formal type.
All numeric widths/signedness and integer-pointer cases remain accepted; pointer
receivers can receive aggregate/array addresses. Struct result addresses remain
valid. This does not prove every allowed implicit numeric conversion is free of
backend widening limitations; the check preserves the current semantic family
boundary and does not invent a new language policy.

A consumed result must correspond to a callee return. Extra return registers
cannot refer beyond callee slots. Omitted ExtraRetTys remains valid for existing
MIR fixtures; provided metadata must be compatible. Discarding results remains
valid. FallibleCY outcome extras retain their existing independent verifier.

New tests prove malformed rejection, all scalar/address coercions, discarded
results, conditional calls and declared extern argument shape. Existing focused
raw/final/native regressions cover unsigned widening/mixed arithmetic/signed
comparison, typed printing and pointer memory effects. No new target execution
or corpus coverage count is claimed by this verifier change.

Buffered ABI integration still requires the owner's independent descriptor
checks (unique contiguous ParamIndex, correct ReturnIndex/layout, callee metadata
match). This ordinary checker handles complete argument slots automatically;
it does not infer hidden ownership from arbitrary metadata.

First full MIR2 gate exposed one incomplete test fixture: countdown recursively
returns u8 but Contract.Returns was nil (`TestLoopOriginsTailRecursionMove`).
The fixture now declares its actual u8 return ABI; behavior and lineage checks
are unchanged. First failure log is retained separately. No verifier exception
was added to accept a missing consumed return declaration.

Terminal full MIR2 PASS85.624s after correcting the incomplete fixture; focused
call shapes/VM refusal/lineage PASS0.009s; selected actual-source native pipeline
PASS8.140s. Buffered owner confirms independent VerifyStructReturnBuffers hook
will coexist and uses full physical Params plus SourceParamCount for public APIs.
