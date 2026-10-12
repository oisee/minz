# Native Agon VDU avoids ANSI terminal-mode dependency

The source-model-verified TUI Hello stream was diagnostically translated: cursor ANSI row/column becomes VDU31,x,y (zero-based); clear becomes VDU12; ANSI colors become VDU17,logical and VDU17,128+background. ANSI standard0..7 matches Agon default palette0..7; bright adds8. Reset selects logical7 on black. 313 ANSI bytes become221 VDU bytes.

Full shipped VDP rendered the translated candidate in ordinary mode, with no VDU23,0,255. Captured frame shows the expected box, greeting and colored line. Visual inspection only, no independent pixel oracle or simultaneous CPU/MOS execution. Native VDU uses default8-pixel font height; terminal mode used16-pixel font height, so physical dimensions differ while character coordinates remain equal.

This is feasibility evidence, not a new compiler/library result. Source agon.tui_render remains unchanged. A coherent library change must cover cursor, colors/reset, hide/show, scroll and reverse semantics, then whole CPU+VDP keyboard/return behavior. Simply mixing remaining ANSI primitives with native VDU is not an acceptable final state.

Protocol checked against pinned agon-vdp source1056ee38ec1cc68ed327bd32ee47d04b1c30597c: video/vdu.h VDU12,17,31; video/agon_palette.h defaultPalette10; video/context/graphics.h setTextColour. Native cursor visibility VDU23,1,0/1 checked in video/vdu_sys.h. These files are archived on share with source revision/license.

Reproduce rendering: /usr/bin/python3 scripts/agon_vdp_stream_capture.py --vdp <vdp_console8.so> --stream reports/agon-vdu-feasibility/candidate-vdu.bin --output <new-dir>. Candidate derivation is retained in translate.py. No terminal flag. Initial MOS general poll releases VDP boot wait; result.json pins firmware and stream. Headerless stream is not a MOS executable.
