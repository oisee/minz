# O2: MIR byte comparisons against constants

The MIR pass now accepts a word constant alongside a same-block `ext u8`.
Constants are interpreted as 16-bit bit patterns. Values in [0,255] become
unsigned byte comparisons, including signed predicates mapped to unsigned ones.
Other values fold to a boolean using the original signed/unsigned predicate.
Both operand orders are supported. Original constants and extensions remain
available to their other consumers. No codegen changes were made.

Block parameters carrying word values are skipped: the existing pass and shape
facts do not provide a zero-extension proof for them. Missing comparison widths,
raw flag observations, hard constraints and non-unique definitions are refused.

Coverage includes all 256 byte inputs, ten predicates, both operand orders,
signed and unsigned word types, and 20 constants around byte/sign/word
boundaries. Tests also check emitted MIR shape, source positions, refusal cases
and preservation of shared operands. The C89 native test covers all byte inputs,
eleven constants, both orders and default/Grace compilation on the Z80 emulator.

Baseline: `origin/main` at `00600b28d`, detached worktree
`/mnt/ssd-510/minz/tmp/o2-before`. Main suite only; heldout was not run.
Artifacts: `/mnt/ssd-510/minz/bench/o2-before` and
`/mnt/ssd-510/minz/bench/o2-after`. Every MinZ measurement passed; no profile
regressed in bytes or T-states. Bytes below use the harness's `ProgramBytes`;
T is `TotalT` over the frozen workload.

| Case | Default bytes; T | Size bytes; T | Grace bytes; T |
|---|---:|---:|---:|
| abs_diff | 27→27; 220,904→220,904 | 27→27; 220,904→220,904 | 27→27; 220,904→220,904 |
| bitwise8reg | 199→199; 1,808,384→1,808,384 | 199→199; 1,808,384→1,808,384 | 183→183; 1,677,312→1,677,312 |
| copy_bytes | 182→182; 301,600→301,600 | 182→182; 301,600→301,600 | 182→182; 301,600→301,600 |
| crc8 | 399→373; 367,390,393→273,876,793 | 399→373; 367,390,393→273,876,793 | 391→365; 359,221,433→265,707,833 |
| fat12_decode | 167→167; 32,000→32,000 | 167→167; 32,000→32,000 | 167→167; 32,000→32,000 |
| gcd | 74→57; 5,109,358→4,953,966 | 74→57; 5,109,358→4,953,966 | 68→51; 3,222,405→3,067,013 |
| memset_fill | 304→304; 25,911,040→25,911,040 | 304→304; 25,911,040→25,911,040 | 304→304; 25,911,040→25,911,040 |
| mul8x8 | 75→55; 2,836,992→1,485,312 | 75→55; 2,836,992→1,485,312 | 75→55; 2,836,992→1,485,312 |
| reverse_loop11 | 592→592; 47,729,280→47,729,280 | 592→592; 47,729,280→47,729,280 | 592→592; 47,729,280→47,729,280 |
| strlen_bytes | 294→293; 25,150,080→21,607,040 | 294→293; 25,150,080→21,607,040 | 294→293; 25,150,080→21,607,040 |
| struct_roundtrip | 85→85; 935,936→935,936 | 85→85; 935,936→935,936 | 85→85; 935,936→935,936 |
| sum_array | 313→313; 7,581,952→7,581,952 | 313→313; 7,581,952→7,581,952 | 313→313; 7,581,952→7,581,952 |

Failing-first: `TestNarrowByteCompareConstants` was run against detached
`origin/main`, then repeated on the final fixture with the implementation diff
saved and reversed using `git apply -R`. All 800 predicate/type/constant/order
subcases failed with "comparison was not narrowed or folded" (exit 1).
Restoring the patch made `TestNarrowByteCompareConstant*` pass (exit 0),
including all 204,800 byte-input combinations and the refusal cases.
Logs: `/mnt/ssd-510/minz/tmp/o2-failing-first.log` and `o2-focused.log`.

The required MIR2/HIR/C89/Nanz/Z80asm package command passed, including the
native sweep. Log: `/mnt/ssd-510/minz/tmp/o2-unit.log`.

The quick baseline/candidate assertion matrix includes casts, struct promotion,
signed arithmetic, promoted arithmetic and signed literals across seven C/Nanz
files, with negative controls enabled, both Z80 and MIR2. Both checks exited 0;
there were no known failures or inventory changes. The initial full default
matrix (1,731 units per backend) was stopped as too expensive for the quick gate;
the full matrix remains for the manager. Quick report:
`/mnt/ssd-510/minz/tmp/o2-assert-matrix-quick.json`.

Quick matrix counts: 167/167 assertions pass on both baseline and candidate,
for each backend; 167 negative controls per backend, zero unexpected passes,
zero listing errors, zero changed/new failures.

Required fuzz judges passed with `-j 4` and `--no-reduce`:
- Direct-call seeds 0–1999: 2,000/2,000 pass.
- Folded seeds 0–299: 300/300 pass.
- Every mismatch, assembly/compiler error and oracle-error counter is zero.

Artifacts: `/mnt/ssd-510/minz/tmp/o2-fuzz-direct/results.json` and
`/mnt/ssd-510/minz/tmp/o2-fuzz-folded/results.json`.

The initial full pipeline command, restricted to one worker while other checks
ran, hit its 30-minute total timeout. It reported no individual assertion failure;
the active test was `TestMemoryHostAppendCopyTargetDomain` (41 seconds elapsed).
That entire test subsequently passed in isolation on both origin/main and the
candidate (candidate: 35.206 seconds). The optional broad baseline diagnostic
was stopped to free the workers for a full candidate rerun. Logs:
`/mnt/ssd-510/minz/tmp/o2-pipeline.log`, `o2-main-timeout-case.log`, and
`o2-candidate-timeout-case.log`.

The four-worker serial rerun was stopped after 268 completed top-level tests
without assertion failures, because the full 824-test package is largely serial.
The exact remaining inventory was divided into four groups of 139 tests, each
using `GOMAXPROCS=1`, `GOFLAGS=-p=1`, `go test ./pkg/pipeline/ -run '^(...)$'
`-count=1 -timeout 30m -v`. No test domains were reduced. The runner checks that
the completed prefix plus all groups accounts for all 824 names in the CI
inventory. Manifest and results:
`/mnt/ssd-510/minz/tmp/o2-pipeline-inventory.json`; runner:
`/mnt/ssd-510/minz/tmp/o2-finish-pipeline.py`.

`TestSeededPointerWalkMeasuredCaller` fails with identical costs on unchanged
origin/main and this candidate: masks 0/2 cost 120,327 T / 57 B; masks 1/3 cost
144,573 T / 150 B. The existing test requires mask 1 to improve speed despite
these measurements. Its independent result/access checks passed before that
performance assertion. This is a pre-existing failure; the test is unchanged.
Baseline reproduction: `/mnt/ssd-510/minz/tmp/o2-main-seeded-pointer.log`.

The first set of remainder groups reached their 30-minute group budgets with
656 names accounted for: 602 passed, 53 skipped under their normal prerequisites,
and the one reproduced baseline failure. The other 168 names were repartitioned
into four groups of 42, preserving every completed result. The second manifest
is `/mnt/ssd-510/minz/tmp/o2-pipeline-round2.json`; its runner is
`o2-resume-pipeline.py`. This avoids dropping slow tests or repeating already
completed domains.

Final complete pipeline inventory: **824/824 names accounted for: 739 PASS,
84 normal prerequisite SKIP, 1 FAIL**. The only failure is the identical
origin/main `TestSeededPointerWalkMeasuredCaller` expectation described above.
All four follow-up groups exited 0; no previously unaccounted test failed.
There are no missing names in `o2-pipeline-round2.json`. The overall pipeline
gate remains non-green because of the pre-existing failure, and the prescribed
monolithic command also exceeded its total timeout. Neither is reported as a
passing monolithic run.

The required unit-package command was repeated on the final source with two
jobs while the last two pipeline processes ran. MIR2, HIR, C89, Nanz, Z80asm and
its regression subpackage all passed (exit 0). Final log:
`/mnt/ssd-510/minz/tmp/o2-unit-final.log`.

The final main-suite benchmark rerun also exited 0 and reproduced the table
exactly: all 36 MinZ case/profile measurements PASS; no byte or T regression.
No production code changed after the judge binaries and final benchmarks.

Final disposition: the MIR change and its focused/native tests are green, with
zero fuzz/matrix failures and no benchmark regressions. **EXIT 1 / overall
NO-GO** records the existing pipeline failure and monolithic timeout; the
unrelated seeded-pointer test was not changed to force a green gate.
