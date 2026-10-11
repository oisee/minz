The first private emitter draft preserves original lhs/rhs order for Ult/Uge and sets pendingFlagReg to the current result. It invalidates A only and clears swapped/two-test state. scratchLiveAfter uses function live-out and reverse instruction kills, so a successor-only live A/pair is accounted for. Before generic pending materialization is the right bounded selection point, provided all live pending values and unresolved physical operands are refused.

Two review requests sent to owner before native full30:

1. Conservatively exclude constVals/deadConsts operands as the stated initial scope. Canonical computeDeadConsts does not fold normal word comparison operands, so no existing source wrongcode is asserted; this is a fail-closed experiment restriction for suppressed/unresolved physical values. Old generic word comparison itself does not promise rematerialization of such forged states.
2. Native tests initially stopped at the internal done label. They demonstrate actual C/Z, pair/SP preservation and20/23T, but need an additional run through consumer branch/RET to prove Ult/Uge orientation in generated consumers. Live-A successor fallback already uses an actual future-live parameter.

Pending-state refusal and cache invalidation should have direct meaningful tests; no new memory authority or production pipeline activation is proposed. Final source review remains open until owner's focused terminal/frozen code and these test gates are available. The independent arithmetic oracle is complete; no native CPU has run in this review.
