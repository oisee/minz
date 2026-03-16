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
| [sqlite_demo.abap](sqlite_demo.abap) | Database ops | `FORM`, `PERFORM`, string params |

## Setup

### Prerequisites

| Tool | Version | Why |
|------|---------|-----|
| **Go** | 1.24+ | Compiler is written in Go |
| **Node.js** | 18+ | Runs [abaplint](https://github.com/abaplint/abaplint) parser (TypeScript) |
| **npm** | 9+ | Installs `@abaplint/core` package |

### Install (one time)

```bash
# 1. Build the MinZ compiler
cd minzc
make build          # produces ./mz binary

# 2. Install the ABAP parser bridge
cd pkg/abap/bridge
npm install         # downloads @abaplint/core (~7 packages, MIT license)
cd ../../..
```

That's it. The bridge is a single `parse.mjs` script that calls `@abaplint/core` and outputs JSON — no global installs, no build steps, no native modules.

### Verify

```bash
# Should print "PASS" — parses ABAP and lowers to HIR
go test ./pkg/abap/ -v -run TestCompileToHIR
```

## Quick start

```bash
# Compile ABAP to Z80 assembly
./mz ../examples/abap/hello.abap -o hello.a80

# Emit HIR to see the intermediate representation
./mz ../examples/abap/fibonacci.abap --emit=hir

# Emit MIR2 to see the optimized IR
./mz ../examples/abap/fizzbuzz.abap --emit=mir2

# Full pipeline to CP/M binary (if target supports it)
./mz ../examples/abap/hello.abap -o hello.com -t cpm
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

### Phase 3 (planned)
- `SELECT` → SQLite via MZV host functions (prototype working! see `examples/nanz/sqlite_demo.nanz`)
- Open SQL → SQLite transpilation (later: client-server protocol over Z80 I/O ports)

### Not planned (Z80 limitations)
- `CALL FUNCTION` — no RFC on a Spectrum
- `ALV` — the CRT *is* the grid
- `SAP GUI` — 256x192 pixels is all you get
- `ABAP Debugger` — use `mzv -trace` instead

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

## Troubleshooting

**`abaplint bridge: ... is Node.js installed?`**
- Install Node.js 18+ (`brew install node` / `apt install nodejs`)
- Run `cd minzc/pkg/abap/bridge && npm install`

**`no ABAP file parsed`**
- abaplint requires `REPORT zname.` as the first statement for programs
- The bridge auto-detects file type from source: `REPORT` → `.prog.abap`, `CLASS` → `.clas.abap`

**`unsupported: SomeStatement`**
- Not all 392 ABAP statement types are lowered yet — the parser handles them fine (abaplint is complete), but the Go lowerer only maps Phase 1+2 constructs to HIR
- Want to add one? Look at `minzc/pkg/abap/lower.go` — each statement type is a `case` in `lowerStmt()`

**Slow first compile**
- First run downloads/compiles `@abaplint/core` via npm — subsequent runs are fast (~400ms)
- The bridge is a cold `node` process per compile; caching/daemon mode is planned

## How abaplint works

[abaplint](https://github.com/abaplint/abaplint) by [Lars Hvam Petersen](https://github.com/larshp) is a standalone ABAP linter and parser written in TypeScript. It:

- Parses **all** ABAP syntax (7.02–7.57) into a 3-layer AST: Token → Statement → Structure
- Supports 392 statement types, 100+ expression types
- Runs without an SAP system — pure static analysis
- MIT licensed, no external dependencies
- Also powers [@abaplint/transpiler](https://github.com/abaplint/transpiler) (ABAP → JavaScript)

We use `@abaplint/core` as an npm library. The bridge script (`parse.mjs`, ~80 lines) feeds source code to abaplint, walks the AST, and emits JSON that Go can deserialize. abaplint handles all the ABAP parsing complexity — we just consume structured nodes.

---

*ABAP: Advanced Business Application Programming.
Z80: Advanced Business Application Processor (retroactively).*
