# Next native milestone: streaming EVENTS.E → LNote/ornaments

Status: both local transport suites pass: **342 TRD executions +6 exact storage
units +4 native mutants;282 CP/M executions +42 chunk contracts +9 directory
success executions +2 native mutants**. Three separate COM programs run via
saved files. The full default fresh run and combined frozen replay pass locally.
Required CI/PR integration is pending. This implementation is on
`spike/autosiril-native-arrange`; it is not yet merged. PR #267 supplied the
preceding TRD stage on main cbd7cb220.

Source facts checked locally:
- Original Ruby autooisee.rb: rmod2pmod705+, pmod2lmod728+, apply_delays781+, downmix877+.
- Existing VM implementation: research/autosiril-vm/engine.nanz build_lnotes375+.
- Current native stream config: sources[1,0,0], modes[m,p,m]; original mapping 2me,1p,1m.
- lazy_flatten.nanz poly branch emits EACH active pitch at the same channel/row. EVENTS.E has not lost chord members. Grouping equal keys is sufficient; do not substitute a mono-only arrangement.

Bounded shape:
1. Read ASIRv1 EVENTS.E, validate header/count/END/exact EOF and monotonic channel/row order.
2. Collect ONE channel/row group (pitch set128 + action + minimum). Duplicate pitches must follow Ruby uniq/sort semantics; mixed action/type in a cell is invalid for this producer.
3. Reuse exact Ruby/VM clipping by upper median, max_offset and orn_repeat; globally intern bounded ornament definitions by handle, not pointer. Preserve discovery order across channel/row traversal. Ornament0 is initial repeated zero.
4. Apply native Crystal keep_offset/ornament_shift kernels through audited actual typed HIR, config sample/ornament and existing drum/envelope tables. All LNote fields and centi-volume1500 must match original Ruby pmod2lmod. m/p/m Chronos includes channel0 envelope mode; do not drop it.
5. Stream each arranged record to a sink and compare all LNote/ornament fields with a fresh original Ruby result. Avoid l_grid/l_words/song-sized arrays from the VM prototype. Explicit N/N+1 rejection for row-group, ornament count/bytes/repeat and record bounds; preserve stream cleanup.
6. Prove raw/final MIR2 and assembled Z80 on full Chronos plus chord/median/clip/duplicate/release/envelope/drum edge fixtures. Report code/data/spills/buffers/stack/I/O/T-states.

The [kind3 arrangement format](format.md) uses fixed16-byte tagged payloads for LNotes and ornament fragments. Definitions are emitted at discovery, before the discovering cell. END counts both roles. Kind2 remains the unchanged input event format.
Echo/mix follow after LNote semantics and bounded sink are verified. Streaming echo also needs collision priority and delayed horizon from actual Ruby, not just shifting note-on times.

Fresh current Chronos oracle measurements:
{
  "flatten_events_per_channel": [
    771,
    719,
    663
  ],
  "lnote_records_per_channel": [
    771,
    663,
    663
  ],
  "max_group_notes": 6,
  "ornaments": [
    [
      0,
      0
    ],
    [
      0,
      0,
      3,
      3
    ],
    [
      0,
      0,
      2,
      2
    ],
    [
      0,
      0,
      5,
      5
    ],
    [
      0,
      0,
      7,
      7
    ],
    [
      0,
      0,
      1,
      1
    ],
    [
      0,
      0,
      2,
      2,
      3,
      3,
      4,
      4
    ],
    [
      0,
      0,
      1,
      1,
      2,
      2,
      3,
      3,
      4,
      4,
      5,
      5
    ]
  ]
}

Current fresh oracle invocation uses defaults from research/autosiril-vm/oracle.rb: per_beat8, mapping2me,1p,1m, orn_repeat2, max_offset24, delay6/12. Preserve these values when comparing full Chronos (the older explicit verify_chronos script used a different max_offset argument, irrelevant to flatten but relevant to LNotes).

Validated initial profile: at most512 incoming members in one cell (matching the upstream active-handle bound),128 pitch-set entries, orn_repeat1..32,64 interned definitions including ornament0, and4096 pooled ornament bytes. Resource exhaustion must be explicit and tested at N/N+1. The existing2046-note/512-active/1024-events-per-channel producer capacities remain unchanged.

Acceptance: full current Chronos m/p/m mapping, including envelope channel0 and all poly notes, matches2097 LNote records and8 definitions from original Ruby pmod2lmod on raw/final MIR2 and guarded assembled Z80. A lossless, versioned output schema and bounded chunk sink must preserve every LNote field and ornament definition; no full-song grids or native pointers in serialized data. Exercise malformed stream/order/count/EOF and resource boundaries, measure all RAM/stack/code/I/O separately, retain the upstream gates, and deliver implementation through its own PR and required CI. Echo/mix and final VT2/VTX are subsequent stages.

## First native proof and duplex I/O

All2153 incoming events, all2097 arranged LNotes and all8 ornament definitions
match the original full Ruby oracle. The output is35823 logical bytes in140
sectors (including9 definition-fragment records);55 sector reads and142 writes
make197 calls. Reads and writes interleave: `reader_chunk` keeps unread bytes,
`output_chunk` keeps pending output, and `input_chunk` remains the shared
transport scratch buffer. No whole-song array is introduced.

Image11417B =10327 code/stub +839 loaded globals +251 static spills.
External reservations:1536B fixed workspace (1424 array bytes plus holes),4096B
ornament pool in bank0 and16384B bank8 bounce reservation (256 used).
Total33433B plus stack/runtime reservations. Maximum below-entry stack56B,
excluding the incoming two-byte return;138654496 kernel T-states for Chronos,
excluding ROM/disk latency. Workspace4000..45FF, image6000..8C98,
stackB800..BFFF; poolC000..CFFF. Guards retain all16 physical banks,
fixed writes, PC/stack, paging restoration/IRQ policy and port traffic. The
ornament pool is mutable while streaming; writes after input close are rejected.

The proof compares complete wire bytes and resulting disk images across all3
engines, every successful sector operation, consecutive allocation, zero final
padding, catalog/system publication order, free-space accounting, admitted disk
changes and flush/reload. It is one positive fixture via actual sector API traps,
not physical ROM/FDC/hardware, a bootable disk or a complete converter.

Reproduce with an explicitly reused **completed fresh** preceding stage:

```sh
# Only needed when no completed fresh upstream proof is available:
python3 research/autosiril-native/trd-flatten/verify.py \
  --autosiril /path/to/autosiril --out /tmp/arrange-input

python3 research/autosiril-native/arrange/prove_positive.py \
  --upstream /tmp/arrange-input --autosiril /path/to/autosiril \
  --out /tmp/arrange-positive
```

`prove_positive.py` checks upstream source, Ruby/MIDI, full-oracle and saved-disk
hashes, links the actual typed Crystal keep_offset/ornament_shift functions,
and saves source/driver hashes, MIR, assembly, linked binary, TRDs, expected/
actual wire, I/O traces and receipts. `--refresh-evidence` saves the receipt.
The reference encoder serializes Ruby snapshots; it supplies no musical policy
to the target. Target tables and the source Nanz VM algorithm are also hashed.
The script honestly records upstream reuse; fresh/frozen full arrangement gates
remain to be built before this implementation's PR.

## TRD matrix and exact storage bounds

`oracle.rb` calls the actual Ruby `pmod2lmod`; a wrapper only observes return
values of the original `squize_ornament` to record discovery order. The encoder
therefore handles poly+envelope cells whose final kind is `e`, and definitions
which a mono/drum cell discovers but does not select. No reference function
replaces target musical computation.

114 cases × raw/final MIR2/Scorpion Z80 =342 executions. Twenty cases consume
real saved native second-stage disks. Cases cover duplicate/unsorted chord
members, upper-median and exact/over clipping, low/high octave-shift boundaries,
all128 drum pitches with/without envelope, literal samples/ornaments,
row65535,512/513 group members,1024/1025 channel events,repeat1/32 and invalid0/33,
64/65 definitions, framing/order/type/count/END/EOF errors, disk geometry/catalog
boundaries, exhaustion during append and data/catalog/system sector failures.
All engines must agree on complete wires/disks, counters and I/O traces.

At repeat32, a legitimate127-member chord fills the pool to exactly4096 bytes;
128 members request4128 and fail before touching the pool. Storage watermarks
are multiples of repeat, and repeat1 cannot fill4096 under the3072-event stream
bound. To exercise the exact4096/4097 byte comparison as well, six separately
labelled storage-unit executions seed `orn_used`4094/4095 and call the actual
`ornament_find(2)` with normalized candidate[0,1]. They verify status, final
watermark, last valid bytes and the trailing canary. These seeded states are
explicit unit probes, not invented full-song fixtures or reduced capacities.

Four native-only mutants must fail with specific guard messages: broken paging
restore,257-byte bounce copy, pool write followed by restoration after input
close, and config write followed by restoration. Configuration is installed
before profiling and must remain immutable during target execution.

```sh
# Fresh actual Ruby arrangement; explicitly reuses a completed fresh upstream:
python3 research/autosiril-native/arrange/verify_trd.py \
  --upstream /tmp/arrange-input --autosiril /path/to/autosiril \
  --out /tmp/arrange-trd-fresh

# Frozen replay on output of the preceding TRD stage; no Ruby/Crystal needed:
python3 research/autosiril-native/arrange/replay_trd.py \
  --upstream /tmp/autosiril-trd-flatten --out /tmp/arrange-trd-frozen
```

`verify_trd.py --refresh-evidence` saves the source-bound receipt and frozen
case matrix. It validates/replays the positive full Chronos proof first, calls
actual Ruby afresh for every arrangement fixture, and preserves full results.
The upstream MIDI/Crystal run is checked and honestly recorded as reused;
`verify.py` below is the full default fresh gate.

## CP/M and the complete gate

The same core, reader staging and serializer use the unchanged common CP/M
binding with small lifetime wrappers. `EVENTS.BIN` is the input and
`ARRANGE.BIN` the output; both physical padding and short progress refills are
checked. The host OS shim performs file operations only. Make uses exclusive
create, preserving a preexisting output rather than relying on truncation.
All documented directory success codes0..3 are accepted by the common binding.

CP/M make publishes a filename immediately: malformed input after the header,
resource exhaustion, write failure or a failed close can leave an empty,
partial or complete visible file. The matrix checks existence, physical record
count and exact independently expected byte prefixes. Every injected failure
must actually be reached; all FCBs and target lifetime flags must close even
when an earlier error is already set. No rollback/atomic publication is claimed.

94 cases ×3 engines =282 executions, including20 actual saved second-stage
CP/M files, matching all TRD musical fixtures and CP/M-specific framing/OS
contracts.42 additional chunk executions include1/7-byte reads on a multi-record
drum stream (writes interleave while the physical input record is saved),
empty progress, stalled readers, invalid statuses/counts/capacities/padding
metadata and a short write acknowledgment.9 more executions use directory
codes1/2/3. Native-only restoring pool/config writes must be rejected.

The native CP/M image is13646B =7054 code/bootstrap +6416 loaded globals +176
static spills; no external data reservation. Stack56B below entry, excluding
the incoming return; Chronos136737911 kernel T-states,103 physical record reads
(including EOF),280 writes, one open/make/DMA call and two closes. Standalone
runs additionally execute the bootstrap; their logs are retained. SP EFFE,
stack floor E000 and HALT/HL status are the verifier contract, not proven
physical CP/M TPA/CCP bootstrap compatibility. Outputs must initially be absent.

`PAIR.COM`, `FLATTEN.COM` and `ARRANGE.COM` also execute as three independent
mze processes in one directory. Only the first input MIDI and the native images
are installed by the host; subsequent programs consume actual saved files.
Their NOTES/EVENTS/ARRANGE files are compared with the separately checked native
matrix outputs. This third stage still stops at LNotes/ornaments; echo, mix,
tracker patterns and final VT2/VTX output are later stages.

```sh
# Default: regenerate original Ruby/Crystal and ALL preceding native file gates.
python3 research/autosiril-native/arrange/verify.py \
  --autosiril /path/to/autosiril --out /tmp/arrange-fresh

# Frozen required CI: producers have already run in earlier CI steps.
python3 research/autosiril-native/arrange/replay.py \
  --trd-upstream /tmp/autosiril-trd-flatten \
  --cpm-upstream /tmp/autosiril-common-flatten \
  --producer /tmp/autosiril-transports --mze /tmp/mze \
  --out /tmp/arrange-frozen
```

`verify.py --refresh-evidence` saves all source-bound receipts and the frozen
actual-Ruby matrix. Optional `--reuse-upstream` explicitly reuses a completed
fresh TRD flatten proof after source/Ruby/MIDI checks and records that reuse;
the default regenerates it. `--go-overlay` is forwarded when building preceding
standalone tools. `replay.py` needs neither Ruby nor Crystal and compares all
measured outputs/counters/I/O/disk hashes with fresh evidence. CI uploads the
`autosiril-arrange` artifact with source, MIR, placed assembly, binaries, saved
disks/files, failure/mutant logs and standalone receipts.
