# ABAP SELECT scalar transport and signed operation status

SELECT SINGLE now checks for a row before assigning targets. Missing rows
retain the previous object and yield SY-SUBRC=4; successful transport yields
0. SY-SUBRC is a shared signed four-byte object in native code and MZV, rather
than separate host state and a zero-valued u16 native stub.

The old sqlite_column_int interface returns only u16. ABAP TYPE i now uses
bounded text transport followed by the already checked signed i32 parser.
Character targets receive content copies with padding/truncation; strings
receive owned payloads with capacity checks. Actual FORM reference targets
are written through their passed address, not a newly created local variable.

SELECT headers are collected once instead of concatenating the flat header
with duplicate descendant tokens. Scalar INTO names are case-insensitive and
accept the @ host-object prefix; undeclared targets are diagnosed. Explicit
column/target cardinality is checked. The generated internal-table byte
buffer also now declares its actual array extent instead of a one-byte scalar
with a larger initializer.

## Verified evidence

The test-owned row model supplies the result text and row status. Thirteen
normal cases check raw VM, final VM and assembled Z80:

- 0, 65537, -65537, INT_MAX and INT_MIN;
- a missing row retaining the prior value;
- fixed-character padding, truncation and empty content;
- an owned string and a CHANGING integer target;
- SELECT loops with no row or an explicit EXIT.

Six fault cases cover invalid decimal input, overflow, non-ASCII transport,
more than 255 text bytes, a null text pointer and query failure. Errors stop
before replacing the target. Successful native calls restore PC/SP and retain
canaries and the saved return word. Faults use the existing terminal DI/HALT
contract and retain the active caller stack; this is not SAP exception unwind.

A separate MZV test uses actual SQLite for seven cases: wide positive/negative
values, both i32 boundaries, no row, overflow and NULL. Both raw and final VM
execute the real query/step/column/finalize host functions. Only the open handle
is injected after the test initializes the database. Byte-valued SQLite
columns are returned as their content, rather than a formatted list of bytes.

The assembled Z80 cases use the row model. They do **not** prove native SQL
server or SAP database parity. Existing sql.sqlite native-port linkage, full
SQL statement admission, INTO TABLE/ALV semantics and guarded CP/M closure
still require work. NULL is an explicit unsupported transport fault here;
its ABAP initial-value conversion is not implemented. Strings need sufficient
inferred owned capacity; there is no heap-growth claim.

## Reproduce

From minzc:

```sh
go test ./pkg/abap -run '^TestSQL' -count=1 -v
go test ./cmd/mzv -run '^TestABAPSQLActualSQLiteTransport$' -count=1 -v
go test ./pkg/abap ./pkg/abap/internal/abapparse ./cmd/mzv -count=1
```

Set MINZ_ABAP_SQL_ARTIFACTS to save the actual row-model ASM/BIN. Test sources
and model contracts are in
[sql_scalar_test.go](../../minzc/pkg/abap/sql_scalar_test.go), and actual
SQLite cases are in [abap_sql_test.go](../../minzc/cmd/mzv/abap_sql_test.go).

This checkpoint advances the native ABAP hardening goal. It is not completion
of the TUI adapter, general SQL runtime or integration into main.
