# CP/M capture harness review

Reviewed test-only checkpoint `f313f841` from permanent `cpm-memory-capture`, based on e51846f1. The exact COM SHA087a9c7376591a82169b963891493ab6389cf26ebab728ef08a63e7664c73112 and extent2574 are required before execution. Entry0100, CCP stackFDFE and zero return sentinel are explicit. Existing `setupCPMBDOS` enables all RAM and installs the existing emulator BDOS handler; BDOS2 writes actual stdout. No alternative output adapter or permissive native-memory callback is installed.

The existing ten-million-cycle execution limit remains in force. A concurrent pipe drain avoids stdout filling and blocking the emulator before that limit. On ordinary Execute return, including returned error, the pipe is closed and os.Stdout restored. Full65536 RAM, initial/final registers, actual output, exact COM, cycles/error/exit code are saved before semantic assertions.

Source expected output is the actual45-byte input `(fun add ((a u8) (b u8)) u8 (return (+ a b)))`, followed by parsed output and counts17nodes/24stringbytes. The final output assertion is useful but not sufficient for full program state. The later independent saved auditor must check status1, source bytes and C terminator, node/arena metadata, stack/canaries, and untouched executable bytes through independently resolved symbols. Warm-boot PC0/exit0 alone is not a semantic status proof.

No blocking source defect identified for the bounded released run. Scope limits: pipe read errors are currently discarded, and cleanup is not deferred through panics. Returned Execute errors do restore stdout; arbitrary panic safety is not claimed. No CPU execution was performed by this reviewer.
