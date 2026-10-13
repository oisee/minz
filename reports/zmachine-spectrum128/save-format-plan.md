# Native save format: measured budget and required restore state

2026-10-13. This is an implementation plan supported by real host-VM saves,
not an implemented native SAVE/RESTORE feature.

The compressed original dynamic image needed by restart occupies 9172 bytes.
Remaining bank tail: 2204 bytes. Resident headroom before stack: 1461 bytes.
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
