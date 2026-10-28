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

The fourth book chapter is now integrated through PR #271/main9cf3a71d5;
the checked93-page EPUB/PDF edition is published as
[book-compiler-days-2026-10-27](https://github.com/oisee/minz/releases/tag/book-compiler-days-2026-10-27).
The first edition remains an historical snapshot. Alice requested a break after
this release: do not start the native renderer without her next command.
See [full handover](HANDOVER-2026-10-27.md). i7 remains read-only; do not control
Herdr without HERDR_ENV. Honor user stop/pause.

## First actual reference smoke (not native rendering)

A short reference experiment ran while the separate book PR #271 was in CI.
It calls the original Ruby renderer on an actual saved native Chronos MIXED file;
there is no independent row model and no sixth-stage target execution yet.

```sh
python3 research/autosiril-native/render/reference_smoke.py \
  --input /path/to/actual-native-mix/cpm/measured/native_chronos/z80/MIXED.BIN \
  --autosiril /path/to/autosiril --out /tmp/render-reference
```

The kind5 input is decoded and every represented LNote field is checked after
the original Ruby constructor. The actual canonical input has54912 physical
bytes,3215 cells and three extents1353. Original render_rows1352 produces
exactly1352 text lines of49 bytes each,67600 bytes including LF separators.
Output SHA256:
`cce454680b5275bc5daa7943638dbec482a518d405e3984df164ec97ee5e7b47`.

Actual Ruby PARAMS indexing coerces7.35 to index7 and10.5 to10: it emits `7` and
`A`, not rounded volumes. The recorded Ruby version is part of this evidence.
This is not yet a proof for other inputs or another language implementation.

Crucially,67600 bytes already exceed a single255-sector TRD file (65280B).
The next design must specify explicit output segmentation or a truthful size
failure; silently truncating Chronos or shrinking the admitted input profile
would not implement the selected stage. The smoke does not establish that all
future valid inputs have49-byte rows; wider sample/parameter edge cases still
need actual-Ruby tests before fixing an output wire.

## Actual Ruby versus independent row-state model

The subsequent planning check passes203 cases and10320 post-row observations:
25 saved native fifth-stage inputs (19 valid streams and six Chronos group
variants), plus178 synthetic row/history/parameter cases. The deliberately
failed upstream chain is inventoried separately as an absent MIXED file with
fault951; it is not treated as a successful renderer input.

```sh
python3 research/autosiril-native/render/prove_planning.py \
  --upstream /path/to/complete-native-mix --autosiril /path/to/autosiril \
  --out /tmp/render-planning
```

`oracle.rb` calls the unchanged original renderer. A read-only TracePoint at
the outer row-block return observes pforms, penote and all three current notes,
including cached pitch/octave/envelope fields. The observer's line location is
guarded by the exact original-source hash. `model.py` independently computes
the row transitions and output using integer centi-volumes; only the domain's
constant tables come from original Ruby. Every output byte and observed state
matches. The model does not mutate its input fixture.

Witnesses include envelope demotion, octave adjustment, pforms captured before
note mutation, and empty-note envelope reset. All four observed text widths
46..49 occur: original PARAMS indexing can return nil for sample32/255, leaving
that character absent. A fixed49-byte logical-row assumption is therefore not
the admitted language behavior.

The computation uses three iterator heads and bounded row state; input fixtures,
Ruby arrays and collected proof sinks still live on the host. This is a host
algorithm/semantic comparison, not a target RAM bound or a native sixth stage.
See [planning receipt](evidence/planning.json).
