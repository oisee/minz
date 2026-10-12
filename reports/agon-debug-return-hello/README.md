# Agon Hello: exact return boundary in eZ80 debugger

Same public ordinary BIN as the nine-mode VDU matrix. Fab full-device debugger stops at entry040045 and final RET040051. Entry and pre-RET SPL both0bfe66, IY both0bfe75, IX both0bc32e. HL before RET is000000. All24 bytes at caller SP match byte-for-byte; little-endian saved return74 08 00 implies000874. Single-step RET lands exactly000874 (MOS POP AF), SPL0bfe69: three-byte ADL return popped. This directly confirms one bounded return/stack-frame contract, not full RAM or arbitrary programs.

Run pinned Fab/Console8 MOS/fullVDP and local SDL3 with --debugger --breakpoint 0x40045 --breakpoint 0x40051, ordinary Hello SD from reports/agon-native-vdu-matrix-d071b99c. LD_PRELOAD capture.so injects repeated SDL A down/up every120 updates so debugger pauses do not lose the input; capture.c retained. Commands at entry: registers; mem SP 24; c. At pre-RET: registers; mem SP 24; s; registers; exit. Raw and ANSI-stripped transcripts retained. A final mem command without0x prefix was rejected by the debugger and supplies no additional evidence; the two successful SP dumps are compared.

This is a separate run from exactVDU matrix; do not combine their input/time observations. Real CPU/MOS/fullVDP, no hardware execution and no global RAM write audit. Preserve the ordinary single-program scope.
