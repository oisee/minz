# Native VDU fixes Agon TUI in ordinary MOS mode

Unchanged TUI Hello now renders a box/greeting in simultaneous CPU+MOS+full VDP. SDL A down/up at texture updates300/302 reaches MOS_GETKEY; next autoexec echo MINZ_FULL_RETURNED and MOS prompt appear. Images visually inspected, no independent pixel/RAM/SP oracle. Outer timeout10s stops still-running MOS(exit124), not the application. Captures at240/420 delegate SDL_UpdateTexture unchanged. No terminal-mode switch.

Library uses VDU31 cursor,17 colors,12 clear,23,1 visibility. Default16-color palette0..7, bright+8, inputs masked0..7. Reverse swaps tracked colors once until color/reset. Scroll is a native full-width viewport with relative coordinates, explicitly different from ANSI margins. Width/height remain declared80x30; dynamic-mode behavior is not proved. All five frozen products assemble in3modes; only Hello wholedevice is checked. Demo/NC fullVDP and remaining primitives need further proof.

Historical ANSI replay explicitly rejects current native-VDU library; old evidence retains exact old library. Immutable live pipeline b6b4e0c7 predates this change and is not restarted. Source example unchanged.

Reproduction follows reports/agon-full-vdp-original with MINZ_AGON_SEND_KEY=1, MINZ_AGON_FRAME=<before.ppm>, MINZ_AGON_RETURN_FRAME=<after.ppm>. cc -shared -fPIC -I <SDL3>/include capture.c -ldl -o capture.so. Run Fab from release root with --renderer sw --firmware console8 --sdcard <dir> --zero --verbose; LD_LIBRARY_PATH=<SDL3>/lib, LD_PRELOAD=<capture.so>, SDL_VIDEODRIVER=dummy, SDL_AUDIODRIVER=dummy. Retained source compiled with minzc-agon-entry/current library. Tool pins: reports/agon-full-vdp-original/result.json and reports/agon-entry-fix/compiler-build.json.
