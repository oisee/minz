# Native streaming MIDI → compact paired notes

The shared `stdlib/midi/smf.nanz` decoder, bounded Nanz note pairing and actual
native Crystal `crystal_quantize(UInt32, UInt32, UInt32)` kernel execute as Z80
machine code. The Go runner transports chunks and checks results; parsing,
channel/pitch matching, FIFO pairing, EOT closure, quantization and arena writes
are all target work. No target musical host callback or GC is used.

## Executed contract

A fresh Crystal export and fresh original Ruby Setup/VNote/midilib oracle agree
on every one of the1,485 Chronos paired notes: two sources,2,984 events and9,058
accepted bytes. The wider gate contains18 Ruby-backed wire cases and20 explicit
target error/resource controls. All38 cases execute with1/7/256-byte chunks on
raw MIR2, final MIR2 and assembled Z80: **342 executions**.

Valid cases exercise duplicate-pitch FIFO, separate MIDI channels, stable active
list compaction, velocity-zero and unmatched releases, running status, skipped
empty tracks/source IDs, inherited-delta EOT closure, half-row rounding, header
extensions, short channel messages, exact512-byte payload, last u16 row,512
active notes,2,046 records and64 nonempty tracks. Controls check malformed and
truncated bytes, decoder tick overflow, first invalid row/release, pre-quantizer
wrap rejection, excess payload/active/notes/tracks and invalid rows-per-beat.
Two EOT controls check partially closed notes before row overflow and synthetic
UInt32 tick overflow. Faults preserve the explicitly checked partial state;
failed parses must not be passed downstream as successful paired input.

The supported profile is SMF0/1, PPQN1..32767,1..256 rows per beat, at most64
tracks,512 active notes,2,046 total records and512-byte meta/SysEx payloads.
Quantized rows must fit u16. A larger timestamp is rejected before the UInt32
Crystal calculation can wrap. Source IDs are assigned to nonempty tracks in
file order; repeated channel/pitch notes close in onset FIFO order.

## Memory and cost

Only a256-byte MIDI input chunk resides on target. Three4-KiB resident pages
hold six-byte `start:u16, off:u16, pitch:u8, source:u8` records (little-endian).
Each page holds682 records and four protected padding bytes. IDs are1-based.
This is the same packed format as the independently checked flatten stage.
The active list stores512 u16 IDs and512 channel bytes, avoiding the VM port's
large track×channel×pitch tables.

| Linked storage | Bytes |
|---|---:|
| Code, including decoder and UInt32 arithmetic helpers |14,646|
| Declared MIR globals, including complete arena/workspace |14,827|
| Exact backend static spill declarations |517|
| **Complete linked image** |**29,990**|

Observed below-entry stack is58 bytes on Chronos and62 bytes over the complete
corpus, excluding the two-byte return sentinel. These are measured corpus peaks,
not an all-input stack bound. Image origin is0x4000, below a reserved0xe000..0xeffe
stack region. Native Chronos costs including target boot/feed/finish calls:

| Input chunk | T-states |
|---|---:|
|1 byte |435,310,507|
|7 bytes |432,593,107|
|256 bytes |432,152,807|

These are emulator measurements, not realtime or physical hardware claims.
The full MIDI file is retained by transport, outside target memory.

## Import and memory checks

The binding checks the manifest, actual scalar ABI, effects and derived error
ABI. `hir.VerifyClosedNames` checks the selected isolated export before linking:
unknown reads, implicit writes and address references cannot capture parent
Nanz globals. Explicit locals/parameters remain valid; ordinary HIR verification
keeps Nanz's previous implicit-local behavior. Actual-Lanz read/write/address
mutations and a lying fallible manifest are rejected during admission. The same
closed-name check also repairs the existing flatten selector bridge.

The native runner checks return PC/SP, image/stack canaries, no port I/O, write
heatmaps, exact compiler spill declarations and no execution of declared data.
Transport fields/chunk and arena padding are protected. Restoring an input byte
or padding byte to its unchanged value still fails; two mutation tests verify
that the write guard observes the write itself. Every backend's full records
and independent fault/source/active/peak/consumption/event expectations are
checked, then all backend states are compared. Receipts store record hashes;
the provenance-bound case matrix retains all expected records.

## Reproduce

```sh
python3 research/autosiril-native/midi/verify.py \
  --autosiril /path/to/autosiril --out /tmp/minz-native-midi-proof

go -C minzc test ./cmd/autosiril-midi-native -count=1
```

The fresh gate rebuilds the actual Crystal exporter and Go runner and executes
the original Ruby classes. It reads the reference checkout/MIDI without writing
there. Assembly/binary, raw/final MIR, oracle products, source/tool hashes and
execution receipts go outside the checkout. It needs Go, Ruby/midilib and
Crystal/compiler library. `--refresh-fixtures` explicitly updates the frozen
reference products after the complete fresh gate passes.

The Go test replays all342 frozen executions and transport/ABI/write-guard
negatives without Ruby/Crystal tools. It is included in the required ordinary
CI gate. Evidence is under `evidence/`, reference products under `fixtures/`.

This executable ends at paired records. The separate flatten image is not
linked here: separately fitting stages do not prove one combined image or a
complete MIDI→VT2 converter. OS/disk transport, physical bank switching and
later LNote/echo/mix/pattern/VT2 output remain separate work.
