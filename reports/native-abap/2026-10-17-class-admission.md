# ABAP class declarations: retain signatures or refuse them

The adapter no longer silently drops method sections, attribute additions or
unmatched implementations. This hardens the concrete scalar method path; it
does not implement a general ABAP object runtime or source-level dispatch.

## Concrete problems closed

An EXPORTING section previously fell through the signature parser's default
branch. CHANGING parameters were recorded but not passed by the method lowerer.
Both now receive an explicit refusal. The semantic-AST type gate enforces this
too, so bypassing source parsing cannot grant the missing ABI.

Class DATA parsing previously stopped at TYPE, losing LENGTH, VALUE and other
additions. It now uses the strict DATA parser. Attribute VALUE initialization
is explicitly refused because instance creation/initialization is not
implemented. Unknown class members, CLASS-DATA, interface declarations,
inheritance and unrepresented definition additions are also refused. Defaults,
OPTIONAL, exception sections and unrepresented method signature additions do
not become stray parameters or disappear.

An IMPLEMENTATION must match a definition and every METHOD body must match a
declared method. Duplicate definitions/members/bodies are diagnosed. A declared
method without a body no longer produces an empty executable function. An
explicitly empty METHOD remains distinct from an absent implementation.
Unsupported executable statements inside methods still propagate their source
locations through the existing admission checks.

## Preserved path and evidence

The supported signature parser retains fully typed IMPORTING scalar arguments
and one RETURNING VALUE(name) TYPE result. CLASS-METHODS stays static. The new
VALUE(a) IMPORTING spelling lowers to the actual scalar argument name, rather
than producing VALUE, parentheses or a phantom parameter.

Fifteen real-source admission probes cover lost initializers, READ-ONLY,
inheritance/FINAL, CLASS-DATA/interfaces, EXPORTING/CHANGING, OPTIONAL/RAISING,
unmatched/duplicate methods/classes and missing bodies. An empty implementation
is checked separately. Four semantic-AST probes check unsupported inheritance,
interface and method output contracts before lowering.

Three new function-boundary calls to a parsed VALUE-importing static method
pass raw VM, final VM and assembled Z80 for INT_MIN, 65537 and INT_MAX. The
existing twelve scalar method/function calls also pass. The shared native
harness checks four-byte observable globals, caller loop index, memory canary,
saved return word and final PC/SP. These remain function-boundary gates: the
harness calls the generated concrete entry; they do not prove ordinary ABAP
method invocation, default-reference IMPORTING aliasing, object construction or
virtual dispatch. Fully typed FORM USING/CHANGING has separate real PERFORM
reference evidence.

## Reproduce

From minzc:

```sh
go test ./pkg/abap -run '^(TestClass|TestScalar|TestUnsupportedStatementsAreNeverDropped)' -count=1 -v
go test ./pkg/abap ./pkg/abap/internal/abapparse ./cmd/mzv -count=1
```

[Class admission tests](../../minzc/pkg/abap/class_admission_test.go),
[adapter](../../minzc/pkg/abap/parse.go) and
[type gate](../../minzc/pkg/abap/type_admission.go) preserve those boundaries.
Full ABAP, embedded-parser and MZV package suites pass after this change.
Remaining output/SQL/runtime admission and main integration are still required.
