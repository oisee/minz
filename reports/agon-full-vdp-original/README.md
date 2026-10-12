# Whole eZ80/MOS/full VDP baseline: original TUI does not render ANSI

The exact public CLI TUI Hello product from the Agon entry-fix matrix was loaded by real MOS autoexec.txt and run under Fab Agon Emulator1.2.5 with its full shipped VDP. The framebuffer captured from the real SDL texture update shows literal ANSI sequences and greeting, not the intended box. This corroborates the offline VDP diagnostic with a simultaneous whole CPU/MOS/VDP execution.

No input key was injected. Application remains waiting for a key. The outer timeout8s ended the capture (exit124); this is an observation cutoff, not a completed application or clean-return PASS. No runtime or source patch was applied. Pixel content visually inspected; no independent pixel oracle.

Research capture.c interposes SDL_UpdateTexture via LD_PRELOAD and saves the 240th texture update, then delegates to SDL unchanged. SDL_VIDEODRIVER=dummy, SDL_AUDIODRIVER=dummy, renderer sw. Built with system cc and the locally built SDL3 headers. Driver, firmware and program hashes are retained. This is not a compiler dependency.

Reproduce: place program.bin and autoexec.txt in an SD directory; run Fab from its extracted release directory with --renderer sw --firmware console8 --sdcard <dir> --zero --verbose. Environment LD_LIBRARY_PATH=<localSDL3>/lib, LD_PRELOAD=<compiledcapture.so>, MINZ_AGON_FRAME=<frame.ppm>, SDL_VIDEODRIVER=dummy and SDL_AUDIODRIVER=dummy. Stop after8seconds. The capture source compilation command is cc -shared -fPIC -I <SDL3>/include capture.c -ldl -o capture.so.

Next fix must align Agon library rendering with native VDU while keeping ordinary MOS key input and return. Original programs remain unchanged.
