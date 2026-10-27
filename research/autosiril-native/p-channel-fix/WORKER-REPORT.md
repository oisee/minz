# WORKER-REPORT: canonical profile (max_offset 6) executed through the native chain

Worker: claude-z, worktree `/tmp/minz-autosiril-p-channel-z`, branch
`fix/autosiril-p-channel`. Fresh evidence in
`/tmp/minz-z-workers/p-channel-results/checkpoint-p2/` (interactive run) and
`.../checkpoint-p2-script/` (single-script replay). Everything here is
read-only against the original Ruby tree (pinned `autooisee.rb` SHA256
`41f621b555fc089effd711319e723494576eb47ea0282847082ec91c1cdd4c01`), the CI
artifacts and the planning fixtures.

## Verdict (checkpoint-p1, unchanged)

First bad stage: **3 (arrange)**; divergent fields `note`+`ornament`; root
cause **fixture configuration** — the chain and its references were generated
with `max_offset` 24 instead of the canonical argv's 6 (`chronos.mid 2me,1p,1m
8 6 12 0 64 2 6`). Not an arrange algorithm, echo, mix, or compiler bug.

## What checkpoint-p2 adds: execution instead of prediction

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

```sh
python3 research/autosiril-native/p-channel-fix/prove_canonical6.py \
  --autosiril /path/to/autosiril \
  --upstream /path/to/autosiril-mix-fresh/upstream/cpm/standalone \
  --mze /path/to/mze --out /tmp/p2
```

Single-script replay verified end-to-end in `checkpoint-p2-script/` (receipt:
`receipt.json`). Stage tracer, now portable (every path a flag or
`AUTOSIRIL_*` env var, `--kind5-cells` for native kernel output):

```sh
ruby research/autosiril-native/p-channel-fix/oracle_stages.rb \
  --autosiril /path/to/autosiril > ruby_stages.json
python3 research/autosiril-native/p-channel-fix/trace_stage_divergence.py \
  --ruby ruby_stages.json --events … --arrange … --delayed … \
  --kind5-cells … --out stage_diffs.json --assert-equal
```

## Changed files (this worktree only)

- `p-channel-fix/prove_canonical6.py` (new) — full canonical6 execution proof.
- `p-channel-fix/trace_stage_divergence.py` — portable paths, `--kind5-cells`
  parser for native mix kernel output, physical-padding strip.
- `p-channel-fix/ERRATUM-CHRONOS-DIVERGENCE.md` (new) — corrected text for the
  parent; the old VT2 `CHRONOS-DIVERGENCE.md` "identical config, not fixture"
  framing was the reverse of the proven cause.
- `p-channel-fix/WORKER-REPORT.md` — this report; supersedes the p1
  expectation-only "after fix 0 all stages" statement.
- `p-channel-fix/.gitignore` (new) + removal of the accidentally committed
  `__pycache__/trace_stage_divergence.cpython-314.pyc`.

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
- Native CP/M/TRD/VT2 tail (render onward) remains in progress; nothing here
  claims the whole converter or VT2/VTX packing. STOP marker untouched; no
  push, no PR, no messages.
