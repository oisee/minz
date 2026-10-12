# Reproducible Agon MOS TUI matrix

Run scripts/agon_mos_tui_replay.py with explicit --compiler, --emulator, --mos and a new --output directory. It compiles each public CLI ASM/BIN, runs actual eZ80/MOS firmware, compares the complete stream against the independent source model and checks subsequent MOS echo with one boot banner. A failure stops the run and retains diagnostics; no partial winner or blanket PASS.

Four scenarios x three compiler modes: TUI Hello key; TUI Demo key; Mini NC immediate ESC; Mini NC Up at zero, Enter and ESC. Source files are unchanged. Each output package pins compiler, emulator and firmware SHA256 plus all commands, source, library, expected/actual streams and logs. Compilation timeout120s, execution30s. This is research-only tooling.

VDP is fake: decode only its known ESC diagnostic, reject others. Passing proves exact emitted ANSI stream and MOS return, not actual graphical display, full RAM/SP, Down navigation or real hardware. NC stdin VT line adds CR, yielding source keys11,13,27. No literal output harvested as an oracle.

Independent model: scripts/agon_tui_stream_model.py. Production entry fix:31030f07 (preserves IY and called voidmain returns RET). Report and README in main remain at the earlier CLI Hello update; these research proofs are linked from the branch and copied to share.
