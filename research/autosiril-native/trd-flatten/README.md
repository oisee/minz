# Next-stage experiment: one-pass flattened events appended to TRD

**Status: semantic prototype on raw/final MIR2 only.** Native Z80 placement,
Scorpion paging/stack guards, malformed inputs and disk-error matrix are the
next gate. This directory does not supersede the completed native first-stage
TRD proof or claim a full native TRD chain yet.

The unchanged common note reader/event serializer and lazy flatten generator
are bound to the existing target-side TRD directory reader. The two actual
Crystal selectors still come from the closed typed bridge. Host hooks implement
sector I/O only; the input disk is the file image written by the native Z80
MIDI pairing utility.

## Unknown output length does not require a prepass

The first writer can preflight an exact note-file length after pairing. A lazy
flatten generator has not produced its final event count at output creation.
`append.nanz` instead checks directory/free-space availability, writes sectors
forward within the free tail, and publishes actual length/allocation at finish.
Each successful write advances one sector and the acknowledged logical count.

Counts are bounded by the available sectors, the8-bit TRD sector-count field
(at most255 sectors), the16-bit file length, and the existing musical bounds.
A short final chunk is zero padded; another write after that short chunk fails.
Only successful exhaustion publishes the directory and updates free-space
metadata. A write failure leaves unpublished free-tail data; there is no rollback
or crash-atomic two-sector metadata transaction.

This is one pass through the generator, with no target event array or host
musical logic. The reference is the complete independently verified CP/M event
file backed by the original Ruby oracle.

## Observed Chronos prototype

- Native first-stage `NOTES.N`: all1485 notes and two sources loaded from TRD.
- Both raw and final MIR2 emit all2153 events,12939 logical bytes,51 allocated
  sectors,98 sector calls. `yielded==2153` checks there was no counting prepass.
- Event bytes match the independently verified CP/M output; physical tail is
  zero fill. Every loaded note is compared to its input record.
- Actual write positions are checked:51 consecutive data sectors, then catalog
  and system sector. Free position/count and directory count reflect actual use.
- Disk changes are restricted to the appended extent, one new directory entry
  and five system metadata bytes; prior input files remain unchanged.
- A MIR2 OpStore observer checks arena padding and rejects arena writes after
  input close, including restoring writes. This observer is not a native CPU
  trace and does not observe host memcpy/memset or every possible VM access.

The prototype uses the existing capacities2046 notes,512 active handles and1024
events per channel. It is a single positive Chronos fixture, not the full
malformed/resource/OS-error matrix. Resident size, native stack and paging
restoration have not yet been verified for this second image.

## Reproduce the prototype

First run the fresh shared CP/M chain gate to produce a native first-stage TRD
and an independently checked event reference. All outputs stay outside checkout.

```sh
python3 research/autosiril-native/stream-flatten/verify.py \
  --autosiril /path/to/autosiril --out /tmp/common-file-chain
python3 research/autosiril-native/trd-flatten/compose.py --out /tmp/trd-flatten-probe
cd minzc
go run /tmp/trd-flatten-probe/prototype.go \
  -source /tmp/trd-flatten-probe/flatten-trd.nanz \
  -disk /tmp/common-file-chain/producer/trd/measured/chronos_full/scorpion-z80/disk.trd \
  -expected /tmp/common-file-chain/measured/chronos_full/z80/EVENTS.BIN \
  -out /tmp/trd-flatten-probe/measured
```

The generated Go file retains the existing verifier's unused CP/M entry as
`original_main` to reuse its audited importer/helpers; only the new prototype
entry executes. This is temporary research composition, not a new public CLI.
The recipe emits per-engine TRD images/events, receipts, raw/final MIR2 and
**unlinked** assembly. That assembly has not been placed/assembled/executed as
a native second-stage utility. No ROM, physical FDC, bootable disk or hardware
compatibility is implied.
