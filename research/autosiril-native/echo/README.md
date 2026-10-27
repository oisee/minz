# Bounded native echo: fourth streaming stage

Work branch: `spike/autosiril-native-echo`, based on main `4db828c61` / PR #268.
The fourth independently linked stage reads native `NOTES.N` + `ARRANGE.A`
(or `NOTES.BIN` + `ARRANGE.BIN`) and writes `DELAYED.D` / `DELAYED.BIN`.
The complete milestone still requires green fresh/frozen CI, artifact audit and
PR merge. Mix/pattern/VT2/VTX remain subsequent stages.

## Streaming algorithm and profile

A target pass over the companion NOTES stream determines the extent from **all
sources**, including unselected ones. It closes that input, switches the name,
then opens ARRANGE. Empty notes give base extent1; otherwise the extent is
`u32(max_off)+max(delay1,delay2)+1`, independently of unvoiced/wide flags.
Accepted wide echoes may extend an individual channel. Sparse kind3 input alone
cannot reveal trailing nil-array lengths.

The ring has65 tagged slots for a maximum effective delay64. Before consuming
rowR, flush pending rows<R; on a channel change, flush the previous channel.
Insert the original start with priority; an original release preserves an older
echo already occupying that row. Schedule second echo, then first echo, from
the **original input cell**, and place only into absent/release destinations.
An older second echo can therefore beat a newer first echo. A different live
absolute row must never alias the same slot. No recursive echoes, full-song grid,
note array or GC is needed.

The admitted producer profile uses centi-volume1500 and the cool-envelope
settings. The actual typed Crystal `crystal_echo_volume` export produces1050/735
through a closed scalar HIR ABI with checked effects. Other input volumes are
explicitly rejected. Delayed kind'e' cells renew to sample2/envelope15; all other
LNote fields and every ornament fragment are retained. Source e fields are
checked against the original tables. Configured delays are0..32, wide doubles
them, and unvoiced suppresses scheduling.

Input limits remain2046 paired notes,64 sources and1024 LNotes/channel. Output
can grow to3072 delayed cells/channel; it does not inherit the1024 input bound.
Definition limits are64 /4096 expanded bytes. DATA rows remainu16: calculate a
scheduled row inu32 and reject>65535 **before** casting. Channel extent metadata
isu32 and may exceed65536 for trailing silence. See the
[kind4 wire format](format.md); no field is a native pointer.

## Measured full Chronos

Raw MIR2, final MIR2 and native Z80 agree byte-for-byte with actual Ruby:
2097 original LNotes →3215 delayed cells (1189/1013/1013),8 definitions /42
expanded bytes /9 fragments,3 final extents1353. Output is54880 logical bytes,
215 TRD sectors or54912 physical CP/M bytes. Wire SHA256:
`2b115c050b50dfa0b3cececdc84dfb4144eaae6daa37c67600b2df9058093b25`.

| Measurement | TRD Scorpion Z80 | CP/M Z80 |
|---|---:|---:|
|Code + stub/bootstrap|15631B|12417B|
|Resident globals|905B|2997B|
|Static spills|383B|293B|
|Linked image|16919B|15707B|
|External fixed workspace reservation|2048B (2035 used)|0B|
|Bank8 disk bounce reservation|16384B (256 used)|0B|
|Stack below entry, excluding caller return word|48B|46B|
|Kernel T-states|407076575|402852467|
|Sector API calls|404 (187 reads/217 writes)|—|
|Physical128B reads/writes|—|364 /429|
|Input opens / all closes|2 /3|2 /3|

TRD image+external reservations total35351B before stack/runtime reservations.
The ring uses no bank0 memory. All16 physical banks are profiled: only the
first256 bytes of bank8 may be written through the paged window. Fixed code,
globals, spills, workspace, stack, PC/data execution, paging ports, mapping and
IRQ are guarded. Configuration write counters reject transient writes even
when restored. CP/M observes IFF1/IFF2 and fixed storage/stack/PC/port invariants.

## Reference and native chains

There are36 musical cases:16 isolated boundary cases and20 real saved native
arrangement chains,108 executions per transport. Each native case verifies
both companion and arrangement bytes against its upstream disk/files; receipts
link upstream and fourth-stage binaries. Paths in the fixture are relative,
so CI consumes its own newly generated upstream images.

The Ruby adapter calls the original `seq2vmod` on every paired companion note,
with cpr1 because the wire already contains quantized rows. It verifies that the
adapter preserves start/off/pitch. The resulting original Ruby `max_row`
determines base extent; Python's sizing formula is only an independent assertion.
The original `apply_delays` then determines cells and final channel lengths.
Fresh Crystal export/manifest bytes are checked and actually linked, not hosted.

The real last-u16-row chain is deliberately retained: Ruby can describe delayed
rows>65535, and native echo must return980. Its fixture is marked
`partial_reference`; only representable Ruby cells are serialized for checking
any written prefix, never as a successful full output. Both transports require
the overflow fault. TRD publishes nothing; CP/M has an empty made file.

A separate large case produces6144 cells /104537 logical bytes. CP/M accepts
104576 physical bytes. TRD rejects after255 written data sectors and leaves an
exact unpublished prefix without changing free-space/catalog metadata. Inputs
are not reduced to make the transports appear equivalent.

Four separate mze processes execute
`PAIR.COM → FLATTEN.COM → ARRANGE.COM → ECHO.COM` from INPUT.MID. The first three
files reproduce the preceding integrated proof; DELAYED.BIN matches guarded
native execution exactly. Every COM/data file and the emulator have hashes.

## Failure and resource coverage

| Suite | Executions |
|---|---:|
|75 malformed/resource/config/row-overflow cases, both transports ×3 engines|450|
|45 sector/publication +15 BDOS cases ×3 engines|180|
|Chunk controls on each input, both transports, plus CP/M success codes|141|
|Actual native restoring-write / paging / bounce mutants|6 rejections|

The matrix includes2046/2047 companion notes;64/65 definitions;4096/4097 expanded
bytes;1024/1025 incoming cells; row overflow; source order; header metadata;
magic/version/kind/width/flags/reserved/tag/count/END/EOF; source LNote/envelope
and canonical fragment invariants. Both input lifetimes and ring cleanup are
checked after failures.

Sector failures cover each input open/read, first/middle/last Chronos data
writes and every catalog/system publication operation. Exactly215 free sectors
succeed;214 leave a checked unpublished partial prefix. All16 bytes of a newly
published TRD descriptor, padding and free-space updates are checked. If catalog
publication succeeds before the system operation fails, the file is already
visible while free-space accounting is unchanged. This is **not atomic**.

CP/M checks failed input opens/reads/closes, make/write/output-close, preexisting
output and actual partial physical prefixes. Chunk tests target each input
independently: tolerated empty progress, stalls, EOF with bytes, oversize and
unknown statuses, invalid capacity/padding, short write, and reads split into1
or7 bytes. Dense65-slot use remains correct. Directory success values1,2,3 are
accepted and normalized.

Native mutants must fail for their named observer: bad bank restore,257-byte
bounce copy, a bank0 write which restores its old byte, and restoring scalar/mode
config writes. Ordinary writes to the owned ring are allowed.

## Reproduce and review

Default verification rebuilds original Ruby/Crystal and **all inherited gates**
through arrangement before executing echo:

```sh
python3 research/autosiril-native/echo/verify.py \
  --autosiril /path/to/autosiril --out /tmp/echo-fresh
```

`--reuse-upstream /path/to/completed-fresh-arrangement` is an explicit development
shortcut; its source, Ruby and actual native files are checked and reuse is
recorded. `--refresh-evidence` saves source-bound receipts and the portable
fixture. Local macOS mze builds can use `--go-overlay /path/to/overlay.json` for
the existing user syscall.Select fix; overlay/replacement hashes are recorded.
The user file is not committed. Linux CI builds the clean repository.

Frozen replay executes every musical/failure/contract/mutant test and the four
COM programs against the current upstream native artifacts:

```sh
python3 research/autosiril-native/echo/replay.py \
  --upstream /tmp/autosiril-arrange --producer /tmp/autosiril-transports \
  --mze /tmp/mze --out /tmp/echo-frozen
```

CI runs a full fresh job with pinned Autosiril44925349147d, Crystal1.20.1 and
midilib4.0.2, and an independent inherited/frozen job. The preserved required
`PR gate` is an always-run barrier requiring both jobs to succeed. Every old
compiler/native gate remains in place. Fresh and frozen artifacts retain
sources, native binaries, MIR, assembly, files/disks, traces and receipts.

Receipts: [complete](evidence/receipt.json), [full Chronos](evidence/positive-receipt.json),
[TRD/20 native chains](evidence/trd-receipt.json),
[CP/M/20 native pairs/four COM programs](evidence/cpm-receipt.json),
[robustness](evidence/robustness-receipt.json).
The [original65-slot simulation](planning-receipt.json) is historical planning
material; native execution is proved separately.

These are API-trap experiments with a fixed verifier ABI, not physical
ROM/FDC/hardware/boot, CP/M TPA/CCP bootstrap, full VT2/VTX, MOS or eZ80 proof.
i7 stays read-only. Herdr is not controlled outside HERDR_ENV. The four root user
files and retained40626B in-process image remain untouched.
