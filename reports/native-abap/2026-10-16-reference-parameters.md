# ABAP FORM: real reference parameters

`PERFORM bump CHANGING x` now passes the address of x. A FORM sees the incoming value and its writes affect that same object immediately. USING also preserves ABAP reference semantics, including writes and aliasing with CHANGING or global variables. A reference passed to another FORM keeps its original address.

For example, with x=5 and y=9:

```abap
FORM mix USING a TYPE i CHANGING b TYPE i c TYPE i.
  b = 10.
  c = a + 1.
  a = a + 2.
ENDFORM.
PERFORM mix USING x CHANGING x y.
```

Because a and b refer to x, the final state is x=12, y=11. The previous scalar/output promotion could not preserve this relationship.

Six unchanged [reference sources](../../minzc/pkg/abap/testdata/form-reference) were executed independently by `@abaplint/transpiler` and `@abaplint/runtime` 2.14.8. Their JSON results record provider and Node versions, source SHA256 and observed globals. [Regression tests](../../minzc/pkg/abap/reference_params_test.go) compare these values against raw/final MIR VM and assembled Z80, including native return PC/SP. Another native check captures CP/M output `5 6 7 13 ` while preserving live references and local data.

The work also exposed a shared HIR defect: taking the address of a parameter selected a same-name global. Addressable locals now take precedence, with a regression that checks the local result and unchanged global. Main/event lowering errors also propagate instead of dropping the report body.

## Admission and compatibility

Fully typed reference-mode FORM parameters are supported at this checkpoint. Untyped parameters and VALUE/LIKE modes are refused explicitly. A literal USING argument is allowed only after a conservative transitive effect check proves the formal cannot be written. Ordinary variables preserve aliasing.

For existing direct compiler assertion comments, a FORM with one CHANGING parameter also gets a scalar wrapper under its plain name. The wrapper creates separate input/output objects, initializes the output to zero, calls the reference implementation and returns that output. Real PERFORM calls always use the reference entry; the wrapper is an external testing convention.

This is a reference-contract checkpoint, not full ABAP conformance. Its numeric values fit the existing representation. Exact 32-bit signed i semantics, string storage and copying, expression admission and the final main integration gate remain open. Reproduction commands and the independent reference dependency lock are in [research/native-abap](../../research/native-abap/README.md).
