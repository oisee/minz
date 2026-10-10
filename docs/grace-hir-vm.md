# Grace on HIR, checked through MIR2 VM

The opt-in `--hir-grace` pass runs the existing MinZ Grace engine over a
structural HIR view before function splitting and lowering. `--grace` remains
the independent MIR2 switch. Both can be enabled together:

```text
HIR0 -> HIR Grace -> HIR1 -> lowering -> MIR0 -> MIR Grace -> MIR1
```

The first rule is [constant_if.grace](../minzc/pkg/hir/rules/constant_if.grace).
It selects the live arm of an `if` with a literal boolean condition and keeps
that arm as a structured block. It preserves calls, stores, return and loop
control in the selected arm. Dynamic conditions are untouched. Functions with
labels/gotos are excluded, as are selected arms containing local bindings
(including loop variables), because lowering treats branch environments
differently from ordinary blocks. This is a plumbing experiment, not a claim
of performance improvement.

The HIR adapter exposes lexical blocks with deterministic function/path IDs;
it does not pretend they are CFG blocks. Register facts and CFG edges are
unavailable. Mutations use a registered HIR action. No abapiti code or Datalog
engine is imported in this experiment.

Each firing rebuilds the view and structurally verifies lowered MIR2. A failed
MIR2 verification restores the affected `if` and returns an error. There is no
full HIR verifier yet. The module-wide budget defaults to 100 rewrites; hitting
the budget stops safely even if candidates remain. `hir.RunGrace` returns
per-rule counts. A second invocation continues from that point.

## Run

From `minzc/`, with Go 1.24 or later:

```sh
go run ./cmd/minzc ../examples/nanz/grace_hir_constant_if.nanz --emit=mir2-raw -o /tmp/grace-before.mir
go run ./cmd/minzc ../examples/nanz/grace_hir_constant_if.nanz --hir-grace --emit=mir2-raw -o /tmp/grace-after.mir
go run ./cmd/minzc ../examples/nanz/grace_hir_constant_if.nanz --hir-grace --grace --emit=mir2 -o /tmp/grace-both.mir
go test ./pkg/hir ./pkg/pipeline -run '^(TestGraceHIR|TestHIRGrace)' -count=1
```

All assertions in this example use MIR2 VM; these commands do not require Z80
emission. The tests lower separately constructed original and rewritten HIR,
then use fresh VMs to compare return values, observed global memory and ordered
host calls for all 256 u8 inputs, including wraparound. They also check nested
loops, a no-else branch, selected/discarded division-by-zero traps, rewrite
budgets, idempotence and conservative guards. Independent expected values
prevent two equally wrong VM runs from passing the arithmetic checks.

Pipeline tests cover all four HIR/MIR Grace switch combinations, including an
intentionally failing MIR2 assertion. Passing asserts must stay passing and
failing asserts must stay failing.

These are differential tests, not a general equivalence proof. Error tests
compare trap causes because block names change. Step counts are not observable
program output; reaching a VM gas limit would make an equivalence comparison
inconclusive. Future rules need their own effect guards, VM scenarios and
memory/I/O observations.


## Measurements (2026-10-10)

Scope: the two-function example above, Go 1.26.8, linux/amd64, Ryzen 7 5800X.
Timings are medians of five runs of 2000 iterations with `-cpu=1`. Parsing is
excluded from compile timings; the MIR2 pipeline includes its four VM asserts
and skips Z80 emission. VM timings include a fresh VM and 512 calls per batch.

| Metric | Ordinary path | With HIR Grace |
|---|---:|---:|
| Raw MIR2 blocks | 6 | 2 |
| Raw MIR2 instructions, excluding terminators | 10 | 2 |
| Raw VM instruction steps, 512 calls | 2048 | 512 |
| Optimized MIR2 blocks | 4 | 2 |
| Optimized MIR2 instructions, excluding terminators | 6 | 2 |
| Optimized VM instruction steps, 512 calls | 1280 | 512 |
| Lowering, including Grace/validation when enabled | 5.81 us | 41.91 us |
| MIR2 pipeline time | 137.71 us | 144.90 us |
| MIR2 pipeline allocated bytes | 148784 | 147602 |
| Raw VM batch time | 217.62 us | 173.21 us |
| Optimized VM batch time | 190.88 us | 173.52 us |

The optimized instruction-step reduction is 60%; observed optimized VM batch
elapsed time falls by about 9.1%. After adding fact extraction and a second
rule, the MIR2 pipeline costs about 5.2% more; Grace plus lowering alone costs
about 36 us more. These timings replace the earlier one-rule prototype results.
These are small-example results, not corpus
or Z80 speedups. Both ordinary Go MIR2 passes and MIR2 Grace produce the same
4-block/6-instruction baseline here. Terminators are not counted by the VM's
current gas counter. There were no other literal `if true/false` matches in the
existing `.nanz` example sources in this checkout; this rule alone does not
establish useful coverage on real programs.

Reproduce from `minzc/`:

```sh
go test ./pkg/pipeline -run '^TestHIRGraceMeasurements$' -v -count=1
go test ./pkg/pipeline -run '^$' -bench '^BenchmarkHIRGrace' -benchtime=2000x -count=5 -cpu=1
```

The benchmarks and counters are in `pkg/pipeline/hir_grace_measure_test.go`.

The second rule, `hir-invariant-base`, hoists typed invariant address-base additions
in real `irc_client.nanz:region_match`. It saves 199 VM instructions over 200 bytes
after ordinary MIR passes or MIR Grace (2403 → 2204), with 11.7% less VM-call time
in a five-run measurement. See [the full report](grace-hir-vm-e2e.md#11-реальное-правило-неизменная-база-адреса-в-цикле)
for source, dumps, fact analysis, guards and overflow/capture tests.
