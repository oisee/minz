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
for another checkout. Each top-level assert (or whole sandbox) is compiled with `--asserts-force z80`,
a 30 second timeout (`--timeout`), and `SOURCE_DATE_EPOCH=0`. Workers have
separate temporary mirrors of tracked files; relative includes/imports resolve
in the same directory layout. The compiler selects parsed assertions with `--assert-lines`, preserving source line numbers. The original files are never modified.

Single mode prints versioned JSON and exits 1 for failures. Comparison prints
counts and regression locations. `--runs K` repeats each invocation; mixed
outcomes are flaky. Default parallelism is the CPU count. Detailed exit rules
and the compiler protocol appear below.

## Differential fuzzing

```sh
python3 scripts/fuzz_diff.py --mz /tmp/main-mz --seed 0 --count 500 -j 16 --output /tmp/fuzz-results
```

This ports the `fuzz2.py` prototype's existing subset: wrapping u8/u16
arithmetic, bitwise operations, division/remainder, casts, helper calls,
comparisons, branches, and bounded loops. Each program seed fixes both the
source and inputs regardless of scheduling. A Python interpreter computes
an assertion checked by both the production MIR2 VM and Z80 emulator.

Wrong-value failures produce `seed-N.nanz`, reduced until no complete helper
function, control block or individual statement can be deleted while preserving a wrong-value
failure, and `seed-N.original.nanz`. This is a local deletion minimum. `--no-reduce` saves full reproducers for
fast smoke triage without deletion reduction. Syntax,
compiler, assembly, timeout and oracle errors are counted separately; compiler failures
save `seed-N.error.nanz`. `results.json` records all seeds and diagnostics.
Exit status is 1 if any program fails or cannot be checked. `--timeout` bounds
each compiler invocation, including reduction attempts.

Run self-tests with `python3 scripts/test_judges.py`; `go test ./pkg/hir` also
runs them, skipping when Python 3 is unavailable.

The initial 500-program findings are recorded in [fuzz_findings.md](fuzz_findings.md).

The compiler is the authority for inventory (`--list-asserts`, JSON lines) and
execution (`ASSERTS: executed=N passed=P failed=F` on stderr). A successful
isolated check requires exit 0 and exactly one receipt with executed=passed=1,
failed=0. Sandboxes run together and require executed=passed=their member count.
`--assert-lines` selects frontend-parsed assertions, including multiline forms.
Negative controls are enabled by default (`--controls`; skip with
`--no-controls`). For each assertion, a copied source is parsed and its expected
HIR value is mutated by `--assert-control-line`; flipping bit 32 puts the value
outside the Z80 return range. Any control exiting 0 fails the gate.

Comparison inventories come from separate checkouts and their respective
compilers. `--baseline-root` supplies a checkout; its default is an archive of
local `origin/main`. `--candidate-root` defaults to `--root`. Old binaries need
reporting/selection instrumentation before comparison; missing receipts never
pass. Failing candidate-only assertions also fail the gate. Removed or changed expressions fail unless `--allow-assert-changes` is
set. Added assertions and changed first diagnostics for failures on both sides
are reported. Mixed repeated outcomes are flaky and exit 1. JSON is an object
with version=1, summary, and results. Zero inventory, baseline passing no units,
enumeration failures and Python exceptions exit 2.

The fuzzer now runs forced MIR2 and Z80 separately. It distinguishes
MIR2≠oracle, Z80≠MIR2 (MIR2 must first agree with the oracle), and assembly
failures. Reduced reproducers must retain their original failure class.

Known Z80 failures are recorded in `scripts/assert_known_failures.json` (override
with `--known-failures PATH`). Each entry identifies a whole execution unit by
file, first line and whitespace-normalized expression; for a sandbox, join all
member expressions in source order with spaces. Entries include the exact full
first-error line, reason and date. Every candidate run must fail with that exact
line to report `known failure` and exempt the unit from the failure gate.
A passing entry fails with `known failure fixed — remove the entry`; a changed
diagnostic fails with `known failure changed`; a missing or changed assertion
also fails. Negative controls, flaky runs and inventory checks still apply.
Restricted `--glob` runs audit only entries whose file matches those globs.
Summary classifications retain raw outcomes and also count known failures and
stale entries; JSON results retain the full diagnostics.
