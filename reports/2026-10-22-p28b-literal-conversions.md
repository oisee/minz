# P28b: integer destination conversions and literal proof templates

P28 selected a wider arithmetic domain for expressions such as `a:u8 xor -1`
and `a:i8 + 200`, but lowering did not consistently convert the resulting
register to a declared destination type. A call to a u8 parameter could therefore
receive 65498 instead of 218, and a u8 return could retain the same wide value.

`lowerExprAs` now reuses the integer conversion helper for both narrowing and
widening. Direct calls convert to the resolved callee parameter before ABI
placement; indirect and multi-result calls already use this helper. The Nanz
frontend inserts explicit integer argument conversions for typed indirect calls
after resolving forward expression types. This preserves the verifier's strict
consumed-callback ABI checks, including rejection of forged signature metadata. Single
returns, conditional return arms, multi-returns, and folded returns respect the
return contract. Assignment stores, struct field initialization, and runtime
array initialization explicitly narrow wide expressions. Existing static global
initializers already serialize the destination byte width; the regression
covers scalar, struct, and array globals as well. An arithmetic expression still
executes in its own selected domain before the destination conversion.

The append-copy and arena-peel prototype source templates now retain
`IntLitExpr.Untyped`, matching parsed Nanz. Their generated constants remain
typed. The peel prefix rebuilds its address expression with typed constants,
matching the independent proof's generated-prefix template. The audit of every
production `reflect.DeepEqual` comparison in `pkg/pipeline` and `pkg/hir` found
that other literal-bearing source templates already use the P28 source literal
constructors; the remaining typed templates describe generated HIR. Strict
metadata, value, type, statement-order, and origin comparisons remain intact.

`TestNanzLiteralNarrowingVMZ80` checks explicit 16-to-8 MIR conversions and runs
all 256 byte inputs, both operand orders, u8/i8 returns, and direct and indirect
calls through raw MIR2, optimized MIR2, and Z80. Additional cases cover global
initializers, struct fields, local/global array stores, conditional returns,
and multiple return values. The new test is in the sorted fast CI inventory.

Literal-only subexpression folding is a follow-up. Folding `0 - 2`, `100*100`,
`200+100`, and `~0` to exact untyped values needs a separate rule for retaining
exact values across operations that currently have machine-width wraparound.
This repair leaves that language change outside the two blocking fixes.

## Validation

The final integration base is `1815f5a35`: O5 #236, R1 #237, O7 #238, and the
Grace self-loop fix #239. The source was rebased cleanly after main advanced
during validation. The separate baseline worktree is
`/mnt/ssd-510/minz/tmp/p28b-o7-before`; production sources there are origin/main.
The final regression file is copied unchanged to the baseline and to the
original P28 worktree at `97ff0dad8`.

`TestNanzLiteralNarrowingVMZ80/u8/+/0` fails on final origin/main: the
valid typed-callback call is rejected with `callback invocation disagrees with
explicit signature; ABI adapter required`. The u16 intermediate `x + 300`
needs conversion to the declared u8 parameter. Original P28 also fails on the
nested typed-callback field-initializer case. The changed compiler passes.
The property checks explicit 16-to-8 MIR conversions and executes byte inputs
through arithmetic, bitwise operations, division/remainder, and shifts, both
operand orders where defined, narrow u8/i8 returns, and direct/indirect calls.
Undefined shifts and division cases are excluded by the independent model.

The related pipeline selection covers existing literal typing, signed widths,
folds, global aggregate initialization, boolean extension, and typed indirect
calls. It also includes the newly merged byte-scan, byte-fill, ordered-copy,
and Grace infinite-loop regressions; it passes in 45.323 seconds. The final
expanded narrow-context property passes separately in 8.724 seconds, using raw
MIR2, optimized MIR2, and native Z80 assertions. This expansion changed only
the test's operator/input table; compiler source remained fixed during the
full test and benchmark runs.
The CI inventory is complete, with 853
entries and the new test in the sorted fast profile.

The required unit command passes all packages on the final integration base:

```sh
cd minzc
nice -n 19 go test ./pkg/mir2/... ./pkg/hir/... ./pkg/c89/... ./pkg/nanz/... ./pkg/z80asm/... -count=1
```

Package times: mir2 107.364s, hir 58.782s, c89 136.136s, nanz 170.793s, z80asm
0.058s, z80asm/regression 0.006s. The unchanged HIR consumed-callback tests still
reject forged ABI signatures.

Both copied critic repros pass with the rebuilt final CLI. The unmodified
independent `gen28.py` produced 1,856 files: 1,216 valid cases and 640 expected
domain-error cases. All 640 rejections have the explicit-wide-cast diagnostic.
The raw campaign has 1,196 passing valid files and 20 native image overflows,
with no value mismatches. These are exactly the same 20 overflow filenames in
the critic's original P28 `res-new.txt`. They contain unasserted functions with
undefined giant constant shift counts, overflowing the native assertion image.
The critic supplied `g2` retry files omitting those functions. Each retry's
complete ordered assertion list and asserted `l`/`ln` function bodies were
checked byte-for-byte against the original generated file. All 20 retries pass.
Every generated defined assertion therefore passes, with zero wrong values
across the 1,216 valid cases. Raw failures and separate retries are retained;
compiler behavior was not changed to accommodate the campaign.

Direct-call fuzz seeds 0–499 pass 500/500 with four workers. Folded seeds 0–99
pass 100/100 with four workers. Both have zero MIR2/oracle mismatches,
Z80/MIR2 mismatches, assembly failures, compiler errors, and oracle errors.
No larger fuzz campaign or assert_matrix was run.

## Main-suite measurements

Both commands completed successfully, using the separate origin/main worktree
for before and the rebased branch for after:

```sh
nice -n 19 python3 research/c89-benchmark/run.py --out /mnt/ssd-510/minz/bench/p28b-final-before --sdcc-root / --suite main --jobs 4
nice -n 19 python3 research/c89-benchmark/run.py --out /mnt/ssd-510/minz/bench/p28b-final-after --sdcc-root / --suite main --jobs 4
```

All 36 MinZ cells pass, with identical ProgramBytes and TotalT. There is no byte
or T-state regression in default, size, or Grace. B/T below means
ProgramBytes/TotalT. No heldout suite was run.

| Case | Default B/T before → after | Size B/T before → after | Grace B/T before → after |
|---|---|---|---|
| abs_diff | 27/220,904 → 27/220,904 | 27/220,904 → 27/220,904 | 27/220,904 → 27/220,904 |
| bitwise8reg | 95/831,488 → 95/831,488 | 95/831,488 → 95/831,488 | 95/831,488 → 95/831,488 |
| copy_bytes | 172/109,332 → 172/109,332 | 172/109,332 → 172/109,332 | 172/109,332 → 172/109,332 |
| crc8 | 315/94,778,402 → 315/94,778,402 | 315/94,778,402 → 315/94,778,402 | 315/94,778,402 → 315/94,778,402 |
| fat12_decode | 167/30,336 → 167/30,336 | 167/30,336 → 167/30,336 | 167/30,336 → 167/30,336 |
| gcd | 57/4,953,966 → 57/4,953,966 | 57/4,953,966 → 57/4,953,966 | 51/3,067,013 → 51/3,067,013 |
| memset_fill | 297/3,588,390 → 297/3,588,390 | 297/3,588,390 → 297/3,588,390 | 297/3,588,390 → 297/3,588,390 |
| mul8x8 | 45/1,096,192 → 45/1,096,192 | 45/1,096,192 → 45/1,096,192 | 45/1,096,192 → 45/1,096,192 |
| reverse_loop11 | 561/23,644,160 → 561/23,644,160 | 561/23,644,160 → 561/23,644,160 | 561/23,644,160 → 561/23,644,160 |
| strlen_bytes | 283/3,574,400 → 283/3,574,400 | 283/3,574,400 → 283/3,574,400 | 283/3,574,400 → 283/3,574,400 |
| struct_roundtrip | 59/567,296 → 59/567,296 | 59/567,296 → 59/567,296 | 59/567,296 → 59/567,296 |
| sum_array | 313/7,581,952 → 313/7,581,952 | 313/7,581,952 → 313/7,581,952 | 313/7,581,952 → 313/7,581,952 |

## Full memory-host/caret comparison

The final-base commands run every selected test with the required timeout:

```sh
cd minzc
nice -n 19 go test ./pkg/pipeline -run 'TestMemoryHost|TestCaretXOR' -count=1 -timeout 60m
```

Both final runs complete with identical outcomes: **72 pass, 3 known failures,
5 optional skips**, across all 80 top-level tests. Every subtest outcome also
matches. Baseline time is 2785.405s; branch time is 2661.937s. The existing
`TestCaretXORAndPointerDereferenceVMZ80/negative` passes, and all repaired append,
peel, selector, and policy-search tests have no new failures. The excluded
failures remain `TestMemoryHostFactoryOriginalParser`,
`TestMemoryHostLoopAssociationOriginalParser`, and
`TestMemoryHostOriginalSelfLanzTarget`. See the [complete per-test table](2026-10-22-p28b-memory-tests.md),
including subtests, for both outcomes of every test.

The initial
full comparison on the earlier R1/O5 base `879b56389` completed with identical
outcomes: 72 pass, 3 known failures, 5 optional exports/diagnostics skipped,
across all 80 top-level tests (including all subtests). Its three failures are
exactly FactoryOriginalParser, LoopAssociationOriginalParser, and
OriginalSelfLanzTarget, as excluded by the task. That baseline took 2908.760s;
the branch took 2808.135s. Its raw logs and per-test CSV remain archived under
`/mnt/ssd-510/minz/tmp/p28b-memory-*`.

All heavy work used nice 19, the requested SSD TMPDIR/GOCACHE, and the load <=12
gate with 120-second retries. Go package parallelism was limited while sharing
slots with the memory-host processes. Those processes were briefly suspended
for each four-worker benchmark/judge phase and resumed in finally blocks,
keeping at most four active jobs. Their 60-minute wall clock limits remained
in effect. The complete pipeline package was not run.

Final-base artifacts are under `/mnt/ssd-510/minz/tmp/p28b-final-*` and
`p28b-rebased-*`: raw full-test JSON, generated model cases/results/retries,
CLI repro results, quick-judge reports, and focused/unit logs. Original critic
files were read without modification.

Final regression source SHA-256: `8da4df12b9464cc10293f31eff5c383bcf9637ca98810c513e3db4013eac65b8`.

## Files changed in P28b

* `minzc/pkg/hir/lower.go`
* `minzc/pkg/nanz/parse.go`
* `minzc/pkg/pipeline/memory_host_append_copy_prototype.go`
* `minzc/pkg/pipeline/memory_host_arena_peel_prototype.go`
* `minzc/pkg/pipeline/nanz_literal_typing_test.go`
* `scripts/ci/pipeline_profiles.json`
* `reports/2026-10-22-p28b-literal-conversions.md`
* `reports/2026-10-22-p28b-memory-tests.md`
