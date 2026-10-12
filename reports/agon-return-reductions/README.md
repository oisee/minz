# Agon MOS return reductions

Original tui_hello repeats the MOS boot banner after draw_box. getkey alone (including tail call) and getkey followed by text return normally. draw_box followed by text reproduces the banner without key input. Generated draw_box uses IYH as scratch.

Differential external ADL wrapper probes: preserve IY only removes the second banner; zero HL status only does not; preserve IY plus zero status returns normally. Exact original payload was retained and the wrapper appended, changing only the header jump address. Wrappers are hand-encoded PUSH IY / CALL 040045 / POP IY / optional LD HL,0 / RET.

In-source PUSH/POP probes did not complete the following MOS echo: their explicit void main emits DI/HALT in the old bare-metal path. This is a separate entry termination defect, not evidence against IY preservation.

MOS source audit: tomm/agon-mos src/misc.S exec24 uses IY as frame pointer and explicitly requires preserving registers. The firmware map retained in the emulator source places exec24 at 000811. Source checkout is a later revision than firmware, so the differential actual-firmware probe is decisive. Tool/firmware hashes: reports/agon-mos-console-matrix/summary.json. Upstream source and license archived on share.

These are reductions and negative observations, not graphical proof. Fake VDP cannot render ANSI. All stdout/stderr, exact BIN/ASM and inputs retained.
