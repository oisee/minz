# WORKER-REPORT: first upstream cause of the 62-cell Chronos p-channel divergence

Worker: claude-z, worktree `/tmp/minz-autosiril-p-channel-z`, branch
`fix/autosiril-p-channel`. Fresh evidence in
`/tmp/minz-z-workers/p-channel-results/checkpoint-p1/`. Everything here is
read-only against the original Ruby tree (pinned `autooisee.rb`
SHA256 `41f621b555fc089effd711319e723494576eb47ea0282847082ec91c1cdd4c01`),
the CI artifacts and the planning fixtures.

## Verdict

**First bad stage: 3 (arrange, `ARRANGE.BIN` kind3). Divergent fields:
`note` + `ornament`.** Stage 2 is byte-perfect: all 2,097 flatten groups in
`EVENTS.BIN` equal the original Ruby `pmod` pitch sets (channel/row/pitch/type,
zero diffs, zero missing). The arrange stage received the correct dyad
`{51,58}` at channel 1 row 259 but emitted base note 51 with ornament
`[0,0,7,7]`, where Ruby's `squize_ornament` with the canonical settings clips
offset 0 (mediana 7, `|0-7| = 7 > max_offset 6`), rebases to note 58 with
ornament `"L0,0"`.

**Root cause: the native chain's Chronos fixtures and their Ruby references
were generated with `max_offset` 24 instead of the canonical invocation's 6.**
The canonical argv `chronos.mid 2me,1p,1m 8 6 12 0 64 2 6` ends in
`orn_repeat 2, max_offset 6`. The chain used `max_offset 24`, which enters at:

- `research/autosiril-vm/oracle.rb:7` — `max_offset=ARGV.fetch(4,'24')`, the
  default of the `ruby-full` reference that `arrange/build_matrix.py:35`
  asserts against (self-consistent at 24, so every arrange gate stays green);
- `research/autosiril-native/arrange/corpus.py` — `config()` default
  `offset=24`, used by the `chain_chronos_full` case;
- `research/autosiril-native/arrange/core.nanz:6` — `global max_offset:u8 = 24`
  baked into the standalone chain `ARRANGE.COM`.

This is a fixture/configuration divergence, **not** an arrange algorithm or
compiler bug (proof below). It also corrects the checkpoint-3 hypothesis: the
native path did see `{51,58}`, not `{51,52}`; ornament id 4 in the native
intern table maps to `[0,0,7,7]`, not to Ruby's id-4 text `"L0,0,1,1"`.

## Proof

1. `oracle_stages.rb` dumps every unchanged Ruby stage of the canonical
   invocation (pmod/lmod/dmod/mmod + settings) as JSON.
2. `trace_stage_divergence.py` diffs each native stage wire file against it,
   all fields, plus extra/missing cells:

   | boundary | native vs Ruby | diffs |
   |---|---|---:|
   | stage2 `EVENTS.BIN` (kind2 groups) vs `pmod` | 2,097 groups | **0** |
   | stage3 `ARRANGE.BIN` (kind3) vs `lmod` | 2,097 cells | **31** (5 `note`+`orn`, 26 `orn`-bytes-only) |
   | stage4 `DELAYED.BIN` (kind4) vs `dmod` | 3,215 cells | **62** (11 note-field, 51 ornament-only) |
   | stage5 planning `native_chronos` kind5 vs `mmod` | 3,215 cells | **62** |

   No extra or missing cells at any stage. The 5 note+ornament originals are
   rows 259, 262, 265, 395, 398 (channel 1); echo copies at ±6/±12 rows bring
   the note-field count to 11, and the 26 ornament-content cells
   (rows 420–508, 1325, 1326: one extra chord offset survives clipping at
   limit 24) plus their echoes bring the total to exactly the known 62 kind5
   cells / 62 rendered rows.

3. **Algorithm-vs-config:** running the actual arrange oracle (real Ruby
   `pmod2lmod`, `arrange/oracle.rb`) over the native chain's own
   `EVENTS.BIN` groups with `config.offset = 6` reproduces the canonical
   `lmod` **exactly**: 0 cell differences across all 2,097 cells and all 7
   ornament definitions equal (`oracle_offset6_check.json`). So with the
   canonical `max_offset` the stage-3 output is provably correct; only the
   configuration value carried by the chain is wrong.

## Reproduce

```sh
ruby research/autosiril-native/p-channel-fix/oracle_stages.rb \
  > /tmp/minz-z-workers/p-channel-results/checkpoint-p1/ruby_stages.json
python3 research/autosiril-native/p-channel-fix/trace_stage_divergence.py \
  --ruby /tmp/minz-z-workers/p-channel-results/checkpoint-p1/ruby_stages.json \
  --assert-equal
```

`--assert-equal` currently exits 1 with `155 differences remain`
(31 + 62 + 62); after the fixtures are regenerated at canonical
`max_offset 6` it must print `all stage boundaries equal canonical Ruby` and
exit 0. This is the regression that crosses the actual stage boundary (full
canonical chain vs full canonical Ruby, every field, every stage).

Input hashes (read-only inputs, SHA256 prefixes): `EVENTS.BIN`
`7a6a96d5a438bc27…`, `ARRANGE.BIN` `d6b41caa2f26b4ec…`, `DELAYED.BIN`
`943c45a8b5f5d98c…`, `NOTES.BIN` `4ecfa8a78d0146fe…` from
`/tmp/minz-ci270-artifacts/autosiril-mix-fresh/upstream/cpm/standalone/`
(verified identical to `measured/native_chronos/z80/`). Evidence:
`stage_diffs.json`, `row_sets.json`, `oracle_offset6_input.json`,
`oracle_offset6_out.json`, `oracle_offset6_check.json` in the checkpoint
directory.

## Changed files (this worktree only, new files only)

- `research/autosiril-native/p-channel-fix/oracle_stages.rb` — canonical
  Ruby stage dumper (SHA-guarded, read-only on the original tree).
- `research/autosiril-native/p-channel-fix/trace_stage_divergence.py` —
  stage-boundary tracer/regression.
- `research/autosiril-native/p-channel-fix/WORKER-REPORT.md` — this report.

Before/after diff counts: before 2,097/31/3,215-62/3,215-62 (per table);
after the prescribed fix: 0 at every boundary (proven for stage 3 via the
offset-6 oracle run; stages 4/5 are deterministic transforms of stage 3 and
their existing gates compare against regenerated references).

## Exact conflict with retained gates (report, not silently replaced)

Regenerating the Chronos chain at canonical `max_offset 6` changes shared,
already-merged gate material — **parent must integrate**; I did not touch it:

- `ARRANGE.BIN`, `DELAYED.BIN` (echo wire SHA256
  `2b115c050b50dfa0b3cececdc84dfb4144eaae6daa37c67600b2df9058093b25` in the
  echo README), the mix `native_chronos` kind5 fixture, and the
  arrange/echo/mix evidence receipts and goldens must be regenerated from the
  pinned actual reference at offset 6. The 54912 B CP/M size stays (same cell
  counts; only pitch/ornament bytes change).
- **Unaffected:** the 40,626 B linked flatten image and `NOTES.BIN`/
  `EVENTS.BIN` (stages 1–2 never consume `max_offset`; stage 2 verified
  byte-identical here), per-delay/per-beat/sources settings (already
  canonical), and the arrange kernel itself.
- Suggested minimal fix points for the parent: generate `ruby-full` with
  `research/autosiril-vm/oracle.rb` argv `… 6` for the Chronos reference,
  set the `chain_chronos_full`/standalone chain arrange config
  `max_offset = 6` (corpus chain case + `core.nanz` standalone default),
  leaving the synthetic 24-offset boundary cases (`upper_median_clip_zero`,
  `clipping_exact_and_over`, …) and their fixtures untouched, then regenerate
  downstream goldens from the pinned reference. Alternatively keep 24 but then
  the chain must not be presented as canonical-invocation parity — which the
  task rules out.

## Remaining scope

- Actual fixture/golden regeneration + PR/CI: parent.
- Honest native MIDI→VT2 stays blocked until the chain is regenerated at
  canonical settings; after that, re-run the tracer with `--assert-equal`.
- Native CP/M/TRD/VT2 tail (render_into_text onward) remains in progress;
  nothing here claims the whole converter. STOP marker untouched; no push,
  no PR, no messages.
