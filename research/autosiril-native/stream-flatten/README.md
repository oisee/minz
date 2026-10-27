# Both sequential file stages through the common chunk API

The second utility now uses the same `read_chunk/write_chunk/finish` contract
as the [interchangeable MIDI producer](../transports/). Its target-side ASIR
reader and event serializer contain no BDOS calls. The original shared
`lazy_flatten.nanz` generator, 2046-note/512-active bounds and real Crystal
`pick_slot`/`cell_action` roots remain unchanged.

`compose.py` reuses the existing closed flatten bridge and verifier, takes
the **same writer functions** from `transports/pair-file.nanz`, and binds the
**same CP/M implementation** from `transports/cpm.nanz`. Composition changes
only filename constants; the binding supplies input-padding metadata. It does not fork
the record-I/O implementation or musical algorithm.

## A chunk boundary is not a physical record boundary

The first adapter worked for CP/M's normal128-byte reads but a7-byte progress
adapter exposed a bug: valid zeros after END were mistaken for an extra record.
The common reader now counts consumed wire bytes. With a128-byte padding
quantum, it checks zeros up to the physical boundary even if they arrive through
multiple short progress chunks. It then requires explicit EOF; a further record
is rejected even if all its bytes are zero. A binding with quantum0 instead
requires exact logical EOF, appropriate for TRD directory lengths.

The quantum is explicit input metadata supplied by the binding, separate from
the writer's output chunk capacity. Invalid capacities and padding metadata
are rejected before reads/output creation. Empty progress retries are bounded;
EOF with a nonzero count, unknown status, oversized reads and short writes fail.

The existing ASIR v1 checks remain: version/kind/width/flags/reserved metadata,
source and note capacities, note fields, DATA tags, END count/signature, physical
padding, truncation, close/write errors and generator overflow. Each channel
pulls up to1025 events to reject overflow beyond1024 and allow generator cleanup
at exact capacity. Three channels are consumed sequentially; no full event array
is materialized by the target.

## Verification

The common reader/writer is checked against the original independent corpus:

- **201 executions**: 67 real producer, resource, framing and OS-error fixtures
  on raw/final MIR2 and native Z80. Every2153 Chronos event is compared.
- **30 transport-contract executions**: progress chunks of1 and7 bytes, empty
  progress, stalls, EOF with bytes, oversized reads, unknown status, short write,
  zero and oversized output capacities.
- **9 directory-success executions** verify normalized CP/M2.2 success codes
  1/2/3 for open/create/close on all three engines, using the shared binding.
- The original **three native negative mutants** still fail: a restoring write
  to the input arena, bypassed note capacity and removed END signature check.
- Two separate **common** `.COM` programs execute through ordinary mze. Only
  files persist from MIDI pairing to flatten; notes/events are compared byte for
  byte against the independently executed native probes.

The fresh verifier regenerates Ruby/Crystal references and runs the unchanged
original CP/M chain and interchangeable-transport gates before the new stage.
Frozen CI takes the already verified reference/producer outputs and validates
source, matrix, generated source/driver and complete standalone output hashes.
It does not infer expectations from the modified target.

## Budget of the common CP/M flatten image

| Item | Measured |
|---|---:|
| Loaded image | 24247 B |
| Code/bootstrap | 9525 B |
| Globals, including12288-byte note arena and256-byte input buffer | 14517 B |
| Static spills | 205 B |
| External target data | 0 |
| Maximum stack below entry | 65 B |
| Chronos kernel | 449872550 T-states |
| Ordinary mze standalone | 449872581 T-states |

Stack excludes the incoming two-byte return address. Chronos performs83 BDOS
reads (82 records plus EOF),102 writes, one input open/output create/DMA setup
and two closes. It emits2153 events:12939 logical bytes,13056 stored bytes.
The original special-purpose flatten image was22750 bytes and446711944
standalone T-states. The shared API/padding accounting adds1497 image bytes
and about0.7% runtime in this particular offline conversion. No real disk latency
is included, and this does not claim a real-time MIDI converter.

## Reproduce

```sh
python3 research/autosiril-native/stream-flatten/verify.py \
  --autosiril /path/to/autosiril --out /tmp/minz-common-flatten-fresh \
  --refresh-evidence
```

Requires Go, Ruby and Crystal. On macOS an optional `--go-overlay` can supply
the local mze tick-input portability change without committing user files. Use
a fresh output directory outside the checkout. After a composition-only fix,
`--reuse-reference-root /path/to/previous-fresh-output` can reuse completed
independent reference/producer gates: their recorded source hashes, original
Ruby source and MIDI digest are checked. The new common-stage tests still run
in full; the receipt explicitly records reference reuse.

CI uses Go/Python and prior completed gate directories:

```sh
python3 research/autosiril-native/stream-flatten/replay.py \
  --reference /tmp/autosiril-file-chain \
  --producer /tmp/autosiril-transports \
  --mze /path/to/mze --out /tmp/autosiril-common-flatten
```

Assembly, binary, raw/final MIR2, individual cases, mutants, receipts and both
standalone programs/files are retained outside checkout and uploaded by CI as
`autosiril-common-flatten`.

## Scope

CP/M measurements use mze's host BDOS services, not a resident Z80 BDOS/BIOS
image. The research bootstrap sets SP=`EFFE`, guards a kernel stack floor at
`E000`, and stops with HALT/HL status. It does not derive the TPA top from a
physical CP/M installation or provide a portable return to CCP. Stack/cycle
figures measure the target kernel, not a physical OS's internal service costs.

Under native CP/M2.2 the output filenames must initially be absent; BDOS22 is
make-file, not a portable overwrite operation. mze host-file truncation does
not establish overwrite semantics on another CP/M implementation.

This change connects the second stage to the common interface using the proven
CP/M binding. It does not yet provide a TRD flatten utility: that adapter also
needs an output allocation/length policy for a stream whose final count is not
known until generator exhaustion. A two-pass count or checked bounded append
reservation can solve that separately without host musical logic or a full
event array. No physical TR-DOS ROM, overlay loader, MOS file implementation or
complete native VT2/VTX converter is implied. The separate original CP/M stages,
old342/171/21 oracles and40626-byte in-process image remain available.
