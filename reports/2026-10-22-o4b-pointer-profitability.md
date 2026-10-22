# O4b: conservative affine-pointer profitability gate (2026-10-22)

The gate fixes all critic n=100 default/size slowdowns and all size growth.
The requested retention of every named win is **not achieved**: retaining only
two persistent word values necessarily rejects the critic's `n_u16_down`
(two walkers plus a live word IV) and `c_param_alias` (two walkers, word IV,
and invariant word bound). This is a conservative performance NO-GO under
the complete requested acceptance criteria, despite the green regression gates.
No function/benchmark names are used by the optimizer.

## Implementation

Before mutating a loop, reject any IV-dependent address not covered by the
replacement set. Loads/stores and surviving pointer-add DAGs are checked.
Model address-DAG DCE, then count walkers, live word loop parameters and
invariant word state needed on subsequent iterations. Reject a budget over
two pairs. Constants and normal object addresses can be rematerialized;
dead invariant forwarding arguments do not consume the budget. Reject proven
constant trips below four, including guarded DJNZ.

In OptSize, compile a baseline from the same preprocessed HIR, preserving
profile decisions and running the complete MIR, contracts, allocation and Z80
pipeline. Keep the affine candidate only if its assembled module is no larger.
Unknown assembly sizes select the baseline. The byte-copy recognizer remains
available in both alternatives. This also applies through CompileHIRWithOptions.
The MIR range/no-wrap proof, wrap semantics and memory order are unchanged.

## Files touched

- minzc/pkg/mir2/affine_pointer_walk.go
- minzc/pkg/mir2/counted_pointer_walk.go
- minzc/pkg/mir2/affine_pointer_walk_test.go
- minzc/pkg/pipeline/pipeline.go
- minzc/pkg/pipeline/affine_pointer_walk_native_test.go
- minzc/pkg/pipeline/affine_pointer_profit_test.go
- scripts/ci/pipeline_profiles.json
- reports/2026-10-22-o4b-pointer-profitability.md

## Tests and evidence

`TestAffinePointerWalkProfitabilityNative` commits the critic's `n_u8_fwd`,
`c_iv_body` and `c_pressure` sources, emulator costs of the original indexed
lowering, raw/optimized MIR VM comparisons and native memory comparisons.
It also checks `c_u8_bound255`: default still walks, while size must fit the
original 945 bytes. The test is in the sorted CI fast inventory; all 855 compiled pipeline
entries match the manifest exactly.

With the implementation diff reversed against initial HEAD cee138d14, the
three requested probes fail in both default and size:

| Probe | T(base) | T(ungated) |
|---|---:|---:|
| n_u8_fwd | 48696 | 96078 |
| c_iv_body | 57493 | 87117 |
| c_pressure | 212390 | 232362 |

The same final test also fails against origin/main (1815f5a35 initially,
a7c98cee4 after rebase, `o4b-rebased-failing-main.log`) because its
positive walking-pointer shape is absent. Logs under
`/mnt/ssd-510/minz/tmp/o4b-*.log` include reversed-patch failure, original-main
failure and restored passing runs. The three regressing probes return exactly
to the baseline costs with the gate.

`TestAffinePointerWalkProfitabilityRefusals` verifies partial byte-wrapping
addresses, an invariant word bound, extra word state, and a known three-trip
DJNZ and the signed 0xffff (−1) zero-trip case are refused without mutating MIR. Existing native tests still check both
VMs and Z80 memory for all 84 source/profile cases; shape requirements permit
intentional profitability/size refusals. Descending word/wrap coverage remains.

Required package run on the rebased compiler at a7c98cee4 (GOMAXPROCS=2,
-p 2, -count=1; two package jobs plus the two single-process campaigns):

```
ok  	github.com/minz/minzc/pkg/mir2	129.867s
ok  	github.com/minz/minzc/pkg/hir	62.219s
ok  	github.com/minz/minzc/pkg/c89	78.185s
ok  	github.com/minz/minzc/pkg/nanz	118.302s
ok  	github.com/minz/minzc/pkg/z80asm	0.090s
ok  	github.com/minz/minzc/pkg/z80asm/regression	0.009s
```

Final post-rebase focused run: affine-pointer native/profitability, counted-pointer
alias overlap, HIR Grace pointer walks, profile-guided sum, literal-typing contexts,
and seeded pointer legality/domain/boundary: PASS (8.091s). Compiler-mode admission
also passed in the earlier focused run.
Additional related checks: ProfileGuidedSumVMZ80 and seeded
address-domain/boundary/legality pass. TestSeededPointerWalkMeasuredCaller fails:
its manually generated HIR pointer candidate is 172149T/147B, while its original
candidate is 120147T/54B. The exact same failure is reproduced with the O4b
implementation patch reversed on cee138d14. It also fails on origin/main, including a7c98cee4 after rebase,
(candidate 143775T/141B vs original 120147T/54B), so O4b does not introduce
the failing assertion. This
pre-existing performance assertion is left intact and is another NO-GO item.
The full pipeline package was not run.

## Critic and bounded judges

The read-only critic harness was copied temporarily, run against the candidate,
and removed before committing. All 40 probes / 1080 measurements were compared
with `crit-o4/tt.join` in the final post-rebase logs `o4b-rebased-focused.log` and
`o4b-rebased-base-campaign.log`: zero default/size n=100 time regressions, zero size byte
regressions. The three `c_i16_down_word(n=32768)` overflow/nontermination reports
(one per profile) are identical in the supplied baseline and candidate; no new
raw/optimized/native discrepancy appears.

Selected n=100 costs (wrap rows use n=40):

| Probe | Baseline T | Default T | Size T | Baseline / default / size bytes |
|---|---:|---:|---:|---|
| c_u8_down_alias | 35089 | 9460 | 35089 | 983 / 985 / 983 |
| n_u16_down | 42379 | 42379 | 42379 | 996 / 996 / 996 |
| c_param_alias | 36734 | 36726 | 36726 | 1022 / 1020 / 1020 |
| c_u8_bound255 | 14896 | 7782 | 14896 | 945 / 947 / 945 |
| c_wrap_param | 28937 | 28937 | 28937 | 131 / 131 / 131 |
| c_wrap_down | 25735 | 18595 | 25735 | 144 / 148 / 144 |
| n_wrap_param | 23236 | 18341 | 18341 | 119 / 118 / 118 |

The 150-module/1800-function generator was rerun on both origin/main and the
candidate at ZZSEED=0, ZZMODS=150, again after rebasing onto a7c98cee4.
Both pass (269.579s baseline including the 40 probes, 268.848s candidate). All 1800 individual T-state counts are identical, totaling
143365226→143365226 T. The supplied gen.join uses a different, unrecorded seed;
its qN_M labels alone cannot be compared with these runs. The same original
generator source and 1800-case count were retained. Logs: o4b-rebased-base-campaign.log
and o4b-rebased-campaign.log. The generator has no array-address walks, so the final
small-constant normalization does not affect these cases.

Bounded judges, candidate CLI, seed 0 and -j 4 --no-reduce:

| Mode | Seeds | Result |
|---|---|---|
| direct-call | 0–499 | 500/500 pass |
| folded | 0–99 | 100/100 pass |

Both were rerun after rebasing and report zero MIR/oracle mismatches, zero
Z80/MIR mismatches, and zero assembly/compiler/oracle errors. Logs:
`o4b-rebased-direct.log`, `o4b-rebased-folded.log`.
No larger fuzz campaign or assert_matrix was run.

## Main-suite benchmark

Before: separate detached origin/main worktree
`/mnt/ssd-510/minz/tmp/o4b-before` at a7c98cee4 (including P28 literal typing).
After: the rebased opt/o4-pointer-sr compiler at 2b8691558; the final amend only
updates this report. Both baseline/candidate benchmark matrices were rerun after
rebasing, and every measured value matches the earlier 1815f5a35-parent runs. Both runs used
`--suite main --jobs 4 --sdcc-root / --include-refs`, nice 19 and SSD temp/cache.
The heldout suite was not run. Artifacts:
`/mnt/ssd-510/minz/bench/o4b-rebased-before` and `o4b-rebased-after`.
All 36 MinZ and 24 Nanz reference cells pass, with no bytes or T regression.
The two baseline SDCC struct_roundtrip compilation failures remain unchanged.

Each cell is **program bytes before→after / total T before→after**.

| Case | Default B / T | Size B / T | Grace B / T |
|---|---|---|---|
| abs_diff | 27→27 / 220904→220904 | 27→27 / 220904→220904 | 27→27 / 220904→220904 |
| bitwise8reg | 95→95 / 831488→831488 | 95→95 / 831488→831488 | 95→95 / 831488→831488 |
| copy_bytes | 172→172 / 109332→109332 | 172→172 / 109332→109332 | 172→172 / 109332→109332 |
| crc8 | 315→315 / 94778402→94778402 | 315→315 / 94778402→94778402 | 315→315 / 94778402→94778402 |
| fat12_decode | 167→167 / 30336→30336 | 167→167 / 30336→30336 | 167→167 / 30336→30336 |
| gcd | 57→57 / 4953966→4953966 | 57→57 / 4953966→4953966 | 51→51 / 3067013→3067013 |
| memset_fill | 297→297 / 3588390→3588390 | 297→297 / 3588390→3588390 | 297→297 / 3588390→3588390 |
| mul8x8 | 45→45 / 1096192→1096192 | 45→45 / 1096192→1096192 | 45→45 / 1096192→1096192 |
| reverse_loop11 | 561→559 / 23644160→11818880 | 561→557 / 23644160→21032960 | 561→559 / 23644160→11818880 |
| strlen_bytes | 283→283 / 3574400→3574400 | 283→283 / 3574400→3574400 | 283→283 / 3574400→3574400 |
| struct_roundtrip | 59→59 / 567296→567296 | 59→59 / 567296→567296 | 59→59 / 567296→567296 |
| sum_array | 313→313 / 7581952→7581952 | 313→313 / 7581952→7581952 | 313→313 / 7581952→7581952 |

Nanz reverse_loop11 default/Grace: 578→567 bytes,
37508460→11662060 T. C default/Grace retain 23644160→11818880 T.
C size selects its smaller indexed alternative (557 bytes vs the walking
candidate's 559); its 21032960 T is below origin/main but sacrifices much of
O4's speed win. This tradeoff follows the mandatory no-growth size policy.

Compiler implementation diff SHA-256 (three compiler files, relative to
cee138d14): `31776a482965ff0a354f4855ce017eadfdb6280ffa603611e6561662c3bbbd79`.

No push. Final acceptance: NO-GO for retention of all specified wins; the
requested strict two-pair gate and the original current lowering cannot retain
the three-word n_u16_down/c_param_alias recurrences. No budget exception was
added. The compiler changes and regression fixes are committed for review.
