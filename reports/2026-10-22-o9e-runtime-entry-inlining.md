# O9e: preserve callable runtime entries across inlining

Baseline: `origin/main` at `33418f9057450ef422ed349594e0bce408b9c881`.
Branch: `opt/o9-small-kernels`. Local commit only; main-suite benchmarks only.

## Change

`Internal` excludes a function from implicit public pruning roots. ABAP uses
it for generated runtimes, including Agon filesystem provider functions that
VM clients invoke by name. It is not proof that every caller is in the IR.
The O9 single-call-site inliner treated absence from the pruning roots as
private linkage, spliced the provider body into its sole IR caller and let
pruning remove the independently callable definition.

HIR functions and MIR function attributes now carry explicit `Private` and
`ReferencedByName` properties. Unrestricted single-use inlining requires
positive private proof. Existing narrowly permitted export copies still keep
their callable definitions and pass the complete emitted-size gate. An explicit
name reference overrides private linkage and remains a root through ordinary
MIR pruning and CP/M runtime closure construction. It also remains a root during
Agon platform binding. The trivial inliner can remove a call to a named entry;
the entry definition is still retained.

C static definitions supply private proof. Selectively imported Nanz helpers
that are not exported by that import also supply it. Imported C functions keep
their declared C linkage; absence from a Nanz alias list does not make an
externally linked C function private. Existing address, relocation, indirect
ABI, recursion, opaque assembly, compile-time and hard-register guards remain.
No optimizer decision depends on a benchmark or runtime function name.

The Agon frontend explicitly marks its compiled provider API, buffer accessors
and platform wrappers as name-referenced entries. Other built-in runtime bodies
remain subject to their existing pruning policy; in particular unused `sel_show`
must still disappear from ordinary CP/M reports.

## Regression coverage

- `TestSingleCallInternalHelperVMZ80`: an Internal helper without private proof
  retains its call and callable definition, checked on all 256 byte inputs.
- `TestSingleCallNamedEntriesVMZ80`: persisted linkage metadata, an imported-style
  name, explicit private plus name-reference precedence, an entry with no IR
  callers and an entry consumed by trivial inlining. Raw/final MIR VM and native
  Z80 execution cover all 256 byte inputs.
- `TestSingleCallSiteInlineVMZ80`: C static and an actual selective Nanz import
  still inline the private CFG helper, with raw/final VM and native exhaustive
  byte-input checks.
- `TestAgonFrontendNamedEntryWithoutReportCalls`: the original ABAP file picker
  with only its report body emptied retains the provider API through platform
  binding and production compilation; direct VM invocation and the native
  entry label are checked without adding an assertion root before compilation.
- MIR proof-boundary tests cover unknown linkage and an explicit name reference.
  MIR-only pruning and CP/M closure tests retain named entries and dependencies.
- The two new pipeline tests are in the sorted fast CI inventory.

## Failing-first evidence

The unchanged new ABAP test was copied into the detached origin/main worktree.
`TestAgonFrontendNamedEntryWithoutReportCalls` fails there in 0.092 s with
`provider entry _abap_fs_agon_encode removed`. Log:
`/tmp/o9e-abap-failing-main.log`. It passes on the fixed branch, together with
`TestAgonFrontendCompiledNameProvider` and `TestABAPGuardedCPMFullReports`
(9.940 s, `/tmp/o9e-abap-targeted-final.log`).

The new pipeline name-entry regression also fails behaviorally on origin/main:
its uncalled `idle` entry is removed (`/tmp/o9e-failing-main.log`). Both new
pipeline regressions fail on the original O9 branch, where the provider CFG is
consumed (`/tmp/o9e-failing-branch.log`). Reversing the core production patch
with `git apply -R /tmp/o9e-production.patch` reproduces both failures, exit 1,
0.039 s (`/tmp/o9e-failing-reversed.log`); the patch was restored immediately.
The final focused pipeline run passes in 10.254 s
(`/tmp/o9e-focused-final.log`).

## Package validation

The exact workflow package command is:

```sh
nice -n 19 go test -short -timeout 30m ./pkg/z80effects/... ./pkg/hir/... ./pkg/mir2/... ./pkg/z80asm/... ./pkg/c89/... ./pkg/codegen/... ./pkg/parser/... ./pkg/plm/... ./pkg/lanz/... ./pkg/lizp/... ./pkg/abap/... ./cmd/mzv/...
```

| Check | origin/main | Fixed O9 branch |
|---|---|---|
| Exact workflow package list | PASS, exit 0 | PASS, exit 0 |
| `go test ./pkg/abap -count=1` | PASS, 384.091 s | PASS, 403.905 s |

Logs: `/tmp/o9e-ci-main.log`, `/tmp/o9e-ci-final.log`,
`/tmp/o9e-abap-main.log`, `/tmp/o9e-abap-final.log`.
The workflow package run includes ABAP and mzv; it is not a claim that the
entire GitHub Actions workflow or every pipeline shard was run locally.

The requested fresh unit command passes, exit 0:

```sh
nice -n 19 go test ./pkg/mir2/... ./pkg/hir/... ./pkg/c89/... ./pkg/nanz/... ./pkg/z80asm/... -count=1
```

MIR2 109.745 s; HIR 53.357 s; C89 131.972 s; Nanz 160.760 s;
z80asm 0.058 s; assembler regressions 0.006 s.
Log: `/tmp/o9e-units-final.log`. The existing aggregate import regression also
passes independently in 0.097 s (`/tmp/o9e-c89-import-final.log`).

Only the two new and three related pipeline tests were executed:
`TestSingleCallNamedEntriesVMZ80`, `TestSingleCallInternalHelperVMZ80`,
`TestSingleCallSiteInlineVMZ80`, `TestSingleCallExportCopiesVMZ80`, and
`TestInternalFunctionsAndExplicitEntries`. The full pipeline package was not run.
The complete inventory matches 896 entries, including 525 fast entries; the
fast list is sorted (`/tmp/o9e-inventory-final.log`). The four CI sharding helper
unit tests also pass (`/tmp/o9e-inventory.log`).

All runs export SSD TMPDIR and GOCACHE and use nice 19, GOMAXPROCS=1 and
bounded Go package parallelism. Benchmark and fuzz worker counts are four.

## Bounded judges

The compiler built from the final source passes direct-call seeds 0–499
(500/500) and folded seeds 0–99 (100/100), each with `-j 4`, exit 0.
Both runs report zero MIR/oracle mismatches, native/MIR mismatches, assembly
failures, compiler errors and oracle errors. Logs:
`/tmp/o9e-judge-direct.log`, `/tmp/o9e-judge-folded.log`; JSON artifacts:
`/mnt/ssd-510/minz/bench/o9e-judge-direct` and `o9e-judge-folded`.
No larger campaign, heldout benchmark or assertion matrix was run.

## Main-suite before/after

Both runs use the requested driver with `--sdcc-root / --suite main --jobs 4`
and nice 19. Before uses the separate detached origin/main worktree at
`/mnt/ssd-510/minz/tmp/o9e-main`; after uses this branch's final production source.
Outputs are `/mnt/ssd-510/minz/bench/o9e-before` and `o9e-after`.
The after directory also retains `production.patch`, recording the uncommitted
production source used to build the measurement tool before the final commit.

All 48 MinZ cells (default, size, Grace and no-Grace) pass. Source and workload
hashes match. No profile worsens in bytes or T. Every MinZ byte/T result also
matches the previous O9d after measurements: all existing O9 gains are retained.
The two SDCC struct_roundtrip compile failures are unchanged. The table compares
the whole O9 branch against main; those gains are not attributed to this fix.
Each cell is program bytes before → after; total native T before → after.

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

The additional no-Grace profile also has zero regressions. Exact comparison:
`/tmp/o9e-bench-comparison.log`. Driver logs:
`/tmp/o9e-bench-before.log`, `/tmp/o9e-bench-after.log`.

## Files touched

- `minzc/pkg/abap/agon_frontend_runtime.go`
- `minzc/pkg/abap/agon_frontend_runtime_test.go`
- `minzc/pkg/abap/agon_z80_runtime.go`
- `minzc/pkg/c89/lower.go`
- `minzc/pkg/hir/hir.go`
- `minzc/pkg/hir/lower.go`
- `minzc/pkg/mir2/cpm_runtime_tui.go`
- `minzc/pkg/mir2/func.go`
- `minzc/pkg/mir2/funcprune.go`
- `minzc/pkg/mir2/funcprune_test.go`
- `minzc/pkg/mir2/inline_single.go`
- `minzc/pkg/mir2/inline_single_test.go`
- `minzc/pkg/nanz/parse.go`
- `minzc/pkg/pipeline/named_entry_inline_test.go`
- `minzc/pkg/pipeline/pipeline.go`
- `minzc/pkg/pipeline/single_call_inline_test.go`
- `reports/2026-10-22-o9e-runtime-entry-inlining.md`
- `scripts/ci/pipeline_profiles.json`
