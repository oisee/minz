# Independent association wrapper review

Reviewed association checkout commit `3839b5c1afe4faf8c8bd11194af9dd7e30c5e39f`;
exact file hashes are in `association-review-snapshot.json`. Read-only review;
no CPU replay and no production edits. No material acceptance hole found in the
reviewed synchronous factory/observation contract.

The outer wrapper owns workload JSON before the first user factory call. Its own
freshness set spans all five calls, including the compiled candidate and both
sentinels; retained pointers prevent allocator reuse. Baseline internally binds
three fresh outputs. Only output index one is excluded from later source identity
checks because normal compilation changes that private candidate. Its evidence
is captured before later factory calls and contains owned values/strings rather
than candidate AST pointers.

Fourth-call mutation of original or baseline sentinel is caught before profiling.
Fifth-call mutation of any uncompiled retained source, including the profile input,
is caught by rechecks against baseline compatibility and full FunctionSiteMap.
FunctionSiteMap binds all origin paths and complete function ABI/body identities;
final LoopSiteMap additionally preserves exact input loop identities. No new
factory invocation happens after the final checks. Arbitrary asynchronous mutation
is outside this synchronous callback contract and is not claimed as protected.

Failed evidence is accepted only when the profiler returned a typed access denial.
Each invocation preserves failure presence before diagnostic Error text is erased,
then compares exact typed Kind/Address/Write, ordered events, globals, regions and
results with the closed raw baseline. Completed is checked independently. Gas or
infrastructure errors have no typed access denial and return nil association
rather than becoming successful or equivalent failed evidence. Per-invocation
counts are generated internally; callers cannot supply evidence/counts/callbacks.

Profiler compatibility is checked using its own closed semantics context and is
not overwritten with baseline compatibility. Source identity is independently
checked with baseline context; native costs/image remain the baseline's private
uninstrumented compilation. This associates source observations with a measured
baseline, and does not prove transformed-loop lineage or authorize PGO decisions.

Existing tests exercise fourth/fifth delayed original mutation, typed partial
write/read-denial prefix, gas refusal and original thirty-call baseline. Parent
reports full eleven-site independent oracle equality. Those executions were not
repeated during this bounded review.
