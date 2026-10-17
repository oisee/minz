# ABAP: report and procedure admission; Nanz TUI boundary

The semantic adapter now refuses declarations whose behavior or ABI was
previously omitted. This is part of the native ABAP hardening audit, not
implementation of the dialog runtime.

| Source/contract | Current result |
| --- | --- |
| Plain `REPORT name.` | Admitted |
| REPORT LINE-SIZE, NO STANDARD PAGE HEADING, MESSAGE-ID | Explicit refusal |
| FORM TABLES / RAISING | Explicit refusal at the header section |
| Typed FORM USING / CHANGING references | Existing verified behavior retained |
| FUNCTION CHANGING | Refused: reference ABI not implemented |
| Multiple FUNCTION EXPORTING results | Refused: result ABI not implemented |
| Single scalar FUNCTION result | Existing VM/Z80 boundary test retained |

Five valid source probes use the embedded Go parser and require the adapter's
specific diagnostic. Two semantic-AST probes ensure unsupported function
outputs cannot bypass admission. The function test remains an ABI-boundary
test; it does not establish ordinary source-level CALL FUNCTION support.

Validation on this change:

- Focused admission, reference and scalar-return suite: PASS, 19.900 s.
- Full `pkg/abap`: PASS, 51.990 s.
- Full `cmd/mzv`: PASS, 1.616 s.
- Embedded parser `pkg/abap/internal/abapparse`: PASS, 0.208 s.
- `git diff --check`: PASS.

The first combined invocation accidentally named a nonexistent package
`pkg/abap/nativeparser`, so its aggregate exit status was FAIL despite the
ABAP and MZV suites passing. The actual embedded-parser package was then run
separately and passed. No failing test was skipped or relabelled.

The [PBO/PAI and Nanz TUI design](2026-10-16-pbo-pai-nanz-tui.md) now specifies
ownership, edit/submit dispatch and atomic checked field transport. The TUI
README identifies the historical u16/default-only input and lack of ABAP
handler dispatch. Connecting the Nanz renderer to hardened i32/c fields is
the next dialog extension; it is not yet implemented. SQL admission/error
handling and integration into main remain open in the current ABAP goal.
