# Report #079 — Pascal Frontend: Corpus Analysis & Feature Matrix

**Date:** 2026-03-15
**Status:** Research complete, ready for implementation
**Decision:** Write our own parser (TP3.0 subset for Z80)

---

## Part 1: Real-World Corpus Sources

### Tier 1: Z80/CP/M Pascal (PRIMARY targets)

| Source | What | Files | Dialect | Why |
|--------|------|-------|---------|-----|
| **pleumann/pasta80** | TP3.0 compiler for Z80+ZX Spectrum, with test suite | ~30 .pas | TP 3.0 | **THE gold standard** — `tests/all.pas` is 1500+ lines, covers every TP3 feature |
| **davidly/cpm_compilers** | Same benchmark (sieve, ttt, e) for 6 CP/M Pascal compilers | ~20 .pas | TP1, TP2, MT+, JRT, Oxford | Shows dialect differences |
| **cpm.z80.de** | CP/M Pascal compiler archive | Archives | MT+, SVS, UCSD | Historical reference |

### Tier 2: Turbo Pascal Games (SECONDARY — feature census)

| Source | What | Features Used |
|--------|------|---------------|
| **lhkastenson/MarioPascal** | Mario clone, 20 .pas files, TP6/7 | units, records, pointers, inline asm, sets, with, nested procs, typed const, absolute var |
| **johangardhage/dos-tpdemos** | 20 3D demos, TP7 | units, records, pointers, heavy inline asm, shr/shl |
| **reduz/nuku** | DOS platformer by Godot creator, 9 .pas files | units, records, pointers, asm, OOP (Object), shortint |
| **saulopz/chupaplets** | OOP puzzle game, 22 .pas files, BP7 | units, OOP (Object, Virtual, Constructor), enum, forward |
| **01e9/pascal-games** | Tetris/Tank/Snake TP7 | units, records, case, longint, 4D arrays |
| **Erfaniaa/pascal-games** | 16 simple games | Crt, Graph, goto, repeat..until |

### Tier 3: Large Real-World Programs

| Source | What | Size |
|--------|------|------|
| **delphidabbler/swag** | 57-category TP code archive (1997) | Thousands of .pas |
| **romiras/turbo-pascal-archive** | DOS Navigator 1.51 source + units | Dozens of .pas |
| **Renegade BBS** | Complete BBS system | ~50 .pas, inline asm |

### Tier 4: Reference Compilers

| Source | What | Useful for |
|--------|------|------------|
| **akrennmair/pascal** | Go ISO Pascal parser, MIT, 9K LOC | Grammar reference, test suite (iso7185pat.pas) |
| **lkesteloot/turbopascal** | Web-based TP5.5 compiler | Clean example .pas files |

---

## Part 2: Feature Census from Real Code

Analysis of 6 game repos + PASTA/80 tests + CP/M benchmarks:

### UNIVERSAL (every program uses)

| Feature | Example | HIR mapping |
|---------|---------|-------------|
| `program Name; begin..end.` | Program structure | Module |
| `const X = 42;` | Untyped constants | Inline literal |
| `var X: Integer;` | Variables | VarDeclStmt |
| `procedure P; begin..end;` | Procedures | Func (RetTy=void) |
| `function F: Integer; begin F := x; end;` | Functions (result = name) | Func + implicit return var |
| `if..then..else` | Conditional | IfStmt |
| `for I := 0 to N do` | Counted loop | ForRangeStmt |
| `begin..end` | Block | Block |
| `X := Y + Z;` | Assignment + arithmetic | AssignStmt + BinExpr |
| `WriteLn(...)` | Output | Extern call |
| Types: `Integer, Byte, Boolean, Char` | Basic types | i16, u8, bool, u8 |
| `A[I]` | Array indexing | IndexExpr |
| `(* comment *)` and `{ comment }` | Comments | Skip |

### VERY COMMON (4+ of 6 repos)

| Feature | Example | HIR mapping | Notes |
|---------|---------|-------------|-------|
| `record..end` | `TPoint = record X,Y: Integer; end;` | StructTy | Backbone of game state |
| `case X of` | Multi-branch | SwitchStmt | Heavy in game loops |
| `while..do` | Pre-check loop | WhileStmt | |
| `repeat..until` | Post-check loop | `do { body } while (!cond)` | Desugar to while |
| Typed const | `FortyTwo: Integer = 42;` | Global with init | TP-specific, mutable! |
| `Pointer`, `^T`, `New/Dispose` | Dynamic memory | TyPtr + alloc extern | |
| `Word` type | Unsigned 16-bit | u16 | |
| `string[N]` | Fixed-length string | [N+1]u8 | Length-prefixed |
| Multi-dim arrays | `array[0..9] of array[0..9]` | Nested ArrayTy | Up to 4D in games |
| `var` params (by reference) | `procedure Swap(var A, B: Integer)` | TyPtr + auto AddrOf | |
| Nested procedures | `procedure Inner; begin..end;` inside outer | Hoist to module | |
| `for I := N downto 0` | Reverse loop | ForRange (reversed) | |

### COMMON (2-3 repos)

| Feature | Example | HIR mapping | Notes |
|---------|---------|-------------|-------|
| `inline(...)` / `asm..end` | Z80 machine code | AsmStmt | Critical for hardware |
| `set of Byte`, `in` | Bit sets | u8/u16 bitfield | Collision detection |
| `with R do` | Record shorthand | Desugar field access | |
| `shr`, `shl` | Bit shift | BinExpr `>>`, `<<` | |
| `$I include` | Include files | Preprocessor | Sprite data, system files |
| `absolute $80` | Memory-mapped var | VarDeclStmt.At | CP/M command line |
| `Inc(X)`, `Dec(X)` | Increment/decrement | `X := X + 1` | Desugar |
| `Ord()`, `Chr()` | Type cast | CastExpr | |
| `external` | External functions | IsExtern | PASTA/80 uses this for Z80 |
| `register` | Calling convention | Register annotation | PASTA/80 specific |

### RARE (0-1 repos, skip for v1)

| Feature | Used by | Decision |
|---------|---------|----------|
| `goto/label` | Beginners only | **SKIP** |
| Variant records | 0/6 repos! | **SKIP** |
| OOP (`Object`) | 2/6 (nuku, chupaplets) | **SKIP for v1** |
| Enumerated types | 1/6 (chupaplets) | **SKIP for v1** (use const) |
| Subrange types | 1/6 | **SKIP** |
| `Real` (float) | PASTA/80 tests | **SKIP** (no Z80 FPU) |
| `uses` (units) | All TP5+ | **Phase 2** (complex) |
| File I/O | 4/6 | **Phase 2** (needs OS) |

---

## Part 3: Minimum Viable Pascal (Phase 1)

Based on the corpus, here's what we need to compile real CP/M programs:

### Grammar (TP3.0 subset)

```ebnf
Program     = 'program' Ident ';' Block '.' .
Block       = [ConstDecl] [TypeDecl] [VarDecl] {ProcDecl | FuncDecl} CompoundStmt .

ConstDecl   = 'const' (Ident '=' Expr ';' | Ident ':' Type '=' Expr ';')+ .
TypeDecl    = 'type' (Ident '=' Type ';')+ .
VarDecl     = 'var' (IdentList ':' Type ';')+ .

Type        = 'Integer' | 'Byte' | 'Word' | 'Boolean' | 'Char'
            | 'string' '[' Expr ']'
            | 'array' '[' Range '..' Range ']' 'of' Type
            | 'record' FieldList 'end'
            | '^' Ident .

ProcDecl    = 'procedure' Ident ['(' ParamList ')'] ';' Block ';' .
FuncDecl    = 'function' Ident ['(' ParamList ')'] ':' Type ';' Block ';' .
ParamList   = [('var')?  IdentList ':' Type] (',' ...) .

CompoundStmt = 'begin' StmtList 'end' .
Stmt        = AssignStmt | ProcCallStmt | CompoundStmt
            | IfStmt | CaseStmt | WhileStmt | RepeatStmt | ForStmt
            | InlineStmt .

AssignStmt  = Designator ':=' Expr .
IfStmt      = 'if' Expr 'then' Stmt ['else' Stmt] .
CaseStmt    = 'case' Expr 'of' (CaseLabel ':' Stmt ';')+ ['else' Stmt] 'end' .
WhileStmt   = 'while' Expr 'do' Stmt .
RepeatStmt  = 'repeat' StmtList 'until' Expr .
ForStmt     = 'for' Ident ':=' Expr ('to'|'downto') Expr 'do' Stmt .
InlineStmt  = 'inline' '(' Bytes ')' .

Expr        = SimpleExpr [RelOp SimpleExpr] .
SimpleExpr  = ['+' | '-'] Term {AddOp Term} .
Term        = Factor {MulOp Factor} .
Factor      = IntLit | StrLit | CharLit | BoolLit
            | Designator | '(' Expr ')' | 'not' Factor
            | FuncCall .

Designator  = Ident {'.' Ident | '[' Expr ']' | '^'} .

RelOp       = '=' | '<>' | '<' | '>' | '<=' | '>=' | 'in' .
AddOp       = '+' | '-' | 'or' .
MulOp       = '*' | 'div' | 'mod' | 'and' | 'shr' | 'shl' .
```

### Phase 1 → HIR Mapping Table (what we implement first)

| Pascal | HIR | LOC estimate |
|--------|-----|-------------|
| `program..end.` | Module | 20 |
| `const/var/type` | Globals, VarDecl, Types | 100 |
| `procedure/function` | Func | 80 |
| `if/case/while/repeat/for` | IfStmt/SwitchStmt/WhileStmt/ForRange | 120 |
| `begin..end` | Block | 10 |
| Assignment `:=` | AssignStmt | 30 |
| Expressions | BinExpr/UnaryExpr/CallExpr | 150 |
| Arrays | ArrayTy + IndexExpr | 50 |
| Records | StructTy + FieldExpr | 60 |
| Pointers `^T` | TyPtr + LoadExpr/StoreStmt | 40 |
| `var` params | TyPtr + auto AddrOf | 30 |
| `string[N]` | [N+1]u8 | 30 |
| Typed constants | Global with init | 20 |
| Nested procs | Hoist + rename | 40 |
| `inline(...)` | AsmStmt | 30 |
| Builtins (Inc/Dec/Ord/Chr/WriteLn) | Desugar/extern | 50 |
| **Total** | | **~860 LOC** |

### Phase 2 (after v1 works)

- `uses` / `unit` (cross-file import)
- `set of` (bitfield)
- `with` (desugar)
- File I/O (extern to OS)
- `external` / `register` (PASTA/80 calling conventions)
- `$I` include
- `absolute` address

---

## Part 4: Synthetic Test Cases (from spec + corpus)

### Test 1: Minimal (HelloWorld)
```pascal
program Hello;
begin
  WriteLn('Hello from Pascal on Z80!');
end.
```

### Test 2: Sieve of Eratosthenes (CP/M benchmark)
```pascal
program Sieve;
const size = 100;
type flagType = array[0..size] of Boolean;
var i, k, prime, count: Integer;
    flags: flagType;
begin
  count := 0;
  for i := 0 to size do flags[i] := true;
  for i := 0 to size do begin
    if flags[i] then begin
      prime := i + i + 3;
      k := i + prime;
      while k <= size do begin
        flags[k] := false;
        k := k + prime;
      end;
      count := count + 1;
    end;
  end;
  WriteLn('Primes: ', count);
end.
```

### Test 3: Records + Pointers
```pascal
program RecordTest;
type
  TPoint = record
    X, Y: Integer;
  end;
var
  P: TPoint;
  PP: ^TPoint;
begin
  P.X := 10;
  P.Y := 20;
  WriteLn(P.X + P.Y);
end.
```

### Test 4: Procedures + var params
```pascal
program SwapTest;
var A, B: Integer;

procedure Swap(var X, Y: Integer);
var Tmp: Integer;
begin
  Tmp := X;
  X := Y;
  Y := Tmp;
end;

begin
  A := 1; B := 2;
  Swap(A, B);
  WriteLn(A, ' ', B);
end.
```

### Test 5: Nested procedures + for downto
```pascal
program Nested;
var Total: Integer;

procedure Outer(N: Integer);
var I: Integer;

  procedure Inner(X: Integer);
  begin
    Total := Total + X;
  end;

begin
  for I := N downto 1 do
    Inner(I);
end;

begin
  Total := 0;
  Outer(10);
  WriteLn(Total);
end.
```

### Test 6: Case + Repeat..Until
```pascal
program CaseRepeat;
var Ch: Char; Done: Boolean;
begin
  Done := false;
  repeat
    Ch := 'B';
    case Ch of
      'A': WriteLn('Alpha');
      'B': WriteLn('Beta');
      'C': WriteLn('Gamma');
    else
      WriteLn('Other');
    end;
    Done := true;
  until Done;
end.
```

### Test 7: String operations
```pascal
program StringTest;
var S: string[20];
    I: Integer;
begin
  S := 'Hello';
  for I := 1 to 5 do
    Write(S[I]);
  WriteLn;
  WriteLn('Length: ', Length(S));
end.
```

### Test 8: Typed constants (TP-specific mutable const)
```pascal
program TypedConst;
const
  Counter: Integer = 0;
  Name: string[10] = 'MinZ';
var I: Integer;
begin
  for I := 1 to 5 do
    Counter := Counter + I;
  WriteLn(Name, ': ', Counter);
end.
```

### Test 9: Multi-dim array (from real games)
```pascal
program Board;
type TBoard = array[0..2, 0..2] of Byte;
var
  B: TBoard;
  R, C: Integer;
begin
  for R := 0 to 2 do
    for C := 0 to 2 do
      B[R, C] := R * 3 + C;
  WriteLn(B[1, 1]);
end.
```

### Test 10: Inline Z80 assembly (PASTA/80 style)
```pascal
program InlineAsm;
var X: Byte;
begin
  X := 42;
  inline($3E / $00);  { LD A, 0 }
end.
```

---

## Part 5: Implementation Plan

### Architecture (same as PLM/Lizp pattern)

```
Pascal source → lex → parse → pascal.AST → lower → *hir.Module
                                                        ↓
                                              HIR → MIR2 → PBQP → Z80
```

### Files to create

| File | Lines | Purpose |
|------|-------|---------|
| `pkg/pascal/lex.go` | ~200 | Tokenizer |
| `pkg/pascal/parse.go` | ~400 | Recursive descent parser |
| `pkg/pascal/ast.go` | ~100 | AST node types |
| `pkg/pascal/lower.go` | ~200 | AST → HIR lowering |
| `pkg/pascal/pascal_test.go` | ~200 | 10 test cases from above |
| `cmd/minzc/main.go` | +10 | Wire `.pas` extension |
| `pkg/nanz/parse.go` | +3 | Cross-lang import |
| **Total** | **~1100** | |

### Milestones

1. **Lex + Parse**: program/const/var/procedure/function, expressions, basic types → ~400 LOC
2. **Lower to HIR**: if/while/for/case/repeat, arrays, records → ~200 LOC
3. **Wire + Test**: compile sieve.pas to Z80 asm → validation
4. **Phase 2**: units, sets, with, file I/O

---

## Appendix: PASTA/80 all.pas Feature Coverage

The `tests/all.pas` from PASTA/80 (1500+ lines) tests:

- Comments: `(* *)`, `{ }`, `//`
- Constants: untyped, typed, arrays, booleans, reals
- Types: enum, record, subrange, string[N], array, typedef
- Variables: simple, array, multi-dim, record
- Control: if/else, case, while, repeat, for/downto, nested
- Expressions: arithmetic, relational, boolean, bit ops, precedence
- Procedures: value params, var params, nested, forward, overlay
- Functions: result assignment, recursion
- Strings: concat, length, indexing, copy, pos, delete, insert
- Pointers: new, dispose, nil, dereference
- Builtins: Inc, Dec, Odd, Abs, Sqr, Ord, Chr, Lo, Hi, Swap, Addr
- Assert: compile-time assertions
- Inline: Z80 machine code bytes
- Compiler directives: `{$A+}`, `{$S+}`, `{$R-}`

This is our **reference compliance test** for Phase 1.
