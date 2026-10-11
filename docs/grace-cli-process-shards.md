# Full CLI tests in three independent processes

Full `cmd/minzc` tests passed: **71/71 PASS, 0 SKIP**, **279.75s** including
**4.52s** building the test binary. Each top-level test appeared exactly once
in the discovered list, shard assignment, run event and terminal event.
Source snapshot: `0cbfeff7`; runner addition is recorded as an untracked file
in the exact run plan with its own SHA. No CLI globals or `t.Parallel` changed.

[Runner](../scripts/run_cli_process_shards.py) builds the package test binary
once, discovers its `Test*` entries and creates exact anchored, disjoint regexes:
SearchCLI; PolicyCLI+MemoryHost; remaining tests. Three processes each start
with fresh globals and a unique temp directory. Tests remain sequential inside
each process. `test2json` records all top-level/subtest results and elapsed times;
summary admission rejects missing/duplicate tests, failed processes, malformed
JSON or missing terminal events. New discovered tests automatically join a shard.

| Shard | Tests | Wall seconds |
|---|---:|---:|
| SearchCLI | 7 | 274.59 |
| PolicyCLI + MemoryHost | 11 | 90.77 |
| Rest | 53 | 43.62 |

The sum of top-level test durations from this same run is **399.51s**;
399.51/279.75 ≈ **1.43**. That sum estimates current serial work, not an actual
serial rerun: concurrent load and instrumentation affect durations. The retained
historical serial CLI result was **425.234s** at snapshot `5fe165`, so this run
finished about34% sooner than that recorded time, but source and machine load
differ. This is not a controlled same-snapshot A/B claim; no costly extra serial
repeat was run.

The bottleneck is one individual `TestSearchCLICollectAndPairedReplay`:
**270.36s**, already96.6% of total wall including build. Test-group parallelism
cannot make that test shorter. Other major tests: policy collect/replay43.81s,
legacy selected output43.11s, auto measured compilation13.55s. A possible next
step is independently compiling search candidates in isolated worker processes,
with exact input/result binding; shared-state `t.Parallel` is not a solution.
That candidate-level change is not implemented or measured here.

The runner clears inherited `MINZ_CPM_OVERFLOW_ARTIFACTS`,
`MINZ_MIXED_COPY_REGION_CLI_ARTIFACTS`, `MINZ_CALL_SPILL_CLI_ARTIFACTS` to avoid
fixed-output collisions; it also clears `GOFLAGS`, recording only affected names,
never sensitive values. Its output directory must be empty, so previous evidence
cannot be overwritten. The default per-process Go timeout is20m, three workers
are bounded; source commit, working-tree status, Go version, binary/runner SHA,
build/shard/full wall time and exit codes are recorded.

Repeat from the repository root:

```sh
GOCACHE=/tmp/minz-grace-gocache python3 scripts/run_cli_process_shards.py \
  --go /home/alice/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.8.linux-amd64/bin/go \
  --output /absolute/new-empty-evidence-directory
```

[Manifest](grace-cli-process-shards/manifest.json) preserves plan, summary,
full per-process logs, gzip JSON event streams, discovery/build/progress and
historical result. The local execution checkpoint also retains the test binary
and its recorded SHA; generated binary is not committed as repository source.
Simple independent partition/duplicate-refusal checks passed. The full actual
three-process run is the primary validation of discovery, isolation and coverage.
