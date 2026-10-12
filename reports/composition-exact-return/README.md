# Composition oracle: exact CALL continuation

Previous bounded PC/SP check observed the boot DI/HALT end and admitted either neighboring PC. A corrupt return could skip DI and still satisfy it. The new runner stops at the original CALL continuation (8003) with restoredSPff00, before executing any boot continuation instructions. Halt inside the function, jumping to8004 and incorrectSP are rejected by actual assembled negative programs; a validRET is accepted.

All162 additional-argument domains passed again:10616832 native calls in ordinary/HIR Grace Size/Speed, independent mathematical byte models. Argument counts remain asserted. This establishes values and exact control/stack return, not fullRAM/flags effects. No production change; no profitability claim. Historical prior reports remain scoped to their older checks.

Command: go test ./pkg/hir -run '^Test(NativeCompositionAdditionalArgumentBoundaries|CompositionReturnOracleRejectsWrongMachineCode)$' -count=1 -timeout=15m -v. Go1.26.8, GOCACHE=/tmp/minz-grace-gocache. Raw terminal log retained. Current fullpipeline is immutable and predates this focused test-only strengthening.
