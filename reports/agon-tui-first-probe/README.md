# Agon TUI first execution: not a graphical PASS

The ordinary public CLI product executes enough to emit the source greeting, box-text and cursor/color sequences. Fab CLI fake VDP reports every ESC as an unknown packet; it does not render ANSI. After input a the MOS startup banner repeats before the subsequent echo. Host exit is 0, but this does not prove a normal clean application return or rendered UI.

Therefore this probe does not promote tui_hello or the other graphical programs. Exact source, library, ASM/BIN, input/output and observation retained. Need a real VDP/terminal output oracle and explanation of the repeated MOS banner. main ends in JP tui_read_key (tail call); this is a clue, not a diagnosed backend error. No production change made from this observation.

Tools and firmware are the same pinned ones in reports/agon-mos-console-matrix/summary.json. Compile: minzc-99ab4b54 examples/agon/tui_hello.nanz -b ez80 -o tui.bin. Run: agon-cli-emulator --mos mos_console8.bin --sdcard <directory containing tui.bin> --unlimited-cpu --zero, with retained stdin.txt.
