# P5: deterministic production code generation

Original measurement baseline: `origin/main` at `7063d30bdf087eb6992b76cea7ea717efba6a1ef`.
FIX1 rebased this branch onto `origin/main` at
`d0104ab5428d827b23c0df4d71c73b804bd9a140` (emulator cycles and multiply/constant fixes).
The corpus sizes and hash counts below are historical measurements against the
original baseline; they were not remeasured after this rebase.

Branch: `fix/P5-deterministic-codegen`. Measurements use the default Z80
backend, fresh compiler processes, and `--asserts none` for assembly generation.

## Results

- 124 tracked inputs from `git ls-files 'examples/nanz/*.nanz' 'examples/c89/*.c' 'examples/c/*.c'` (including nested files), five separate processes per input and compiler.
- Main: **48 files with multiple distinct assembly outputs**. Branch: **0**.
- Both compile 123 inputs on all five runs. `examples/c89/fatfs/diskio.c` fails on both because its platform/storage headers are missing.
- 110 inputs assemble successfully on every run on both versions, using each version's `cmd/mza` with raw binary output. The other 14 are excluded consistently; their rows below show a dash for sizes.
- Main total assembled bytes per corpus run: [60165, 60137, 60162, 60121, 60196]. Main **min/median/max: 60,121/60,162/60,196**. Branch: **60,388 bytes**, identical in all five runs (226 bytes above main's median).
- The original `--asserts all -o /dev/null` comparison covered 835 tracked inputs (236 successful exits and 599 failures on both versions). **The previous claim of no assertion-outcome changes was wrong.** File-level exit codes and the first failure hide later assertions. The isolated assertion comparison below supersedes that claim.

## Isolated assertion comparison after FIX1 rebase

The earlier file-level comparison was insufficient: a failing assertion stops
execution, hiding later outcomes. Following the critic's `one.sh` approach,
we blanked every other assertion candidate in each source (preserving line
numbers), kept the isolated file beside its original to preserve includes,
and invoked `--asserts-force z80 -o /dev/null` in fresh processes. Source
annotations saying `via mir2` were deliberately forced onto Z80. Each candidate
ran once on the deterministic branch and three times on rebased main, with
`SOURCE_DATE_EPOCH=946684801` explicitly set. This covered **1,738 candidates**:
the critic's 1,729 jobs plus nine additional tracked candidates, including the
new C/Nanz multiply examples. No run timed out (60-second limit).

The complete per-candidate exit codes and expressions are in
[the assertion CSV](2026-10-02-Deterministic-Codegen-asserts.csv).
Of these, 1,165 succeeded on both compilers in every run; 569 failed on both
in every run; four had differing outcomes, listed below. These are compilation
exit outcomes: syntax errors, unsupported archived inputs, imported assertions,
and Frill property checks can prevent reaching the isolated candidate. Such
failures are not counted as proof that the selected assertion itself failed.
The four differences below each have a diagnostic naming the selected assertion.

| Isolated source assertion | Main passes / 3 | Branch passes / 1 | Failing result |
| --- | ---: | ---: | --- |
| `examples/c/c99_math8.c:78` — `saturating_add(100,50) == 150` | 2 | 0 | Branch gets 200 instead of 150 |
| `examples/c/c_edge_cases.c:53` — `double_inc(5) == 11` | 1 | 0 | Branch gets 20 instead of 11 |
| `examples/zx/plasma.nanz:70` — `color_to_attr_s(0,201,0) == 0x44` | 2 | 1 | One main run gets 4 instead of 68 |
| `examples/c89/static_local.c:36` — `test_independent() == 20` | 2 | 1 | One main run gets 0 instead of 20 |

Thus `saturating_add(100,50)` changed from mostly passing in this main sample
to consistently failing with the branch's frozen allocation. `double_inc`
also freezes a failing choice; main passed 1/3 in both this sweep and a separate
targeted check. The critic's earlier sample passed 3/3 for saturating_add and
1/3 for double_inc. Three nondeterministic main runs cannot establish the
underlying pass frequency. Conversely, plasma's color assertion and the
independent static counters now consistently pass on the branch while main
remains intermittent in this sample.

**Fixing determinism freezes one allocation; that frozen choice exposes the
known 16-bit add `dst == rhs` bug in the two failing C assertions.** In
`saturating_add`, the branch emits `LD H,D; LD L,E; ADD HL,HL`, overwriting the
right operand and computing `2a`. This is wrong code, even though it is now
reproducible. The add fix belongs to a separate branch. FIX1 does not tune the
allocator tie-break or claim to fix that add bug.

For completeness, the critic also found the following reverse changes with
the original baseline. All remain successful on this branch, but all three
updated-main runs passed them too, so they are **not observed differences in
the rebased comparison**:

| Source assertion | Original critic main passes / 3 | Rebased main passes / 3 | Branch passes / 1 |
| --- | ---: | ---: | ---: |
| `examples/frill/showcase.frl:114` — `inc2 5 == 6` | 1 | 3 | 1 |
| `examples/c/c99_math8.c:79` — `average(10,20) == 15` | 1 | 3 | 1 |
| `examples/c/c23_constexpr_enum.c:71` — `next_color(3) == 0` | 2 | 3 | 1 |
| `examples/c/c99_enum_typedef.c:55` — `opposite(1) == 3` | 1 | 3 | 1 |
| `examples/c89/static_local.c:15` — `test_counter() == 3` | 2 | 3 | 1 |

Full diagnostics: `/tmp/p5-fix-{asserts,extra-asserts}.jsonl`; runner:
`/tmp/p5-fix-asserts.py`; supplemental C diagnostic sample:
`/tmp/p5-fix-target-asserts.jsonl`. The original critic observations are in
its `ares.txt`. Assembly confirming the frozen add sequence is in
`/tmp/p5-fix-saturating.a80`.

## Ordering audit and choices

The audit enumerated Go map range statements using `go/ast` and `go/types` in
all production HIR frontends (Nanz/MinZ, C/ObjC and its vendored parser, PL/M,
Lanz, Lizp, Pascal, ABAP, Frill), HIR lowering/splitting, MIR2, pipeline and
Z80 assembler. It also checked goroutines and time/random calls.

| Stage | Output-sensitive ordering | Resolution |
| --- | --- | --- |
| HIR splitting | Crossing-variable map determines generated parameter/call ABI | Lexical variable order, matching existing HIR loop lowering |
| C struct lookup | First matching struct/union layout depends on map iteration | Source declaration order |
| C struct promotion | First field-name match changes tuple return widths | Ordered declaration index, retaining last definition for duplicate names |
| ObjC | Inherited method map changes trampoline/function order and selector prefix choice | Ordered selectors, inherited declarations first; overrides retain their slot |
| Nanz interfaces | Multiple implementors appear in unstable ambiguity diagnostics | Sorted implementor names |
| Call graph | Map of callee counts makes edge ordering unstable | First call-site order; repeated calls retain their original slot |
| PBQP allocation | R0/R1 reduction mutates neighbours while ranging states | Ascending virtual register ID, matching the existing RN final tie-break |
| TSMC spilling | Pair map changes patch-store emission order | Ascending spilled register ID |
| Z80 accumulator saves | Map-first A value and equal definition-index candidates | Lowest virtual register ID; latest-definition priority is retained |
| C clock macros | `__DATE__` / `__TIME__` read compilation wall clock | UTC timestamp when `SOURCE_DATE_EPOCH` is set; local wall clock when unset; malformed or out-of-range values return an error |
| Assembler macros | Serial parameter replacement ranges a map | Simultaneous substitution in one scan of the original body, matching whole identifiers; argument text is never rescanned |

Ordering policies preserve the common first-inserted traversal for small maps
where declaration order exists. A 1,000-run baseline PBQP tie sample (two
accumulator constants and their sum) chose `B/A/A` 792 times and `A/B/A` 208
times; the branch chose the modal `B/A/A` assignment on all 1,000 runs. The
large baseline examples have many distinct whole-program outputs, so there is
no single modal assembly to preserve. Cost tables, optimization priorities,
candidate physical-register order and strict-less-than contract comparisons
are unchanged.

Other default-path maps copy dictionaries, accumulate integer counts/sets,
write independent entries, or collect slices sorted before use. Examples:
HIR environment cloning and mutation scans, MIR2 class propagation and union
find, register-info/cost-vector construction, liveness (RegSet iterates ascending
bits), pruning (reachable set, then declaration-order emission), peephole
reference/dead-label sets, independent global-store chains sorted by offset,
clobber/push lists, and pipeline summaries/label audits. No default-path
compiler goroutines or random calls were found. Vendored C parser debug
logging uses wall clock but does not enter emitted assembly.

PFCCO's experimental PBQP contract solver is not the production default;
`OptimizeContracts` uses greedy contracts. Its caller map builds adjacency
lists, but remaining edges contribute commutative integer costs, degree-one
reductions have only one surviving edge, and node/choice tie-breaks use fixed
slices. It does not make an output-sensitive map-first decision. Optional
Grace/LIR/other target backends are outside this production determinism claim.

## Regression evidence

`TestProductionDeterminism` reparses and compiles seven tracked real inputs
**20 times each in-process**, with byte-identical assembly required. It covers
`self_tokenizer`, array loops, function-pointer calls, arena structs, CP/M string
literals, the C89 benchmark, and ObjC inheritance. Running it on untouched main
fails; the initial six-input version differed by run 1 on `self_tokenizer` and
the arena allocator. The ObjC input independently differed by run 1 before its
frontend fix.

Each implementation file was temporarily reverted to `origin/main`, its
focused regression test run (exit **1**), and its fix restored and retested
(exit **0**). This includes all three accumulator-selection paths, struct and
union fallback, both struct-return and out-parameter promotion, ObjC trampoline
emission and ambiguous selector matching, clock macros, and macro substitution.
Revert-check logs are in `/tmp/p5-revert-*.log`, restoration logs in
`/tmp/p5-restore-*.log`; corpus artifacts and hashes are in
`/tmp/p5-corpus-{main,branch}/results.json`. All-assert outcomes are in
`/tmp/p5-all-asserts/results.json`.

## Required gates

FIX1 gates run sequentially from `minzc` after rebasing, with `set -o pipefail`,
`GOCACHE=/tmp/minz-go-cache` and `GOFLAGS=-buildvcs=false`.

| Command | Exit | Result |
| --- | --- | --- |
| `go build ./pkg/... ./cmd/...` | 0 | Passed |
| `go test ./pkg/hir ./pkg/mir2 ./pkg/pipeline ./pkg/cparse ./pkg/z80asm -count=1` | 1 | HIR and MIR2 pass; existing `TestGraceVerify` failures below |
| `go test -short ./pkg/c89/... -count=1` | 0 | Passed |
| `go test ./pkg/nanz -skip '^TestShowcaseCompileAssemble$' -count=1` | 0 | Passed |

Updated main reproduces the same four `TestGraceVerify` failures:
`18_tail_recursion.nanz` (MIR2 fib assertion), `hello_cpm_fib.nanz` (Z80 gcd),
`screen_customer.nanz` /
`screen_report.nanz` (Grace-path split branch argument count). The old multiply failure disappears after the rebase. These optional
Grace gate failures are not introduced by the determinism changes.
Logs: `/tmp/p5-fix-gate-{build,core,c89,nanz}.log` and
`/tmp/p5-fix-main-grace.log`.

FIX1 clock/macro regressions fail with the implementations reverted (exit 1)
and pass after restoring them (exit 0): `/tmp/p5-fix-revert-red.log`,
`/tmp/p5-fix-restored-green.log`. `TestProductionDeterminism` explicitly sets
`SOURCE_DATE_EPOCH=946684801`; all seven inputs pass 20 runs on the branch,
while updated main fails for tokenizer, arena, and ObjC inheritance
(`/tmp/p5-fix-main-determinism.log`). Reproducibility for programs using C
clock macros requires setting `SOURCE_DATE_EPOCH`.

## Per-input corpus measurements

Distinct counts are SHA-256 hashes of emitted assembly across five processes.
Main byte columns are per-input min/median/max; the aggregate above is
min/median/max of the five **whole-corpus totals**, not summed per-input bounds.

| Tracked input | Main distinct | Branch distinct | Main bytes min/median/max | Branch bytes |
| --- | ---: | ---: | --- | ---: |
| `examples/c/array_desig.c` | 3 | 1 | 220/224/224 | 224 |
| `examples/c/c11_anon_struct.c` | 1 | 1 | 137/137/137 | 137 |
| `examples/c/c11_features.c` | 1 | 1 | 251/251/251 | 251 |
| `examples/c/c23_attributes.c` | 1 | 1 | 168/168/168 | 168 |
| `examples/c/c23_bitint.c` | 5 | 1 | 257/259/263 | 259 |
| `examples/c/c23_constexpr_enum.c` | 5 | 1 | 322/328/331 | 328 |
| `examples/c/c23_digit_sep.c` | 1 | 1 | 116/116/116 | 116 |
| `examples/c/c23_embed.c` | 2 | 1 | 148/152/152 | 152 |
| `examples/c/c23_misc.c` | 1 | 1 | 243/243/243 | 243 |
| `examples/c/c23_preview.c` | 1 | 1 | 152/152/152 | 152 |
| `examples/c/c23_stdbit.c` | 2 | 1 | 1475/1475/1475 | 1475 |
| `examples/c/c99_bitops.c` | 3 | 1 | 382/384/384 | 384 |
| `examples/c/c99_control_flow.c` | 1 | 1 | 620/620/620 | 620 |
| `examples/c/c99_ctype.c` | 1 | 1 | 1152/1152/1152 | 1152 |
| `examples/c/c99_enum_typedef.c` | 4 | 1 | 315/315/317 | 317 |
| `examples/c/c99_math8.c` | 5 | 1 | 301/304/306 | 305 |
| `examples/c/c99_pointers.c` | 1 | 1 | — | — |
| `examples/c/c99_stdbool.c` | 2 | 1 | 129/129/129 | 129 |
| `examples/c/c99_structs.c` | 1 | 1 | 381/381/381 | 381 |
| `examples/c/c_edge_cases.c` | 2 | 1 | 222/229/229 | 229 |
| `examples/c89/assert_test.c` | 1 | 1 | 79/79/79 | 79 |
| `examples/c89/bench.c` | 1 | 1 | 125/125/125 | 125 |
| `examples/c89/bench_extended.c` | 1 | 1 | 878/878/878 | 878 |
| `examples/c89/bitops.c` | 1 | 1 | 255/255/255 | 255 |
| `examples/c89/c99_c11.c` | 3 | 1 | 485/487/487 | 487 |
| `examples/c89/c99_compound.c` | 2 | 1 | 283/285/285 | 285 |
| `examples/c89/c99_control.c` | 1 | 1 | 197/197/197 | 197 |
| `examples/c89/c99_designated.c` | 1 | 1 | 260/260/260 | 260 |
| `examples/c89/c99_preproc.c` | 1 | 1 | 249/249/249 | 249 |
| `examples/c89/c99_types.c` | 1 | 1 | 253/253/253 | 253 |
| `examples/c89/cast_test.c` | 1 | 1 | 87/87/87 | 87 |
| `examples/c89/compound_lit.c` | 1 | 1 | 170/170/170 | 170 |
| `examples/c89/constfold_showcase.c` | 1 | 1 | 211/211/211 | 211 |
| `examples/c89/cpm_io.c` | 1 | 1 | 397/397/397 | 397 |
| `examples/c89/dowhile_break.c` | 2 | 1 | 749/752/752 | 749 |
| `examples/c89/enum_test.c` | 1 | 1 | 126/126/126 | 126 |
| `examples/c89/fatfs/diskio.c` | 0 | 0 | — | — |
| `examples/c89/fatfs/ff.c` | 3 | 1 | — | — |
| `examples/c89/fatfs/ffsystem.c` | 1 | 1 | 0/0/0 | 0 |
| `examples/c89/fatfs_constructs.c` | 2 | 1 | 764/766/766 | 766 |
| `examples/c89/fatfs_lowlevel.c` | 5 | 1 | 1511/1513/1520 | 1519 |
| `examples/c89/flex_array.c` | 1 | 1 | 21/21/21 | 21 |
| `examples/c89/func_ptr.c` | 2 | 1 | 228/230/230 | 230 |
| `examples/c89/game_helpers.c` | 1 | 1 | 305/305/305 | 305 |
| `examples/c89/goto_test.c` | 1 | 1 | 58/58/58 | 58 |
| `examples/c89/hello.c` | 1 | 1 | 62/62/62 | 62 |
| `examples/c89/import_test.c` | 1 | 1 | — | — |
| `examples/c89/imported_math.c` | 1 | 1 | 135/135/135 | 135 |
| `examples/c89/include_test/main.c` | 1 | 1 | 98/98/98 | 98 |
| `examples/c89/logical.c` | 5 | 1 | 326/328/330 | 330 |
| `examples/c89/math16.c` | 1 | 1 | 342/342/342 | 342 |
| `examples/c89/math8.c` | 1 | 1 | 108/108/108 | 108 |
| `examples/c89/minmax_pair.c` | 1 | 1 | 49/49/49 | 49 |
| `examples/c89/nested_struct.c` | 1 | 1 | 594/594/594 | 594 |
| `examples/c89/sdcc_benchmark.c` | 1 | 1 | 337/337/337 | 337 |
| `examples/c89/sizeof_test.c` | 1 | 1 | 126/126/126 | 126 |
| `examples/c89/static_local.c` | 4 | 1 | 145/146/150 | 145 |
| `examples/c89/string8.c` | 1 | 1 | 233/233/233 | 233 |
| `examples/c89/struct_promote.c` | 4 | 1 | — | — |
| `examples/c89/switch.c` | 1 | 1 | 561/561/561 | 561 |
| `examples/c89/ternary.c` | 1 | 1 | 166/166/166 | 166 |
| `examples/c89/typeof_test.c` | 1 | 1 | 43/43/43 | 43 |
| `examples/c89/union_test.c` | 1 | 1 | 20/20/20 | 20 |
| `examples/nanz/01_sum_array.nanz` | 1 | 1 | 132/132/132 | 132 |
| `examples/nanz/02_sum_array_idiomatic.nanz` | 1 | 1 | 28/28/28 | 28 |
| `examples/nanz/03_filter_map_chain.nanz` | 1 | 1 | 50/50/50 | 50 |
| `examples/nanz/04_lut_popcount.nanz` | 1 | 1 | 512/512/512 | 512 |
| `examples/nanz/05_four_pointers.nanz` | 2 | 1 | 15/15/18 | 15 |
| `examples/nanz/06_pbqp_weighted.nanz` | 1 | 1 | 21/21/21 | 21 |
| `examples/nanz/07_ix_load_store.nanz` | 1 | 1 | 21/21/21 | 21 |
| `examples/nanz/08_arena_allocator.nanz` | 5 | 1 | 1103/1108/1110 | 1112 |
| `examples/nanz/09_function_pointers.nanz` | 1 | 1 | 30/30/30 | 30 |
| `examples/nanz/10_adt_option.nanz` | 2 | 1 | 239/239/239 | 239 |
| `examples/nanz/11_match_expression.nanz` | 1 | 1 | 43/43/43 | 43 |
| `examples/nanz/12_state_machine.nanz` | 1 | 1 | 99/99/99 | 99 |
| `examples/nanz/13_result_error.nanz` | 4 | 1 | 103/105/105 | 105 |
| `examples/nanz/14_error_propagation.nanz` | 1 | 1 | 140/140/140 | 140 |
| `examples/nanz/15_error_enforcement.nanz` | 1 | 1 | 98/98/98 | 98 |
| `examples/nanz/16_rotate_sled.nanz` | 1 | 1 | 48/48/48 | 48 |
| `examples/nanz/17_bool_retflag.nanz` | 1 | 1 | 172/172/172 | 172 |
| `examples/nanz/18_tail_recursion.nanz` | 1 | 1 | 119/119/119 | 119 |
| `examples/nanz/20_strref_test.nanz` | 1 | 1 | 61/61/61 | 61 |
| `examples/nanz/21_hashmap_test.nanz` | 5 | 1 | 975/979/983 | 979 |
| `examples/nanz/30_stream_test.nanz` | 1 | 1 | 356/356/356 | 356 |
| `examples/nanz/50_impl_showcase.nanz` | 1 | 1 | 191/191/191 | 191 |
| `examples/nanz/51_impl_cpm_demo.nanz` | 1 | 1 | 206/206/206 | 206 |
| `examples/nanz/abap_screen.nanz` | 5 | 1 | 1414/1414/1415 | 1415 |
| `examples/nanz/assert_test.nanz` | 1 | 1 | 12/12/12 | 12 |
| `examples/nanz/canvas_house.nanz` | 1 | 1 | 893/893/893 | 893 |
| `examples/nanz/hello_cpm.nanz` | 1 | 1 | 47/47/47 | 47 |
| `examples/nanz/hello_cpm_fib.nanz` | 2 | 1 | 265/268/268 | 268 |
| `examples/nanz/irc_client.nanz` | 5 | 1 | — | — |
| `examples/nanz/meta_screen.nanz` | 1 | 1 | 735/735/735 | 735 |
| `examples/nanz/mul16_gpu_test.nanz` | 1 | 1 | 121/121/121 | 121 |
| `examples/nanz/nc.nanz` | 5 | 1 | — | — |
| `examples/nanz/rotozoomer.nanz` | 3 | 1 | — | — |
| `examples/nanz/sap_mara_cpm.nanz` | 2 | 1 | 1926/1965/1965 | 1965 |
| `examples/nanz/sap_mara_demo.nanz` | 2 | 1 | 2153/2158/2158 | 2153 |
| `examples/nanz/screen_alv.nanz` | 5 | 1 | 3181/3212/3213 | 3213 |
| `examples/nanz/screen_customer.nanz` | 5 | 1 | 1960/1962/1962 | 1963 |
| `examples/nanz/screen_declarative.nanz` | 5 | 1 | 2287/2287/2288 | 2289 |
| `examples/nanz/screen_report.nanz` | 5 | 1 | 3591/3599/3599 | 3599 |
| `examples/nanz/self_lanz_parser.nanz` | 5 | 1 | — | — |
| `examples/nanz/self_parser.nanz` | 5 | 1 | — | — |
| `examples/nanz/self_tokenizer.nanz` | 5 | 1 | 4453/4474/4519 | 4653 |
| `examples/nanz/sha256.nanz` | 1 | 1 | 541/541/541 | 541 |
| `examples/nanz/sql_test6.nanz` | 3 | 1 | 733/739/742 | 742 |
| `examples/nanz/sqlite_demo.nanz` | 1 | 1 | 500/500/500 | 500 |
| `examples/nanz/sqlite_test.nanz` | 1 | 1 | 609/609/609 | 609 |
| `examples/nanz/test_irc_minimal.nanz` | 1 | 1 | — | — |
| `examples/nanz/tetris_cpm.nanz` | 5 | 1 | 3597/3599/3616 | 3607 |
| `examples/nanz/tetris_tui.nanz` | 5 | 1 | 3496/3500/3514 | 3512 |
| `examples/nanz/tiny.nanz` | 1 | 1 | 10/10/10 | 10 |
| `examples/nanz/tui_commander.nanz` | 5 | 1 | 1240/1240/1240 | 1240 |
| `examples/nanz/tui_commander_l3.nanz` | 1 | 1 | 407/407/407 | 407 |
| `examples/nanz/tui_cpm.nanz` | 1 | 1 | 594/594/594 | 594 |
| `examples/nanz/tui_demo.nanz` | 1 | 1 | 28/28/28 | 28 |
| `examples/nanz/tui_screen.nanz` | 1 | 1 | 421/421/421 | 421 |
| `examples/nanz/tui_zx.nanz` | 1 | 1 | 576/576/576 | 576 |
| `examples/nanz/typed_print.nanz` | 4 | 1 | 768/770/770 | 772 |
| `examples/nanz/widemath.nanz` | 2 | 1 | 644/648/648 | 648 |
| `examples/nanz/zsql.nanz` | 5 | 1 | — | — |
| `examples/nanz/zsql_zx.nanz` | 2 | 1 | — | — |
| `examples/nanz/zsql_zx_real.nanz` | 2 | 1 | — | — |
