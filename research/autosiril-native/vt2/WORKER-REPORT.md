# WORKER-REPORT — claude-z, VT2 Step9/10/11 checkpoint 2 (2026-10-27)

Owned directory: `research/autosiril-native/vt2/` only. No renderer,
compiler, codegen, workflow, book, fixture or global-config files touched.
No push/PR/release/agents/Herdr/SSH. Original `/Users/alice/dev/autosiril`
used read-only; both hashes verified every run
(autooisee `41f621b5…1cdd4c01`, module_template `8037f85a…7e254ad6`).

The full eventual goal is unchanged: a native file renderer plus VT2,
inheriting every earlier gate, with fresh/frozen CI/audits/PR/main. This
checkpoint is another HOST-proof-only step toward it; no renderer edits, no
native execution, no target results, no finished MIDI→VT2 converter.

## Parent findings addressed (checkpoint 1 → 2)

A. Bounded-memory core refactored for real (`model_vt2.py`): numeric slice
   spans instead of slice lists; pattern bytes streamed row-by-row to a
   spool file with an incremental digest (no joined payload, no ps×50 RAM
   accumulator); file-backed spool/index (u32 id, u32 length, u64 offset,
   digest), u32 order records and bit-per-pattern use flags (one live flag
   byte); dedup compares two spool ranges in ≤256 B chunks; the final VT2
   output is streamed to a file. Proof-side observations (corpus lines,
   statistics, output hashing) are collected explicitly outside the core
   and documented as such.
B. u16 order/id design was wrong: the full admitted profile permits
   render_rows 65567 with pattern_size 1 → 65568 slots and ids/order above
   65535. All records are u32 now. New proven upper-bound cases
   65535/65536/65567 rows and the negative-wrap case (65567 rows, unique
   note at row 0, ps 1, skip −65536; tail starts at row 31, wraps, first
   unique note lands exactly at pattern index 65536). Ruby's actual slicing
   and unusual divisible/floor count rule preserved; nothing capped.
C. Forced-collision proof: 548 cases rerun with a constant candidate digest
   (56 contain real duplicates) — outputs byte-identical to the normal run
   and to Ruby; bytes, not digests, govern dedup. Truncated index (1 and
   47 bytes removed) raises instead of EOF; spool truncated inside a
   declared range raises during streamed compare/copy. The 554 checkpoint-1
   cases are kept unchanged.
D. Transport facts corrected and recomputed from the receipt. Canonical
   Chronos (ps 64 derived from per_beat 8, skip 64 from the canonical
   test_chronos.sh ARGV, REAL ornaments): 62421 B — FITS TRD 65280.
   ps 64 / skip 0: 65222 B — fits (58 B margin). ps 13: 73387 B — exceeds.
   Plain renderer without patterns: 67600 B — exceeds. The checkpoint-1
   claim "65039..73238, typical oversize" was wrong (ps 8 → 45458 and the
   default ps 64 both fit); corrected in DESIGN-TARGET.md.
E. Ornament provenance fixed. The 406 real-rows corpus cases and the 22
   chronos variants are now explicitly labeled partial-real (rows real,
   ornaments/ARGV SYNTHETIC). A canonical pass runs the unchanged original
   Steps 1..8 in-process on the real `test/chronos.mid` with the canonical
   ARGV `chronos.mid 2me,1p,1m 8 6 12 0 64 2 6`, derives the REAL ornament
   map (`L0,0`→0 … 6 definitions) and compares full Step9/10/11: model ==
   oracle byte-exact for all three variants (ps64/skip64, ps64/skip0,
   ps13/skip0), and the output equals the stored reference
   `test_outputs/original/chronos_original.txt` except the single
   environment-derived Author date line (proof pins 1970.01.01; reference
   2025.09.14). Provenance finding: the planning native_chronos rows differ
   from the original-pipeline render in 62 of 1352 rows (upstream native
   chain, not Step9-11); reported, not hidden. kind5 does NOT retain the
   ornament text→id map (open decision recorded).
F. Portable evidence receipt committed at
   `research/autosiril-native/vt2/evidence/checkpoint2-receipt.json`
   (SHA256 `06b45df4…f7e4e7`); full run receipt with per-case rows stays in
   the output directory.

## Command

```sh
python3 -I research/autosiril-native/vt2/prove_vt2.py \
  --autosiril /Users/alice/dev/autosiril \
  --planning /tmp/minz-render-planning-v4 \
  --out /tmp/minz-z-workers/vt2-results/checkpoint2-20261027-a
```

## Results

- OK: 558 corpus cases (+4 upper-bound), 548 forced-collision reruns,
  3 canonical variants; 267511 pattern slots total; every output byte and
  statistic equal (file SHA256, ornaments SHA256, play-order string, use
  flags, wet sizes, pattern counts).
- Suites: 126 synthetic, 4 upper-bound, 406 planning-real (partial-real),
  22 chronos variants (partial-real).
- Output: `/tmp/minz-z-workers/vt2-results/checkpoint2-20261027-a/`
  (`cases.json`, `oracle-vt2/*.vt2`, per-case `spill*/` with
  patterns/index/order/use files and streamed `model.vt2`,
  `truncation/`, `receipt.json`, run receipt SHA256
  `8046af3b31ece955b19dadb2879a30330aa3cba4acab7b56a509471d2be09073`).
  Ruby 2.6.10p210; header date pinned 1970.01.01 (recorded in receipt).

## Limitations (truthful scope)

- HOST proof only: actual original Ruby vs a bounded host model. No native
  sixth-stage execution, no target memory/T-state measurements, no CI runs.
- The bounded core's peak-memory claim is structural (by construction of
  the code paths), not instrumented heap measurement.
- The planning corpus rows are native-chain renders; the canonical pass
  avoids this by rendering original rows in-process, but the 62-row
  divergence between those two row sources is an upstream question for the
  parent (echo/mix stages), not resolved here.
- Ornament derivation for the canonical pass reruns actual Ruby host-side;
  the native chain still needs a wire/decision for ornament text (or
  offsets + orn_repeat) — see DESIGN-TARGET.md §Remaining decisions.

## Commit

Local commit on `spike/autosiril-vt2-z` (see `git log -1`; nothing pushed).
