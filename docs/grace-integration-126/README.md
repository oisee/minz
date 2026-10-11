# Full gate regression and independent typed-address correction

The frozen708e77c5 full pipeline terminated FAIL165.407s: existing
TestCompileHIRRejectsBeforeDumpAndLowering exposed ResolveErrorABI(nil)
before HIR verification.01fd3576 restores structural admission before CY
visitors and runtime field access; malformed nil function/typed-nil statement
controls added. Existing and both-public-API native tests PASS0.029s.

Independent byte fusion audit found rawImm address adjacency differs from
stored typed constant bits. Example u8 Const256 executes as0: raw reads
255/0, whereas the old fusion falsely read255/256. Roota7954042 corrects
matcher, metadata verifier, and ordinary/retained MemoryHost address proofs.
Frozen counterexample and corrected tests retained in reports/fusion-*.

Integrated typed-address/fault/callback/native/root preflight check PASS:
HIR0.009s, MIR2 0.010s, pipeline7.151s. Actual FAT helper native benefit and
wrap tests rerun here. A new full pipeline runs on a7954042; the previous
full CLI still runs on708e77c5 and must keep that attribution. No full
current suite PASS claimed. Source rename MIR2→MIR remains deferred.

Full integrated HIR/MIR2/Nanz subsequently PASS27.587/24.403/13.031s.
