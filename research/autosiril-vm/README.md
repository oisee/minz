# Autosiril: Crystal + Nanz → HIR → MIR2 VM → VT2

VM-first functional port on `spike/autosiril-vm`, based on PR #252. Actual
`.mide.txt` modules now match the Ruby converter for all six standard mappings.
Memory is deliberately generous; this port is not yet a Z80-sized application.

## Languages and execution

- `music.cr`: musical scalar kernels compiled by the **native Crystal semantic
  frontend**: quantization, cell/pitch policy, octaves, chord-base adjustment,
  ornament clipping, echo volume and legacy pattern count.
- `engine.nanz`: reuses the real SMF decoder, pairs notes, builds the grid,
  flattens poly/mono/drum cells, builds LNotes/ornaments, applies echo, downmixes
  to three channels, resolves envelope conflicts, renders rows, splits and
  deduplicates patterns and constructs the play order.
- `cmd/autosiril-vm`: links signature-checked Crystal HIR functions into the Nanz
  module and runs raw or optimized MZV. The host orchestrates stages for intermediate checks, reads input, decodes CLI
  syntax, serializes snapshots and writes the VT2 header/components computed by MIR2.
  No host function supplies the musical kernels or performs MIDI parsing.
- `tables.nanz` and `vt2-header.txt`: frozen data from the reference's constants
  and `module_template.rb`; the native 31 sample definitions are preserved.

The native scalar Crystal bridge now supports UInt32, unsigned division and
remainder, and lazy conditional expressions whose branches need no lowering
preludes. Signed floor division and conditional branches requiring fallible
lowering preludes are explicitly rejected rather than given different behavior.
This does not extend the separate record exporter to all UInt32 record shapes.

## Verified result

Run from the repository root (Go, Ruby/midilib and Crystal with compiler library):

```sh
python3 research/autosiril-vm/verify.py --autosiril /path/to/autosiril
```

The script builds the real native exporter and Go consumer. Source Autosiril
and MIDI assets are read only; all generated modules/snapshots default to
`/tmp/minz-autosiril-vm-results`.

Six standard mappings—chronos, flim, three imrav variants and tottoro—and five
synthetic fixtures run in raw and optimized MIR2. Every stage is compared with
the actual Ruby `autooisee.rb` classes/functions before its main section:

1. Paired notes, track/source/channel/pitch, original ticks and quantized rows.
2. Exact order/pitch/type in flattened grid cells.
3. LNotes, ornaments, delayed channels and three-channel downmix.
4. Rendered tracker lines, pattern text/usage and play order.
5. Complete VT2 module text and the actual written `.mide.txt` file.

The final run produced **22/22 byte-identical modules before metadata
normalization**, including the six real configurations in both VM modes.
The verifier allows only Author/ArgList metadata normalization for portability;
receipts separately record pre-normalization equality and SHA-256. Intermediate
echo volumes are compared in centi-volume (Ruby's 7.349999999999999 vs 7.35),
while final text is compared exactly. All other intermediate fields are exact.

| Real mapping | Rows | Module bytes |
|---|---:|---:|
| chronos | 1,352 | 62,453 |
| flim | 1,674 | 87,800 |
| imrav_simple | 3,794 | 133,587 |
| imrav_medium | 3,794 | 143,438 |
| imrav_hard | 3,794 | 143,520 |
| tottoro_example | 3,776 | 190,972 |

Synthetic controls cover overlap/FIFO, velocity-zero release, pending notes at
EOT, zero-length notes, channel separation and mixed retrigger/release cells.
Thirty-three scalar kernel cases run in native Crystal, raw and optimized MIR2,
including an unselected zero-divisor branch, selected division error, u32 high
bit and high ticks. Four exporter rejection controls cover signed division,
out-of-width shifts and conditional lowering preludes (including a nested
fallible condition). The earlier native
Crystal scalar/outcome corpus is also rerun; Go tests cover ABI rejection and
actual result storage above 128 KiB.

## Memory and deliberate boundaries

The current VM heap is **87,179,690 bytes**, with observed accesses above 87 MB.
Large indexed globals use existing MZV wide addressing. Explicit language
pointers retain the 16-bit ABI: decoder descriptors/backing bytes are placed
first and checked to fit. IDs in the musical tables are u32 handles, not fake
wide language pointers. The tables are generous fixed-capacity prototypes with
explicit exhaustion errors; run one conversion per fresh VM instance.

No target RAM optimization, banking, disk I/O or audio playback is claimed.
The output is VortexTracker II text, **not VTX AY-register frames**. This first
port covers the six standard configurations with default diatonic transposition
0; optional key/transposition processing and Bitphase `.btp` output remain.
A direct Z80 build is not supported by this large-memory VM layout.

Echo starts from volume 15 as in the reference; its two possible levels are
represented as centi-volume 1050/735. This preserves ordering and final integer
formatting without pretending that the frontend gained general Float support.
Legacy quirks are preserved for parity, including the reference pattern-count
boundary rule and the fallback downmix volume comparison against the base row.
The input decoder currently has a 512-byte per-event payload buffer.
Behavior outside valid SMF/PPQN, supported source mappings and the documented
prototype capacities is not universal Ruby-compatibility.

`evidence/module-results.json` is the current receipt. `cells-results.json` is
the earlier first-stage milestone, identified by its own source hashes/commit;
`typed-regression.json` records the existing scalar bridge regression run.
No original MIDI files or generated full songs are committed.

## Agon/eZ80 follow-up

ADL can address a flat 16 MB space ([Zilog manual](https://www.zilog.com/docs/UM0077.pdf));
AgonLight2 has 512 KiB external SRAM ([Olimex](https://www.olimex.com/Products/Retro-Computers/AgonLight2/open-source-hardware)).
The repository has ADL assembly/MOS output and an independent eZ80 codegen.
The current `pkg/ez80` tests pass, but that is not proof of complete wide-pointer
language semantics. A fresh large-array HIR probe still emits 16-bit address
arithmetic for TargetAgon, versus 32-bit for TargetMZV. Ordinary PointerTo also
retains the 16-bit language ABI. The separately verified agon-z80 runtime uses
a 64-KiB application window. Before using flat RAM from this port, validate a
24-bit pointer/layout path across 0xFFFF, arrays/structs and MOS boundaries, then
shrink the prototype tables to actual available RAM. No Agon hardware execution
or full-flat-ABI claim is made by these VM receipts.
