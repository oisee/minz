# VT2 Step9/10/11: proven semantics and target-facing design (checkpoint 3)

Scope of this checkpoint: a reproducible actual-Ruby vs independent bounded
host model proof for the remaining original Step9 `split_into_patterns`
(pattern split, deduplication, play order), Step10 `render_ornaments` and
Step11 `module_template`/`write_vt_output`, plus a concrete target-facing
plan. This is HOST planning evidence only. There is no native sixth-stage
execution, no target result and no finished MIDI→VT2 converter here. The
full eventual goal — a native file renderer plus VT2, inheriting every
earlier gate, with fresh/frozen CI, audits and PR/main — is unchanged; this
checkpoint is one more verified host step toward it.

Original reference: `/Users/alice/dev/autosiril` (read-only),
`autooisee.rb` SHA256
`41f621b555fc089effd711319e723494576eb47ea0282847082ec91c1cdd4c01`,
`module_template.rb` SHA256
`8037f85a1c92cc32e494ba6b62a70be99fae5de041050addbf05844b7e254ad6`.
Oracle: `oracle_vt2.rb` evals the unchanged original functions (guarded by
exact source hash and line positions, read-only) and calls them directly;
for the canonical pass it additionally runs the unchanged original Steps 1..8
in-process on the real `test/chronos.mid`. Model: `model_vt2.py` computes
everything independently with bounded, file-backed state (see below); only
the static template literal text is taken from the original
`module_template.rb` source. Prover: `prove_vt2.py`.

## Proven semantics (all bytes and statistics equal; 558 corpus cases + 548 forced-collision reruns + 3 canonical variants)

- Pattern count: divisible `max_row % pattern_size == 0` gives
  `max_row/pattern_size + 1`; nondivisible gives floor division. This is NOT
  mathematical ceil. `max_row == 0` gives exactly one pattern.
- `patterns == 0` (possible when `0 < max_row < pattern_size`): the loop body
  never runs and `wet_play_order_txt` is `"L"`, then `"L"[-1] = ''` makes it
  the EMPTY string. The header then reads `PlayOrder=` with nothing after it.
- Slicing follows Ruby `Array#slice(start, length)` edges with
  `start = skip_lines + i*pattern_size`: `start == lines.size` returns `[]`
  (empty pattern), `start > size` returns `nil`. Both produce the same `""`
  pattern text, so nil-slice patterns dedup against empty-slice patterns.
  Negative `skip_lines` normalizes per-pattern (`size + start`), out-of-range
  negative start yields nil; later patterns can recover into range. The
  model implements these edges as NUMERIC spans (`slice_span`), never as
  materialized slices.
- Dedup key is the exact pattern bytes (lines joined with LF, trailing LF).
  First occurrence gets `use_patterns[i] = true`; later duplicates keep
  `false` and the play order lists the FIRST duplicate's index. A partial
  tail pattern (fewer lines) is never equal to a full pattern.
- Emission (Step11): `puts header` then for each `i` with
  `use_patterns[i] && pattern_text != ""`: `puts "\n[Pattern#{i}]"` and
  `puts pattern_text`. Ruby `puts` does not double the trailing newline of
  `pattern_text`, so each pattern block is exactly
  `\n[PatternN]\n` + pattern bytes. Used-but-empty patterns (index) still
  appear in the play order but emit no block.
- Step10: `ornaments.invert` (later insertion wins on duplicate ids), sort by
  id, skip id 0, emit `\n[Ornament#{id}]\n#{text}\n` per id.
- Step11 header: `Title = ARGV[1].to_s` (empty when ARGV[1] absent),
  `Author = "oisee/siril^4d " + Time.new.strftime("%Y.%m.%d")`,
  `PlayOrder = wet_play_order_txt`, `ArgList = ARGV.join(' ')`, then a fixed
  literal `[Sample1]..[Sample31]` block. The template's date interpolation
  itself contains quotes; extraction must be interpolation-aware.
- The original `write_vt_output` never closes its file; the oracle forces the
  flush (GC) before hashing. Oracle plumbing only.

## Upper-bound profile coverage (new in checkpoint 2)

The earlier design wrongly assumed `pattern_size >= 16` and u16 pattern
ids/order. The full admitted kind5 profile permits `render_rows` up to 65567
and `pattern_size` 1, i.e. up to 65568 pattern slots with order indices and
`[PatternN]` ids above 65535. Proven cases (all bytes equal to Ruby):

| case | rows | ps | skip | patterns | unique | note |
|---|---:|---:|---:|---:|---:|---|
| ub_rows65535_ps1 | 65535 | 1 | 0 | 65536 | 3 | ids fit u16 exactly |
| ub_rows65536_ps1 | 65536 | 1 | 0 | 65537 | 3 | first id 65536 overflows u16 |
| ub_rows65567_ps1 | 65567 | 1 | 0 | 65568 | 3 | profile maximum |
| ub_rows65567_ps1_skipneg65536 | 65567 | 1 | -65536 | 65568 | 2 | negative wrap |

The negative-wrap case exercises Ruby's unusual count rule plus per-pattern
negative-start normalization at the profile edge: pattern 0 reads row
`65567-65536 = 31`, patterns wrap at index 65536, and the single unique row
0 is reached exactly at pattern index 65536 (first-occurrence/use flag and
play-order entry prove the index is not truncated to u16 anywhere).
Consequence for the design: pattern ids, order entries and any index-file
record fields are u32; metadata is file-backed, not RAM arrays.

## Forced-collision and truncation proofs (new in checkpoint 2)

- 548 cases were rerun with a deliberately constant candidate digest (every
  pattern "collides"); 56 of them contain real duplicates. Outputs are
  byte-identical to the normal run and to Ruby in all 548: equality is
  decided by streamed byte comparison of the stored ranges, so a checksum
  collision (forced or accidental) is never musical equality. The target
  checksum can therefore be any cheap 32-bit sum/CRC.
- A truncated index file (partial final record, tested with 1 and
  RECORD−1 = 47 bytes removed) raises `IndexTruncated` at open/scan — it is
  never treated as EOF. A spool file truncated inside a declared pattern
  range raises `SpoolTruncated` during streamed comparison/copy — never a
  silent partial match.

## Bounded core (host model, checkpoint 2 refactor)

The checkpoint-1 model still materialized slice lists, whole joined
patterns, payload copies and the entire output bytearray, and its design
rested on a `pattern_size × 50` RAM accumulator. All of that is gone. The
intended-bounded core in `model_vt2.py` now:

1. computes slice edges as numeric spans (never lists);
2. streams each pattern's bytes row-by-row into a spool file while updating
   an incremental digest (no pattern is ever fully held in RAM; no ps×50
   accumulator);
3. appends one fixed record per pattern to an index file (u32 id, u32
   length, u64 offset, digest) and scans it record-by-record for dedup;
4. verifies candidates by comparing two spool ranges in ≤CHUNK (256 B)
   pieces; dedup order/use decisions go to file-backed u32 order records
   and bit-per-pattern use-flag records (exactly ONE partially-filled flag
   byte is live in RAM);
5. streams the final VT2 to a file: header, then each used non-empty
   pattern copied from the spool in ≤CHUNK pieces; the play-order string is
   reproduced from the order file with Ruby's `"L"[-1]=''` framing.

Checkpoint 3 closes the remaining checkpoint-2 gap: the header itself is
now STREAMED, not assembled. The template literal (a fixed source
constant) is parsed once into bounded literal/interpolation segments
(`template_segments`); `write_vt_output` writes each literal fragment
directly, streams `#{play_order}` as one u32 order entry at a time read
from the order file (Ruby `"L"` framing: `"L"[-1]=''`, empty for zero
patterns), and streams `#{ornaments_txt}` as one bounded ornament fragment
at a time (≤64 definitions of ≤4096 expanded bytes; title/date/ArgList are
bounded metadata strings). `run_case` builds the proof statistics (order
string, use flags, wet sizes, joined ornaments text) only AFTER the output
file is finished; the output path no longer calls `order_text_from_store`
or assembles any header/order/ornaments string.

Peak core state: one row, one index record, one flag byte, ≤CHUNK compare
buffers, counters. Proof-side observations (corpus line lists, statistics
arrays, hashing finished outputs) are collected explicitly OUTSIDE the core
and after computation; nothing in the core depends on them. The two-pass
publication plan below maps 1:1 onto this structure.

## Canonical Chronos: real ornaments, real ARGV (new in checkpoint 2)

The corpus rows are real, but the checkpoint-1 chronos cases used SYNTHETIC
ornaments/ARGV; their sizes are NOT original-module sizes. The canonical
pass now runs the unchanged original Steps 1..8 in-process on the real
`test/chronos.mid` with the canonical `test/test_chronos.sh` invocation
(`chronos.mid 2me,1p,1m 8 6 12 0 64 2 6`; per_beat 8 → derived
pattern_size 64; skip_lines 64; orn_repeat 2; max_offset 6; Title = ARGV[1]
= the source mapping `2me,1p,1m`), derives the REAL ornament map
(`L0,0`→0 skipped, `L0,0,3,3`→1 … 6 definitions, matching the stored
reference byte-for-byte) and compares full Step9/10/11 output against the
model with the pinned 1970.01.01 date:

- model == oracle in every byte/statistic for all three variants;
- `test_outputs/original/chronos_original.txt` (2025.09.14) equals the
  pinned-date canonical output except the single Author date line —
  the entire remainder of the 1477-line reference module is reproduced
  exactly, which also confirms the real ornament derivation independently.

Provenance finding: the planning `native_chronos` rows (native chain, proven
equal to Ruby render *on the same kind5 input*) differ from the original
full-pipeline render in 62 of 1352 rows. The canonical variants therefore
use the original render; the planning-row chronos cases remain labeled
partial-real (rows real, ornaments/ARGV synthetic). Checkpoint 3 pins this
divergence to its first difference and mechanism — see
[CHRONOS-DIVERGENCE.md](CHRONOS-DIVERGENCE.md): identical invocation
settings; first diff at row 259, `noteB` block (channel 1), note
`A#3`→`D#3` / ornament `.`→`4`; root cause is inside the native chain's
kind5 fixture (62 cells, all channel 1: the native `p`-channel path
collapsed a `{51,52}` dyad to note 51/ornament `L0,0,1,1` where Ruby's
`squize_ornament` mediana/max_offset filter collapses the actual `{51,58}`
dyad to note 58/ornament `L0,0`; the native channel 2 renders the same
source row correctly). A real upstream native-chain content divergence,
not a fixture-profile or invocation difference; parent coordinates the fix.

## Transport measurements (corrected; TRD file limit 65280 B / 255 sectors)

Canonical plain render (no patterns/ornaments) is 67600 B and exceeds TRD.
The complete VT2 module, measured with REAL ornaments/ARGV/date framing on
the original render rows (actual Ruby writer):

| configuration | patterns | unique | file bytes | TRD 255-sector |
|---|---:|---:|---:|---|
| canonical: ps 64, skip 64 (test_chronos.sh) | 21 | 19 | 62421 | fits |
| ps 64, skip 0 (derived default ps, default skip) | 21 | 19 | 65222 | fits (58 B margin) |
| ps 13, skip 0 (divides) | 105 | 105 | 73387 | exceeds |

Checkpoints-1 numbers, recomputed from this receipt (planning rows,
SYNTHETIC empty ornaments/ARGV — labeled as such, not original-module
sizes): ps 8 → 45458 (fits), ps 64 → 65056 (fits), ps 5 → 72417,
ps 13 → 73238, ps 26 → 72397, ps 676 → 71595, ps 1351 → 71529, all skip-past-end
degenerates → 3990. The sweep over ps 5..1351 at skip 0 spans 45458..73238 B
(the earlier "65039..73238" range and "typical sizes exceed TRD" claim were
wrong: both ps 8 and the default ps 64 fit).

Corrected conclusion: dedup makes the CANONICAL Chronos VT2 (62421 B) and
even the default-skip variant (65222 B) fit the 255-sector TRD file; plain
rendering without patterns (67600 B) and many other original-semantics
pattern_size choices (ps 13/26/5/676/1351 → 69–73 KB) still exceed it.
CP/M (156800 B proven) holds every measured case. Naive adjacent-ASCII-line
RLE still GROWS the canonical text (69822 B, 1293 runs) — compression must
not be assumed; the later VTX packing stage is where compression belongs.
The TRD publication path still reports an explicit size failure after pass A
for over-limit modules; if a smaller TRD artifact is wanted it must come
from pattern_size/skip choices made upstream (still original semantics),
never from silent reduction.

## ARGV / Setup semantics (title trap, unchanged)

`ARGV[1]` is NOT a human title. In the original `Setup`, `ARGV[1]` is the
source mapping (`source_mapping = ARGV[1] || "1d-2me-3p,..."`), and Step11
passes `ARGV[1].to_s` straight into `Title=`; the canonical Chronos module's
title is literally `2me,1p,1m`. If ARGV[1] is absent the header title is
empty, regardless of the mapping default. `--format VALUE` is deleted from
ARGV before `ArgList`. `pattern_size` = ARGV[5] (0/absent → per_beat*64
halved while >127; 64 for the canonical per_beat 8); `skip_lines` = ARGV[6]
(default 0); `orn_repeat` = ARGV[7] (default 1, canonical 2); `max_offset` =
ARGV[8] (default 12, canonical 6); `out_file` derived from `in_file`.

## Metadata kind5 does and does not retain

Retained by the fifth-stage kind5 wire: three trimmed extents, cells,
definitions (≤64 / 4096 expanded bytes) and the original global
`render_rows` (u32). NOT retained: module title (ARGV[1] semantics above),
full ArgList, render/publish date, and the ornament TEXT→id map that Step10
consumes — kind5 definition fragments carry the ornament bytes, and the
canonical pass shows the VT2 ornament text is `squize_ornament`'s
comma-joined offset list built from the MIDI chords (each offset repeated
`orn_repeat` times), which is NOT reconstructible from the ids stored in
cells alone. The canonical pass derives it by rerunning actual Ruby Steps
1..5; the native chain needs a decision: carry the ornament text (or
text-equivalent offsets + orn_repeat) through a new wire section, or
regenerate it target-side from the interned definitions plus orn_repeat
(still to be proven against Ruby). Nothing may be silently discarded.

## Target-facing bounded plan (design, not executed)

All state below is fixed-size or file-backed; no unbounded Hash/arrays, no
string interning, no GC assumption. The host core above is the executable
blueprint of steps 3–6.

1. Row stream: the Step8 row kernel emits one ASCII row at a time; rows
   before `skip_lines` (after Ruby negative-start normalization) are
   consumed and discarded.
2. Pattern build: each pattern's rows stream into a patterns file; an
   incremental checksum digests them. `pattern_size`, `skip_lines`,
   `render_rows` are config constants from the input wire.
3. Pattern spill + index: one fixed record per pattern (u32 id, u32 length,
   u64 offset, candidate checksum) appended to an index file; u32 ids and
   order entries because the admitted profile reaches 65568 slots. Peak RAM:
   one row, one record, one flag byte, one compare chunk.
4. Collision-safe dedup: a checksum only CANDIDATES a match (linear index
   scan); equality is decided by byte-wise ≤CHUNK comparison of stored
   ranges — proven immune to forced collisions. Index/spool truncation
   faults explicitly.
5. Play order: u32 order entries plus use-flag bits, both file-backed
   records (no RAM arrays; the 8.2 KB u16 estimate of checkpoint 1 was
   wrong twice over — size and width).
6. Two-pass publication (no target Hash): pass A renders/splits/dedups and
   writes pattern blocks to a provisional file while collecting order/use
   records; pass B streams the final file — template literal from the
   image, play-order string built incrementally from the order records,
   then each used non-empty pattern copied through a chunked owned reader.
   No musical or formatting host callbacks: pattern boundaries, dedup,
   order, ornament text and all formatting are target code; the host only
   provides I/O.
7. Failure behaviour: input validation (extents, definitions, END/EOF,
   tails) as in the fifth stage. Size is known after pass A; a transport
   that cannot represent the final size must fail honestly BEFORE pass B
   writes anything. Canonical Chronos fits TRD; other original-semantics
   choices may not, and that verdict is per-measurement, not assumed.

## Remaining decisions for the parent

1. Wire extension: title / ArgList / date fields (or fixed values) for the
   sixth-stage input; date policy on target (no RTC: fixed field vs build
   constant).
2. Ornament source: new wire section carrying ornament text (or offsets +
   orn_repeat) vs target-side regeneration from interned definitions — the
   latter still needs its own actual-Ruby proof; the canonical pass's
   derivation is host-side only.
3. Target checksum width (32-bit recommended) and record layout (the host
   record layout above is a proven template).
4. pattern_size/skip_lines provenance: fixed image constants vs wire fields
   mirroring ARGV[5]/ARGV[6].
5. Whether the provisional two-pass publication writes to the transport
   directly (TRD rewrite-in-place) or to a scratch file — TRD free-space
   accounting differs; needs a decision before the target kernel lands.
