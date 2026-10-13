# Hardware-array library goal

User goal set October 12, 2026: expressive Nanz array/screen library using
metafunctions and Antique Toy techniques; independently verified Z80 behavior,
measured speed/size alternatives; working 32×48 ZX rotozoomer in a screen and
interrupt capable Spectrum emulator; code/demo/reports in main and shared assets.

Completion requires all of these, not merely successful operation-level tests:

| Requirement | Current evidence / remaining work |
|---|---|
| Declarative fill | stdlib/hw/fill.nanz: pointer and preserving LDIR; native tests |
| Declarative copy | stdlib/hw/copy.nanz pointer/LDIR; 13-case native/VM matrix PASS |
| Masked sprites | Static byte-aligned masks: pointer/compiled; independent VM/native tests PASS |
| Texture walks | Reusable @texture_walk with runtime setup and static unroll; library full-main demo PASS |
| Independent bounds/memory/PC/SP | Independent fill/copy/sprite/texture oracles; raw/final VM where applicable; full native RAM, exact PC/SP and live values PASS; texture: 480 native datasets |
| Speed and size alternatives | Fill pointer/LDIR measured; copy pointer/LDIR measured; mask pointer/compiled measured; texture unroll costs measured; 60-variant / 480-runtime-data native matrix PASS |
| Full Spectrum display+IRQ execution | Both dedicated and shared-library main proven: ROM/ULA/IRQ, 64 screens per mode |
| Main integration | PR85 merged into main at 26cc879e; all six required CI contexts succeeded; main ROM/IRQ and launcher verification PASS |
| Shared artifacts | Ten operation/demo/checkpoint packets: 1,530 manifest entries verified against repository and /mnt/safe/minz/grace/2026-10-12, with no mismatches |

Work order: copy semantics and measured primitives, masks/compiled constants,
reuse layouts and operations in the roto demo, full screen+IRQ run and runnable
packaging, curated main integration. New IR layers are not required for current
library generators. Speed may spend bytes; Size remains a separate choice.

## Verification on the main-based integration tree

The hardware operation and full-machine suite passed in 105.759 seconds. The full
Nanz package passed in 29.746 seconds and the reviewed executable catalog in
3.612 seconds. These historical timings describe the integration tree. PR85 subsequently merged
into main at `26cc879e` on October 13; all required checks succeeded.

The reusable texture kernel was tested in 60 compiled variants on eight changing
runtime setups/textures each. The library demo additionally executed its actual
main on the Spectrum emulator with the real 48K ROM, IM1 interrupts and ULA
contention: 64 complete 6912-byte screens per mode, two ordered angle cycles,
315 ROM interrupts (252 during drawing), exact draw return PC/SP and preservation
of both register banks across the interrupt handler.

Run `scripts/play_zx_rotozoomer.sh` for the SDL window, or append
`--headless --frames 200 --dump-scr build/roto-screens --screenshot build/roto.png`.
This starts from the verified ROM-initialized snapshot. Physical hardware and
tear-free presentation are outside the current evidence.

## Main verification, October 13

PR [#85](https://github.com/oisee/minz/pull/85) merged as `26cc879e` after
all six required CI contexts succeeded. A clean checkout of that main commit
ran the library full-machine test: PASS 4.309 seconds, 64 independently checked
screens per mode, 315 ROM interrupts with 252 during drawing, exact draw PC/SP
and register preservation. Its ASM/BIN and 192 completed screen dumps match
the retained packets byte for byte. The headless launcher also completed 200
frames, producing 160 distinct 6912-byte screen images and a valid PNG.

All 1,530 entries in ten repository manifests match their shared copies.
The separate full pipeline on `154e0a05` completed all 583 entries exactly once:
546 PASS, 37 existing conditional SKIP, zero FAIL. The later host-global-address
repair is verified separately by explicit pointer-width regressions and the full
Z3 fixture smoke; main CI also passed those fixtures. These results establish the
named library/demo scope, not correctness of every program: fuzz findings remain
recorded, and physical Spectrum execution and tear-free output are unproven.

PR checks now run an explicit fast profile; deep domains run on release tags,
nightly and manual audits. See [testing policy](../../scripts/ci/TESTING.md).
Main verification logs, native exports and manifests are under
`/mnt/safe/minz/grace/2026-10-13/main-26cc879e-*`.
