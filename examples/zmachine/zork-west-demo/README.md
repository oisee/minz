# Zork I: West of House micro-demo

This is a **2,378-byte Z-machine v3 story**, not the full Zork I game. It keeps a tiny, playable opening scene: four sides of the house, the mailbox, and the leaflet. Commands include `LOOK`, `OPEN MAILBOX`, `TAKE LEAFLET`, `READ LEAFLET`, `INVENTORY`, directions, and `QUIT`. It has a deliberately small custom parser; the original Zork parser, objects, puzzles, and save/restore are not included.

The ZIL source and text are an adaptation of the [MIT-licensed Zork I source](https://github.com/historicalsource/zork1/tree/97b7b3d68c075dd9af7da499c3e9690ada3471fd) at commit `97b7b3d68c075dd9af7da499c3e9690ada3471fd`. The full license is in [LICENSE-ZORK1.txt](LICENSE-ZORK1.txt). This demo does not include ZILF libraries or a Z-machine interpreter. It does not grant trademark rights to Zork branding.

The committed `zork-west-demo.z3` is 2,378 bytes (SHA-256 `916d31de9303c9084f275bf6e4e07536e4752988573b06833c23e7768a54e532`). Its header says version 3; dynamic memory ends at offset `0x02ce` and high memory starts at `0x05a7`. The story is small enough to leave ample space in a 48K Spectrum address space, but a complete **Nanz Z-machine interpreter + display/input + loader** does not exist yet and must be measured separately. This file is a fixture for J1–J3 in [BACKLOG.md](../../../BACKLOG.md), not a working Spectrum release.

To rebuild, use the pinned [ZILF/ZAPF 1.9.0 Linux x64 toolchain](https://github.com/taradinoc/zilf/releases/tag/1.9) (archive SHA-256 `06ff0e59eff6e6896fd9ce71d16c100365abbc537cf030f2bd31beb9384d0155`). Set `ZILF` and `ZAPF` to absolute executable paths, then run:

```sh
ZILF=/path/to/zilf ZAPF=/path/to/zapf ./build.sh /tmp/zork-west-demo.z3
sha256sum /tmp/zork-west-demo.z3
```

`build.sh` fixes release and serial fields with `zapf -r 1 -s 000001`; two clean builds should yield the committed hash. The story was exercised with `dfrotz` (Frotz 2.44, commit `fe66e2ef738354f5e2fe37c7c01b1781ef659acd`) using this transcript:

```text
read leaflet   → You cannot see a leaflet here.
open mailbox   → Opening the small mailbox reveals a leaflet.
read leaflet   → WELCOME TO ZORK! ...
take leaflet   → Taken.
inventory      → You have a leaflet.
go north       → North of House
go west        → West of House
close mailbox  → The mailbox is closed.
quit           → Goodbye.
```
