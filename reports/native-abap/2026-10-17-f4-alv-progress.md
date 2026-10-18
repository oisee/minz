# F4 → CSV → ALV: continuation checkpoint

The full objective is unchanged: one unchanged ABAP report selects FLEET.CSV
through F4 and CL_GUI_FRONTEND_SERVICES, uploads rows, constructs a typed internal
table, and displays it through a limited cl_salv_table on real CP/M and Agon
paths. Independent semantics, VM/native execution, actual captures and locally
saved evidence are required. ZX is optional. DATASET is not the selected design.

## Internal-table review follow-up

The previous critic stopped without a final verdict. Its log and fresh probes
identified capacity multiplication overflow and missing component actuals in
PERFORM. The capacity check now uses a division bound before multiplication;
tests cover eight ordinary and overflowing capacities, including exactly 32768
bytes and a seven-byte row's rounded limit. Reverting the guard reproduces the
panic/incorrect admission, restoring it passes.

PERFORM USING/CHANGING now admits flat TYPE i / TYPE c components by reference,
including repeated actual aliases and forwarding through another FORM. Unknown,
missing, nested, table-body and incompatible component actuals are refused.
Two unchanged component/APPEND reports now have independently regenerated
abaplint transpiler/runtime 2.14.8 fixtures, including source SHA and table images.
A same-family sol-high critic returned MERGE GO for both fixes after rerunning
regressions and six additional literal/admission probes. Integration gates passed: build, ABAP/parser/MZV/HIR/MIR2 packages, ordinary
CP/M CLI and the fast pipeline (438 selected tests, three shards, all exit0).
The newly frozen fixtures also pass a final directed run. This note does not
claim main has merged the slice.

## Measured platform boundaries

A fresh real-CLI replay at f4099ca38 produces fleet.com of 25,588 bytes, occupying
[0x0100,0x64F4). A Z80 emulator enters at the actual COM origin with synthetic
BDOS2, reproduces the frozen fleet output, and observes 62 application stack
bytes. It ends in HALT, not an OS return. This is target-origin semantic evidence,
not proof of guarded-runtime admission or deployment on arbitrary real CP/M.

`-t cpm --runtime=cpm --runtime-stack=1024` rejects retained sel_show ASM. The
existing guarded runtime certifies all retained ASM before closure pruning;
reachable output ASM retains unused injected helpers. Do not fix this by giving
unrecognised assembly authority based only on its name or Internal flag.

Agon target-only assembly has a measured 69-byte relocation mismatch: ship is
addressed at 0x046E9C but the prepended header physically moves it to 0x046EE1.
It also contains 14 CP/M CALL 5 instructions. Explicit `-t agon -b ez80` instead
refuses the report at a wide-memory value-home check. A MOS header and a plausible
size therefore do not establish an Agon execution result.

These runtime/backend limitations precede the internal-table slice. They remain
required work for the full F4/ALV objective and are not declared solved here.
See [placement evidence](2026-10-17-f4-alv-artifacts/placement-review.txt) and
[CLI replay](2026-10-17-f4-alv-artifacts/placement-cli.jsonl).

## Remaining sequence and completion evidence

1. Close internal-table review, final gates, exact reviewed SHA integration.
2. Implement frontend-services + SPLIT + F4 on CP/M from the existing v2 spec;
   review reused WIP individually, discard its DATASET path. Require real files,
   cancel preservation, missing-file status, overflow fault7 and red mutants.
3. Implement limited SALV factory/display with compile-time columns and Nanz TUI;
   verify the unchanged fleet report and rejected unsupported additions.
4. Establish correct CP/M startup/OS return and sound runtime dependencies.
5. Establish Agon relocation, MOS console/files, eZ80 values/memory and native run
   of the same report. Do not substitute a host VM result for Agon execution.
6. Record real screenshots/GIF, label speed changes, and preserve source, input
   CSV, binary, invocation, measurements and checksums locally.

No PR, merge, ALV implementation or native Agon success is claimed by this checkpoint.
