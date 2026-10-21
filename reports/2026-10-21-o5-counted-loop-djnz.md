# O5: HIR counted loops to DJNZ (2026-10-21)

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
