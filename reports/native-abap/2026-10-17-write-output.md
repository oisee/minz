# ABAP WRITE: counted text and default templates on Z80

Literal WRITE now emits counted character bytes instead of a zero-terminated
C-string. An embedded NUL no longer truncates the literal. The path also uses
the existing ASCII/16-bit-length admission checks, so direct literal output
cannot bypass the character model's explicit refusals.

Default WRITE string templates now preserve literal text, decode supported
escapes and parse complete interpolation expressions. Numeric interpolation
uses the decimal-value writer without the extra separator of ordinary numeric
WRITE. Fixed TYPE c interpolation trims trailing blanks; owned strings retain
their actual length. Escaped braces, pipe, backslash and newline/tab/carriage
return are literal data. Unsupported escapes, nested templates, formatting
options or expression kinds receive errors.

## Complete-program evidence

Two unchanged reports execute through independent @abaplint/transpiler and
runtime 2.14.8, raw VM, final VM and assembled Z80:

| Report | Observable result |
|---|---|
| [templates](../../minzc/pkg/abap/testdata/write-reference/templates.abap) | `<-65537><A>!`, exact newline/tab/escaped delimiters, then `<-65536>!`; TYPE c storage retains its five bytes while interpolation trims the output |
| [literals](../../minzc/pkg/abap/testdata/write-reference/literals.abap) | `a`, NUL, `b`, escaped apostrophe and backtick text with retained spaces; x remains 65537 |

The references retain SHA256 of the complete unchanged source and exact output
bytes, including NUL. They execute the whole report without injecting FORM
calls or changing input. Native tests run both bare main/HALT and the existing
called-main contract. They check output byte by byte, four-byte x/seen globals,
all five y bytes, zero ABAP fault, memory canary, final PC/SP and the saved
outer return word for called main. These checks are not a general proof of
arbitrary output runtime memory safety or full SAP list formatting.

A quoted-expression adapter probe additionally checks whitespace and a brace
inside a literal. Its scope is the template adapter, separate from the two
complete-program reference comparisons.

## Strict formatting boundary

WRITE consumes a whole operand segment. CURRENCY, TO, NO-SIGN and other
unrepresented additions cannot be interpreted as extra values, including
when an application object has a matching name. Template WIDTH and other
unimplemented formatting options are rejected. Non-ASCII direct and template
literals remain explicit refusals on this byte-character target.

ULINE previously emitted an arbitrary fixed line of seventy dashes. It is now
explicitly refused until list width/horizontal-line layout is implemented.
SKIP/NEW-LINE additions are refused rather than ignored. This checkpoint does
not establish full list positioning, formatting, cursor state or general
SAP-compatible terminal output. The existing bare newline output uses the
target console convention; these full-report gates do not use SKIP/NEW-LINE.

## Reproduce

From minzc:

```sh
MINZ_ABAP_WRITE_ARTIFACTS=/tmp/minz-abap-write-artifacts go test ./pkg/abap -run '^TestWrite' -count=1 -v
go test ./pkg/abap ./pkg/abap/internal/abapparse ./cmd/mzv -count=1
```

Normal compiler and test execution uses the embedded Go parser and needs no
Node/npm. Node is only the isolated independent reference generator, whose
source, versions and resulting JSON are retained with the research artifacts.
The [native tests](../../minzc/pkg/abap/write_test.go) retain source/reference,
actual ASM and binary when the artifact variable is set. Existing character,
owned-string and addressable numeric WRITE gates also pass after the change.
Full ABAP, embedded-parser and MZV package suites pass after the change.
Remaining SQL/resource/runtime admission and main integration remain required.
