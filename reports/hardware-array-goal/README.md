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
| Independent bounds/memory/PC/SP | Fill matrix proven; copy and dedicated full-main roto proven; library full-main demo proven; reusable texture matrix PASS |
| Speed and size alternatives | Fill pointer/LDIR measured; copy pointer/LDIR measured; mask pointer/compiled measured; texture unroll costs measured; 60-variant / 480-runtime-data native matrix PASS |
| Full Spectrum display+IRQ execution | Both dedicated and shared-library main proven: ROM/ULA/IRQ, 64 screens per mode |
| Main integration | Design reports merged PR86; Library and prerequisites staged in PR85; local integrated gates PASS; GitHub required checks pending |
| Shared artifacts | Fill, preserving LDIR and roto packets copied and checksummed |

Work order: copy semantics and measured primitives, masks/compiled constants,
reuse layouts and operations in the roto demo, full screen+IRQ run and runnable
packaging, curated main integration. New IR layers are not required for current
library generators. Speed may spend bytes; Size remains a separate choice.
