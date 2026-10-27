# Next native milestone: bounded echo with explicit timeline extent

Selected after PR #268 merged as4db828c61 with9 green checks; downloaded CI arrangement artifacts matched fresh evidence. Work branch:spike/autosiril-native-echo. The native TRD echo kernel now passes its first full-song and musical-boundary proofs; the complete error/CI milestone remains open; the initial CP/M proof and four-COM chain also pass. The simulation below is historical planning evidence.

Read actual Ruby autooisee.rb apply_delays780+ and VM engine.nanz apply_echo.
A host planning simulation over a fixed65-slot array matched actual Ruby in10
cases, including the actual native Chronos ARRANGE stream. This planning result alone did not prove
native execution; the independent native measurements are listed below. The source-bound planning receipt is saved alongside this plan; the temporary scripts remain at /tmp/minz-echo-planning until the reproducible native oracle is built.

Observed Chronos:2097 input LNotes →3215 delayed cells (1189/1013/1013),
all3 channel lengths1353; live pending maximum13. A dense wide-delay fixture
reaches all65 slots. Other cases cover original starts blocking earlier echoes,
release replacement/retention, older second echo beating newer first echo,
zero/equal/reversed delays, u/w flags, envelope renewal and large sparse gaps.

Ring rule: before consuming rowR flush pending rows<R; insert/replace the
original if destination is absent or original.type!='r', otherwise preserve
what earlier echoes put there. Schedule second then first from the ORIGINAL
input object, never from delayed output. Place only into nil/release cells.
A future original start restores its priority; an original release behaves like
the release/nil destination Ruby pre-cloned before scheduling. Therefore no
lookahead/full grid is needed. Effective delay<=64 (configured0..32, wide×2)
keeps all live rows within a64-row span; index=row%65 must have either no tag or
the identical absolute row. Never overwrite a different live row.

Centi-volumes1050/735 are valid for the producer's nominal1500 input; use the
actual typed Crystal echo_volume kernel. Ruby kind'e' renewal at these volumes
sets sample2/envelope15 under the current cool-envelope profile and preserves
note/ornament/type/kind/enote. Validate the producer's input invariants; do not
silently pretend the constant-volume kernel supports arbitrary input volumes.

Important extent boundary: ASIR kind3 carries nonempty LNotes/ornaments, not
trailing nil-array lengths. Ruby seq2vmod max_row uses ALL paired-note sources,
including unselected ones, and max(off,off+delay1,off+delay2). Do not infer length
only from sparse selected LNotes. A bounded target pass over the existing
NOTES.N/NOTES.BIN companion can compute max_off without storing notes (empty notes must retain Ruby base_rows1, not delay+1), then
open ARRANGE.A/ARRANGE.BIN. This retains all-source extent without host musical
logic or modifying the verified first3 stages. Chronos native NOTES max_off1340
plus max_delay12 gives base_rows1353, matching original Ruby exactly.

Proposed lossless output extension: ASIRv1 kind4, fixed16-byte tagged payloads;
pass kind3 LNote/definition roles0/1 and add role2 channel extent:u32LE metadata.
Emit all3 final channel extents, including empty/unvoiced trailing silence.
Validate framing/count/END/EOF and definition fragments. The full Chronos
candidate would be54880 logical bytes (3215 LNotes +9 definition fragments +3
extent records); this fits the existing TRD file limit. That was the planning sizing
calculation; the first native proof now measures exactly the same54880 bytes. Row data remain u16 unless explicitly
versioned wider; check overflow, never wrap. CP/M can stream larger files than
TRD; preserve transport-specific capacity failures rather than shrinking music
inputs just to make both transports succeed.

Acceptance for the selected next goal:
actual Ruby/Crystal oracle, raw/final MIR2/native, sequential companion/input
files and CP/M/TRD bindings; exact collision priority, end extents and u/w/error
cases; fixed memory/physical guards/mutants, fresh/frozen CI and a separate PR.
Mix/pattern/VT2/VTX remain later stages. i7 read-only; no Herdr control outside
HERDR_ENV; preserve root user files and all integrated gates.


Implementation requirements to settle early:
- Strictly parse NOTES kind1 and ARRANGE kind3 on target, including definition
  fragment framing; no host note/pitch/extent computation. An unselected late
  source must still determine the initial global extent. With no paired notes,
  Ruby leaves max_row0 and base_rows1. With notes, use u32(max_off)+max(delay1,
  delay2)+1, independently of u/w; wide echoes may extend individual channels.
- New kind4 extent payloads use role2, channel byte1, row-count:u32LE at2..5,
  zero reserved6..15. The LNote row field remainsu16. Compute scheduled rows in
  u32 and explicitly reject data rows>65535 instead of wrapping; extent metadata
  can faithfully exceed65536 because unselected notes/trailing silence require
  no out-of-range LNote row. Validate record counts and terminal metadata.
- Nominal input volume1500, cool-envelope profile and configured delay0..32,
  wide/unvoiced flags are checked. Keep the producer's2046/512/1024 capacities.
  Echo can produce up to3 cells per input LNote; do not impose the old output
  event bound1024 on delayed cells. Keep CP/M and TRD file limits explicit.
- Pass every ornament definition unchanged; preserve all LNote fields and use
  the actual typed Crystal echo_volume kernel through the audited closed ABI.
- Full Chronos must match3215 delayed cells,3 extents1353 and all definitions
  against actual Ruby. Include collision priority, release/zero/equal/reversed
  delays,u/w/envelope, sparse gaps, full65-slot window, empty/unselected extent,
  malformed input and exact resource/overflow/I/O state cases.
- Deliver independently linked fourth images using real saved native companion
  and arrangement files, with fresh/frozen native/VM/CI proofs, memory/port/IRQ/
  mapping guards and restoring-write mutants. Measure code/data/spills/external
  RAM/stack/I/O/T-states. Preserve every integrated gate and user file, and merge
  only through a PR with required CI. API traps are not ROM/hardware/boot proof.


## First native checkpoint

The separately linked fourth TRD stage reads and closes the real saved NOTES.N
companion, reopens ARRANGE.A produced by the third native stage, then writes
DELAYED.D. Target code computes all extents and echo scheduling. The host only
transports sectors, executes the original Ruby oracle and verifies bytes/state.
Fresh typed Crystal export is byte-checked against the admitted scalar fixture
and actually linked through the closed ABI/effects checks.

Full Chronos agrees byte-for-byte in raw MIR2, final MIR2 and guarded native
Scorpion Z80:3215 cells (1189/1013/1013),8 definitions/42 expanded bytes,
3 extents1353,54880 logical bytes/215 sectors. Output wire SHA256:
`2b115c050b50dfa0b3cececdc84dfb4144eaae6daa37c67600b2df9058093b25`.

| Native measurement | Bytes/count |
|---|---:|
|Code plus sector stub|15631B|
|Loaded globals|905B|
|Static spills|383B|
|Linked image|16919B|
|External fixed workspace reservation|2048B (2035 used)|
|Bank8 disk bounce reservation|16384B (256 used)|
|Stack below entry, excluding caller's return word|48B|
|Sector API calls|404 (187 reads/217 writes)|
|CPU T-states|407076575|

Image+external reservations total35351B, before stack/runtime reservations.
The ring uses no bank0 memory. All16 physical banks are profiled: only the
bank8 first256 bytes may be written through the paged window. Code/stack/fixed
workspace/configuration/PC/port/mapping/IRQ checks remain enabled. Two input
open/close lifetimes are observed independently.

The initial17-case/51-execution musical matrix includes maximum65-slot use,
zero/equal/reversed delays, starts/releases and collision priority, u/w,
envelopes, sparse gaps, empty notes, unselected late sources, extent>65536,
last representable data row, and1024 input cells producing3072 output cells.
A104537B expected output is deliberately retained: TRD rejects after255 data
sectors, leaves an exact unpublished data prefix, and preserves free-space/
catalog state. CP/M accepts that larger file in all3 engines:104537 logical/104576 physical bytes,6144 cells.

Reproduce this checkpoint with an existing **complete fresh arrangement gate**
(the reuse is explicit and its source/Ruby/native artifacts are checked):

```sh
python3 research/autosiril-native/echo/prove_positive.py \
  --upstream /tmp/minz-arrange-complete-fresh-v1 \
  --autosiril /path/to/autosiril --out /tmp/echo-positive
python3 research/autosiril-native/echo/prove_musical.py \
  --upstream /tmp/minz-arrange-complete-fresh-v1 \
  --autosiril /path/to/autosiril --positive /tmp/echo-positive \
  --out /tmp/echo-musical
```

Remaining acceptance work: all saved native input chains; malformed input,
capacity/row overflow, transport and publication failure matrix; small/short/stalled
chunk contracts and directory status variants; restoring-write mutants; default full fresh/frozen
required CI, downloaded-artifact audit and PR merge. This checkpoint is not a
claim that the full native echo milestone is complete.


## CP/M checkpoint

The same17 actual-Ruby musical cases pass raw MIR2, final MIR2 and native Z80
through the common CP/M binding (51 executions). Full Chronos reads both saved
native files, closes each input independently, and writes54912 physical bytes
for the54880B logical stream. The larger104576B output is retained, rather than
reducing the input to fit TRD.

| Native CP/M measurement | Value |
|---|---:|
|Code plus bootstrap|12417B|
|Resident globals|2997B|
|Static spills|293B|
|COM image, no external workspace|15707B|
|Stack below entry, excluding caller return word|46B|
|Full Chronos kernel T-states|402852467|
|Open / close / make / DMA calls|2 /3 /1 /2|
|Physical128B reads / writes|364 /429|

A fresh sequence of four separate mze processes executes PAIR.COM,
FLATTEN.COM, ARRANGE.COM and ECHO.COM from INPUT.MID. NOTES.BIN, EVENTS.BIN,
and ARRANGE.BIN exactly reproduce the preceding integrated standalone proof;
DELAYED.BIN exactly matches the guarded native fourth-stage result. The
standalone emulator and every COM/data file have hashes in the CP/M receipt.
This uses CALL5 API traps with the existing SP/HALT verifier contract, not a
physical CP/M TPA/CCP/bootstrap claim.

```sh
python3 research/autosiril-native/echo/prove_cpm.py \
  --upstream /tmp/minz-arrange-complete-fresh-v1 \
  --positive /tmp/echo-positive --musical /tmp/echo-musical \
  --out /tmp/echo-cpm
```

Checkpoint receipts: [full Chronos](evidence/positive-receipt.json),
[TRD musical boundaries](evidence/musical-receipt.json),
[CP/M musical boundaries and four standalone programs](evidence/cpm-receipt.json).
These receipts explicitly record upstream reuse and pending full acceptance work.

## Robustness checkpoint

The expanded drivers check explicit failure contracts in addition to final byte
comparison. Both observe each input open/close independently, require ring
cleanup, and reject configuration writes even when a later write restores the
old value. The TRD driver preserves successful/failed sector-call traces,
publication state, exact data prefixes, sector padding, catalog/system changes,
and free-space accounting. CP/M preserves immediate-make visibility, exact
physical prefixes, independent closes and preexisting output contents.

| Suite | Current cases / executions |
|---|---:|
|Malformed NOTES/ARRANGE and resource boundaries|60 cases ×3 engines ×2 transports =360|
|Sector / BDOS / directory / space / publication failures|45 TRD +15 CP/M cases, ×3 =180|
|Chunk behavior and CP/M success codes|141 executions|
|Actual native guard mutants|4 TRD +2 CP/M rejections|

These include2046/2047 companion notes;64/65 definitions;4096/4097 expanded
ornament bytes;1024/1025 incoming LNotes; scheduled-row overflow before a u16
cast; malformed magic/version/kind/width/flags/reserved/tag/count/END/EOF;
source ordering and LNote/definition invariants. Capacity checks keep input and
delayed-output limits separate.

Sector failures cover both independent input opens/reads, first/middle/last
Chronos data writes and every catalog/system publication operation. Exact215
free sectors succeed for Chronos;214 leave an unpublished exact partial prefix.
If catalog publication succeeded before the system-sector failure, the file is
already visible while free-space accounting remains unchanged. This is checked
explicitly and is not crash-atomic publication. CP/M tests include each input
close, output close, failed make, preexisting output and partial physical files.

Chunk controls independently target the companion and arrangement reader:
zero progress, repeated stalls, EOF with bytes, oversize/unknown status, invalid
capacity/padding quantum, and a short write. Read splitting to1 and7 bytes is
checked on both transports, including the dense65-slot case. Successful CP/M
directory results1,2,3 remain accepted and normalized.

The six real native mutants include a bad bank restore, a257-byte bounce copy,
a bank0 write which restores its old byte, and restoring scalar/mode config
writes. Each must fail for its named profiler rejection, not merely for a final
output mismatch. No blanket prohibition is imposed on normal writes to the
owned ring workspace.

```sh
python3 research/autosiril-native/echo/prove_robustness.py \
  --upstream /tmp/minz-arrange-complete-fresh-v1 \
  --positive /tmp/echo-positive --musical /tmp/echo-musical \
  --out /tmp/echo-robustness
```

The wrapper explicitly checks the reused fresh Ruby/Crystal checkpoint and the
unchanged target kernels, then records current sources and all child proofs.
This still leaves all20 native input chains, default full fresh/frozen replay,
required CI, artifact audit and PR merge for the complete echo milestone.

Before final acceptance, extend this checkpoint with the20 saved arrangement
chains (including the last-u16-row chain, whose delayed DATA overflow must be
reported rather than hidden). Make fixture paths relative for frozen CI replay.
The final verifier audit should also compare all16 bytes of a newly published
TRD descriptor and observe CP/M IFF1/IFF2, in addition to the existing per-step
Scorpion IRQ/mapping guards. Retain every older CI gate; the current PR gate's
60-minute timeout may need adjustment based on the measured complete replay.

[Robustness checkpoint receipt](evidence/robustness-receipt.json) records681
ordinary executions and6 actual native mutant rejections on one source snapshot.
