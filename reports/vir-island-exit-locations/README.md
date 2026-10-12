# VIR solver exit locations for future island contracts

Global and per-instruction solution parsers now attach ExitLocations (vreg→physical loc) to the final PIROp. Emission ignores this metadata; no allocation or instruction choice changed. Global parser retains all solved values; per-instruction retains values with locations at the last schedule point. This provides authoritative solver output needed to bind the next island, but stitching is NOT yet implemented.

Tests cover pinned word/global exit placement and unconstrained per-instruction exit value matching final PIR destination. Original assert family and continuation test PASS6.768s. The per-instruction test avoids DstHint because an existing SMT generation bug places assert statements inside total_cost. A probe moving hard DstHint outside cost allowed those queries but exposed count5=0 instead5 and GCD failure; probe reverted. Rejected full log retained. No claim of valid hinted per-instruction path yet.

Both full immutable runs predating metadata continue independently. Further work: solver contracts and accurate boundary liveness, then actual island handoff. Width/island allocation probes remain rejected.
