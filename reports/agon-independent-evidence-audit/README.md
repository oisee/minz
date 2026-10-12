# Agon: independent retained-evidence audit

Reparses retained raw debugger output and complete wire streams independently from producer summaries. Executes no CPU. All9 public CLI VDU runs and9 debugger return runs are complete, share exact source/BIN and emulator/MOS/VDP identities. Source-level expected streams are regenerated, not trusted from saved PASS flags. Each raw return snapshot requires preserved SPL/IY/IX, matching24-byte caller frame, statusHL0, ADL1 and exact PC/SPL after three-byteRET. Saved JSON is then compared to parsed evidence.

Manifests must cover every retained file without duplicates, omissions or unsafe paths; every SHA256 is verified. Matrix labels must match the complete Cartesian product. Retained source-level model files must match the audited model.

Six negative controls reject incorrect returnPC, incorrectSP, nonzeroHL, changed caller frame byte, mutated text and missing following MOS return marker. Run: python3 scripts/agon_evidence_audit.py --wire reports/agon-native-vdu-matrix-d071b99c --returns reports/agon-debug-return-matrix-d071b99c --output <new result.json>.

This strengthens bounded command/text and caller-frame evidence. It does not execute the CPU again, audit allRAM writes, prove independent framebuffer pixels or hardware execution. Exact navigation and early-exit return are separate runs on identical binaries, not one combined trace.
