# WORKER-REPORT — claude-z, VT2 Step9/10/11 checkpoint 1 (2026-10-27)

Owned directory: `research/autosiril-native/vt2/` only. No renderer,
compiler, codegen, workflow, book, fixture or global-config files touched.
No push/PR/release/agents/Herdr/SSH. Original `/Users/alice/dev/autosiril`
used read-only; both hashes verified
(autooisee `41f621b5…1cdd4c01`, module_template `8037f85a…7e254ad6`).

## What was done

Reproducible actual-Ruby vs independent bounded host model proof for the
remaining Step9 split_into_patterns/dedup/play order, Step10
render_ornaments, Step11 module_template/write_vt_output, plus a
target-facing design. HOST planning only — no native execution, no target
results, no finished MIDI→VT2 converter claimed.

Files (this directory): `oracle_vt2.rb` (evals unchanged original functions,
hash/line guarded, pinned clock, GC flush because the original never closes
its file), `model_vt2.py` (independent bounded model: file-backed pattern
spill + record index, collision-safe byte dedup, no unbounded Hash),
`prove_vt2.py` (corpus generator + comparator), `DESIGN-TARGET.md`
(semantics, metadata inventory, target plan, transport findings, open
decisions).

## Command

```sh
python3 research/autosiril-native/vt2/prove_vt2.py \
  --autosiril /Users/alice/dev/autosiril \
  --planning /tmp/minz-render-planning-v4 \
  --out /tmp/minz-z-workers/vt2-results/checkpoint-20261027-a
```

## Results

- OK: 554/554 cases, 5302 pattern slots; every output byte and statistic
  equal (file SHA256, ornaments SHA256, play-order string, use flags, wet
  sizes, pattern counts).
- Corpus: 126 synthetic boundary cases (pattern-count grid, nil/empty slice
  edges, negative/beyond skip, duplicates, blank rows, ornaments incl. 64
  definitions and invert collisions, argv/title variants, 4096-byte
  pattern-size boundaries), 406 real planning cases (all 203 rendered
  outputs at ps16 and ps64/max_row−1), 22 Chronos variants (dividing /
  non-dividing pattern sizes 5..1351, skips ±13/±1352/±1353, max_row 0).
- 62 cases exercise real dedup; ceil-mutant negative control disagrees on
  462 cases (the divisible +1 / nondivisible floor rule is not ceil).
- Output: `/tmp/minz-z-workers/vt2-results/checkpoint-20261027-a/`
  (`cases.json`, `oracle-vt2/*.vt2`, per-case spill, `receipt.json`).
  Ruby 2.6.10p210; header date pinned 1970.01.01 (recorded in receipt).

## Key findings (details in DESIGN-TARGET.md)

- `ARGV[1]` is the source-mapping argument, not a human title; Step11 writes
  it verbatim into `Title=`; absent → empty. `--format` pair is removed from
  ARGV before `ArgList`. kind5 retains none of title/argv/date — wire
  decision needed.
- patterns==0 edge: `"L"[-1]=''` makes the play order string empty.
- nil-slice and empty-slice patterns are the same `""` and dedup together;
  used-but-empty patterns stay in the play order but emit no block.
- Chronos VT2 measured 65039–73238 B across ps 5..1351: exceeds the TRD
  255-sector file for typical sizes (honest size failure required); CP/M
  156800 B holds all. Naive line RLE grows the text (69822 B) — not a fix.
- Target plan: bounded pattern accumulator (ps×50 B), file-backed spill +
  record index, checksum-candidate + byte-verified dedup, u16 order list
  (≤8.2 KB), two-pass publication so the header play order needs no Hash.

## Remaining decisions for parent

Wire fields (title/argv/date), ornament text→id source for the native chain,
target checksum width, pattern_size/skip provenance, provisional-file vs
in-place TRD publication. See DESIGN-TARGET.md §Remaining decisions.

## Commit

Local commit on `spike/autosiril-vt2-z` (see `git log -1` for SHA; nothing
pushed). A previous session's uncommitted `oracle_vt2.rb`/`model_vt2.py`
drafts were completed and fixed as part of this checkpoint.
