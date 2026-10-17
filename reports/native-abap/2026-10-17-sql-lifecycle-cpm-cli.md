# ABAP SELECT lifetime and guarded CP/M through the CLI

RETURN now closes active SELECT statements before leaving a processing block.
The lowerer maintains one lexical cleanup stack for SQL cursors and indexed
DO/WHILE state. Reverse-order cleanup preserves nesting: SELECT inside DO
closes before the DO index is restored; DO inside SELECT restores its index
before the enclosing SELECT closes. Entering a FORM/method isolates this
stack, so a callee RETURN cannot close the caller's statement or unwind its
loop. EXIT still uses the nearest loop's normal epilogue; CONTINUE keeps the
cursor open and advances through the loop condition.

## Runtime evidence

Ten source-driven row-model scenarios pass raw VM, final VM and assembled
Z80: processing-block and top-level RETURN, nested SELECT, interleaved indexed
loops, callee/caller isolation, CONTINUE and normal exhaustion. Distinct
statement handles log exact close order; a second trace records SY-INDEX at
each close. The native harness checks observable globals, saved return word,
canary and final PC/SP. This is compiler resource/control-flow evidence,
separate from execution against a native SQL server.

Four further scenarios execute through the actual SQLite host on both VMs.
They retain the real query/step/column/finalize implementation and record
calls to finalize: nested exits close handle 2 before handle 1, while CONTINUE
finishes normally. Database setup supplies three identical 65537 rows; it does
not replace SQL query execution with a model.

This actual-host gate exposed a separate bug: SQL's pooled connections gave
each `:memory:` connection a different database. The first SELECT read t;
the nested SELECT failed with `no such table: t`. Each logical sqlite_open now
uses a unique shared-cache in-memory URI so simultaneous cursors see one
logical database. A dedicated test opens multiple handles in one VM and
another VM, holds two cursors per handle concurrently and verifies isolated
contents across all of them. No external database dependency was added.

Terminal arithmetic/conversion faults remain terminal, with no exception
unwinding claim. Native SQL server linkage, SQL error/status handling beyond
the admitted scalar transport, and general database lifetime remain separate.

## Precise table boundary

The old INTO TABLE/ALV prototype hard-coded a database handle, eight rows and
fixed-width text slots. It could truncate valid results while appearing to
implement an ABAP internal table. The strict frontend now refuses those
paths, including through the semantic AST. Scalar SELECT remains available.
Typed/internal-table capacity and proper ALV semantics are follow-up work;
the prototype is not silently treated as reliable support.

## Guarded CP/M CLI

The backend API already admitted the two numeric guarded reports, but the CLI
allowed only Nanz/Pascal for this runtime. ABAP now reaches the same backend
validator; no ASM, pointer, stack or alternate-mode restrictions were removed.

Two unchanged numeric reports compile through the CLI to COM and ASM. Each
COM is byte-identical to assembling the emitted ASM. Native execution matches
the independent reference's three signed four-byte globals, preserves final
PC/SP and the saved return word, and checks every byte outside the image
except the permitted outer return word. The existing unsupported character
pointer path still refuses and leaves no output image. Nanz/Pascal runtime,
byte-primitive and overflow/flags CLI checks also pass.

Actual rebuilt binaries additionally compile the template report, run it
through MZV with exact reference output, and emit a numeric guarded COM with
`PATH=/nonexistent`. The [current guide](../../examples/abap/README.md) contains
the verified commands. They use `-t cpm`; the compiler's default target is
Spectrum. The older book is marked historical to prevent its Wasm/u16/TUI
claims from being mistaken for this implementation.

## Reproduce

From minzc:

```sh
go test ./pkg/abap -run '^(TestSQLLexicalCleanup|TestSQLPrototypeTables)' -count=1 -v
go test ./cmd/mzv -run '^(TestABAPSQLActualSQLiteCleanup|TestSQLiteMemoryHandles)' -count=1 -v
go test ./cmd/minzc -run '^(TestABAPCPMOrdinaryCLI|TestCPMRuntime|TestPascalCPM)' -count=1 -v
go test ./pkg/abap ./pkg/abap/internal/abapparse ./cmd/mzv -count=1
```

Full ABAP, embedded-parser and MZV packages pass, plus the relevant compiler
CLI runtime gates. Source-driven row-model evidence is in
[sql_lifecycle_test.go](../../minzc/pkg/abap/sql_lifecycle_test.go); actual
SQLite gates in [abap_sql_test.go](../../minzc/cmd/mzv/abap_sql_test.go) and
[sqlite_memory_test.go](../../minzc/cmd/mzv/sqlite_memory_test.go); CLI/native
COM evidence in [cpm_abap_cli_test.go](../../minzc/cmd/minzc/cpm_abap_cli_test.go).
Main integration and final admission audit remain required.
