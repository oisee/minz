# ZX 32×48 rotozoomer using shared Nanz generators

The demo now imports hw.texture and hw.fill. @texture_walk describes destination,
page-aligned mutable texture, runtime setup symbols and unroll factor; no local
hardware kernel generator remains. Six @fill spans prepare the fixed bitmap.
The eight angle/setup tables are still produced by genuine Nanz metafunctions.

Actual main with 48K ROM/IM1/ULA contention passes 64 completed screens in each of
ordinary, Grace Size, Grace Speed: 323 video frames,315 ROM IRQ,252 during draw. Every
6912-byte screen matches independent math, frame order 0..31 twice, PC/SP and ROM
IRQ main/alternate-register preservation hold. Faster fill initialization reduced
startup from 331 video frames for 64 draws to 323; repeated draw cadence ~10 fps.

A separate direct-kernel experiment covers all 6 unroll factors across 32 frames:

| Unroll | Binary bytes (original init/setup) | Kernel frame 0 T |
|---|---:|---:|
| 1 | 1388 | 323907 |
| 2 | 1464 | 315459 |
| 4 | 1616 | 311235 |
| 8 | 1920 | 309123 |
| 16 | 2528 | 308067 |
| 32 | 3744 | 307539 |

These direct kernel counts exclude draw_frame wrapper; previous 312363 T includes
that wrapper. Bytes refer to experiment using original init, not current library
demo. Final reusable library native matrix will retain its own exact packets.
No automatic selector: factor 4 is the demo choice; Speed may choose a larger factor.

Running snapshot is in native/ordinary/running.sna. Build MZX then:

    minzc/mzx --model 48k --no-audio --snapshot reports/zx-rotozoomer-library/native/ordinary/running.sna

This is full Spectrum emulator evidence, not physical hardware or tear-free output.
The library and demo are still in research branch until curated main integration.
