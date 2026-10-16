# Aggregate scalarization: initial audit

Base main 548ee6ce; work branch feat/c89-good-enough. This is an unfinished
checkpoint, not a completed coverage claim or a main-ready change.

Two checked-in strict-C89 fixtures are accepted by:

```sh
cc -std=c89 -pedantic-errors -fsyntax-only minzc/pkg/c89/testdata/aggregate_scalarization/*.c
```

The first exploratory fixtures used variable aggregate initializers, a C99
extension rather than strict C89. They are archived separately; the committed
fixtures use constant initializers followed by field assignments.

## Reproduced errors

- Strict mutated_return: main native Z80 check(0) returns 0 instead of 513.
  The first extension-based probe returned 1 instead of 513 on raw VM and Z80.
  Both derive from promotion replaying initializers instead of reading the
  object's current values at return.
- make_pair → consume(pair): promotion deletes the aggregate declaration but
  leaves a whole-object argument. HIR Verify reports unknown variable p before
  execution. This is a lowering/ABI error, not a claimed observed native result.
- The promised struct-argument decomposition is described in promote.go's
  header but not implemented: consume still takes one pointer-shaped parameter.
  A separate structural ABI test requires two scalar arguments.

## Partial correctness change and remaining work

Returning a declared object now emits its current field values. The strict
mutation regression passes nine values on raw VM and default/Grace/Size final
VM and native Z80: 27 native calls and 36 VM calls.

Retaining the object exposes an allocation-lifetime problem in the existing
MinmaxNestedStructReturnVMZ80 repeated-call test: raw VM exhausts its 64-KB heap.
The other pending aggregate roundtrip and ABI tests remain red. Do not alter
oracles, hide these failures or declare the branch ready. The next change must
scalarize eligible aggregate storage/arguments, retaining necessary materialized
objects at escape/volatile/indirect boundaries and fixing repeated-call lifetime.

Artifacts, failed probes and corrected syntax checks are saved locally under
/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/c89-good-enough/.
