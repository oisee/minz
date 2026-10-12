# Byte-result arithmetic preserves its live pair neighbour

VIR GCD PBQP fallback assigns live b to B and modulo result to C. Inline division writes BC; saveLiveScratch previously skipped the entire pair because dst C overlaps it. GCD(12,8) returned0. The saver now preserves a live neighbour even when the other byte is the result, stages that result in A around POP pair, then restores the result byte.

Original TestVIR_Assert_GCD now PASS with five unchanged assertions, raw MIR VM and native fallback. Native regression covers all256 dividends×255 nonzero divisors for result B and C (130560 calls), an independently generated live neighbour, exact PC/SP. PASS0.149s. Existing division/signed/scalar/scratch-save checks PASS2.831s. Full MIR2 completed PASS, exit0, 68.001s; full-mir2.txt retained. Session57081 is terminal.

This is a production codegen fix exposed by the research path. The still-running full pipeline b6b4e0c7 predates it; its result cannot verify this change. VIR Lookup remains incorrect, the entry-liveness-only probe did not fix it and was reverted. Full Z3-enabled VIR run predates both research fixes.

Lookup follow-up: width-only constraints still returned10 for idx1; extending terminator-argument liveness back to definitions returned0. Neither proves correctness; both probes reverted. Existing exported source/MIR/ASM are retained in reports/vir-return-placement. No Lookup fix claimed.
