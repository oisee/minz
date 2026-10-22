# R2: Grace defaults and infinite-loop preservation

Historical initial proposal; the HIR default and validation claims below are
superseded by [the R2b critic rework](2026-10-22-r2b-grace-default.md).

Baseline: `origin/main` / `840b1c465` (the worktree's starting revision).

## Step 1: preserve reachable infinite loops

The root cause is MIR Grace's `empty-block-elim` action
(`redirectAndDelete`), not HIR Grace or return-based reachability.
For an empty `cycle: jmp cycle`, redirecting predecessors still targets
`cycle`; removing that block then leaves dangling edges. The sibling
`param-forward-elim` action (`elimParamForward`) has the same problem for
self-loops with block arguments. Both actions now reject self-targeting
jumps. Multi-block cycles can still collapse to a valid self-loop.
`EliminateDeadBlocks` already traverses forward from entry and needs no change.

Failing-first tests ran with all compiler files still identical to the baseline:

- `TestGraceKeepsSelfLoopTargets`: both plain and argument-carrying self-loops
  fail verification with `branch to unknown block @cycle`.
- `TestGracePreservesInfiniteLoops`: C `for (;;)` / `while (1)` and Nanz
  `loop {}` / `while true` empty loops fail with MIR Grace, and pass with
  HIR Grace alone. The exact C23 example fails with the reported
  `branch to unknown block @loop_body2` on both assertion backends.
- Side-effecting loops use an external call; MIR VM counts repeated calls
  until its gas limit. A bounded native execution prefix checks that each
  loop stays in its assembled program. Empty VM loops are checked by CFG
  traversal because MIR VM terminators do not consume gas.

Logs: `/mnt/ssd-510/minz/bench/r2-logs/both-before.log` (clean baseline;
initial fixture syntax mistakes were corrected before this replay).

Explicit patch reversal (`git diff` / `git apply -R`), followed by the same
focused tests, reproduced both failures; the patch was restored on exit.
Log: `r2-logs/step1-reversed.log`. With the fix, both new tests pass, along
with `TestGraceDJNZExitRedirectKeepsTarget`,
`TestGraceRunner_PointerThreadingRealLoopShape`,
`TestMIRGraceAssemblyAPIUsesSameNativePipeline`, and
`TestHIRGracePointerWalkAssertions` (`r2-logs/step1-after.log`).

## Step 2: production defaults

The CLI now defaults both `--grace` and `--hir-grace` to true.
`--no-grace` disables both; `--grace=false` and `--hir-grace=false` independently
disable MIR and HIR Grace. `DefaultOptions`, `CompileHIR`, no-options
`CompileHIRSteps`, and meta-specialization use the same production defaults.
Explicit `Options` remain explicit so research variants retain their policies.
The two measured CLI mode admission checks now accept the ordinary defaults
and reject conflicting disable overrides. The benchmark retains `grace` and
adds `nograce`, with default/size options based on `DefaultOptions`.

`TestDefaultOptionsEnableGrace`, `TestGraceCLIFlagsDefaultOn`, and
`TestGraceCLIOverrideAssembly` fail with the default patch explicitly reversed
(`r2-logs/step2-reversed.log`). The new API and flag-default tests passed with
the change (`r2-logs/step2-after.log`); final override replay is recorded below.

Pinned/coverage test changes:

- `TestCompilerModeCLIActualSelectedArtifacts`: origin/main itself failed its
  old mode-1 speed pin. Both baseline and new code measured modes 0/2 at
  2220 T / 46 B and modes 1/3 at 2255 T / 52 B. Pin mode 0 for both
  objectives, retaining all artifact, goal, overwrite, and hash checks.
  Baseline log: `r2-logs/compiler-mode-origin.log` (only logging added there).
- `TestCompilerModeCLIPathsAndFlags` and `TestMemoryHostCLIValidation`:
  conflicting HIR Grace overrides now disable the pass instead of enabling
  an already enabled default; refusal assertions remain unchanged.
- `TestExhaustiveJudgeGCD`: retain an explicit non-Grace variant so the
  new default does not silently collapse two configurations into one.
- `TestPtrWhile_E2E_SumArray`: its hand-written caller pinned count to C;
  Grace's emitted ABI takes it in A. Build argument loads from actual allocation
  metadata instead. The source loop and all native expected sums are unchanged.
- No specialization threshold changed: native measurements remain 26552 T
  (unrolled) and 27523 T (compact), correctly separated by the 27000-T budget.
  No reviewed evidence hash changes have been needed.

Pipeline inventory: 832 tests/examples, exactly classified with sorted fast/deep
lists; both new pipeline tests are in fast.

## Final validation

Completed on 2026-10-22. All heavy commands ran at nice 19, with SSD TMPDIR /
GOCACHE and at most four jobs; load checks delayed starts whenever load exceeded 12.

- Required command: `cd minzc && go test ./pkg/mir2/... ./pkg/hir/...
  ./pkg/c89/... ./pkg/nanz/... ./pkg/z80asm/... -count=1`.
  MIR2 (120.351 s), HIR (56.619 s), C89 (85.887 s), Z80 assembler and its
  regression subpackage passed. The initial Nanz failure was the stale caller
  described above; the complete Nanz rerun passed (135.551 s).
  Logs: `r2-logs/units.log`, `r2-logs/nanz-after.log`.
- Final C23 replay forces each source `via mir2` annotation to the selected
  backend and checks execution receipts: **9 executed / 9 passed on MIR VM**
  and **9 executed / 9 passed on Z80**. The strengthened final test also fails
  with the loop fix explicitly reversed. Logs:
  `r2-logs/loop-receipts-{reversed,after}.log`. Other loop cases retain their
  bounded native execution and repeated-call VM checks.
- Exactly 41 affected pipeline tests passed (48.074 s); related CLI default,
  override, measured-mode and memory-host tests passed (23.542 s).
  The exact selector is `/mnt/ssd-510/minz/tmp/r2-validation.regex` and
  output is `r2-logs/focused-after.log`. Both API and real CLI assembly
  comparisons passed, including independent disabling of each pass.
- Specialization tests, native Spectrum budgets and snapshot boot passed in
  `r2-logs/step2-after.log`; the one stale compiler-mode pin in that initial
  run was independently checked on origin/main and repaired as documented.
- Direct-call seeds 0–499: **500/500 PASS**; folded seeds 0–99:
  **100/100 PASS**, both with `-j 4`. All mismatch/error categories are zero.
  The CLI ran with its defaults, without a Grace wrapper. Logs are
  `r2-logs/direct-call.log` / `r2-logs/folded.log`; exact seed records are
  `/mnt/ssd-510/minz/bench/r2-judges/{direct-call,folded}/results.json`.
- No heldout benchmark, full assert_matrix, or larger fuzz campaign was run.

## Main-suite benchmark

Before: isolated worktree `/mnt/ssd-510/minz/tmp/r2-before`, revision
`840b1c465`; output `/mnt/ssd-510/minz/bench/r2-before`.
After: this branch's default-switch source; output
`/mnt/ssd-510/minz/bench/r2-after`. Both used
`python3 research/c89-benchmark/run.py --sdcc-root / --suite main --jobs 4`.
All 36 before / 48 after MinZ rows passed; the same two SDCC
`struct_roundtrip` compile failures remain visible in both outputs.

Every case's default and size bytes/T are no worse; the Grace column is
unchanged. New default equals old Grace exactly in every case; `nograce`
equals old default exactly. Workload/source hashes and global bytes are
unchanged. Bytes below are the driver's `ProgramBytes`; T is total emulated
T-states over the frozen workload, not host wall time.

| Case | Default B; T | Size B; T | Grace B; T |
|---|---:|---:|---:|
| abs_diff | 27→27; 220,904→220,904 | 27→27; 220,904→220,904 | 27→27; 220,904→220,904 |
| bitwise8reg | 101→101; 872,448→872,448 | 101→101; 872,448→872,448 | 101→101; 872,448→872,448 |
| copy_bytes | 182→182; 301,600→301,600 | 182→182; 301,600→301,600 | 182→182; 301,600→301,600 |
| crc8 | 340→326; 165,616,162→152,877,602 | 340→326; 165,616,162→152,877,602 | 326→326; 152,877,602→152,877,602 |
| fat12_decode | 167→167; 30,336→30,336 | 167→167; 30,336→30,336 | 167→167; 30,336→30,336 |
| gcd | 57→51; 4,953,966→3,067,013 | 57→51; 4,953,966→3,067,013 | 51→51; 3,067,013→3,067,013 |
| memset_fill | 299→299; 18,403,840→18,403,840 | 299→299; 18,403,840→18,403,840 | 299→299; 18,403,840→18,403,840 |
| mul8x8 | 47→47; 1,167,872→1,167,872 | 47→47; 1,167,872→1,167,872 | 47→47; 1,167,872→1,167,872 |
| reverse_loop11 | 561→561; 23,644,160→23,644,160 | 561→561; 23,644,160→23,644,160 | 561→561; 23,644,160→23,644,160 |
| strlen_bytes | 293→293; 19,468,800→19,468,800 | 293→293; 19,468,800→19,468,800 | 293→293; 19,468,800→19,468,800 |
| struct_roundtrip | 71→71; 612,352→612,352 | 71→71; 612,352→612,352 | 71→71; 612,352→612,352 |
| sum_array | 313→313; 7,581,952→7,581,952 | 313→313; 7,581,952→7,581,952 | 313→313; 7,581,952→7,581,952 |

Default and size each total **242,874,392 → 228,248,879 T** (−6.02%).
Grace remains **228,248,879 T**. No regression in any MinZ profile.

## Files changed across both commits

- `CLAUDE.md`
- `minzc/cmd/minzc/grace_defaults_test.go`
- `minzc/cmd/minzc/hir_profile_modes.go`
- `minzc/cmd/minzc/hir_profile_modes_test.go`
- `minzc/cmd/minzc/main.go`
- `minzc/cmd/minzc/memory_host_cli.go`
- `minzc/cmd/minzc/memory_host_cli_test.go`
- `minzc/cmd/minzc/meta_specialize.go`
- `minzc/pkg/hir/exhaustive_judge_test.go`
- `minzc/pkg/mir2/grace_infinite_loop_test.go`
- `minzc/pkg/mir2/grace_runner.go`
- `minzc/pkg/nanz/ptr_while_e2e_test.go`
- `minzc/pkg/pipeline/grace_defaults_test.go`
- `minzc/pkg/pipeline/grace_infinite_loop_test.go`
- `minzc/pkg/pipeline/pipeline.go`
- `reports/2026-10-21-r2-grace-default.md`
- `research/c89-benchmark/README.md`
- `research/c89-benchmark/measure.go`
- `research/c89-benchmark/run.py`
- `scripts/ci/pipeline_profiles.json`
