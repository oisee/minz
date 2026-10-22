# O8c: gate diamond motion on carry fusion

Baseline: unchanged `origin/main` at `2b70eaa74`. This commit follows O8b
(`8aff03dd4`) on `opt/o8-shift-carry`. No benchmark or function names are used
by the optimization. No push or heldout-suite run was performed.

## Change and legality

`HoistDiamondCommon` now accepts only a logical one-bit shift whose discarded
bit is exactly the bit tested by the head. The common proof also requires a
single branch consumer, removable single-use test/zero producers in the head,
constant count one, the same typed source bit through preserving copies, and
8- or 16-bit width. Shared mask values, other predicates, arithmetic right
shifts, counts above one, unrelated inputs, and other arithmetic stay in their
original diamonds. Existing dominance, exclusive-predecessor, purity and
constraint checks remain. Only total logical shifts are speculated.

Both stages of `PrepareShiftCarry` use that proof. Stage two leaves `both`'s
shared mask and compare in place, avoiding boolean materialization. Codegen
still independently proves physical homes and the emitted carry provenance.

The full C probe exposed another profitability boundary: moving the join shift
while an arm still reads its old input keeps a separate shifted value live
across the arm, causing accumulator saves. Preparation now refuses that case,
including uses through bit-preserving copies and arm edge operands. This
restores `c1/lsr` from the intermediate 5,964 T to the original 4,776 T.

PR scope note: the inherited `condCode` and `restoreLiveOutOverrides` changes
affect **all functions**, not just fused diamonds. They prioritize a predicate
preserved as a byte over stale flag selections and reserve its scratch home
during terminator shuffles. The inherited coalescer changes also affect all
functions: hard/ABI classes, type compatibility and infinite-cost homes are
checked on every recoloring path, with reverse affinity for suitable byte
shift loop edges and carefully discounted immediate-only counts. Those broader
changes remain in this branch and are covered by the required unit packages
and bounded judges; this commit changes only MIR preparation and its tests.

## Failing-first and validation

- `TestShiftCarryProfitabilityVMZ80`: reversed the production patch with
  `git apply -R`, ran the new test, then restored it. All seven shapes fail in
  both Grace modes (exit 1), with lost BIT predicates and/or larger T/bytes.
  Log: `branch-red.log`. The tests now pass.
- `TestShiftCarryCarryKernelsVMZ80`: on unchanged origin/main, fails for
  remaining BIT predicates and missing carry branches (exit 1); log
  `origin-red.log`. All eight subtests pass on this branch. The kernels are
  crc8, mul8 (the mul8x8 loop), crc16 and lfsr.
- `TestShiftCarryCriticProbesVMZ80`: exact copied p1/p2/p3/c1 fixtures, every
  function and each native assertion checked in raw MIR2, final MIR2 and Z80.
  Both Grace modes pass all 114 module budgets; the seven isolated shapes add
  14 size/timing budgets. Total: **128/128 non-regressing budgets**.
- Structural refusal coverage includes wrong bits, count two, SAR, shared
  test/zero producers, unrelated inputs, old-input arm uses and stage-two
  shared predicates. All three new pipeline tests are in the sorted fast
  inventory (869 entries total, 498 fast; exact compiled inventory check PASS).

Isolated shapes (both Grace modes give the same values):

| Shape | Origin T / bytes | O8b before this fix T / bytes | After T / bytes |
|---|---:|---:|---:|
| wb | 1,148 / 14 | 1,720 / 16 | 1,148 / 14 |
| wb2 | 1,176 / 14 | 1,752 / 16 | 1,176 / 14 |
| wb3 | 1,148 / 14 | 1,720 / 16 | 1,148 / 14 |
| hu | 5,203 / 18 | 9,578 / 26 | 5,203 / 18 |
| both | 2,336 / 25 | 3,912 / 37 | 2,336 / 25 |
| sar | 1,176 / 14 | 1,752 / 16 | 1,176 / 14 |
| sh2 | 1,372 / 18 | 1,944 / 18 | 1,372 / 18 |

Required unit command:

```sh
cd minzc
nice -n 19 go test ./pkg/mir2/... ./pkg/hir/... ./pkg/c89/... ./pkg/nanz/... ./pkg/z80asm/... -count=1
```

```text
ok  	github.com/minz/minzc/pkg/mir2	122.873s
ok  	github.com/minz/minzc/pkg/hir	51.598s
ok  	github.com/minz/minzc/pkg/c89	74.667s
ok  	github.com/minz/minzc/pkg/nanz	122.373s
ok  	github.com/minz/minzc/pkg/z80asm	0.092s
ok  	github.com/minz/minzc/pkg/z80asm/regression	0.008s
```

Scoped pipeline run: PASS (89.261s; see `pipeline-final.log`). Includes the three new
pipeline tests plus diamond, shift-branch, multiply and four O8b division-seed
regressions; the full pipeline package was not run.

Judges: direct-call seeds 0–499, four workers, **500/500 PASS**; folded seeds
0–99, four workers, **100/100 PASS**. Both use `--fail-on all`; no larger
campaign or assert_matrix was run. Logs: `fuzz-direct.log`, `fuzz-folded.log`.

All heavy commands run with nice 19, SSD TMPDIR/GOCACHE, at most four jobs and
an uptime/load guard that waits 120 seconds and retries if load exceeds 12.

## Critic probes: T vs origin/main for every function

T is summed over the unchanged native assertions (entry through RET). Each
row passes with MIR Grace off **and** on. Fixture
hashes and measurement details are in `testdata/o8c/README.md`.

| Probe/function | Calls | T origin → after (Grace off) | T origin → after (Grace on) |
|---|---:|---:|---:|
| p1.nanz/both | 28 | 2,336 → 2,336 | 2,336 → 2,336 |
| p1.nanz/cr | 14 | 646 → 534 | 646 → 534 |
| p1.nanz/crc16 | 28 | 11,072 → 8,936 | 11,072 → 8,936 |
| p1.nanz/crc16r | 28 | 13,327 → 10,915 | 13,327 → 10,915 |
| p1.nanz/crc8 | 28 | 13,819 → 6,971 | 13,819 → 6,971 |
| p1.nanz/crc8eq | 28 | 12,901 → 7,721 | 12,901 → 7,721 |
| p1.nanz/diff | 28 | 1,248 → 1,248 | 1,248 → 1,248 |
| p1.nanz/ext7 | 14 | 988 → 988 | 988 → 988 |
| p1.nanz/glob | 28 | 12,733 → 12,733 | 12,733 → 12,733 |
| p1.nanz/hk | 28 | 1,104 → 1,104 | 1,104 → 1,104 |
| p1.nanz/hm | 28 | 14,014 → 14,014 | 14,014 → 14,014 |
| p1.nanz/hs | 28 | 2,046 → 2,046 | 2,046 → 2,046 |
| p1.nanz/hu | 63 | 5,203 → 5,203 | 5,203 → 5,203 |
| p1.nanz/hw | 28 | 5,418 → 5,418 | 5,418 → 5,418 |
| p1.nanz/hz | 28 | 976 → 976 | 976 → 976 |
| p1.nanz/i16s | 14 | 959 → 959 | 959 → 959 |
| p1.nanz/ineg | 28 | 2,176 → 2,176 | 2,176 → 2,176 |
| p1.nanz/ishl | 28 | 1,260 → 628 | 1,260 → 628 |
| p1.nanz/joinold | 28 | 2,512 → 2,512 | 2,512 → 2,512 |
| p1.nanz/lc | 35 | 12,525 → 12,525 | 12,525 → 12,525 |
| p1.nanz/lfsr | 42 | 281,516 → 214,728 | 281,516 → 214,728 |
| p1.nanz/mul8 | 42 | 21,222 → 18,534 | 21,222 → 18,534 |
| p1.nanz/mul8b | 42 | 15,390 → 13,758 | 15,390 → 13,758 |
| p1.nanz/nest | 28 | 1,464 → 1,464 | 1,368 → 1,368 |
| p1.nanz/nest2 | 28 | 9,095 → 4,443 | 9,095 → 4,443 |
| p1.nanz/old | 28 | 1,644 → 1,372 | 1,644 → 1,372 |
| p1.nanz/old2 | 28 | 1,532 → 1,228 | 1,532 → 1,228 |
| p1.nanz/pc | 14 | 5,615 → 4,935 | 5,615 → 4,935 |
| p1.nanz/press | 28 | 43,131 → 41,339 | 43,131 → 41,339 |
| p1.nanz/sar | 28 | 1,176 → 1,176 | 1,176 → 1,176 |
| p1.nanz/sh2 | 28 | 1,372 → 1,372 | 1,372 → 1,372 |
| p1.nanz/sh2b | 28 | 1,372 → 1,372 | 1,372 → 1,372 |
| p1.nanz/sx | 14 | 1,702 → 1,380 | 1,702 → 1,380 |
| p1.nanz/tlo | 14 | 674 → 674 | 674 → 674 |
| p1.nanz/tlo7 | 14 | 730 → 377 | 730 → 377 |
| p1.nanz/unused | 28 | 820 → 820 | 820 → 820 |
| p1.nanz/w7 | 14 | 840 → 840 | 840 → 840 |
| p1.nanz/wb | 28 | 1,148 → 1,148 | 1,148 → 1,148 |
| p1.nanz/wb2 | 28 | 1,176 → 1,176 | 1,176 → 1,176 |
| p1.nanz/wb3 | 28 | 1,148 → 1,148 | 1,148 → 1,148 |
| p2.nanz/same | 4 | 280 → 174 | 280 → 174 |
| p2.nanz/same16 | 2 | 4,333 → 4,299 | 4,333 → 4,299 |
| p2.nanz/same2 | 3 | 2,928 → 2,664 | 2,928 → 2,664 |
| p3.nanz/bitret | 4 | 360 → 360 | 360 → 360 |
| p3.nanz/ldg | 30 | 2,685 → 2,685 | 2,685 → 2,685 |
| p3.nanz/ldp | 30 | 5,145 → 4,590 | 5,145 → 4,590 |
| p3.nanz/ldp2 | 30 | 32,040 → 30,120 | 32,040 → 30,120 |
| p3.nanz/shrbit | 30 | 1,971 → 1,482 | 1,971 → 1,482 |
| p3.nanz/shtw2 | 30 | 1,785 → 1,425 | 1,785 → 1,425 |
| p3.nanz/shtwice | 30 | 1,785 → 1,785 | 1,785 → 1,785 |
| p3.nanz/twotest | 30 | 1,875 → 1,275 | 1,875 → 1,275 |
| c1.c/crc16 | 8 | 3,875 → 2,775 | 3,875 → 2,775 |
| c1.c/crc8 | 28 | 14,715 → 6,971 | 14,715 → 6,971 |
| c1.c/crc8s | 28 | 32,011 → 32,011 | 32,011 → 32,011 |
| c1.c/crcr | 8 | 4,646 → 3,470 | 4,646 → 3,470 |
| c1.c/lsr | 6 | 4,776 → 4,776 | 4,776 → 4,776 |
| c1.c/mul | 30 | 11,660 → 9,580 | 11,660 → 9,580 |


| Module bytes | Origin → after (Grace off) | Origin → after (Grace on) |
|---|---:|---:|
| p1.nanz | 974 → 852 | 973 → 851 |
| p2.nanz | 221 → 214 | 221 → 214 |
| p3.nanz | 220 → 186 | 220 → 186 |
| c1.c | 192 → 159 | 192 → 159 |

## Main-suite benchmark

Before: isolated origin/main worktree `/mnt/ssd-510/minz/tmp/o8c-main`.
After: this branch. Both run the requested command with `--suite main` and
`--jobs 4`, writing `/mnt/ssd-510/minz/bench/o8c-{before,after}`. All 48 MinZ
measurements pass on each side. **No byte or T regression in any profile**,
including nograce. Retained default gains: crc8 **69,665,126 T**, mul8x8
**948,736 T**. Cells show bytes before→after / T before→after.

| Case | Default bytes / T | Size bytes / T | Grace bytes / T |
|---|---|---|---|
| abs_diff | 27→27 / 220,904→220,904 | 27→27 / 220,904→220,904 | 27→27 / 220,904→220,904 |
| bitwise8reg | 95→95 / 831,488→831,488 | 95→95 / 831,488→831,488 | 95→95 / 831,488→831,488 |
| copy_bytes | 172→172 / 109,332→109,332 | 172→172 / 109,332→109,332 | 172→172 / 109,332→109,332 |
| crc8 | 315→308 / 94,778,402→69,665,126 | 315→308 / 94,778,402→69,665,126 | 315→308 / 94,778,402→69,665,126 |
| fat12_decode | 167→167 / 30,336→30,336 | 167→167 / 30,336→30,336 | 167→167 / 30,336→30,336 |
| gcd | 51→51 / 3,067,013→3,067,013 | 51→51 / 3,067,013→3,067,013 | 51→51 / 3,067,013→3,067,013 |
| memset_fill | 297→297 / 3,588,390→3,588,390 | 297→297 / 3,588,390→3,588,390 | 297→297 / 3,588,390→3,588,390 |
| mul8x8 | 45→42 / 1,096,192→948,736 | 45→42 / 1,096,192→948,736 | 45→42 / 1,096,192→948,736 |
| reverse_loop11 | 559→559 / 11,818,880→11,818,880 | 557→557 / 21,032,960→21,032,960 | 559→559 / 11,818,880→11,818,880 |
| strlen_bytes | 283→283 / 3,574,400→3,574,400 | 283→283 / 3,574,400→3,574,400 | 283→283 / 3,574,400→3,574,400 |
| struct_roundtrip | 59→59 / 567,296→567,296 | 59→59 / 567,296→567,296 | 59→59 / 567,296→567,296 |
| sum_array | 313→313 / 7,581,952→7,581,952 | 313→313 / 7,581,952→7,581,952 | 313→313 / 7,581,952→7,581,952 |

## Files touched

- `minzc/pkg/mir2/{diamond_common,shift_carry}.go` and their two unit test files.
- `minzc/pkg/pipeline/shift_carry_profitability_test.go`.
- `minzc/pkg/pipeline/testdata/o8c/{p1.nanz,p2.nanz,p3.nanz,c1.c,origin-costs.json,README.md}`.
- `scripts/ci/pipeline_profiles.json`.
- This report.

External logs, command wrapper, validation script and machine-readable
comparison: `/mnt/ssd-510/minz/tmp/o8c/` (`validation.json`).
