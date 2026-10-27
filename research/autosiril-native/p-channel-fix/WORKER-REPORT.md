# WORKER-REPORT: canonical profile (max_offset 6) executed through the native chain

Worker: claude-z, worktree `/tmp/minz-autosiril-p-channel-z`, branch
`fix/autosiril-p-channel`. Checkpoint-p3 evidence in
`/tmp/minz-z-workers/p-channel-results/checkpoint-p3/` (`fresh/` with Ruby,
`frozen/` without Ruby, plus `fresh.log`/`frozen.log`). Everything here is
read-only against the original Ruby tree (pinned `autooisee.rb` SHA256
`41f621b555fc089effd711319e723494576eb47ea0282847082ec91c1cdd4c01`), the CI
artifacts and the planning fixtures.

## Verdict (checkpoint-p1, unchanged)

First bad stage: **3 (arrange)**; divergent fields `note`+`ornament`; root
cause **fixture configuration** — the chain and its references were generated
with `max_offset` 24 instead of the canonical argv's 6 (`chronos.mid 2me,1p,1m
8 6 12 0 64 2 6`). Not an arrange algorithm, echo, mix, or compiler bug.

## What checkpoint-p3 adds: the complete FILE fifth stage, frozen replay and CI

Checkpoint-p2 proved stages 3/4 by CP/M file execution but stage 5 only through
the mix **kernel** fixture trap. Checkpoint-p3 completes the FILE chain and makes
the proof portable:

| boundary | native execution | vs actual Ruby | diffs |
|---|---|---|---:|
| stage3 `ARRANGE.BIN`@6 | recompiled `ARRANGE-CANONICAL6.COM` through mze | `lmod` | **0** (2,097 cells) |
| stage4 `DELAYED.BIN`@6 | unchanged upstream `ECHO.COM` through mze | `dmod` | **0** (3,215 cells) |
| stage5 `MIXED.BIN`@6 **file** | **actual unchanged upstream `MIX.COM`** (sha256 `8ad63e83…`, from the fresh `cpm/standalone` chain) through mze on `DELAYED.BIN`@6 + `NOTES.BIN` | `mmod` | **0** (3,215 cells, 8 definition payloads identical to `DELAYED.BIN`@6, lengths `[1353,1353,1353]`, render_rows 1352; logical 54,880 B / physical 54,912 B, sha256 `8aa9cfa2…`) |
| stage5 kernel (additional layer) | mix kernel raw-mir2/final-mir2/z80 | `mmod` | **0** (3,215 cells) |

Scope is recorded per stage in the receipt (`stage_scope`): stages 1–2 reuse the
byte-identical verified upstream `NOTES.BIN`/`EVENTS.BIN` (PAIR.COM/FLATTEN.COM
not re-executed — counts `[1,1,1]`/policies `---` match the existing default),
stage 3 file, stage 4 file, stage 5 **file** plus kernel as an additional layer.
The renderer input for the future Renderer6 worker is recorded in
`receipt.json` → `renderer_input` (canonical6 `MIXED.BIN`, `DELAYED.BIN`,
`NOTES.BIN` + lengths/render_rows).

- **Frozen replay without Ruby**: `fixtures/canonical6-frozen.json.gz` (32 KB,
  generated from this actual-Ruby fresh run) pins config, settings, source and
  upstream-binary hashes (EVENTS/NOTES/ECHO.COM/MIX.COM/ARRANGE-CANONICAL6.COM),
  the generated arrange and kernel sources, all three expected wire files and
  every boundary. `replay_canonical6.py` regenerates the arrange source,
  recompiles, executes ARRANGE/ECHO/MIX through mze and the mix kernel through
  all three engines, and checks every boundary — Ruby never runs. The fresh
  proof (`--fixture`) verifies the frozen expectations against a live Ruby run,
  so the fixture can never decay into native self-agreement.
- **CI**: `native_echo_fresh` now runs the canonical6 fresh proof (with fixture
  verification) and uploads `autosiril-canonical6-fresh`; `native_mix_frozen`
  builds mze and runs the canonical6 frozen replay off the downloaded
  `autosiril-echo` artifact and the replay-produced `MIX.COM`, uploading
  `autosiril-canonical6-frozen`. All inherited jobs, gates, artifacts and
  required fail conditions are unchanged; every command is portable Linux/macOS.
- **TRD canonical6**: explicitly **pending**, not blocking the CP/M proof — the
  TRD chain (TRD flatten/arrange/echo/mix adapters) is a separate body of work
  and was not attempted in this checkpoint.

## What checkpoint-p2 added: execution instead of prediction

Checkpoint-p1 stated "after the fixtures are regenerated at 6 the boundaries
must be equal" as an **expectation** (only stage 3 was proven via the Ruby
oracle). This checkpoint **executes** stages 3/4/5 natively at the canonical
profile and compares every boundary to the actual full Ruby pipeline:

| boundary | native execution | vs actual Ruby | diffs |
|---|---|---|---:|
| stage2 `EVENTS.BIN` | unchanged upstream | `pmod` | **0** (2,097 groups) |
| stage3 `ARRANGE.BIN`@6 | probe raw-mir2+final-mir2+z80 **and** standalone `ARRANGE.COM` via mze | `lmod` | **0** (2,097 cells, all fields, 7/7 ornament defs) |
| stage4 `DELAYED.BIN`@6 | unchanged upstream `ECHO.COM` via mze on the canonical `ARRANGE.BIN` | `dmod` | **0** (3,215 cells) |
| stage5 mix cells@6 | native mix kernel raw-mir2+final-mir2+z80 | `mmod` | **0** (3,215 cells) |

Zero missing/extra cells at every boundary; final renderer input rows are
identical to Ruby `mmod` rows. The stage-3 standalone output is byte-equal to
the wire encoding of actual Ruby `pmod2lmod` at offset 6 (IDs included), and
actual Ruby `downmix` over the native `DELAYED.BIN`@6 reproduces canonical
`mmod` exactly before the native kernel is checked against it.

## Explicit profiles; nothing committed is relabeled

- **canonical6** (new, explicitly named): generated source variant
  `arrange-cpm-canonical6.nanz` — the `compose.py` output with the single
  baked default `global max_offset:u8 = 24 → 6` — executed by the existing
  arrange probe (config-injection mechanism, `max_offset` as data at 6), as
  standalone `ARRANGE-CANONICAL6.COM` through mze, and downstream through the
  unchanged `ECHO.COM` and the existing mix kernel.
- **legacy24** (preserved): the committed chain, its goldens, receipts and
  the 40,626 B flatten path are untouched; `trace_stage_divergence.py` still
  reproduces its self-consistent 31/62/62 diff signature against canonical
  Ruby (`stage_diffs_legacy24.json` in the checkpoint directory).

No compiler default, gate, golden, workflow or shared fixture was modified.

## Reproduce (all paths are flags; no hardcoded temporary paths)

Fresh (Ruby required; verifies the committed frozen fixture against actual Ruby):

```sh
python3 research/autosiril-native/p-channel-fix/prove_canonical6.py \
  --autosiril /path/to/autosiril \
  --upstream /path/to/autosiril-mix-fresh/upstream/cpm/standalone \
  --mix-com /path/to/autosiril-mix-fresh/cpm/standalone/MIX.COM \
  --mze /path/to/mze \
  --fixture research/autosiril-native/p-channel-fix/fixtures/canonical6-frozen.json.gz \
  --out /tmp/p3-fresh
```

Frozen (no Ruby; regenerates + reexecutes the actual stages against the frozen
actual-Ruby expectations — the exact command shape the CI frozen job runs):

```sh
python3 research/autosiril-native/p-channel-fix/replay_canonical6.py \
  --upstream /path/to/autosiril-echo/cpm/standalone \
  --mix-com /path/to/autosiril-mix-frozen/cpm/standalone/MIX.COM \
  --mze /path/to/mze --out /tmp/p3-frozen
```

Both were executed end-to-end in `checkpoint-p3/` (`fresh/` and `frozen/`,
receipts `receipt.json`). The stage tracer now also accepts `--kind5-file`
(an actual `MIXED.BIN`) in addition to `--kind5-cells` (kernel output).

## Changed files (this worktree only)

- `p-channel-fix/prove_canonical6.py` — stage5 FILE execution of the actual
  unchanged `MIX.COM` through mze with full kind5 comparison (cells, definition
  payloads, role metadata), dual file/kernel boundary trace, frozen-fixture
  verification/refresh, per-stage scope and renderer-input receipt.
- `p-channel-fix/replay_canonical6.py` (new) — frozen no-Ruby replay.
- `p-channel-fix/fixtures/canonical6-frozen.json.gz` (new) — frozen fixture
  generated from the pinned actual Ruby run.
- `p-channel-fix/trace_stage_divergence.py` — `--kind5-file` MIXED.BIN parser.
- `p-channel-fix/WORKER-REPORT.md` — this report.
- `.github/workflows/ci.yml` — canonical6 fresh step + upload in
  `native_echo_fresh`, canonical6 frozen replay + upload in `native_mix_frozen`
  (explicitly authorized for this scoped proof integration; no inherited gate,
  artifact or fail condition changed).

## Hashes (checkpoint-p2-script receipt)

`ARRANGE-CANONICAL6.COM` 13646 B, ARRANGE.BIN@6 (physical 35840 B / logical
35806 B) sha256 `60bfcc62…`, DELAYED.BIN@6 54912 B sha256 `18e08c86…`, mix
cells 51440 B. Stage1/2 binaries (`NOTES.BIN`/`EVENTS.BIN`) are unaffected by
`max_offset` and byte-identical to upstream (verified in the trace).

## Remaining scope

- Parent integration choice: promote canonical6 (regenerate shared goldens
  from the pinned reference at offset 6) or keep both profiles as explicit
  fixtures; the retained-gate conflict list from checkpoint-p1 still applies —
  no shared gate was touched here.
- TRD canonical6 profile execution: pending (see above), not blocking the
  CP/M proof.
- Native render/VT2 tail remains in progress; nothing here claims the whole
  converter or VT2/VTX packing. The CP/M FILE chain through MIX is complete
  at both profiles. STOP marker untouched; no push, no PR, no messages.
