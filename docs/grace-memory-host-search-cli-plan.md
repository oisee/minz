# Next stage: explicit eight-candidate search CLI

Plan reviewed independently in476e266f and a668ee24. Implementation release
waits for the current integrated search pipeline gate. No CLI code is changed
by this document.

The new mode is `--memory-host-arena-append-search DIR`, with existing
`--memory-host-workload FILE`. Two optional named association inputs are
`--memory-host-search-legacy-association FILE` and
`--memory-host-search-preserve-association FILE`: both present means replay
the exact supplied whole v2 envelopes; both absent means collect both fresh.
One present, explicitly empty new flags, orphan flags, or old/fixed modes
combined with search are refused. Values and Cobra Changed are checked.
Ordinary compilation and existing fixed-policy routes remain as before.

Cache bounded source bytes once. Check exact known source SHA, independently
parse with the supplied logical filename, CPM and complete single-unit map
before any native collection. No imports/source substitution. Factories
parse those immutable owned bytes; the API independently checks full HIR.
Read/own strict workload and both association buffers. Each association is
limited to32MiB and the pair to64MiB. Malformed second supplied input must
fail before CPU or artifacts. Search itself performs both fresh replays.

Actual GoalSpeed/GoalSize search all eight candidates; Osize sets the search
objective only. Preserve deterministic Legacy-first primary ties. Select
target through the typed policy/mask reference in the complete search;
write exact selected ASM/BIN and optional -o ASM without recompilation.

Reserved outputs: source.nanz, workload.json, input-legacy-association-v2.json,
input-preserve-association-v2.json, search-v1.json, decisions-search-v1.json,
selected.a80, selected.bin; policy-legacy and policy-preserve each contain
selection-v2.json and original/arena/append/both directories with
baseline-v2.json, target.a80, target.bin, final.mir2 and allocation.json.
All50 prospective files plus -o participate in canonical-parent/symlink and
SameFile checks against source/workload/both inputs and each other before
CPU; recheck immediately before writes. Build JSON payloads before mkdir.
Semantic errors create no artifact directory; IO errors may leave partial
files and must remain errors.

Decision schema `minz.memory-host-arena-append-search-cli-decision/v1` binds
actual goal, typed selected policy/mask and choices, source SHA/name, SHA of
exact saved search bytes and both exact saved association bytes, selected
BIN SHA, measured T-states and bytes. Complete nested search is authoritative;
no duplicate editable cost/proof or partial winner fallback. Supplied pretty
association bytes remain exact even when typed allocation JSON normalizes
internally after SHA validation.

Focused checks cover dispatch/emptyChanged/conflicting modes, strict paired
inputs and both caps, source gate before collection, all-file alias inventory
including hardlinks/symlink parents and second policy tree, short fresh
collect/paired replay/-o identity, actual short Size ordering, and refusal
without writes when either policy fails. Reuse existing independent API
ownership/ranking checks; synthetic inputs cannot attest native costs.

After source freeze and two reviews, preserve an exact CLI executable and
run full30 actual Speed and actual Size separately, plus bounded short
collect/replay. Independent saved-only auditors verify inputs/executable,
all eight candidates, native observations/state/events, eleven profile
counters, proofs and decision/file SHA. An offline alternate ranking must
remain labelled derived. Parent gates, publication and whole offline sync
close the stage.
