# Как MIDI из Autosiril добрался до Z80

Russian illustrated article: [index.html](index.html). Includes a generated
editorial cover, interactive bit-field explorer, state/data-flow diagrams,
corpus comparison, memory accounting, real Nanz and actual emitted Z80.

Build from repository root:

```sh
python3 reports/midi-from-disk-2026-10-26/build.py
```

`build.py` inserts verbatim Nanz source and saved assembler output. The small
bit-field probe was compiled with `pipeline.CompileHIRSteps`, variant search
disabled, and its `kind(163) == 5` assertion passed optimized MIR2 and native
Z80 oracle execution. The artifact directory also contains HIR and MIR2.

Corpus/reproduction evidence lives in `research/binary-stream`. The article
separates working Nanz SMF decoding from the proposed TRD transport adapter,
physical TR-DOS ROM validation and musical playback. The Crystal bridge is an
experimental separate frontend; it does not yet expose this MIDI API.

Cover: built-in `image_gen.imagegen`, `assets/midi-spectrum.png`, original
1536×1024 PNG copied into the repository. Exact generation prompt and tool mode
are recorded in `assets/illustration-prompt.txt`. The cover is an illustration,
not a hardware screenshot or technical wiring diagram.

QA: Chromium desktop 1440×1000, mobile 390×844 and 320×844; image loads, local
links resolve, no horizontal document overflow or JavaScript errors. Bit slider
checked at 0, 31, 32, 163, 255; actual ASM disclosure opens. Screenshots inspected.
