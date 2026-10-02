# Shared Z80 judges

These tools require Python 3 and its standard library. Build `mz` with
`cd minzc && go build -o /tmp/mz ./cmd/minzc`.

## Corpus assertions

```sh
python3 scripts/assert_matrix.py --mz /tmp/mz -j 16
python3 scripts/assert_matrix.py --baseline /tmp/main-mz --candidate /tmp/branch-mz -j 16 --json /tmp/matrix.json
```

The default corpus is tracked `examples/nanz/*.nanz`, `examples/c89/**/*.c`,
and `examples/c/*.c`, plus all tracked Pascal (`.pas`), PL/M (`.plm`), ABAP
(`.abap`), Frill (`.frl`), Lanz (`.lanz`), Lizp (`.lizp`), and Objective-C (`.m`) examples. Repeat `--glob` to choose another corpus and use `--root`
for another checkout. Each top-level assert (or whole sandbox) is compiled in separate `--asserts-force z80` and `--asserts-force mir2` passes,
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
By default exit status is 1 if any program fails or cannot be checked.
`--fail-on backend` exits 1 only for Z80/MIR2 mismatches and assembly failures;
all findings still appear in logs, JSON and reproducers. Tool exceptions exit 2
under either policy. Assembly failures take precedence even when MIR2 also
disagrees with the oracle. `--timeout` bounds
each compiler invocation, including reduction attempts.

Run self-tests with `python3 scripts/test_judges.py`; `go test ./pkg/hir` also
runs them, skipping when Python 3 is unavailable.

The initial 500-program findings are recorded in [fuzz_findings.md](fuzz_findings.md).

The compiler is the authority for inventory (`--list-asserts`, JSON lines) and
execution (`--assert-receipt` enables `ASSERTS: executed=N passed=P failed=F`
on stderr). Default compilation prints no receipt; judge scripts request it
explicitly. A successful
isolated check requires exit 0 and exactly one receipt with executed=passed=1,
failed=0. Sandboxes run together and require executed=passed=their member count.
`--assert-lines` selects frontend-parsed assertions, including multiline forms.
Negative controls are enabled by default (`--controls`; skip with
`--no-controls`). For each assertion, a copied source is parsed and its expected
HIR value is mutated by `--assert-control-line` and zero-based
`--assert-control-element`. One control per tuple element flips bit 32 of
exactly that element (or the scalar), putting it outside the Z80 return range.
Any control exiting 0 fails the gate.

Comparison inventories come from separate checkouts and their respective
compilers. `--baseline-root` supplies a checkout; its default is an archive of
local `origin/main`. `--candidate-root` defaults to `--root`. Old binaries need
reporting/selection instrumentation before comparison; missing receipts never
pass. Failing candidate-only assertions also fail the gate. Removed or changed expressions fail unless `--allow-assert-changes` is
set. Added assertions and changed first diagnostics for failures on both sides
are reported. Mixed repeated outcomes are flaky and exit 1. By default JSON has version=2 and a `passes` object with separate `z80` and
`mir2` reports, including wall time. `--backend z80` or `--backend mir2` selects
one pass and retains the version=1 report shape. Zero inventory, baseline passing no units,
Python exceptions exit 2. Listing failures are reported by file for each build;
a candidate listing failure absent from baseline fails the gate. Examples that
fail listing on both builds remain visible in `listing_errors`.

The fuzzer now runs forced MIR2 and Z80 separately. It distinguishes
MIR2≠oracle, Z80≠MIR2 (MIR2 must first agree with the oracle), and assembly
failures. Reduced reproducers must retain their original failure class.

Known Z80 failures are recorded in `scripts/assert_known_failures.json` (override
with `--known-failures PATH`). Each entry identifies a whole execution unit by
file, first line and whitespace-normalized expression; for a sandbox, join all
member expressions in source order with spaces. Entries include the exact full
first-error line, reason and date. Every candidate run must fail with that exact
line to report `known failure`. Only `fail both` and candidate-only `added` units
can be exempted; a baseline-passing regression always fails the gate. Entries
default to Z80; set `backend` to `mir2` for an MIR2 exposure.
A passing entry fails with `known failure fixed — remove the entry`; a changed
diagnostic fails with `known failure changed`; a missing or changed assertion
also fails. Negative controls, flaky runs and inventory checks still apply.
Restricted `--glob` runs audit only entries whose file matches those globs.
Summary classifications retain raw outcomes and also count known failures and
stale entries; JSON results retain the full diagnostics.

Assertions reported at line 0 are grouped as one unit per file, with one
negative control for that unit. Default compiler invocations never select or
mutate assertions: both options take effect only when explicitly supplied.
C `#embed` expansion preserves source lines so listing and selection agree.

For a paired default-mode sweep of all tracked example sources, including
frontends and directories outside the assertion corpus:

```sh
python3 scripts/compile_sweep.py --baseline /tmp/main-mz --candidate /tmp/branch-mz -j 16 --json /tmp/default-sweep.json
```

This compares exit codes and exact stdout/stderr bytes without filtering.
Each source uses a fresh temporary output directory, reused at the same path
for the baseline and candidate. All generated files (assembly, binary and other
emitted files) are compared byte-for-byte, including file presence. JSON reports
the number of emitted files compared and their sizes and SHA-256 digests.
The script reports every difference and exits 1 on differences.

For CI timing, `--no-controls` runs the positive checks alone;
`--controls-only --mz /tmp/branch-mz` runs negative controls alone.
`--runs N` repeats positive checks and controls N times. Each backend report
includes `wall_seconds`; use `-j 4` to measure CI-sized passes.

Explicit `--asserts wasm` and `--asserts llvm` retain MIR2 checks for MIR2
assertions. WASM sandbox assertions referencing absent exports are skipped,
as on main, and skipped assertions are not counted in execution receipts.

`--no-controls` cannot detect a weakened assertion checker when all positive
expectations still pass. CI must run controls at least whenever assertion,
pipeline, or judge script code changes, and nightly. Tuple arity is supplied by
`--list-asserts` as `tuple_elements`; every element receives its own control.

WASM sandbox assertions retain main's acceptance of an empty return result.
Their error messages still include the source line (main omitted it), and call
errors include `call error:`; top-level messages are unchanged.

Run `python3 scripts/test_assert_mutants.py` to build M4 (checks only element 0)
and M5 (skips element 0) in temporary copies, and verify that the tuple fixture's
matrix exits nonzero on both Z80 and MIR2 for each mutant.

## CI checks

The existing required **PR gate** keeps its name and Go/toolchain checks.
The `Compiler checks` reusable workflow builds `mz` from the PR head (not the
merge ref) and `github.event.pull_request.base.sha`, then runs these jobs on
PRs and pushes to main:

- **Assert matrix** compares separate baseline/head inventories on both Z80
  and MIR2. It preserves `assert_matrix.py`'s exit status, including newly
  failing, removed/changed assertions, unexpectedly passing controls, and
  known-failure audit errors. JSON, logs and summaries are uploaded even on
  failure; the log and step summary include the newly failing locations.
- **Judge self-tests** sets `JUDGE_MZ` to the candidate and runs
  `test_judges.py`. When controls are enabled it also builds and checks the
  two tuple mutants with `test_assert_mutants.py`.
- **Default output sweep** compares exact default diagnostics and generated
  files. It is advisory (`continue-on-error`), because the sweep has no
  allowlist and PRs can intentionally change output. Its raw exit status and
  differences remain in the report.
- **Short fuzz** checks fixed seeds 0–2999 with `--no-reduce`. The fuzzer has
  no baseline mode, so this gate checks the candidate alone with
  `--fail-on backend`: Z80/MIR2 mismatches and assembly failures block the PR.
  MIR2/Python-oracle disagreements and other findings remain in the job summary
  and artifacts without failing the PR. No seeds are skipped. Tool exceptions
  still fail the job. Full reproducers and JSON are artifacts.

Workers default to `nproc`. Baseline binaries are cached by full base SHA plus
candidate `minzc/go.sum` hash, OS and architecture; no fallback cache keys are
used. Both binaries and the exact base archive are passed to the jobs within
that workflow run. These new checks are not made required by workflow code;
Alice can configure branch protection separately.

Positive checks normally use `--no-controls`. A plain `git diff` against the
base enables controls and tuple mutant tests for changes under `scripts/`,
`minzc/cmd/minzc/`, `minzc/pkg/pipeline/`, or assertion-parsing frontend
packages (`nanz`, `c89`, `pascal`, `plm`, `abap`, `frill`, `lanz`, `lizp`).
It also covers `hir/hir.go`, `hir/assert*.go`, and the assertion-executing
`mir2wasm/runner.go` and `mir2llvm/runner.go`. Deleted and renamed paths count.
The exact filter is in `scripts/ci/metadata.sh`; extend it when assertion
handling moves. `--no-controls` alone cannot catch a weakened checker.

**Nightly compiler audit** runs at 03:17 UTC and supports `workflow_dispatch`.
It compares main with `HEAD~1`, always enables controls, repeats the matrix
with `--runs 2`, runs both mutant tests, and fuzzes seeds 0–9999 using the
fuzzer's default `--fail-on all` policy. Judge jobs
are report-only; artifacts include their raw statuses and summaries. Build or
infrastructure failures remain visible as workflow failures.

**Advisory Go lint** runs only on PRs from the `minzc/` module using pinned
`golangci-lint v2.13.2` (built with Go 1.26.8). Its minimal configuration
runs govet, staticcheck, errcheck, ineffassign and unused with
`--new-from-rev=<PR base SHA>` over `./pkg/... ./cmd/...`. This scope
excludes the intentionally invalid scratch programs in `minzc/scripts/analysis`
that cannot be package-loaded. `gofmt -l` checks only changed Go files.
Warnings are GitHub annotations; JSON, tool errors and counts are artifacts.
The job uses `continue-on-error`, so lint never blocks the PR.

To reproduce all jobs sequentially with timings and retained logs:

```sh
# Install tools to a writable directory using Go 1.26.8.
GOBIN=/tmp/minz-ci-tools go install github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
GOBIN=/tmp/minz-ci-tools go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2
PATH=/tmp/minz-ci-tools:$PATH BASE_REV=origin/main JOBS=4 CONTROLS=true \
  bash scripts/ci/simulate.sh
```

`BASE_REV` must support assertion inventory, selection and execution receipts;
older pre-J2a binaries cannot be used for a valid comparison. Use a synthetic
base at current `origin/main` to test CI wiring. The script prints its temporary
report directory and each raw exit code; advisory sweep/lint failures do not
change its final status. `CONTROLS` overrides the committed-path filter when
simulating uncommitted changes. For a nightly simulation also set
`NIGHTLY=true RUNS=2 FUZZ_COUNT=10000 BASE_REV=HEAD~1`. To rerun one job, set
`CI_BIN` to the printed compiler directory, `REPORT_DIR` to a writable output
directory, and run `bash scripts/ci/run.sh matrix` (or `judges`, `sweep`, `fuzz`).
`MATRIX_ROOT` selects a disposable checkout; `MATRIX_GLOB` narrows a matrix
run for a regression fixture. CI leaves both unset and audits the full corpus.

To verify that the CI matrix wrapper rejects a codegen regression after building
both compilers:

```sh
CI_BIN=/tmp/minz-ci.YOUR_RUN JOBS=4 bash scripts/ci/check-regression.sh
```

This creates a tracked fixture and compiler copy under `/tmp`, requires exit 0
from the original compiler, substitutes OR for XOR in the copy, then requires
matrix exit 1 with a newly failing Z80 assertion and unchanged MIR2 results.
