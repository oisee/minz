# Shared Z80 judges

These tools require Python 3 and its standard library. Build `mz` with
`cd minzc && go build -o /tmp/mz ./cmd/minzc`.

## Corpus assertions

```sh
python3 scripts/assert_matrix.py --mz /tmp/mz -j 16
python3 scripts/assert_matrix.py --baseline /tmp/main-mz --candidate /tmp/branch-mz -j 16 --json /tmp/matrix.json
```

The default corpus is tracked `examples/nanz/*.nanz`, `examples/c89/**/*.c`,
and `examples/c/*.c`. Repeat `--glob` to choose another corpus and use `--root`
for another checkout. Each assert is compiled alone with `--asserts-force z80`,
a 30 second timeout (`--timeout`), and `SOURCE_DATE_EPOCH=0`. Workers have
separate temporary mirrors of tracked files; relative includes/imports resolve
in the same directory layout. Other assertion lines in the selected source
are replaced by blank lines. The original files are never modified.

Single mode prints JSON and exits 1 for any failure. Comparison prints counts
for pass both / newly pass / newly fail / fail both and lists regression
locations with their first diagnostic. It exits 1 only for newly failing
assertions. `--runs K` repeats each compiler invocation; an assertion passes
only when every repetition passes. Default parallelism is the CPU count.

## Differential fuzzing

```sh
python3 scripts/fuzz_diff.py --mz /tmp/main-mz --seed 0 --count 500 -j 16 --output /tmp/fuzz-results
```

This ports the `fuzz2.py` prototype's existing subset: wrapping u8/u16
arithmetic, bitwise operations, division/remainder, casts, helper calls,
comparisons, branches, and bounded loops. Each program seed fixes both the
source and inputs regardless of scheduling. A Python interpreter computes
an assertion checked by the production Z80 compiler/emulator.

Wrong-value failures produce `seed-N.nanz`, reduced until no complete helper
function, control block or individual statement can be deleted while preserving a wrong-value
failure, and `seed-N.original.nanz`. This is a local deletion minimum. Syntax,
compiler, timeout and oracle errors are counted separately; compiler failures
save `seed-N.error.nanz`. `results.json` records all seeds and diagnostics.
Exit status is 1 if any program fails or cannot be checked. `--timeout` bounds
each compiler invocation, including reduction attempts.

Run self-tests with `python3 scripts/test_judges.py`; `go test ./pkg/hir` also
runs them, skipping when Python 3 is unavailable.

The initial 500-program findings are recorded in [fuzz_findings.md](fuzz_findings.md).
