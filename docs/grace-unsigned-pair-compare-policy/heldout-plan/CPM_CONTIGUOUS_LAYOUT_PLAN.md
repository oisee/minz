# CP/M COM layout: bounded resolution plan

Read `self-lanz-cpm-main` diagnostic wrapper `171acff4` and MZA COM serialization. No assembly, emulator, or production edit was performed for this plan. The string repair and separately named CString main correction must close before a successful CP/M smoke is attempted.

The mapping defect is concrete: MZA `handleORG` changes logical current address without adding output padding, while encoded instructions/data append to one output byte slice. `generateCOMFile` checks first origin 0x0100 and size, then returns that slice. Thus the second ORG at 0x6000 makes declared stack labels differ from their physical COM file offsets. The old 6157-byte diagnostic cannot be loaded as though it populated the entire declared span through 0x7004.

Prefer a single `ORG 0x0100` and serialize only startup, module code/data, and the BDOS bridge. Define stack/canary addresses as explicit EQU constants (low canary 0x6000, stack low 0x6002, top/high canary 0x7002); do not serialize empty stack storage. Startup saves incoming SP in an ordinary data word, validates available memory, writes both canaries, installs private SP, calls the explicitly adapted main, checks canaries, restores incoming SP, and exits via JP 0. The six-pair BDOS-2 bridge and actual peek/poke contracts remain unchanged.

Before touching the fixed resources, read the conventional BDOS boundary word at 0x0006 and reject a TPA that cannot cover the complete source-buffer interval [0xC000,0xD000). Require code/global/string bytes below 0x6000, stack disjoint from code and fixed arenas, and incoming OS stack outside the private stack/resources. Canary words are executable initialization, not assumed file-loaded values. A lower usable BDOS address or insufficient private stack must produce a clear refusal path. mze currently models BDOS at 0xFE00; that does not prove arbitrary CP/M machines provide the same RAM.

An alternative is a single ORG plus explicit DS padding from the actual code end to 0x6000 and then serialized stack/canaries. This can form a valid flat COM but unnecessarily includes the stack bytes. If chosen, assert the resulting file length is exactly image_end minus 0x0100; the same mapping checks still apply.

Required static gates before execution:

- One ORG only; entry 0x0100; no overlapping emitted address ranges; code_end <=0x6000.
- Independently assemble and compare raw BIN and target-COM output. For every listing byte at address a, require `COM[a-0x0100]` to equal that byte; each defined emitted object must fit the same interval.
- File size must equal the contiguous serialized image end minus 0x0100. EQU-only stack labels must have no corresponding serialized storage claim.
- Validate exact startup save/install/restore and all bridge pushes/pops, explicit HALT-to-return adaptation, peek/poke allocated ABI, and unchanged source/derived-source hashes.
- Add static malformed multi-ORG/gap and insufficient-layout refusal cases. Retain the original diagnostic and its mapping failure.

A later one-shot mze run checks actual console bytes, parser state, canaries, SP restoration, normal BDOS entry, and warm-boot exit against independent expectations. Capture complete source, derived assembly, COM, listing, symbols, input/output, executable hashes, and command. It is CP/M BDOS emulation smoke, not a physical CP/M OS run or a PGO certificate.
