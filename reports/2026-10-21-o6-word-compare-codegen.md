# O6: word comparison codegen (2026-10-21)

## Change and legality

Signed i16 comparisons with zero, consumed only by the immediately following
`br_if`, test bit 7 of the pair's high byte. HL, DE and BC use BIT directly;
IX/IY load their high byte into preserved A because Z80 cannot BIT an index half.
`> 0` and `<= 0` additionally OR the high/low bytes on the nonnegative path.
The negative path sets Z, encoding a nonpositive input without subtracting.

Word equality/inequality with zero uses `LD A,hi; OR lo` for all five pairs,
for both branches and boolean values. Existing accumulator preservation protects
live A, including stack restoration when no scratch register is free.
Zero constant loads are suppressed only when every use matches a proven
immediate/test lowering; any other use retains the load.

Other signed i16 comparisons consumed only by the following `br_if` branch on
S xor V (including Z for greater/less-or-equal) without SCF/CCF. Operand swapping
and edge-copy trampolines retain their existing meaning. A comparison qualifies
only when it is the block's last instruction and its sole use in the function
is the branch condition. Value consumers and mixed consumers retain the existing
canonical boolean path. No benchmark names participate in matching.

`strlen_bytes` already compares a byte against zero on origin/main at this
baseline: its hot path is `LD A,C; AND A`, not a word SBC. It is unchanged.
`reverse_loop11` uses `BIT 7,D` for its signed loop bound.

## Tests and evidence

Baseline: origin/main `dda8fab65ad86389210b4814f2670818cb63f1bb`, checked out
separately at `/mnt/ssd-510/minz/tmp/O6-before`. The final test file was copied
into that otherwise unchanged compiler checkout for failing-first execution.

`TestWordCompareBranchFlags`: 120 subtests across all five pairs, six signed
predicates, zero/nonzero RHS, branch/value consumers, and signed overflow
boundaries. All pairs and A are live, forcing the exhausted-register preservation
path. It checks shape, assembles, executes on Z80, and checks the boolean, A,
and every pair. Origin/main fails 60 subtests (SBC zero tests and signed carry
materialization); the changed compiler passes all 120.

`TestWordCompareBranchVMZ80`: 1,270 assertions per default/Grace mode across
signed boundaries, signed zero bounds, signed boolean values, and unsigned zero
equality. Raw MIR2, optimized MIR2 and Z80 all pass. Added to the sorted pipeline
fast inventory.

Final validation:

- Required compiler package command: PASS, exit 0 (MIR2, HIR, C89, Nanz,
  Z80 assembler and assembler regression package).
- Direct-call seeds 0–499: PASS 500/500, exit 0; all error/mismatch counts zero.
- Folded seeds 0–99: PASS 100/100, exit 0; all error/mismatch counts zero.
- Complete pipeline command: FAIL, exit 1, cumulative 30-minute timeout
  (`1800.020s`), with no reported assertion failures. At timeout,
  `TestMemoryHostPolicySelectorLateMutationAndSharing` had run 19 seconds
  (subtest `/12` had run 4 seconds). The stack was in
  `hir.buildAutomaticProfileContextCompatibility`, hashing the running test
  executable with `io.Copy`, not in comparison lowering or Z80 execution.
- Independent comparison of the active timeout test: PASS on origin/main
  (`189.729s`) and this branch (`171.12s` for the test), with no assertion
  failures. This confirms that the active test passes independently; it is not
  evidence that either complete suite fits the cumulative timeout.
- Final focused pipeline run: PASS, exit 0 (`191.211s` total), containing both
  that test and `TestWordCompareBranchVMZ80`. The new regression passed all
  1,270 assertions in each mode on the final implementation (`20.07s`).

The complete pipeline gate is not green: the timeout leaves later tests
unverified. The focused baseline comparison cannot establish that the entire
origin/main suite also exceeds 30 minutes. No unrelated runtime/profile code was
changed to bypass the required check.

All heavy commands use `nice -n 19`, `TMPDIR=/mnt/ssd-510/minz/tmp`,
`GOCACHE=/mnt/ssd-510/minz/gocache`, and at most four build/jobs workers.
Commands for final validation run sequentially:

```sh
go test ./pkg/mir2/... ./pkg/hir/... ./pkg/c89/... ./pkg/nanz/... ./pkg/z80asm/... -count=1
go test ./pkg/pipeline/ -count=1 -timeout 30m
python3 scripts/fuzz_diff.py --mz <final-mz> --mode direct-call --seed 0 --count 500 -j 4 --output <direct>
python3 scripts/fuzz_diff.py --mz <final-mz> --mode folded --seed 0 --count 100 -j 4 --output <folded>
```

Logs and status files: `/mnt/ssd-510/minz/bench/O6-logs/`.
No heldout suite, larger fuzz campaign, or assert_matrix was run.

## Main-suite before/after

Both runs use `research/c89-benchmark/run.py --sdcc-root / --suite main --jobs 4`.
Before: `/mnt/ssd-510/minz/bench/O6-before`; after:
`/mnt/ssd-510/minz/bench/O6-after`. Bytes are the runner's `ProgramBytes`,
including global storage, and T is the full workload's `TotalT`.
All 36 MinZ case/profile combinations pass. None regresses in bytes or T.
`reverse_loop11` saves 26 bytes and 16,088,320 T-states (40.49%) per profile.

| Case | Profile | Bytes before → after | T before → after |
|---|---|---:|---:|
| abs_diff | default | 27 → 27 | 220,904 → 220,904 |
| abs_diff | size | 27 → 27 | 220,904 → 220,904 |
| abs_diff | grace | 27 → 27 | 220,904 → 220,904 |
| bitwise8reg | default | 101 → 101 | 872,448 → 872,448 |
| bitwise8reg | size | 101 → 101 | 872,448 → 872,448 |
| bitwise8reg | grace | 101 → 101 | 872,448 → 872,448 |
| copy_bytes | default | 182 → 182 | 301,600 → 301,600 |
| copy_bytes | size | 182 → 182 | 301,600 → 301,600 |
| copy_bytes | grace | 182 → 182 | 301,600 → 301,600 |
| crc8 | default | 340 → 340 | 165,616,162 → 165,616,162 |
| crc8 | size | 340 → 340 | 165,616,162 → 165,616,162 |
| crc8 | grace | 326 → 326 | 152,877,602 → 152,877,602 |
| fat12_decode | default | 167 → 167 | 30,336 → 30,336 |
| fat12_decode | size | 167 → 167 | 30,336 → 30,336 |
| fat12_decode | grace | 167 → 167 | 30,336 → 30,336 |
| gcd | default | 57 → 57 | 4,953,966 → 4,953,966 |
| gcd | size | 57 → 57 | 4,953,966 → 4,953,966 |
| gcd | grace | 51 → 51 | 3,067,013 → 3,067,013 |
| memset_fill | default | 299 → 299 | 18,403,840 → 18,403,840 |
| memset_fill | size | 299 → 299 | 18,403,840 → 18,403,840 |
| memset_fill | grace | 299 → 299 | 18,403,840 → 18,403,840 |
| mul8x8 | default | 47 → 47 | 1,167,872 → 1,167,872 |
| mul8x8 | size | 47 → 47 | 1,167,872 → 1,167,872 |
| mul8x8 | grace | 47 → 47 | 1,167,872 → 1,167,872 |
| reverse_loop11 | default | 587 → 561 | 39,732,480 → 23,644,160 |
| reverse_loop11 | size | 587 → 561 | 39,732,480 → 23,644,160 |
| reverse_loop11 | grace | 587 → 561 | 39,732,480 → 23,644,160 |
| strlen_bytes | default | 293 → 293 | 19,468,800 → 19,468,800 |
| strlen_bytes | size | 293 → 293 | 19,468,800 → 19,468,800 |
| strlen_bytes | grace | 293 → 293 | 19,468,800 → 19,468,800 |
| struct_roundtrip | default | 71 → 71 | 612,352 → 612,352 |
| struct_roundtrip | size | 71 → 71 | 612,352 → 612,352 |
| struct_roundtrip | grace | 71 → 71 | 612,352 → 612,352 |
| sum_array | default | 313 → 313 | 7,581,952 → 7,581,952 |
| sum_array | size | 313 → 313 | 7,581,952 → 7,581,952 |
| sum_array | grace | 313 → 313 | 7,581,952 → 7,581,952 |

## Final status

The implementation, focused tests, required compiler packages, bounded judges,
and all main-suite benchmark profiles pass. The required complete pipeline run
exhausted its cumulative 30-minute timeout; later tests remain unverified.
Final task status: EXIT 1, solely for the incomplete full pipeline gate.
