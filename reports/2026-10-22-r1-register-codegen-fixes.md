# R1: dead edge arguments, byte steps, word multiply saves, conversion identities

Baseline: `origin/main` = `213080842b7adfffce828a2175f99bf7c79cf3cb`.
Only the frozen main benchmark suite was used. Neither heldout nor assert_matrix
was run. Commands use nice 19, the shared SSD TMPDIR/GOCACHE, four jobs, and a
load gate that checks uptime and sleeps 120 seconds while load exceeds 12.

## P1

`DeadBlockArgElim` recounts uses after each removal round until no parameter
can be removed. Entry parameters and the implicit DJNZ counter retain the
existing protections. Both pipeline entry points repeat elimination after
return CFG rewrites; the Steps entry point does this for Go and Grace.

Failing-first: `TestDeadBlockArgElimForwardedExitFixpoint` on origin/main:
`head retains forwarded dead parameter: [{2 u8 general}]`.
The test checks both parameters disappear in one call, a second call is stable,
MIR verification succeeds, and the VM returns the original result.
`TestDeadForwardedLoopArgNative` checks fib modulo 65536 for 0, 1, 2, 10, 20,
and 255 iterations in the VM and Z80 emulator in all three profiles.

## P2

Byte add/sub by small constants can copy the left operand directly to a
different primary byte register, then increment/decrement it. Constant-load
suppression and emission share the same predicate. Arithmetic flags must be
unobserved according to the existing conservative proof: carry observers,
inline assembly, flag ABI returns/parameters, and fallible carry contracts
are excluded. DEC results also invalidate subtraction-comparison flag reuse:
DEC preserves carry and cannot replace SUB's unsigned comparison flags.
The optional pre-PBQP graph change is deliberately omitted.

Failing-first: `TestByteSmallStepDifferentHomes` on origin/main:
`small folded constant retains a register load` for add/sub by 1, 2, and 3.
The test checks emission and every byte input in both VM and Z80, including
wraparound. Additional tests cover observed flag contracts and an unsigned
comparison following DEC with either incoming carry value.

## P8

Word multiplication queries liveness using the original instruction identity
before any commutative staging copy. HL and BC saves include their byte aliases
and exclude the instruction's destination. Small constant recipes copy HL to
BC with two LD instructions instead of a stack transfer, including the second
copy in the multiply-by-six recipe. Necessary live saves remain intact.

Failing-first: `TestMul16DeadScratchPairs` on origin/main finds PUSH/POP with
no live scratch values, and stack-based HL-to-BC copies. The new test checks
dead scratch needs no saves and checks live HL/H/L/BC/B/C survive, with boundary
multiplicands on Z80. The accumulator loop also runs on the VM and Z80.

## P9

A local, uniquely defined widening extension followed by truncation back to
exactly the source width becomes a move of the source. Both zero and sign
extension preserve these low bits. Other extension users retain their wide
value; DSE removes an extension only when unused. Hard classes, redefinitions,
non-dominating local definitions, and different result widths are excluded.

Failing-first: `TestTruncExtensionIdentity` on origin/main retains the truncation.
The test checks the move and dead-extension removal and executes all byte
inputs for signed/unsigned source types and both extension operations. Refusal
tests cover widths, hard classes, redefinitions, and definition order.
`TestTruncExtensionCRCBitNative` checks all 256 CRC bit inputs on VM and Z80,
all three profiles, with no `LD H, 0` in assembly.

## Validation

Failing-first tests were copied without production changes into a detached
origin/main worktree at `/mnt/ssd-510/minz/tmp/r1-origin-main`; all four failed
for the intended shape reasons. Full output:
`/mnt/ssd-510/minz/bench/r1-logs/failing-first.log`.

P1's focused native C fixture (`fib`, direct function entry through RET,
bootstrap excluded): default and size `fib(10)` improve **3060 → 492 T**,
**68 → 16 bytes**; `fib(20)` improves **6060 → 942 T**. Grace is unchanged
at 442 T / 15 bytes for `fib(10)` and 842 T for `fib(20)`. The loop body is
`ADD HL, DE; INC C; EX DE, HL; JRS head`. The remaining profile difference is
in the return/compare layout, outside the loop arithmetic. All six requested
input counts pass VM and Z80 checks in all three profiles.

The P8 tests also contain a full forced-allocation accumulator loop: DE holds
s, A holds k, and HL/BC are dead scratch. It verifies no PUSH/POP and correct
VM/Z80 sums for 0, 1, 2, 10, 200, and 255 iterations. Existing multiply fixtures
now make asserted scratch values explicitly live after the instruction; their
preservation assertions remain intact.

P9's initial full unit run exposed a native-code bug in the existing C89 control
matrix: default nested_switch(4) returned 11 instead of 12; raw VM, Grace,
and size were correct. The first mask reused A's tracked copy of a different
SSA operand and overwrote the original A value, which remained live through
successor blocks. P9 therefore also preserves unrelated live accumulator
values in binary ALU fast paths that reuse A, and preserves a live A operand
in flag-OR lowering. Folded constants are excluded because they have no
resident register value. The existing liveness/scratch helper is reused;
the fold has no source/function-name conditions.

TestByteALUCopyAliasPreservesLiveAccumulator reproduces the bug with legal
MIR, fixed A/C locations, and a successor edge with no arguments. It failed
before repair for add/sub/and/or/xor and passes afterward for all 256 byte
inputs on both VM and Z80. The complete C89 control matrix passes all three
modes after repair. The C89 test now includes function/input names in assertion
diagnostics. Evidence: p9-unit-initial.log, p9-control-debug.log,
p9-alias-failing.log, p9-alias-fixed.log, and p9-control-fixed.log.


## Final validation

| Fix | Required unit packages | Selected pipeline + probes | Direct-call 0–499 | Folded 0–99 | Main MinZ cells |
|---|---|---|---:|---:|---:|
| P1 | PASS | PASS, 35/35 probes | 500/500 | 100/100 | 36/36 |
| P2 | PASS after fixture correction | PASS, 35/35 probes | 500/500 | 100/100 | 36/36 |
| P8 | PASS | PASS, 35/35 probes | 500/500 | 100/100 | 36/36 |
| P9 | PASS after accumulator repair | PASS, 35/35 probes | 500/500 | 100/100 | 36/36 |

Every judge run has zero MIR2/oracle mismatches, Z80/MIR2 mismatches, assembly
failures, compiler errors, and oracle errors. Each uses four jobs. There are
no byte/T regressions against either origin/main or the preceding fix in any
of the 36 main cells or 35 critic probes.

The required unit command was run with count=1:
    cd minzc && nice -n 19 go test ./pkg/mir2/... ./pkg/hir/... ./pkg/c89/... ./pkg/nanz/... ./pkg/z80asm/... -count=1

The six reported packages include pkg/z80asm/regression. P2's first run failed
only because the new assembler fixture omitted a global label; after adding
ra_fragment, full mir2 was rerun successfully. Its other packages passed with
the same production source. P9's initial C89 failure was repaired generically,
then the entire required unit command was rerun and passed. Final unit and
pipeline output is in p9-unit.log and p9-pipeline.log.

Pipeline execution was limited to the four new native tests, related
TestDemandedBytePromotionNative, and the two supplied critic harnesses. The
full pipeline package was never executed. The native CRC bit test checks all
256 inputs on VM and Z80 in default, size, and Grace, with no LD H,0.

The final explicit reverse-patch proof saves the production diff against
213080842, applies it in reverse, runs the focused regressions (FAIL), restores
it, then runs them again (PASS). It includes the extra accumulator-alias
regression and both additional multiply liveness/loop tests.
Evidence: all-production.patch, reversed-failing-first.log,
restored-focused.log, explicit-proof.log under
/mnt/ssd-510/minz/bench/r1-logs/.

The final pipeline inventory has 833 entries and exactly matches the manifest;
all four new pipeline tests are in the sorted fast list. Temporary critic
tests were removed and were never committed. gofmt and git diff --check pass.

## Native examples

| Fixture | Profiles | Before bytes → after | Before T → after |
|---|---|---:|---:|
| C fib(10), t outside loop | default, size | 68 → 16 | 3060 → 492 |
| C fib(10), t outside loop | Grace | 15 → 15 | 442 → 442 |
| C acc(255), s += k*5 | all three | 27 → 27 | 40577 → 37262 |

The ordinary C accumulator keeps s in HL and its bound in C; both remain live
across multiply, so both saves are necessary. It gains 13 T per iteration from
the direct HL-to-BC copy. The forced dead-scratch loop verifies no PUSH/POP
when HL/BC contain no live value. The Nanz c15 critic gains the additional dead
BC-save removal: 38807 → 30137 T. Its result is checked by the supplied harness.

## Reproduction and artifacts

Baseline worktree: /mnt/ssd-510/minz/tmp/r1-origin-main, detached at 213080842.
Benchmark roots: /mnt/ssd-510/minz/bench/r1-before and r1-{p1,p2,p8,p9}-after.
Logs: /mnt/ssd-510/minz/bench/r1-logs/.

Each benchmark invokes research/c89-benchmark/run.py with --sdcc-root /,
--suite main, and --jobs 4. Each fix was measured before its local commit;
toolchains.json therefore records the predecessor HEAD while the working
tree contains the fix listed here. The final source is the four local commits.
No push, heldout run, large fuzz campaign, or assert_matrix run was performed.

Tables use ProgramBytes (occupied program/storage bytes) and TotalT from the
frozen workload. Default and size happen to be identical in this suite.
Critic sizes are entire module bytes, not per-function bytes.

## Files touched

- minzc/pkg/c89/control_matrix_native_test.go
- minzc/pkg/mir2/constprop.go
- minzc/pkg/mir2/deadblockarg.go
- minzc/pkg/mir2/deadblockarg_fixpoint_test.go
- minzc/pkg/mir2/mul16_promoted_byte_spill_native_test.go
- minzc/pkg/mir2/mul16_small_constants_test.go
- minzc/pkg/mir2/narrow_byte_arithmetic.go
- minzc/pkg/mir2/trunc_ext_identity.go
- minzc/pkg/mir2/trunc_ext_identity_test.go
- minzc/pkg/mir2/z80codegen.go
- minzc/pkg/mir2/z80codegen_acc_alias_test.go
- minzc/pkg/mir2/z80codegen_alu.go
- minzc/pkg/mir2/z80codegen_inst.go
- minzc/pkg/mir2/z80codegen_mul16_liveness_test.go
- minzc/pkg/mir2/z80codegen_muldiv.go
- minzc/pkg/mir2/z80codegen_small_step_test.go
- minzc/pkg/pipeline/pipeline.go
- minzc/pkg/pipeline/ra_deadarg_native_test.go
- minzc/pkg/pipeline/ra_mul16_native_test.go
- minzc/pkg/pipeline/ra_small_step_native_test.go
- minzc/pkg/pipeline/trunc_ext_native_test.go
- reports/2026-10-22-r1-register-codegen-fixes.md
- scripts/ci/pipeline_profiles.json

## Per-fix benchmark and probe tables


### p1-after versus origin/main

| Case | Default bytes / T | Size bytes / T | Grace bytes / T |
|---|---:|---:|---:|
| abs_diff | 27→27 / 220,904→220,904 | 27→27 / 220,904→220,904 | 27→27 / 220,904→220,904 |
| bitwise8reg | 101→101 / 872,448→872,448 | 101→101 / 872,448→872,448 | 101→101 / 872,448→872,448 |
| copy_bytes | 182→182 / 301,600→301,600 | 182→182 / 301,600→301,600 | 182→182 / 301,600→301,600 |
| crc8 | 340→326 / 165,616,162→152,877,602 | 340→326 / 165,616,162→152,877,602 | 326→326 / 152,877,602→152,877,602 |
| fat12_decode | 167→167 / 30,336→30,336 | 167→167 / 30,336→30,336 | 167→167 / 30,336→30,336 |
| gcd | 57→57 / 4,953,966→4,953,966 | 57→57 / 4,953,966→4,953,966 | 51→51 / 3,067,013→3,067,013 |
| memset_fill | 299→299 / 18,403,840→18,403,840 | 299→299 / 18,403,840→18,403,840 | 299→299 / 18,403,840→18,403,840 |
| mul8x8 | 47→47 / 1,167,872→1,167,872 | 47→47 / 1,167,872→1,167,872 | 47→47 / 1,167,872→1,167,872 |
| reverse_loop11 | 561→561 / 23,644,160→23,644,160 | 561→561 / 23,644,160→23,644,160 | 561→561 / 23,644,160→23,644,160 |
| strlen_bytes | 293→293 / 19,468,800→19,468,800 | 293→293 / 19,468,800→19,468,800 | 293→293 / 19,468,800→19,468,800 |
| struct_roundtrip | 71→71 / 612,352→612,352 | 71→71 / 612,352→612,352 | 71→71 / 612,352→612,352 |
| sum_array | 313→313 / 7,581,952→7,581,952 | 313→313 / 7,581,952→7,581,952 | 313→313 / 7,581,952→7,581,952 |

### p2-after versus origin/main

| Case | Default bytes / T | Size bytes / T | Grace bytes / T |
|---|---:|---:|---:|
| abs_diff | 27→27 / 220,904→220,904 | 27→27 / 220,904→220,904 | 27→27 / 220,904→220,904 |
| bitwise8reg | 101→95 / 872,448→831,488 | 101→95 / 872,448→831,488 | 101→95 / 872,448→831,488 |
| copy_bytes | 182→181 / 301,600→289,120 | 182→181 / 301,600→289,120 | 182→181 / 301,600→289,120 |
| crc8 | 340→326 / 165,616,162→152,877,602 | 340→326 / 165,616,162→152,877,602 | 326→326 / 152,877,602→152,877,602 |
| fat12_decode | 167→167 / 30,336→30,336 | 167→167 / 30,336→30,336 | 167→167 / 30,336→30,336 |
| gcd | 57→57 / 4,953,966→4,953,966 | 57→57 / 4,953,966→4,953,966 | 51→51 / 3,067,013→3,067,013 |
| memset_fill | 299→299 / 18,403,840→18,403,840 | 299→299 / 18,403,840→18,403,840 | 299→299 / 18,403,840→18,403,840 |
| mul8x8 | 47→47 / 1,167,872→1,167,872 | 47→47 / 1,167,872→1,167,872 | 47→47 / 1,167,872→1,167,872 |
| reverse_loop11 | 561→561 / 23,644,160→23,644,160 | 561→561 / 23,644,160→23,644,160 | 561→561 / 23,644,160→23,644,160 |
| strlen_bytes | 293→293 / 19,468,800→19,468,800 | 293→293 / 19,468,800→19,468,800 | 293→293 / 19,468,800→19,468,800 |
| struct_roundtrip | 71→59 / 612,352→567,296 | 71→59 / 612,352→567,296 | 71→59 / 612,352→567,296 |
| sum_array | 313→313 / 7,581,952→7,581,952 | 313→313 / 7,581,952→7,581,952 | 313→313 / 7,581,952→7,581,952 |

### p8-after versus origin/main

| Case | Default bytes / T | Size bytes / T | Grace bytes / T |
|---|---:|---:|---:|
| abs_diff | 27→27 / 220,904→220,904 | 27→27 / 220,904→220,904 | 27→27 / 220,904→220,904 |
| bitwise8reg | 101→95 / 872,448→831,488 | 101→95 / 872,448→831,488 | 101→95 / 872,448→831,488 |
| copy_bytes | 182→181 / 301,600→289,120 | 182→181 / 301,600→289,120 | 182→181 / 301,600→289,120 |
| crc8 | 340→326 / 165,616,162→152,877,602 | 340→326 / 165,616,162→152,877,602 | 326→326 / 152,877,602→152,877,602 |
| fat12_decode | 167→167 / 30,336→30,336 | 167→167 / 30,336→30,336 | 167→167 / 30,336→30,336 |
| gcd | 57→57 / 4,953,966→4,953,966 | 57→57 / 4,953,966→4,953,966 | 51→51 / 3,067,013→3,067,013 |
| memset_fill | 299→299 / 18,403,840→18,403,840 | 299→299 / 18,403,840→18,403,840 | 299→299 / 18,403,840→18,403,840 |
| mul8x8 | 47→47 / 1,167,872→1,167,872 | 47→47 / 1,167,872→1,167,872 | 47→47 / 1,167,872→1,167,872 |
| reverse_loop11 | 561→561 / 23,644,160→23,644,160 | 561→561 / 23,644,160→23,644,160 | 561→561 / 23,644,160→23,644,160 |
| strlen_bytes | 293→293 / 19,468,800→19,468,800 | 293→293 / 19,468,800→19,468,800 | 293→293 / 19,468,800→19,468,800 |
| struct_roundtrip | 71→59 / 612,352→567,296 | 71→59 / 612,352→567,296 | 71→59 / 612,352→567,296 |
| sum_array | 313→313 / 7,581,952→7,581,952 | 313→313 / 7,581,952→7,581,952 | 313→313 / 7,581,952→7,581,952 |

### p9-after versus origin/main

| Case | Default bytes / T | Size bytes / T | Grace bytes / T |
|---|---:|---:|---:|
| abs_diff | 27→27 / 220,904→220,904 | 27→27 / 220,904→220,904 | 27→27 / 220,904→220,904 |
| bitwise8reg | 101→95 / 872,448→831,488 | 101→95 / 872,448→831,488 | 101→95 / 872,448→831,488 |
| copy_bytes | 182→181 / 301,600→289,120 | 182→181 / 301,600→289,120 | 182→181 / 301,600→289,120 |
| crc8 | 340→323 / 165,616,162→138,516,002 | 340→323 / 165,616,162→138,516,002 | 326→323 / 152,877,602→138,516,002 |
| fat12_decode | 167→167 / 30,336→30,336 | 167→167 / 30,336→30,336 | 167→167 / 30,336→30,336 |
| gcd | 57→57 / 4,953,966→4,953,966 | 57→57 / 4,953,966→4,953,966 | 51→51 / 3,067,013→3,067,013 |
| memset_fill | 299→299 / 18,403,840→18,403,840 | 299→299 / 18,403,840→18,403,840 | 299→299 / 18,403,840→18,403,840 |
| mul8x8 | 47→45 / 1,167,872→1,096,192 | 47→45 / 1,167,872→1,096,192 | 47→45 / 1,167,872→1,096,192 |
| reverse_loop11 | 561→561 / 23,644,160→23,644,160 | 561→561 / 23,644,160→23,644,160 | 561→561 / 23,644,160→23,644,160 |
| strlen_bytes | 293→293 / 19,468,800→19,468,800 | 293→293 / 19,468,800→19,468,800 | 293→293 / 19,468,800→19,468,800 |
| struct_roundtrip | 71→59 / 612,352→567,296 | 71→59 / 612,352→567,296 | 71→59 / 612,352→567,296 |
| sum_array | 313→313 / 7,581,952→7,581,952 | 313→313 / 7,581,952→7,581,952 | 313→313 / 7,581,952→7,581,952 |

### Critic probes (T; module bytes in parentheses)

| Probe | Before | P1 | P2 | P8 | P9 |
|---|---:|---:|---:|---:|---:|
| nanz c1 | 496 (642) | 496 (642) | 440 (615) | 440 (613) | 440 (613) |
| nanz c2 | 11,792 (642) | 11,792 (642) | 11,792 (615) | 11,792 (613) | 11,792 (613) |
| nanz c3 | 134,943 (642) | 134,943 (642) | 133,413 (615) | 133,413 (613) | 133,413 (613) |
| nanz c4 | 61,214 (642) | 61,214 (642) | 59,014 (615) | 59,014 (613) | 59,014 (613) |
| nanz c5 | 21,543 (642) | 21,543 (642) | 15,911 (615) | 15,911 (613) | 15,911 (613) |
| nanz c6 | 35,952 (642) | 35,952 (642) | 35,952 (615) | 35,952 (613) | 35,952 (613) |
| nanz c6b | 35,789 (642) | 35,789 (642) | 35,789 (615) | 35,789 (613) | 35,789 (613) |
| nanz c7 | 458 (642) | 458 (642) | 458 (615) | 458 (613) | 458 (613) |
| nanz c8 | 649 (642) | 649 (642) | 649 (615) | 649 (613) | 649 (613) |
| nanz c9 | 5,289 (642) | 5,289 (642) | 5,289 (615) | 4,639 (613) | 4,639 (613) |
| nanz c10 | 16,131 (642) | 16,131 (642) | 16,131 (615) | 16,131 (613) | 16,131 (613) |
| nanz c11 | 949 (642) | 949 (642) | 949 (615) | 949 (613) | 949 (613) |
| nanz c12 | 133 (642) | 133 (642) | 133 (615) | 133 (613) | 133 (613) |
| nanz c13 | 649 (642) | 649 (642) | 579 (615) | 579 (613) | 579 (613) |
| nanz c14 | 587 (642) | 587 (642) | 587 (615) | 587 (613) | 587 (613) |
| nanz c15 | 38,807 (642) | 38,807 (642) | 38,807 (615) | 30,137 (613) | 30,137 (613) |
| nanz c16 | 104,068 (642) | 104,068 (642) | 104,068 (615) | 104,068 (613) | 104,068 (613) |
| nanz c17 | 4,364 (642) | 4,364 (642) | 3,374 (615) | 3,374 (613) | 3,374 (613) |
| nanz c18 | 2,952 (642) | 2,952 (642) | 2,852 (615) | 2,852 (613) | 2,852 (613) |
| c f1 | 689 (375) | 689 (375) | 619 (365) | 619 (365) | 619 (362) |
| c f2 | 12,900 (375) | 12,900 (375) | 12,900 (365) | 12,900 (365) | 12,900 (362) |
| c f3 | 789 (375) | 789 (375) | 789 (365) | 789 (365) | 701 (362) |
| c f4 | 11,629 (375) | 11,629 (375) | 11,629 (365) | 11,629 (365) | 11,629 (362) |
| c f5 | 1,081 (375) | 1,081 (375) | 1,081 (365) | 1,081 (365) | 1,081 (362) |
| c f6 | 33,169 (375) | 33,169 (375) | 33,169 (365) | 33,169 (365) | 33,169 (362) |
| c f7 | 1,264 (375) | 1,264 (375) | 1,214 (365) | 1,214 (365) | 1,214 (362) |
| c f8 | 1,190 (375) | 1,190 (375) | 1,190 (365) | 1,190 (365) | 1,190 (362) |
| c f9 | 1,266 (375) | 1,266 (375) | 1,142 (365) | 1,142 (365) | 1,142 (362) |
| crit m8 | 53,541 (223) | 53,541 (223) | 53,541 (215) | 53,541 (215) | 53,541 (215) |
| crit rng | 9,190 (223) | 9,190 (223) | 9,190 (215) | 9,190 (215) | 9,190 (215) |
| crit cnt | 11,241 (223) | 11,241 (223) | 9,841 (215) | 9,841 (215) | 9,841 (215) |
| crit xr | 8,241 (223) | 8,241 (223) | 8,241 (215) | 8,241 (215) | 8,241 (215) |
| crit fib | 942 (223) | 942 (223) | 942 (215) | 942 (215) | 942 (215) |
| crit tri | 144,482 (223) | 144,482 (223) | 105,542 (215) | 105,542 (215) | 105,542 (215) |
| crit many | 19,724 (223) | 19,724 (223) | 19,724 (215) | 19,724 (215) | 19,724 (215) |

## Re-measured on the current base (2026-10-22)

The tables above were measured against 213080842, before O5 (#236). After the
rebase onto 66ecc77b4 the i5 gate (SDCC 4.6 matrix) shows, default profile:
bitwise8reg 101→95 B, 872,448→831,488 T; copy_bytes 182→181 B,
301,600→289,120 T; crc8 322→315 B, 110,291,362→94,778,402 T (−14%, now below
SDCC 4.6's 104,720,964 T); mul8x8 47→45 B, 1,167,872→1,096,192 T;
struct_roundtrip 71→59 B, 612,352→567,296 T. Heldout: checksum16
58,206,776→56,128,056 T, popcount8 171,847,680→168,892,160 T. No cell worse.
