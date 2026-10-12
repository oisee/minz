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
| Masked sprites | Not implemented in the common library |
| Texture walks | Existing independent roto meta/ASM spike; reusable abstraction pending |
| Independent bounds/memory/PC/SP | Fill matrix proven; copy and dedicated full-main roto proven; sprite/library demo pending |
| Speed and size alternatives | Fill pointer/LDIR measured; copy pointer/LDIR measured; sprite/texture choices pending |
| Full Spectrum display+IRQ execution | Full dedicated-kernel main proven: ROM/ULA/IRQ and 64 screens per mode |
| Main integration | Design reports merged PR86; compiler/library still research branch |
| Shared artifacts | Fill, preserving LDIR and roto packets copied and checksummed |

Work order: copy semantics and measured primitives, masks/compiled constants,
reuse layouts and operations in the roto demo, full screen+IRQ run and runnable
packaging, curated main integration. New IR layers are not required for current
library generators. Speed may spend bytes; Size remains a separate choice.
