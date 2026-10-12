# Explicit schedule live-out use for island boundaries

OpKeepAlive is a scheduling-only src0 use with an empty zero-cost Z80 pattern. It extends the value lifetime through preceding clobbers without introducing an executable instruction. This is needed before ExitLocations can serve as a meaningful island boundary contract: a dead value's former location does not prove its bits survive the schedule.

Regression keeps word0x9182 live after an asm clobber of DE/D/E. Solver chooses a surviving word location; actual assembled Z80 independently checks its bits and exact return PC/SP. PASS0.023s. Exit-location and complete original VIR assert family PASS6.827s.

Production source lowering does not emit this operation. Automatic island handoff remains unimplemented; this commit provides its required explicit liveness operation, not complete island correctness. Hinted solver, CFG width and independent island boundary moves remain open. Existing immutable gates run their prior checkpoint.
