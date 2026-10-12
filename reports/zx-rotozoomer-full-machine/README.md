# Full ZX rotozoomer with ROM interrupts

The existing metafunction-generated kernel now passes full-machine execution,
not merely direct draw_frame calls. TestZXHalfCellRotozoomerFullMachine loads the
assembled program at 0x8000 into pkg/spectrum.Machine with embedded 48K ROM, ULA
contention and IM1 interrupts. It starts actual main, initializes IY=0x5c3a and a
usable stack, and observes ordinary instruction execution without ROM traps.

Each of ordinary / Grace Size / Grace Speed completes 64 draws: 0..31 twice, 331 video
frames, 315 accepted ROM IRQs, 252 during rendering. Every completed 6912-byte
screen matches an independent model. Draw return PC/SP and preservation of main
and alternate registers across real ROM IRQ execution are checked. ROM FRAMES
counts the 315 accepted interrupts. Repeated draws are paced at five 50 Hz frames,
about 10 fps in this emulator; physical hardware remains unverified.

The Nanz source is still the earlier dedicated kernel spike. Re-expressing it
through the common library is remaining work on the active goal. This report
does not imply masked sprites, reusable texture walk or main integration done.

Run an exported snapshot interactively from a checkout with SDL development
libraries and a display:

    make -C minzc mzx
    minzc/mzx --model 48k --snapshot reports/zx-rotozoomer-full-machine/native/ordinary/running.sna --no-audio

Or validate without a window:

    cd minzc
    go build -tags mzx_headless -o mzx-headless ./cmd/mzx
    ./mzx-headless --model 48k --snapshot ../reports/zx-rotozoomer-full-machine/native/ordinary/running.sna --frames 200 --dump-scr /tmp/roto-screens

Snapshot avoids assuming initial ROM system variables on a cold --run invocation.
Saved native/ contains source-derived ASM/BIN, completed screen dumps, running
SNA and result JSON with ROM/binary digests and completion timestamps. The snapshot
is taken after a machine frame and may show a partially drawn next frame; the
completed-*.scr files are captured exactly after draw return.
