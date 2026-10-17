# ABAP declarations: refuse approximations before lowering

This checkpoint strengthens the [native ABAP objective](objective.md); it
is not completion of the numeric/character or full-program gates.

## What changed

Declaration admission now runs before runtime and ABI generation. The current
adapter admits declared `i`, `c` and `string` types. Previously `int8` became
u8, `f`/`p` became u16, and unknown names fell back to u16. These types now
produce explicit errors, as do `n`, `x`, `d`, `t` and other types whose exact
semantics have not been implemented. This is an intentional compatibility
break for programs that previously compiled with approximate storage.

The check visits report globals, parameters, FORM/function parameters,
class attributes/method signatures, events and nested control-flow bodies.
It also applies to direct semantic-AST lowering, not just the Go parser path.
It covers types represented in that AST; it does not certify signature details
that the adapter has not yet preserved.

DATA/PARAMETERS parsing now consumes only represented TYPE, LENGTH and
VALUE/DEFAULT additions. LIKE, DECIMALS, OBLIGATORY, AS CHECKBOX and other
unimplemented additions are errors. Duplicate/missing additions, zero or
nonliteral LENGTH and LENGTH on i/string are refused. VALUE IS INITIAL is
retained as the type-specific default rather than parsed as an expression.
Unspecified elementary DATA defaults to c of length one; explicit numeric
PARAMETERS no longer retain the implicit character length.

## Verification

Seventeen real-source refusal cases include nested declarations and reference
FORM parameters. Additional token-level malformed-addition probes ensure no
ignored tail produces a semantic declaration. A positive source checks default
character type, VALUE IS INITIAL and signed numeric parameter initialization.
[Directed log](2026-10-17-type-artifacts/admission.txt).

Full ABAP package tests passed in 15.640 seconds; embedded parser tests passed
in 0.197 seconds, both uncached.
[Package log](2026-10-17-type-artifacts/core.txt).
No backend algorithm changed in this checkpoint. Existing arithmetic and
reference probes remain part of the ABAP package run.

## Still required

Admitting c/string declarations does not establish their storage/copy semantics.
Owned storage, assignment/truncation/padding, CLEAR and reference aliases must
be implemented and verified before the goal is complete. Full-width input,
system fields, remaining executable admission, two independent full reports
and main integration also remain required. Unsupported numeric types can be
added later only with exact representations and tests, not fallback mappings.
