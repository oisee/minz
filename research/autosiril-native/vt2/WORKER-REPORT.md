# WORKER-REPORT — claude-z, VT2 Step9/10/11 checkpoint 3 (2026-10-27)

Owned directory: `research/autosiril-native/vt2/` only. No renderer,
compiler, codegen, workflow, book, fixture or global-config files touched.
No push/PR/release/agents/Herdr/SSH. Original `/Users/alice/dev/autosiril`
used read-only; both hashes verified every run
(autooisee `41f621b5…1cdd4c01`, module_template `8037f85a…7e254ad6`).
Sibling trees `/tmp/minz-autosiril-native-render`, `/tmp/minz-ci270-artifacts`
and `/tmp/minz-render-planning-v4` were READ only, never modified.

The full eventual goal is unchanged: a native file renderer plus VT2,
inheriting every earlier gate, with fresh/frozen CI/audits/PR/main. This
checkpoint is another HOST-proof-only step toward it; no renderer edits, no
native execution, no target results, no finished MIDI→VT2 converter, and no
native VT2 claim.

## Parent findings addressed (checkpoint 2 → 3)

A. The checkpoint-2 output path was still unbounded in its header stage:
   `run_case` called `order_text_from_store` (unbounded parts list + join)
   BEFORE writing, `module_header` interpolated the whole play order into a
   whole header string, and `write_vt_output` encoded that whole header.
   Now the output is genuinely STREAMED end to end
   (`model_vt2.py`): the template literal is parsed once into bounded
   literal/interpolation segments (`template_segments`, fixed source
   constant, no case data); `write_vt_output` writes literal fragments
   directly, streams `#{play_order}` one u32 order entry at a time from the
   order file (Ruby `"L"` framing, empty for zero patterns), streams
   `#{ornaments_txt}` one bounded ornament fragment at a time
   (`ornament_fragments`: ≤64 defs × ≤4096 B; the joined text is never
   assembled on the output path), interpolates bounded title/date/ArgList
   strings, and then copies each used non-empty pattern from the spool in
   ≤CHUNK pieces. `module_header` (whole-string regex) is removed.
   `run_case` writes the output FIRST and only afterwards builds the proof
   statistics (`order_text_from_store`, use flags, wet sizes,
   `render_ornaments` — now explicitly proof-only, composed from the same
   fragment generator so the two cannot drift). Output no longer depends on
   any collected proof statistic. All byte-exact checks are kept and pass.
B. The 62-row native-vs-original Chronos divergence is no longer an open
   upstream question. New `chronos_divergence_repro.rb` +
   `CHRONOS-DIVERGENCE.md` (+ `evidence/checkpoint3-chronos-divergence.json`)
   pin it reproducibly: settings identical (max_row 1352, ps 64, skip 64,
   per_beat 8, per_delay 6, per_delay2 12, orn_repeat 2, max_offset 6,
   key 0, cpr 15.0, chan [me,p,m], sources [1,0,0]); FIRST difference at
   rendered row 259, `noteB` block (channel index 1), columns 22/28:
   `A#3 2F.F` → `D#3 2F4F` (note A#3→D#3, ornament .→4). The 62 differing
   rows are exactly 62 differing cells of the native kind5 fixture
   (`cases.json` entry `native_chronos`, the mix-stage output the render
   stage consumed) vs actual Ruby `mmod` — all channel 1, all
   kind5 {note 51, ornament 4} vs Ruby {note 58, ornament 0}. Mechanism:
   Ruby's `squize_ornament` on the pmod dyad {58,51} (offsets [0,7]) uses
   mediana 7 and the max_offset=6 filter to delete offset 0 and rebase to
   note 58 with ornament `L0,0` (id 0); the native kind5 ornament id 4 is
   `L0,0,1,1`, i.e. the native `p`-channel path collapsed a {51,52} dyad —
   the second chord member itself was wrong (52 vs 58) at or before the
   pmod→lmod collapse, after the shared flatten (native channel 2 renders
   the same source row correctly), and the error propagates into the
   echo-decay copies (rows 271–277, 395–435…). Verdict: a real upstream
   native-chain content divergence, NOT an invocation/fixture-profile
   difference and NOT a native-render-stage bug (that stage was proved
   equal to Ruby render on the same kind5 input, and that proof remains
   valid). The native kind5 == Ruby mmod end-to-end equality was never
   proved (the echo stage audit compared only
   per_beat/sources/notes/max_row/flattened). Parent coordinates the fix;
   nothing was altered to hide the mismatch (original Ruby, goldens and
   target code unchanged).

## Commands

```sh
python3 -I research/autosiril-native/vt2/prove_vt2.py \
  --autosiril /Users/alice/dev/autosiril \
  --planning /tmp/minz-render-planning-v4 \
  --out /tmp/minz-z-workers/vt2-results/checkpoint3-20261027-a

ruby research/autosiril-native/vt2/chronos_divergence_repro.rb \
  --autosiril /Users/alice/dev/autosiril \
  --planning /tmp/minz-render-planning-v4
```

## Results

- OK: 558 corpus cases (+4 upper-bound), 548 forced-collision reruns,
  3 canonical variants; 267511 pattern slots (same totals as checkpoint 2 —
  the streaming refactor changed no output byte); every check equal
  (file SHA256, ornaments SHA256, play-order string, use flags, wet sizes,
  pattern counts, file bytes).
- Suites: 126 synthetic, 4 upper-bound, 406 planning-real (partial-real),
  22 chronos variants (partial-real).
- Divergence reproducer output: 62 differing rows, first row 259 block
  noteB cols [22, 28]; kind5-vs-mmod 62 cells all channel 1; squize replay
  {58,51}→base 58 ornament `L0,0` matching Ruby.
- Output: `/tmp/minz-z-workers/vt2-results/checkpoint3-20261027-a/`
  (`cases.json`, `oracle-vt2/*.vt2`, per-case `spill*/` with
  patterns/index/order/use files and streamed `model.vt2`, `truncation/`,
  `receipt.json` with the streaming-output-path record). Portable evidence
  committed: `evidence/checkpoint3-receipt.json` (SHA256
  `d686541a…d96d2c8`) and `evidence/checkpoint3-chronos-divergence.json`
  (SHA256 `fce2679d…e71a09`).
- Ruby 2.6.10p210; header date pinned 1970.01.01 (recorded in receipt);
  canonical reference equals pinned output except the single Author date
  line, as in checkpoint 2.

## Limitations (truthful scope)

- HOST proof only: actual original Ruby vs a bounded host model. No native
  sixth-stage execution, no target memory/T-state measurements, no CI runs,
  and no claim of a native VT2 implementation.
- The bounded core's peak-memory claim is structural (by construction of
  the code paths), not instrumented heap measurement. The proof statistics
  are still unbounded host-side collections, but they are built strictly
  AFTER the output file is finished and are not on the output path.
- The divergence investigation localizes the first difference and its
  mechanism to the native chain's kind5 fixture (`p`-channel dyad
  {51,52} vs {51,58} at row 259 ff.); WHICH native stage instruction
  produces 52 instead of 58 was not traced (requires reading/running the
  sibling native stage sources, owned by other workers); left to the
  parent's coordination.
- Ornament derivation for the canonical pass still reruns actual Ruby
  host-side; the native chain wire decision for ornament text (or offsets +
  orn_repeat) remains open — see DESIGN-TARGET.md §Remaining decisions.

## Commit

Local commit on `spike/autosiril-vt2-z` (see `git log -1`; nothing pushed).
