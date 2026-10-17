# ABAP on MinZ: checked native subset

The ordinary compiler embeds a Go translation of abaplint from abapiti. It
parses ABAP without Node/npm or a Wasm fallback, then feeds the shared
HIR → MIR2 → Z80 pipeline. Parser provenance and reproduction are in
[PROVENANCE.md](../../minzc/pkg/abap/internal/abapparse/PROVENANCE.md).

This is a checked subset, not a complete SAP runtime. The native hardening
work is tracked in [objective and evidence](../../reports/native-abap/objective.md).
The older examples in this directory include experimental syntax and runtime
paths; their presence does not establish that the strict frontend admits them.

## Build and run

From the repository's `minzc` directory:

```sh
go build -o mz ./cmd/minzc
go build -o mzv ./cmd/mzv
./mz pkg/abap/testdata/write-reference/templates.abap -t cpm -o templates.a80
./mzv --headless pkg/abap/testdata/write-reference/templates.abap
```

The source exercises signed 32-bit arithmetic, fixed characters and default
WRITE templates. Generated Z80 execution, including exact output and PC/SP,
is checked by the regression commands below. Running it in MZV is VM execution.

```sh
go test ./pkg/abap/internal/abapparse -count=1
go test ./pkg/abap -run '^(TestCharacterFullReport|TestStringFullReport|TestWriteFullReports)' -count=1 -v
go test ./pkg/abap -run '^TestABAPGuardedCPMFullReports$' -count=1 -v
```

For the two checked numeric guarded CP/M fixtures, use
`-t cpm --runtime=cpm --runtime-stack=1024`. These options do not certify every
reachable character/input/SQL runtime path; unsupported guarded paths refuse
compilation. The guarded native test assembles the actual COM runtime image.

## Current admission

- TYPE i is checked signed i32, with overflow and division rules; TYPE c owns
  fixed-length bytes, TYPE string owns a bounded length/capacity object.
- Fully typed reference FORM USING/CHANGING passes actual object addresses.
  Alias/forwarding/CLEAR gates use real PERFORM calls and unchanged reference
  programs; scalar method harnesses are a separate, narrower ABI boundary.
- IF/ELSEIF, short-circuit expressions, DO/WHILE and early exits have VM/Z80
  evidence. Indexed loops preserve full-width SY-INDEX and caller state.
- Counted WRITE and default templates preserve bytes. General list layout,
  ULINE, unsupported formatting additions and non-ASCII text are refused.
- Scalar SELECT transport has real-SQLite VM tests and a native row model.
  RETURN closes statements in lexical order. Native SQL server linkage is
  separate. INTO TABLE/ALV fixed-slot prototypes are explicitly refused.
- PARAMETERS uses checked i32/fixed-character transport. Connecting the
  [Nanz TUI with PBO/PAI](../../reports/native-abap/2026-10-16-pbo-pai-nanz-tui.md)
  is a design awaiting implementation; general dynpros are not supported.
- Unsupported types, operators, additions, exception blocks and runtime
  namespace collisions produce errors rather than approximate executable code.

Normal compilation never downloads a parser or launches Node. The explicit
ParseNode bridge is retained for research comparison only. A parser refusal
or semantic admission error is a compiler diagnostic; installing Node does
not bypass it. The Go parser is pinned to core 2.120.56, so syntax support is
bounded by that version and its translated implementation.

For per-feature evidence and limits, follow the linked reports in the
[goal record](../../reports/native-abap/objective.md). Historical examples and
[the older book](../../docs/ABAP_on_MinZ_Book.md) are not a current conformance
matrix.
