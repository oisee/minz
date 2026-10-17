# Owned ABAP character fields through selection input

PARAMETERS TYPE c now passes the actual object address and its full u16
length to the screen adapter. Both native input and MZV edit that object;
the old temporary C-string transport no longer replaces changes made in
INITIALIZATION. Empty input retains the current field. Nonempty input copies
content, truncates to field length and pads the remainder with spaces.

The new sel_register_c boundary carries name, length and object address.
It reads fixed-length content, so embedded NUL bytes are preserved and a
300-byte field does not become a 44-byte field. MZV writes exactly the owned
object's length, without adding a NUL terminator outside it. Existing
sel_register_str remains a separate legacy buffer protocol for other callers.

Native acquisition reuses the bounded 20-byte console line from the integer
input checkpoint. Typed Nanz validates the full supplied payload as ASCII
before writing a replacement, then calls the existing character-copy helper.
Non-ASCII input produces terminal fault 4 without committing the field or
entering report validation/execution events. Native line length and field
storage length are separate: large fields can retain and display their full
contents even though a console edit supplies at most 20 bytes.

Fallback prompts now display the current INITIALIZATION value, trimming only
trailing spaces from the displayed character field. The numeric formatter
has a reusable no-trailing-space helper for prompt brackets; ordinary WRITE
retains its existing compact output convention.

## A real stdin race, found by the CLI check

MZV already has a single goroutine reading stdin into a shared channel for
ports, keyboard and TUI hosts. Selection input previously read os.Stdin
independently. Small programs sometimes consumed input first; a 300-byte
INITIALIZATION gave the goroutine enough time to take the bytes, leaving the
screen with its default value.

Selection input now consumes that shared channel. Its buffered reader fetches
one channel byte at a time, so it does not steal a transcript's later program
input. The same reader handles all selection fields. Interactive selection
keys assemble fragmented CSI sequences from the channel with a short timeout
for bare ESC. This is input plumbing, not implementation of ABAP PBO/PAI.

## Verification

- Eighteen full-report cases run raw VM, final VM and assembled Z80: implicit
  character length 1, lengths 3 and 300; empty input, short/long text, spaces,
  embedded NUL and invalid non-ASCII. Tests check the owned field, the snapshot
  taken by AT SELECTION-SCREEN, START-OF-SELECTION progress, exact output,
  fault code, hostile BDOS register clobbers, stack canary and return PC/SP.
- Twelve real MZV host cases check copying and refusal at address zero, with
  a guard byte immediately after the object.
- A two-field shared-channel transcript checks text and i32 transport and
  leaves a later program-input byte untouched. Arrow/F8/bare-ESC/TAB key
  assembly has five directed cases.
- Twelve real MZV CLI pipe runs cover lengths 3 and 300, including NUL and
  explicit rejection. They run with PATH=/nonexistent.
- The full ABAP, embedded Go-parser and MZV packages pass, including existing
  unchanged-program independent-reference and native arithmetic/output tests.

The new input expectations follow the adapter's explicit byte-character and
empty-input contracts. They do not claim parity with a complete SAP dialog
runtime. Native report execution here uses called-main plus the BDOS model;
guarded --runtime=cpm remains required before main integration.

## Reproduce

From minzc:

```sh
go test ./pkg/abap -run '^TestCharacterSelectionInputVMAndZ80$' -count=1 -v
go test ./cmd/mzv -count=1 -v
go test ./pkg/abap ./pkg/abap/internal/abapparse ./cmd/mzv -count=1
go build -o /tmp/mzv ./cmd/mzv
```

Save as input.abap:

```abap
REPORT zinput.
PARAMETERS p TYPE c LENGTH 3 DEFAULT 'old'.
INITIALIZATION.
  p = 'XYZ'.
START-OF-SELECTION.
  WRITE p.
```

```sh
printf '\n' | /tmp/mzv --headless input.abap
printf 'A\n' | /tmp/mzv --headless input.abap
```

The first prints `XYZ`; the second prints `A` followed by two spaces.
System fields, remaining operator admission, guarded CP/M closure and main
integration remain required by the overall goal.
