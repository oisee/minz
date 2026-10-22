# O9c: register aliases, dead global reads, and spilled-pointer byte loads

The common Z80 emission hook now invalidates physical copy aliases from the
instruction effects table. A write to either byte of a pair invalidates aliases
of that pair as well as the byte; POP, EX, EXX, raw indirect loads, and emitLD8
follow the same rule. Unknown effects discard all aliases. Read-only uses retain
valid aliases. Higher-level move helpers can establish a fresh alias after the
write is emitted. This fixes stale A/E equality after loading E from memory.

Scratch restoration now stages a byte result through alternate AF while popping
its containing pair. Both the independently live/pending main accumulator and
its current flags survive. This covers the spilled-pointer load whose L result
must survive restoring an unrelated live H, without needing another free main
register. The change applies to the shared scratch restoration helper.

After inlining, EliminateDeadGlobalLoads iterates ordinary DSE and unused global
read removal. It proves a direct address of a declared, normally allocated,
non-volatile global and a positive load width within that object. It preserves
volatile fields/objects, absolute-address globals, unknown pointers, sequential
byte metadata, and reads wider than the object. No function or benchmark name
participates in any production matcher.

Files changed:

- `minzc/pkg/mir2/z80codegen.go`: invalidate aliases at instruction emission.
- `minzc/pkg/mir2/z80codegen_muldiv.go`: preserve main AF during byte-result restoration.
- `minzc/pkg/mir2/dead_global_load.go`: bounded ordinary-global dead-read proof and fixpoint cleanup.
- `minzc/pkg/mir2/dead_global_load_test.go`: removable read/expression and preservation guards.
- `minzc/pkg/mir2/z80codegen_alias_write_test.go`: indirect/byte/pair writes, POP/exchanges, and read-only controls.
- `minzc/pkg/pipeline/pipeline.go`: run the cleanup immediately after single-call inlining.
- `minzc/pkg/pipeline/o9c_codegen_test.go`: five C fixtures over byte boundaries, original and retained-copy Nanz products, and a forced spilled-pointer allocation checked in the VM and emulator.
- `minzc/pkg/pipeline/testdata/o9c/`: copied critic fixtures `r139m.c`, `r139f.c`, `r152m.c`, `r152min.f.c`, `r108min.c`, `r19f.nanz`, and `r292f.nanz`. Added the matching MIR2 assertion to the copied r19 fixture; critic originals remain untouched.
- `scripts/ci/pipeline_profiles.json`: all three new pipeline tests in the sorted fast list.
- This report.

Failing-first evidence is in `/mnt/ssd-510/minz/bench/o9c-evidence`:

- `origin-failing-first.log`: the initial TestByteLoadInvalidatesCopyAliasVMZ80 reproduces r139m on origin/main: Z80 returns 61313 instead of 65168; MIR2 is correct. r152m returns 7967 instead of 44718.
- `origin-strengthened-red.log`: the final boundary matrix fails on both hand-inlined C fixtures, and the alias-write unit test rejects stale aliases on all tested mutations.
- `reversed-red.log`: reverse-apply the production patch while retaining the tests. All five C fixtures fail; r19 retains a dead global load, r292 returns 1531 instead of 65268, and the forced spilled-pointer test exposes the unsafe accumulator staging. Restore the exact patch before any green runs.
- `final-reversed-red.log`: retained-copy candidates run first. r19 returns 34103 instead of 34253; r292 returns 1531 instead of 65268. The spilled-pointer fixture copies the right byte but returns 94 instead of 93.
- `origin-scratch-native-red.log`: the same forced spilled-pointer native mismatch on origin/main, independently of dead-load elimination and inlining.
- `pipeline-green.log` and `final-pipeline-green.log`: the new tests plus related byte-load, single-call-inline, byte-pack, widening-sum, pointer, scratch, BDOS, and assertion-receipt tests pass.

All commands use nice level 19, TMPDIR=/mnt/ssd-510/minz/tmp,
GOCACHE=/mnt/ssd-510/minz/gocache, and at most four jobs. The `run-heavy` wrapper
records uptime and waits 120 seconds before retrying if the one-minute load
exceeds 12. Baseline worktree: `/mnt/ssd-510/minz/tmp/o9c-origin-main`, detached
at origin/main `33418f905`.

Critic campaigns use the unmodified read-only g3.py and g2.py generators, six
functions per seed, seeds 1–200 and 1–400 respectively. Both baseline and branch
compile every seed with `--assert-receipt`, using four workers. Both MIR2 and Z80
assertions execute. g3 has 36/36 passing assertions per branch seed; g2 has 24/24.

- g3: branch 200/200 pass, baseline 199/200; baseline seed 38 is repaired.
- g2: branch 400/400 pass, baseline 376/400; all 24 baseline failures are repaired.
- New failures: zero in either campaign.

The task's stated 13/400 baseline count differs from this full independent run.
The actual failing origin/main seeds are **5, 10, 72, 83, 93, 94, 98, 126, 155,
184, 187, 218, 220, 270, 271, 288, 291, 294, 304, 321, 341, 342, 379, 399**.
All failures are Z80 assertion mismatches. `critic-results.json`,
`critic-campaign.log`, the exact runner, generated sources, assembly, and receipts
preserve the comparison. This run checks the baseline even when the branch
passes, rather than conditionally checking only failing branch seeds.

Validation:

- Required command `go test ./pkg/mir2/... ./pkg/hir/... ./pkg/c89/... ./pkg/nanz/... ./pkg/z80asm/... -count=1`: PASS in every package; `units-green.log`.
- Targeted pipeline command `go test ./pkg/pipeline -run 'TestByteLoad|TestSpilledPointerByteLoad|TestInlineDeadGlobalLoad|TestSingleCall|TestWideningByteSum|TestBytePair|TestAdjacentPointer|TestStructMethodPointerAndFieldScratch|TestCPMSourceBDOSCallerLiveState|TestInlineLocalPointer|TestLizpInlineAssertReceipts' -count=1`: PASS, including the final strengthened tests.
- Direct-call judge seeds 0–499, `-j 4 --no-reduce`: 500/500 PASS; no oracle, MIR2, Z80, compiler, or assembler failures.
- Folded judge seeds 0–99, `-j 4 --no-reduce`: 100/100 PASS; no failures.
- Actual Go-listed pipeline inventory: 891 entries, 520 fast; no duplicate, stale, or unclassified tests; fast manifest sorted. Only inventory listing and targeted pipeline execution were used.
- Both requested main-suite benchmark runs: PASS. All 48 MinZ rows (default, size, grace, nograce) have identical input/workload hashes, with zero regressions in bytes or T-states. Outputs: `/mnt/ssd-510/minz/bench/o9c-before` and `o9c-after`; comparison and table in the evidence directory.
- Campaign scope is exactly 200 g3 and 400 g2 seeds. Judge scope is exactly 500 direct-call and 100 folded seeds. No heldout suite, assert_matrix, or full pipeline execution. No push.

Every shell/build/test/benchmark driver and its logs are retained in the evidence
directory. The initial campaign predates a small refactor of the global-size map;
the final compiler is checked against all 600 saved sources again, including its
MIR2/Z80 receipts and byte-for-byte assembly comparison.

Each cell is **bytes; T-states**, origin/main → O9c.

| Case | Default bytes; T | Size bytes; T | Grace bytes; T |
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

The O9b gains in bitwise8reg, fat12_decode, struct_roundtrip, and sum_array are
unchanged by these correctness fixes.

Final compiler confirmation: all 600 saved critic sources pass again, with
identical assembly for every case and complete passing MIR2/Z80 receipts.
See `critic-final.log` and `critic-final-results.json`.

The branch retains all ten prior O9/O9b commits above origin/main `33418f905`.
The final local commit is followed by `git rebase origin/main`; no push is
performed.

EXIT 0

