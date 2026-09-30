# Zork I: West of House micro-demo

This is a **2,378-byte Z-machine v3 story**, not the full Zork I game. It keeps a tiny, playable opening scene: four sides of the house, the mailbox, and the leaflet. Commands include `LOOK`, `OPEN MAILBOX`, `TAKE LEAFLET`, `READ LEAFLET`, `INVENTORY`, directions, and `QUIT`. It has a deliberately small custom parser; the original Zork parser, objects, puzzles, and save/restore are not included.

The ZIL source and text are an adaptation of the [MIT-licensed Zork I source](https://github.com/historicalsource/zork1/tree/97b7b3d68c075dd9af7da499c3e9690ada3471fd) at commit `97b7b3d68c075dd9af7da499c3e9690ada3471fd`. The full license is in [LICENSE-ZORK1.txt](LICENSE-ZORK1.txt). The Nanz interpreter is a separate experimental implementation in this directory; ZILF libraries are not included. This demo does not grant trademark rights to Zork branding.

The committed `zork-west-demo.z3` is 2,378 bytes (SHA-256 `916d31de9303c9084f275bf6e4e07536e4752988573b06833c23e7768a54e532`). Its header says version 3; dynamic memory ends at offset `0x02ce` and high memory starts at `0x05a7`. The experimental [Z3 interpreter in Nanz](zvm.nanz) runs this story on `mzv`. It now covers the CZECH v0.8 Z3 assertion corpus, including arithmetic, objects, properties, indirect variables, text, random, and verification. Its `@z3_symbols` metafunction turns the declarative Z3 A2 glyph mapping into a typed decoder at compile time; the runtime interpreter remains ordinary Nanz. The core instruction loop and story semantics are in Nanz; `file_read` and `tui_read_line_or_eof` are the host I/O boundary. This is not yet a complete Z3 interpreter or a Spectrum release.

To run the checked transcript from a MinZ checkout:

```sh
cd minzc
go build -o /tmp/minz-mzv ./cmd/mzv
../examples/zmachine/zork-west-demo/smoke.sh /tmp/minz-mzv
```

The smoke compares [transcript.in](transcript.in) against [transcript.out](transcript.out). The same commands were run in `dfrotz` 2.44; game text and state changes agree, aside from Frotz's status line and line wrapping. `mzv` currently needs `-H` for this reproducible piped-input run. The core uses byte arrays for 16-bit Z-machine locals and return addresses because the present Nanz/MIR2 VM truncates `u16` array elements; this is tracked under C3 in the backlog.

For the external CZECH v0.8 Z3 assertion corpus, build the pinned `czech.inf` as described in the [test-corpus report](../../../reports/2026-09-30-ZMachine-V3-Test-Corpus-RU.md), then run `bash ../examples/zmachine/zork-west-demo/czech-smoke.sh /tmp/minz-mzv /path/to/czech.z3` from `minzc`. The script verifies the story hash and checks all 349 assertions plus key print samples; CZECH's other 19 print cases still need an independent transcript oracle. CZECH is not copied into this repository. The interpreter currently loads one small story into a fixed Nanz array; it has no save/restore, output streams, or 128K story paging. Those are separate requirements before claiming general Z3 support.

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
