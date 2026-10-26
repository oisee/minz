# Native streaming MIDI → compact paired notes (first prototype)

The shared `stdlib/midi/smf.nanz` decoder, bounded Nanz note pairing and the
actual native Crystal `crystal_quantize(UInt32, UInt32, UInt32)` kernel execute
on Z80. The host transports chunks and compares results; it performs no MIDI
parsing, pairing or quantization. Target code stores six-byte records in a
three-page arena (2,046-note capacity), with a stable512-entry active list that
is intended to retain FIFO order for repeated channel/pitch notes.

## First executed checkpoint

A fresh Crystal export and a fresh original Ruby Setup/VNote/midilib oracle
agree on **all1,485 Chronos paired notes**, two sources,2,984 events and9,058
accepted bytes. Chunk sizes1/7/256 each execute on raw MIR2, final MIR2 and Z80:
**nine executions**. Peak active count is2 and final active count is0.

The linked image is29,990 bytes, including14,827 bytes of declared MIR globals;
the remaining15,163 bytes include code and backend static spills. Observed
below-entry stack is58 bytes, excluding the two-byte return sentinel. Native
costs are435,310,507 /432,593,107 /432,152,807 T-states for chunks1/7/256,
including target boot/feed/finish calls. These are emulator measurements,
not realtime or physical hardware claims.

Only a256-byte input chunk is resident on target. The complete MIDI file is not
stored there. The512-byte payload buffer is caller-owned, and the note arena
uses the same six-byte records/page padding as the separate flatten stage.

The native probe checks image/stack canaries, PC/SP, no port I/O, write heatmaps,
exact compiler spill declarations, no execution of declared data, unchanged
transport input and protected page padding. The u16 row guard runs before the
Crystal UInt32 quantizer can wrap. The actual binding is checked against the
manifest, scalar ABI, effects and derived error ABI.

## Reproduce

```sh
python3 research/autosiril-native/midi/verify.py \
  --autosiril /path/to/autosiril --out /tmp/minz-native-midi-proof
```

The verifier rebuilds the actual Crystal exporter and Go runner, reads the
reference checkout/MIDI without writing there, and writes generated assembly,
binary, raw/final MIR, oracle products, source/tool hashes and receipts outside
the checkout. Run with Go, Ruby/midilib and Crystal/compiler library installed.

This is a research checkpoint. The duplicate/channel/EOT/error/capacity corpus,
CI replay and guard mutation tests are still pending; Chronos alone does not
establish all pairing edge cases. Hard limits are64 tracks,512 active notes,
2,046 records,512-byte meta/SysEx payloads and u16 quantized rows. The prototype
currently fixes eight rows per beat. Diagnostics and the runner API are not yet
the production compiler interface.

This executable ends at paired records. The separately checked flatten image
is not linked here: separately fitting stages do not prove one combined image
or a complete MIDI→VT2 converter. No OS/disk transport, physical bank switching,
downstream LNote/echo/mix/pattern/VT2 writer or general native Autosiril claim.
