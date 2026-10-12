# Call-only literal producer placement prototype

The production change causing the arena solver-input drift is 1f87d7b7: integer literal call arguments now use the declared parameter width and index-class moves. The typed fix is retained. Forcing only historical Arena_init/arena_split conventions on current typed input assembles1130→1121B and reproduces the exact historical190T arena_split instruction sequence (static identity, not new native measurement).

The bounded generic prototype recognises a unique same-block u16/i16 OpConst consumed solely by one direct call argument. It models no boundary adapter because the matching apply pass changes the producer class before allocation. Shared/later/phi/opaque/hard/ABI-fixed producers refuse. Literal payload, type and source metadata stay unchanged. Greedy is unchanged. Focused placement/refusal tests pass; no emulator execution.

The prototype assembles1130→1128B but retains arena_split220T code. It is a partial cost-model improvement, not restoration of the historical30T function gain and not yet native validated. The stronger word-pair conflict diagnostic is deliberately excluded from production: it produced worse IX-based argument code and requires a physically justified joint model. No fixed function-name rule is proposed.
