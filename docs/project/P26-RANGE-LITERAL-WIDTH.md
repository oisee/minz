# P26: range induction references defaulted to u8

On origin/main `dda8fab65`, the Nanz loop `for i in 300..310 { s = s + i }`
returns 485 on Z80 instead of 3045. The same source passes MIR2 assertions.

## First incorrect representation

`mz --help` documents `--emit=hir`, `--emit=mir2-raw`, and `--emit=mir2`.
The pipeline test saves those same stages, plus assembly, through
`MINZ_P26_ARTIFACTS`. The bounds already have the correct literal types:

```text
for i in 300:u16..310:u16
  (s:u16) = ((s:u16) + (i:u8)):u16
```

`nanz.parseFor` parsed the body without registering the induction variable.
`parsePrimary` therefore assigned its default u8 type to an unknown name
(or used an enclosing same-named variable's type). HIR lowering bound the
actual induction value as u16, but trusted the body's u8 annotation when
widening the addition operand. Raw MIR2 already contains the contradiction:

```text
block @loop_head2(..., %r6: u16, ...):
...
%r9 = ext %r6 : u8 -> u16
```

That instruction survives optimized MIR2 unchanged. Thus no MIR narrowing
pass first loses the high byte. Z80 correctly implements the requested
zero extension of the low byte, producing sum(44..53). The VM's extension
implementation retains the input integer and masks to the destination width,
which explains why the malformed IR happens to pass the VM assertion.

## Fix and scope

The parser binds the induction variable while parsing the body and restores
any enclosing type afterward. The original shared HIR `RangeType` selected the wider bound
type, with unsigned comparison for equal-width mixed signedness (superseded by
the P26b rules below after the critic found signed-range regressions). The parser,
verifier and lowerer use it. Lowering explicitly widens both initial bounds:
previously `0..n` with n:u16 bound a byte SSA register as a word, while a wide
start with a byte end selected a byte comparison. Bounds are still evaluated
once, in their enclosing scope. Same-width conversions preserve bit patterns;
widening uses each source's signedness. This changes the range binding type,
not arithmetic optimizations or codegen, and uses no function-name matching.

## Regression evidence

`TestRangeLiteralWidthVMZ80` checks the body reference's HIR type, raw/final
MIR2 execution, and MIR2/Z80 assertions against independent expected values.
It covers wide literal bounds, 250..260, local and parameter bounds, 0..300,
wide-start empty ranges, signed negative ranges (including a byte literal
end), a byte control, and restoration of a shadowed outer byte. Nanz shapes
run default, size and Grace profiles; a C unsigned for-loop is a passing
frontend control using `for (unsigned i = 300; i < 310; i++)`. The test is in
the sorted fast CI inventory. The minimal source fixture is `minzc/pkg/pipeline/testdata/p26/wide.nanz`.

With the production patch reversed (`git diff > fix.patch; git apply -R`),
the final test fails on origin/main: wide and parameter loops return 485,
250..260 returns 1521, 0..300 returns 33586, and the signed loop returns 763
instead of -5. A signed start with a byte literal end also returns 0 in
both VMs instead of -5 because the old comparison used the end's unsigned
byte type. The empty `lo:u16 = 256` to `hi:u8 = 10` range returns 45 in
raw/final MIR2 instead of 0. The byte and C controls pass. After reapplying
the patch, the test passes in all three profiles.

Validation logs, stage dumps and the reversible patch are retained under
`/mnt/ssd-510/minz/tmp/p26-evidence/`. Benchmark artifacts are under
`/mnt/ssd-510/minz/bench/p26-{before,after}`; before uses a separate detached
origin/main worktree. Heavy commands use nice 19, SSD TMPDIR/GOCACHE,
GOMAXPROCS up to 4 and at most four jobs. Only the main benchmark suite and the
requested small judge seed ranges are used.

Completed package gate (`-count=1`, `GOFLAGS=-p=4`): mir2 225.110s,
hir 130.117s, c89 199.417s, nanz 233.007s, z80asm 0.130s and its regression
subpackage 0.011s, all PASS. After the verifier's final one-line binding
alignment, a dedicated HIR rerun also passed (76.011s). The full pipeline
run uses the final implementation.

## Main-suite benchmark

All 36 MinZ case/profile pairs PASS. Program bytes and total workload T-states
are unchanged in every case; there are no regressions. Cells show before→after.

| Case | Default bytes; T | Size bytes; T | Grace bytes; T |
|---|---:|---:|---:|
| abs_diff | 27→27; 220904→220904 | 27→27; 220904→220904 | 27→27; 220904→220904 |
| bitwise8reg | 101→101; 872448→872448 | 101→101; 872448→872448 | 101→101; 872448→872448 |
| copy_bytes | 182→182; 301600→301600 | 182→182; 301600→301600 | 182→182; 301600→301600 |
| crc8 | 340→340; 165616162→165616162 | 340→340; 165616162→165616162 | 326→326; 152877602→152877602 |
| fat12_decode | 167→167; 30336→30336 | 167→167; 30336→30336 | 167→167; 30336→30336 |
| gcd | 57→57; 4953966→4953966 | 57→57; 4953966→4953966 | 51→51; 3067013→3067013 |
| memset_fill | 299→299; 18403840→18403840 | 299→299; 18403840→18403840 | 299→299; 18403840→18403840 |
| mul8x8 | 47→47; 1167872→1167872 | 47→47; 1167872→1167872 | 47→47; 1167872→1167872 |
| reverse_loop11 | 587→587; 39732480→39732480 | 587→587; 39732480→39732480 | 587→587; 39732480→39732480 |
| strlen_bytes | 293→293; 19468800→19468800 | 293→293; 19468800→19468800 | 293→293; 19468800→19468800 |
| struct_roundtrip | 71→71; 612352→612352 | 71→71; 612352→612352 | 71→71; 612352→612352 |
| sum_array | 313→313; 7581952→7581952 | 313→313; 7581952→7581952 | 313→313; 7581952→7581952 |

Quick judges (final compiler): direct-call seeds 0–499, `-j 4`: **500/500**;
folded seeds 0–99, `-j 4`: **100/100**. Both report zero MIR2/oracle,
Z80/MIR2, assembly, compiler and oracle errors. No larger campaign was run.

The requested single-process full pipeline command reached its global 30-minute
timeout (1800.018s), with no assertion failures reported. At timeout,
`TestMemoryHostPolicySelectorLateMutationAndSharing` had been active for
2m29s; the stack was in the existing automatic profile compatibility helper.

The origin/main comparison (`dda8fab65`, same runtime GOMAXPROCS=4,
`go test ./pkg/pipeline/ -count=1 -timeout 30m -v`, compile jobs limited to 1)
also reached the global timeout (1800.027s). It had completed 252 passing and
13 skipped top-level tests, with no assertion failures; its active test was
`TestMemoryHostArenaAppendSelectorOriginal30/size`. Thus a 30-minute aggregate
pipeline timeout is also present without this patch.

A supplemental eight-shard inventory attempt (all 829 entries, no `-short`,
three workers alongside the one baseline job) completed shard 1 successfully:
104 entries, 96 PASS and 8 SKIP. Shards 2 and 3 reached their 30-minute timeouts.
Remaining workers were stopped when the baseline comparison budget ended.
Across the partial shard logs, 240 top-level tests PASS and 17 SKIP, with no
assertion failures. Full pipeline coverage remains incomplete on this host;
this is an **EXIT 1** gate result, despite the focused regression, compiler
package tests, judges and benchmark being green. Logs and the partial inventory
summary are retained in `pipeline-shards/` under the evidence directory.

Final failing-first logs are `failing-first-final.log` (production patch
reversed, exit 1) and `focused-final.log` (patch reapplied, PASS). Both execute
the final test source, including the 256..10 empty range and C loop declaration.

## P26b critic corrections (origin/main 213080842)

The original P26 commit incorrectly let a literal's provisional u8 type choose
unsigned comparisons for `0..n` with n:i8. It also chose u16 for a typed u16/i16
pair, losing the historical signed end comparison. Both execute an empty range
as 255 or 65535 iterations when n is -1.

`RangeType` now treats integer literal bounds as values. One literal adopts the
other bound's type when representable; otherwise it widens with that bound's
signedness through the supported 8/16/24/32-bit integer types. Two literals
choose the smallest supported type holding both values (signed when either is
negative). Typed mixed bounds use the smallest signed type holding both domains:
u8/i8 uses i16, u16/i16 uses i24, and u24/i24 uses i32. Thus a u16 lower bound of
zero and an i16 upper bound of -1 still produce an empty range, preserving the
correct origin/main result without a lossy i16 conversion. Typed pairs involving
u32 and a signed bound have no supported common type holding both domains:
these deliberately retain origin/main's **end-bound type**, including its
bit-pattern interpretation of out-of-domain values. This is a compatibility
fallback, not a claim of lossless promotion. A literal which cannot fit any
supported type of the required signedness is rejected with a
`range literal ... does not fit ...` diagnostic.

The parser retypes literal bounds before binding the loop variable, keeping
parser, verifier and lowerer consistent. An explicit `for i: T` converts both
bounds to T and binds the body at T, including explicit truncation. No arithmetic
optimization or benchmark-name match is involved.

The five critic repros were copied without modifying their originals to
`/mnt/ssd-510/minz/tmp/p26b-evidence/`. Findings 1, 2, 3 and 5 are fixed and
covered by MIR2 and Z80 assertions under all three profiles. Additional cases
cover mixed byte/word bounds, signed literal widening to i16/i24, signed literal pairs, explicit
narrowing, and diagnostics for unrepresentable literals. All are subtests of the
existing `TestRangeLiteralWidthVMZ80`, already in the fast CI inventory.

Finding 4 is a separate range-source fusion follow-up. `RangeSourceExpr.ExprTy`
is hard-coded u8, but changing that alone is insufficient: `lowerRangeLoop`
also hard-codes its count parameter and zero comparison to u8/ClassCounter and
uses DJNZ. It computes `Hi-Lo` for a forward range but exposes the remaining
count as its element, without applying the lower-bound offset; 300..310 thus
exposes 10..1 and sums to 55. Callback parameter inference and count/element
lowering must be corrected together, with explicit tests for source direction
and filter/fold semantics. This patch leaves that path unchanged and records the exact
300..310 expected-3045 reproducer as skipped subtest
`range_source_wide_followup`, with both MIR2 and Z80 assertions ready to enable.

Failing-first evidence: `failing-origin-main-final.log` runs the final test against detached
origin/main 213080842 and fails (including i8 start and explicit word cases).
`failing-p26-head-final.log` runs the final tests after reversing the P26b production
patch with `git apply -R` and fails: negative ends produce 255/65535, signed i8
start produces 0 instead of -5, explicit u16 produces 1240 instead of 2520.
`production-final.patch` was reapplied afterward. `focused-final.log` records the passing
implementation. Evidence is in `/mnt/ssd-510/minz/tmp/p26b-evidence/`.

The `byte_shapes.go` check compiles literal `0..10`, `0..n` with n:u8, and
nested byte loops using default, size and Grace options on both worktrees.
`diff -qr byte-before byte-after` is empty: all nine assembly files are identical.

The main-suite benchmarks were rerun at
`/mnt/ssd-510/minz/bench/p26b-{before,after}`. Before uses a separate detached
origin/main worktree under the SSD tmp directory. All 36 MinZ case/profile pairs
PASS with identical bytes and T-states. No heldout suite was run.

| Case | Default bytes; T | Size bytes; T | Grace bytes; T |
|---|---:|---:|---:|
| abs_diff | 27→27; 220904→220904 | 27→27; 220904→220904 | 27→27; 220904→220904 |
| bitwise8reg | 101→101; 872448→872448 | 101→101; 872448→872448 | 101→101; 872448→872448 |
| copy_bytes | 182→182; 301600→301600 | 182→182; 301600→301600 | 182→182; 301600→301600 |
| crc8 | 340→340; 165616162→165616162 | 340→340; 165616162→165616162 | 326→326; 152877602→152877602 |
| fat12_decode | 167→167; 30336→30336 | 167→167; 30336→30336 | 167→167; 30336→30336 |
| gcd | 57→57; 4953966→4953966 | 57→57; 4953966→4953966 | 51→51; 3067013→3067013 |
| memset_fill | 299→299; 18403840→18403840 | 299→299; 18403840→18403840 | 299→299; 18403840→18403840 |
| mul8x8 | 47→47; 1167872→1167872 | 47→47; 1167872→1167872 | 47→47; 1167872→1167872 |
| reverse_loop11 | 561→561; 23644160→23644160 | 561→561; 23644160→23644160 | 561→561; 23644160→23644160 |
| strlen_bytes | 293→293; 19468800→19468800 | 293→293; 19468800→19468800 | 293→293; 19468800→19468800 |
| struct_roundtrip | 71→71; 612352→612352 | 71→71; 612352→612352 | 71→71; 612352→612352 |
| sum_array | 313→313; 7581952→7581952 | 313→313; 7581952→7581952 | 313→313; 7581952→7581952 |

The requested full pipeline command (`-count=1 -timeout 30m -v`) reached its
aggregate timeout on both this branch (1800.020s) and detached origin/main
213080842 (1800.026s). Each completed 247 passing and 12 skipped top-level tests,
with no assertion failures. Both were executing
`TestMemoryHostPolicySelectorLateMutationAndSharing/24` at timeout; the parent
test had been running for 1m55s on the branch and 1m49s on main. The stacks are
in the existing automatic profile compatibility compilation helper. Both runs
used GOMAXPROCS=2 and GOFLAGS=-p=2, for at most four concurrent jobs in total.
Logs are `pipeline.log` and `pipeline-origin-main.log` in the evidence directory.
This reproduces the aggregate timeout without P26/P26b; full pipeline coverage
remains incomplete, so the task gate is **EXIT 1** despite no observed semantic
or benchmark regression.

Final-source validation: the final 24/32-bit widening review followed the
aggregate pipeline runs above. The focused test and full compiler package gate
were rerun after that review; aggregate pipeline coverage remains partial.
`focused-final.log` passes the critic corrections and additional i24 boundary
case under default, size and Grace, with both assertion engines; only the
explicitly documented range-source follow-up is skipped. The final
`failing-p26-head-final.log` and `failing-origin-main-final.log` both exit 1
with the exact final test source; `production-final.patch` is the reversible
production diff used for that check.

Final package gate (`unit-final.log`, -count=1): mir2 PASS 152.835s, hir PASS
71.795s, c89 PASS 117.106s, nanz PASS 156.879s, z80asm PASS 0.105s and
z80asm/regression PASS 0.020s. Final quick judges (`direct-final.log`,
`folded-final.log`) are 500/500 seeds 0–499 and 100/100 seeds 0–99, respectively,
with -j 4 and zero mismatch/compiler/assembler/oracle errors. Final benchmark
JSON was rechecked: all 36 cells remain identical to the table. A final
byte-assembly comparison also returns an empty diff for all nine files.
No heldout benchmark suite, assert_matrix, larger seed range or push was run.
