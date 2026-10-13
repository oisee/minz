# Native save format: measured budget and required restore state

2026-10-13. This is an implementation plan supported by real host-VM saves,
not an implemented native SAVE/RESTORE feature.

The compressed original dynamic image needed by restart occupies 9172 bytes.
Remaining bank tail: 2204 bytes. Resident headroom before stack: 1205 bytes after the native RLE preflight.
Three compact 32-frame/256-word VM saves were sampled:

| Route | Compressed state | Compressed XOR dynamic | Total with lengths |
|---|---:|---:|---:|
| Initial |135 |318 |457 |
| Mailbox/leaflet |213 |407 |624 |
| Cellar/lamp/sword |278 |469 |751 |

Full decoded state is 1786 bytes; dynamic memory 11288 bytes. XOR uses the exact
original story prefix, not current memory. Encoded blobs were decoded and compared
with the entire saved state and dynamic bytes. This proves these samples fit,
not a full-game compression bound. Different games may not fit this tail.

First native format should include story identity (release/serial/checksum/static
boundary), format/version, configured capacities, decoded/encoded lengths and
integrity checks, in addition to payload. Measure that complete format before
claiming fit. Require failure on capacity overflow; never truncate. Validate all
encoded runs, identities and bounds before modifying live state. RAM save remains
volatile unless the emulator snapshot or a later disk backend preserves it.

Restore must return PC, frame, stack depth, locals, return metadata, evaluation
stack, RNG, screen-enabled flag, stream3 depth/table pointers. Preserve current
flags2 bytes 16/17 as required by Z3. Review which interpreter I/O state is outside
SAVE scope and document it. Do not copy transient decoder/native scratch state.

The existing host restore serializes RNG/screen/stream3 metadata at snapshot
bytes 22..57 but does not restore them; native implementation must address this.
Host behavior should get independent regressions before changing the shared core.
For the native format, tests must distinguish RNG continuation and stream3 target
restoration so matching a simple room transcript cannot mask omitted state.

Next: stream compact state serialization into bounded banked storage; derive a
bounded delta codec from original archived bytes; perform a preflight decode
validation, then apply. Exercise SAVE→world mutation→RESTORE and corrupt/foreign
saves with memory unchanged on rejection. Keep original/archive bytes unchanged.

Probe command (run from repository root):

```sh
python3 scripts/zmachine/probe_save_delta.py /path/to/mzv /path/to/story.z3 /path/to/output
```

## Bounded reference record (implementation checkpoint)

`scripts/zmachine/save_format.py` now defines a 32-byte header, followed by
RLE state and RLE XOR dynamic. This remains a host reference, not native SAVE.
All integers are little endian except story identity copied verbatim:

| Offset | Bytes | Field |
|---:|---:|---|
|0|4|MZ3R magic|
|4|1|Format version 1|
|5|1|Configured frame count|
|6|2|Configured stack word count|
|8|2|Dynamic memory decoded bytes|
|10|2|State decoded bytes|
|12|2|State encoded bytes|
|14|2|Dynamic delta encoded bytes|
|16|10|Story release (2), serial (6), checksum (2)|
|26|4|Story file bytes|
|30|2|CRC-16/CCITT-FALSE over header 0..29 and payload|

CRC detects accidental corruption, not malicious modification. Identity follows
release/serial/checksum plus file size and dynamic boundary; it is not a
cryptographic hash of the complete story. Exact record length is mandatory.
Decoded state validates PC, active frame/return PCs, stack depths, stream flags
and table bounds before it can be applied. Current flags2 must be preserved by
the future native apply step. Reference codec returns separate decoded buffers;
it never applies them to live interpreter memory.

Measured complete records: initial 485 B, mailbox 652 B, cellar 779 B. These
three real saves decode byte-for-byte, including all RNG and stream3 fields.
Six reference tests cover every single-bit record corruption, every truncation,
trailing input, foreign identity/configuration, bounded capacity failure,
malformed RLE with recomputed CRC, and invalid decoded state. Native integration,
transactional slot publication, and actual world/RNG/stream restoration remain.
