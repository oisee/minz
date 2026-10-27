# Native renderer worker checkpoint — 2026-10-27

## Result

Completed the first meaningful native-renderer checkpoint: the original Autosiril
three-channel row renderer executes as Nanz/HIR/MIR2 target code and is compared
against the existing actual-Ruby planning corpus on raw MIR2, final MIR2, and an
assembled Z80 image. Both ASCII rows and a complete post-row row-state packet
must match for every engine.

Exact proof command:

```sh
python3 research/autosiril-native/render/prove_kernel.py \
  --planning /tmp/minz-render-planning-v4 \
  --autosiril /Users/alice/dev/autosiril \
  --out /tmp/minz-z-workers/render-results/kernel-20261027-clean
```

Output is preserved in `/tmp/minz-z-workers/render-results/kernel-20261027-clean`.
The clean log is adjacent as `kernel-20261027-clean.log`. The exact measured
receipt is also copied to `evidence/native-kernel.json`.

Commit status: a local commit was requested, but the managed workspace could not
write the worktree index:
`fatal: Unable to create '/Users/alice/dev/minz/.git/worktrees/minz-autosiril-render-z/index.lock': Operation not permitted`.
No SHA is claimed. The intended commit consists solely of the five new owned
render files shown by `git status`; the checkout is otherwise clean.

## Coverage

- 203/203 cases and 10,320/10,320 rows on each engine.
- 609 case/engine results: 203 raw MIR2, 203 final MIR2, 203 assembled Z80.
- 515,988 Z80 text bytes and 794,640 Z80 post-row state bytes matched Ruby.
- Observed line widths are exactly 46, 47, 48, and 49.
- Witnessed transitions: envelope demotion, octave raise, pforms-before-mutation,
  and empty-envelope reset.
- Z80 execution checks passed for return contract, PC trap/image bounds, SP floor,
  IRQ state, memory writes only to globals/backend spills/stack, no executed
  declared data, no unexpected port I/O, unchanged config, and canaries.
- Image: 7,839 bytes total; 790 global bytes, 88 spill bytes, 6,961 code bytes;
  peak observed stack usage 34 bytes.
- Source/MIR/ASM/binary SHA-256:
  - Nanz: `686eb8e722ec5b53c8719047b4e1bde5be1a921d8c42ed69d5b3cbba829a0594`
  - raw MIR2: `01cc0c3ab26f8c1276567ad6cb8a5cd53c4268486deebc047125149aa889316c`
  - final MIR2: `ebdc3efbadb8f0bd9456f630a45a26e77c5a30e6b1bd3cc5a803c1cdaf90ccef`
  - assembly: `e42e240c58e9183ce512ad8fb14d70604de30122cda71726fdbdbc6cc3a128df`
  - COM/binary: `e8158b59563f3357878eb4b13fbe268c4890eaa390f9a5d7e64528d8857c72a0`

## Scope and limitations

- This is a row-kernel proof only. Source records and sink packets use explicit
  fixture traps (`RST 8` and `RST 0x10`); it is not the later independently
  linked CP/M/TRD file program or six-COM chain.
- The host proof generator serializes already-generated actual-Ruby fixtures and
  compares collected target bytes. During target execution it performs no musical
  decision, envelope selection, formatting, row scheduling, setter recalculation,
  or result lookup.
- Target storage is bounded to three 16-byte channel heads, one current row of
  three mutable notes, two retained/sentinel note slots, previous forms/penote,
  one 52-byte line packet, and one 82-byte state packet. Whole-song arrays are
  not loaded into Z80 RAM. The kernel admits 9,216 source cells total and rejects
  unsorted or non-increasing lane rows.
- `render_rows` is a target-side `u32`, including zero and tails beyond the
  render interval; this corpus's maximum is 1,352 rows. Source rows are admitted
  through 65,535, matching the fifth-stage fixture profile.
- Malformed/failure-injection, capacity overflow, cleanup, CP/M, TRD, >64K,
  segmentation, bank-restore, and six-COM gates remain future milestone work.
  No eventual-milestone completion is claimed.

No wakeup daemon was created, legacy STOP markers were not modified, and files
under the future patterns/VT2 worker's ownership were not touched.

## Second checkpoint — high rows and feed guards

The prior run completed but timed out at the worker checkpoint boundary after
writing its authoritative receipt. It was not repeated. The exact command was:

```sh
python3 research/autosiril-native/render/prove_edges.py \
  --baseline /tmp/minz-z-workers/render-results/checkpoint2-baseline-d \
  --autosiril /Users/alice/dev/autosiril \
  --out /tmp/minz-z-workers/render-results/checkpoint2-edges-l
```

Preserved stdout (`checkpoint2-edges-l.log`) says:
`PASS 4 actual-Ruby high-row cases; 262174 rows on each engine; 4 feed checks; 4 mutants`.
The proof first checks the pinned Ruby source SHA-256
`41f621b555fc089effd711319e723494576eb47ea0282847082ec91c1cdd4c01`, then executes actual
Ruby for `65535`, `65536`, `65567`, and empty-input `65536` rows. Raw MIR2, final MIR2,
and assembled Z80 each match all 262,174 exact text/state rows across those four cases
(12 engine executions). A final event is placed at source row65,535; rows after it use
unchanged state. Kind5 source row fields remain `u16`, so no impossible source index above
65,535 is fabricated. The render loop remains `u32`.

The state wire now preserves both historical encodings: a 77-byte packet with `u16` row
through65,535, and a 79-byte packet with `u32` row from65,536 upward. The fixture decoder
accepts both lengths and reports `stateRows` incremented at each sink packet rather than
dividing collected byte length by77. After the timeout, the remaining console-only use of
`len(actualState)/77` was fixed. The smallest replay was a zero-row case:

```sh
cd minzc
go run /tmp/minz-z-workers/render-results/checkpoint3-reporting/render-kernel.go \
  --root .. \
  --source ../research/autosiril-native/render/kernel.nanz \
  --input /tmp/minz-z-workers/render-results/checkpoint3-reporting/zero.json \
  --out /tmp/minz-z-workers/render-results/checkpoint3-reporting/measured
```

It printed `PASS zero_rows/{raw-mir2,final-mir2,z80} rows0`; its receipt records zero rows
for each engine. The reporting replay regenerates the generated driver from the baseline
header plus the updated owned driver body. It does not replace the authoritative edge run.

Kernel-feed guards reject duplicate rows, unsorted rows, invalid feed status, and a
9,217th source cell (`fault958`, `fault958`, `fault904`, and `fault983` respectively).
Protective target mutants that accept duplicates/capacity overflow, truncate high-row
trace, or skip demotion refresh all fail. These are fixture-feed checks, not malformed-file
or CP/M I/O coverage. On each source feed the kernel drops its cached envelope-note state
and recomputes it from admitted records; a future lossless kind5 file adapter must validate
source/envelope-note coherence before relying on that cache (or preserve original behavior
precisely).

Portable evidence is `evidence/edges.json`
(SHA-256 `c3296f0963a9774d91f32130c68c47e4f2aba710d3ae178cc9e29161b42792f8`). It records
per-case source counts, Ruby hashes, engine hashes/counts, checks, mutants, the authoritative
receipt SHA-256, and the limitations. The baseline remains exactly203 cases and10,320 post-row
states per engine. Independently linked CP/M/TRD file stages, physical128-byte-record framing,
full file failure/cleanup matrices, all16 banks, six-COM integration, inherited CI/audits,
PR/main integration, and VT2 remain unfinished.

## Third checkpoint — CP/M adapter work retained, execution not yet proven

A separate target adapter (not a fixture transfer trap) was started by composing the proven
row kernel with `cpm.nanz`, replacing `RST8/RST16` with BDOS CALL5. It uses a full
validation pass, three owned input FCB/cursor/buffer contexts, streaming text and state
writers, and writes `META.BIN` plus `REND.INF`. `META.BIN` preserves the lossless kind5
definition fragments and role2/role3 metadata for a later VT2/container stage. `REND.INF`
records text/state logical lengths and original render rows so zero-padded physical
CP/M128-byte records are not mistaken for exact logical bytes. No target whole-song array
is introduced; definitions remain bounded to the admitted4,096-byte companion store.

The composed source parses through Nanz/HIR:

```sh
python3 - <<'PY'
from pathlib import Path
r=Path('research/autosiril-native/render')
k=(r/'kernel.nanz').read_text().split('@extern(0x0008)',1)[0]
Path('/tmp/render-cpm-parse.nanz').write_text(k+'\n'+(r/'cpm.nanz').read_text())
PY
cd minzc
go run /tmp/minz-z-workers/tmp/render/parse-check.go /tmp/render-cpm-parse.nanz
```

The focused real-file proof command is:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 research/autosiril-native/render/prove_cpm.py \
  --artifacts /tmp/minz-ci270-artifacts/autosiril-mix-fresh \
  --autosiril /Users/alice/dev/autosiril \
  --planning /tmp/minz-render-planning-v4 \
  --out /tmp/minz-z-workers/render-results/checkpoint3-cpm-z
```

It selects the actual native Chronos kind5 file and an actual empty kind5 file and compiles
raw MIR2, final MIR2 and an independently assembled Z80 COM image. It does **not** pass:
`raw-mir2 fault 956` (`/tmp/minz-z-workers/render-results/checkpoint3-cpm-z/prove-cpm.log`).
The failure is in END/count/magic cursor decoding. A temporary diagnostic that skipped the
magic tail then hit the MIR2 VM's1,000,000,000-step gas ceiling
(`checkpoint3-cpm-ac/prove-cpm.log`); that diagnostic was removed and strict checks restored.
Therefore no CP/M file-stage success, output equivalence, budget, cleanup, >64K, malformed
file, or six-COM claim is made. The smallest next command is the proof command above after
instrumenting `cursor_word`/`cursor_end`; investigate evaluation order and trace bytes around
the END record, then profile the scan/render loop before raising any execution budget.

Current adapter hashes:

- `cpm.nanz`: `02d378231622f4ecb9bc6e1a3ef4ce6fbc498c65bf487356bd530d825b0f2784`
- `cpm-driver.go.txt`: `d00c96087a45015d15b82f270260e7c0b62c07fc800b8cc90fddfb9175b398b5`
- `prove_cpm.py`: `9c90593a04139fe3909d53f9ae555356068f7f17703860cce49507f9c277ad4f`

The full obligations remain: finish all actual saved kind5 chains, deliberate upstream
failures, malformed framing/order/padding/EOF, BDOS failure/cleanup/reopen, >64K streamed
output, all16 physical banks, TRD255-sector behavior, six-COM integration, VT2, fresh/frozen
inherited CI/audits, and PR/main review. No shared compiler/MIR/pipeline/default-codegen
files, root user files, legacy STOP marker, SSH credentials, push, PR, or merge were touched.

Parent audit clarification: the independent edge rerun also passed all12
positive engine executions (262174 row states per engine). Current negative
and mutant harness exits at the first raw-MIR2 runtime failure; these four
mutants are therefore raw-MIR2 negative controls, not independent native-Z80
mutant executions. Logs were checked to contain runtime mismatches rather
than compiler syntax errors. `evidence/parent-edges-audit.json` records the
independent command, receipt/source hashes and scope. Full file/native guard
coverage remains an acceptance requirement.

## Fourth checkpoint WIP (parent checkpoint after runner timeout)

The adapter remains UNPROVEN: no successful standard tri-engine CP/M receipt.
The actual kind5 ordering is definitions, cells, four trailing metadata records,
END/count/magic. An independent GLM review copy reached raw-MIR2 fault0 after
fixing ordering, then failed STATE.BIN physical-length validation. Latest edits
also adjust state/REND.INF framing; the30min runner ended before a completed
proof/report. Earlier adapter hashes/commands above are historical. Current
source hashes, next fresh command and explicit scope are recorded in
`evidence/cpm-wip-checkpoint4.json`. Parent only removed trailing whitespace
before retaining this WIP; no execution success is inferred from that edit.
Next worker must define state/metadata companion framing precisely and prove
real-file execution against fresh actual Ruby without relaxing old contracts.

## Fifth checkpoint — actual CP/M tri-engine proof for all saved kind5 files

The retained WIP now passes the standard proof and the full legacy saved-input matrix:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 research/autosiril-native/render/prove_cpm.py \
  --artifacts /tmp/minz-ci270-artifacts/autosiril-mix-fresh \
  --autosiril /Users/alice/dev/autosiril \
  --planning /tmp/minz-render-planning-v4 \
  --out /tmp/minz-z-workers/render-results/checkpoint5-cpm-n
```

Result: `PASS 114 actual-kind5 CP/M executions`. This comprises all25 positive
saved native kind5 inputs on raw MIR2, final MIR2, and assembled Z80 (75
executions) plus 13 wire/BDOS guards on each engine (39 executions). Chronos
remains exact at67,600 ASCII bytes and105,456 state bytes; the empty input
produces zero logical text and state bytes.

The proof decodes each saved MIXED file, reruns the pinned original Ruby for all
25 cases in a fresh output tree, compares those fresh outputs with admitted
planning evidence, and only then compiles/executes the target. Every expected
state row is independently decoded by row index, length, forms, previous note,
and three post-row notes. After execution, all75 positive engine outputs are
decoded again from disk, including physical zero padding and logical lengths.

The explicit observational contracts are:

- `STATE.BIN v1`: each row is length-prefixed—a77-byte payload withu16 row
  through65,535, or79-byte payload withu32 row from65,536 upward. Empty input
  writes zero logical bytes.
- `META.BIN v1`: an`ARMD` companion, not kind5. Its14-byte header carries
  version, source kind, raw definition-fragment count, fixed role-record count4, and reserved
  zero space; exact definition fragments and four role records follow. No cells
  are implied.
- `REND.INF v1`: exact 17 logical bytes (`RINF`, version, text/state logical
  lengths, original render rows), padded to one128-byte CP/M record with zeros.

Guard coverage now rejects wrong header kind, malformed definition data,
cell-order regression, metadata-before-cells, role2 extent corruption, END count
and magic corruption, nonzero physical padding, and injected BDOS input-open,
input-read, output-create, output-write, and output-close failures. Guards run
on all three engines, retain exact fault codes, verify unchanged input, and
check live-handle cleanup. Non-partial failures must create no output; write and
output-close failures must retain record-aligned partial output.

The final image is28,474 bytes (21,589 code,6,432 globals,453 backend spills).
The fresh rerun also preserved the203-case/10,320-row kernel baseline and the
262,174-row high-row edge proof. Exact hashes, contracts, coverage, paths, and
remaining limitations are in `evidence/cpm-checkpoint5.json`.

This is not a full renderer milestone claim. TRD16-bank/IO/publication,
canonical six conversion, six-COM integration, inherited fresh/frozen CI audits,
VT2/VTX, and main/PR review remain open.

## Checkpoint 6: admission profile holes closed (2026-10-27)

All five open admission items from the pause handover are closed on CP/M;
162 executions pass (54 cases × raw MIR2 / final MIR2 / Z80), up from 114.

1. **Definition profile.** The adapter stores semantic payload (4096 bytes,
   64 lengths) instead of 256 raw 16-byte fragments, and rebuilds canonical
   fragments for `META.BIN`. Distinct-id count and raw fragment count (up to
   568) are separate; cell references are checked against distinct ids; an
   unfinished definition before the first cell is rejected.
2. **Independent decoder.** `decode_definitions` validates ids, fragment
   framing and the aggregate limit; `ruby_case` no longer caps raw fragments
   at 64. The `ARMD` header count is the raw fragment count.
3. **Cache coherence.** Cells whose cached fields differ from the original
   `LNote` constructor (enote, and sample/envelope for kind `e`) fault 986;
   a nonzero reserved byte or a kind-`e` note outside `ENV_OFFSETS` faults 987.
   Previously these were normalized or, for the reserved byte, the cell was
   silently skipped.
4. **REND.INF** is 17 logical bytes; the audit checks padding from byte 17.
5. **Canonical6.** The actual canonical6 `MIXED.BIN` (sha256 `8aa9cfa2…`) is a
   fresh-Ruby case, and ARRANGE@6 → ECHO → MIX → RENDER run as four COM
   programs under mze (136M / 403M / 1324M / 621M T-states) with every
   intermediate equal to the verified chain. PAIR/NOTES outputs are reused
   byte-identical, not re-executed.

New edge cases: 9-byte definition, 64 distinct definitions, 4096-byte payload,
568 fragments, reference to definition 63; rejected 65th id, 4097 bytes,
incomplete and reordered fragments, undefined reference, four cache
corruptions and the reserved byte. Run against the old adapter, the 4096-byte
and 568-fragment inputs were rejected and the incomplete/undefined/cache/
reserved inputs were accepted. Image 29,339 bytes. `kernel.nanz` is unchanged,
so the kernel and high-row proofs were not rerun. Details:
`evidence/cpm-checkpoint6.json`.

Still open: TRD 16-bank renderer, native VT2/VTX writer, CI wiring for this
proof.

## TRD checkpoint 1 (2026-10-27)

The renderer now runs on TRD over the actual native TRD mix disks. 75/75
executions pass (25 cases × raw MIR2 / final MIR2 / Scorpion Z80), and the
CP/M proof still passes 162/162 after the refactor.

- `common.nanz` holds the transport-neutral kind5 reader, validation and
  META/REND encoders; `cpm.nanz` and `trd.nanz` only supply I/O.
- The TRD binding allows one live output, so text and state come from two
  identical render passes. Each stream is split into `OUTPUTn T` /
  `STATEn S` segments of 65,280 bytes (255 sectors); their concatenation
  must equal the CP/M logical stream. All seven Chronos variants need two
  text and two state segments. Empty streams publish no file.
- The 32,235-byte image fits after compacting the shared encoders and moving
  the stack floor to 0xBF00 (254-byte budget, 60 bytes measured). The
  fixed/paged RAM, PC, IRQ and port guards of the mix specialization are kept.

Not yet covered on TRD: profile-edge and canonical6 cases (no native TRD mix
disk exists for them), sector failures, disk-full and malformed inputs,
renderer guard mutants, CI. See `evidence/trd-checkpoint1.json`.

## TRD checkpoint 2: failures and disk-full (2026-10-27)

297/297 executions pass (99 cases × 3 engines):

- The 25 native TRD mix disks, plus the profile-edge and canonical6 cases on
  seeded disks that hold only the actual `MIXED M` bytes (labelled as such).
- A failure injected at each of the 58 sector calls of `chain_single`, and at
  six points inside Chronos, including during the second text segment.
- Disk full: 12 free sectors succeed, 11 fail on creating `META`, 1 fails on
  the first text write; Chronos with room for exactly one text segment fails
  creating the second.

Failure cases are checked by invariants rather than one expected code. Every
published segment must be an exact prefix in stream order, and free-space
accounting must match the published files. The one admitted exception is the
inherited binding's non-atomic `finish()`: a fault 903 between the catalog
write and the system-sector write leaves the last file visible but uncounted.
Fixing that ordering is a separate change to the shared binding. See
`evidence/trd-checkpoint2.json`.
