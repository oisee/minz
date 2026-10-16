# ABAP expressions: preserve the whole expression or refuse it

The previous adapter could reduce an assignment to its first token or first
binary operation, discard the rest of a condition, ignore ELSEIF, and lower
unknown expression kinds to zero or unknown operators to addition. These
paths could accept source that they did not execute faithfully.

The adapter now consumes the complete expression with arithmetic/comparison/
logical precedence, parentheses and unary signs/NOT. AND and OR use the
existing HIR short-circuit lowering and return canonical booleans. It uses a
node's complete token list without duplicating descendant tokens. Unsupported
or incomplete expressions are errors rather than truncated values. Unknown
semantic expression kinds and unary/binary operators are also lowering errors.

ELSEIF chains are retained in order, including the final ELSE. Counted DO
parses its entire count expression. The new native probe found an additional
old execution gap: DO called sy_set_index without an implementation, while
the native sy_get_index stub returned zero. Both now access actual module
storage, verified by summing SY-INDEX inside a three-iteration DO.

## Independent execution evidence

Two unchanged sources and independently generated results live in
[expression-reference](../../minzc/pkg/abap/testdata/expression-reference/).
The reference is @abaplint/transpiler + @abaplint/runtime 2.14.8 using the
[reproducible reference script](../../research/native-abap/reference.mjs).
Each result includes the input SHA256 and provider version.

- precedence.abap checks a multi-operation assignment, parentheses and
  multiplication precedence, NOT/comparison binding, and compound AND/OR.
  Reference globals: x=19, y=6, seen=101.
- control.abap checks a parenthesized DO count, SY-INDEX reads, several
  ELSEIF branches and short-circuit OR whose skipped operand divides by zero.
  Reference globals: x=7, y=9, seen=0.

The test explicitly invokes FORM probe, as does the independent reference.
It compares actual global memory in raw/final MIR VM and assembled Z80,
with native return PC=F100 and SP=F000. A declaration alone does not execute
the FORM. These are function-boundary probes, not full report acceptance.

Admission tests also reject unsupported function calls, exponentiation,
IS INITIAL, malformed/incomplete expressions, numeric literals outside the
current literal parser, and unsupported templates. This does not mean those
ABAP features cannot be added later; they currently have no admitted semantics.

## Remaining scope

This checkpoint preserves expression structure; it does not establish full
ABAP numeric semantics. TYPE i is still represented with the old 16-bit type.
Division rounding, signed DIV/MOD, overflow, complete character/string
storage, remaining executable syntax admission, system-field semantics and
nested-loop SY-INDEX restoration still require hardening. Ordinary WRITE
formatting and all declaration additions are not certified by these probes.
The full goal and main integration remain open.

Validation: the complete ABAP package and native parser parity suite pass.
