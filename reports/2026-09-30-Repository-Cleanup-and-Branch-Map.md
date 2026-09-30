# MinZ repository cleanup and branch map — 2026-09-30

Base: `7cee4aae` (the last `master` commit, now `main`). `main-archive` points to that exact commit and retains everything removed from `main` in this cleanup. This report describes the current checkout, not the September audit's older checkout.

## Branches

Fourteen remote branches whose tips were ancestors of the default branch were deleted. GitHub's default branch and the local tracking branch were renamed from `master` to `main` without creating a parallel line of history. Ten side branches remain, plus `main` and `main-archive`.

| Branch | Compared with `main` | Finding | Disposition |
|---|---:|---|---|
| `feat/c89-fatfs` | 1 ahead, 952 behind | Its one patch is equivalent to a patch already in `main`. | Candidate for later deletion after owner review. |
| `feat/c89-frontend` | 1 ahead, 1159 behind | Its one documentation patch is equivalent. | Candidate for later deletion. |
| `feat/lir-z80-hardening` | 22 ahead, 854 behind | All 22 patches are equivalent to patches already in `main`. | Candidate for later deletion. |
| `feat/mir2gpu` | 8 ahead, 291 behind | All eight patches are equivalent. | Candidate for later deletion. |
| `feat/mzd-dynamic-analysis` | 1 ahead, 1150 behind | Adds emulator-based analysis; absent from `main`. | Preserve. Superseded in scope by the rebased branch, but its implementation differs. |
| `feat/mzd-dynamic-rebased` | 3 ahead, 2 behind | Adds `--dynamic`, an observation fix, and 138 lines of tests. | Preserve; review against today's disassembler before integration. |
| `feat/z3-optimal-codegen` | 3 ahead, 776 behind | Changes LIR/WFC behavior and adds paper examples. | Preserve; rebase and test on current LIR before merging. |
| `feature/tap-loader` | 1 ahead, 1598 behind | Adds a TAP loader to `mzrun`; absent from `main`. | Preserve; verify against the existing `mztap` CLI. |
| `research/abi-optimization-paper` | 4 ahead, 1248 behind | Paper v2 plus changes to paper v1. | Preserve until v1 changes are compared with the rescue branch. |
| `research/pfcco-paper-v2` | 1 ahead, 3 behind | Rescues the v2 TeX, PDF, and email. Those three blobs exactly match the older research branch. | Preserve; likely integration source for paper v2. |

The counts are Git ancestry, not file-change counts. “Equivalent” means `git cherry main origin/<branch>` found no unique patch IDs; it does not prove the branch can be deleted without a final review of metadata or tags. No side branch was deleted during this second pass.

## Cross-reference and ownership map

```text
main
├── minzc/                 Go module (go 1.24.0, toolchain go1.24.3)
│   ├── cmd/               compiler, assembler, emulator, disassembler, tools
│   ├── pkg/               production MIR2/Z80 plus experimental targets
│   ├── Makefile           local smoke and test commands
│   └── pkg/*/*_test.go    unit/corpus tests; some depend on optional data
├── examples/              language and target samples; ZVDB is an optional gitlink
├── corpus/intel80tools    optional external PL/M corpus gitlink
├── stdlib/                source libraries used by examples/compiler
├── docs/                  active guides, ADRs, and books
├── reports/               dated evidence; historical measurements need their base SHA
├── research/              papers and experiments, not release gates
├── .github/workflows/     PR CI, releases, benchmark/security/nightly workflows
└── main-archive           pre-trim snapshot of historical material
```

`corpus/intel80tools` and `examples/zvdb-minz` were gitlinks with no `.gitmodules`, so `git submodule status` failed. Both upstream repositories and the pinned commit objects exist. Restoring `.gitmodules` repairs the mapping without removing either optional input. The PL/M corpus test now uses the repository-relative submodule by default and accepts `MINZ_PLM_CORPUS_DIR` for another checkout.

The root `archive/` contained 1,833 tracked paths and `docs/_archive_2025/` contained 250. No active source import depends on them. Only one active report mentions the historical docs directory as a factual location; active references to `archive/` are mostly old cleanup notes, ignores, or comments. Both trees, the stale 2025 `CONTEXT.md`, the old README copy, the unreferenced root `book/` draft, and the machine-specific `minzc/test_no_tree_sitter.sh` were removed from `main`; all remain reachable through `main-archive`. The separate `docs/book/` remains the live book collection.

A simple scan of relative Markdown links in 444 non-archive documentation files found 124 candidate broken links before the trim. Four root README links, two changelog links, and one `CLAUDE.md` archive link were corrected. The same scan now finds 52 candidates in 433 files. Do not treat that count as a strict CI gate yet: Markdown code blocks and versioned history can produce false positives. Keep the remaining links in a repair queue rather than deleting active docs solely because a link is broken.

## CI and protection sequence

The last CI run on `main` failed during setup: Go 1.20/1.21 conflicted with the current Go module, root npm cache/install expected a lockfile and package that are not tracked, and `golangci-lint@latest` rejected the old config. The old Makefile smoke target referenced a missing `hello_clean.minz`; `test-quick` and `test-all` piped test output through filters and could return success on a failing `go test`.

The new `CI / PR gate` uses the module's Go toolchain, builds `mz`, `mza`, and `mze`, runs the locally green MIR2/Z80 assembler/C89/codegen/parser/pipeline/PL-M packages, then compiles and assembles `examples/fibonacci.minz`. The Makefile smoke fixture and exit codes are fixed. This is a **narrow, honest gate**, not a claim that the full suite passes: ABAP, Nanz, VIR, and z80testing have known failures in the broader short run and remain tracked in `BACKLOG.md`.

The first clean GitHub run of the gate [passed on `dc67880a`](https://github.com/oisee/minz/actions/runs/36756578785). A final run is required after any later documentation-only commit before setting the check as mandatory.

The duplicate `Build and Release MinZ` workflow is retained for tags/manual invocation but removed from PR and branch-push triggers; it still needs release-path modernization. Benchmark and security workflows remain available manually, with security also scheduled, while their old PR configurations are repaired. `release.yml` remains the separate tag/manual release workflow; two unindented heredoc bodies that made its YAML invalid were corrected. These workflows are not evidence of a green release pipeline.

The model is the active [open-abap-core PR workflow](https://github.com/open-abap/open-abap-core/blob/main/.github/workflows/test.yml): a small mandatory unit check, with broader integration/performance work explicitly separate. Here, the build/assemble smoke is included in the mandatory gate because it has a verified clean local path. Protect `main` only after the new PR gate succeeds on GitHub and choose exactly its stable check name as required. Enable required PR reviews, block force pushes and deletions, and require up-to-date branches if the repository's collaboration pattern warrants it.

## Ranked follow-up

1. **P0 / quick win:** confirm `CI / PR gate` succeeds on a clean GitHub runner; then make it the required `main` check. A failing run needs a code/config fix before protection.
2. **P0 / foundational:** re-run the broad short suite in a clean checkout, assign owners to ABAP/Nanz/VIR/z80testing failures, and keep their status visible. Add corrected packages to the required gate only after they pass reproducibly.
3. **P1 / quick win (done):** make the optional PL/M corpus test portable and document how to initialize the two submodules.
4. **P1 / foundational:** audit and repair 2026 active-doc links, beginning with top-level status pages and `book/` references; build a parser-aware link checker before making it blocking.
5. **P1 / foundational:** modernize release, benchmark, and security workflows; separate release tests from PR tests and verify tag builds in a dry run.
6. **P2 / review:** resolve the six unique side branches, then delete the four patch-equivalent ones after a final owner check. Do not merge stale branches wholesale.
