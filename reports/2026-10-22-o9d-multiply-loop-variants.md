# O9d: select ordinary lowering for helper-heavy counted loops

Baseline: `origin/main` at `33418f9057450ef422ed349594e0bce408b9c881`.
Branch: `opt/o9-small-kernels`. Main-suite benchmarks only; no push.

## Cause and change

`TestCountedLoopConstantProfitabilityVMZ80` reproduced the O9 failure:
ordinary c3(3) = 133,413 T, default DJNZ = 134,408 T. Three independent
search gates prevented the better complete product from being selected:

1. Every CALL made the baseline cost unsupported, before any candidate trial.
2. Earlier per-function Grace toggles consumed the 16-compilation budget before
   c3's ordinary-loop trial.
3. Ordinary lowering allocates c3's parameter in B rather than A; the unchanged
   boundary gate would reject that candidate even though no other function or
   data references c3.

The versioned `z80-loop-max-v2` scorer recognizes the complete 19-byte encoded
add-and-shift byte multiplication leaf, including its internal branch targets.
It checks instruction shape rather than symbol names. The proven eight-bit
counter gives a worst-case cost of 463 T including RET, plus the CALL's 17 T.
Every caller instruction, including B/BC/DE saves, setup and restores, remains
in the ordinary final encoded CFG score. Changed helper bodies and unknown or
recursive calls remain unsupported. This is a static worst-case model, not an
all-input runtime-improvement proof.

Search sweeps each lowering axis across functions before optional Grace axes;
the deterministic function ordering, module-wide cap and complete-module trial
compilation remain. A compiler-selected entry ABI may change only when both
products have the same logical scalar return/parameter signature and no MIR or
assembly reference to that entry. Fixed/address-taken/error/explicit conventions,
struct returns, incoming calls/pointers and opaque assembly retain the refusing
gate. All other functions, data and helpers still require identical context.
The selected Steps retain their exact MIR, allocation, assembly and assertions.
No counted-loop legality rule or arithmetic semantics changed.

The original c3 test now measures 133,413 → 133,413 T (ordinary → selected),
and its complete module shrinks from the previous default's 469 to 463 bytes.

## Failing-first tests

`TestVariantSearchByteMultiplyVMZ80` keeps the original multi-function budget
context, renames the entry to `product`, requires an ordinary-loop decision and
no DJNZ, and checks independent wrapping-byte results in the MIR VM and Z80
emulator for all 256 multipliers. Every selected native runtime is no greater
than the explicit ordinary-lowering build.

`TestVariantSearchByteMultiplyHelperCostVMZ80` checks the helper summary against
native timing for all 256 multiplier bit patterns, including its exact 15 T per
add-bit difference, renamed helper symbols, a changed counter opcode and an
unknown recursive CALL. `TestVariantSearchEntryABIRefusals` covers independent
entries, direct calls, address data, explicit register contracts and opaque
assembly aliases. All three are registered in sorted fast pipeline inventory.

The first two tests fail behaviorally on origin/main: the ordinary winner is
missing, DJNZ remains, and the byte helper cannot be scored. Log:
`/tmp/o9d-failing-main.log` (15.679 s). The same exact tests fail after reversing
the production patch with `git apply -R`, restored by an EXIT trap:
`/tmp/o9d-failing-reversed.log` (15.627 s); k=3 is 133,413 → 134,408 T.
They pass with the fix: `/tmp/o9d-new-tests.log` (16.002 s).

## Validation

The requested package command passes, exit 0, with shared-host settings
`TMPDIR=/mnt/ssd-510/minz/tmp`, `GOCACHE=/mnt/ssd-510/minz/gocache`,
`GOMAXPROCS=1`, `GOFLAGS=-p=3` (one slot was reserved for focused pipeline tests):

```
nice -n 19 go test ./pkg/mir2/... ./pkg/hir/... ./pkg/c89/... ./pkg/nanz/... ./pkg/z80asm/... -count=1
```

MIR2 132.391 s; HIR 66.037 s; C89 167.088 s; Nanz 180.912 s;
z80asm 0.067 s; assembler regressions 0.006 s. Log: `/tmp/o9d-units.log`.

All 97 fast pipeline tests matching `CountedLoop|Variant|Profit|Shift|Byte`,
plus the four original A1 Grace API/policy/assertion tests, pass in 277.896 s.
This includes every `TestCountedLoop*` and `TestVariantSearch*` test. The selected
97-test set is the union, not 97 plus four. Log: `/tmp/o9d-pipeline-fast.log`;
exact names: `/tmp/o9d-fast-names`. The complete pipeline package was not run.
The final search pruning guard is additionally checked by rerunning all
`TestVariantSearch*` tests: pass, 35.825 s (`/tmp/o9d-variants-final.log`). Inventory matches all 894 entries (523 fast),
including the three newly registered tests: `/tmp/o9d-pipeline-inventory.log`.

Bounded judges pass with four workers, exit 0: direct-call seeds 0–499 =
500/500; folded seeds 0–99 = 100/100. Both have zero oracle/MIR mismatches,
native/MIR mismatches, assembly failures, compiler errors and oracle errors.
The compiler is built from the final production code. Logs:
`/tmp/o9d-judge-direct.log`, `/tmp/o9d-judge-folded.log`; JSON results under
`/mnt/ssd-510/minz/bench/o9d-judge-{direct,folded}`. No larger campaign,
assertion matrix or heldout benchmark was run.

## Main-suite before/after benchmark

Both runs use `nice -n 19 python3 research/c89-benchmark/run.py --sdcc-root /
--suite main --jobs 4`, shared SSD TMPDIR/GOCACHE, GOMAXPROCS=1 and at most
four jobs. Before uses the detached origin/main worktree
`/mnt/ssd-510/minz/tmp/o9d-main`. Outputs:
`/mnt/ssd-510/minz/bench/o9d-before` and `o9d-after`.

All 48 MinZ cells (default, size, Grace and no-Grace) pass. Exact source/workload
hashes match and no profile worsens in program bytes or native T. The two
SDCC struct_roundtrip compile failures are unchanged. The table compares the
entire O9 branch including this fix against main; existing O9 improvements are
not attributed to this follow-up. Each cell is before → after, program B; T.

| Case | Default B; T | Size B; T | Grace B; T |
|---|---|---|---|
| abs_diff | 27→27; 220,904→220,904 | 27→27; 220,904→220,904 | 27→27; 220,904→220,904 |
| bitwise8reg | 95→76; 831,488→655,360 | 95→76; 831,488→655,360 | 95→76; 831,488→655,360 |
| copy_bytes | 172→172; 109,332→109,332 | 172→172; 109,332→109,332 | 172→172; 109,332→109,332 |
| crc8 | 308→308; 69,665,126→69,665,126 | 308→308; 69,665,126→69,665,126 | 308→308; 69,665,126→69,665,126 |
| fat12_decode | 167→148; 30,336→24,384 | 167→148; 30,336→24,384 | 167→148; 30,336→24,384 |
| gcd | 51→51; 3,067,013→3,067,013 | 51→51; 3,067,013→3,067,013 | 51→51; 3,067,013→3,067,013 |
| memset_fill | 297→297; 3,588,390→3,588,390 | 297→297; 3,588,390→3,588,390 | 297→297; 3,588,390→3,588,390 |
| mul8x8 | 42→42; 948,736→948,736 | 42→42; 948,736→948,736 | 42→42; 948,736→948,736 |
| reverse_loop11 | 559→559; 11,818,880→11,818,880 | 557→557; 21,032,960→21,032,960 | 559→559; 11,818,880→11,818,880 |
| strlen_bytes | 283→283; 3,574,400→3,574,400 | 283→283; 3,574,400→3,574,400 | 283→283; 3,574,400→3,574,400 |
| struct_roundtrip | 59→45; 567,296→344,064 | 59→45; 567,296→344,064 | 59→45; 567,296→344,064 |
| sum_array | 313→307; 7,581,952→6,145,792 | 313→307; 7,581,952→6,145,792 | 313→307; 7,581,952→6,145,792 |

The additional no-Grace profile also has zero byte/T regressions.
Comparison: `/tmp/o9d-bench-comparison.log`.

## Files touched

- `minzc/pkg/pipeline/variant_cost.go`
- `minzc/pkg/pipeline/variant_search.go`
- `minzc/pkg/pipeline/variant_difference.go` (cost-model comment)
- `minzc/pkg/pipeline/variant_cost_test.go`
- `minzc/pkg/pipeline/variant_multiply_test.go`
- `scripts/ci/pipeline_profiles.json`
- `reports/2026-10-22-o9d-multiply-loop-variants.md`

