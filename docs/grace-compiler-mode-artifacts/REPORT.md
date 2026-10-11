# Atomic measured-mode artifacts and real-program CLI repeats

The measured compiler-mode CLI now stages every payload in a temporary sibling
directory, flushes/closes the files, and publishes the complete evidence with
Linux `renameat2(RENAME_NOREPLACE)`. An existing evidence path, even an empty
directory or symlink, is refused before measurement; a destination created during
staging is also protected by the no-replace rename. Choose a fresh directory for
another run. No user evidence is overwritten.

The optional selected `-o` file is staged in its own parent directory before
publication, then atomically renamed with exactly the winning BIN/ASM bytes.
Existing regular output files can be replaced; symlinks/nonregular output paths
and canonical input/output/evidence ancestor relationships are rejected.
Dependency aliases are checked after the imported source paths have been resolved
and before measurement. Temporary files/directories are cleaned on failures.

Each published path is atomic independently. This is not a transaction spanning
both the directory and `-o`: if the final output rename fails, the complete
evidence directory remains and the command returns an error. File Sync protects
staged content; there is no promise of full filesystem durability across power
loss. The no-replace directory implementation is Linux-specific; other platforms
still build but return an explicit unsupported-publication error.

Regression checks reproduce a conflicting staged path, output staging failure,
existing destination (including empty directory), imported hard link, canonical
parent/child and symlink-parent aliases. Failures preserve the old output and
leave no partial final directory. Actual native CLI tests cover both goals and
BIN/ASM outputs and reject a rerun into existing evidence. Focused checks pass
in 4.223 seconds (`tests.txt`).

## Whole-program CLI repeats

These reuse already published training inputs, not new heldout experiments.
All candidates execute the complete unchanged source, through actual CLI
executables; no weak replacement source or oracle is used.

| Program / mode | Baseline | Winner | Size |
|---|---:|---:|---:|
| HashMap / compiler modes | 26 346 T | 25 378 T, mode 1 | 990 → 981 B |
| Self Lanz parser / existing memory-host selector | 1 016 414 T | 992 049 T, mask 2 | 2 364 → 2 344 B |

HashMap uses `--hir-profile-compiler-modes` with the existing 15-call workload.
Independent Python verifies results, all 82 global bytes and exact access counts
for all four native candidates. The external `.bin` output matches the winner.

The parser requires byte host callbacks and `memory-host-workload/v1`; it cannot
be honestly passed to the generic profile-workload runner. Its existing
`--memory-host-arena-append` CLI repeat verifies all three training inputs against
an independent syntax/parser model, full resource/global state and ordered byte
events. All candidate ASM/BIN and selected BIN are saved here.

This repeat found a separate legacy CLI output bug: `-o parser.bin` actually
contains assembly text, while the evidence directory's `selected.bin` is correct.
The pre-fix output is retained as `parser-output-before-fix.bin`; `audit.py`
asserts the mismatch so it cannot silently become a claimed binary result.
The atomic publication change only affects measured compiler modes; the existing
memory-host selector's publication remains unchanged in this commit.

Frozen executable: `/home/alice/dev/minz-artifacts/grace-checkpoints/mz-compiler-mode-artifacts`,
SHA256 `98cb98b4e3e32f0cc33b991345e84f99d536a2b30ddfd5ae361b5783a5839394`.
The reports include the executable identity, complete candidate artifacts,
CLI logs, independent audit and SHA256 manifest. The independent parser audit
uses the earlier frozen training inputs in `../grace-parser-prospective-heldout`.
