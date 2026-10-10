Fixed the stale A alias in [z80codegen_inst.go](/tmp/minz-grace-pgo-resume/minzc/pkg/mir2/z80codegen_inst.go:129): emitted constants now invalidate destination copy tracking.

Evidence: the original workload, restored through a temporary overlay, failed with tally **25 instead of 11** before the fix and passes afterward. Final assembly loads tally with `LD A,D; ADD A,1`.

Added assembled/emulated regressions for ADD, SUB, AND, OR, and XOR. Full MIR2 and pipeline suites pass.

No pipeline/HIR/CLI edits, no commit; other agents’ edits preserved.