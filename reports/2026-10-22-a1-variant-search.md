# A1: per-function native variant search, first stage (2026-10-22)

Frozen baseline at task start: `origin/main` / `1815f5a35224ac5b47b8b5d08d47939625a4594b`.
Worktree: `opt/a1-variant-search`. No push, heldout benchmark, larger fuzz
campaign or assertion matrix is part of this work.

## Implementation and contract

Search retains the current production compilation as its initial incumbent.
It makes one deterministic, function-name-sorted coordinate sweep. For each
input function it considers ordinary counted-loop lowering, permitted counted
lowering, MIR Grace toggled, and HIR Grace toggled. Previously accepted choices
remain in subsequent trials. There are at most four additional complete module
compilations per function. Exact raw-module-IR ties for the lowering/HIR axes
are skipped before allocation. This is a generation filter, not an accepted
partial compilation. Candidates compile serially because compiler/backend
caches have not been established safe for concurrent candidate compilation.

Each trial owns its HIR graph and recompiles the whole module, including
signature resolution, splitting, PFCCO, allocation and final Z80 emission.
There is no assembly-body splicing. The exact retained `Steps`, MIR,
allocation and assembly travel together; only provenance comments are updated
after selection. Assertions run against the retained product, not every trial.
`DisableVariantSearch` / CLI `--no-variant-search` preserves the old pipeline.
Explicit profile/runtime/table experiments, non-Z80 emission, intermediate-only
compiles and `DisableCountedLoops` retain their existing paths.

Per-function overrides select HIR Grace through a function allowlist, MIR
Grace through the existing driver, and counted lowering through a tri-state
HIR policy. Permit bypasses only the nesting and word-pair profitability
gates. Constant endpoint, unused-IV, unsigned/type/range admission and all
existing mutation, alias, volatile and control-transfer checks remain.
No function/benchmark names participate in legality or scoring.

The versioned `z80-loop-max-v1` objective reads **assembled opcode bytes**.
Natural loops are reduced from inside out, including entry/exit paths, copies,
spills, trampolines and actual JR/JP encodings. A proved constant unsigned +1
recurrence has N ordinary backedges or N-1 DJNZ backedges. Unknown loops use
16 backedges, with source identity/condition agreement required. The fixed
endpoint proof also excludes the impossible overshoot exit in the reversed
`CP; JR Z,exit; JR C,exit` lowering. Dense-map dominators were replaced with
bounded bitsets; CFGs over 4096 instructions and overflowing costs refuse.

A maximum-path score alone is insufficient for unknown branch arms. Those
comparisons additionally require matching symbolic predicates and carried
register effects, with non-increasing cost on **each** matching path. The
first proof implementation supports pure byte-register leaf functions with
an ordinary byte result in A and at most one unknown loop. Unmodeled effects,
flag predicates, calls, irreducibility, multiple loop exits, ambiguous/missing
loop labels, changed unknown loops or changed ABI/caller/helper/data context
retain the incumbent. Fixed straight-line loop nests have exact static costs.
Changed branchy fixed nests outside the path proof retain the incumbent too.

Speed must strictly reduce scored T without growing the complete module.
`--Osize` strictly reduces assembled module bytes. Ties retain the incumbent.
This is a static-model guarantee for admitted comparisons, not a claim of
universal runtime optimization. Size mode has its explicit byte objective.
Structured `VariantDecisions`, function traces and `; [trace] variant=...`
annotations record recipes, model, scores, candidate counts and refusal reasons.

## Focused tests and failing-first evidence

Five new pipeline tests are in the sorted `fast` inventory:

- `TestVariantSearchGCDVMZ80`: one emitted CP replaces the repeated compare;
  MIR and native checks include zero inputs and nontrivial byte pairs.
- `TestVariantSearchNestedVMZ80`: a permutation nest with the pressure balance
  reversed uses two DJNZ loops only when profitable. Its baseline is
  2446 T / 40 B and selected product 2327 T / 38 B. The exact q106_11-derived
  fixture (seed 1106, index 11; renamed `probe`) keeps the existing one-counted
  recipe; the whole-module trial changes its helper/ABI and is refused.
- `TestVariantSearchPointerSizeVMZ80`: original
  `examples/nanz/01_sum_array.nanz`, size selection, zero length and wrapping
  byte sums in both MIR VM and native execution.
- `TestVariantSearchScorerVMZ80`: exact static/native equality for all three
  counted policies and three input pairs, including the reversed-compare exit.
- `TestVariantSearchRefusesCrossingPaths`: a lower maximum cannot hide a slower
  cheap branch arm; unsupported parity and changed context refuse as well.

The exact GCD test failed on clean origin/main (`/tmp/a1-failing-origin.log`).
The requested patch-reversal check also failed with the implementation removed
using `git diff --cached --binary > /tmp/a1-production.patch; git apply -R ...`:
`duplicate compare remained`, two CP instructions. The patch was restored by
an EXIT trap. Log: `/tmp/a1-failing-reversed.log`. This was a behavioral failure,
not a build failure. With search, `(12,8)` measures 152 T and one CP.

New search tests plus all `TestCountedLoop*` pipeline tests passed after the
final scorer representation change: `/tmp/a1-focused-bitset.log`, 16.641 s.
The complete pipeline package was not run.

## Main-suite benchmark

Both runs use the requested runner with `--suite main --jobs 4 --sdcc-root /`.
Before is a separate detached origin/main worktree in
`/mnt/ssd-510/minz/tmp/a1-base`; outputs are
`/mnt/ssd-510/minz/bench/a1-before` and `a1-after`. All 36 MinZ cells pass.
Source and workload hashes match and no profile grows or slows down.

Default and size GCD select MIR Grace, saving 1,886,953 T (38.09%) and six
bytes. Grace already has that result. CRC8 ties the fresh baseline: all three
profiles on this newer origin/main already emit the same instructions,
315 B / 94,778,402 T. The older supplied report's CRC8 figures are not the
baseline used here.

| Case | Default bytes; T | Size bytes; T | Grace bytes; T |
|---|---|---|---|
| abs_diff | 27→27; 220,904→220,904 | 27→27; 220,904→220,904 | 27→27; 220,904→220,904 |
| bitwise8reg | 95→95; 831,488→831,488 | 95→95; 831,488→831,488 | 95→95; 831,488→831,488 |
| copy_bytes | 172→172; 109,332→109,332 | 172→172; 109,332→109,332 | 172→172; 109,332→109,332 |
| crc8 | 315→315; 94,778,402→94,778,402 | 315→315; 94,778,402→94,778,402 | 315→315; 94,778,402→94,778,402 |
| fat12_decode | 167→167; 30,336→30,336 | 167→167; 30,336→30,336 | 167→167; 30,336→30,336 |
| gcd | 57→51; 4,953,966→3,067,013 | 57→51; 4,953,966→3,067,013 | 51→51; 3,067,013→3,067,013 |
| memset_fill | 297→297; 3,588,390→3,588,390 | 297→297; 3,588,390→3,588,390 | 297→297; 3,588,390→3,588,390 |
| mul8x8 | 45→45; 1,096,192→1,096,192 | 45→45; 1,096,192→1,096,192 | 45→45; 1,096,192→1,096,192 |
| reverse_loop11 | 561→561; 23,644,160→23,644,160 | 561→561; 23,644,160→23,644,160 | 561→561; 23,644,160→23,644,160 |
| strlen_bytes | 283→283; 3,574,400→3,574,400 | 283→283; 3,574,400→3,574,400 | 283→283; 3,574,400→3,574,400 |
| struct_roundtrip | 59→59; 567,296→567,296 | 59→59; 567,296→567,296 | 59→59; 567,296→567,296 |
| sum_array | 313→313; 7,581,952→7,581,952 | 313→313; 7,581,952→7,581,952 | 313→313; 7,581,952→7,581,952 |

### Compilation time

Source-to-final-assembly wall time, milliseconds, before → after. Each entry is
one fresh-source compilation of the same frozen main-suite C input, including
C frontend and all search/assembly/proof work; emulator and artifact-file I/O
are excluded. Both harness binaries use the same measurement source, built
against their respective worktrees. GOMAXPROCS=1 and low priority were used;
shared-host scheduling makes these observations, not stable latency thresholds.
Raw CSV: `/tmp/a1-compile-before.csv`, `/tmp/a1-compile-after.csv`.

| Case | Default ms | Size ms | Grace ms |
|---|---:|---:|---:|
| abs_diff | 4.00→8.35 | 2.63→9.84 | 2.64→8.64 |
| bitwise8reg | 9.35→58.46 | 9.70→70.70 | 8.74→43.59 |
| copy_bytes | 3.42→4.88 | 4.08→18.68 | 4.83→6.53 |
| crc8 | 26.56→67.78 | 24.77→63.45 | 25.41→67.70 |
| fat12_decode | 6.07→21.22 | 5.79→30.87 | 6.27→19.14 |
| gcd | 1320.64→3216.20 | 1372.39→3677.36 | 1652.06→2064.21 |
| memset_fill | 2.16→3.14 | 2.47→7.01 | 3.30→3.55 |
| mul8x8 | 3.61→7.72 | 3.33→7.55 | 3.77→7.71 |
| reverse_loop11 | 3.79→9.20 | 3.79→8.75 | 4.37→10.00 |
| strlen_bytes | 2.25→1.64 | 1.48→3.91 | 2.25→2.15 |
| struct_roundtrip | 4.38→17.43 | 4.27→27.89 | 5.52→18.07 |
| sum_array | 5.23→9.62 | 3.33→14.98 | 5.71→10.31 |

## Validation notes

The initial required unit command passed MIR2 (132.104 s), HIR (81.905 s)
and assembler suites, but C89 and Nanz hit their 600-second package deadlines.
There were no preceding assertion failures. Active cases were
`TestDifferential_Z80_vs_SDCC` and
`TestShowcaseCompileAssemble/tetris_playable.nanz`. Initial log:
`/tmp/a1-units.log`. This exposed avoidable search work: baseline-unsupported
speed functions now skip candidate generation entirely, and an all-disabled
HIR Grace allowlist no longer invokes the no-op driver. Both changes preserve
native selection; they avoid work that could never produce an accepted trial.
The required command is rerun with a 30-minute per-package allowance.


Bounded judges on the final compiler both pass, exit 0: direct-call seeds
0–499 with `-j 4`, 500/500; folded seeds 0–99 with `-j 4`, 100/100. Both report
zero MIR/oracle mismatches, native/MIR mismatches, assembly failures, compiler
errors and oracle errors. Logs: `/tmp/a1-judge-direct.log` and
`/tmp/a1-judge-folded.log`. No other fuzz campaign or assertion matrix was run.

The final required unit command passes, exit 0, with
`TMPDIR=/mnt/ssd-510/minz/tmp`, `GOCACHE=/mnt/ssd-510/minz/gocache`,
`GOMAXPROCS=2`, and `GOFLAGS='-p=2 -timeout=30m'`:

```
nice -n 19 go test ./pkg/mir2/... ./pkg/hir/... ./pkg/c89/... ./pkg/nanz/... ./pkg/z80asm/... -count=1
```

MIR2 104.105 s; HIR 53.633 s; C89 463.798 s; Nanz 1047.815 s;
z80asm 0.095 s; assembler regressions 0.006 s. Log:
`/tmp/a1-units-final.log`. The longer package allowance is material: these
are not claimed as a pass under the original 600-second default.

The final focused pipeline run passes in 14.219 s: all five new tests, all
`TestCountedLoop*`, `TestMIRGraceAssemblyAPIUsesSameNativePipeline`,
`TestHIRGraceSizePolicy`, `TestHIRGraceMIR2Assertions`, and
`TestGracePreservesInfiniteLoops`. Log: `/tmp/a1-focused-complete.log`.
The extra losing-candidate subtest checks both MIR results and native code:
q106-derived default 249816 T, legal permitted recipe 442302 T. Search retains
the default. All three portable functional search tests also fail behaviorally
on origin/main (`/tmp/a1-failing-origin-all.log`): duplicate GCD compare,
one rather than two counted loops in the winning nest, and absent search
provenance. Their same final test source passes on this branch.

The supplied generator was copied temporarily, unchanged, with
`ZZSEED=1000 ZZMODS=150`. Both complete 1800-function native campaigns pass
against the generator's independent Go results. Fresh baseline:
242586095 T; selected: **242584694 T**. **Zero functions slower**, one faster:
`q9_11`, 66627 → 65226 T. All other functions tie. Logs:
`/tmp/a1-campaign-before.log`, `/tmp/a1-campaign-after.log`; summary:
`/mnt/ssd-510/minz/bench/a1-campaign-summary.json`. The final search campaign
completed in 1169.035 s, baseline in 229.543 s. Provisional campaigns were
stopped to remove avoidable analysis/compilation work; they are not counted
as complete validation. The temporary generator and fixture extractor were
removed and are not committed.

The compiled pipeline inventory has 855 entries; all five new top-level
tests are in sorted `fast`. No complete pipeline test execution was run.

## Baseline movement and files

While validation was running, another change advanced the shared remote ref
to `a7c98cee4` (P28 literal typing). This branch and both before/after
experiments remain based on frozen `1815f5a35`. No concurrent main changes
were mixed into the experiment; the results above refer to that exact base.

Files touched:

- `minzc/cmd/minzc/main.go`
- `minzc/pkg/hir/clone.go`
- `minzc/pkg/hir/counted_loop.go`
- `minzc/pkg/hir/grace.go`
- `minzc/pkg/hir/lower.go`
- `minzc/pkg/hir/static_loop.go`
- `minzc/pkg/mir2/func.go`
- `minzc/pkg/pipeline/pipeline.go`
- `minzc/pkg/pipeline/variant_cost.go`
- `minzc/pkg/pipeline/variant_cost_test.go`
- `minzc/pkg/pipeline/variant_difference.go`
- `minzc/pkg/pipeline/variant_search.go`
- `minzc/pkg/pipeline/variant_search_test.go`
- `scripts/ci/pipeline_profiles.json`
- `reports/2026-10-22-a1-variant-search.md`

Final status: EXIT 0 against the frozen task baseline, with the explicit
30-minute compiler-package test allowance above. Local commit only; no push.
