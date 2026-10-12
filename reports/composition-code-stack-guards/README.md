# Composition: immutable code and bounded stack guards

All162 existing composition boundary domains pass again in ordinary/HIR Grace Size/Speed:10616832 finalMIRVM calls and10616832 native calls, independent modulo256 models and exact native CALL continuation/SP. Native additionally compares every emitted code-image byte and two16-byte guards atfec0..fecf andff00..ff0f after every call. Private stackfed0..feff may change. This is bounded memory integrity, not an allRAM write trace or stack high-water measurement.

Three assembled negative programs deliberately write42 into code8000, lower guardfec0 or upper guardff00 and then return normally. All reach the exact return continuation but fail the memory oracle. Prior corrupt control-flow programs remain rejected. These controls prevent correct arithmetic/RET from masking memory corruption in the checked regions.

Command: go test ./pkg/hir -run '^Test(NativeCompositionAdditionalArgumentBoundaries|CompositionReturnOracleRejectsWrongMachineCode|CompositionMemoryOracleRejectsNativeWrites)$' -count=1 -timeout=15m -v. Go1.26.8, GOCACHE=/tmp/minz-grace-gocache. TerminalPASS3.368s. Test-only change; prior retained compiled artifacts remain historical. No profitability or fullRAM/flag claim; fullpipeline remains a separate immutable job.
