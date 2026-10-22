# R2b: Grace default after critic review

Baseline: `origin/main` at `76e8070d4` (2026-10-22). The rebase retained the
self-loop fix as the first local commit because that fix was absent from main.
This report supersedes the initial R2 default-flip report.

## Compilation policy

The ordinary default is MIR Grace on, HIR Grace opt-in. `--no-grace` disables
both, while `--grace=false` and `--hir-grace` select either half independently.
The historical benchmark `grace` profile explicitly enables both passes;
it no longer aliases whichever options happen to be the current default.
`--grace-ablation` adds independent MIR-only and HIR-only columns.

Linear auto/profile application pins both passes off. This preserves its guard
label proof and the old options fingerprint without accepting incompatible
measurements. Joint search pins MIR Grace off and permits explicit HIR Grace.
Compiler-mode/layout/memory-host searches keep their own measured candidate
options. Explicit conflicting enable requests are rejected with a useful
message. The exact policy is documented in [the CLI guide](../docs/grace-pgo-cli.md).
Executable, HIR, workload and all remaining option checks stay strict.

The indirect-call assembly tests now compare against `DefaultOptions()` and
retain native execution. The CLI override test uses the actual sum-array
pointer loop plus a compare loop, and asserts that each pass changes output.
The automatic linear profile test compares the chosen variant to the actual
native timing row instead of pinning a historical pointer-walk winner; it also
checks fixed options, native execution digests, exact decision totals and saved
model replay.

## Validation and measurements

Artifacts are under `/mnt/ssd-510/minz/bench/r2b-*` and
`/mnt/ssd-510/minz/bench/r2b-logs`. Heavy commands use nice 19, SSD TMPDIR and
GOCACHE, GOMAXPROCS/Go package parallelism at most four, and load gating before
each command. Work is serialized when a benchmark uses four jobs. No heldout
benchmark, assert_matrix, larger fuzz run or push is authorized or performed.

The main suite no longer has the older crc8 Grace improvement: at this
baseline both historical profiles already measure 315 B / 94,778,402 T.
Checksum16 belongs to the heldout suite and is deliberately left to the manager.

### Baseline CLI failures

The full unmodified main suite ran for 1353.065 s and failed these five tests:

- `TestHIRProfileAutoMeasuredCompilation`: expected pointer-walk, measured
  original 311 T vs pointer 416 T.
- `TestHIRProfileForwardCopyCompilation`: expected speed mask 1, selected 0.
- `TestHIRProfileMixedCopyWeightedCompilation`: expected speed mask 7, selected 3.
- `TestCompilerModeReplayActualFrozenHeldout`: pinned 5568 T, measured 5172 T.
- `TestCompilerModeCLIActualSelectedArtifacts`: old speed winner pin disagreed
  with measured mode 0 (2155 T / 46 B).

The first four were identified by the critic. The fifth was independently
reproduced in this run. Winner/timing assertion repairs keep native oracles,
output equality, proof and digest checks; no production selection objective or
cost threshold is changed to satisfy these tests.

### Pass ablation and the size fix

All twelve main cases pass all four pass combinations. MIR-only equals both;
HIR-only equals neither. Only GCD changes at rebased main: 57→51 B and
4,953,966→3,067,013 T. The other eleven main cases are unchanged. This supports
MIR Grace as the ordinary default and leaves HIR Grace opt-in.

Whole assembled module sizes, rebased source and compiler, before the forwarding
profitability fix:

| Source | Neither | MIR only | HIR only | Both |
|---|---:|---:|---:|---:|
| grace_seeded_sum | 54 | 54 | 148 | 150 |
| 01_sum_array | 171 | 171 | 247 | 254 |
| grace_weighted_checksum | 143 | 143 | 227 | 237 |
| c99_control | 162 | 177 | 162 | 177 |
| c99_preproc | 267 | 288 | 267 | 288 |

The MIR regressions came from eliminating forwarding blocks with outgoing
arguments reached by conditional edges. Allocation/codegen then had to recreate
the copies in out-of-line trampolines, increasing branches and live ranges.
Both empty-block and parameter-forward actions now conservatively retain these
blocks. Unconditional forwarding and blocks without outgoing copies remain
eligible. This is a CFG-shape profitability guard, with no function-name or
input-value dependence; declining a rewrite preserves the original semantics.
The separately committed self-loop guards remain intact.

After the forwarding fix:

| Source | Neither | MIR only (default) | HIR only | Both |
|---|---:|---:|---:|---:|
| grace_seeded_sum | 54 | 54 | 148 | 148 |
| 01_sum_array | 171 | 171 | 247 | 247 |
| grace_weighted_checksum | 143 | 143 | 227 | 228 |
| c99_control | 162 | 162 | 162 | 162 |
| c99_preproc | 267 | 267 | 267 | 267 |

`TestGraceConditionalCopiesStayInlineNativeVM` first failed on the old MIR
runner (`Grace=true moved conditional copies into trampolines`), then passed
with the guard: low/high selection and nested clamping, 36 execution assertions
per backend and pass setting, no trampolines and no size increase. The default
compare-loop test also executes on both backends and requires a fused `br_if2`.
It fails on clean main and with the pipeline patch explicitly reversed, then
passes restored. Logs: `copies-{before,after}.log`,
`failing-first-{main,reversed}.log`, `focused-pipeline.log`.

The focused CLI checks pass (56.245 s), including all critic failures and the
five stale baseline expectations. Current mode costs are 2155 T / 46 B for
modes 0/2 and 2190 T / 52 B for 1/3. Copy candidates measure 5432 T / 15 B
(original) vs 34366 T / 137 B (pointer), so original wins both goals. Mixed copy
mask 3 is fastest at 69,491 T / 285 B; mask 2 is smallest at 174 B (108,185 T).
The retained proof, independent native-state oracles and exact output/digest
checks all pass. No measurement threshold was changed.

### Validation orchestration note

Automatic approval review declined a proposed concurrent measurement; the
existing serialized measurement queue was retained. Later, cancelling a queued
validation shell left its child alive. That orphan briefly overlapped the final
unit/CLI work. Both overlapping CLI process groups were stopped, and their CLI
logs were discarded as acceptance evidence. Final CLI validation restarted in a
clean log (`after-cli-final.log`), followed by pipeline, judges and benchmarks in
one process queue. The completed final unit run passed all packages. The
orchestration mistake is recorded here rather than claiming uninterrupted
serialization throughout the session.

### Required package tests

Before the additional comparison-domain guard, the source passed the exact requested package command, `-count=1`:
MIR2 128.775 s, HIR 61.487 s, C89 93.333 s, Nanz 138.347 s,
Z80 assembler 0.084 s and assembler regression subpackage 0.009 s.
`units-before-domain.log` ends with exit status 0. Pipeline inventory has 854 entries,
all classified without duplicates, with a sorted fast list including all new
tests. Only the requested affected pipeline selector is run, not the whole
package.

### Full CLI result

`go test ./cmd/minzc -count=1 -timeout 60m -v` passed before the comparison-domain guard:
1067.279 s, exit 0 (`cli-before-domain.log`). All five baseline failures now pass:
auto 17.13 s, forward-copy 2.31 s, mixed-copy 7.31 s, frozen-mode replay 4.99 s,
actual compiler-mode artifact selection 15.23 s. No final CLI failure remains.
The full run also covers ordinary indirect-call assembly, all profile/site
modes, layout/mode replay, memory-host policy/search/legacy replay and native
meta-specialization/Spectrum snapshot checks.

### Additional legality fix before final validation

Review found that `fuse-cmp-brif2` accepted comparisons without proving its
terminator's domain. `TermBrIf2` emits an unsigned byte CP, so signed ordering
and 16-bit operands cannot be treated this way. The new
`TestGraceThreeWayComparisonDomainNativeVM` failed before the fix: signed
`(-128,127)` returned 1 instead of 2 on MIR VM. After the fix it passes all 11
assertions on each of MIR VM and Z80, including word values with identical low
bytes, 65535, negative signed bytes, equality and zero-trip cases.

Fusion now proves that both registers are unsigned bytes from parameter,
block-parameter and instruction type facts; rejects incompatible source type
metadata and hard flag constraints; and requires a comparison-only,
single-predecessor, non-entry body with no parameter remapping. Both comparison
results must have one use. This prevents high-byte/signedness loss, speculation
across the equality exit and deletion of a shared body. Unknown facts decline
fusion. The unsigned-byte positive test still emits the fused branch.

The earlier pipeline run was stopped for this source correction; its partial
log is `pipeline-pre-domain.log`, not final acceptance evidence. It also found
stale parser reference assertions. All three consumers reproduce on clean main
at 1,260,288 T / 2,269 B, SHA256
`9209f1d325bd81d808a5dcd28c20ef5de73189a4180c88397de49029be3b052b`.
The prior pin was 1,261,070 T / 2,289 B. The test-only reference is refreshed;
source, MIR, allocation, ASM and BIN are byte-identical between main and this
branch for direct, factory and instrumented routes, with all independent
parser state/event oracles retained. Full before/after exports are in
`r2b-parser-reference/{main,after}`. Focused corrected tests pass in 11.908 s.

## Final main-suite benchmark

Before: clean main `76e8070d4`, `/mnt/ssd-510/minz/tmp/r2b-main`,
`r2b-before`. After: corrected default and both profitability/legality guards,
`r2b-after`. Both commands use `run.py --sdcc-root / --suite main --jobs 4`;
after additionally uses `--grace-ablation` to retain all four pass combinations.
All 36 before / 72 after MinZ rows pass. Both runs retain the same two SDCC
struct-roundtrip compile errors; none is a MinZ failure. Source and workload
hashes and declared global storage agree case by case. Bytes are ProgramBytes;
T is total emulated execution over the frozen workload.

| Case | Default B; T | Size B; T | Grace B; T |
|---|---:|---:|---:|
| abs_diff | 27→27; 220,904→220,904 | 27→27; 220,904→220,904 | 27→27; 220,904→220,904 |
| bitwise8reg | 95→95; 831,488→831,488 | 95→95; 831,488→831,488 | 95→95; 831,488→831,488 |
| copy_bytes | 172→172; 109,332→109,332 | 172→172; 109,332→109,332 | 172→172; 109,332→109,332 |
| crc8 | 315→315; 94,778,402→94,778,402 | 315→315; 94,778,402→94,778,402 | 315→315; 94,778,402→94,778,402 |
| fat12_decode | 167→167; 30,336→30,336 | 167→167; 30,336→30,336 | 167→167; 30,336→30,336 |
| gcd | 57→51; 4,953,966→3,067,013 | 57→51; 4,953,966→3,067,013 | 51→51; 3,067,013→3,067,013 |
| memset_fill | 297→297; 3,588,390→3,588,390 | 297→297; 3,588,390→3,588,390 | 297→297; 3,588,390→3,588,390 |
| mul8x8 | 45→45; 1,096,192→1,096,192 | 45→45; 1,096,192→1,096,192 | 45→45; 1,096,192→1,096,192 |
| reverse_loop11 | 561→561; 23,644,160→23,644,160 | 561→561; 23,644,160→23,644,160 | 561→561; 23,644,160→23,644,160 |
| strlen_bytes | 283→283; 3,574,400→3,574,400 | 283→283; 3,574,400→3,574,400 | 283→283; 3,574,400→3,574,400 |
| struct_roundtrip | 59→59; 567,296→567,296 | 59→59; 567,296→567,296 | 59→59; 567,296→567,296 |
| sum_array | 313→313; 7,581,952→7,581,952 | 313→313; 7,581,952→7,581,952 | 313→313; 7,581,952→7,581,952 |
Regressions: []
default T 140976818 → 139089865
size T 140976818 → 139089865
grace T 139089865 → 139089865

Default/size total 140,976,818→139,089,865 T (−1.34%), and 2391→2385 B.
Historical Grace remains 139,089,865 T / 2385 B. No main-suite profile regresses.
MIR-only equals both passes, and HIR-only equals neither, in every main case.
The extra legality guard preserves the full GCD benefit.

## Final source unit result

After the signed/word legality fix, the exact required package command passes
again (`units.log`, exit 0): MIR2 151.840 s, HIR 79.361 s, C89 115.391 s,
Nanz 162.892 s, assembler 0.095 s, assembler regression 0.008 s.
The exact new comparison-domain test also fails on unmodified origin/main
(`domain-main.log`, got 1 instead of 2) and passes on this branch on both
backends. Inventory is now 855 entries with all four new pipeline tests in the
sorted fast list.

## R2b files touched

- `CLAUDE.md`
- `docs/grace-pgo-cli.md`
- `minzc/cmd/minzc/default_output_test.go`
- `minzc/cmd/minzc/grace_defaults_test.go`
- `minzc/cmd/minzc/hir_profile.go`
- `minzc/cmd/minzc/hir_profile_auto.go`
- `minzc/cmd/minzc/hir_profile_auto_test.go`
- `minzc/cmd/minzc/hir_profile_forward_copy_test.go`
- `minzc/cmd/minzc/hir_profile_grace_policy_test.go`
- `minzc/cmd/minzc/hir_profile_modes.go`
- `minzc/cmd/minzc/hir_profile_modes_replay_test.go`
- `minzc/cmd/minzc/hir_profile_modes_test.go`
- `minzc/cmd/minzc/indirect_emission_test.go`
- `minzc/cmd/minzc/main.go`
- `minzc/cmd/minzc/memory_host_cli.go`
- `minzc/cmd/minzc/memory_host_cli_test.go`
- `minzc/pkg/mir2/grace_runner.go`
- `minzc/pkg/pipeline/grace_compare_domain_test.go`
- `minzc/pkg/pipeline/grace_conditional_copies_test.go`
- `minzc/pkg/pipeline/grace_defaults_test.go`
- `minzc/pkg/pipeline/memory_host_baseline_test.go`
- `minzc/pkg/pipeline/pipeline.go`
- `minzc/pkg/pipeline/profile.go`
- `minzc/pkg/pipeline/profile_cost_domain_test.go`
- `reports/2026-10-21-r2-grace-default.md`
- `reports/2026-10-22-r2b-grace-default.md`
- `research/c89-benchmark/README.md`
- `research/c89-benchmark/measure.go`
- `research/c89-benchmark/run.py`
- `scripts/ci/pipeline_profiles.json`

## Final source full CLI result

After all production corrections, `go test ./cmd/minzc -count=1 -timeout 60m -v`
passes again: 104 top-level tests, zero failures, 1077.973 s, exit 0.
`after-cli-final.log` is the acceptance log. All five clean-main failures are
resolved; all ordinary, auto/joint/PGO, compiler/layout replay, memory-host and
meta-specialization paths in the complete CLI package pass. The required final
pipeline selector contains 163 entries after adding the domain regression.

## Final pipeline and judge result

The full requested selector, `go test ./pkg/pipeline -run
'Profile|Grace|CompilerMode|MemoryHost' -count=1 -timeout 60m -v`, completed in
3050.565 s: 155 passed, seven optional checks skipped, one fixed-timing test
failed. Clean main reproduces that same failure: `TestSumProfileCostFullU8Domain`
expects 278 T at n=1 but actual native execution is 268 T. Its positive-trip
entry reference is refreshed 241→231 T; no production cost model/selection
threshold changes. The complete focused rerun passes in 55.865 s, verifying all
1022 address/length scenarios on original/pointer MIR VM and assembled Z80,
including equality, fallback, wrapping and all u8 lengths. Every selected
functional test is therefore verified: 156 passed across the full run and the
corrected focused replay; no unresolved failure remains. Production code did
not change after the full run. Acceptance logs: `after-pipeline.log`,
`main-cost-domain.log`, `cost-domain-after.log`; full rows in
`/mnt/ssd-510/minz/bench/r2b-cost-domain/full-u8-cost-domain.json`.

The seven skips retain existing optional fixture/export guards:
`TestCombinedContractTableGraceNative`,
`TestMemoryHostPolicySelectorOriginalParserExport`,
`TestMemoryHostArenaPeelDiagnostic`, `TestMemoryHostArenaPeelShapeExport`,
`TestMemoryHostPolicySearchOriginalParserExport`,
`TestMemoryHostPolicySearchShortCrossoverExport`, and
`TestMeasuredCommonGraceArtifacts`.

- Direct-call fuzz: seeds 0–499, `-j 4`, **500/500 pass**, exit 0.
- Folded fuzz: seeds 0–99, `-j 4`, **100/100 pass**, exit 0.
- Both judges: MIR2/oracle, Z80/MIR2, assembly/compiler/oracle errors all zero.
- No larger campaign, heldout benchmark or assert_matrix was run.

## Final critic-fixture audit

`size-final.jsonl` records all 20 pass/fixture combinations, native image and
ABI hashes, and public function/global addresses. Default MIR-only and neither
have the same bytes, ABI hash and public addresses in all five fixtures. Four
images are byte-identical. In c99_control the sole instruction difference is
threading `JRS .clamp_cond_join6` directly to `.clamp_cond_join3`, bypassing an
unconditional jump; the other differences are removed label aliases/comments.
Thus the clamp in-range path saves one short unconditional jump (12 T), and
other paths retain their costs. No critic-fixture default grows or slows.
C99 MIR-only footprint regressions are removed: 177→162 and 288→267 B.

The benchmark source was measured from this working tree before the requested
end-of-task local commit; the recorded base commit is 0c396c11e and this commit
contains the compiled production changes. The untouched-main baseline is
76e8070d4. No push is performed. The separate self-loop fix remains the first
local commit because it was absent from the main revision used for rebase.
