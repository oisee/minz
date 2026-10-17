# Signed 32-bit ABAP selection input

PARAMETERS TYPE i now keeps all four bytes through both input paths. The
native fallback parses a bounded decimal line in typed Nanz and writes the
parameter only after successful validation. The MZV screen registers the
current i32 value after INITIALIZATION and retrieves i32, rather than u16.
Empty input preserves that current value.

## Input contract

The native console line holds up to 20 bytes. The admitted numeric grammar
allows surrounding ASCII spaces, an optional plus/minus sign and at least
one decimal digit. Every supplied byte participates in validation; tabs,
embedded spaces, nondecimal prefixes and trailing junk are invalid.
The result must be between -2147483648 and 2147483647.

Unsigned magnitude accumulation checks the next digit against the signed
limit before multiplying by ten. INT_MIN therefore works without overflowing
a signed intermediate. A zero-length line keeps the existing parameter;
a nonempty all-space line is invalid. Native fault 4 denotes invalid syntax
and fault 5 denotes an out-of-range value. Both terminate before committing
the parameter or executing START-OF-SELECTION. Catchable SAP exceptions and
selection-screen retry are not implemented by this checkpoint.

MZV independently validates edits using Go's signed 32-bit decimal parser
before committing screen fields. Errors no longer silently retain a default.
The getter's host boundary is retained through constant-call elimination.
The real MZV CLI now supplies the byte-output host used by typed character
and i32 output helpers; those helpers execute ordinary MIR rather than being
replaced with host implementations.

## Native buffer correction

The old BDOS line buffer had 22 initializer bytes but a scalar u8 type. VM
allocation retained the initializer; native emission reserved one byte.
Input consequently overwrote the adjacent error-code global. Its declared
storage is now [u8; 22]. Selection text/header buffers receive array types
matching their allocation sizes too. This fixes storage allocation; complete
character-field input/default/event transport still needs its own audit.

## Evidence

- One complete report, including INITIALIZATION and START-OF-SELECTION, runs
  24 numeric/empty/error inputs on raw VM, final VM and assembled Z80. Tests
  check all four bytes of the parameter and report result, exact output,
  error code, input-call count, outer return address, stack canary and PC/SP.
  The native BDOS model clobbers primary registers to exercise IO saves.
- Four full-width hosted values run through raw/final VM registration,
  show and retrieval; they check INITIALIZATION and exclude fallback input.
- Nine MZV host cases check valid input and explicit refusals.
- Four native Spectrum line-kernel cases check signed input, backspace/delete,
  full-buffer editing and return PC/SP. Echo is replaced by a deliberately
  clobbering stub; these do not establish complete Spectrum report rendering.
- Six real MZV CLI pipe runs verify output/exit status with PATH=/nonexistent.
  Full ABAP, embedded Go-parser and MZV package tests pass.

Numeric input expectations follow the stated adapter grammar and signed i32
bounds. They are not an independent SAP dialog-runtime oracle. Existing
unchanged-report abaplint-runtime reference tests remain in the full ABAP
suite. Native execution here uses the existing called-main contract and BDOS
model; the guarded --runtime=cpm pipeline is still a required follow-up.

## Reproduce

From minzc:

```sh
go test ./pkg/abap -run '^(TestIntegerSelection|TestIntegerInputZXLineAcquisition)' -count=1 -v
go test ./cmd/mzv -run '^TestSelectionHostSigned32Input$' -count=1 -v
go test ./pkg/abap ./pkg/abap/internal/abapparse ./cmd/mzv -count=1
go build -o /tmp/mzv ./cmd/mzv
```

Save this as input.abap:

```abap
REPORT zinput.
PARAMETERS p TYPE i DEFAULT 7.
INITIALIZATION.
  p = -65537.
START-OF-SELECTION.
  WRITE p.
```

```sh
printf '%s\n' '-2147483648' | /tmp/mzv --headless input.abap
```

The output is `-2147483648 `, including the compact formatter's trailing
space. A blank input line prints `-65537 ` instead.

System fields, remaining executable admission, character input transport,
guarded CP/M closure and integration into main remain open.

Follow-up: [character input hardening](2026-10-17-character-input.md) replaces
the temporary character buffers with an owned-field protocol, fixes the
shared-stdin race and makes fallback prompts display current values. The
integer input and compact WRITE contracts remain unchanged.
