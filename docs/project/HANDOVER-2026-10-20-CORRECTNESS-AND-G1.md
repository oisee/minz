# Handover: correctness and scalar generators, 2026-10-20

This snapshot is based on `67093aa97` (PR #216), after the correctness sprint.
It supersedes the October handovers linked in the [project index](README.md).
Production is HIR → MIR2 → Z80 with PFCCO/PBQP. PR #85 retired LIR;
there is no `--lir` flag or production `feat/lir-backend` path. VIR remains
an offline oracle ([ADR 0043](../adr/0043-vir-demoted-to-offline-oracle.md),
[ADR 0047](../adr/0047-mir-native-candidates-and-offline-vir.md)).
[ADR 0044](../adr/0044-lir-retirement-and-rule-migration.md) retains earlier
compatibility wording about `--lir`; that wording predates the removal in #85.

## What merged and why

| PR | Work | Result / reason |
|---|---|---|
| #55 | P0, J0/J1 | Live accumulator fix and exhaustive GCD judge with negative controls. |
| #56–#59 | Emulator timing, P3–P5 | Real T-states and run budgets; arithmetic and deterministic codegen fixes. |
| #60 | L0/L1 | Disabled native LIR emission rather than silently emitting wrong code; subsequently retired by #85. |
| #61 | S0 | Shared Z80 location/alias specification in `pkg/z80spec`. |
| #62–#64 | P7, C1, P8 | Live values across calls, codegen file split, valid spill operands and split labels. |
| #65–#69 | J2a, P9, C2, P10/P11 | Assertion receipts/controls, extern clobber ABI, CI judges, spilled assertion parameters and callee memory parameters. |
| #85 | Compiler integration | Shared HIR/MIR2/Z80 path; retired LIR rather than merging it with VIR. |
| #205 | G0 | Iteration audit, generator contract scaffold, callback return/asm fixes. |
| #206 | P12 | Byte spill accesses respect slot width; direct-call seeds 0–999 improved from 734 to 998 passing. |
| #207 | P13 | Preserve live saved values when destinations are reused; same corpus reached 999 passing. |
| #208 | P15 | Keep a constant load when both ALU operands use it; same corpus reached 1000/1000. |
| #209 | Import/assert lines | Preserve blank lines after `#import`; assert matrix had 0 unexpected control passes, exit 0. |
| #210 | CI | Direct-call fuzz findings became blocking. |
| #211 | P17 / G0-7 | Parse the whole scalar return expression, including `return (a)-(b)`. |
| #212 | P16 | Protect a live accumulator around NEG/BitGet. |
| #213 | P14 | Sign-extend narrow signed values at word width; adds signed differential fuzzing. |
| #214 | P18 | Restore live-out scratch values as one parallel copy. |
| #215 | P19 | Scope conditional-return peephole labels per function; division/remainder in one module no longer collide. |
| #216 | G1 | Scalar `gen name(p) -> T { yield e }` with caller-owned pull frames. |

The manager reports that the correctness sprint merged with a test that fails
without each fix, judge evidence, cross-family critic approval and green CI.
This documentation pass did not rerun the full compiler judges.

## Judge snapshot and limits

Manager-supplied results for `605c7e8a2` (after #215, before G1):

| Judge | Recorded result | Scope |
|---|---|---|
| Direct-call fuzz | 20000/20000 | Seeds 0–19999 |
| Signed direct-call fuzz | 4000/4000 | `--signed`, seeds 0–3999 |
| Folded fuzz | 1000/1000 | Mostly constant-folded wrapper; does not establish parameterized Z80 codegen correctness |
| Assert matrix after #209 | 0 unexpected control passes; exit 0 | Supplied result, not a fresh measurement at #216 |

These are bounded corpus results, not general correctness claims. See
[scripts/README.md](../../scripts/README.md) for receipts, known failures,
negative controls, comparison inventories and exit policies.

G1's pull path is implemented; fusion (G2) is planned. Measurements from
[`gen_yield_measure_test.go`](../../minzc/pkg/nanz/gen_yield_measure_test.go)
(`TestG1GenYieldMeasurements`, Z80 emulator; reproduced by an independent
critic before #216 merged):

| | generated pull | hand-written cursor (G0) | inline loop (approx. lower bound) |
|---|---|---|---|
| `bytes`: frame RAM / `next()` code / T per element | 4 B / 113 B / 877 | 4 B / 76 B / 555 | ~169 |
| `run_bytes` (RLE): frame RAM / `next()` code / T per element | 6 B / 525 B / 747 | 6 B / 195 B / 710 | ~128 |

Frame RAM equals the hand-written cursors. The generated RLE pull is within ~5%
of the hand cursor, the byte cursor is 1.58x. Both are 3–5x above an inline
loop: the pull-protocol overhead G2 fusion is meant to remove. Generated frames
are stack allocas; the hand-written cursors use global frames. The test logs
numbers; it has no performance gate.

## Reproduce the judges

Commands follow `scripts/README.md`, with shared-host parallelism reduced to
`-j 6`, `nice -n 19`, and temporary files on `/mnt/ssd-510/minz`.
Use a task-specific temporary directory; the baseline binary must be built from
the revision being compared, with inventory/receipt support.

```sh
judge_dir=$(mktemp -d /mnt/ssd-510/minz/d1-judges.XXXXXX)
export TMPDIR="$judge_dir"
(cd minzc && nice -n 19 go build -o "$judge_dir/mz" ./cmd/minzc)
nice -n 19 python3 scripts/test_judges.py
nice -n 19 python3 scripts/assert_matrix.py --mz "$judge_dir/mz" -j 6
nice -n 19 python3 scripts/fuzz_diff.py --mz "$judge_dir/mz" --mode direct-call --fail-on all --seed 0 --count 20000 -j 6 --output "$judge_dir/direct"
nice -n 19 python3 scripts/fuzz_diff.py --mz "$judge_dir/mz" --signed --mode direct-call --fail-on all --seed 0 --count 4000 -j 6 --output "$judge_dir/signed"
nice -n 19 python3 scripts/fuzz_diff.py --mz "$judge_dir/mz" --mode folded --fail-on all --seed 0 --count 1000 -j 6 --output "$judge_dir/folded"
(cd minzc && nice -n 19 go test ./pkg/nanz -run '^TestG1GenYieldMeasurements$' -v)
```

For baseline/candidate comparisons, set `baseline_mz` to the separately built
baseline binary and `baseline_root` to its checkout, then run:

```sh
nice -n 19 python3 scripts/assert_matrix.py --baseline "$baseline_mz" --baseline-root "$baseline_root" --candidate "$judge_dir/mz" -j 6 --json "$judge_dir/matrix.json"
nice -n 19 python3 scripts/compile_sweep.py --baseline "$baseline_mz" --candidate "$judge_dir/mz" -j 6 --json "$judge_dir/sweep.json"
```

## CI gates

- **PR gate** retains Go/toolchain checks.
- **Compiler checks / Direct-call fuzz findings** is required per the manager's #210 snapshot. It uses `--mode direct-call --fail-on all`; any finding or tool exception fails. Branch protection is external configuration; workflow code alone does not prove it is required.
- **Assert matrix** checks MIR2 and Z80, receipts, baseline regressions and known-failure entries. Negative controls and tuple mutants run for assertion/judge-sensitive changes and nightly; positive-only checks cannot detect a weakened checker.
- **Judge self-tests** validate judge behavior. **Short fuzz** uses folded mode with the weaker `--fail-on backend` policy on PRs; inspect its oracle findings too.
- **Default output sweep** and Go lint are advisory on PRs. Nightly audits enable controls, repeat the matrix, and run folded/direct-call fuzz with `--fail-on all`.

Use the CI simulation and regression-check commands in `scripts/README.md` when
changing CI wiring. A compiler fix must have a regression that fails without it;
a green positive corpus alone is insufficient.

## Open items and repro pointers

Status here is the manager's snapshot, not a live PR query. No external PR
statuses were queried in this documentation pass.

| Item | Failure / next action | Repro pointer |
|---|---|---|
| P20 | Assertion comparison ignores the declared signed return type: `fun f() -> i16 { return 0 - 495 }` with `assert f() == -495` reports "got 65041" on mir2 and z80 (the computed bits are right). | Fix on branch `fix/P20-assert-signed-compare` (critic GO with one must-fix: the first version masked bool results to 1 bit, so a bool returned as 2 matched `== 0` — the judge must never get looser; review fix in progress). |
| P21 | Storing a flag-located bool through a pointer emits invalid `LD (HL), F`: `p^ = (i & 1) == 1` assembles on MIR2 but not on Z80; also blocks bool generators that `yield <comparison>`. | Fix committed locally on branch `fix/P21-store-cmp-through-ptr`, not yet judged or reviewed. `TestG1BoolComparisonYield` (gen_yield_e2e_test.go) skips on Z80 until it lands and then un-skips itself. |
| P22 | MIR2 panic `appending instruction to sealed block @sw_join` on a switch without default whose cases all return. | Repro: `fun f(v: u8) -> u8 { switch v { case 1: return 1 case 2: return 2 } return 0 }`. Not started. |
| G0-3/4/9 | Narrow length/count underflow or wrap. | `TestG0Finding_ReversedSliceIsEmpty`, `TestG0Finding_CrossedRangeBounds`, `TestG0Finding_RangeCounterWidth` in [`iteration_audit_e2e_test.go`](../../minzc/pkg/nanz/iteration_audit_e2e_test.go). |
| G0-5 | Unsupported iterator methods emit invalid assembly. | `TestG0Finding_UnsupportedMethodRejected` in that file; reject cleanly or implement semantics. |
| G0-8 | Capturing callback with early return lacks a valid CALL fallback. | `TestG0Finding_CapturingCallbackReturn` in that file. |
| G0-L1..L4 | Legacy `.minz` iterator suite and callback ABI/array defects. | [`iterator_claims_legacy_test.go`](../../minzc/pkg/nanz/iterator_claims_legacy_test.go), [audit](../Nanz_Iteration_Audit.md). |

Skipped findings are open evidence, not passing acceptance tests. Do not infer
that the fixes above close every allocator, loop or assertion defect.

## Next work and operating model

G2 is demand-preserving fusion with a valid pull fallback under
[ADR 0050](../adr/0050-nanz-generator-fusion.md). Preserve ordered effects,
creation without body effects, early stop and source demand; compare manual
fused/pull and generated pull/fused consumers before making performance claims.
G3 is a real streaming workload; G4 is a bounded queue/ISR adapter. Resolve
P20–P22 and remaining audit findings alongside this track. The
[revised plan](PLAN-2026-10-01-BACKEND-SYNTHESIS.md) keeps shared-spec and
production-rule work without resurrecting LIR.

Workers are GLM-5.3 through Claude Code on Z.ai and Codex gpt-6.1-sol. A Claude
session manages scope, runs `fuzz_diff`, `assert_matrix`, `compile_sweep` and
`test_judges`, obtains a critic from a different model family, and merges after
green CI. On the shared host use `nice -n 19`, judges at `-j 6`, and SSD temp.
Workers commit locally only and never push or use `gh`; the manager pushes, opens PRs and merges after green CI.

## Peer-handover reconciliation

The local peer handover was read as historical input. Its spike summary
(#55–#69), shared-spec work and warning about stale CLAUDE metrics remain
useful and are incorporated above. Its checkout lag, worktree counts, draft PR
statuses and external follow-ups were not reverified and are not current claims.
Its claim that G0/G1 had not started is superseded by #205/#216. Its M3
LIR/VIR merge proposal missed #85 and must not guide new work.

It also omitted the v0.25.x releases. Local tags include `v0.25.0`, `v0.25.1`,
`v0.25.2`, `v0.25.3` and `v0.25.4`; `v0.25.0` points to the #93 embedded-demo
release merge, and `v0.25.4` to `d2d8f7ba3`. These tags establish release history,
not current corpus correctness. Do not restart from the peer's older checkout
or its design-only generator status.

## State at pause (2026-10-20, end of session)

Work was paused at the owner's request after G1 merged. Nothing is half-merged;
open branches are local/remote work in progress:

| Branch | State | To resume |
|---|---|---|
| `fix/P20-assert-signed-compare` | Not pushed. Fix committed (dca6f2a3c); a review fix (never mask bits above the declared width; bool must be exactly 0/1) was in progress by codex at pause | Re-judge (`assert_matrix` with controls, Go tests), re-run critic, open PR |
| `fix/P21-store-cmp-through-ptr` | One local commit by codex, judges/critic pending | Run judges and a critic; then un-skip check of `TestG1BoolComparisonYield` |
| P22 | Not started | Write a failing test from the repro above |
| `docs/D1-handover-plan-metrics` | This handover, the plan revision and the CLAUDE.md status rewrite | — |

Generator line next: G2 fusion under ADR 0050 (see above). Workers' worktrees
live under `.local/worktrees/<task>` in the main checkout (untracked); task files
are `.local/TASK.md` in each.
