# P27: literal-left comparisons lose signedness on equal widths

`fun f(x:i16)->bool{return 32767 < x}` returned true for `x=-32768`
in both MIR2 VM and Z80 on origin/main (`213080842`). The critic's original
`k8.nanz` also keeps a computed byte live across the comparison; the
standalone bool return reproduces the defect without register pressure.

HIR `lowerCond` computes a common comparison width and selects the comparison
predicate. It intends to use the nonliteral peer's type when the left operand
is an integer literal. However, it selected the right type only for a strictly
larger width. Nanz parses 32767 as u16, so an i16 peer tied in width and lost
its signedness. The raw MIR already contained `cmp.ult` with no signed source
type. Both execution paths faithfully executed that incorrect predicate;
operand normalization and Z80 `normalizeSignedCmp` were downstream of the bug.
The byte sibling `127 < x:i8` has the same cause.

On a width tie with an integer literal on the left, use the right operand's
type for the comparison width/type. This supplies both the signed predicate
and the signed `SrcTy` consumed by the VM and code generator. A strictly wider
literal retains its existing width/type. Nonliteral-left comparison policy
is unchanged. C's frontend explicitly converts both operands to the usual
arithmetic conversion type before HIR lowering, so those conversions remain
authoritative, including unsigned int comparisons.

`TestLiteralLeftSignedCompareVMZ80` checks raw MIR predicates and signed source
width, raw and final MIR2 execution, and `via z80` assertions for all six
operators, boundary and negative constants, i8/i16, u8/u16, C signed char,
short/int and unsigned char/short. Each predicate appears as a returned bool
and as a branch; the original critic's live-byte case is retained. The test
is registered in the sorted pipeline `fast` inventory.

Validation results and the main-suite before/after table are recorded below.

## Failing-first evidence

With only `minzc/pkg/hir/lower.go` reversed using `git apply -R` (and then
restored), the final regression exits 1 on origin/main compiler code. Raw
MIR uses `ult` instead of `lt` and has no signed `SrcTy`. Independent VM and
Z80 assertions both report `compare(-32768)` returning 1 for `32767 < x`,
expected 0; the byte sibling reports the same error at `127 < -128`.
The original critic context returns 14 instead of 21.

The complete matrix has 480 subcases. Logs and the copied read-only critic
reproducer are under `/mnt/ssd-510/minz/tmp/p27-evidence/`. The baseline
benchmark uses a detached origin/main worktree at
`/mnt/ssd-510/minz/tmp/p27-before`.

The reversed-fix run reports 54 failing subcases: 36 expose wrong results
independently in raw VM and Z80, and 18 equality/inequality subcases expose
missing signed source metadata. The fixed matrix passed before the broader
suite; the final version independently selects Z80 assertions during production
compilation so VM failures cannot mask emulator failures.

## Package validation

With `TMPDIR=/mnt/ssd-510/minz/tmp`, `GOCACHE=/mnt/ssd-510/minz/gocache`,
`GOMAXPROCS=4`, `GOFLAGS=-p=4`, and commands under `nice -n 19`:

- Requested compiler package command: PASS. MIR2 174.981s, HIR 67.867s,
  C89 136.518s, Nanz 175.895s, Z80 assembler 0.164s and regression 0.012s.
- Final focused regression: PASS, exit 0, 53.956s. All 480 subcases,
  independent Z80 assertions, raw/final VM assertions, and the critic context pass.
- Prescribed full pipeline command: FAIL, exit 1, cumulative 30-minute
  timeout (1800.021s). No assertion failure was reported before timeout.
  The active test was `TestMemoryHostPolicySelectorUnobservedAndTies` (7s
  elapsed). Its stack was in `hir.buildAutomaticProfileContextCompatibility`,
  hashing the running test executable through `io.Copy` at
  `minzc/pkg/hir/profile_context.go:51`. This code is unchanged by P27.
  The monolithic suite is not green; tests after the timeout remain unverified.

## Bounded judges and main-suite benchmarks

The freshly built `mz` passed direct-call seeds 0–499 (500/500), signed
direct-call seeds 0–499 (500/500), and folded seeds 0–99 (100/100), all with
`-j 4 --no-reduce`. Every oracle mismatch, Z80/MIR2 mismatch, assembly,
compiler, and oracle error count is zero. No larger campaigns or assert matrix
were run.

Both benchmarks used `research/c89-benchmark/run.py --sdcc-root / --suite main
--jobs 4`, with outputs `/mnt/ssd-510/minz/bench/p27-before` and
`/mnt/ssd-510/minz/bench/p27-after`. Before is detached origin/main
`213080842`; after is the candidate working tree before its final local commit.
All 36 MinZ case/profile rows pass and are unchanged in initialized bytes,
program bytes, globals and T-states. The heldout suite was not run.

Each cell below is initialized **bytes before→after; total T-states
before→after**. Default and size agree on every case.

| Case | Default B; T | Size B; T | Grace B; T |
|---|---:|---:|---:|
| abs_diff | 27→27; 220,904→220,904 | 27→27; 220,904→220,904 | 27→27; 220,904→220,904 |
| bitwise8reg | 101→101; 872,448→872,448 | 101→101; 872,448→872,448 | 101→101; 872,448→872,448 |
| copy_bytes | 182→182; 301,600→301,600 | 182→182; 301,600→301,600 | 182→182; 301,600→301,600 |
| crc8 | 340→340; 165,616,162→165,616,162 | 340→340; 165,616,162→165,616,162 | 326→326; 152,877,602→152,877,602 |
| fat12_decode | 167→167; 30,336→30,336 | 167→167; 30,336→30,336 | 167→167; 30,336→30,336 |
| gcd | 57→57; 4,953,966→4,953,966 | 57→57; 4,953,966→4,953,966 | 51→51; 3,067,013→3,067,013 |
| memset_fill | 299→299; 18,403,840→18,403,840 | 299→299; 18,403,840→18,403,840 | 299→299; 18,403,840→18,403,840 |
| mul8x8 | 47→47; 1,167,872→1,167,872 | 47→47; 1,167,872→1,167,872 | 47→47; 1,167,872→1,167,872 |
| reverse_loop11 | 561→561; 23,644,160→23,644,160 | 561→561; 23,644,160→23,644,160 | 561→561; 23,644,160→23,644,160 |
| strlen_bytes | 293→293; 19,468,800→19,468,800 | 293→293; 19,468,800→19,468,800 | 293→293; 19,468,800→19,468,800 |
| struct_roundtrip | 71→71; 612,352→612,352 | 71→71; 612,352→612,352 | 71→71; 612,352→612,352 |
| sum_array | 313→313; 7,581,952→7,581,952 | 313→313; 7,581,952→7,581,952 | 313→313; 7,581,952→7,581,952 |

## Timeout comparison and gate status

The timeout-active `TestMemoryHostPolicySelectorUnobservedAndTies` passes
independently on unchanged origin/main (13.024s) and the candidate (14.974s),
both exit 0. Logs are `timeout-case-before.log` and `timeout-case-after.log`
in the evidence directory. This rules out an assertion failure in that case;
it does not establish that the entire origin/main suite fits the 30-minute
budget. Recent mainline O6 validation reports the same cumulative timeout and
executable-hashing stack class.

P27's focused regression, requested package tests, bounded judges, and all
main-suite benchmark profiles pass. The complete pipeline gate remains
incomplete because the monolithic command timed out, leaving later tests
unverified. Final status: **EXIT 1 for the full pipeline timeout**.
