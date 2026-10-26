# ADR 0052 Autosiril demand driven bounded pipeline

## Status

Accepted experimental direction. Date: 2026-10-26. First milestone is paired
notes → flattened cells; the complete converter remains the VM reference.

## Context

The functional Nanz/Crystal port matches the Ruby oracle on 11 scores in raw and
optimized MIR2. It materializes sustain for each row and retains grid, flattened,
LNote, echo, mix and rendered forms. Its 87 MB fixed reservation is unsuitable for
128/256 KiB machines. Existing scalar gen/yield lowers to a caller-owned frame
and next function without requiring GC or a private stack.

## Decision

Replace stages incrementally with demand-driven producers and bounded state:
MIDI events → paired intervals → active-note row sweep → flattened cells → echo
window → downmix → patterns → sequential VT2 output. Each edge has an explicit
ordering, lifetime, demand and exhaustion contract. Views do not copy backing
storage; generators compute elements; a randomly indexed paged collection has a
separate storage/recomputation contract. Do not conflate these abstractions.

First replace only the grid/flatten stage. Consume paired intervals ordered by
onset within each source, maintain active handles, visit start/off boundary rows,
and emit flattened entries through an actual scalar generator. Continue-only rows
produce no cell in the reference. Preserve original note-ID order, simultaneous
start/release, zero-length notes and m/d/p selection exactly. Validate ordering and
representable ranges explicitly. The first compact profile may reject oversized
inputs; rejection is preferable to silently wrapping rows or handles.

Use ADR 0051 accessors with flat and one-window simulated backends. All temporary
near accesses end before yield. Partial consumption must avoid traversing the
rest of a score; independent cursor state must be documented (a shared scratch
arena initially allows one live stream). General generator fusion is not required.

Later stages may reuse phase arenas, keep echo history in ring buffers, intern
ornaments/patterns with exact comparison on hash collisions, and stream text to
disk. Preserve echo's originals-before-echo precedence and channel-major downmix
fallback against the original base row. Row-wise fusion is not automatically
semantics preserving. SMF track ordering, NoteOn/Off FIFO and unmatched EOT notes
remain the decoder/pairing stage's responsibility.

## Acceptance and measurement

Compare the exact flattened row/pitch/type/order with Ruby for all 11 existing
scores in raw and optimized MIR2. Test retrigger, release+continue, poly ordering,
zero-length, early stop, sticky exhaustion, source disorder, bank boundaries,
capacity failure and stale leases. Record peak active notes, reserved/used note
bytes, scratch bytes, frame bytes, yielded entries, VM steps and bank switches.

A stage running below 128 KiB is not a claim that the complete converter fits.
The final budget must include code/stack/runtime, parser and every live stage.
The ~191 KB VT2 example requires streamed output on a 128 KiB system. Measure
128/256 KiB budgets and report exhaustion explicitly; do not promise fit in advance.

## Native bounded checkpoint (2026-10-26)

The next [streaming MIDI pairing prototype](../../research/autosiril-native/midi/README.md)
also runs the input stage on target: shared SMF decoder, bounded ordered active
list and actual Crystal UInt32 quantizer. A fresh Ruby oracle matches all1,485
Chronos paired records for1/7/256-byte chunks on raw/final MIR2 and Z80 (nine
executions). Linked image29,990 bytes, observed stack58 bytes; the target holds
only a256-byte input chunk. This is a separate executable from flattening.
Duplicate/channel/EOT/capacity/error corpus and CI replay remain required before
claiming the pairing stage's complete bounded contract.

The follow-up [whole Chronos proof](../../research/autosiril-native/chronos/README.md)
extends the checked resident profile to three 4-KiB pages and1,024 output slots:
all1,485 paired intervals and all three mapped flatten channels match the full
Ruby oracle (771/719/663 events). Raw/final MIR2 and assembled Z80 replay the
whole song and third-page/output capacity controls in21 executions. Code and
static data occupy25,340 bytes; observed stack is74 bytes on the song and80
including boundary controls. This is still paired-note flattening with resident
mapping simulation, not native MIDI parsing, physical banking or a full converter.

The same paired-note sweep and real exported Crystal selectors now have a
[reproducible native Z80 gate](../../research/autosiril-native/README.md):
47 fresh Ruby-backed cases plus10 target lifecycle/error controls run on raw MIR2,
final MIR2 and assembled Z80 (171 executions). Cases include a Chronos24-note
excerpt, p/m/d modes, demand limits, row65535 and page crossings; this is not the
full11-score native conversion acceptance.

One-bank/two-bank profiles reserve682/1364 six-byte records, retain512 active
handles and a bounded output sink. Their complete native images are13,525/17,621
bytes:6,923 code bytes,6,444/10,540 declared global bytes and158 static spill
bytes. Observed below-entry stack peaks are74/80 bytes, not an all-input bound.
The proof checks source/tool hashes, native PC/SP, read-only input, memory write
ranges and independent cold-factory/demand counters. Physical bank switching,
MIDI parsing/pairing and subsequent music/output stages remain separate work.

## Consequences

Avoiding sustain materialization can reduce memory and traffic by orders of
magnitude. Compact row/ID representations impose explicit limits. Pull dispatch
and accessors may cost more instructions; report these costs before attempting
fusion or hiding bank effects. Ruby exact parity remains the compatibility gate.

## References

- [Region handles and banked arenas](0051-region-handles-and-banked-arenas.md)
- [Demand preserving generator fusion](0050-nanz-generator-fusion.md)
- [Functional reference](../../research/autosiril-vm/README.md)
- [Reference grid/flatten implementation](../../research/autosiril-vm/engine.nanz)
