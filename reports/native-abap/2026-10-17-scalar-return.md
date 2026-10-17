# ABAP scalar RETURN: preserve the result and enclosing index

RETURN and processing-block EXIT now return the current scalar result object
when lowering a function's single EXPORTING result or a method's RETURNING
result. The return context is isolated for each procedure. An early exit from
indexed loops restores the saved indexes before returning the typed result;
a FORM without a scalar result still uses its ordinary reference/void return.

The check also exposed a parser-adapter omission: CLASS-METHODS signatures
were dropped. The pinned parser tokenizes that keyword as CLASS, -, METHODS.
The adapter now normalizes this declaration and preserves its static status.
A static method does not receive a hidden instance pointer, even when its
class also contains instance attributes.

## Evidence and scope

Twelve function-boundary calls pass raw VM, final VM and assembled Z80:

- Early negative RETURN preserves INT_MIN and -65537.
- RETURN from two nested indexed loops produces 65538 and restores the
  harness caller's SY-INDEX=23.
- EXIT outside a method loop behaves as a processing-block return.
- RETURN before assigning the result yields its initial zero value.
- The semantic-AST function path returns full-width INT_MIN, 65537 and INT_MAX,
  rather than executing the assignment after RETURN.

The native harness checks all three four-byte result globals, restored caller
index, return PC/SP, a memory canary and the saved return word. Both scalar
method and scalar function result declarations remain type-checked objects.

These are function-boundary ABI gates, not unchanged complete-program
reference comparisons. The method fixture is parsed from ABAP, then the HIR
harness calls its concrete scalar entry. The function gate supplies the
FunctionDecl semantic AST directly. Ordinary FUNCTION source preprocessing
and source-level class method invocation are separate admission paths; this
report does not claim general CALL FUNCTION, object dispatch or method-call
syntax support. No SAP runtime or exception-unwind claim is made.

The [method source](../../minzc/pkg/abap/testdata/scalar-return/method.abap)
and [harness](../../minzc/pkg/abap/scalar_return_test.go) preserve those
boundaries explicitly. The independent full-report gate remains established
by the earlier numeric, character/string and DATA-scope fixtures.

## Reproduce

From minzc:

```sh
go test ./pkg/abap -run '^TestScalar' -count=1 -v
go test ./pkg/abap ./pkg/abap/internal/abapparse ./cmd/mzv -count=1
```

Set MINZ_ABAP_SQL_ARTIFACTS to retain the harness's actual ASM/BIN (the helper
is shared with SQL transport tests; these scalar cases use no database model).
Final semantic admission, guarded CP/M closure, main integration and the
bounded Nanz TUI connection remain open.
