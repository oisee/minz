# Two native TRD utilities: MIDI → NOTES.N → EVENTS.E

The second utility binds the existing common ASIRv1 note reader/event writer
and unchanged `lazy_flatten` to TRD sector I/O. The two actual Crystal selectors
still come from the closed typed bridge. The first native Z80 program saves
`NOTES.N`; a separately linked second image reads that saved TRD and appends
`EVENTS.E`. Host callbacks provide sector I/O, not note transfer or musical logic.

## One pass, including unknown-length output

A lazy generator does not know its event count when opening the output file.
`append.nanz` writes forward in the free tail and publishes the actual logical
length/sector count after successful exhaustion. There is no counting prepass,
full event array or GC. Capacities remain 2046 notes, 512 active handles and
1024 events per channel.

Each acknowledged chunk advances one sector; a short final chunk is zero padded
and forbids subsequent writes. The writer checks available space, the 255-sector
limit and the 16-bit logical length. It publishes a catalog entry, then updates
the system sector. **These are two separate writes, not a crash-atomic commit.**
A data failure leaves unpublished tail bytes; failure after the catalog write can
leave a visible complete event file with stale free-space accounting. The gate
checks both the returned error and that precise on-disk state.

## Reproducible evidence

The matrix contains 92 cases × three engines (raw MIR2, final MIR2, Scorpion Z80):

- All Chronos notes/events; 21 successful native first-stage output disks are
  consumed directly by the second native program. The gate records separate
  image hashes and the exact intervening disk hash.
- Original independent Ruby framing/version/count/EOF and N/N+1 resource
  fixtures, adapted only for exact TRD logical lengths instead of CP/M padding.
- Directory lengths/extents, duplicate/missing names, malformed geometry,
  overlapping free tail, catalog slots 15/16/127 and full directory.
- Last free sector, full disk sentinel, exactly 51 Chronos output sectors and
  exhaustion after 50 successful writes.
- Failures at each of the eight single-note sector operations, including all
  catalog/system reads and writes; first/middle/last Chronos DATA write failures.

Every case checks status, publication, free-space state, admitted disk changes,
consecutive successful data writes and flush/reload equality. Engine traces and
resulting disk/event hashes must agree. Loaded notes are checked individually;
successful generator yield counts must equal the independent event count.
Three actual native mutants must be rejected: broken paging restoration,
257-byte bounce copy, and an arena write followed by restoration after close.

Fresh verification regenerates the original Ruby/Crystal references and retains
the preceding native MIDI, file, transport and common-flatten gates. CI replays
those frozen independent vectors without requiring Ruby or Crystal.

```sh
# Fresh semantic provenance and all preceding file/transport gates:
python3 research/autosiril-native/trd-flatten/verify.py \
  --autosiril /path/to/autosiril --out /tmp/trd-flatten-fresh

# Frozen CI, after the existing file-chain and first-transport gates:
python3 research/autosiril-native/trd-flatten/replay.py \
  --reference /tmp/autosiril-file-chain \
  --producer /tmp/autosiril-transports --out /tmp/trd-flatten-ci
```

`verify.py --refresh-evidence` saves the fresh receipt in this directory.
`--reuse-reference-root /tmp/previous-common-flatten-fresh` explicitly reuses a
completed fresh upstream run after checking repository, Ruby and MIDI hashes;
it records that reuse. The default always regenerates the references.
Outputs must be outside checkout in an empty directory. Receipts, per-engine
saved disks/events, raw/final MIR2, compiler assembly, placed assembly and linked
native binaries are retained; CI uploads `autosiril-trd-flatten`.

## Memory and Chronos budget

The observed image is 13248 bytes: 12844 code/stub, 126 loaded globals and
278 static spills. Separate reservations are 2048 fixed workspace bytes,
12288 arena bytes in bank0, and the whole 16384-byte bank8 bounce region
(256 used). The sum is 43968 bytes, plus stack/runtime reservations.

Fixed workspace is 4000..47FF; code/data image is 6000..93BF and stack is
B800..BFFF. The arena remains immutable after input close. Native guards track
all 16 physical banks, fixed writes, PC/stack, paging ports, IRQ policy and
mapping restoration after every disk call. The stub copies through fixed
`input_chunk`, restores bank0 and returns with interrupts disabled; no borrowed
near pointer survives a bank switch or stage boundary. Research EQU placement
admits zero-initialized arrays; this is not a new core `Global.At` relocation.
MIR2 OpStore observation separately checks arena writes/padding, not host memcpy.

Chronos produces 2153 events from 1485 notes/two sources, 12939 logical bytes,
51 output sectors and 98 sector calls. The observed kernel costs 450683266
Z80 T-states, excluding ROM/disk latency; maximum stack below entry is 65 bytes,
excluding the incoming two-byte return. These figures describe the positive
Chronos path; per-case measurements live in the receipt.

## Scope

This is the emulator's actual MIT TRD **sector API trap** with guarded Scorpion
memory, not physical TR-DOS ROM/FDC/hardware execution or a bootable disk.
The independently linked utilities still run under the verifier's entry/return
contract. Full MIDI→VT2/VTX, MOS/eZ80 transport and production command packaging
remain future stages. The separate in-process 40626-byte experiment is retained,
not treated as this disk layout.

A boundary case also exposed and fixed `TRDFile.ReadFile`: its logical track was
modified while reading side1, causing subsequent sectors to come from side0.
A focused Go regression covers crossing the side boundary, continuing on side1,
and the final disk sectors.
