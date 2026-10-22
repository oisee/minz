# O9b gate fixes — 2026-10-22

Validated production commit: `f0e9bf921`. Rebased onto `origin/main` at
`33418f905` (A1 variant search, #244); all nine O9 commits are retained. The
following commit adds stronger regression coverage and this validation record.

The retained-export byte-size gate recursively compiled its baseline with the
same assertion-bearing HIR module. Each product ran the assertion backends before
selection, so every successful assertion was executed twice. Compilation of
allocated alternatives now happens in `compileMIRProductSteps`; `compileMIRSteps`
runs assertions only on the selected MIR, allocation, and assembly. The affine
size gate uses the same separation. A1's counted-loop policies remain attached
to both gates, and mandatory comparisons bypass its refusing MIR cache so they
always produce complete allocated code.

The CP/M failure came from `remapInst` creating a non-nil empty `ExtraRets` slice
on pointer instructions. CP/M's strict local-address proof deliberately requires
canonical instruction metadata and rejected that clone. Straight-line and CFG
cloning now preserve absent metadata as nil, while real extra returns still get
owned, remapped slices. The validator and `TestCPMSourceBDOSCallerLiveState`
assertions are unchanged. The byte-leaf proof already rejects calls, externs,
ASM, and opaque effects; it was not responsible for this failure.

Files changed in this round:

- `minzc/pkg/pipeline/pipeline.go`: separate product selection and assertion execution; preserve A1 gate integration.
- `minzc/pkg/mir2/inline.go`: preserve nil extra-return metadata while remapping real returns.
- `minzc/pkg/mir2/inline_single.go`: preserve nil slices in CFG cloning.
- `minzc/pkg/pipeline/inline_assert_receipt_test.go`: `TestLizpInlineAssertReceipts`, all seven original tokenizer assertions separately, both backends, with variant search enabled and disabled (28 one-execution checks).
- `minzc/pkg/pipeline/inline_local_pointer_test.go`: `TestInlineLocalPointerCPMNative`, straight-line and branching private C helpers; check inlining, canonical field metadata, MIR VM result, and CP/M Z80 execution.
- `minzc/pkg/mir2/inline_metadata_test.go`: `TestInlinePreservesAbsentReturnMetadata`, exact pointer shape plus independent extra-return remapping and ownership.
- `scripts/ci/pipeline_profiles.json`: register both new pipeline tests in the sorted fast list.
- This report.

Failing-first evidence is under `/mnt/ssd-510/minz/bench/o9b-evidence`:

- `origin-failing-first.log`: tests copied into the separate `origin/main` worktree. `TestInlineLocalPointerCPMNative` fails because the private pointer helper is not inlined; `TestInlinePreservesAbsentReturnMetadata` fails because origin does not remap nonempty extra returns. The original receipt and BDOS tests pass there, as expected for O9 regressions.
- `reversed-fixes-red.log`: reverse-apply just the fixes before rebasing. Every tokenizer assertion reports `{Executed:2 Passed:2 Failed:0}`; the new pointer test and unchanged BDOS test both fail with `CP/M runtime invalid derived local address`; the metadata test rejects the noncanonical field clone.
- `rebased-reversed-red.log`: repeat with the final, strengthened tests after rebasing. Search-disabled tokenizer checks reproduce the duplicate receipt, both pointer helper shapes fail CP/M admission, and the metadata and BDOS regressions fail.
- `rebased-restored-green.log`: restore the exact production patch and rerun the same tests; both packages pass.
- `strengthened-tests-green.log`: all 28 receipt subtests and both native local-pointer subtests pass.

All validation used nice level 19, `TMPDIR=/mnt/ssd-510/minz/tmp`,
`GOCACHE=/mnt/ssd-510/minz/gocache`, and at most four jobs. Load was checked before
each heavy command and remained below 12.

Validation results:

- Required unit command: `go test ./pkg/mir2/... ./pkg/hir/... ./pkg/c89/... ./pkg/nanz/... ./pkg/z80asm/... -count=1` — PASS.
- Targeted pipeline command: `go test ./pkg/pipeline -run 'TestCPM|Lizp|Assert|Inline|BytePair|Widening|AdjacentPointer|TestVariant|TestOptionalBackend|TestLLVMMode|TestSingleCall' -count=1` — PASS, 57 test entries; the final test-only extensions were rerun separately and pass.
- Lizp-only assert matrix, baseline and candidate compilers from their own worktrees, `--glob 'examples/lizp/*' -j 4`: 102/102 pass on each backend, 102 negative controls per backend rejected, no newly failing assertions, inventory changes, or unexpected control passes.
- Direct-call judge seeds 0–499, `-j 4`: 500/500 PASS; no mismatches or compiler/assembler/oracle errors.
- Folded judge seeds 0–99, `-j 4`: 100/100 PASS; no mismatches or compiler/assembler/oracle errors.
- Pipeline manifest matches all 888 test entries, with no duplicates, stale entries, or unclassified tests; fast list is sorted.
- Main-suite benchmark: both exact requested runs PASS, baseline in `/mnt/ssd-510/minz/tmp/o9b-origin-main`, outputs `/mnt/ssd-510/minz/bench/o9b-before` and `o9b-after`. All 48 MinZ measurements pass, with identical source/workload hashes and zero regressions in bytes or T-states, including nograce. Default gains remain sum_array 6,145,792 T, bitwise8reg 655,360 T, and fat12_decode 24,384 T.

Each cell is **bytes; T-states**, origin/main → O9b.

| Case | Default | Size | Grace |
|---|---:|---:|---:|
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

Raw logs, commands, receipts, benchmark TSV, and summaries are in
`/mnt/ssd-510/minz/bench/o9b-evidence`; benchmark profiles retain their MIR,
assembly, and per-call measurements. No full pipeline run, heldout suite, larger
fuzz campaign, or full-repository assert matrix was run. No commits were pushed.

EXIT 0
