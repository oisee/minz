# Autosiril paired notes → native Z80 flattened events

The real lazy flatten algorithm now executes as assembled Z80 with its actual
Crystal `cell_action` and `prefer` selectors. The Go runner loads paired intervals
and checks results; all musical choice, iteration, note access and output-array
writes execute as Nanz/Crystal target code. No target musical host callback or GC
is used. This is a bounded stage, not the whole MIDI → VT2 converter.

## Executed result

The fresh verifier rebuilds the native Crystal semantic exporter, exports the
actual music source, and invokes the actual Ruby `vmod2rmod/rmod2pmod` functions.
It compares all emitted row/pitch/kind/order with raw MIR2, final MIR2 and Z80:

- 47 Ruby-backed cases cover p/m/d selection, empty input, overlap, duplicate
  pitches, zero-length/retrigger/release, early limits0/1/3/7, source255, absent
  source, row65535, 512 active notes, a real Chronos source0 first24-note excerpt,
  and first/last records across the 4-KiB bank boundary, last valid handle1364 and
  rejection of declared handle1365.
- Ten target controls check cold creation, sticky exhaustion, exclusive cursors,
  rejected-consumer lease preservation, invalid interval/order, active-set/note
  capacity errors and stale mapping epochs.
- Each case executes on three backends: **171 executions**. Five transport
  rejection cases prevent truncated fields, over-capacity input and output limits.
- Independent expectations check demand: cold factory performs no payload reads,
  visits no rows and yields nothing; zero demand performs eager validation only;
  normal consumption yields exactly the Ruby prefix and visits exactly its
  boundary rows. Shared engine agreement is an additional check.

The supported paired input is onset-ordered within each source, row0..65535,
pitch0..127 and source0..255. Original note-ID ordering is retained. This does
not claim that an arbitrary MIDI file fits the profile or that a Chronos excerpt
is the entire song.

The subsequent [whole Chronos gate](chronos/README.md) admits a checked three-bank
profile and compares all1,485 paired notes and all three flatten channels of
that song, including third-page and1,024-event output capacity controls.

## Native memory and timing

All native support functions, lifecycle controls, a bounded128-entry output sink
and compiler-generated static spills are included in the linked images:

| Configuration | Note capacity | Code | MIR globals | Static spills | Linked image | Observed stack below entry |
|---|---:|---:|---:|---:|---:|---:|
| One 4-KiB bank | 682 records | 6,923 B | 6,444 B | 158 B | 13,525 B | 74 B |
| Two 4-KiB banks | 1,364 records | 6,923 B | 10,540 B | 158 B | 17,621 B | 80 B |

The active set retains the original512-handle limit;513 simultaneous notes fail
explicitly. Objects occupy six bytes and never straddle a page; four padding
bytes per bank remain protected. Output capacity is explicit, and consumption
limits cannot exceed it. Configuration anchors are checked rather than blindly
patching a changed source file.

The overlap/zero-length poly fixture emits8 events in892,341 T-states. The
Chronos24-note excerpt uses4,768,234 T for poly and4,841,288 T for mono/drum. The
largest boundary fixture takes58,308,647 T. These are emulator observations,
not hardware timing or a realtime guarantee. Accessor/division overhead remains
an optimization opportunity.

Images are linked at0x4000 below a reserved0xe000..0xeffe stack region. Stack
figures are observed peaks over this corpus, excluding the initial two-byte
return sentinel; they are not an all-input stack bound. MIDI decoder/pairing,
VT2 writer, OS/runtime integration and physical bank switching are outside the
measurement. Banked/churn paths simulate translation over resident native RAM.

## Memory and ABI checks

The runner checks return PC/SP, image/stack canaries, byte-identical read-only
note storage and a CPU write heatmap. Writes are allowed only to declared MIR
globals, exact zero DB/DW backend spill declarations and reserved stack bytes.
Restoring a note byte to the same value still fails the write guard; a negative
mutation test demonstrates this. Data execution is rejected.

Crystal binding admission checks actual parameter/results, reachable dependencies
and derived error ABI, not just manifest declarations. Fallible, captured,
aggregate-result, external, memory/IO/unknown-effect selectors are refused. A
mutant actual fallible LANZ function with a lying manifest is rejected before
lowering through a controlled error.

## Reproduce

From the repository root, with Go, Ruby/midilib and Crystal/compiler library:

```sh
python3 research/autosiril-native/verify.py \
  --autosiril /path/to/autosiril --out /tmp/minz-autosiril-native-proof

go -C minzc test ./cmd/autosiril-native -count=1
```

The first command rebuilds tools and writes fresh Ruby input/output, complete
case matrices, generated bounded Nanz, raw/final MIR, assembly/binaries, source
and tool hashes, native write geometry and execution receipts outside the repo.
It reads the reference checkout/MIDI only. The second command replays all171
executions from provenance-bound frozen actual Crystal/Ruby products, plus
transport/ABI/guard mutation negatives, without requiring Ruby/Crystal in CI.
The package is included in the required ordinary CI gate.

The original VM stage file remains unchanged. Bounded profile substitutions
change capacity guards/storage and add the native verification sink/entry; they
do not rewrite the music algorithm. The full VM reference and earlier 66 executions (11 scores × 2 VM modes ×
3 arena modes) remain separate evidence with their original capacities and scope.
Historical `first-probe.*` artifacts describe the earlier single-case experiment;
its smaller image/control scope is not the current gate.

For the later sequential file utilities, see [common transports](transports/),
[common CP/M flatten](stream-flatten/) and [native TRD flatten](trd-flatten/).
The TRD path uses two independent images joined by saved disk bytes, with
bounded one-pass append and explicit catalog/system failure-state checks.
