# VIR island continuation keeps the MIR terminator at block end

emitPIRWithLabels emitted a MIR block terminator at every island segment, including a segment ending inside that block. A CALL helper at an internal split could then be followed by RET and peephole-converted to JP helper, discarding remaining operations. The emitter now emits the block terminator only when the segment reaches the original block end.

Regression explicitly stitches two pieces of the same block: CALL helper, then LD A,42. Assembled native execution must return42 at exact PC=f100/SP=f000; only one terminal RET is emitted. PASS. Full TestVIR_Assert_ family and selected entry/return tests PASS6.727s.

This fixes continuation placement, not the still-incomplete live-value transfer between independently solved islands or CFG width domains. Rejected width/same_pair experiments are not enabled. Existing immutable VIR c86bfcd5 and production pipeline b6b4e0c7 runs predate this change and continue independently.
