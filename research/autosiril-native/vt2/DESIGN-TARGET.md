# VT2 Step9/10/11: proven semantics and target-facing design (checkpoint 1)

Scope of this checkpoint: a reproducible actual-Ruby vs independent bounded
host model proof for the remaining original Step9 `split_into_patterns`
(pattern split, deduplication, play order), Step10 `render_ornaments` and
Step11 `module_template`/`write_vt_output`, plus a concrete target-friendly
plan. This is HOST planning evidence only. There is no native sixth-stage
execution, no target result and no finished MIDI→VT2 converter here.

Original reference: `/Users/alice/dev/autosiril` (read-only),
`autooisee.rb` SHA256
`41f621b555fc089effd711319e723494576eb47ea0282847082ec91c1cdd4c01`,
`module_template.rb` SHA256
`8037f85a1c92cc32e494ba6b62a70be99fae5de041050addbf05844b7e254ad6`.
Oracle: `oracle_vt2.rb` evals the unchanged original functions (guarded by
exact source hash and line positions, read-only) and calls them directly.
Model: `model_vt2.py` computes everything independently with bounded,
file-backed state; only the static template literal text is taken from the
original `module_template.rb` source. Prover: `prove_vt2.py`.

## Proven semantics (all bytes and statistics equal, 554 cases)

- Pattern count: divisible `max_row % pattern_size == 0` gives
  `max_row/pattern_size + 1`; nondivisible gives floor division. This is NOT
  mathematical ceil. `max_row == 0` gives exactly one pattern. A ceil mutant
  disagrees on 462 cases (negative control).
- `patterns == 0` (possible when `0 < max_row < pattern_size`): the loop body
  never runs and `wet_play_order_txt` is `"L"`, then `"L"[-1] = ''` makes it
  the EMPTY string. The header then reads `PlayOrder=` with nothing after it.
- Slicing uses Ruby `Array#slice(start, length)` edges with
  `start = skip_lines + i*pattern_size`: `start == lines.size` returns `[]`
  (empty pattern), `start > size` returns `nil`. Both produce the same `""`
  pattern text, so nil-slice patterns dedup against empty-slice patterns.
  Negative `skip_lines` normalizes per-pattern (`size + start`), out-of-range
  negative start yields nil; later patterns can recover into range.
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

## ARGV / Setup semantics discovered (title trap)

`ARGV[1]` is NOT a human title. In the original `Setup`, `ARGV[1]` is the
source mapping (`source_mapping = ARGV[1] || "1d-2me-3p,4m[2]+,..."`), and
Step11 passes `ARGV[1].to_s` straight into `Title=`. If ARGV[1] is absent the
header title is empty, regardless of the mapping default. `--format VALUE`
is deleted from ARGV before anything else, so `ArgList` never shows it.
`pattern_size` defaults to `per_beat*64` halved while > 127 (64 for the
default per_beat 4; `ARGV[5]`, 0 falls back to the derived default);
`skip_lines` defaults to 0 (`ARGV[6]`); `out_file` is derived from `in_file`.

## Metadata kind5 does and does not retain

Retained by the fifth-stage kind5 wire: three trimmed extents, cells,
definitions (≤64 / 4096 expanded bytes) and the original global
`render_rows` (u32). NOT retained: module title (ARGV[1] semantics above),
full ArgList, render/publish date, and (to be confirmed by the render-stage
owner) the ornament text→id map that Step10 consumes — in the original
pipeline it comes from `pmod2lmod`, not from the rendered rows. Open
decision for the parent: extend the sixth-stage input wire with title,
argv-list and a pinned date field, or fix their values in the target image;
nothing may be silently discarded.

## Target-facing bounded plan (design, not executed)

All state below is fixed-size or file-backed; no unbounded Hash/arrays, no
string interning, no GC assumption.

1. Row stream → pattern accumulator. The Step8 row kernel emits one ASCII
   row at a time. A bounded accumulator holds the current pattern's bytes
   only: at most `pattern_size × 50` bytes (49-char rows + LF; nil-character
   PARAMS rows can be shorter). `pattern_size`, `skip_lines`, `render_rows`
   are config constants from the input wire, not data-dependent.
2. Skip handling. Rows before `skip_lines` (after Ruby negative-start
   normalization) are consumed and discarded; the accumulator counts
   pattern-local rows exactly like the slice edges above.
3. Pattern spill + index. Each completed pattern is appended to a patterns
   file; a fixed record (u16 index, u32 offset, u32 length, candidate
   checksum) is appended to an index file. Peak RAM: one row, one record,
   one compare chunk.
4. Collision-safe dedup. A new pattern's checksum only CANDIDATES a match
   against earlier index records (linear file scan, O(P) records,
   P ≤ 65536/pattern_size + 1). Equality is decided by byte-wise chunked
   comparison of the stored range against the in-RAM pattern bytes. A hash
   collision is never musical equality. The target checksum can therefore be
   a cheap 32-bit sum/CRC — bytes decide.
5. Play order. A u16 order entry per pattern (worst case ≈ 8.2 KB at
   pattern_size 16, u16 DATA rows) plus the use flags (bitmask) are the only
   arrays kept in RAM; both are bounded by the admitted profile.
6. Two-pass publication (no target Hash needed for the header). Pass A:
   render/split/dedup, writing pattern blocks to a provisional file and
   collecting order/use bits. Pass B: write the final file — stream the
   header (template literal from the image, then the play-order string built
   incrementally from the order list), then copy each used non-empty pattern
   from the provisional file through a chunked owned reader. No musical or
   formatting host callbacks: pattern boundaries, dedup, order, ornament
   text and all formatting are target code; the host only provides I/O.
7. Failure behaviour. Input validation (extents, definitions, END/EOF,
   tails) stays as in the fifth stage. Size is known after pass A; a
   transport that cannot represent the final size must fail honestly BEFORE
   pass B writes anything (see measurements).

## Transport measurements (Chronos, 1352 rows × 49 chars)

Canonical plain render is 67600 B; the complete VT2 module adds the fixed
3935-byte header/ornaments/samples literal plus pattern headers. Measured
final VT2 sizes from the proof receipt (actual Ruby writer, pinned date):

| case | patterns | unique | file bytes | TRD 255-sector (65280 B) |
|---|---:|---:|---:|---|
| ps 8 (divides) | 170 | 100 | 45458 | fits |
| ps 13 (divides) | 105 | 105 | 73238 | exceeds |
| ps 26 (divides) | 53 | 53 | 72397 | exceeds |
| ps 64 (nondiv) | 21 | 19 | 65056 | fits (barely) |
| ps 5 (nondiv) | 270 | 256 | 72417 | exceeds |
| ps 676 | 3 | 3 | 71595 | exceeds |
| ps 1351 | 1 | 1 | 71529 | exceeds |
| skip 1352 (all patterns empty) | 21 | 1 | 3990 | fits |

Findings: dedup only rescues TRD when the song is genuinely repetitive at
the chosen pattern size (ps 8: 170→100 unique; skip-past-end degenerates).
For typical pattern sizes the honest Chronos VT2 is 69–73 KB and exceeds the
255-sector TRD file limit; CP/M (156800 B proven) holds every measured case.
Naive adjacent-ASCII-line RLE (u32 count + u8 length + payload) GROWS the
canonical text to 69822 B (1293 runs) — compression must not be assumed.
VT2 has no multi-file spanning, and shrinking the admitted input profile or
truncating the song is not acceptable. Therefore: the TRD publication path
reports an explicit size failure after pass A for over-limit modules; CP/M
is the transport that carries full Chronos. A "reversible display
representation" (packed display rows) would change the format and is
rejected for VT2 output; the later VTX packing stage is where compression
belongs, as a separate bounded step. If a smaller TRD artifact is wanted, it
must come from pattern_size/skip choices made upstream (still original
semantics), never from silent reduction.

CP/M publication: pad the final sector per the existing fifth-stage writer
semantics and record exact logical length; TRD length is exact bytes with
ceil(bytes/256) sectors. Catalog/writer-snapshot and reopen rules are
inherited unchanged from the fifth stage; every handle closes on every exit.

## Remaining decisions for the parent

1. Wire extension: title / ArgList / date fields (or fixed values) for the
   sixth-stage input; date policy on target (no RTC: fixed field vs build
   constant).
2. Ornament source: confirm how the ornament text→id map reaches the sixth
   stage in the native chain (kind5 definitions vs a new section).
3. Target checksum width (32-bit recommended) and record layout.
4. pattern_size/skip_lines provenance: fixed image constants vs wire fields
   mirroring ARGV[5]/ARGV[6].
5. Whether the provisional two-pass publication writes to the transport
   directly (TRD rewrite-in-place) or to a scratch file — TRD free-space
   accounting differs; needs a decision before the target kernel lands.
