# Compiler implementation transfer audit

Audit of research `feat/grace-hir-vm` at `fe8ea7ff` against `origin/main` at `56b8aa09`. Read-only compiler audit; files in this report directory are the only changes made by the auditor. The merge base is `4743cb7c`.

The full research diff contains approximately 29,724 files, overwhelmingly accumulated experiment output. A coherent first implementation PR should transfer the compiler delta, input fixtures, related example/stdlib corrections and the four design decisions; leave bulk generated evidence in research. Current main prose/status reports must be retained and reconciled separately.

## Concrete first PR

`first-pr-source-paths.txt` records 884 changed paths under `minzc`, `examples` and `stdlib` (including intended removals). This contains 218 changed production Go files and 589 changed Go tests, plus rules and test data. Add `docs/adr/0044-lir-retirement-and-rule-migration.md`, `0045-raw-pointer-semantics.md`, `0046-optimisation-policy-and-certificates.md`, and `0047-mir-native-candidates-and-offline-vir.md`.

Include the complete package/CLI source delta together. `pipeline.go` and `cmd/minzc/main.go` directly call the added HIR verification/Grace/profile APIs, MIR2 contract and runtime support, checked local tables, and profile/memory-host helpers. Merely omitting feature-specific directories/files does not produce independent buildable slices. Smaller semantic PRs require consciously adapting these central dispatchers and their options/tests, which carries more integration risk than this first landing.

The source removal set is confined to LIR retirement: `minzc/pkg/lir`, old `pkg/pipeline/lir_test.go`, and two HIR LIR judges (one replaced by native lowering tests). Do not restore LIR through a broad main-tree overlay. Conversely do not replace README, language books or main-only status packages wholesale with the older research versions. `inventory.json` lists main-only report files to preserve. There are no research changes to root `.gitignore`, `minzc/go.mod`, or `minzc/go.sum`.

## Assets and fixture preservation

The five `minzc/pkg/hir/rules/*.grace` files are mandatory compile-time embedded assets. MIR2 Grace rules live in Go source. No checked-table JSON or ENRT asset is mandatory for default production compilation: checked local replacement uses explicit table paths and optional SHA256 pins. Legacy VIR table discovery remains external/research behavior; the existing repository `regalloc_table.json` and `minzc/pkg/vir/regalloc_exhaustive.json` are unchanged. Z3 and `/mnt/safe/z80-compiler/tables/*.enr` are external requirements of generation/research tests, not assets to add to the default compiler distribution.

Do not exclude all `docs`/`reports`: imported tests consume frozen inputs and independent expectations from those trees. Static scanning found 233 existing direct literal fixture references and 85 dynamically assembled report/docs bundle roots. `candidate-fixture-paths.txt` is the conservative input set: 746 files, 49,228,079 bytes, comprising direct files, input-like bundle root files, and frozen subtrees. This is the practical initial preservation set for isolated validation; it also contains existing example/stdlib paths that need no duplicate transfer.

`fixture-paths.txt` expands that set by following existing JSON manifest references: 6,936 files, 251,925,143 bytes. Use it when keeping all included manifests referentially complete. The expansion includes generated outputs linked by report SHA manifests and is explicitly conservative; it is not a claim that every path is consumed by normal tests. Copying entire referenced bundles would instead require roughly 17,220 files and 1.25 GB, so it is unnecessary.

Especially necessary fixtures include:

- `docs/grace-real-leaf-local-tables/caller-table.json` and `caller-harness.nanz` for solver-independent checked-table readmission and native caller validation.
- `docs/grace-pgo-scalar-artifacts/workload.json` for invariant, joint-site, rule-choice, weighted-packet and CLI profile tests.
- `docs/grace-parser-prospective-heldout/artifacts/source.nanz`, its frozen identity and training/heldout input records: this immutable baseline deliberately differs from the subsequently corrected current parser example.
- `docs/grace-pfcco-prospective-heldout/artifacts/{training,heldout}-workload.json`, prospective frozen records, and the arithmetic heldout frozen call records.
- `docs/grace-wide-extension/original-zvm.nanz`, original-source snapshots, native expected binary stdout/event/state files, and freeze manifests used by whole-program tests.

Keep frozen source identities and source maps intact. Many tests compare the current example/stdlib source hashes against freezes; transferring fixtures without the corresponding corrected examples and stdlib files can fail identity checks even when Go builds.

Root `.gitignore` globally ignores `*.bin`, `*.a80`, `*.mir` and `*.test`. Already tracked files transfer normally, but newly added input binary fixtures need narrowly scoped exceptions or explicit staging from the inventory. Avoid broad exceptions that reopen generated output collection. Keep binaries that encode expected input/output; do not add test executables. A reproducible fixture archive could be a later cleanup, with a digest-verified extraction helper invoked before tests, but current tests hardcode these repository paths and cannot read an archive directly. Preserve paths for the first PR.

## Review and validation boundary

Run compile-only checks on all affected packages, then HIR/MIR2/VIR and frontend package gates, focused checked-table/PFCCO/Grace composition tests, and CLI gates with fixtures present. Use `scripts/run_cli_process_shards.py` for the large CLI suite if needed; it uses Python standard library, separate processes and temporary directories and generates output only in its explicit output directory. Agon replay/audit scripts belong with optional platform evidence, not the mandatory compiler core.

Do not claim prior research logs validate the new integration commit. Record the integration branch identity and new outputs. A second PR can publish selected evidence packages, the retained research audit tools, explanatory docs and reconciled overview prose after the implementation gates are complete. The existing documentation PR and user-confirmed interactive Tetris run remain separate evidence; the interactive run was reported with score 165 and one completed line.

## Inventory method and limits

`build_inventory.py` reconstructs inventories without modifying source. `inventory.json` contains identities, direct test references, dynamic bundle roots, mandatory assets, source/deletion paths and JSON reference edges. Paths assembled through arbitrary Go expressions cannot be resolved exactly by static scanning; the candidate set conservatively preserves bundle root inputs and frozen subtrees, and runtime failures may reveal additional nested inputs. JSON closure follows existing repository/sibling/bundle files, including path keys in checksum maps; it does not fetch external resources or pretend missing paths exist. Narrow the closure only after isolated integration tests establish those omitted files are unnecessary.
