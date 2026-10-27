# Next native milestone: streaming EVENTS.E → LNote/ornaments

Status: **one complete Chronos positive case passes raw/final MIR2 and guarded
native Scorpion Z80** in `spike/autosiril-native-arrange`. No full
malformed/resource/IO matrix, native mutation suite, CP/M adaptation or required
CI integration is claimed yet. PR #267 supplied the preceding TRD stage on main
cbd7cb220. Its saved native output disk is this program's actual input.

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

Implemented guards in the initial profile (full N/N+1 validation still pending): at most512 incoming members in one cell (matching the upstream active-handle bound),128 pitch-set entries, orn_repeat1..32,64 interned definitions including ornament0, and4096 pooled ornament bytes. Resource exhaustion must be explicit and tested at N/N+1. The existing2046-note/512-active/1024-events-per-channel producer capacities remain unchanged.

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
