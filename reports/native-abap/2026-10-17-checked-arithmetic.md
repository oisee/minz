# ABAP integer arithmetic: checked execution on VM and Z80

This is an intermediate checkpoint in the [native ABAP goal](objective.md).
The ordinary compiler uses the embedded Go parser and needs no Node/npm.
Neither full ABAP compatibility nor completion of the goal is claimed.

## Behavior

ABAP `TYPE i` arithmetic now uses checked signed 32-bit operations. Addition,
subtraction, multiplication and negation detect overflow. `/` rounds halves
away from zero for the admitted integer calculation path; `DIV` and `MOD`
use a nonnegative remainder, including negative operands. Zero divided by
zero returns zero; a nonzero numerator divided by zero stops execution.
These rules follow [ABAP arithmetic operators](https://eduardocopat.github.io/abap-docs/7.54/abenarith_operators/).

The helpers are ordinary Nanz compiled through HIR, MIR2 and Z80. On failure,
`_abap_error_code` becomes 1 for division by zero or 2 for overflow. The VM
raises a halt error; native code disables interrupts and halts. The failing
assignment and subsequent statement do not commit. This is a terminal error:
the active caller stack is retained, not unwound. TRY/CATCH compatibility is
not established by this checkpoint.

## Evidence and practical limits

The [unchanged source](../../minzc/pkg/abap/testdata/arithmetic-reference/arithmetic.abap),
[inputs](../../minzc/pkg/abap/testdata/arithmetic-reference/inputs.json) and
[independent results](../../minzc/pkg/abap/testdata/arithmetic-reference/results.json)
cover 13 division cases. Expected globals come from
`@abaplint/transpiler` and `@abaplint/runtime` 2.14.8 on Node 22.23.3.
The harness injects input after initialization and calls FORM probe: this is
function-boundary evidence, not the required full-report execution gate.

Ten additional fault cases and six valid boundary cases use specification-derived
expectations. The independent runtime has its integer overflow guard disabled,
so it is not the overflow oracle. All 29 cases compare raw/final VM and native
Z80 observable memory. Successful native calls restore PC/SP. Fault checks
verify HALT, disabled interrupts, intact outer return address, a stack canary,
and unchanged destination globals. General mixed-type expressions and chained
calculation contexts still need their own admission/reference coverage.

The arithmetic test assembles once and reuses the image across fresh machines;
its directed run took 0.432 seconds. [Arithmetic log](2026-10-17-arithmetic-artifacts/arithmetic.txt).

## Backend correction found by these probes

A boolean needed by later CFG blocks could be allocated in flags, then lost
when another comparison changed flags. PBQP and greedy allocation now require
a byte home for predicates live across blocks. Local branch predicates can
still use flags. A native regression covers 64 input pairs for each allocator,
including observable writes and restored stack.
[Regression log](2026-10-17-arithmetic-artifacts/cross-block-flags.txt).

The complete HIR, MIR2, ABAP and generated-parser package run passed:
HIR 36.502 s, MIR2 128.377 s, ABAP 9.387 s; parser cached.
[Core log](2026-10-17-arithmetic-artifacts/core.txt).
These are package checks, not a claim that all compiler/frontends passed.

## Next requirements

Owned character/string storage and copying, precise admission of other ABAP
types, full-width selection input, remaining syntax/system-field semantics,
two independent full-report references and main integration remain required.
The [Nanz TUI integration design](2026-10-16-pbo-pai-nanz-tui.md) can reuse the
same HIR/backend, but editable signed-32-bit field transport must be implemented
before connecting ABAP parameter screens. A PBO/PAI-style validation/retry
adapter is the first bounded dialog step, not a full SAP dynpro runtime.
