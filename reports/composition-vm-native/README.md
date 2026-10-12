# Composition: mathematical oracle ↔ final MIR VM ↔ assembled Z80

Four unchanged transferred sources a1/a2/a4/a8, ordinary/GraceSize/GraceSpeed. Each of162 domains exhausts65536 a/b pairs at frozen c and c/d boundaries0/1/127/128/254/255. Final MIR VM and assembled native each execute10616832 calls and compare with independent modulo256 models. Per-domain input counters require131072 VM+native invocations. VM must return exactly one value without failure; native must reach original CALL continuation8003 with restoredSPff00 before executing DI/HALT.

Twelve variant artifacts retain exact Nanz source, final MIR, options, bootstrapped ASM and assembled BIN. These binaries include the CALL/DI/HALT harness, so sizes are not whole product performance claims. Halt-in-function, jump-past-continuation and badSP native controls are rejected. This verifies values and native return, not rawHIR VM, fullRAM/flags effects or profitability.

Command: MINZ_COMPOSITION_EVIDENCE=<new directory> GOCACHE=/tmp/minz-grace-gocache go test ./pkg/hir -run '^Test(NativeCompositionAdditionalArgumentBoundaries|CompositionReturnOracleRejectsWrongMachineCode)$' -count=1 -timeout=15m -v. Go1.26.8. Focused terminalPASS3.360s; current immutable fullpipeline is separate and predates these added tests. Compiler production sources unchanged.
