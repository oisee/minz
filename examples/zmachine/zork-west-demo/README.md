# Zork I: West of House micro-demo

This is a **2,378-byte Z-machine v3 story**, not the full Zork I game. It keeps a tiny, playable opening scene: four sides of the house, the mailbox, and the leaflet. Commands include `LOOK`, `OPEN MAILBOX`, `TAKE LEAFLET`, `READ LEAFLET`, `INVENTORY`, directions, and `QUIT`. It has a deliberately small custom parser; the original Zork parser, objects, puzzles, and save/restore are not included.

The ZIL source and text are an adaptation of the [MIT-licensed Zork I source](https://github.com/historicalsource/zork1/tree/97b7b3d68c075dd9af7da499c3e9690ada3471fd) at commit `97b7b3d68c075dd9af7da499c3e9690ada3471fd`. The full license is in [LICENSE-ZORK1.txt](LICENSE-ZORK1.txt). The Nanz interpreter is a separate experimental implementation in this directory; ZILF libraries are not included. This demo does not grant trademark rights to Zork branding.

The committed `zork-west-demo.z3` is 2,378 bytes (SHA-256 `916d31de9303c9084f275bf6e4e07536e4752988573b06833c23e7768a54e532`). Its header says version 3; dynamic memory ends at offset `0x02ce` and high memory starts at `0x05a7`. The experimental [Z3 interpreter in Nanz](zvm.nanz) runs this story on `mzv`. It covers the CZECH v0.8 Z3 assertion corpus, including arithmetic, objects, properties, indirect variables, text, random, and verification. Its `@z3_symbols` and `@z3_unicode_table` metafunctions turn declarative Z3 glyph mappings into typed decoders at compile time; the runtime interpreter remains ordinary Nanz. File and terminal operations remain host I/O boundaries. This is not yet a complete Z3 interpreter or a Spectrum release.

To run the checked transcript from a MinZ checkout:

```sh
cd minzc
go build -o /tmp/minz-mzv ./cmd/mzv
../examples/zmachine/zork-west-demo/smoke.sh /tmp/minz-mzv
```

The smoke compares [transcript.in](transcript.in) against [transcript.out](transcript.out). The same commands were run in `dfrotz` 2.44; game text and state changes agree, aside from Frotz's status line and line wrapping. `mzv` currently needs `-H` for this reproducible piped-input run. The core uses byte arrays for 16-bit Z-machine locals and return addresses because the present Nanz/MIR2 VM truncates `u16` array elements; this is tracked under C3 in the backlog.

To play the included demo interactively after building `mzv`:

```sh
cd examples/zmachine/zork-west-demo
/tmp/minz-mzv -H zvm.nanz
```

Try `look`, `open mailbox`, `take leaflet`, `read leaflet`, `inventory`, and `quit`. For a separately built MIT Zork I v3 story, copy `zvm.nanz` and the story into a persistent directory, naming the story `zork-west-demo.z3`, then run `mzv -H /path/to/directory/zvm.nanz`. Save, transcript, and command files are written beside that Nanz source.

For the external CZECH v0.8 Z3 assertion corpus, build the pinned `czech.inf` as described in the [test-corpus report](../../../reports/2026-09-30-ZMachine-V3-Test-Corpus-RU.md), then run `bash ../examples/zmachine/zork-west-demo/czech-smoke.sh /tmp/minz-mzv /path/to/czech.z3 /path/to/dfrotz` from `minzc`. The script verifies the story hash and all 349 assertions. With Frotz supplied, it also compares the text of all 19 print cases, while reporting the remaining blank-line differences. CZECH is not copied into this repository. The [small MIT Z3 fixtures](../z3-fixtures/README.md) cover memory output, nested streams, transcript, command recording/playback, save/restore and a routine near the 128 KiB address limit.

The external [MIT Zork I](https://github.com/historicalsource/zork1) build of 86,928 bytes (SHA-256 `66e54935b47bf9d76e05bc97ba42844ae8e7294a0625bc08c9cac206f4b68b51`) passes `full-zork-smoke.sh`: `LOOK`, `OPEN MAILBOX`, `TAKE LEAFLET`, `READ LEAFLET`, `INVENTORY`, `QUIT`, plus `save → restore`. With Frotz supplied as a third argument, the six-command game text matches after excluding Frotz's status lines. Its binary is not included here. The Nanz interpreter loads up to 128 KiB on `mzv`; its PC uses a 32-bit value and saves the 17 needed address bits in three bytes per return frame. Save files use the interpreter's private format in `zvm-save.dat` beside the Nanz source; they are checked against the story release, serial, and checksum. This format is not Quetzal. Transcript and command files are `zvm-transcript.txt` and `zvm-commands.txt` in the same directory.

The remaining Z3 work is the terminal screen/status model, sound, rigorous input/error boundaries, and longer interactive-game transcripts. Default Z3 extra characters render as UTF-8 on screen and in transcript output, while memory stream 3 stores their original ZSCII bytes. UTF-8 keyboard and command-playback input maps defined extra characters to ZSCII and folds accented uppercase letters before tokenisation; unrepresentable characters become `?`. The 128 KiB heap allocation is specific to host `mzv`; Spectrum bank mapping and the Z80 build remain a separate milestone. Nanz/MIR2 can index large arrays and perform 32-bit arithmetic, but its current pointer ABI is 16-bit, so this interpreter keeps host-visible buffers before the large story allocation. Its wide-PC return calculation is split into explicit steps to avoid a known narrowing bug.

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
