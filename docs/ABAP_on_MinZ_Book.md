# ABAP on MinZ: Enterprise Programming on Z80

*A practical guide to running ABAP on 8-bit hardware*

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

On MZV, this renders an interactive selection screen:

```
┌─ Selection Screen ──────────────────┐
│                                    │
│  P_NAME    [Z80 User            ]  │
│  P_COUNT   [5                   ]  │
│                                    │
│  [Enter=Execute]                   │
└────────────────────────────────────┘
```

The screen host functions (`sel_register_str`, `sel_register_int`, `sel_show`, `sel_get_int`) handle input on MZV. On Z80/CP/M, the same functions would use BDOS calls for console I/O.

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
| Button | `button "Label" key FK` | `button "Execute" key F8` |

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

---

## 9. The Pipeline in Detail

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

## 10. Examples Gallery

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

---

## 11. Nanz + ABAP: Best of Both Worlds

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

---

## 12. Architecture Reference

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

*ABAP: Advanced Business Application Programming.*
*Z80: Advanced Business Application Processor (retroactively).*

*Your ZX Spectrum is now an enterprise-grade ABAP runtime.*
