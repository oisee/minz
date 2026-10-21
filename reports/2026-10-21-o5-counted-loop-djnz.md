# O5: HIR counted loops to DJNZ (2026-10-21)

The original O5 record below is historical. The O5b follow-up at the end
supersedes its admission policy and records measurements against dda8fab65.

The HIR lowerer recognises unsigned byte `while i < hi` / `while i != hi`
loops with a trailing `i = i + 1`, including the C89 frontend's postfix
snapshot. Nanz ForRange loops already synthesise that shape, so they share
this path. Direct constant endpoints also admit unsigned 16-bit induction
when their non-wrapping difference is at most 255.

The entry condition guards zero trips and reversed `<` ranges. A separate
byte counter uses ClassCounter and TermDJNZ. A body that reads the induction
variable carries and increments it; otherwise the taken exit reconstructs
its final value from the saved bound. The zero-trip exit retains the start.
The bound is evaluated once. Source HIR is not mutated, and source-loop
provenance is preserved.

Conservative exclusions: signed induction, unproven wide counts, calls,
volatile accesses, memory-loaded C bounds, aliased scalar binders, mutation of either endpoint,
control transfers, and indexing by the induction variable. Indexing retains
the existing MIR pointer-walk opportunity: an exploratory version that also
converted those loops regressed copy_bytes and sum_array and was discarded.
Consequently memset_fill and the outer indexed crc8 loop retain their old
lowering; crc8's fixed-count inner loop benefits. No 16-bit DEC countdown or
new pointer-walk transformation is included.

## Failing-first evidence

The final three TestCountedLoop tests were copied into a detached origin/main
worktree at 68f199c7e and run with `go test ./pkg/pipeline -run
'^TestCountedLoop' -count=1`. Exit 1:

```
TestCountedLoopDJNZVMZ80: counted loop must lower to TermDJNZ <
TestCountedLoopC89AndRangeShape: C counted loops: got 0 DJNZ, want 2
TestCountedLoopProofAndFallback: range start width u16 false
```

The tests check MIR shape and verification, execute MIR VM and Z80 emulator
at byte boundaries (including != wraparound), observe the post-loop C
induction value, exercise range-loop continue fallback, and verify refusals.
All three are registered in the sorted pipeline fast inventory.

An additional runtime regression reads i and the bound only inside a
conditional expression. Temporarily restoring the older use scanner makes
TestCountedLoopDJNZVMZ80 fail: n=3 returns 3, expected 5. Restoring the complete
expression walker passes MIR VM and Z80 checks, including n=128 and n=255.
The complete walker also collects aggregate operands and indexed targets.

The full audit exposed a new DJNZ native Fibonacci regression: fib(2) returned
2 rather than 1, while origin/main passed. Z80 override restoration discarded
a saved A value merely because its loop parameter would be overwritten on the
next iteration, even though edge arguments still read that old value. The
codegen fix preserves relocated values needed by the terminator or live-out
successors. The new Fibonacci state regression and all three original
Fibonacci compiler profiles pass after the fix.

## Validation

All three new counted-loop tests fail on the detached baseline. The combined
counted-loop and original Fibonacci/pipe focused run passes (3.102s).
The complete frozen-source run passed MIR2 (153.214s), HIR (56.502s), C89
(91.845s), Nanz (166.060s), z80asm (0.118s) and its regression package
(0.009s), exit 0. The requested unsharded full pipeline command timed out at 30 minutes
(1800.033s), during TestMemoryHostPolicySelectorFixedPolicy/0/speed;
no earlier FAIL lines were emitted. It is not reported as passing.
A complete non-short inventory audit selected all 828 tests in 12 batches,
four workers, with a 30-minute per-process timeout. Nine batches passed;
batch 8 hit TestSeededPointerWalkMeasuredCaller; batch 6 timed out during
TestMemoryHostPolicySearchLifecycle/48; batch 10 timed out during
TestTetrisLineProgressionThresholdMachine/lines-31/grace-size. The existing CI runner was copied to
/tmp with only -test.short replaced by -test.count=1; the repository runner
and pre-existing tests were not modified. The test active at timeout passes in isolation on origin/main (264.435s).
The seeded-pointer performance assertion also fails identically on
origin/main: masks 0/2 measure 120327T / 57B and masks 1/3 measure
144105T / 150B. Its fixed requirement that mask 1 win is already false.
The later Fibonacci failure in batch 10 was new and has been fixed, rather
than classified as a baseline issue. The original Fibonacci/pipe test and parallel-state counted-loop regressions
pass after the codegen correction. The final fast-profile rerun passed all 457 selected tests in eight
batches, four workers, without -short, exit 0. The complete deep audit is
not green: the pre-existing seeded-pointer performance assertion and the two
timeouts remain explicitly reported; timed-out batches did not finish all
selected tests.
The matcher also rejects narrow arithmetic steps and typed literals outside
their representable domain, so truncation cannot masquerade as a +1 step or
an in-range bound. Final verification used nice -n 19, the SSD TMPDIR/GOCACHE, and stage
parallelism capped at four jobs.

The candidate-only assertion matrix ran both backends over 1,731 units:
Z80 1,596 pass / 135 fail; MIR2 1,632 pass / 99 fail. It returned exit 1.
All 1,739 negative controls per backend were rejected (zero unexpected passes).
All failing/listing-error files were then compared with origin/main using the
final binary: 214 units per backend, zero newly failing checks, no new listing
errors, zero unexpected control passes, comparison exit 0. The same 135 Z80
and 99 MIR2 failures occur on baseline; 43 diagnostic differences per backend
are temporary-path differences only. These existing unsupported-feature and
example failures are not described as a clean candidate-only matrix.

Direct-call fuzz, seeds 0–1999, -j 4: 2000/2000 pass, exit 0.
Folded fuzz, seeds 0–299, -j 4: 300/300 pass, exit 0.
Both report zero MIR/oracle mismatches, Z80/MIR mismatches, assembly,
compiler or oracle errors. The final main-suite benchmark refresh after the codegen correction also
passes all 36 MinZ builds with zero byte or T-state regressions.

Final status: EXIT 1. The new focused checks, compiler/assembler suites, fast
pipeline, differential judges, assertion comparison and main benchmarks pass.
The requested complete pipeline gate did not finish green: the existing seeded
performance assertion and the deep timeouts above are not hidden or waived.
No heldout benchmark suite was run. Changes are committed locally; no push.

## Main-suite benchmark

Before: detached origin/main 68f199c7e, output
`/mnt/ssd-510/minz/bench/o5-before`. After:
`/mnt/ssd-510/minz/bench/o5-after`. Both used `run.py --sdcc-root / --suite main
--jobs 4`; heldout was not run. Cells are initialized program bytes and total
workload T-states, before → after. All 36 MinZ builds PASS, with no byte or
T-state regression. Default/size crc8 saves 15 bytes and 30.5% T-states;
Grace saves 5 bytes and 25.8% T-states.

| Case | Default bytes; T | Size bytes; T | Grace bytes; T |
|---|---|---|---|
| abs_diff | 27→27; 220,904→220,904 | 27→27; 220,904→220,904 | 27→27; 220,904→220,904 |
| bitwise8reg | 101→101; 872,448→872,448 | 101→101; 872,448→872,448 | 101→101; 872,448→872,448 |
| copy_bytes | 182→182; 301,600→301,600 | 182→182; 301,600→301,600 | 182→182; 301,600→301,600 |
| crc8 | 342→327; 176,060,962→122,368,162 | 342→327; 176,060,962→122,368,162 | 328→323; 163,322,402→121,216,802 |
| fat12_decode | 167→167; 30,336→30,336 | 167→167; 30,336→30,336 | 167→167; 30,336→30,336 |
| gcd | 57→57; 4,953,966→4,953,966 | 57→57; 4,953,966→4,953,966 | 51→51; 3,067,013→3,067,013 |
| memset_fill | 299→299; 18,403,840→18,403,840 | 299→299; 18,403,840→18,403,840 | 299→299; 18,403,840→18,403,840 |
| mul8x8 | 51→51; 1,331,712→1,331,712 | 51→51; 1,331,712→1,331,712 | 51→51; 1,331,712→1,331,712 |
| reverse_loop11 | 587→587; 39,732,480→39,732,480 | 587→587; 39,732,480→39,732,480 | 587→587; 39,732,480→39,732,480 |
| strlen_bytes | 293→293; 19,468,800→19,468,800 | 293→293; 19,468,800→19,468,800 | 293→293; 19,468,800→19,468,800 |
| struct_roundtrip | 71→71; 612,352→612,352 | 71→71; 612,352→612,352 | 71→71; 612,352→612,352 |
| sum_array | 313→313; 7,581,952→7,581,952 | 313→313; 7,581,952→7,581,952 | 313→313; 7,581,952→7,581,952 |


## O5b: profitability correction (2026-10-21)

Base: origin/main dda8fab65. Original O5: 84f589608. O5b keeps the
existing correctness proofs and restoreLiveOutOverrides fix, but admits only
loops with directly proven constant endpoints whose body does not read the
induction variable. Compiler-generated bound temporaries are accepted only
when their MIR definition is a constant or a value-preserving integer cast/move
of one. Wrapping truncations, negative values and sign-changing casts are
refused; TestCountedEndpointConstantCasts covers these cases. Dynamic loops retain
ordinary lowering, including nested loops with dynamic bounds. This is an
IR-fact gate, with no source-name or benchmark-name matching.

The first attempted gate admitted dynamic loops with an unused IV and byte
state. Although it fixed the three slow hot loops, the seven-function module
still occupied 241 bytes versus 223 on base. That attempt was discarded.
The final gate gives up the dynamic-loop wins as well as the regressions;
it preserves the fixed-count bit-loop opportunity in crc8. It does not claim
to fix general back-edge trampolines, INC selection, pair copies or entry
merges. Those require separate codegen work with its own correctness proofs.
The pre-existing 300..310 wide-range bug is outside this change.

### Critic probes: measured base / O5 / O5b

All three measurements use the unmodified source from crit-o5/zz_crit_test.go,
Options{AssertMode:"none"}, and measureGraceZ80Compiled. T-states cover function
entry through RET; bytes cover the complete seven-function module, including
spill storage. Baseline and original O5 were measured in separate detached
worktrees. The original critic directory was read only.

| Probe and arguments | Base T | O5 T | O5b T |
|---|---:|---:|---:|
| m8(100,3) | 53,541 | 58,705 | 53,541 |
| rng(5,200) | 9,190 | 7,667 | 9,190 |
| cnt(200) | 11,241 | 6,505 | 11,241 |
| xr(200) | 8,241 | 12,701 | 8,241 |
| fib(20) | 942 | 1,511 | 942 |
| tri(60) | 144,482 | 44,585 | 144,482 |
| many(100,3,5) | 19,724 | 16,306 | 19,724 |
| **Module bytes** | **223** | **382** | **223** |

TestCountedLoopProfitabilityVMZ80 compiles both canonicalization settings
using a per-compilation DisableCountedLoops option, verifies raw MIR, executes
the MIR VM and Z80 emulator against a Go reference, and checks T(new) <= T(old)
and module bytes(new) <= bytes(old). It covers the seven original workloads
and zero/reversed ranges, byte arithmetic wraparound and n=255. Both new
pipeline tests are registered in the sorted fast inventory.

### Failing-first evidence

On detached origin/main dda8fab65, TestCountedLoopFixedProfitability fails:
`fixed-count loop must use TermDJNZ`. The same test checks raw MIR shape,
emitted DJNZ, and MIR/Z80 results with O5b. The new performance test also
fails on original O5 84f589608 with only the differential-option scaffolding
copied into that detached worktree: m8 53541 -> 58705 T, xr 8241 -> 12701 T,
fib 942 -> 1511 T, and module bytes 223 -> 382. With O5b these are equal to
ordinary lowering (see table). Logs: /tmp/o5b-baseline-probes.log,
/tmp/o5b-original-probes.log and /tmp/o5b-focused-final.log.

TestZ80LiveOutRestoreOverwrittenParam calls restoreLiveOutOverrides directly,
without HIR canonicalization, allocation or DJNZ emission. It checks back-edge
arguments, exit arguments, successor liveness, dead values, ordinary jumps and
returns, including flags and stack preservation on the emulator. On origin/main
its backArgument, exitArgument and liveOut cases fail: A remains 0x27 instead
of the saved 0x53. The ordinary-jump, return and dead cases pass there.
Baseline log: /tmp/o5b-baseline-restore.log.


### O5b main-suite matrix

Fresh before: detached origin/main dda8fab65, output
/mnt/ssd-510/minz/bench/o5b-before. After: frozen O5b working tree before
its local follow-up commit, output /mnt/ssd-510/minz/bench/o5b-after.
Both ran `nice -n 19 python3 research/c89-benchmark/run.py --out ...
--sdcc-root / --suite main --jobs 4`, with SSD TMPDIR/GOCACHE and Go build
parallelism capped at four. All 36 MinZ measurements PASS; zero byte or
T-state regressions. Default and size crc8 retain the full O5 saving of
53,692,800 T (32.42% against this baseline) and 15 bytes; Grace saves
42,105,600 T (27.54%) and five bytes. No heldout suite was run.
Cells show complete program bytes; total workload T-states, before → after.

| Case | Default bytes; T | Size bytes; T | Grace bytes; T |
|---|---|---|---|
| abs_diff | 27→27; 220,904→220,904 | 27→27; 220,904→220,904 | 27→27; 220,904→220,904 |
| bitwise8reg | 101→101; 872,448→872,448 | 101→101; 872,448→872,448 | 101→101; 872,448→872,448 |
| copy_bytes | 182→182; 301,600→301,600 | 182→182; 301,600→301,600 | 182→182; 301,600→301,600 |
| crc8 | 340→325; 165,616,162→111,923,362 | 340→325; 165,616,162→111,923,362 | 326→321; 152,877,602→110,772,002 |
| fat12_decode | 167→167; 30,336→30,336 | 167→167; 30,336→30,336 | 167→167; 30,336→30,336 |
| gcd | 57→57; 4,953,966→4,953,966 | 57→57; 4,953,966→4,953,966 | 51→51; 3,067,013→3,067,013 |
| memset_fill | 299→299; 18,403,840→18,403,840 | 299→299; 18,403,840→18,403,840 | 299→299; 18,403,840→18,403,840 |
| mul8x8 | 47→47; 1,167,872→1,167,872 | 47→47; 1,167,872→1,167,872 | 47→47; 1,167,872→1,167,872 |
| reverse_loop11 | 587→587; 39,732,480→39,732,480 | 587→587; 39,732,480→39,732,480 | 587→587; 39,732,480→39,732,480 |
| strlen_bytes | 293→293; 19,468,800→19,468,800 | 293→293; 19,468,800→19,468,800 | 293→293; 19,468,800→19,468,800 |
| struct_roundtrip | 71→71; 612,352→612,352 | 71→71; 612,352→612,352 | 71→71; 612,352→612,352 |
| sum_array | 313→313; 7,581,952→7,581,952 | 313→313; 7,581,952→7,581,952 | 313→313; 7,581,952→7,581,952 |


### O5b validation on frozen source

Required compiler suites, `go test ./pkg/mir2/... ./pkg/hir/... ./pkg/c89/...
./pkg/nanz/... ./pkg/z80asm/... -count=1`: PASS. Final times: MIR2 137.987s,
HIR 83.552s, C89 123.345s, Nanz 213.945s, z80asm 0.076s and assembler
regressions 0.007s. Final TestCountedLoop* pipeline checks PASS;
TestZ80LiveOutRestore* direct unit checks PASS. CI inventory validation
finds 833 total pipeline entries, with both new tests classified in sorted fast.

Quick judges only: direct-call seeds 0–499, -j 4: 500/500 PASS; folded seeds
0–99, -j 4: 100/100 PASS. Each reports zero MIR/oracle mismatches,
Z80/MIR mismatches, assembly failures, compiler errors and oracle errors.
No larger fuzz campaign or assertion matrix was run for O5b.
Logs: /tmp/o5b-units-final.log, /tmp/o5b-focused-final.log,
/tmp/o5b-restore-final.log, /tmp/o5b-judge-direct-call.log and
/tmp/o5b-judge-folded.log. Compiler-source changes were frozen before this
validation; the report was updated as results arrived.


The requested complete pipeline run (`go test ./pkg/pipeline/ -count=1
-timeout 30m -v`) timed out after 1800.050s, exit 1. It completed 257
top-level tests with PASS and emitted no earlier FAIL lines. All five
TestCountedLoop* tests also passed inside this unfiltered run. The active
case was TestMemoryHostArenaAppendSelectorOriginal30/speed, running for
18 seconds when the package-wide 30-minute deadline expired. Remaining
tests were not completed; this is not reported as a green full suite.
Log: /tmp/o5b-pipeline-final.log.

The earlier expensive fixed-policy case also passes in this full run.
Its 0/speed variant passes alone on clean dda8fab65 in 85.961s (72.78s
inside the subtest), log /tmp/o5b-baseline-policy.log. A fresh exact-baseline
check of the known TestSeededPointerWalkMeasuredCaller assertion fails:
candidates 0/2 are 120327T/57B; candidates 1/3 are 144105T/150B, and its
requirement that candidate 1 improve on 0 is false. Log:
/tmp/o5b-baseline-seeded.log. The final full run timed out before reaching
that test; its baseline failure is a separate observation, not the cause
of this timeout.


The active-at-timeout case was then run alone on both frozen O5b and clean
origin/main with `-run '^TestMemoryHostArenaAppendSelectorOriginal30$/^speed$'
-count=1 -timeout 10m -v`. Both PASS: O5b 67.085s, baseline 66.512s, with
identical target T/byte measurements for all four candidate masks. Logs:
/tmp/o5b-isolated-final.log and /tmp/o5b-isolated-baseline.log. This rules out
a failure in that case when isolated; it does not turn the unfinished complete
suite into a pass.

Files touched by O5b:

- minzc/pkg/hir/counted_loop.go
- minzc/pkg/hir/counted_loop_test.go
- minzc/pkg/hir/lower.go
- minzc/pkg/mir2/z80codegen_live_out_restore_test.go
- minzc/pkg/pipeline/counted_loop_djnz_test.go
- minzc/pkg/pipeline/counted_loop_profitability_test.go
- minzc/pkg/pipeline/pipeline.go
- scripts/ci/pipeline_profiles.json
- reports/2026-10-21-o5-counted-loop-djnz.md

O5b final status: EXIT 1 solely because the requested complete pipeline gate
hit its 30-minute cumulative timeout. The probe performance/size requirements,
all 36 main-suite MinZ measurements, compiler/assembler suites, new focused
checks, CI inventory and bounded judges pass. No functional failure preceded
the timeout. No push; the follow-up is committed locally.
