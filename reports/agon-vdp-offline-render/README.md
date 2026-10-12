# Full VDP rendering of retained TUI stream: terminal mode required

The same exact native TUI Hello ANSI stream was replayed through the shipped full VDP library using its UART and framebuffer API. It produces a box, white greeting and colored second line only after VDU23,0,255 enables terminal mode. Ordinary VDU mode shows escape sequence text, not a box. Both captures use MOS general poll23,0,128,1 to release the firmware boot wait. Screenshots were visually inspected; no independent pixel oracle.

This is offline replay of an already CPU/MOS-verified stream, NOT simultaneous whole application execution. Existing agon.tui_render does not enable terminal mode. Consequently the present library does not yet establish an idiomatic graphical app contract on default VDP. It also needs terminal-mode keyboard/exit handling: enabling terminal changes input from MOS events to serial bytes. Do not merely prepend terminal enable and claim the key loop still works.

Reproduce with /usr/bin/python3 scripts/agon_vdp_stream_capture.py --vdp <vdp_console8.so> --stream <observed-ansi.bin> [--terminal] --output <new-directory>. Use system Python: Conda's older libstdc++ cannot load the shipped firmware. Exact initialization bytes, stream, framebuffer PPM/PNG and firmware hash retained.

Full Fab GUI now starts using locally built SDL3 and dummy SDL drivers. SDL3-3.4.18 source and build provenance are retained on share; no system libraries changed. Full keyboard/CPU/VDP proof remains the next step.
