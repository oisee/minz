# Mini NC: independent native VDU navigation oracle

The unchanged ordinary public CLI BIN runs on real eZ80/MOS and full VDP. Transparent VDP proxy records and forwards every transmitted byte unchanged; normal SDL→PS2→MOS input delivers nine Down, Enter, Up, Enter, Escape. Exact timing/capture.c is retained from the boundary experiment.

All 3151 application bytes match the independent source-level text/layout/navigation model translated to native VDU. It requires every ordered redraw, selection at index7 after two excess Down presses, selection at index6 after Up, final cursor visibility/reset/clear and following MOS echo/prompt. Complete wire contains3248 bytes including firmware traffic. No expected byte is derived from compiled output. Replay: python3 reports/agon-native-vdu-wire-nc/verify.py.

Build vdp-tee.c with cc -shared -fPIC -ldl; capture.c with local SDL3 includes. Run the pinned Fab release with --renderer sw --firmware console8 --vdp <tee.so> --sdcard <9084e512 NC sdcard> --zero --verbose. Set MINZ_REAL_VDP=<original vdp_console8.so>, MINZ_VDP_STREAM=<wire.bin>, LD_LIBRARY_PATH=<SDL3>/lib, LD_PRELOAD=<capture.so>, dummy SDL video/audio. MINZ_AGON_FRAME=<last.ppm>, MINZ_AGON_UP_FRAME=<up.ppm>, MINZ_AGON_RETURN_FRAME=<returned.ppm>. timeout14s exit124 stops continuing MOS; following echo verifies application return. Firmware/tool pins are retained in predecessor reports/agon-full-vdp-demo-nc and reports/agon-native-vdu-wire-hello.

This is an independent command/text oracle for this bounded ordinary sequence. Framebuffers remain visual evidence, not an independent pixel/RAM/SP oracle. No all-mode, arbitrary-input or hardware claim.
