# VIR CFG: terminator argument lifetime and PHI identity

Lookup(1) returned10 rather than20. The entry idx only appears in terminator arguments, so marking it live solely at the final op allowed XOR A to overwrite its original ABI value. Terminator arguments now remain live back to their definition (or block entry). Incoming liveness now maps old source IDs to destination PHI IDs. Identity-changing PHI edges no longer also emit redundant same-ID copies into the destination.

Both changes together are needed: either isolated probe still failed. Original four Lookup assertions now pass; expanded checks cover every valid index0..7 of the unchanged eight-byte array. Complete TestVIR_Assert_ family after the three fixes passes (exact counts in result.json); includes MultiReturn and GCD. Research Z3 is available only on this test process PATH. Full older VIR run and production pipeline remain independent immutable jobs, not proof of this new solver change.

Saved source/MIR/ASM/allocation and edge-debug log show real idx preservation before XOR and correct loop increments. This is bounded original regression evidence, not general CFG/island optimality or allRAM proof. Command: PATH=<research-z3-bin>:$PATH VIR_ASSERT_EXPORT=<output> go test ./pkg/vir -run '^TestVIR_Assert_' -count=1 -timeout=10m -v; exit0,6.732s. Expanded Lookup repeat PASS0.140s.
