# Selected next bounded milestone: native Ruby-compatible downmix

Selected after echo PR #269 merged as `4b82cd7f5`, with11 green checks and
independent audits of downloaded fresh/frozen artifacts. Work branch:
`spike/autosiril-native-mix`. No fifth native musical file stage is claimed yet.

Original reference: `autooisee.rb` downmix877..919. The existing VM implements
later stages, but the independently linked native file chain currently ends at
echo. This milestone will add a fifth file stage on actual saved native
DELAYED files, while preserving all preceding gates and user files.

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
steps below are separate; native mix, kind5 serialization and the fifth musical
COM program remain pending.


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
The whole kernel probe image is7053 bytes:6582 code/bootstrap,323 globals and148
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
lowering, not a fifth independently linked file-stage or five-COM chain. Actual
kind4 parsing, lossless definitions, NOTES render timing, kind5 output, multi-FCB
or TRD orchestration, full malformed/physical-IO matrix, fixed bank placement,
restoring-write mutants, required fresh/frozen CI and PR integration still remain.
