# ABAP on Z80 🤯

> *"Your ZX Spectrum is now an enterprise-grade ABAP runtime."*

MinZ compiles **ABAP** to native **Z80 machine code** via the HIR/MIR2 pipeline.
The parser is powered by [abaplint](https://github.com/abaplint/abaplint) by Lars Hvam Petersen — a full ABAP parser written in TypeScript.

## How it works

```
ABAP source  →  abaplint (Node.js)  →  JSON AST  →  Go lowerer  →  HIR  →  MIR2  →  Z80
```

That's right: your `REPORT` program goes through **7 compilation stages** to become Z80 opcodes.

## Examples

| File | What it does | ABAP features |
|------|-------------|---------------|
| [hello.abap](hello.abap) | Hello world | `DATA`, `WRITE` |
| [fibonacci.abap](fibonacci.abap) | Fibonacci sequence | `WHILE`, arithmetic |
| [fizzbuzz.abap](fizzbuzz.abap) | FizzBuzz | `IF/ELSEIF/ELSE`, `MOD` |
| [guessing_game.abap](guessing_game.abap) | Number guessing | `DO..TIMES`, `CASE/WHEN`, `EXIT` |
| [bubble_sort.abap](bubble_sort.abap) | Bubble sort | Nested `WHILE`, `CASE`, swap pattern |
| [forms.abap](forms.abap) | Subroutines | `FORM`, `PERFORM`, `USING/CHANGING` |
| [oop.abap](oop.abap) | OOP shapes | `CLASS`, `INTERFACE`, `METHOD`, inheritance |
| [sysinfo.abap](sysinfo.abap) | System report | Multiple `WRITE`, string output |

## Quick start

```bash
# Prerequisites: Node.js, Go
cd minzc/pkg/abap/bridge && npm install

# Compile ABAP to Z80 assembly
./minzc examples/abap/hello.abap -o hello.a80

# Or emit HIR to see the intermediate representation
./minzc examples/abap/fibonacci.abap --emit=hir
```

## Supported ABAP constructs

### Phase 1 (working now)
- `REPORT` — program declaration
- `DATA` — variable declaration with `TYPE` and `VALUE`
- `WRITE` — console output (maps to CP/M BDOS)
- `IF / ELSEIF / ELSE / ENDIF` — conditionals
- `WHILE / ENDWHILE` — pre-condition loops
- `DO [n TIMES] / ENDDO` — counted and infinite loops
- `CASE / WHEN / ENDCASE` — multi-branch dispatch
- `FORM / ENDFORM` — subroutines
- `PERFORM` — subroutine calls with `USING` / `CHANGING`
- `EXIT` / `CONTINUE` — loop control
- Arithmetic: `+`, `-`, `*`, `/`, `MOD`
- Comparisons: `=`, `<>`, `<`, `>`, `<=`, `>=`, `EQ`, `NE`, `LT`, `GT`, `LE`, `GE`
- Logical operators: `AND`, `OR`

### Phase 2 (scaffolded)
- `CLASS / ENDCLASS` — OOP with zero-cost dispatch
- `INTERFACE / ENDINTERFACE` — structural interfaces (no vtables!)
- `METHOD / ENDMETHOD` — instance methods
- `CREATE OBJECT` — object instantiation
- `INHERITING FROM` — single inheritance

### Not planned (Z80 limitations)
- `SELECT` — no database on Z80 (shocking, we know)
- `CALL FUNCTION` — no RFC on a Spectrum
- `ALV` — the CRT *is* the grid
- `SAP GUI` — 256x192 pixels is all you get

## Why?

Because we can. And because the idea of an SAP consultant debugging ABAP on a ZX Spectrum is objectively hilarious.

Also: it validates that MinZ's HIR pipeline is truly language-agnostic. If we can compile ABAP to Z80, we can compile *anything* to Z80.

## Architecture

The ABAP frontend lives in `minzc/pkg/abap/` with three layers:

1. **Bridge** (`bridge/parse.mjs`) — calls `@abaplint/core` to parse ABAP into a structured AST, emits JSON
2. **Parser** (`parse.go`) — Go code that invokes Node.js, deserializes JSON, builds semantic `Program`
3. **Lowerer** (`lower.go`) — converts semantic ABAP constructs to HIR nodes

The lowerer maps ABAP concepts to Z80-friendly HIR:
- `DATA lv_x TYPE i VALUE 42` → `mir2.Global{Name: "lv_x", Ty: TyU16, Init: [42, 0]}`
- `WRITE lv_x` → `hir.CallExpr{Fn: "abap_write", Args: [VarRef("lv_x")]}`
- `FORM name USING p1` → `hir.Func{Name: "name", Params: [{Name: "p1", Ty: TyU16}]}`
- `IF cond ... ENDIF` → `hir.IfStmt{Cond: ..., Then: ..., Else: ...}`
- `CLASS lcl_foo` → `mir2.StructTy{Name: "lcl_foo"}` + method functions

---

*ABAP: Advanced Business Application Programming.
Z80: Advanced Business Application Processor (retroactively).*
