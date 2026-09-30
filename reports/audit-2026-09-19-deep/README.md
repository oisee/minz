# Deep-audit evidence — 2026-09-19

[Report](../2026-09-19-Deep-Technical-Audit-RU.md). These are audit probes and recorded observations, not fixes or a replacement for the project's regression suite. They print observations; successful probe execution can intentionally demonstrate a defect. Review the output, not only the probe's exit code.

`provenance.json` identifies local revisions and hashes. Neighbor repositories were not fetched for this audit. GPU scheduling, race traces and parallel feed order are nondeterministic; mismatch counts/hashes can differ on repeat. The alias counts refer only to the exact hashed 4v file.

Use the following commands from each named working directory. Outputs go to stdout or `/tmp`; the committed observations need not be overwritten. The example paths assume sibling repositories under `/home/alice/dev`; adjust them to your checkout layout.

## MinZ readers, index and production checks

Working directory: `minz/minzc`.

```sh
go run ../reports/audit-2026-09-19-deep/minz-probe/main.go
go test -json ./pkg/mir2 -run 'Contract|Parallel|Div|Mod|SMC' -count=1 -timeout 60s
go build -o /tmp/minz-deep-mz ./cmd/minzc
go build -o /tmp/minz-deep-mza ./cmd/mza
```

Working directory: `minz`.

```sh
python3 reports/audit-2026-09-19-deep/check_imports.py /tmp/minz-deep-mz
/tmp/minz-deep-mz fun/che_cascade.nanz -o /tmp/minz-deep-che.a80
/tmp/minz-deep-mza /tmp/minz-deep-che.a80 -o /tmp/minz-deep-che.bin
python3 reports/audit-2026-09-19-deep/verify_4v.py ../z80-optimizer/data/enriched_4v.enr
python3 reports/audit-2026-09-19-deep/check_dense.py ../z80-optimizer/data/ix_expanded_6v_dense.bin
```

The independent Python 4v checker validates count, EOF, domains, assignment length and physical overlap. It is not an instruction execution verifier. `check_dense.py` reads at most 100,000 records and stops after three feasible ones; it does not allocate the complete table.

## Upstream generator, reader and GPU probes

Working directory: `z80-optimizer`.

```sh
GOTOOLCHAIN=go1.25.0 go run ../minz/reports/audit-2026-09-19-deep/upstream-probe/main.go
GOTOOLCHAIN=go1.25.0 go build -o /tmp/minz-deep-feed ./cmd/gen6v-ix-feed
GOTOOLCHAIN=go1.25.0 go build -o /tmp/minz-deep-build-table ./cmd/build-ix-table
nvcc -O2 -o /tmp/minz-deep-regalloc cuda/z80_regalloc.cu
python3 ../minz/reports/audit-2026-09-19-deep/check_feed.py /tmp/minz-deep-feed
/tmp/minz-deep-regalloc --server < ../minz/reports/audit-2026-09-19-deep/gpu-alias-input.jsonl
python3 ../minz/reports/audit-2026-09-19-deep/check_gpu_cost.py /tmp/minz-deep-regalloc
printf '%s\n' '{"cost":-1,"assignment":[],"error":"parse error"}' | /tmp/minz-deep-build-table -out /tmp/minz-deep-error-table.bin
```

The five alias cases each have a search space of one. The cost consistency probe uses 80 × 16,807 static assignments, with a fixed Python RNG seed. It compares the returned assignment against an independently computable additive optimum. This is a bounded probe, not a corpus-scale GPU search.

`builder-error.json` records the converter accepting an error response as infeasible. `gpu-cost-pair.json` contains all mismatches observed in the recorded run, including the cost matrices.

## AY synchronous versus asynchronous paths

Working directory: `gpuforce`.

```sh
GOTOOLCHAIN=go1.25.0 go run ../minz/reports/audit-2026-09-19-deep/audio-probe/main.go
GOTOOLCHAIN=go1.25.0 go test -race ./pkg/ayumi -run '^TestOrchestrator$' -count=1 -timeout 5s
```

The synchronous probe renders samples without a sound device or network. `repeat_identical=true` is a positive control; write-order differences diagnose the separate register semantics issue. The async test is expected to fail on the audited revision; the bounded timeout also protects against shutdown hangs. `ay-race-excerpt.txt` preserves one representative race and the final panic trace.

## Recorded files

- `table-4v.json`: complete 4v scan with source SHA-256 and first alias counterexample.
- `dense-index.json`: real dense-file prefix and calculated memory lower bound.
- `feed-order.json`: canonical-key comparison and observed parallel hashes.
- `minz-probe.json`, `upstream-probe.json`, `builder-error.json`: format/error boundary failures.
- `gpu-alias-{input,output}.jsonl`, `gpu-cost-pair.json`: bounded GPU evidence.
- `audio-render.json`, `ay-race-excerpt.txt`: separate sync/async evidence.
- `mir2-focused.json`, `import-cli.json`: positive production checks.
