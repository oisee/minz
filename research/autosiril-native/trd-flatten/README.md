# Next-stage experiment: one-pass flattened events appended to TRD

**Status: one positive Chronos proof on raw/final MIR2 and native Scorpion Z80.**
The malformed/resource/disk-error matrix and required CI integration remain
pending. Three native guard mutations are already rejected. This is an experimental second image, separate
from the integrated native first-stage TRD proof.

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
- Raw/final MIR2 and assembled Z80 emit all2153 events,12939 logical bytes,51 allocated
  sectors,98 sector calls. `yielded==2153` checks there was no counting prepass.
- Event bytes match the independently verified CP/M output; physical tail is
  zero fill. Every loaded note is compared to its input record.
- Actual write positions are checked:51 consecutive data sectors, then catalog
  and system sector. Free position/count and directory count reflect actual use.
- Disk changes are restricted to the appended extent, one new directory entry
  and five system metadata bytes; prior input files remain unchanged.
- MIR2 OpStore observation checks arena padding and writes after input close;
  it does not observe host memcpy/memset or every VM access. Native profiling
  separately checks all16 physical banks, permitted fixed writes, PC/stack,
  every disk-call mapping restoration, IRQ policy and paging-port traffic.
  The input-open transition snapshots physical arena write counts, catching
  restoring writes after close.

The prototype uses the existing capacities2046 notes,512 active handles and1024
events per channel. It is a single positive Chronos fixture, not the full
malformed/resource/OS-error matrix. Only the complete Chronos positive path currently has the native proof.

## Positive native budget

The image is13248 bytes:12844 code/stub +126 loaded globals +278 static spills.
Separate reservations are2048 fixed-workspace bytes,12288 arena bytes in bank0
and16384 bytes for the bank8 bounce region (256 used). Their sum is43968 bytes,
plus fixed stack/runtime reservations. The maximum kernel stack is65 bytes
below entry, excluding the incoming two-byte return. Chronos takes450683266
Z80 T-states, excluding actual disk latency/ROM service execution.

Fixed arrays occupy4000..47FF; the image occupies6000..93BF, with stack
B800..BFFF. The sector stub and all physical guards are composed from the
existing first-stage verifier, changing only admitted array placements and
removing the absent per_beat setup/checks. This uses the same research EQU
placement mechanism, not a new core Global.At relocation implementation.

## Reproduce the prototype

First run the fresh shared CP/M chain gate to produce a native first-stage TRD
and an independently checked event reference. All outputs stay outside checkout.

```sh
python3 research/autosiril-native/stream-flatten/verify.py \
  --autosiril /path/to/autosiril --out /tmp/common-file-chain
python3 research/autosiril-native/trd-flatten/compose.py --out /tmp/trd-flatten-probe
cd minzc
go build -o /tmp/trd-flatten-probe/probe /tmp/trd-flatten-probe/prototype.go
/tmp/trd-flatten-probe/probe \
  -source /tmp/trd-flatten-probe/flatten-trd.nanz \
  -disk /tmp/common-file-chain/producer/trd/measured/chronos_full/scorpion-z80/disk.trd \
  -expected /tmp/common-file-chain/measured/chronos_full/z80/EVENTS.BIN \
  -out /tmp/trd-flatten-probe/measured
```

The generated Go file retains the existing verifier's unused CP/M entry as
`original_main` to reuse its audited importer/helpers; only the new prototype
entry executes. This is temporary research composition, not a new public CLI.
The recipe emits per-engine TRD images/events, receipts, raw/final MIR2,
unlinked compiler assembly and the placed/assembled native program. It executes
the emulator’s actual TRD API traps with guarded Scorpion memory, not a physical
ROM/FDC. No bootable disk, physical hardware compatibility or complete
MIDI→VT2/VTX converter is implied.

Native guard mutations can be replayed with `negative.py` using the compiled
probe, composed source, shared `trd-stub.a80`, the same input disk and event
reference. The script selects only the native engine and requires specific
rejections for broken restore, bounce overrun and a restoring arena write.
These three checks do not substitute for the pending disk-error/resource matrix.
