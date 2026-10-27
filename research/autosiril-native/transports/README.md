# Interchangeable Autosiril transports: CP/M and Scorpion TRD

The first native utility now composes the **unchanged**
[`midi/pair.nanz`](../midi/pair.nanz) with one common file producer and one
transport binding. MIDI parsing, FIFO pairing, source assignment and the real
Crystal `quantize` function execute on raw MIR2, final MIR2 and assembled Z80.
Host hooks perform bytes/OS operations only. No GC or native pointers are stored
in ASIR files.

This directory is the common-transport successor to the original CP/M producer;
the original [147-case file gate](../files/) and standalone `PAIR.COM` remain
unchanged. The second [CP/M flatten utility](../file-flatten/) and the separate
[40626-byte in-process image](../linked/) remain available. Moving the flatten
stage behind this same API, then binding it to TRD, is the next step.

## Contract

`read_chunk()` fills the fixed `input_chunk[256]` and returns a u32:

| Bits | Meaning |
|---|---|
| 0..15 | byte count |
| 16..31 | 0 progress, 1 EOF, 2 error; unknown values are errors |

Progress may have count0; 1024 consecutive empty chunks produce fault912. EOF
requires count0 (otherwise911); count above256 produces210. The producer copies
each input byte into the existing incremental MIDI feed, so no borrowed pointer
survives refill or a paging transition.

`write_chunk(count)` uses the same packed result and must acknowledge exactly
`count`; a short acknowledgement is fault902. `finish(0)` closes input and
`finish(1)` closes output; zero means success. `open_input()` sets physical
output capacity (CP/M128, TRD256) and input-padding policy. Invalid capacity
is rejected before reading or creating output. The bounded buffer and ASIR
serializer are common code, not duplicated musical logic.

Input padding quantum is explicit binding metadata:128 for CP/M records,0 for
TRD's exact logical length. The MIDI producer counts position within a physical
record across short refills. After declared SMF END it accepts1A only to that
record boundary, then requires EOF. Chunks of1/7 bytes therefore do not turn
valid padding into a new record, and an extra padding-only record remains an
error. Invalid padding metadata is rejected before reading.

The CP/M binding normalizes successful directory results0..3 to the common
zero-success contract for BDOS15/16/22. This matters on CP/M2.2 even though the
current mze provider returns0. Read/write statuses retain their distinct rules.
See the original [Digital Research CP/M2 interface manual](https://www.gaby.de/cpm/manuals/archive/cpm22htm/ch5.htm).

ASIR v1 notes: 12-byte header (`ASIR`,1,1,6,0,sources:u16LE,reserved:u16LE0),
then tag01 plus six note bytes per DATA record, then
`FF count:u32LE 45 4E 44 21` (`END!`). Physical zero fill is outside logical data.
Chronos is **10416 logical bytes**: CP/M stores10496 bytes, TRD allocates41 sectors
(10496 bytes) and records logical length10416 in its directory. The logical
streams are compared directly across the two transports. The new common CP/M
image also runs independently through ordinary `mze`: 458365926 T-states and
byte-identical physical `NOTES.BIN`.

## Verified execution

The fresh verifier runs the original Ruby oracle and native Crystal semantic
export, preserves the old342 MIDI executions and original147 CP/M executions
including standalone mze, then runs:

- **147** common CP/M executions (49 fixtures × raw/final MIR2/Z80).
- **186** TRD executions (62 fixtures × raw/final MIR2/Scorpion Z80): complete
  Chronos, all original capacity/error cases, record boundaries and empty file,
  failures of each of eight disk operations, malformed directory/free-space
  metadata, duplicate input/existing output, full directory/disk and last sector.
- **48** API-contract executions: empty progress, permanent stalls, EOF with
  bytes, oversized reads, unknown status, short writes and invalid capacity/input
  padding quantum; progress chunks of1/7 bytes across ordinary/exact physical
  record endings and an extra padding record.
- **9** directory-success executions: codes1/2/3 for open, create and both
  closes, each on raw/final MIR2 and assembled Z80, with real underlying file I/O.
- **Three rejected native mutants**: broken mapping restoration, a257-byte
  bounce write, and a write of the original canary value to arena padding.
  The last mutant needs write-count evidence; comparing final memory alone
  would miss it. Rejections must contain their expected guard diagnostic.

All1485 Chronos notes and two sources match Ruby; every stored note is checked.
Disk writeback is flushed and reloaded. Input directory/payload remains intact;
changes are restricted to preflight-reserved output sectors and metadata.
The raw/final interpreters perform real sector reads/writes without interpreting
MIDI or manufacturing musical results.

## Memory and I/O budgets

| Item | Common CP/M | Scorpion TRD |
|---|---:|---:|
| Loaded image | 32352 B | 20731 B |
| Code/bootstrap or code/stub | 16929 B | 20019 B |
| Loaded globals | 14942 B | 170 B |
| Static spills | 481 B | 542 B |
| External fixed workspace | 0 | 2432 B |
| External note arena | included in globals | 12288 B in bank0 |
| Additional bounce bank reservation | 0 | 16384 B in bank8,256 B used |
| Maximum kernel stack below entry | 70 B | 70 B |
| Full Chronos Z80 T-states | 458365843 | 455426256 |

Stack measurements exclude the two-byte incoming return address. CP/M Chronos
uses72 BDOS reads (71 records plus EOF),82 writes, one open/create/DMA setup and
two closes. TRD uses83 sector calls including catalog/system metadata. T-states
exclude real disk latency; these are offline converter measurements.

The smaller TRD binary is **not** a smaller total memory claim: loaded20731 +
fixed2432 + arena12288 + reserved bounce bank16384 = **51835 bytes**, plus the
fixed stack reservation and reserved ROM/runtime areas.

| Region | Placement and rule |
|---|---|
| Fixed workspace | `4000..497F`, explicitly consumes screen RAM |
| Code, stub, remaining globals and spills | `6000..B0FA`, fixed banks5/2 |
| Stack reservation | `B800..BFFF`, initial SP=`BFFE` |
| Note arena | `C000..EFFF`, physical bank0; three4096-byte regions with4-byte padding each |
| Disk bounce | bank8, first256 bytes; whole16K bank reserved |
| Return sentinel | `5F00`; never executed |

The linker admits only named, zero-initialized fixed arrays and validates their
sizes before replacing their data reservations with `EQU` symbols. This research
placement step does not claim that core codegen now relocates every `Global.At`.
The separate full in-process image has a different layout and is not used as
this disk-service image.

The stub runs with interrupts disabled, maps bank8, copies the fixed buffer,
calls the **actual emulator's MIT-licensed `3D13` API trap**, copies back and
restores `1FFD=00`, `7FFD=10` before returning. Guards inspect each return,
every executed PC, paging-port traffic, fixed writes, all16 physical bank write
heatmaps, arena padding and unused-bank canaries. Code and stack never occupy
the switched window. No bank8 near pointer escapes the stub.

The Scorpion profiler's paged heatmaps previously admitted eight pages, despite
the256K memory model exposing16. They now cover16; a regression checks all banks
and exports page8/page15. This does not change the legacy eight-bank snapshot
export format.

## Reproduce

From the repository root, with Go, Ruby, Crystal and the original Autosiril repo:

```sh
python3 research/autosiril-native/transports/verify.py \
  --autosiril /path/to/autosiril --out /tmp/minz-transports-fresh \
  --refresh-evidence
```

On macOS the current mze tick-input portability change may be supplied via
`--go-overlay /path/to/overlay.json`; it affects only standalone mze, not the
transport algorithm or probes. The proof records overlay provenance inherited
from the fresh CP/M reference. Local user files are not committed by this gate.

CI needs only Go and Python. It validates source hashes and replays the frozen
independent expectations, then checks matrix and logical-stream hashes against
the fresh receipt:

```sh
python3 research/autosiril-native/transports/replay.py --mze /path/to/mze --out /tmp/minz-transports-ci
cd minzc
go test ./pkg/spectrum ./pkg/spectrum/formats -count=1
```

Use a fresh output directory. Generated `.bin`, assembly, raw/final MIR2,
per-engine `.trd` images, note files, contract cases, mutant sources and logs are
kept outside checkout. CI uploads them as `autosiril-transports`.

After a transport-only edit, `verify.py --reuse-reference-root /path/to/completed-reference`
can reuse completed fresh `files/verify.py` products after checking their source
hashes and the original Ruby/MIDI digests. All changed-transport executions still
run, and the receipt explicitly records reference reuse.

## Limits

This is the **Scorpion256 memory model plus TRD/API traps**, not physical
TR-DOS ROM, FDC timing, or hardware compatibility. No ROM image is redistributed.
The DI stub has not established a physical ROM's register/workspace/interrupt
ABI. The high-level file scanner/append writer runs on the target; its supported
profile is a160-track/16-sector disk, fixed filenames `INPUT.M` and `NOTES.N`,
append-only allocation with strict metadata checks. It does not compact deleted
entries or implement a general filesystem library.

For the CP/M2.2 binding, output filenames must initially be absent: BDOS22
requires a new filename. The current mze provider truncates an existing host
file, which is not proof of portable overwrite semantics on native CP/M.

Output metadata publication is **not crash-atomic**: failures after a directory
write can leave a visible output descriptor with stale system metadata. These
cases return errors and are tested; consumers must require success before
starting the next stage. This first proof does not provide a bootable disk or
an interactive launcher. No scheduler, overlay loader or full MIDI→VTX converter
is implied. MOS remains a future transport independent of complete eZ80 readiness.
