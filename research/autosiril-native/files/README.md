# Native file stages: first CP/M producer

This research step implements **MIDI file → paired-note file**. The next native
file reader/flatten utility and the Spectrum TRD transport remain pending. The
separate linked prototype in `../linked` does not substitute for those utilities.

`compose.py` appends `pair-cpm.nanz` to the existing, unchanged `../midi/pair.nanz`.
The target performs open/read/close/create/write/close through fixed-address
`CALL 5` with C/DE arguments and A status. It uses the shared incremental SMF
decoder and the actual closed scalar Crystal quantizer. The host supplies OS
record I/O, not musical computation or a precomputed note array.

The generated standalone `program.com` uses `INPUT.MID`, produces `NOTES.BIN`,
and currently boots with eight rows per beat. The matrix enters the same target
entry with each fixture's configured beat. It checks raw MIR2, final MIR2 and
native Z80 independently against original Ruby-derived note records. The Chronos
`.COM` is also executed by the ordinary `mze -t cpm`, using its actual BDOS path;
its entire output must equal the profiled native output.

Run from the repository root:

```sh
python3 research/autosiril-native/files/verify.py \
  --autosiril /path/to/autosiril --out /tmp/autosiril-file-proof
```

The gate first reruns the prior fresh Crystal/Ruby MIDI matrix (342 executions).
It then runs 49 disk cases across three engines (147 executions): the 38 adapted
MIDI cases, three additional Ruby-backed boundary/data cases, an empty host file,
six explicit OS failures, and an extra padding-only record. It tests exact SMF
and output END record boundaries and `0x1a` inside actual MIDI data. The target
retains capacities of 2,046 notes, 512 active notes and 64 source tracks.

The required PR gate also runs `replay.py` against its freshly built `mze`.
This repeats the147 disk executions and standalone Chronos `.COM` without
requiring Ruby/Crystal on the runner. The reconstructed matrix must match the
fresh proof's exact hash: the original38 MIDI fixtures plus11 additional frozen
cases, including the three extra Ruby-derived references. Source changes require
refreshing the fresh proof rather than silently accepting different expectations.

Native checks cover return status, every note, reference counters, arena padding,
immutable configuration, stack floor, memory write/execute spans, canaries and
absence of unexpected port I/O. The source/manifest/generated driver/image hashes
and per-case BDOS call counts are saved with the evidence. Injected OS errors
exercise target status handling; the separate `mze` record tests additionally
exercise partial host writes, FCB repositioning and extent rollover.

On macOS, current main's `cpm_tick_input.go` has an unrelated `syscall.Select`
portability build failure. `--go-overlay /path/to/overlay.json` can supply a local
fix for the standalone `mze` build only; the receipt records that overlay and its
replacement source hashes. It neither modifies nor commits the caller's files.
The existing case-collision directory test also assumes a case-sensitive host
filesystem. Neither issue is presented as a passing full local CP/M test suite.

## Provisional ASIR v1 note stream

All integers are unsigned little endian. These are wire bytes, independent of
native struct layout, pointers and bank padding. The writer and host oracle
currently agree on the representation; freezing a reusable format also requires
the pending target reader and malformed framing tests.

| Byte offset | Meaning |
|---|---|
| 0–3 | ASCII `ASIR` |
| 4 | version 1 |
| 5 | record kind 1: paired note |
| 6 | payload width 6 |
| 7 | flags 0 |
| 8–9 | number of source tracks |
| 10–11 | reserved 0 |

The header is followed by `01 start:u16 off:u16 pitch:u8 source:u8` for each
note in onset/ID order. `ff count:u32 45 4e 44 21` ends the stream, with the exact
number of preceding DATA records and ASCII `END!` after the count. The nonzero
tag and final signature distinguish a missing END/count from zero padding,
including an empty stream. No backpatching is required. Only the remainder of that
last physical 128-byte CP/M record is zero padding. An additional whole physical
record is not padding. The zero-note logical file is 21 bytes; one note is 28.

Input SMF is self-describing: its declared track lengths and final EOT establish
the logical end. After that end, only `0x1a` fill in the remainder of the current
physical record is admitted, followed by physical EOF. This is an explicit
adapter policy; binary `0x1a` before the end is ordinary data. CP/M 2.2 cannot
recover the original sub-record byte length from a short host file. For example,
the truncated-EOT fixture's missing byte is filled with `0x1a`, requesting a
26-byte payload with zero track bytes left: a track-length error (304), whereas
the exact-length wire adapter reports truncation (310). Both reject the input.

Output failures return nonzero status and can leave a partial output file; it
must not be consumed as a successful stage. Atomic publication/rename is not
implemented. The `.COM` bootstrap currently halts with the result in HL, rather
than providing a portable shell exit-code convention. No real CP/M disk image,
TR-DOS ROM, physical paging, MOS file wrapper or complete native VT2/VTX converter
is claimed by this producer proof.
