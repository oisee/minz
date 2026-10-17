# ABAP loop index: full-width state and nested exits

DO and WHILE now maintain a signed 32-bit SY-INDEX for each active loop.
Counted DO evaluates its count once; nonpositive counts execute no body.
Each iteration increments the current counter, and leaving the loop restores
the enclosing index. RETURN restores every indexed loop in the current
procedure, while EXIT outside a loop leaves the processing block. CONTINUE
outside a loop is refused. MZV host accessors and native/inlined accessors use
the same four-byte backing object.

## Evidence

Five unchanged complete ABAP reports match independently executed
`@abaplint/transpiler` and `@abaplint/runtime` 2.14.8, raw MIR VM, final MIR VM
and assembled Z80. Checked globals are full-width signed values; the native
call also checks return PC/SP, a memory canary and the saved return word.

| Report | What it exercises | x | y | seen |
|---|---|---:|---:|---:|
| nested | Nested DO and order-sensitive accumulation | 1122 | 1212 | 0 |
| count-snapshot | Count mutation, WHILE and CONTINUE | 123123 | 3 | 2 |
| unbounded | Unbounded DO/WHILE and EXIT | 123 | 123 | 0 |
| limit | Zero/negative counts and INT_MAX with early EXIT | 1 | 0 | 11 |
| wide | All 65,537 iterations actually execute | 65537 | 65537 | 0 |

Sources and source-hashed reference results are in
[minzc/pkg/abap/testdata/loop-reference](../../minzc/pkg/abap/testdata/loop-reference).
The independent runner is
[research/native-abap/loop-reference.mjs](../../research/native-abap/loop-reference.mjs).
Node is used only to regenerate research expectations; normal tests and
compilation use the embedded Go parser.

Three additional real MZV CLI runs append WRITE statements to observe the
nested, count-snapshot and wide results. These are derived CLI probes, not
the unchanged independent-reference sources. They run with PATH=/nonexistent.

## A reference limitation

A separate directed probe returns from two nested loops inside a FORM called
by an outer loop. Our result is x=1122, y=1; provider 2.14.8 gives x=1121, y=1.
Inspection shows its RETURN emitter restores only the nearest loop backup.
This probe is therefore a rule-based regression gate, not a provider-parity
claim. The current enclosing caller loop must still expose its current pass.

The loop rules follow the ABAP documentation for
[DO](https://eduardocopat.github.io/abap-docs/7.54/abapdo/) and
[WHILE](https://eduardocopat.github.io/abap-docs/7.54/abapwhile/): SY-INDEX
identifies the current loop's pass, and counted DO determines its count once.
Restoration outside all loops is also compared with the independent provider;
these documents alone are not cited as a complete outside-loop specification.

## Admission and remaining work

SY-DATUM, SY-UZEIT, SY-UCOMM, SY-TABIX and unknown system fields now produce
explicit diagnostics instead of approximate zero-valued accessors. SY-SUBRC
still requires its operation/SQL status audit; its legacy admission is not
covered by this loop result. Procedure-local DATA initialization inside loops
and scalar method RETURN also remain separate semantic work.

Nanz TUI connection is the next bounded dialog extension described in the
[updated design](2026-10-16-pbo-pai-nanz-tui.md). This checkpoint neither
implements dynpros nor completes guarded CP/M closure or main integration.

## Reproduce

From minzc:

```sh
go test ./pkg/abap -run '^(TestIndexedLoopFullReportReference|TestIndexedLoopReturnAndProcessingBlockExit|TestSystemFieldAdmission)$' -count=1 -v
go test ./pkg/abap ./pkg/abap/internal/abapparse ./cmd/mzv -count=1
```
