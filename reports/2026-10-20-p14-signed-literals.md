# P14 signed literal comparison repair

Historical initial implementation. The mixed-variable policy and final validation below are superseded by [review fix 1](2026-10-20-p14-review-fix-1.md).

Baseline: origin/main `bd61dfc5f`. Branch: `fix/P14-signed-narrow-const`.

## Root cause and implementation

`minzc/pkg/nanz/parse.go:4944` folds unary-negative integer literals to the smallest signed type: `-2` is an `i8`. `minzc/pkg/hir/lower.go:2430` already widens literals in arithmetic, but the original `lowerCond` independently lowered comparison operands. The right operand of `i16 < -2` therefore remained an `i8` constant.

`minzc/pkg/mir2/vm.go:118` documents width-masked integer storage; `truncate` stores `i8(-2)` as 254. At `vm.go:666`, signed comparisons reinterpret both operands at the single `SrcTy` width. At i16 width, 254 is positive. Equality also compared the distinct stored values 254 and 65534. This explains the agreement between the VM and Z80 failures.

`minzc/pkg/hir/lower.go:3055` now gives comparison literals the wider operand context before lowering, in either operand position and for all six comparisons. Mixed nonliteral comparisons retain the established left-width ordering and full-bit-pattern equality contract, enforced by the unchanged `TestSignedMixedComparisonMatchesSourceWidth`. Their physical zero extension is explicit in MIR2. Arithmetic already emits `sext` for signed widening.

The physical helpers `minzc/pkg/mir2/z80codegen_move.go:332` (`loadSpill16`) and `:1074` (`promote8toPair`) retain their zero-extension behavior. Comparisons and arithmetic now provide explicit semantic conversion before physical staging. `verifySignedWordOperands` at `minzc/pkg/mir2/verify.go:337`, also called at the start of `Z80Codegen`, rejects implicit narrow signed operands in word ALU/comparison instructions. The check accepts explicit `sext` for arithmetic and explicit `ext` for the legacy stored-bit comparison policy; physical register/spill moves do not guess which policy applies.

## Regression evidence

`TestSignedLiteralContexts` in `minzc/pkg/hir/signed_literals_test.go` runs raw MIR2, production optimized MIR2, and assembled Z80. It covers all six comparisons with the literal on either side, i8 contexts, the counter reproducer, negative add/multiply/divide/remainder, initialization, reassignment, i16 arguments and returns, explicit signed widening, existing mixed-variable policy, and large unsigned literals.

`TestSignedOperandWidthInvariant` in `minzc/pkg/mir2/signed_width_test.go` checks both the verifier and codegen rejection. No new tests were added to `pkg/pipeline`; its profile registry therefore needs no additions. The showcase catalog's exact count was intentionally updated from 199 to 200 for the new example.

Red runs, all exit 1:

- `.local/p14-red.log`: original comparison lowering produces wrong values on raw MIR2, optimized MIR2 and Z80.
- `.local/p14-invariant-red.log`: valid MIR2 with implicit signed widening was accepted before the invariant.
- `.local/p14-fuzz-red.log`: new signed oracle/generator tests fail against the origin/main generator.

The final focused comparison-policy run exits 0 (`.local/p14-policy-focused.log`). The manager's exact reproducer was also checked individually on both binaries and both backends (`.local/repro-p14-results.json`):

| Call | Baseline MIR2 and Z80 | Candidate MIR2 and Z80 |
|---|---:|---:|
| `lt(-5)` | 1 | 1 |
| `lt(0)` | 1 | 0 |
| `cnt(-5)` | 259 | 3 |
| `cnt(0)` | 254 | 0 |

`examples/nanz/signed_literals.nanz` contains 40 source assertions, with dual `via mir2` / `via z80` expectations. The complete added expectations are listed in that file.

## Signed generation and oracle

`--signed` opts into i8/i16 variables, negative literals, all six signed comparison operators, wrapping arithmetic, and signed division/remainder. The entry ABI stays the existing three-u16 direct-call ABI; signed variables are created inside the function. The unsigned generator and input mapping are unchanged: seeds 0–1999 matched the origin/main source and argument tuples exactly, 2000/2000. Four baseline fingerprints are also permanent Python regression tests.

The typed oracle models the generated signed subset with two's-complement wrapping at each operation width, typed casts and comparisons, division truncated toward zero, and remainder `a - trunc(a/b)*b`. It keeps MIR2/oracle disagreements as findings. Existing classification, receipts, negative controls, exit policies and thresholds were preserved.

## Final gates and judges

All gates were run sequentially with `set -o pipefail`, `GOCACHE=/tmp/minz-go-cache`, and `GOFLAGS=-buildvcs=false` for Go commands.

| Required gate | Exit |
|---|---:|
| `go build ./pkg/... ./cmd/...` | 0 |
| `go test ./pkg/hir ./pkg/mir2 ./pkg/nanz -count=1` | 0 |
| `go test ./pkg/c89/... -count=1` | 0 |
| `go test ./pkg/pipeline -run 'Z80\|Signed\|Literal' -count=1 -timeout 30m` | 0 |
| `python3 scripts/test_judges.py` | 0 |

Python self-tests ran 39 tests, with 4 optional integration tests skipped. Real compiler coverage came from the Go tests, fuzz runs and matrix. Earlier gate failures exposed fixture formatting, exact showcase count, and the existing mixed-variable comparison policy; they were corrected without weakening existing tests. One prematurely started pipeline process was terminated (exit 143) while C89 was active, then the final gates were run sequentially.

The candidate binary is `/tmp/claude-1000/-home-alice-dev-minz/c1b18760-5569-4099-8061-462392f70247/scratchpad/mz-p14`. The baseline was built from a detached origin/main worktree under `.local/worktrees/p14-baseline`, which was removed after the judges completed.

Fuzz runs used `-j 16`, `--timeout 30`, strict `--fail-on all`, and `--no-reduce` for the full runs. Reduction was performed separately on the sole unsigned finding.

| Corpus | Pass | MIR2 != oracle | Z80 != MIR2 | Assembly / compiler / oracle errors | Exit |
|---|---:|---:|---:|---:|---:|
| Baseline unsigned direct-call 0–999 | 999/1000 | 0 | 1 | 0 / 0 / 0 | 1 |
| Candidate unsigned direct-call 0–999 | 999/1000 | 0 | 1 | 0 / 0 / 0 | 1 |
| Candidate unsigned folded 0–999 | 1000/1000 | 0 | 0 | 0 / 0 / 0 | 0 |
| Candidate signed direct-call 0–1999 | 2000/2000 | 0 | 0 | 0 / 0 / 0 | 0 |

Unsigned baseline/candidate seed statuses are identical, with zero regressions. JSON and reproducers are under `.local/fuzz-{baseline-direct,candidate-direct,candidate-folded,signed-direct}/`.

The assertion matrix used baseline/candidate checkouts, `-j 16`, both backends, and default negative controls and judging rules:

| Backend | Newly failing | Existing newly passing | Added passing | Added failures | Flakes |
|---|---:|---:|---:|---:|---:|
| Z80 | 0 | 0 | 40 | 0 | 0 |
| MIR2 | 0 | 0 | 40 | 0 | 0 |

Existing units passing both: 1532 Z80, 1568 MIR2. Existing units failing both: 137 Z80, 101 MIR2. No newly passing pre-existing assertions were observed; the 40 added assertions are the new signed-literal example, checked under both forced backends.

The matrix exits 1 for pre-existing findings, which were retained: `examples/c89/import_test.c:18,19` produces zero-count execution receipts on both binaries and has two unexpected-success negative controls in each backend pass. The Z80 allowlist also contains two already-fixed-on-baseline entries, `examples/c/c11_anon_struct.c:62,63` (`pair_first()==42`, `pair_second()==99`). Their negative controls do fail correctly. The allowlist and judge were not changed. Full evidence: `.local/assert-matrix.json` and its journal.

The compile sweep compared the shared, tracked origin/main corpus (`--root .local/worktrees/p14-baseline`) so the newly added regression example is accounted for in the assertion matrix rather than treated as a baseline compile regression. Across 846 existing examples: 0 exit changes, 0 stdout changes, 0 stderr changes, 65 emitted-output changes. The sweep exits 1 because its policy flags any output-byte difference. Full per-file hashes and diagnostics: `.local/compile-sweep.json`.

## Other findings retained without fixes

1. **Unsigned Z80/backend mismatch, seed 864.** Original full program: MIR2/oracle 42237, Z80 42481, identical on baseline and candidate. A smaller accepted shape retains initialization/reassignment around a helper call:

```nanz
fun h1(p0:u16,p1:u16)->u16 { return (p1+p1)-(p0+p0) }
fun fuzz_entry(a:u16,b:u16,c:u16)->u16 {
    var v0:u16 = a
    v0 = h1(a,1)
    return v0
}
fun g()->u16 { return fuzz_entry(43418,47214,60935) }
assert fuzz_entry(43418,47214,60935) == 44238 via z80
```

This reduced shape passes MIR2 on both binaries and returns 25915 on Z80 on both binaries, versus 44238 expected. `.local/fuzz-reduced/seed-864-small.nanz` and `small-results.json` retain the evidence. The stock reducer reached a deletion minimum; a return-expression simplification was then verified, followed by another deletion reduction. No global-minimality claim is made.

2. **MIR2/oracle disagreement in a manual direct-return derivative of seed 864.** Removing the caller's local binding changes the finding class, so this was rejected as a reduction of the backend-only finding and retained separately:

```nanz
fun twice_diff(a:u16,b:u16)->u16 { return (b+b)-(a+a) }
fun entry(a:u16)->u16 { return twice_diff(a,1) }
fun g()->u16 { return entry(43418) }
assert entry(43418) == 44238 via z80
```

Both baseline and candidate report 2 on MIR2 and Z80, versus 44238 expected. This diagnostic derivative is outside the full seeded-run counts. Evidence: `.local/fuzz-reduced/seed-864-minimal.nanz`, `minimal-results.json`, and the equivalent original-name derivative `seed-864-tailcall-mir2.nanz`. No oracle expectation or judge was changed to hide it.

## Existing examples with emitted-output changes

- `examples/abap/fibonacci.abap`
- `examples/abap/fizzbuzz.abap`
- `examples/abap/forms.abap`
- `examples/abap/function_test.abap`
- `examples/abap/hello.abap`
- `examples/abap/hello_input.abap`
- `examples/abap/hello_param.abap`
- `examples/abap/hello_simple.abap`
- `examples/abap/hello_zx.nanz`
- `examples/abap/mara_alv_zx.abap`
- `examples/abap/mara_report.abap`
- `examples/abap/name_test.abap`
- `examples/abap/oop_assert.abap`
- `examples/abap/sap_calculator.abap`
- `examples/abap/sap_hello_zx.abap`
- `examples/abap/sap_zx_demo.abap`
- `examples/abap/select_demo.abap`
- `examples/abap/select_loop.abap`
- `examples/abap/selscreen.abap`
- `examples/abap/showcase_zx.abap`
- `examples/abap/string_templates.abap`
- `examples/abap/sysinfo.abap`
- `examples/abap/zfleet_csv_alv.abap`
- `examples/abap/zpickfile_cpm.abap`
- `examples/abap/zsql_mara_zx.abap`
- `examples/abap/zsql_zx.abap`
- `examples/e2e_iterators/iter_map_filter_foreach.minz`
- `examples/feature_tests/01_basic_types.minz`
- `examples/glsl_sphere_demo.minz`
- `examples/mzv_one_small_step.minz`
- `examples/mzv_one_small_step_medium.minz`
- `examples/mzv_one_small_step_simple.minz`
- `examples/mzv_sphere.minz`
- `examples/mzv_sphere_fast.minz`
- `examples/mzv_sphere_shaded.minz`
- `examples/mzv_test_sphere.minz`
- `examples/nanz/08_arena_allocator.nanz`
- `examples/nanz/13_result_error.nanz`
- `examples/nanz/14_error_propagation.nanz`
- `examples/nanz/15_error_enforcement.nanz`
- `examples/nanz/21_hashmap_test.nanz`
- `examples/nanz/abap_screen.nanz`
- `examples/nanz/irc_client.nanz`
- `examples/nanz/nc.nanz`
- `examples/nanz/rotozoomer.nanz`
- `examples/nanz/screen_alv.nanz`
- `examples/nanz/screen_customer.nanz`
- `examples/nanz/screen_declarative.nanz`
- `examples/nanz/screen_report.nanz`
- `examples/nanz/self_lanz_parser.nanz`
- `examples/nanz/self_parser.nanz`
- `examples/nanz/sha256.nanz`
- `examples/nanz/tetris_cpm.nanz`
- `examples/nanz/tetris_tui.nanz`
- `examples/nanz/typed_print.nanz`
- `examples/nanz/widemath.nanz`
- `examples/nanz/zsql.nanz`
- `examples/pascal/factorial.pas`
- `examples/pascal/sieve.pas`
- `examples/system-c/fat12/main.nanz`
- `examples/tail_recursive.minz`
- `examples/tail_sum.minz`
- `examples/zx/console_demo.nanz`
- `examples/zx/tetris.nanz`
- `examples/zx/tetris_playable.nanz`
