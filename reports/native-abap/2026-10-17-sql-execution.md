# Checked SQL execution in the native ABAP frontend

`EXEC SQL` now preserves its complete structured body. Previously the adapter
stopped at the header's period and discarded the request. The adapter now
consumes header, NativeSQL nodes and ENDEXEC together, preserving adjacent
tokens even across parser nodes: `1.5` stays `1.5`, and literal contents such
as `'SQL: EXEC.'` remain data. Native host bindings receive an explicit refusal.

Seed statements from `*!sql` now call the ordinary typed runtime with the
actual opened handle. The old frontend-generated ASM wrapper always used
handle 1. The replacement is checked Nanz code, lowered through HIR/MIR/Z80:

```nanz
fun _abap_sql_exec(db: u16, sql: ^u8) -> void {
    if sqlite_exec(db, sql) != 0 { _abap_fault(6) }
}

fun _abap_sql_step(stmt: u16) -> u8 {
    var rc: u8 = sqlite_step(stmt)
    if rc > 1 { _abap_fault(6) }
    return rc
}
```

Opening handle zero is also a fault. SQL text must fit the 16-bit ASCII
NUL-terminated transport; embedded NUL and non-ASCII are refused for seed,
native-exec and generated SELECT strings. Terminal faults intentionally do
not implement exception unwinding or automatically close an active statement.

The MZV SQLite host reports step results as 1=row, 0=done, 2=error. It checks
Columns, Scan, iterator Err and Close, discards failed cursors and exposes no
partial row. An invalid/closed cursor is an error. Actual SQLite tests produce
one successful row followed by an integer-overflow error in the iterator;
the second step returns 2, rather than claiming normal exhaustion.

Classic unescaped WHERE operands need ABAP host binding. Sending `WHERE n = x`
unchanged to SQLite could read database column x instead of program object x.
The adapter now refuses such RHS operands, range operands and predicate
expressions. Literal comparisons remain admitted. This is a bounded SQLite
adapter, not general SAP Open SQL or a native database-server implementation.

## Evidence

- `TestSQLSeedAndNativeBlockUseOpenedHandle`: raw/final VM and assembled Z80
  use handle 17 for seed, native execution and query. Two executions are
  observed; scalar value, finalize count, memory and PC/SP are checked.
- `TestABAPSQLSeedAndNativeBlockActualSQLite`: occupy handle 1, then run the
  whole report against actual SQLite handle 2. Seed creates the table,
  EXEC SQL inserts 65537 and SELECT reads it in both VMs.
- `TestSQLFailuresDoNotLookLikeSuccess`: open=0, exec=1 and step=2 cause fault 6
  on both VMs and assembled Z80 before subsequent application writes.
- Literal/adjacency, native host binding, NUL/ASCII text and WHERE admission
  probes exercise the real embedded-parser path or explicit semantic API.
- Existing scalar transport and lexical cursor-cleanup tests remain intact.

The combined ABAP/parser/MZV core passed in 53.812 / 0.202 / 1.754 seconds.
Later predicate/text refinements have separate focused passing checks; the
final integration core is recorded in the goal readiness report.

Initial directed probes caught duplicate header token collection and a
newline inserted inside a decimal literal; both failures were corrected.
No native database server is used by the row-model tests. Actual SQLite
coverage is VM-host evidence; these scopes are kept distinct.
