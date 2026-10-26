# Binary streams → Nanz → HIR → Z80

Experiment on `spike/binary-stream-fsm`, based on the completed Crystal/HIR
work at `f122b2c77`. This stage uses the Nanz frontend and the existing HIR;
Crystal does not yet expose this decoder through its bridge.

## What actually runs

- `stdlib/stream/bit_fields.nanz`: a Nanz **meta function**, executed by the
  MIR2 meta VM, which generates a struct and concrete MSB-first masks/shifts.
  Fields are 1–8 bits, total width is 8 or 16 bits, at most eight fields.
  Field names must be quoted strings; block nodes use newlines.
- `stdlib/stream/packet.nanz`: an incremental bounded decoder for the example
  protocol `prefix:u8, length:u16 little endian, payload:length`. It accepts
  borrowed chunks or drains a `ByteRing`, stops at the first pending packet,
  and pops exactly the accepted bytes from the ring.
- `stdlib/midi/smf.nanz`: an incremental Standard MIDI File decoder for
  format 0/1 with PPQN timing. Channel messages, running status, meta events,
  F0/F7 SysEx segments, big-endian lengths, VLQs, track boundaries and EOT.
- `midi_notes.nanz`: a real consumer, counting note-ons and note-offs, including
  velocity-zero note-ons. The example itself is tested through both MIR2 VMs
  and assembled Z80 execution.

```nanz
import stream.bit_fields
@bit_fields("Header") {
    bits "kind" 3
    bits "flags" 5
}
// Generated: Header { kind:u8, flags:u8 }
// Header_decode(raw:u16, dest:^Header)
```

This is a small bit-schema DSL and handwritten reusable state machines, not a
complete Erlang bit syntax implementation or a generic state-machine DSL.
There is no OTP runtime, scheduler, GC or actor system in this experiment.

## SMF interface and ownership

Import `mem.bytes` and `midi.smf`. Allocate a `ByteBuffer` and its backing bytes,
then call `decoder.init(&buffer)` on a `SmfDecoder`.

`decoder.feed_byte(byte,&event) -> u16` encodes **consumed** in the high byte
(0 or 1) and `SmfStatus` in the low byte:

| Status | Value | Meaning |
|---|---:|---|
| MORE | 0 | Accepted input, no complete event yet |
| EVENT | 1 | One complete `MidiEvent` is available |
| BLOCKED | 2 | Previous event has not been acknowledged; consumed = 0 |
| ERROR | 3 | Sticky error; inspect `decoder.error` |
| DONE | 4 | Successful EOF from `finish`, or empty chunk after final EOT |

`decoder.feed(&view,&event,&report)` accepts `ByteView` chunks, returning a
status and `SmfReport {consumed:u16,status:u8,error:u8}`. It stops exactly at an
event; resume with the unconsumed suffix after acknowledging. Empty chunks
never mean EOF. `finish()` reports EVENT while output is pending, DONE only
after all tracks' EOTs have been acknowledged, otherwise a sticky truncation.
A nonempty feed after final EOT reports trailing data.

An event holds zero-based track index, absolute tick within that track, delta,
full wire status (including channel), data1/data2, meta type and payload length.
Channel payload length is zero. Meta/SysEx payload bytes are copied into the
supplied buffer and leased until `ack()`; F0 payload excludes the leading F0
status and includes a terminal F7 when present. F7 segments remain separate.
The decoder preserves wire note-on-with-zero-velocity; the consumer may treat
it as note-off. Events arrive in **file track order**, not merged playback order.

Caller-owned descriptors, event output and input must be separate from the
payload backing memory and remain alive. `ack()` clears logical buffer length;
it does not erase bytes. `init()` resets sticky errors. No allocation happens
inside the decoder. Z80 state is 44 bytes, event 18 bytes, buffer descriptor 6
bytes: 132 bytes with the example's 64-byte payload, excluding consumer
counters, stack and compiler spills. The differential harness uses 512-byte
payload storage, not a whole-file buffer.

Deliberate limits: no SMF format 2, SMPTE division, RIFF/RMID wrapper, real-time
MIDI wire protocol, track merging, musical playback, or semantic validation of
every known meta payload. Oversize payloads are rejected before any payload
write; they are not streamed out in fragments. A caller can choose another
capacity. Four Autosiril fixtures need at most 44 payload bytes per event.

## Autosiril differential results

Source files were read from `oisee/autosiril`'s `test` directory; MIDI assets are
not copied into this repository. `midi_oracle.rb` subclasses **midilib 4.0.2's
MIDIFile parser**, capturing raw callbacks before SeqReader normalization or
synthetic note-off generation. The Nanz implementation shares no parsing code
with that oracle. Every event's track, tick, delta, status, data bytes, meta type
and complete payload is compared, as are format/tracks/division.

| Fixture | Bytes | Wire events | Max tick | Max payload |
|---|---:|---:|---:|---:|
| chronos.mid | 9,058 | 2,984 | 20,101 | 8 |
| flim.mid | 10,267 | 2,360 | 99,690 | 11 |
| imrav.mid | 52,581 | 14,744 | 228,820 | 40 |
| tottoro_example.mid | 29,273 | 8,636 | 225,840 | 44 |
| Total | 101,179 | 28,724 | | |

All 28,724 events match in raw MIR2, optimized MIR2 and assembled Z80 running
in the emulator: **86,172 event comparisons**, plus a blocked-output probe at
every event in every engine. File bytes are supplied incrementally; the largest
file is not loaded into Z80 RAM. This is a decoder test, not audio playback or
a hardware timing measurement. The complete test harness compiles to 10,005
bytes of Z80 code/static data including both input/payload buffers and probes.

Further default tests cover:

- 557 generated bit-field results across all three engines; eight invalid schemas.
- All 128 partitions of an eight-byte two-packet stream, empty chunks, output
  backpressure, explicit EOF, reset and guarded payload storage.
- Ring input at all eight possible head positions, including wrap and unread
  next-frame retention; six generic packet EOF/oversize cases.
- SMF chunk sizes 1, 2, 3, 7, 17, 64, 256; running one-byte/two-byte messages,
  meta and SysEx payloads, empty chunks and unread output.
- 49 malformed/EOF cases per engine: invalid headers, unsupported formats,
  canceled running status, bad data, long VLQs, track/payload bounds, malformed
  EOT, truncation, trailing bytes and tick overflow.
- Four-byte deltas accumulating to **4,294,967,295** ticks without narrowing;
  the next overflowing event is rejected.

## Reproduce

From the repository root:

```sh
# Default synthetic, native and meta coverage; no Ruby or MIDI assets required.
go -C minzc test ./pkg/nanz -run '^(TestBitFieldMetaNative|TestPacketDecoder.*Native|TestSMF.*Native)$' -count=1 -v

# External corpus; requires Ruby with midilib 4.0.2 installed.
AUTOSIRIL_MIDI_DIR=/path/to/autosiril/test \
  go -C minzc test ./pkg/nanz -run '^TestSMFAutosiril$' -count=1 -v
```

Fixture SHA-256:

```text
chronos.mid         0d4c0289fe42c81ea01c9715c9d382807e97549161c419c737e15f18d6abc02f
flim.mid            8b489096c2d310978376047e912813ccc61ae7b0c860217b4fbecb1f4b07d061
imrav.mid           12427da8d94142b1536a32af0ffcd95cfe01ce75aad080d20d09d84621ec5f72
tottoro_example.mid be75cc2f53744e89c90feda21bc97ad0af2b51c1a30ea56ce2884bdc7f3223ff
```

## What improved in the compiler

The example exposed three general bugs, fixed before adding the libraries:
local explicitly typed struct pointers lost field/method metadata; imported
simple enums lost their member tables; HIR splitting omitted `&local` symbol
dependencies. Splitting also now refuses scalar-cell/array interfaces whose
address identity cannot be preserved by value forwarding. Struct storage can
continue to use the existing addressable-parameter ABI. Focused regressions
cover these changes. Payload ADT import metadata is outside the simple-enum fix.

The reusable contribution is compile-time bit-schema generation plus bounded
stream handling built on existing `ByteView`, `ByteBuffer`, `ByteRing` and HIR.
Other frontends can target the same representation; exposing a pleasant Crystal
or Nanz state-machine DSL is a follow-up, not a completed language feature.
