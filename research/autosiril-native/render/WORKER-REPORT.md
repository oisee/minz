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

Parent audit clarification: the independent edge rerun also passed all12
positive engine executions (262174 row states per engine). Current negative
and mutant harness exits at the first raw-MIR2 runtime failure; these four
mutants are therefore raw-MIR2 negative controls, not independent native-Z80
mutant executions. Logs were checked to contain runtime mismatches rather
than compiler syntax errors. `evidence/parent-edges-audit.json` records the
independent command, receipt/source hashes and scope. Full file/native guard
coverage remains an acceptance requirement.
