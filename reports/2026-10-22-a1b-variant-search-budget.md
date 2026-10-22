# A1b: deterministic budget for native variant search (2026-10-22)

## Final gate after the requested fetch/rebase

Final main base is `2b70eaa74` (O4 affine pointers). The requested fetch/rebase
completed, with both commits replayed. The results in this section supersede
the initial frozen-baseline results below for the final branch.

The integration preserves O4's full native size comparison. That comparison
retains the selected counted-loop policy and has the search checkpoint disabled:
a mandatory native baseline must never become an empty deduplication refusal.
MIR deduplication is bypassed when the affine size alternative is active; final
assembly deduplication still applies. Loop-bearing size functions remain
conservatively eligible for MIR Grace. The fixed limit counts top-level search
trials; O4 can add at most one native size comparison per trial, so the bound
is at most 32 native candidate products, plus the ordinary baseline. This
constant factor is independent of function count, wall time, and host speed.

The exact requested unit command passes again under the default package timeout,
exit 0: MIR2 121.850 s, HIR 73.488 s, C89 172.924 s, Nanz 369.301 s,
z80asm 0.065 s and assembler regressions 0.007 s. Log:
`/tmp/a1b-rebased-units.log`. Focused search/counted/Grace tests plus both
`TestAffinePointerWalk*` groups pass, 22.033 s:
`/tmp/a1b-rebased-focused.log`. The complete pipeline package was not run.
Compiled inventory: 868 entries, 497 fast, exact manifest match and sorted fast:
`/tmp/a1b-rebased-inventory.log`.

The exact new budget tests still fail on fresh origin/main, and the frozen
campaign subtest fails there at 66,627 T instead of at most 65,226 T:
`/tmp/a1b-rebased-failing-main.log`. The bounded branch passes them.
The 12-function frozen module is rerun after rebase: zero slower functions,
199,137 → 197,736 T, winner 66,627 → 65,226 T; 16 trials, two MIR ties,
exhausted=true. Log `/tmp/a1b-rebased-frozen-module.log`, 11.904 s.

Final rebased bounded judges pass, exit 0: direct-call seeds 0–499, `-j 4`,
500/500; folded seeds 0–99, `-j 4`, 100/100. Both report zero oracle/MIR and
native/MIR mismatches, assembly failures, compiler errors and oracle errors.
Logs: `/tmp/a1b-rebased-judge-{direct,folded}.log`; artifacts:
`/mnt/ssd-510/minz/bench/a1b-rebased-judge-{direct,folded}`.

### Final benchmark matrix

Fresh detached origin/main worktree:
`/mnt/ssd-510/minz/tmp/a1b-main-rebased`. Outputs:
`/mnt/ssd-510/minz/bench/a1b-rebased-{before,after}`. Both use the requested
main-suite runner with four jobs at low priority. All 48 MinZ cells pass,
including no-Grace, with matching source/workload hashes and no byte or T
regression. Default/size/Grace cells:

| Case | Default B; T | Size B; T | Grace B; T |
|---|---|---|---|
| abs_diff | 27→27; 220,904→220,904 | 27→27; 220,904→220,904 | 27→27; 220,904→220,904 |
| bitwise8reg | 95→95; 831,488→831,488 | 95→95; 831,488→831,488 | 95→95; 831,488→831,488 |
| copy_bytes | 172→172; 109,332→109,332 | 172→172; 109,332→109,332 | 172→172; 109,332→109,332 |
| crc8 | 315→315; 94,778,402→94,778,402 | 315→315; 94,778,402→94,778,402 | 315→315; 94,778,402→94,778,402 |
| fat12_decode | 167→167; 30,336→30,336 | 167→167; 30,336→30,336 | 167→167; 30,336→30,336 |
| gcd | 51→51; 3,067,013→3,067,013 | 51→51; 3,067,013→3,067,013 | 51→51; 3,067,013→3,067,013 |
| memset_fill | 297→297; 3,588,390→3,588,390 | 297→297; 3,588,390→3,588,390 | 297→297; 3,588,390→3,588,390 |
| mul8x8 | 45→45; 1,096,192→1,096,192 | 45→45; 1,096,192→1,096,192 | 45→45; 1,096,192→1,096,192 |
| reverse_loop11 | 559→559; 11,818,880→11,818,880 | 557→557; 21,032,960→21,032,960 | 559→559; 11,818,880→11,818,880 |
| strlen_bytes | 283→283; 3,574,400→3,574,400 | 283→283; 3,574,400→3,574,400 | 283→283; 3,574,400→3,574,400 |
| struct_roundtrip | 59→59; 567,296→567,296 | 59→59; 567,296→567,296 | 59→59; 567,296→567,296 |
| sum_array | 313→313; 7,581,952→7,581,952 | 313→313; 7,581,952→7,581,952 | 313→313; 7,581,952→7,581,952 |

### Final example and compiler timings

The same harness and two-process method described below are repeated against
fresh main and the final bounded branch. All previously passing examples pass;
the seven pre-existing frontend/admission errors remain. There are no timeouts.
The slowest successful example takes 10.797 s on the bounded branch.
Artifacts: `/mnt/ssd-510/minz/bench/a1b-rebased-compile-times/{main,after}.jsonl`.

| Example | Main s | Bounded search s | Candidates | Result |
|---|---:|---:|---:|---|
| abap/bubble_sort.abap | 0.114 | 0.111 | 0 | existing admission error |
| abap/fibonacci.abap | 0.887 | 1.738 | 0 | PASS |
| abap/fizzbuzz.abap | 0.907 | 1.881 | 0 | PASS |
| abap/forms.abap | 0.869 | 1.910 | 0 | PASS |
| abap/function_test.abap | 0.333 | 0.637 | 0 | PASS |
| abap/guessing_game.abap | 0.075 | 0.082 | 0 | existing admission error |
| abap/hello.abap | 0.683 | 1.564 | 0 | PASS |
| abap/hello_input.abap | 0.719 | 1.740 | 0 | PASS |
| abap/hello_param.abap | 0.661 | 1.531 | 0 | PASS |
| abap/hello_simple.abap | 0.630 | 1.574 | 0 | PASS |
| abap/makt_search_cpm.abap | 0.075 | 0.086 | 0 | existing admission error |
| abap/mara_alv.abap | 0.077 | 0.090 | 0 | existing admission error |
| abap/mara_alv_real_zx.abap | 0.078 | 0.096 | 0 | existing admission error |
| abap/mara_alv_zx.abap | 0.644 | 1.678 | 0 | PASS |
| abap/mara_report.abap | 0.829 | 2.192 | 3 | PASS |
| abap/name_test.abap | 0.254 | 0.338 | 0 | PASS |
| abap/oop.abap | 0.081 | 0.080 | 0 | existing admission error |
| abap/oop_assert.abap | 0.281 | 0.522 | 0 | PASS |
| abap/sap_calculator.abap | 0.828 | 1.934 | 0 | PASS |
| abap/sap_hello_zx.abap | 0.813 | 1.869 | 0 | PASS |
| abap/sap_zx_demo.abap | 0.639 | 1.581 | 0 | PASS |
| abap/select_demo.abap | 0.774 | 2.115 | 0 | PASS |
| abap/select_loop.abap | 0.836 | 1.763 | 0 | PASS |
| abap/selscreen.abap | 0.821 | 1.716 | 0 | PASS |
| abap/showcase_zx.abap | 0.828 | 1.873 | 0 | PASS |
| abap/sqlite_demo.abap | 0.070 | 0.078 | 0 | existing admission error |
| abap/string_templates.abap | 0.736 | 1.626 | 0 | PASS |
| abap/sysinfo.abap | 0.691 | 1.645 | 0 | PASS |
| abap/zfleet_csv_alv.abap | 3.994 | 5.858 | 0 | PASS |
| abap/zpickfile_cpm.abap | 2.482 | 10.797 | 1 | PASS |
| abap/zsql_mara_zx.abap | 0.824 | 1.618 | 0 | PASS |
| abap/zsql_zx.abap | 0.892 | 1.637 | 0 | PASS |
| nanz/self_parser.nanz | 2.285 | 7.007 | 2 | PASS |
| nanz/nc.nanz | 1.955 | 2.064 | 0 | PASS |
| nanz/self_tokenizer.nanz | 2.425 | 8.551 | 3 | PASS |
| nanz/tetris_tui.nanz | 0.572 | 2.203 | 2 | PASS |
| nanz/tetris_cpm.nanz | 0.544 | 2.247 | 2 | PASS |
| nanz/grace_tokenizer_checked.nanz | 0.644 | 2.847 | 3 | PASS |
| nanz/irc_client.nanz | 0.327 | 0.500 | 0 | PASS |
| nanz/zsql_zx.nanz | 0.063 | 0.087 | 0 | PASS |
| nanz/zsql.nanz | 0.068 | 0.154 | 0 | PASS |
| nanz/widemath.nanz | 0.068 | 0.320 | 5 | PASS |

C benchmark compilation milliseconds (main → bounded), excluding emulator and
process startup/shutdown, with matching default/size/Grace options:

| Case | Default ms | Size ms | Grace ms |
|---|---:|---:|---:|
| abs_diff | 2.88→5.85 | 2.63→6.54 | 3.13→6.75 |
| bitwise8reg | 8.59→20.23 | 12.00→24.15 | 10.55→17.11 |
| copy_bytes | 5.72→7.80 | 6.08→10.79 | 4.78→8.70 |
| crc8 | 26.14→81.10 | 26.05→76.73 | 28.25→82.02 |
| fat12_decode | 6.65→19.93 | 6.66→21.20 | 7.86→21.29 |
| gcd | 1770.97→2504.77 | 2006.38→2640.06 | 1531.23→2554.29 |
| memset_fill | 6.22→7.82 | 5.64→9.53 | 4.89→9.02 |
| mul8x8 | 6.77→14.78 | 7.41→12.59 | 6.52→15.14 |
| reverse_loop11 | 6.17→8.64 | 17.38→24.11 | 5.71→9.22 |
| strlen_bytes | 3.67→4.60 | 3.81→5.52 | 3.04→4.76 |
| struct_roundtrip | 7.61→12.46 | 6.10→13.66 | 7.78→10.91 |
| sum_array | 8.00→11.83 | 18.13→45.43 | 7.16→11.70 |

The actual rebased CLI again passes all 62 forced assertions (31 MIR2 and 31
Z80, eight invocations), maximum 12.205 s:

| Source | Forced MIR2 s | Forced Z80 s |
|---|---:|---:|
| forms.abap | 2.660 | 5.006 |
| function_test.abap | 2.464 | 7.039 |
| name_test.abap | 2.354 | 6.123 |
| oop_assert.abap | 2.286 | 12.205 |

Receipts: `/mnt/ssd-510/minz/bench/a1b-rebased-abap-forced.json`.
GCD's remaining compilation overhead is the fixed discovery pass and existing
VM-based BranchEquiv refusal described/profiled below; its native result stays
3,067,013 T. No compiler-latency improvement for every small module is claimed.

## Initial frozen-base measurements (before the final fetch)

Frozen main baseline: `84bb145e3`. Original unbounded search: `9d5e35d45`.
Both were measured in separate detached worktrees under
`/mnt/ssd-510/minz/tmp/a1b-{main,search-before}`. No push, heldout suite,
assertion matrix, or larger fuzz campaign was run for this follow-up.

## Change and legality

The baseline remains the first complete native product. A single dry MIR
Grace sweep identifies functions with differing optimized MIR, before PFCCO,
LUT evaluation, allocation, code generation or assembly. Calls are recorded
before constant folding: callers stay eligible because a callee's changed
VM step cost can affect constant folding. After an accepted recipe, later
MIR axes are reconsidered conservatively. Counted axes require source-loop
metadata; ordinary/permitted lowering and HIR Grace retain the exact raw-MIR
identity check before a candidate compilation.

A search-local SHA-256 cache rejects repeated optimized MIR before native
compilation. Its key includes JSON instruction/ABI metadata, the MIR dump,
and string kinds. Recursive types that cannot be serialized disable that
cache. Assembly is also hashed before assembly/scoring, excluding whole-line
diagnostic comments. These caches only refuse trials: no cached assembly
fragment, allocation or accepted rewrite is substituted.

Candidate compilations are limited to `min(16, 4 + 4 * loopFunctions)` per
module. The count includes candidates refused at the optimized-MIR checkpoint;
the initial baseline and one dry discovery pass are separate. Exhaustion keeps
the best exact product found so far. The limit, candidate order, tie handling,
and exhaustion depend on IR and counted work, never elapsed time. No wall-clock
cutoff is used, so host speed cannot change selection. `Steps.VariantSearch`
reports count, limit, exhaustion and both deduplication counts.

The frontend is parsed once. Aggregate/error ABI and initial call signatures
are prepared once in owned HIR; candidates clone that immutable prepared graph
before structural rewrites, splitting and lowering. Full-module PFCCO, boundary,
caller/helper/data checks and final encoded-cost comparisons remain in place.
All changes suppress work or reuse exact refusing facts; legality/admission of
the counted and Grace transformations is unchanged.

`BranchEquiv` refusing results are cached by exact function IR within a search.
The pass admits only call-free pure byte functions, so its result is independent
of module memory and peers. Only false results, which leave IR untouched, are
cached. Changed IR is a new key. No VM state or shared backend cache is reused.

## Failing-first and focused validation

`TestVariantSearchLoopFreeBudgetVMZ80` compiles 40 exported arithmetic leaves,
checks zero candidate compilations and accounting, verifies emitted leaves,
and checks byte wraparound in the MIR VM and Z80 emulator.

`TestVariantSearchDeterministicBudgetVMZ80` compiles 24 loop functions twice,
checks exactly bounded deterministic decisions/assembly and exhausted budget
accounting, and executes both an early selected winner and a late baseline
fallback. The winner remains faster after exhaustion.

Both fail on the original A1 production code: an identity leaf compiles one
candidate; the budget fixture compiles 24 candidates instead of at most 16.
The production-only diff was reversed with `git apply -R`, leaving the exact
new tests in place; an EXIT trap restored it. Evidence:
`/tmp/a1b-failing-reversed.log` (behavioral failures, 3.597 s).
Separate origin/main runs fail behaviorally because search accounting is absent:
`/tmp/a1b-failing-main-final.log`. Its frozen campaign subtest also fails with
66,627 T instead of at most 65,226 T, and one counted loop instead of zero.

Both top-level tests are in sorted `scripts/ci/pipeline_profiles.json` fast.
The existing nested search test additionally freezes the previous campaign's
winning loop shape, under a neutral function name, and checks MIR/native results,
ordinary-loop shape and at most 65,226 T. The previous counted nest winner and
losing counted recipe checks remain.

Final focused checks pass in 21.672 s; see the validation section below.

## Main benchmark: bytes and native T

Commands use `research/c89-benchmark/run.py --sdcc-root / --suite main --jobs 4`
under `nice -n 19`, outputs `/mnt/ssd-510/minz/bench/a1b-before` and `a1b-after`.
All 36 default/size/Grace cells pass with identical source and workload hashes;
there is no byte or T regression. All 12 additional no-Grace cells also pass
without regressions. Main now enables MIR Grace, so GCD is already 3,067,013 T
on this fresh baseline; the selected result is preserved in every profile.

| Case | Default B; T | Size B; T | Grace B; T |
|---|---|---|---|
| abs_diff | 27→27; 220,904→220,904 | 27→27; 220,904→220,904 | 27→27; 220,904→220,904 |
| bitwise8reg | 95→95; 831,488→831,488 | 95→95; 831,488→831,488 | 95→95; 831,488→831,488 |
| copy_bytes | 172→172; 109,332→109,332 | 172→172; 109,332→109,332 | 172→172; 109,332→109,332 |
| crc8 | 315→315; 94,778,402→94,778,402 | 315→315; 94,778,402→94,778,402 | 315→315; 94,778,402→94,778,402 |
| fat12_decode | 167→167; 30,336→30,336 | 167→167; 30,336→30,336 | 167→167; 30,336→30,336 |
| gcd | 51→51; 3,067,013→3,067,013 | 51→51; 3,067,013→3,067,013 | 51→51; 3,067,013→3,067,013 |
| memset_fill | 297→297; 3,588,390→3,588,390 | 297→297; 3,588,390→3,588,390 | 297→297; 3,588,390→3,588,390 |
| mul8x8 | 45→45; 1,096,192→1,096,192 | 45→45; 1,096,192→1,096,192 | 45→45; 1,096,192→1,096,192 |
| reverse_loop11 | 561→561; 23,644,160→23,644,160 | 561→561; 23,644,160→23,644,160 | 561→561; 23,644,160→23,644,160 |
| strlen_bytes | 283→283; 3,574,400→3,574,400 | 283→283; 3,574,400→3,574,400 | 283→283; 3,574,400→3,574,400 |
| struct_roundtrip | 59→59; 567,296→567,296 | 59→59; 567,296→567,296 | 59→59; 567,296→567,296 |
| sum_array | 313→313; 7,581,952→7,581,952 | 313→313; 7,581,952→7,581,952 | 313→313; 7,581,952→7,581,952 |

## Compile times: main, original search, bounded search

Timings are seconds unless marked ms, measured with the same standalone Go
harness linked against each checkout. The harness includes the frontend,
compilation and declared assertions for examples; benchmark C compilation uses
assertion mode none. It resolves Nanz imports using the repository stdlib.
Each input runs in a fresh process with GOMAXPROCS=1, two processes at a time,
at low priority, with a 30-second process deadline. Shared-host load causes
noise; these are observations, not portable latency guarantees. The table uses
process wall time, including startup and final Go runtime work. Source-to-result
compiler times and frontend times are also in the JSONL artifacts.

All 32 top-level ABAP examples and the ten largest top-level Nanz files by byte
size were included. All examples that passed on main pass after this change.
The seven existing frontend/admission errors are unchanged. The original
search has four 30-second timeouts here; the bounded search has none.
Maximum successful example time in the final sweep is 11.633 s.

| Example | Main s | Original search s | Bounded search s | Candidates | Result |
|---|---:|---:|---:|---:|---|
| abap/bubble_sort.abap | 0.098 | 0.100 | 0.106 | 0 | existing frontend/admission error |
| abap/fibonacci.abap | 0.718 | 28.480 | 1.912 | 0 | PASS |
| abap/fizzbuzz.abap | 0.775 | 29.283 | 2.003 | 0 | PASS |
| abap/forms.abap | 0.830 | >30 (timeout) | 2.184 | 0 | PASS |
| abap/function_test.abap | 0.276 | 0.440 | 0.573 | 0 | PASS |
| abap/guessing_game.abap | 0.088 | 0.084 | 0.088 | 0 | existing frontend/admission error |
| abap/hello.abap | 0.664 | 24.281 | 1.746 | 0 | PASS |
| abap/hello_input.abap | 0.734 | 26.523 | 1.907 | 0 | PASS |
| abap/hello_param.abap | 0.648 | 25.065 | 1.759 | 0 | PASS |
| abap/hello_simple.abap | 0.646 | 24.753 | 1.852 | 0 | PASS |
| abap/makt_search_cpm.abap | 0.079 | 0.090 | 0.097 | 0 | existing frontend/admission error |
| abap/mara_alv.abap | 0.085 | 0.124 | 0.110 | 0 | existing frontend/admission error |
| abap/mara_alv_real_zx.abap | 0.081 | 0.124 | 0.107 | 0 | existing frontend/admission error |
| abap/mara_alv_zx.abap | 0.665 | 24.874 | 1.949 | 0 | PASS |
| abap/mara_report.abap | 0.845 | >30 (timeout) | 2.897 | 3 | PASS |
| abap/name_test.abap | 0.217 | 0.266 | 0.387 | 0 | PASS |
| abap/oop.abap | 0.088 | 0.084 | 0.094 | 0 | existing frontend/admission error |
| abap/oop_assert.abap | 0.294 | 0.924 | 0.613 | 0 | PASS |
| abap/sap_calculator.abap | 0.851 | >30 (timeout) | 2.818 | 0 | PASS |
| abap/sap_hello_zx.abap | 0.836 | 29.603 | 2.650 | 0 | PASS |
| abap/sap_zx_demo.abap | 0.697 | 24.567 | 1.998 | 0 | PASS |
| abap/select_demo.abap | 0.813 | 28.586 | 2.293 | 0 | PASS |
| abap/select_loop.abap | 0.760 | 27.247 | 2.037 | 0 | PASS |
| abap/selscreen.abap | 0.740 | 27.567 | 2.072 | 0 | PASS |
| abap/showcase_zx.abap | 0.803 | 28.278 | 2.217 | 0 | PASS |
| abap/sqlite_demo.abap | 0.073 | 0.073 | 0.090 | 0 | existing frontend/admission error |
| abap/string_templates.abap | 0.676 | 27.233 | 2.034 | 0 | PASS |
| abap/sysinfo.abap | 0.667 | 26.734 | 1.872 | 0 | PASS |
| abap/zfleet_csv_alv.abap | 3.199 | 5.601 | 6.749 | 0 | PASS |
| abap/zpickfile_cpm.abap | 2.455 | >30 (timeout) | 11.632 | 1 | PASS |
| abap/zsql_mara_zx.abap | 0.663 | 24.491 | 1.809 | 0 | PASS |
| abap/zsql_zx.abap | 0.684 | 26.139 | 2.141 | 0 | PASS |
| nanz/self_parser.nanz | 1.419 | 12.652 | 7.324 | 2 | PASS |
| nanz/nc.nanz | 1.237 | 2.456 | 2.536 | 0 | PASS |
| nanz/self_tokenizer.nanz | 2.000 | 13.873 | 9.774 | 3 | PASS |
| nanz/tetris_tui.nanz | 0.469 | 5.750 | 2.381 | 2 | PASS |
| nanz/tetris_cpm.nanz | 0.447 | 9.285 | 2.560 | 2 | PASS |
| nanz/grace_tokenizer_checked.nanz | 0.490 | 12.678 | 3.615 | 3 | PASS |
| nanz/irc_client.nanz | 0.401 | 0.479 | 0.579 | 0 | PASS |
| nanz/zsql_zx.nanz | 0.084 | 0.082 | 0.112 | 0 | PASS |
| nanz/zsql.nanz | 0.063 | 0.780 | 0.164 | 0 | PASS |
| nanz/widemath.nanz | 0.073 | 1.751 | 0.411 | 5 | PASS |

Benchmark compilation times below exclude process shutdown/startup and emulator
execution, and are milliseconds. Each cell is main / original search → bounded
search, using default/size/Grace policies matching the benchmark runner.

| Case | Default ms | Size ms | Grace ms |
|---|---:|---:|---:|
| abs_diff | 2.68 / 11.68→8.52 | 2.59 / 10.70→5.98 | 2.69 / 12.10→5.21 |
| bitwise8reg | 8.48 / 43.32→23.00 | 8.51 / 86.70→27.22 | 8.72 / 67.87→21.79 |
| copy_bytes | 7.24 / 7.82→10.06 | 4.73 / 20.40→10.49 | 5.13 / 7.65→12.22 |
| crc8 | 34.88 / 57.54→120.51 | 25.41 / 66.04→112.72 | 23.05 / 90.20→126.33 |
| fat12_decode | 6.73 / 21.49→29.75 | 7.05 / 39.44→30.83 | 7.64 / 32.45→22.28 |
| gcd | 1286.58 / 3965.56→4431.81 | 1376.82 / 5864.08→4236.49 | 1333.76 / 2979.17→3699.70 |
| memset_fill | 4.43 / 6.83→12.04 | 3.71 / 12.38→13.23 | 3.87 / 6.11→10.57 |
| mul8x8 | 4.22 / 11.92→16.91 | 4.36 / 11.54→17.84 | 4.42 / 12.74→19.14 |
| reverse_loop11 | 4.89 / 22.66→14.96 | 4.73 / 15.79→11.60 | 5.19 / 15.56→16.16 |
| strlen_bytes | 2.66 / 5.47→7.39 | 3.80 / 7.02→7.09 | 2.63 / 3.95→5.80 |
| struct_roundtrip | 5.60 / 26.39→15.76 | 5.96 / 64.72→20.39 | 8.94 / 40.03→17.25 |
| sum_array | 5.47 / 16.90→16.56 | 5.54 / 26.95→14.53 | 5.73 / 19.49→11.87 |

Artifacts: `/mnt/ssd-510/minz/bench/a1b-compile-times/{main,before,after}.jsonl`,
`inventory.json`, harness `/mnt/ssd-510/minz/tmp/a1b-time.go` and driver
`/tmp/a1b-measure.py`. The final bounded-search sweep uses the final production code. The earlier
sweep is retained as `after-initial.jsonl` to expose shared-host timing variation.

### GCD baseline cost

CPU profile `/tmp/a1b-gcd-main.cpu` attributes 1.46 of 1.54 sampled CPU seconds
(94.81%) to `BranchEquiv` / `beqTestEquivalent`. It tries removing equality
shortcuts and uses the MIR VM to compare boundary inputs. Removing a zero-input
GCD guard can enter a nonterminating subtraction loop; the patched execution
runs to the VM's default 10,000,000-step limit before the rewrite is refused.
That existing refusing analysis, rather than parsing or allocation, explains
why even main takes about 1.3 seconds. The search-local refusal cache avoids
repeating identical failed proofs across the discovery pass and trials. The
underlying pass/domain and its step limit are deliberately unchanged.


A separate alternating GCD-only run records child user+system CPU seconds
as well as wall time, after the final changes, using one fresh process at a
time. These measurements show that discovery still has a fixed cost on this
small module; default and Grace compilation can be slower than the original
search, while size improves. The large-module gate is addressed by pruning
and the deterministic count cap, rather than a promise of lower latency for
every small input. Native bytes and T do not regress in any profile.

| GCD profile | Main CPU / wall s | Original search CPU / wall s | Bounded search CPU / wall s |
|---|---:|---:|---:|
| default | 1.214 / 1.213 | 2.368 / 2.365 | 2.725 / 2.721 |
| size | 1.636 / 1.634 | 3.782 / 3.778 | 2.408 / 2.404 |
| grace | 1.288 / 1.286 | 2.339 / 2.336 | 2.989 / 2.996 |

Raw alternating measurements: `/mnt/ssd-510/minz/bench/a1b-gcd-cpu-times.json`.

### Original timeout files through the actual CLI

The four sources from `a1-timeouts.txt` were additionally compiled with
`--assert-receipt --asserts-force mir2` and then `--asserts-force z80`, output
`/dev/null`, 30-second process deadline. All eight invocations exit 0, with 31 executed/passing assertions per backend
(62 total), zero failed assertions, maximum 14.422 s:

| Source | Forced MIR2 s | Forced Z80 s |
|---|---:|---:|
| forms.abap | 2.449 | 4.617 |
| function_test.abap | 2.172 | 9.525 |
| name_test.abap | 2.666 | 8.166 |
| oop_assert.abap | 2.788 | 14.422 |

Raw receipts/stdout/stderr: `/mnt/ssd-510/minz/bench/a1b-abap-forced.json`.
These direct invocations do not run the forbidden full assertion matrix.

## Final validation

The exact requested unit command passes on the final code, exit 0, with
`TMPDIR=/mnt/ssd-510/minz/tmp`, `GOCACHE=/mnt/ssd-510/minz/gocache`,
`GOMAXPROCS=1`, `GOFLAGS=-p=4`, and the default ten-minute package timeout:

```
nice -n 19 go test ./pkg/mir2/... ./pkg/hir/... ./pkg/c89/... ./pkg/nanz/... ./pkg/z80asm/... -count=1
```

MIR2 134.496 s; HIR 75.558 s; C89 199.257 s; Nanz 409.027 s;
z80asm 0.112 s; assembler regressions 0.010 s. Log:
`/tmp/a1b-units-final.log`. An earlier complete run also passed without a
timeout override (`/tmp/a1b-units.log`).

Focused pipeline tests: all `TestVariantSearch*`, all `TestCountedLoop*`,
`TestMIRGraceAssemblyAPIUsesSameNativePipeline`, `TestHIRGraceSizePolicy`,
`TestHIRGraceMIR2Assertions`, `TestGracePreservesInfiniteLoops`: pass, 21.672 s.
Log: `/tmp/a1b-focused-final.log`. The complete pipeline package was not run.
The compiled pipeline inventory has 866 entries, exactly matching the manifest;
new top-level tests are in sorted fast. Inventory log:
`/tmp/a1b-pipeline-inventory.log`.

Bounded judges: direct-call seeds 0–499, `-j 4`, 500/500; folded seeds 0–99,
`-j 4`, 100/100. Both exit 0 with zero oracle/MIR mismatches, native/MIR
mismatches, assembly failures, compiler errors and oracle errors. Logs:
`/tmp/a1b-judge-direct.log`, `/tmp/a1b-judge-folded.log`; JSON artifacts under
`/mnt/ssd-510/minz/bench/a1b-judge-{direct,folded}`.

An additional focused frozen-module check uses only the 12 functions from the
previous campaign module containing the winner (seed 1009). Every function's
MIR result and native result match the independent Go-generated expected value.
Zero functions are slower than the no-search baseline; total native T is
199,137 → 197,736. The original winner is still 66,627 → 65,226 T despite
exhausting the cap: 16 trials, limit 16, two MIR ties, exhausted=true.
Log `/tmp/a1b-frozen-module.log`, 12.324 s. The temporary probe test was removed.
No additional random campaign was added.

The full 1,800-function campaign is deliberately left to the manager, per the
instruction against larger local campaigns. Its previously winning fixed
shape is preserved at 65,226 T; bounded judges are reported above without
claiming they substitute for that full gate.

## Files

- `minzc/pkg/pipeline/pipeline.go`
- `minzc/pkg/pipeline/variant_search.go`
- `minzc/pkg/pipeline/variant_search_test.go`
- `scripts/ci/pipeline_profiles.json`
- `reports/2026-10-22-a1b-variant-search-budget.md`

## Module-size default (critic round 1, 2026-10-22)

The first critic measured the pre-budget branch: `examples/zx/tetris_playable.nanz`
4.7 s → 1013 s and `TestShowcaseCompileAssemble` timing out; across 531 example
files search changed no output. With the 16-trial budget tetris still took 77 s
(each trial recompiles the whole 344-function module). Search now runs by default
only for modules with at most 32 functions (`variantSearchMaxFuncs`); larger
modules search with `--variant-search` / `Options.ForceVariantSearch`. Tetris:
5 s. The limit counts functions, so the choice is deterministic.

Corrections to the A1 report: after the rebase, main already has MIR Grace by
default (#241), so the gcd improvement is no longer credited to search; on the
current base search changes no benchmark cell and is infrastructure for later
per-function choices (allocation cost models, counted-loop shapes). Repeated
`[Z80-VALIDATE]`/`[HIR-SPLIT]` stderr lines per candidate are a known follow-up.
