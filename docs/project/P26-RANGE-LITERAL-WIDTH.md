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
any enclosing type afterward. A shared HIR `RangeType` selects the wider bound
type, with unsigned comparison for equal-width mixed signedness. The parser,
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
