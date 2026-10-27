# Native Ruby-compatible downmix: integrated fifth stage

Selected after echo PR #269 merged as `4b82cd7f5`, with11 green checks and
independent audits of downloaded fresh/frozen artifacts. Work branch:
`spike/autosiril-native-mix`. The fifth file stage is measured on CP/M and private Scorpion TRD API traps;
integrated through [PR #270](https://github.com/oisee/minz/pull/270) as
`b58fba0c6` after twelve green checks. Downloaded fresh/frozen artifacts
independently pass1923 executions and six guard mutants each, the40626B retained
image and five-COM chain. The sections below retain the implementation chronology.
Next selected bounded step: original Ruby `render_into_text`.

Original reference: `autooisee.rb` downmix877..919. The existing VM implements
later stages. The original base ended at the independently linked native echo stage; this
merged milestone adds CP/M and TRD downmix on actual saved native DELAYED files, while preserving
all preceding gates and user files.

## Exact semantics to prove before target code

Ruby consumes complete source lanes in abs_index order. sources_mix group sizes
determine destination lanes; the group entries themselves do not select ids.
Start with exactly3 input lanes and3 output groups, counts0..3 summing to3,
and one '+' or '-' policy for each input lane.

'+' replaces an absent/release cell or a strictly lower-volume cell. Equal
volume preserves the earlier lane/cell. '-' first tries that same rule at rowR;
otherwise it tries R+1,R+2,R+3 for absent/release destinations, then drops the
candidate. The historical elsif clauses compare mchan[R].volume again, not the
spill candidate's volume. Preserve this behavior. Incoming original starts have
no special priority over earlier spills, unlike the echo lowering.

A proposed streaming fold merges an already mixed sorted stream with a later
source lane using at most four tagged pending rows and bounded lookahead of the
earlier stream. Prove it against original Ruby first; do not assume echo's
collision rule transfers unchanged. Copy every LNote field and every definition.

## Memory, input lifetimes and metadata

Kind4 is channel-major. A fifth file stage needs independent input cursor/FCB
contexts or sequential merge passes; it cannot pretend that one reader already
provides all lanes in row order. Prefer bounded owned contexts/buffers, never
near borrowed pointers across paging or reopen. Keep the old single-reader API
and first four stages unchanged initially.

One candidate: validate and copy all definitions as an output prefix in a first
DELAYED pass, then reopen up to3 independent selected-lane readers. Each scans
its own file cursor and skips unrelated lanes. This may take4 passes; measure
actual I/O/code/RAM/stack/T-states rather than claiming hardware efficiency.

Downmix trims trailing nils: an empty output array has extent0; occupied cells
and spills determine other extents. Preserve input limits3072 cells/lane and up
to9216 cells total; schedule spill rows inu32, reject DATA>65535 before casting.
CP/M must retain large outputs even when TRD hits its file/space limit.

Important separate state: render_into_text930 uses original `$set.max_row`, not
mixed array lengths. Echo kind4 final extents may be enlarged by wide echoes,
so they cannot always recover that global value. Keep the original NOTES
companion available, derive original max_row through actual Ruby seq2vmod, and
specify an explicit global render_rows:u32 in the next wire kind or retain a
verified companion reread. Empty notes mean original max_row0, not1; the echo
base array extent is max_row+1. Do not lose this distinction while trimming mix.

## First verifiable step and full acceptance

First: actual Ruby downmix oracle over saved native Chronos and explicit
collision/group/volume/release/spill fixtures, compared with the proposed fixed
four-row host model. This is planning evidence only until target execution.
A separate actual Crystal scalar priority export may be useful; do not disturb
old shared music.cr exports/fixtures merely to add a new root.

Then implement Nanz/closed HIR lowering, a versioned lossless wire, real CP/M and
TRD fifth images, and owned multi-input lifetimes. Compare raw/final MIR2/native
against actual Ruby, including all20 native echo inputs and inherited failure
cases, '+/-', ties/lower/higher volumes, spills0..3/drop, empty groups/lanes,
trailing silence/global timing, resource/row/extent overflow, malformed streams,
short/tiny/stalled I/O, publication/free-space/prefixes, handle cleanup,16-bank
memory/PC/stack/port/paging/IRQ checks and restoring-write mutants.

Retain source/binary/MIR/assembly/files/disks/receipts, default full fresh and
frozen required CI, downloaded-artifact audits and a separate PR/main merge.
Preserve every echo/inherited gate, the40626B in-process path and the four root
user files. i7 is read-only; no Herdr control outside HERDR_ENV. API traps are
not physical ROM/FDC/hardware/boot/TPA-CCP/fullVT2VTX/MOS/eZ80 proof.

## First planning result

The original Ruby downmix/seq2vmod adapter and fixed-four-slot host model agree
in119 cases:19 real saved native echo streams, six nontrivial Chronos group/
policy variants, explicit collision/release/spill/gap/overflow/trailing-silence
cases, and every3-lane group composition/policy combination. The failed twentieth
upstream echo chain is checked explicitly: no published TRD output and an empty
CP/M made file. It is not passed off as valid mix input.

The canonical result also equals the existing full original Ruby mixed snapshot.
All serialized note fields and true array lengths agree, input objects stay
unchanged, and the model reaches all four live slots. Empty output extents are0;
original render_rows remain separate. A last-u16-row spill demonstrates why the
future target needs an explicit overflow fault.

```sh
python3 research/autosiril-native/mix/prove_planning.py \
  --upstream /path/to/complete-fresh-native-echo \
  --autosiril /path/to/autosiril --out /tmp/mix-planning
```

[Planning receipt](planning-receipt.json). This is still host-only planning
evidence for the mixing algorithm. The subsequent transport and lazy-composition
steps below are separate. The later target and CP/M sections describe their
additional evidence; this planning receipt itself remains host-only.


## Owned CP/M cursor groundwork

`cursors-cpm.nanz` gives each of three readers its own36-byte FCB,128 unread
bytes, cursor/length andu32 accepted-byte count. The shared DMA record is copied
into the selected owned buffer before returning. The output has a separate FCB
and buffer. Old single-reader transports and the four musical stages are unchanged.

`cursor-probe.nanz` is a transport-only COM experiment: it drains the same saved
DELAYED file through three independent readers in unequal1/7/31-byte bursts and
writes that exact interleaving. This forces the file offsets and unread buffers
apart while output writes overwrite shared DMA. No musical logic is hosted, and
no Crystal import is necessary for this transport test.

```sh
python3 research/autosiril-native/mix/prove_cursors.py \
  --input /path/to/fresh-echo/cpm/measured/native_chronos/z80/DELAYED.BIN \
  --out /tmp/mix-cursors
```

The Chronos54912-byte and large104576-byte saved native inputs each pass21
scenarios on raw MIR2, final MIR2 and native Z80:126 executions total. Scenarios
cover directory success codes0..3, preexisting output, each input-open failure,
make, early/late read and write failures, and all four close failures. All target
handles are released, existing output is unchanged and failed output is an exact
record-aligned prefix. Successful outputs are164736 and313728 bytes, preserving
large CP/M extents. This probe requires a record-aligned physical input; general
stream framing, short reads and stalled I/O belong to the later musical stage.

The image is3843 bytes:2906 code/bootstrap,848 globals,89 static spill bytes.
Maximum observed native stack is28 bytes. The harness checks every profiled write
against globals/spills/stack, PC/image bounds, data execution, return SP, canaries,
IRQ state and absence of port I/O. This is CALL5 API-trap evidence, not hardware
or a physical CP/M bootstrap proof. It does not yet cover TRD readers, format
validation, cursor reopen contracts or restoring-write mutants.

[Chronos evidence](evidence/cursor-chronos.json),
[large-file evidence](evidence/cursor-large.json).

## Composed lazy folds

```sh
python3 research/autosiril-native/mix/prove_lazy.py \
  --planning /tmp/mix-planning --out /tmp/mix-lazy
```

All119 original Ruby cases also match a composed iterator graph with no
intermediate fold-output lists. The first source is an identity stream; merging
three sources into one group needs two four-slot folds. Therefore the safe
pending-slot bound is8, plus two earlier-stream lookahead cells, input-reader
heads and final sink storage. Four slots is a per-fold bound, not the whole
program's RAM budget. Inputs/final results remain host fixtures; target lowering
and measured composed target memory are still required.

[Lazy composition evidence](evidence/lazy-host.json).

## Target downmix kernel

`kernel.nanz` and generated `folds.nanz` now implement the musical merge in
Nanz/HIR. Two distinct pull state machines compose without recursive function
frames or intermediate song arrays. The first lane is an identity stream;
subsequent lanes use the exact nil/release/strict-volume/spill policy above.
All16 payload bytes are cloned, with only destination channel and spill row
changed. The kernel clears pending leases on both success and failure.

```sh
python3 research/autosiril-native/mix/prove_kernel.py \
  --planning /tmp/mix-planning --autosiril /path/to/autosiril \
  --out /tmp/mix-target-kernel
```

The runner invokes actual original Ruby again for123 musical fixtures, including
19 decoded native echo streams and the prior complete policy/group matrix. It
adds9216-cell tests, including all9216 cells in one destination, the3072/3073
input-lane boundary and row65535. Separate negative cases cover malformed group
configuration, duplicate/decreasing source rows and read/write failures.

All136 scenarios pass on raw MIR2, final MIR2 and native Z80:408 executions.
The whole kernel probe image is7070 bytes:6599 code/bootstrap,323 globals and148
static spills. Maximum observed native stack is46 bytes. On three-lane Chronos
both fold units reach four live pending slots. These budgets exclude the later
file parser, physical bindings, kind5 serializer and NOTES companion pass.

Byte-only fixture traps provide/copy one16-byte input/output record; they make
no musical choices. The native harness disables the default screen RST hooks so
these are real intercepted calls, verifies profiled write/PC/stack/config/IRQ/IO
bounds, and compares every accepted field, count, extent and failure prefix.
Original Ruby references and all per-engine cell files/MIR/assembly/COM remain
in the requested output directory.

[Target kernel evidence](evidence/kernel-target.json). This proves target musical
lowering alone. The file-stage evidence below covers additional CP/M behavior;
TRD, the full acceptance matrix, required CI and integration remain outstanding.


## Independently linked CP/M kind4 + NOTES to kind5

`wire.nanz` now binds the target kernel to owned CP/M cursors. NOTES is validated
first, including every source, and yields original render_rows inu32. An initial
full DELAYED validation pass copies all definitions into the kind5 prefix. Then
three actual independent FCB/read-buffer contexts scan the same channel-major
kind4 file, selecting their own source lane while still validating the entire
file through END and physical EOF. Input reopen never resets the live output FCB.

Kind4 definitions may occur between cells. The first pass preserves their exact
payload/order while collecting them before kind5 cells. No full-song array or
ornament heap is needed: each scan retains bounded definition/order counters,
per-lane last rows and its owned16-byte record. Mixing and timing are target Nanz;
the host adapter only implements file/record IO.

```sh
python3 research/autosiril-native/mix/prove_file.py \
  --upstream /path/to/complete-fresh-native-echo --planning /tmp/mix-planning \
  --autosiril /path/to/autosiril --out /tmp/mix-cpm
python3 research/autosiril-native/mix/audit_file.py \
  --run /tmp/mix-cpm --out /tmp/mix-cpm-audit.json
python3 research/autosiril-native/mix/bundle_cpm.py \
  --run /tmp/mix-cpm --upstream /path/to/complete-fresh-native-echo \
  --out /tmp/mix-cpm/standalone
```

The current matrix is187 scenarios on raw MIR2, final MIR2 and native Z80:
561 executions. Actual Ruby is rerun for127 musical/reference fixtures. The
matrix includes the original119 cases, all19 saved native echo inputs, six
Chronos group variants,9K-cell capacity,3072/3073 input cells, row/spill overflow,
64/65 definitions,4096/4097 expanded definition bytes, directory success codes,
preexisting output, all five opens and six closes, early/late read/write failures,
invalid configuration, malformed headers/roles/definitions/cells/END/padding and
extra physical records. The twentieth failed echo chain is explicitly rejected
as an empty DELAYED file before output creation; it is not treated as valid input.

The independent artifact decoder verifies all561 saved outcomes:390 complete
outputs against Ruby fields/lengths/timing and original definition bytes,168 exact
record prefixes, three preserved existing files;75 outcomes require actual
absence. It checks source/native/matrix hashes and retained input files. The full
acceptance matrix is not yet claimed: more malformed boundary cases, stalled/short
common IO, TRD publication/space/bank contracts and restoring-write mutants remain.

Measured COM:23430 bytes =20874 code/bootstrap +1952 globals +604 static spill
bytes. Maximum observed native stack is80 bytes. Canonical Chronos performs five
input opens, six closes,1803 record reads and429 writes, taking1324001265 measured
T-states under API traps. This four-pass input strategy is deliberately bounded
and currently expensive; these figures are not hardware throughput measurements.
All9216 output cells also fit in one destination:156800 physical output bytes,
verified beyond64KB. Original render_rows can exceed65535 without truncation;
empty mixed extents remain0 independently of input echo extents.

`bundle_cpm.py` collects PAIR/FLATTEN/ARRANGE/ECHO from the verified upstream run
and the current measured MIX.COM and MIXED.BIN. It verifies exact NOTES/DELAYED
file links and records all five program/file hashes. This bundle is not a fresh
five-command replay or a physical CP/M bootstrap claim.

[CP/M stage receipt](evidence/cpm-file.json),
[independent artifact audit](evidence/cpm-file-audit.json),
[five-COM bundle manifest](evidence/cpm-chain.json).

The milestone remains active. Still required: the remaining full malformed/catalog/free-space/common-IO matrix,
default complete fresh upstream rebuild plus frozen replay in required CI,
downloaded-artifact audits, separate PR and verified merge to main. No old echo
or inherited gate, in-process path, compiler source or user file has been changed.


## Owned TRD stage and16-bank guards

`cursors-trd.nanz` adapts the existing append-only sector binding without editing
any older stage. Three logical cursor positions/remaining lengths and three
owned256-byte buffers survive catalog reads and output writes through one shared
sector scratch. Input reopen validates metadata independently and restores the
writer's captured free-position/free-count/directory snapshot. A changed snapshot
is rejected rather than silently replacing the live writer's allocation state.

The same target kernel and wire parser are used. Composition changes only the
physical EOF padding rule: CP/M reads zero padding to a128-byte record boundary;
TRD's catalog length is logical, so END must be followed immediately by EOF.
All data/definition/order/extent/timing validation and musical choices remain
in target code. Native sector calls use the real existing Scorpion API trap,
not a host musical implementation.

```sh
python3 research/autosiril-native/mix/prove_trd.py \
  --cpm /tmp/mix-cpm --upstream /path/to/complete-fresh-native-echo \
  --autosiril /path/to/autosiril --out /tmp/mix-trd
python3 research/autosiril-native/mix/audit_trd.py \
  --run /tmp/mix-trd --cpm /tmp/mix-cpm --out /tmp/mix-trd-audit.json
python3 research/autosiril-native/mix/prove_trd_io.py \
  --run /tmp/mix-trd --out /tmp/mix-trd-io
python3 research/autosiril-native/mix/audit_trd.py \
  --run /tmp/mix-trd-io --cpm /tmp/mix-cpm --out /tmp/mix-trd-io-audit.json
python3 research/autosiril-native/mix/prove_trd_guards.py \
  --run /tmp/mix-trd --out /tmp/mix-trd-guards
```

The current128-case matrix passes384 raw/final MIR2/Scorpion executions, including
all19 real saved native echo disks, six Chronos grouping variants, policy/spill/
release/sparse cases and resource boundaries. The actual failed twentieth echo
disk has no DELAYED entry and is explicitly rejected before output. The two
valid156761-byte logical CP/M capacity inputs are explicitly inventoried as
unrepresentable TRD inputs; they are not silently counted as TRD successes.

Two additional actual-Ruby fixtures isolate the output-sector boundary. With
3833 identity cells,65250 input bytes become65267 output bytes and255 sectors
publish successfully. With3834 cells,65267 valid input bytes would become65284
output bytes. Target stops after255 data sectors, leaves their exact prefix and
publishes no MIXED descriptor/system metadata. No byte count is truncated.

Another24 scenarios inject one failure at a measured actual sector API call,
on all three engines:72 executions. They cover system/catalog/data reads for
the initial passes and each of three owned reader opens, later reads, first/
middle/last data writes and all four catalog/system publication operations.
Every opened logical cursor is closed and the mapping returns to bank0. Failure
after a successful catalog write can leave a visible descriptor with unchanged
system counters; that partial publication is checked explicitly, not described
as atomic success or absence.

Independent disk/stream audits verify all456 outcomes, exact data-sector prefixes,
unchanged disk bytes outside permitted spans, catalog descriptors, publication
order/free-space counters, all Ruby note fields/trimmed lengths/original timing,
unchanged definition bytes and identical operation order across engines.

Six native instruction mutants also fail the intended guards. Four write and
restore code/unreserved fixed RAM/bank0/bank8 outside its bounce range; one
changes and restores paging but adds forbidden port traffic; one enables IRQs
transiently. The first five preserve the real target return and complete output
before profile rejection. Thus final RAM equality alone cannot hide the writes.
Mutant source/native assembly/binaries/logs remain in the output artifact tree.

Measured private image:28449 bytes =25216 code/stub +2504 resident globals +729
static spill bytes. It occupies4000..AF20, with fixed stack reservationB800..BFFF
(initialSP=BFFE); maximum measured stack is80 bytes. Earlier6000 and5000 origins
were too small for this implementation. The lower canary is a snapshot of the
unchanged ROM boundary; fixed storage writes are checked by profile, and all16
physical RAM banks are checked, with only bank8's first256 bytes admitted for
bounce writes. Its entire16KB bank is reserved. No near pointer into bank8 escapes
the provider, and paging/IRQ/PC/stack/data-execution/configuration/port guards run
throughout execution and failures.

Canonical Chronos produces54897 logical bytes in215 data sectors, with1130 API
calls and1325486470 measured T-states; measured stack is62 bytes. These are
API-trap observations, not hardware timing. This private layout uses Spectrum
screen/BASIC workspace and is not a normal TR-DOS ROM loader/boot claim. Physical
ROM/FDC/hardware, real CP/M TPA/CCP, full VT2/VTX and MOS/eZ80 remain outside scope.

[TRD musical/resource receipt](evidence/trd-file.json),
[independent disk audit](evidence/trd-file-audit.json),
[sector failure receipt](evidence/trd-io.json),
[sector failure artifact audit](evidence/trd-io-audit.json),
[six native guard mutants](evidence/trd-guards.json).

The full milestone is still active. Remaining work includes further malformed
framing/cursor-lifetime/catalog/free-space/common-IO cases, complete default fresh
rebuild and frozen replay in required CI, downloaded artifact audits and a PR/
main merge. Current experiments reuse a verified fresh upstream artifact and do
not substitute for those required gates.

## Complete gate and required CI

The complete fresh entry point now runs the preceding four native stages,
including actual Ruby/Crystal, then the entire fifth-stage proof:

```sh
python3 research/autosiril-native/mix/verify.py \
  --autosiril /path/to/autosiril --out /tmp/native-mix-fresh
```

Default execution rebuilds the upstream pipeline. `--reuse-upstream` is an
explicit development option, records that reuse and requires a complete fresh
echo receipt with matching source hashes. A local Go overlay can be passed
explicitly to the preceding gate where needed. Neither option is used by the
required fresh CI job.

The matrix now contains1923 raw/final MIR2/native executions:408 kernel,561
CP/M file,384 TRD musical/resource,72 sector failures,102 catalog/free-space,
210 malformed/NOTES boundaries,48 common-IO,12 writer-snapshot and126 owned-cursor
executions. Six restoring-write/paging/IRQ mutants run separately. The unchanged
40626-byte in-process alternative is also recompiled and exercised through nine
complete pipelines/36 phase observations, with the original native image hash.

The expanded tests exposed and fixed two error paths: NOTES EOF after the header
must preserve its original error instead of replacing it with a bad-tag error;
EOF accompanied by a nonzero byte count is an invalid common-IO result. Unknown
statuses are normalized to error. Data/definition/row/group/musical behavior is
unchanged. Production fixed-sector bindings and the target short-read provider
variant remain distinct, and their scopes are recorded in their receipts.

The frozen entry point consumes the ordinary CI's actual echo files and the
compressed actual-fresh reference bundle:

```sh
python3 research/autosiril-native/mix/replay.py \
  --upstream /path/to/completed-frozen-echo --out /tmp/native-mix-frozen
```

It rebuilds native/MIR programs and verifies every expected outcome, operation
trace signature and native image; Ruby and Crystal reference reuse is explicit.
The same source/driver composition is used by fresh and frozen execution.
Artifact audits accept `--upstream` to resolve saved native disks after download;
catalog mutation inputs are read from their retained local artifact directory.

Required CI keeps the entire existing compiler/frozen job unchanged, broadens
the existing fresh job to all five stages, then runs a new frozen fifth-stage
job against the actual frozen echo artifact. The required PR barrier needs all
three jobs. Old echo artifacts remain available alongside the new mix artifacts.
Integration still requires the real required CI runs, downloaded-artifact audit
and PR/main merge; local successes alone do not mark that objective complete.

Earlier evidence links record development checkpoints. Use [the complete gate receipt](evidence/complete-receipt.json) and the portable replay bundle for the current source version.
