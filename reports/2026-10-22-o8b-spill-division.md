# O8b: byte-division spill result correction

Base: `origin/main` at `2b70eaa74`; original O8: `38913e138`.

## Cause and correction

O8 diamond commoning moves identical arithmetic out of exclusive branch arms.
In the supplied signed seeds this extends live ranges and makes a byte division
result spill. `genDivMod8` copied the quotient from B with a formatted register
load, producing `LD _spill_fuzz_entry_r26, B` in seed 233. This is an existing
emitter defect exposed by changed allocation: the forced-spill fixture also
fails on unchanged origin/main. The programs contain no shift-carry candidate;
the new reverse shift affinity is not the source of the invalid move.

The quotient copy now uses `emitLD8`, as the remainder path already does. It
emits absolute-memory stores through A, preserving main A and flags via shadow
AF when needed. Fixed register destinations retain their existing instructions,
and B-to-B remains a no-op. Division arithmetic and diamond/shift transforms are
unchanged. Allocation-dependent copies in the shift paths already use the
location-aware helpers.

The coalescer now applies one legality check to ISA-output, ordinary affinity,
and reverse-shift recoloring. It preserves explicit instruction and parameter
homes, including inline-assembly ABI homes; checks type/location compatibility;
and refuses infinite-cost class homes. A register-only class cannot coalesce to
a spill, and a hard accumulator cannot follow a memory affinity merely because
the soft ClassAcc cost table assigns memory a finite cost.

## Regression coverage

- `TestByteDivisionSpillVMZ80`: force signed/unsigned byte quotient/remainder
  destinations into memory, IYH, A, and B; check the spill-store shape and compare
  MIR2 and assembled Z80 against an independent arithmetic oracle over boundary
  operands, including byte wraparound.
- `TestDiamondDivisionSpillSeedsVMZ80`: retain all four full pressure-sensitive
  seed programs (53, 233, 261, 415), verify raw/final MIR2 and assembled/emulated
  Z80, with and without MIR Grace. Each fixture must contain one direct-call
  assertion; a missing assertion cannot silently pass.
- `TestCoalescePreservesRegisterConstraints`: register-only and hard-class memory
  affinities, plus a register-only ALU destination.
- Both new pipeline tests are registered in the sorted CI fast inventory.

## Validation

External logs and benchmark artifacts are under
`/mnt/ssd-510/minz/tmp/o8b-*` and `/mnt/ssd-510/minz/bench/o8b-{before,after}`.
All heavy commands use nice 19, at most four jobs, SSD temporary/cache paths,
and the uptime load guard (120-second waits while load exceeds 12).

Failing-first evidence (no production fix present):

- On unchanged `origin/main` (`2b70eaa74`),
  `TestByteDivisionSpillVMZ80/{div,sdiv}/spill` fails, with Z80 validation showing
  `LD _spill_probe_r3, B` and the spill-store shape assertion failing (exit 1).
- On original O8 (`38913e138`), `TestDiamondDivisionSpillSeedsVMZ80` fails to
  assemble all four seeds in both Grace modes (eight subtests, exit 1).
- On unchanged origin/main, `TestCoalescePreservesRegisterConstraints` fails:
  `acc/hardtrue` moves to `{LocMem mem 61440}`, and the register-only ALU result
  moves from C to A (exit 1).
- The baseline checks used the separate origin/main worktree with only the new
  test files copied in. The O8 red run preceded application of the production
  patch. Logs: `o8b-main-red.log`, `o8b-o8-red.log`,
  `o8b-main-coalesce-red.log` in the temporary directory above.

Green scoped pipeline run: PASS (28.728 seconds). It includes both new pipeline
tests, `TestDiamondCommonShiftVMZ80`, `TestShiftCarryBranchVMZ80`,
`TestShiftCarryMultiplyVMZ80`, `TestDivModModuleVMZ80`,
`TestZ80UnsignedDivisionOperandSetup`, and `TestSignedSubCmpBoundaryVMZ80`.

Required unit command:

```sh
cd minzc
nice -n 19 go test ./pkg/mir2/... ./pkg/hir/... ./pkg/c89/... ./pkg/nanz/... ./pkg/z80asm/... -count=1
```

PASS: mir2 (134.514s), hir (56.614s), c89 (84.183s), nanz (134.468s),
z80asm (0.077s), z80asm/regression (0.009s). The wrapper additionally sets
`GOMAXPROCS=4`, `GOFLAGS=-p=4`, `TMPDIR=/mnt/ssd-510/minz/tmp`, and
`GOCACHE=/mnt/ssd-510/minz/gocache` before executing the command.

The fresh CLI compiler is `/mnt/ssd-510/minz/tmp/o8b-mz`.
Signed direct-call seeds 0–999: **1000/1000 PASS**, zero errors or mismatches
(`--fail-on all`, four workers). This expanded range is explicitly requested by
O8b; no larger campaigns or assert_matrix are run.

Remaining judges: **PASS**, all with four workers and `--fail-on all`:

| Corpus | Mode | Seeds | Passed |
|---|---|---|---|
| Signed | direct-call | 0–999 | 1000/1000 |
| Default | direct-call | 0–999 | 1000/1000 |
| Masked loops | direct-call | 0–499 | 500/500 |
| Default | folded | 0–99 | 100/100 |

Every run reports zero MIR2/oracle mismatches, Z80/MIR2 mismatches, assembly
failures, compiler errors, and oracle errors. The complete commands and output
paths are preserved in `/mnt/ssd-510/minz/tmp/o8b-validate` and the corresponding
`o8b-fuzz-{signed,direct,masked,folded}.log` files.

## Main-suite benchmark

The before run used the separate, detached origin/main worktree
`/mnt/ssd-510/minz/tmp/o8b-main` at `2b70eaa74`. The after run used this O8
worktree with the production fix. Both used:

```sh
nice -n 19 python3 research/c89-benchmark/run.py --out <output> --sdcc-root / --suite main --jobs 4
```

Outputs are `/mnt/ssd-510/minz/bench/o8b-before` and
`/mnt/ssd-510/minz/bench/o8b-after`. Each MinZ run passed all 48 measurements
(12 cases × default/size/grace/nograce). Comparing every profile's `ProgramBytes`
and `TotalT` finds **zero regressions**. The retained O8 gains are exactly
69,665,126 T for crc8 and 948,736 T for mul8x8. No heldout suite was run.
The table reports program bytes / total workload T-states, before → after.

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

The nograce profile is also non-regressing: crc8 315→308 bytes,
94,778,402→69,665,126 T; mul8x8 45→42 bytes, 1,096,192→948,736 T; the other ten
cases are unchanged. The machine-readable regression check is
`/mnt/ssd-510/minz/bench/o8b-validation.json`.

## Files changed

- `minzc/pkg/mir2/z80codegen_muldiv.go`
- `minzc/pkg/mir2/coalesce.go`
- `minzc/pkg/mir2/coalesce_constraints_test.go`
- `minzc/pkg/pipeline/byte_division_spill_test.go`
- `minzc/pkg/pipeline/testdata/o8b/seed-{53,233,261,415}.nanz` (four exact copies)
- `scripts/ci/pipeline_profiles.json`
- This report.

The pipeline inventory is complete, both new tests are in the sorted fast list,
and `git diff --check` passes. All requested correctness and main-suite checks
pass. No push was performed.
