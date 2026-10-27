# Selected next bounded milestone: Ruby-compatible native text rendering

Selected after downmix PR #270 merged as
`b58fba0c626fd0c82f42dbd6356d56887b4849b0`. Its twelve CI checks passed;
downloaded fresh/frozen artifacts independently passed 1923 executions and six
guard mutants per gate, the unchanged 40626-byte in-process image and five-COM
chain. This document selects work; it is not a sixth-stage implementation claim.

Branch: `spike/autosiril-native-render`, isolated worktree
`/tmp/minz-autosiril-native-render`. Original reference:
`oisee/autosiril` commit `44925349147d5cd4bf3636c3f0eeffa7b46e261c`,
`autooisee.rb` SHA256
`41f621b555fc089effd711319e723494576eb47ea0282847082ec91c1cdd4c01`.

## Bounded Ruby step

Implement original Step8 `render_into_text` (approximately lines922..984), with
the actual `LNote` constructor/setters/renew, Comparable, `note_is_real`,
`note_is_empty`, `enote_is_real`, `print_note`, `print_enote` and `note2txt`
semantics. Pattern splitting/deduplication/play order, VT2 container and VTX
packing are subsequent steps, not substitutes for this one.

First call the actual Ruby function on saved native kind5 MIXED output. Compare
with an independent bounded row-state model, including exact ASCII bytes and
post-row state. This is planning evidence only until target execution. Resolve
float centi-volume to PARAMS indexing and setter side effects from actual Ruby;
do not assume integer formatting or manually recreate apparently equivalent
LNote objects without checking every represented field.

The admitted kind5 profile and previous producer limits must remain unchanged:
three channels, up to9216 total mixed cells (all may be in one channel),64
definitions/4096 expanded bytes, u16 DATA rows and u32 metadata. Use role3's
original global render_rows exactly, not max(channel extents), not max+1. Empty
notes can render zero rows even when an echo base extent was1. Wide/spilled
cells outside the original render range must not extend the loop. Validate the
entire input including such tails and END/EOF rather than ignoring corruption.

Per row preserve:

- cnotes from the three channels; missing cells become distinct empty clones.
- pforms initialized to15, penote initialized to original empty LNote.
- cforms cloned before mutation; update only for real notes at the original point.
- cenote chosen using original enote predicate and Comparable note order,
  including strict ties/source order.
- competing epitch changes kind to m; equal epitch/octave adjustment invokes the
  original note setter. Those setters can recalculate envelope/enote/sample.
- empty-note envelope reset depends on previous forms, previous envelope pitch,
  selected envelope and exact volume15.
- print_note clones and normalizes volume before kind-specific envelope writes;
  reproduce formatting of nil/empty/release and all admitted kinds.
- penote and pforms update only after printing, in original order.

## Streaming and transports

Kind5 is channel-major, while output is row-major. Reuse the proven owned
multi-reader approach or explicit bounded passes, never pretend one offset
is three. Keep fixed row state and lookahead; no full-song arrays or GC.
Definitions are validated losslessly, even if this step's text does not print
them. Specify how later pattern/container stages retain needed definitions.

Decide a deterministic ASCII line/output framing from actual Ruby bytes before
committing a wire contract. Source lines and original row count must survive;
no host musical/formatting callback. Target string emission may use Nanz and
closed Crystal scalar helpers where natural, preserving shared exports.

Produce independently linked sixth CP/M and private Scorpion TRD images that
consume actual fifth-stage files. Measure code/globals/static spills/stack,
RAM reservations, input passes, API calls, output sizes and native T-states.
Preserve CP/M outputs larger than64KB; expose TRD255-sector/space limits with
exact prefixes/publication behavior. Do not cap valid musical inputs merely to
make both transports succeed. Every opened handle must close on every exit;
no borrowed near pointers may outlive paging/reopen.

## Acceptance

Actual Ruby, raw MIR2, final MIR2 and native Z80 must agree on real Chronos and
every supported saved fifth-stage chain, including deliberate failed upstream
chains. Cover envelope competition/ties, octave changes, kinds p/m/d/e,
empty/release/history, centi-volumes, trailing silence, empty original timing,
u16/u32 row boundaries, all-source-one-group and N/N+1 resource limits.

Exercise malformed roles/definitions/cell ordering/metadata/count/END/EOF and
padding; tiny/short/stalled/invalid-status I/O; directory/read/write/close and
publication failures; independent cursor lifetime, reopen/writer snapshot and
cleanup. Profile fixed code/stack/workspace and all16 physical banks, PC/data
execution, ports/paging/IRQ. Retain restoring-write negative controls.

Save source/raw-final MIR/ASM/binaries/files/disks/receipts and standalone six-COM
chain. Preserve all downmix1923+6, echo216+771+6/40links/4COM and earlier gates,
the exact40626B in-process image and four root user files. Require complete
fresh/frozen CI, downloaded-artifact audits and its own verified PR/main merge.
API traps are not physical ROM/FDC/hardware/boot/TPA/fullVT2VTX/MOS/eZ80 proof.

## Coordination and book

Claude owns optimizer/Z80 codegen, `pkg/mir2`, pipeline variant search and ordinary
iterator-chain fusion by default. Our area is Crystal, Autosiril/MIDI,
generator/pull semantics and books. Notify Alice/Claude before changes to shared
MIR/pipeline/iterator fusion or default assembly behavior; one merge at a time,
green required CI. No default codegen change is selected by this plan.

Before a new long implementation run, finish the independently drafted fourth
book chapter about the now-integrated downmix: update status, build and visually
verify EPUB/PDF A5, integrate via a separate PR and publish a new release. The
first edition remains an historical snapshot. i7 remains read-only; do not
control Herdr without HERDR_ENV. Honor user stop/pause.
