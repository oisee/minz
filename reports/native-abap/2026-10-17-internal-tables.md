# ABAP flat structures and standard internal tables (no SELECT)

The ABAP frontend now admits flat structure types, structure variables and
fixed-capacity standard internal tables. They are ordinary compiled code:
HIR, MIR2 and Z80, with no second interpreter. The old fixed-eight-row
`SELECT ... INTO TABLE` prototype stays refused.

## Admitted

- `TYPES: BEGIN OF ty, ... END OF ty.` at report level, with flat
  components `TYPE i` (signed 32-bit) and `TYPE c [LENGTH n]`.
- `DATA ls TYPE ty.` report structure variables. `ls-comp` is readable and
  writable wherever an elementary operand of that type is admitted:
  assignment, arithmetic, comparison, `IF`/`WHILE`/`DO` conditions, `WRITE`
  and string templates. `CLEAR ls`, `CLEAR ls-comp` and the same-type MOVE
  `ls2 = ls1` are also admitted. Character components pad and truncate like
  existing `TYPE c` assignment.
- `DATA lt TYPE [STANDARD] TABLE OF ty [WITH DEFAULT KEY | WITH EMPTY KEY]
  [INITIAL SIZE n].` `TYPE TABLE OF` is the ABAP spelling of the same
  standard table and is accepted as such.
- `APPEND ls TO lt`, `APPEND INITIAL LINE TO lt` (both set SY-TABIX to the
  new line number), `LOOP AT lt INTO ls ... ENDLOOP`,
  `READ TABLE lt INTO ls INDEX n`, `lines( lt )`,
  `DESCRIBE TABLE lt LINES n`, and `CLEAR lt` / `REFRESH lt`.
- `SY-TABIX` is a readable signed 32-bit system field. MZV host accessors use
  the same four-byte backing object as compiled code.

## Storage and semantics

A structure is a byte image of its components in declaration order without
padding: `TYPE c` bytes, then `TYPE i` as four little-endian bytes. A table is
an array of `capacity` line images plus a `TYPE i` line count
(`_itab_count_<name>`). The capacity is a compile-time fact: `INITIAL SIZE n`,
or 16 when INITIAL SIZE is not given. One table may use at most 32768 bytes. Unused
slots, and slots after `CLEAR`/`REFRESH`, have unspecified content. They are
never observable.

This is a deliberate difference from SAP, where INITIAL SIZE is only an
allocation hint. In MinZ, an APPEND to a full table is terminal runtime fault
**7** (`_abap_error_code = 7`, then halt). The capacity check runs before the
count or any table byte is written. Existing codes 1 to 6 keep their meaning.

The following behaviour matches the reference runtime's `loop`, `append` and
`read_table` statements:

- LOOP INTO copies each line into the work area, so the work area never
  aliases a line. The loop end is the current line count at every step, so
  lines appended inside the loop, including to the looped table, are
  processed. The capacity bound still applies.
- In the body, SY-TABIX is the row number. On ENDLOOP, EXIT and RETURN
  (including RETURN from a FORM nested in an outer loop), SY-TABIX goes back
  to its value before the loop. RETURN also unwinds enclosing DO/WHILE/LOOP
  state in reverse nesting order.
- After the loop, SY-SUBRC is 0 if at least one line was processed and 4
  otherwise. An empty table leaves SY-TABIX unchanged.
- READ TABLE INDEX n with 1 <= n <= lines gives SY-SUBRC 0 and SY-TABIX n,
  and copies the line. Any other n gives SY-SUBRC 4 and SY-TABIX 0, and the
  work area is unchanged.
- DESCRIBE TABLE ... LINES also sets SY-TFILL in SAP. SY-TFILL is not an
  admitted field, so that effect cannot be observed.

## Refused (explicit diagnostics, real embedded-parser path)

Nested structures, deep components (`string`, tables, references),
`INCLUDE TYPE`, non-structure `TYPES`, `TYPES` inside procedures or control
blocks, structure `VALUE`/`LIKE`, procedure-local structure or table DATA,
structure/table FORM parameters and class attributes, SORTED/HASHED/generic
tables, `WITH [NON-]UNIQUE KEY`/`WITH KEY` and other keys, `WITH HEADER LINE`,
non-positive or oversized capacity, and tables of elementary or unknown line
types. Also refused: `LOOP ... WHERE/ASSIGNING/REFERENCE INTO/FROM/TO/USING
KEY/GROUP BY/TRANSPORTING`, inline `INTO DATA( )`, LOOP without INTO, MODIFY,
DELETE, INSERT, SORT, COLLECT, FIELD-SYMBOLS/ASSIGN, FREE,
MOVE-CORRESPONDING, `APPEND LINES OF/ASSIGNING/REFERENCE INTO/SORTED BY`,
`READ TABLE ... WITH [TABLE] KEY/BINARY SEARCH/ASSIGNING/REFERENCE
INTO/TRANSPORTING/COMPARING`, READ without INDEX or INTO, DESCRIBE without
LINES, and other DESCRIBE forms. Whole structures or tables used as
elementary operands, mismatched work-area or MOVE types, unknown components,
and built-in functions other than `lines( )` are refused too. Each refusal is a
case in `TestInternalTableAdmission`, which checks the diagnostic text.

## Evidence

Two unchanged whole reports match independent `@abaplint/transpiler` and
`@abaplint/runtime` 2.14.8 results on raw MIR VM, final MIR VM, and assembled
Z80 in both bare-HALT and called-main modes. The comparison covers exact
output bytes, every report object image (structures, scalar globals, table
lines and count), SY-TABIX/SUBRC/INDEX, the fault code, PC/SP, the memory
canary and the saved return word.

| Report | Exercises |
|---|---|
| [fleet](../../minzc/pkg/abap/testdata/itab-reference/fleet.abap) | APPEND of 3 ships with SY-TABIX, LOOP with running total, READ INDEX 2 / INDEX 9 (subrc 4, tabix 0), `lines( )`, DESCRIBE, nested LOOP with outer SY-TABIX after the inner loop, CLEAR then empty LOOP (subrc 4), truncating component assignment, final 2-line table |
| [ledger](../../minzc/pkg/abap/testdata/itab-reference/ledger.abap) | APPEND INITIAL LINE, APPEND to the looped table inside its LOOP (processed), CONTINUE/EXIT, DO inside LOOP, templates of components, work-area changes not reaching the table, RETURN from a FORM loop inside an outer loop, structure MOVE, CLEAR component, REFRESH and re-APPEND |

The expected byte images come from the reference values and the unchanged
source declarations, not from MinZ.

Directed tests:

- `TestInternalTableCapacityFault` uses a **specification oracle**, because
  the reference runtime grows tables without bound. INITIAL SIZE 2 and three
  APPENDs give fault 7 on all engines. The first two lines, count 2, the
  pre-fault state and SY-TABIX 2 are intact. The bytes a third line would
  occupy either keep their pre-run image or belong to an exactly checked
  object.
- `TestInternalTableLoopIntoCopies` and
  `TestInternalTableTabixAcrossProcedures` use inline expectations. Their
  values were cross-checked with the same reference runner.
- `cmd/mzv` `TestABAPInternalTableTabixHost` runs the MZV host accessors.

Mutants. Each one made the named tests fail and was then reverted:

- M1: no capacity check. CapacityFault fails on all four engines. On native
  code the third line overwrote the line count.
- M2: LOOP INTO aliases the line. LoopIntoCopies and both reference reports
  fail.
- M3: no SY-TABIX restore after ENDLOOP. fleet, ledger and the MZV test fail.
- M3b: no restore on RETURN. ledger and TabixAcrossProcedures fail.

## Limits

- Whole reports with i32 tables produce about 25 KiB of Z80 code. The native
  harness therefore loads them at 0x4000, not 0x8000, and asserts that the
  image stays below the 0xE000 canary. Code size has not been optimized.
- Structures and tables exist only at report level. They cannot yet be FORM,
  method or function parameters, procedure-local DATA, or class attributes.
- Numeric `WRITE` formatting (MinZ adds a trailing blank, the reference
  runtime does not) is a separate, existing issue. The fixtures write numbers
  through templates.

## Reproduce

From minzc:

```sh
go test ./pkg/abap -run '^TestInternalTable' -count=1 -v
go test ./cmd/mzv -run '^TestABAPInternalTableTabixHost$' -count=1
MINZ_ABAP_ITAB_ARTIFACTS=/tmp/abap-itab-artifacts go test ./pkg/abap -run '^TestInternalTable' -count=1
```

To regenerate the reference: in an isolated directory, use
`research/native-abap/semantic-package.json` as `package.json` and the lock
file as `package-lock.json`, run `npm ci` (from the local cache it also works
with `--offline`), copy
[itab-reference.mjs](../../research/native-abap/itab-reference.mjs) beside
`node_modules`, and run
`node_modules/node/bin/node itab-reference.mjs SOURCE.abap > SOURCE.json`.
