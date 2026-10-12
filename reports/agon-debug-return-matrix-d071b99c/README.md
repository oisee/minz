# Agon exact return contracts: three examples × three modes

Uses the exact public CLI BINs from the independent VDU matrix. Real eZ80 CPU/MOS and full VDP execute; debugger pauses at entry040045 and final RET040051. Compares SPL/IY/IX and24 bytes at the caller stack, checks HL0, single-steps RET and requires PC equal the little-endian three-byte return in the saved stack and SPL+3. ADL must remain1. Assertions use entry values, not fixed firmware addresses.

Fixed early-exit scenarios: repeated normal SDL A for Hello/Demo; repeated Escape for NC. This run does not test NC navigation; separate nativeVDU matrix does. No global RAM-write, independent pixel or hardware claim. Sources/BINs, exact commands, debugger transcripts, captured registers/caller bytes and research harness retained. Python stdlib PTY handles debugger, no additional runtime package.

Run scripts/agon_debug_return_replay.py with emulator/MOS/VDP/SDL paths, the retained nativeVDU matrix, and a new output directory. summary.json lists completed results and scope. Every BIN wrapper is inspected before applying breakpoint addresses. Existing input captureC prevents debugger pauses from consuming the only key event.
