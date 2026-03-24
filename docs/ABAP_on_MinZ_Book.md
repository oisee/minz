# ABAP on MinZ: Enterprise Programming on Z80

*A practical guide to running ABAP on 8-bit hardware*

### At a Glance

| Ch | Topic | Key Takeaway |
|----|-------|-------------|
| 1 | [Introduction](#1-introduction) | ABAP → abaplint Wasm → HIR → MIR2 → Z80, single binary, no Node.js |
| 2 | [Hello ABAP](#2-hello-abap) | `REPORT` + `WRITE` compiles and runs on MZV in one command |
| 3 | [Data Types](#3-data-types) | `TYPE i` → u16, `TYPE c` → ^u8, `TYPE string` → heap pointer |
| 4 | [Control Flow](#4-control-flow) | IF/WHILE/DO/CASE map directly to HIR — ABAP operators (EQ/NE/LT) supported |
| 5 | [Subroutines](#5-subroutines-formperform) | Single CHANGING param → return value, zero overhead |
| 6 | [OOP](#6-oop-classmethod) | Classes → structs, methods → functions, no vtables — zero-cost dispatch |
| 7 | [Selection Screens](#7-selection-screens-parameters) | PARAMETERS → TUI with focus, colors, TAB/F8/F3 navigation |
| 8 | [Declarative Screens](#8-declarative-screens-with-screen) | `@screen` DSL: 7 lines → full PBO/PAI screen + mini-ALV table grid |
| 9 | [Open SQL](#9-open-sql-select-sqlite) | SELECT → sqlite_query, `*!sql` seeds DB, ABAP wildcards (`*` → `%`) |
| 10 | [Pipeline](#10-the-pipeline-in-detail) | abaplint Wasm → JSON AST → Go lowerer → HIR → MIR2 → Z80/MZV |
| 11 | [Examples](#11-examples-gallery) | Fibonacci, FizzBuzz, Bubble Sort, MARA report, Customer screen |
| 12 | [Nanz + ABAP](#12-nanz-abap-best-of-both-worlds) | The Holy Grail: @screen + SELECT + ALV = SE16 on Z80 |
| 13 | [Architecture](#13-architecture-reference) | Source files, compilation commands, QBE native path |

---

## 1. Introduction

MinZ compiles **ABAP** — the language of SAP enterprise systems — to native **Z80 machine code**. This is not a joke (well, mostly). It demonstrates that MinZ's HIR pipeline is truly language-agnostic: if we can compile ABAP to Z80, we can compile anything.

```
ABAP source  →  abaplint (Wasm)  →  JSON AST  →  Go lowerer  →  HIR  →  MIR2  →  Z80
```

The parser is [abaplint](https://github.com/abaplint/abaplint) by Lars Hvam Petersen — a complete ABAP parser embedded as a Wasm module. No Node.js required. A single `go build` produces a binary that parses ABAP, lowers it to HIR, optimizes it through MIR2, and emits Z80 assembly.

### What Works

| Feature | Status |
|---------|--------|
| `DATA`, `WRITE`, arithmetic | Working |
| `IF/ELSEIF/ELSE` | Working |
| `WHILE`, `DO..TIMES` | Working |
| `CASE/WHEN` | Working |
| `FORM/PERFORM` with USING/CHANGING | Working |
| `CLASS/METHOD` (pure) | Working |
| `PARAMETERS` (selection screen) | Working |
| Events: INITIALIZATION, START-OF-SELECTION | Working |
| `SELECT` (via SQLite on MZV) | Prototype |

### What Won't Work (Z80 Limitations)

- `CALL FUNCTION` — no RFC on a Spectrum
- `ALV` grids — 80×24 terminal is the grid
- SAP GUI — 256×192 pixels is all you get
- Internal tables with millions of rows — you have 48K RAM

---

## 2. Hello ABAP

The simplest ABAP program:

```abap
REPORT zhello.
WRITE 'Hello from Z80!'.
```

Compile and run on MZV:

```bash
mzv examples/abap/hello.abap
```

The compiler:
1. Feeds the source to the embedded abaplint Wasm parser
2. Gets back a JSON AST with statements and expressions
3. The Go lowerer (`pkg/abap/lower.go`) maps each ABAP construct to HIR
4. `WRITE 'Hello'` becomes `CallExpr{Fn: "abap_write_str", Args: [StringLit("Hello")]}`
5. HIR is lowered to MIR2, optimized, and executed on the MIR2 VM

---

## 3. Data Types

ABAP types map to MinZ/Z80 types:

| ABAP Type | MinZ Type | Size | Notes |
|-----------|-----------|------|-------|
| `TYPE i` | `u16` | 2 bytes | Signed arithmetic via patterns |
| `TYPE c LENGTH n` | `[u8; n+1]` | n+1 bytes | Null-terminated string |
| `TYPE string` | `^u8` | 2 bytes | Heap pointer (MZV only) |
| `TYPE p` | — | — | Not supported (BCD) |
| `TYPE f` | — | — | Not supported (float) |

### Variables

```abap
DATA lv_count TYPE i VALUE 42.
DATA lv_name  TYPE c LENGTH 20 DEFAULT 'World'.
```

Becomes:
```nanz
global lv_count: u16 = 42
global lv_name: [u8; 21]  // 20 chars + null
```

---

## 4. Control Flow

### IF / ELSEIF / ELSE

```abap
IF lv_x > 10.
  WRITE 'big'.
ELSEIF lv_x > 5.
  WRITE 'medium'.
ELSE.
  WRITE 'small'.
ENDIF.
```

Maps directly to HIR `IfStmt` with chained else-if. ABAP comparison operators (`EQ`, `NE`, `LT`, `GT`, `LE`, `GE`) are synonyms for `=`, `<>`, `<`, `>`, `<=`, `>=`.

### WHILE

```abap
DATA lv_i TYPE i VALUE 1.
WHILE lv_i <= 10.
  WRITE lv_i.
  lv_i = lv_i + 1.
ENDWHILE.
```

### DO...TIMES

```abap
DO 5 TIMES.
  WRITE SY-INDEX.
ENDDO.
```

The system variable `SY-INDEX` is maintained as a global that increments each iteration.

### CASE / WHEN

```abap
CASE lv_color.
  WHEN 1. WRITE 'red'.
  WHEN 2. WRITE 'green'.
  WHEN 3. WRITE 'blue'.
  WHEN OTHERS. WRITE 'unknown'.
ENDCASE.
```

Lowered to a chain of `IfStmt` nodes with equality checks.

---

## 5. Subroutines (FORM/PERFORM)

```abap
FORM add USING pv_a TYPE i pv_b TYPE i CHANGING pv_result TYPE i.
  pv_result = pv_a + pv_b.
ENDFORM.

DATA lv_sum TYPE i.
PERFORM add USING 3 5 CHANGING lv_sum.
WRITE lv_sum.  " → 8
```

**Lowering rules:**
- `USING` params → read-only function parameters
- Single `CHANGING` param → function return value
- Multiple `CHANGING` → out-params via pointer

This is a key insight: ABAP's `FORM` with one `CHANGING` is exactly a function that returns a value. No overhead.

---

## 6. OOP (CLASS/METHOD)

```abap
CLASS lcl_circle DEFINITION.
  PUBLIC SECTION.
    DATA radius TYPE i.
    METHODS area RETURNING VALUE(rv_area) TYPE i.
ENDCLASS.

CLASS lcl_circle IMPLEMENTATION.
  METHOD area.
    rv_area = radius * radius * 3.
  ENDMETHOD.
ENDCLASS.

DATA lo_c TYPE REF TO lcl_circle.
CREATE OBJECT lo_c.
lo_c->radius = 10.
DATA lv_a TYPE i.
lv_a = lo_c->area( ).
WRITE lv_a.  " → 300
```

**Lowering:** Classes become structs. Methods become standalone functions with `self` pointer. `RETURNING VALUE(...)` becomes the function return. No vtables — everything is monomorphized at compile time (zero-cost dispatch).

---

## 7. Selection Screens (PARAMETERS)

The classic ABAP report pattern: define parameters, show a selection screen, process results.

```abap
REPORT zselscreen.

PARAMETERS: p_name  TYPE c LENGTH 20 DEFAULT 'World',
            p_count TYPE i DEFAULT 5.

INITIALIZATION.
  p_name = 'Z80 User'.

START-OF-SELECTION.
  WRITE 'Hello,'.
  WRITE p_name.

  DATA lv_i TYPE i VALUE 1.
  WHILE lv_i <= p_count.
    WRITE lv_i.
    lv_i = lv_i + 1.
  ENDWHILE.

END-OF-SELECTION.
  WRITE 'Done!'.
```

On MZV, this renders a full TUI selection screen with colors and keyboard navigation:

```
+---------------------------------------------+
|   Selection Screen                          |  <- white-on-blue
|                                             |
|   P_NAME     [Z80 User  ]                  |  <- cyan label, focused field
|   P_COUNT    [5]                            |  <- cyan label
|                                             |
|   [F8=Execute]  [F3=Back]                   |  <- bright buttons
|                                             |
|   TAB=Next  Enter=Edit  F8=Execute  F3=Back |  <- white-on-blue status
+---------------------------------------------+
  MZV verified output — colors via ANSI escapes
```

The screen host functions (`sel_register_str`, `sel_register_int`, `sel_show`, `sel_get_int`) handle input on MZV with full TUI rendering: focus highlighting, TAB navigation, inline editing. On Z80/CP/M, the same functions would use BDOS calls with VT100 escape sequences.

---

## 8. Declarative Screens with @screen

For Nanz programs that want ABAP-style screens, the `@screen` metafunction generates a complete selection screen from a declarative description:

```nanz
@screen("Material Report") {
    field "Material" length 10 default "*"
    field "Plant" length 6
    int   "Count" default 10
    button "Execute" key F8
    button "Back" key F3
}

fun main() -> void {
    screen_init()
    var result: u8 = screen_show()

    if result == SCREEN_EXECUTE {
        tui_puts(c"Material: ")
        tui_puts(screen_material())
        tui_puts(c"\nCount: ")
        _scr_print_u16(screen_count())
        tui_puts(c"\n")
    }
}
```

MZV renders this as a full TUI screen (actual `mzv -H` output):

```
+---------------------------------------------+
|   Material Report                           |  <- white-on-blue title bar
|                                             |
|   Material    [*         ]                  |  <- cyan label, blue focus
|   Plant       [      ]                      |  <- cyan label
|   Count       [10]                          |  <- cyan label
|                                             |
|   [F8=Execute]  [F3=Back]                   |  <- bright buttons
|                                             |
|   TAB=Next  Enter=Edit  F8=Execute  F3=Back |  <- white-on-blue status bar
+---------------------------------------------+
  MZV verified — 7 lines of @screen DSL → full interactive TUI
```

### What @screen Generates

From 7 lines of declaration, `@screen` generates:

| Generated | Purpose |
|-----------|---------|
| `@extern fun tui_*()` | Terminal rendering primitives |
| `const SCREEN_EXECUTE/BACK` | Action constants |
| `global _buf_<name>` | Text field buffers |
| `global _val_<name>` | Integer field values |
| `screen_init()` | Initialize screen state and defaults |
| `screen_render()` | PBO — render title, fields, buttons, status bar |
| `screen_handle_key()` | PAI — handle F-keys, TAB, Enter, ESC |
| `screen_show() -> u8` | Main event loop (PBO→PAI cycle) |
| `screen_<name>() -> ^u8/u16` | Accessor functions for field values |

### Screen Elements

| Element | Syntax | Example |
|---------|--------|---------|
| Text field | `field "Label" length N default "D"` | `field "Name" length 20 default "ACME"` |
| Integer | `int "Label" default N` | `int "Count" default 10` |
| Table | `table "Name" rows N` | `table "Materials" rows 8` |
| Column | `column "Name" width N` | `column "MATNR" width 10` |
| Button | `button "Label" key FK` | `button "Execute" key F8` |

### Table API (Mini-ALV)

The `table` + `column` elements generate a flat-buffer-backed grid display:

| Generated | Purpose |
|-----------|---------|
| `table_init()` | Clear table data, set row count to 0 |
| `table_set_nrows(n)` | Set number of visible rows |
| `table_nrows() -> u8` | Get current row count |
| `table_set_<col>(row, val)` | Set cell value for named column |
| `table_set_str(row, offset, val)` | Set cell by byte offset |

### PBO/PAI Flow

The screen follows the ABAP dynpro pattern:

```
┌─────────────────┐
│  screen_init()  │  ← Set up fields and defaults
└────────┬────────┘
         │
    ┌────▼────┐
    │  PBO    │  ← screen_render(): draw title, fields, buttons
    └────┬────┘
         │
    ┌────▼────┐
    │  wait   │  ← tui_read_key(): block for user input
    └────┬────┘
         │
    ┌────▼────┐
    │  PAI    │  ← screen_handle_key(): F8→Execute, F3→Back, TAB→Next
    └────┬────┘
         │
    ┌────▼─────────┐
    │ action==None? │──yes──→ loop back to PBO
    └────┬─────────┘
         │no
    ┌────▼────┐
    │ return  │  ← SCREEN_EXECUTE or SCREEN_BACK
    └─────────┘
```

### Interactive Features

- **TAB** — cycle focus between fields and buttons
- **Enter** — edit focused text field (inline, with cursor) or activate button
- **F-keys** — direct action (F8=Execute, F3=Back, etc.)
- **ESC** — back/cancel
- Focus highlighting: blue background on active field

### Mini-ALV: Table Display

The `table` element brings ABAP's ALV grid to the terminal. Declare columns, fill data, display:

```nanz
@screen("Material List") {
    field "Filter" length 10 default "*"
    table "Materials" rows 8
    column "MATNR" width 10
    column "MTART" width 6
    column "MEINS" width 4
    button "Execute" key F8
    button "Back" key F3
}

fun load_materials() -> void {
    table_init()
    table_set_matnr(0, c"MAT-001")
    table_set_mtart(0, c"FERT")
    table_set_meins(0, c"ST")
    // ... more rows ...
    table_set_nrows(5)
}

fun main() -> void {
    screen_init()
    load_materials()
    var result: u8 = screen_show()
}
```

MZV renders this as (actual `mzv -H` output):

```
+---------------------------------------------+
|   Material List                             |  <- white-on-blue title bar
|                                             |
|   Filter      [*         ]                  |  <- cyan label, blue focus
|   MATNR      MTART  MEINS                   |  <- bright column headers
|   -----------------------                   |
|   MAT-001    FERT   ST                      |     data rows
|   MAT-002    HALB   KG                      |
|   MAT-003    ROH    L                       |
|   MAT-004    FERT   ST                      |
|   MAT-005    VERP   PC                      |
|                                             |
|   [F8=Execute]  [F3=Back]                   |  <- bright buttons
|                                             |
|   TAB=Next  Enter=Edit  F8=Execute  F3=Back |  <- white-on-blue status bar
+---------------------------------------------+
  MZV verified — mini-ALV with 5 material rows
```

The table uses a flat byte buffer internally (struct arrays have a known VM bug). Each row is `sum(column_widths + 1)` bytes. The `table_set_<col>(row, val)` helpers handle offset calculation automatically.

---

## 9. Open SQL: SELECT → SQLite

ABAP's Open SQL statements are transpiled to SQLite host function calls at compile time. The SQL string is built from the ABAP `SELECT` syntax and emitted as a `sqlite_query()` call.

### Database Setup with `*!sql`

Use comment pragmas to seed the database. These execute as `sqlite_exec()` calls before `main()`:

```abap
*!sql CREATE TABLE scarr (carrid TEXT, carrname TEXT, url TEXT)
*!sql INSERT INTO scarr VALUES ('LH', 'Lufthansa', 'http://www.lufthansa.com')
```

### SELECT SINGLE — One Row

```abap
DATA lv_count TYPE i.
SELECT SINGLE COUNT(*) FROM scarr INTO lv_count.
WRITE lv_count.   " → 3

DATA lv_name TYPE string.
SELECT SINGLE carrname FROM scarr INTO lv_name WHERE carrid = 'LH'.
WRITE lv_name.    " → Lufthansa
```

Transpiled to: `sqlite_query(db, "SELECT COUNT(*) FROM scarr")` → `sqlite_step` → `sqlite_column_int/text` → `sqlite_finalize`.

### SELECT ... ENDSELECT — Row Loop

```abap
DATA lv_id TYPE string.
DATA lv_name TYPE string.
SELECT carrid carrname FROM scarr INTO (lv_id, lv_name).
  WRITE lv_id.
  WRITE lv_name.
ENDSELECT.
```

Output:
```
AA American Airlines
LH Lufthansa
SQ Singapore Airlines
```

Transpiled to: `sqlite_query` → `while sqlite_step == 1 { column_text + body }`.

### Runtime Filtering with `sqlite_query_like`

In Nanz programs, the `sqlite_query_like()` host function builds `WHERE ... LIKE` clauses at runtime from `@screen` field values:

```nanz
var q: u16 = sqlite_query_like(db,
    c"SELECT carrid, carrname FROM scarr ORDER BY carrid",
    c"carrid",
    screen_carrier())   // filter from @screen field
```

ABAP wildcard conventions:
| Filter | SQL | Matches |
|--------|-----|---------|
| `*` or `%` | (no WHERE) | All rows |
| `LH` | `WHERE carrid = 'LH'` | Exact match |
| `A*` | `WHERE carrid LIKE 'A%'` | Starts with A |
| `*A*` | `WHERE carrid LIKE '%A%'` | Contains A |

### Type Mapping

| ABAP Type | SQLite | Host Function |
|-----------|--------|---------------|
| `TYPE i` | INTEGER | `sqlite_column_int` |
| `TYPE string` | TEXT | `sqlite_column_text` |
| `TYPE c LENGTH n` | TEXT | `sqlite_column_text` |

---

## 10. The Pipeline in Detail

### Stage 1: Parsing (abaplint)

The embedded Wasm module contains `@abaplint/core`, a complete ABAP parser. It produces a 3-layer AST:

```
Token  →  Statement  →  Structure
(lexer)   (parser)      (semantic)
```

The bridge script walks the AST and emits JSON with typed nodes:

```json
{"type": "Write", "target": {"type": "StringLiteral", "value": "Hello"}}
```

### Stage 2: Go Lowerer

`pkg/abap/lower.go` maps ABAP semantics to HIR. Key mappings:

| ABAP | HIR |
|------|-----|
| `DATA lv_x TYPE i VALUE 42` | `Global{Name: "lv_x", Ty: TyU16, Init: [42, 0]}` |
| `WRITE lv_x` | `CallExpr{Fn: "abap_write_u16", Args: [VarRef("lv_x")]}` |
| `IF cond` | `IfStmt{Cond: cond, Then: block, Else: block}` |
| `PERFORM name USING a CHANGING r` | `CallExpr{Fn: "name", Args: [a]}` + assign return |
| `lo->method()` | `CallExpr{Fn: "lcl_class_method", Args: [lo]}` |

### Stage 3: MIR2 Optimization

Standard MIR2 passes apply: constant propagation, dead code elimination, register allocation. ABAP-specific: the lowerer generates efficient code for ABAP's value semantics (no heap allocation for structures).

### Stage 4: Z80 / MZV Execution

On MZV: functions like `abap_write_str` are host functions that print to stdout. Selection screen functions (`sel_show` etc.) render interactive TUI screens.

On Z80/CP/M: the same functions compile to BDOS calls. `abap_write_str` becomes `LD C,9 / CALL 5` (print string). The selection screen uses VT100 escape sequences for positioning.

---

## 11. Examples Gallery

### Fibonacci

```abap
REPORT zfibonacci.
DATA: lv_a TYPE i VALUE 0,
      lv_b TYPE i VALUE 1,
      lv_c TYPE i.
DO 10 TIMES.
  WRITE lv_a.
  lv_c = lv_a + lv_b.
  lv_a = lv_b.
  lv_b = lv_c.
ENDDO.
```

Output: `0 1 1 2 3 5 8 13 21 34`

### FizzBuzz

```abap
REPORT zfizzbuzz.
DATA lv_i TYPE i VALUE 1.
WHILE lv_i <= 20.
  DATA lv_mod3 TYPE i.
  DATA lv_mod5 TYPE i.
  lv_mod3 = lv_i MOD 3.
  lv_mod5 = lv_i MOD 5.
  IF lv_mod3 = 0 AND lv_mod5 = 0.
    WRITE 'FizzBuzz'.
  ELSEIF lv_mod3 = 0.
    WRITE 'Fizz'.
  ELSEIF lv_mod5 = 0.
    WRITE 'Buzz'.
  ELSE.
    WRITE lv_i.
  ENDIF.
  lv_i = lv_i + 1.
ENDWHILE.
```

### Bubble Sort

```abap
REPORT zbubble.
DATA: lv_a TYPE i VALUE 64,
      lv_b TYPE i VALUE 34,
      lv_c TYPE i VALUE 25,
      lv_d TYPE i VALUE 12,
      lv_e TYPE i VALUE 22.

* 5-element bubble sort using nested WHILE
DATA lv_swapped TYPE i VALUE 1.
WHILE lv_swapped = 1.
  lv_swapped = 0.
  * Compare adjacent pairs and swap if needed
  IF lv_a > lv_b.
    DATA lv_tmp TYPE i.
    lv_tmp = lv_a. lv_a = lv_b. lv_b = lv_tmp.
    lv_swapped = 1.
  ENDIF.
  * ... (remaining comparisons)
ENDWHILE.

WRITE lv_a. WRITE lv_b. WRITE lv_c. WRITE lv_d. WRITE lv_e.
```

### Material Master Report (SQLite)

```abap
REPORT zmara_report.
PARAMETERS p_matnr TYPE c LENGTH 18 DEFAULT '%'.

START-OF-SELECTION.
  WRITE '=== Material Master Report ==='.
  DATA lv_db TYPE i.
  PERFORM open_db CHANGING lv_db.
  PERFORM query_mara USING lv_db p_matnr CHANGING lv_stmt.
  * ... fetch and print rows ...
  PERFORM close_db USING lv_db.
```

This queries a SQLite database via MZV host functions — the first time MARA has been queried on a Z80 processor.

### Create Customer (@screen)

```nanz
@screen("Create Customer") {
    field "Name" length 20 default "ACME Corp"
    field "City" length 15
    field "Country" length 3 default "DE"
    int   "Credit" default 5000
    button "Execute" key F8
    button "Back" key F3
}
```

MZV output (actual `mzv -H` capture):

```
+---------------------------------------------+
|   Create Customer                           |  <- white-on-blue title bar
|                                             |
|   Name        [ACME Corp           ]        |  <- blue focus on first field
|   City        [               ]             |  <- cyan labels
|   Country     [DE ]                         |
|   Credit      [5000]                        |
|                                             |
|   [F8=Execute]  [F3=Back]                   |  <- bright buttons
|                                             |
|   TAB=Next  Enter=Edit  F8=Execute  F3=Back |  <- white-on-blue status bar
+---------------------------------------------+
  MZV verified — 4 fields, defaults pre-filled
```

---

## 12. Nanz + ABAP: Best of Both Worlds

ABAP excels at business logic patterns: structured data, reports, selection screens. Nanz excels at systems programming: zero-cost abstractions, direct hardware access, metaprogramming.

The `@screen` metafunction brings ABAP's screen paradigm to Nanz. You can mix:

```nanz
// ABAP-style declarative screen
@screen("Inventory Check") {
    field "Warehouse" length 4 default "WH01"
    int   "Threshold" default 100
    button "Execute" key F8
    button "Back" key F3
}

// Nanz-style systems programming
fun check_inventory(wh: ^u8, threshold: u16) -> u16 {
    // Direct Z80 I/O port access for warehouse sensors
    asm z80 (in wh) {
        LD C, (HL)     ; warehouse code
        IN A, (0xFE)   ; read sensor port
    }
    return 0
}

fun main() -> void {
    screen_init()
    if screen_show() == SCREEN_EXECUTE {
        var count: u16 = check_inventory(
            screen_warehouse(), screen_threshold())
        tui_puts(c"Items found: ")
        _scr_print_u16(count)
        tui_puts(c"\n")
    }
}
```

### The Holy Grail: @screen + SELECT + ALV

The complete ABAP report pattern — selection screen, database query, grid display — in one Nanz program:

```nanz
@screen("SCARR Report") {
    field "Carrier" length 3 default "*"
    table "Airlines" rows 8
    column "ID" width 4
    column "Carrier Name" width 22
    column "URL" width 30
    button "Execute" key F8
    button "Back" key F3
}

global db: u16

fun seed_database() -> void {
    db = sqlite_open(0)
    sqlite_exec(db, c"CREATE TABLE scarr (carrid TEXT, carrname TEXT, url TEXT)")
    sqlite_exec(db, c"INSERT INTO scarr VALUES ('AA', 'American Airlines', 'http://www.aa.com')")
    // ... 8 airlines total ...
}

fun run_query() -> void {
    table_init()
    var q: u16 = sqlite_query(db,
        c"SELECT carrid, carrname, url FROM scarr ORDER BY carrid")
    var row: u8 = 0
    while sqlite_step(q) == 1 {
        table_set_id(row, sqlite_column_text(q, 0))
        table_set_carrier_name(row, sqlite_column_text(q, 1))
        table_set_url(row, sqlite_column_text(q, 2))
        row = row + 1
    }
    table_set_nrows(row)
}

fun main() -> void {
    seed_database()
    screen_init()
    run_query()
    screen_show()
    sqlite_close(db)
}
```

MZV renders this as (actual `mzv -H` output):

```
+--------------------------------------------------------------+
|   SCARR Report                                               |  <- white-on-blue
|                                                              |
|   Carrier     [*  ]                                          |  <- filter field
|   ID   Carrier Name           URL                            |  <- bright headers
|   ---------------------------------------------------------- |
|   AA   American Airlines      http://www.aa.com              |
|   AF   Air France             http://www.airfrance.com       |
|   BA   British Airways        http://www.ba.com              |
|   EK   Emirates               http://www.emirates.com        |
|   LH   Lufthansa              http://www.lufthansa.com       |
|   QF   Qantas                 http://www.qantas.com.au       |
|   SQ   Singapore Airlines     http://www.singaporeair.com    |
|   UA   United Airlines        http://www.united.com          |
|                                                              |
|   [F8=Execute]  [F3=Back]                                    |  <- buttons
|                                                              |
|   TAB=Next  Enter=Edit  F8=Execute  F3=Back                  |  <- status bar
+--------------------------------------------------------------+
  MZV verified — 8 airlines from SQLite, displayed in mini-ALV
```

This is the first time an ABAP-style SE16 report has run on a Z80 processor.

---

## 13. Architecture Reference

### Source Files

| File | Purpose |
|------|---------|
| `pkg/abap/parse.go` | ABAP parser (Wasm + JSON deserialization) |
| `pkg/abap/lower.go` | ABAP → HIR lowerer |
| `pkg/abap/bridge/` | abaplint Wasm bridge |
| `cmd/mzv/screen_host.go` | Selection screen host functions |
| `cmd/mzv/tui_host.go` | TUI rendering host functions |
| `pkg/nanz/screen_gen.go` | @screen built-in metafunction |
| `examples/abap/` | ABAP example programs |

### Compilation Commands

```bash
# Compile ABAP to Z80 assembly
mz examples/abap/hello.abap -o hello.a80

# Run on MZV (interactive)
mzv examples/abap/selscreen.abap

# Run headless (defaults/piped input)
echo "" | mzv -H examples/abap/hello.abap

# Emit intermediate representations
mz examples/abap/fibonacci.abap --emit=hir
mz examples/abap/fibonacci.abap --emit=mir2

# Native compilation via QBE
mz examples/abap/hello.abap -b qbe -o hello
./hello
```

---

## Appendix: Test Matrix (MZV + Z80 CP/M)

All examples tested on MZV (MIR2 VM) and through the full Z80 pipeline (`mz` → `mza` → `mze -t cpm`).

| Example | MZV | mz | mza | Size | mze CP/M | Notes |
|---------|-----|-----|-----|------|----------|-------|
| hello.abap | PASS | PASS | PASS | 399B | runs | string via BDOS |
| fibonacci.abap | PASS | PASS | PASS | 457B | PASS | numeric output |
| fizzbuzz.abap | PASS | PASS | PASS | 471B | PASS | IF/ELSEIF + MOD |
| bubble_sort.abap | PASS | PASS | PASS | 756B | PASS | nested WHILE + swap |
| forms.abap | PASS | PASS | PASS | 492B | PASS | FORM/PERFORM |
| sysinfo.abap | PASS | PASS | PASS | 805B | PASS | multiple WRITE |
| hello_param.abap | PASS | PASS | PASS | 526B | PASS | PARAMETERS + sel_show |
| tui_screen.nanz | PASS | PASS | PASS | 446B | runs | TUI host functions |
| meta_screen.nanz | PASS | PASS | PASS | 259B | runs | user @screen metafun |
| abap_screen.nanz | PASS | PASS | PASS | 1445B | runs | manual PBO/PAI |
| screen_declarative.nanz | PASS | PASS | PASS | 1553B | runs | @screen declarative |
| screen_customer.nanz | PASS | PASS | PASS | 1866B | runs | 4-field customer form |
| screen_alv.nanz | PASS | PASS | PASS | 1996B | runs | mini-ALV table grid |
| screen_report.nanz | PASS | PASS | PASS | 3053B | runs | holy grail: screen+SQL+ALV |
| select_demo.abap | PASS | PASS | PASS | 1215B | PASS | SELECT SINGLE + COUNT(*) |
| select_loop.abap | PASS | PASS | PASS | 872B | PASS | SELECT...ENDSELECT loop |

**Summary:** 16/16 MZV, 16/16 compile to Z80, 16/16 assemble. 8/16 produce visible CP/M output, 8/16 run correctly but use TUI host functions (screen rendering is MZV-only until CP/M VT100 bridge is wired).

---

*ABAP: Advanced Business Application Programming.*
*Z80: Advanced Business Application Processor (retroactively).*

*Your ZX Spectrum is now an enterprise-grade ABAP runtime.*
