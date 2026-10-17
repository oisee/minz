# ABAP SELECT: explicit admission before SQL generation

SELECT no longer discards ORDER BY or unknown trailing clauses. The adapter
refuses syntax whose semantics are absent from its SelectStmt representation,
before emitting an executable module. This closes a concrete loss of ordering;
it does not implement sorting or broader Open SQL compatibility.

The checked boundary refuses ORDER BY (including PRIMARY KEY), GROUP BY,
UP TO, PACKAGE SIZE, FOR ALL ENTRIES, BYPASSING BUFFER, CLIENT SPECIFIED,
USING CLIENT and WITH PRIVILEGED ACCESS. DISTINCT, HAVING, set operations,
OFFSET, CONNECTION and APPENDING are also explicit refusals. Host escapes
are accepted in INTO destinations; host expressions in the SQL expression or
WHERE are refused until value binding is implemented. This prevents forwarding
an unbound @variable to the database as raw SQL.

String-literal tokens remain opaque. Queries with literal 'ORDER BY' or
'DISTINCT' still compile, as does a simple constant WHERE comparison and
case-insensitive @target transport. The clauses are checked before INTO or
WHERE consumes their tokens, so moving a supported-syntax addition between
those sections cannot make it disappear.

Eight actual ABAP source probes exercise the embedded parser and Compile
refusal. Three positive sources protect literal handling and scalar input.
A direct adapter probe verifies refusal of an unknown token after a completed
INTO tuple; the pinned parser already rejects unknown source syntax, so this
probe explicitly covers the adapter's own boundary. One attempted UP TO
fixture had invalid clause order and was removed from the adapter evidence:
its upstream parser refusal says nothing about the new SELECT check.

The SQL transport gates remain separate: existing row-model VM/Z80 tests and
actual-SQLite VM tests cover values and control flow, not all SQL semantics.
Native SQL linkage, NULL conversion, SELECT RETURN cleanup, internal-table
capacity/ALV, richer SQL expressions and remaining class/method admission
still need work. General SQL or SAP-runtime support is not claimed.

## Reproduce

From minzc:

```sh
go test ./pkg/abap -run '^TestSelect' -count=1 -v
go test ./pkg/abap ./pkg/abap/internal/abapparse ./cmd/mzv -count=1
```

[Admission tests](../../minzc/pkg/abap/select_admission_test.go) and
[clause gate](../../minzc/pkg/abap/select_admission.go) are checked in with the
change. Full ABAP, embedded-parser and MZV package suites pass, including existing
row-model native gates and actual-SQLite VM transport tests. Main integration
remains required.
