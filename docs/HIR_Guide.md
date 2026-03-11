# HIR — High-Level Intermediate Representation Guide

**Package:** `github.com/minz/minzc/pkg/hir`
**Status:** Production (used by Nanz and PL/M-80 frontends)
**Pipeline position:** Frontend → **HIR** → MIR2 → Z80 asm

---

## What is HIR?

HIR is the bridge between language surface syntax and the register-level MIR2 IR.
It sits where a typed AST would sit in a traditional compiler:

```
Your language
    └─► hir.Module   (you build this)
           └─► LowerModule()   (pkg/hir/lower.go — free)
                  └─► mir2.Module
                         └─► pipeline.CompileHIR()   (free)
                                └─► .a80 Z80 assembly
```

**Why HIR and not MIR2 directly?**

| Feature | HIR | MIR2 |
|---------|-----|------|
| Variable names | ✅ named | ❌ virtual registers only |
| Control flow | ✅ structured (if/while/for) | ❌ basic blocks + branches |
| Types on expressions | ✅ every node | ✅ every instruction |
| Struct literals | ✅ `StructLitExpr` | ❌ manual field stores |
| Iterator fusion | ✅ `ForEachStmt` | ❌ manual DJNZ loops |
| Effort to write | **Low** | Medium |

If your language has functions, named variables, structured control flow, and
structs — HIR is the fastest path to Z80.

---

## The one function you need

```go
import (
    "github.com/minz/minzc/pkg/hir"
    "github.com/minz/minzc/pkg/pipeline"
)

m := &hir.Module{Name: "my_module"}
// ... fill m.Funcs, m.Globals, m.Structs ...

asm, err := pipeline.CompileHIR(m)
// asm is a .a80 string, ready to feed to MZA (mza file.a80 -o file.bin)
```

That's it. One call, full pipeline: HIR → MIR2 → PBQP allocation → Z80.

---

## Module

```go
type Module struct {
    Name       string
    Funcs      []*Func
    Globals    []mir2.Global       // global variables and constants
    Structs    []*mir2.StructTy    // named struct type declarations
    Interfaces []*InterfaceDecl    // structural interfaces (zero-cost)
    Strings    []string            // interned string literals
    Warnings   []string            // populated by pipeline (use-before-init etc.)
    Asserts    []Assert            // compile-time VM assertions
}
```

### Globals

```go
// global counter: u16 = 0
m.Globals = append(m.Globals, mir2.Global{
    Name: "counter",
    Ty:   mir2.TyU16,
    Init: []byte{0, 0},
})

// const lookup_table: [256]u8
m.Globals = append(m.Globals, mir2.Global{
    Name:    "lut",
    Ty:      &mir2.ArrayTy{Elem: mir2.TyU8, Len: 256},
    Init:    tableBytes,  // []byte of length 256
    IsConst: true,
})
```

### Structs

```go
st := &mir2.StructTy{Name: "Point", Fields: []mir2.StructField{
    {Name: "x", Ty: mir2.TyU8},
    {Name: "y", Ty: mir2.TyU8},
}}
m.Structs = append(m.Structs, st)
// st.ByteOffset(0) == 0  (x)
// st.ByteOffset(1) == 1  (y)
```

---

## Function

```go
type Func struct {
    Name       string
    Params     []Param
    RetTy      mir2.Ty   // mir2.TyVoid for void; overridden by RetTys if non-empty
    RetTys     []mir2.Ty // multi-return: []mir2.Ty{mir2.TyU16, mir2.TyU16}
    Body       *Block    // nil = extern declaration
    IsExtern   bool
    ExternAddr uint16    // @extern(0xNNNN) — RST N or CALL 0xNNNN
}

type Param struct {
    Name     string
    Ty       mir2.Ty
    RegClass mir2.RegClass  // 0 = auto; mir2.ClassPointer = force HL; etc.
    SMC      bool           // @smc: bake as LD HL,imm16 (not an ABI slot)
}
```

### Minimum working function

```go
// fun add(a: u16, b: u16) -> u16 { return a + b }
f := &hir.Func{
    Name:  "add",
    Params: []hir.Param{
        {Name: "a", Ty: mir2.TyU16},
        {Name: "b", Ty: mir2.TyU16},
    },
    RetTy: mir2.TyU16,
    Body: hir.Blk(
        hir.Ret(hir.Bin("+",
            hir.Var("a", mir2.TyU16),
            hir.Var("b", mir2.TyU16),
            mir2.TyU16,
        )),
    ),
}
m.Funcs = append(m.Funcs, f)
```

### Register class override

```go
// Force n into A (ClassAcc), result into HL (ClassPointer) — matches Z80 ABI
f := &hir.Func{
    Name: "double",
    Params: []hir.Param{
        {Name: "n", Ty: mir2.TyU8, RegClass: mir2.ClassAcc},
    },
    RetTy: mir2.TyU8,
    Body:  /* ... */,
}
```

**Z80 register classes:**

| Class | Register | Best for |
|-------|----------|----------|
| `ClassAcc` | A | u8 arithmetic, return value |
| `ClassPointer` | HL | u16 pointers, primary return |
| `ClassIndex` | DE | u16 second arg/return |
| `ClassCounter` | B | loop counter (→ DJNZ) |
| `ClassGeneral` | C/D/E/H/L | scratch |

Default calling convention assigns by position:
- pos 0 → ClassPointer (HL) for u16, ClassAcc (A) for u8
- pos 1 → ClassIndex (DE)
- pos 2 → ClassCounter (B) for u8

---

## Statements

All statements implement `hir.Stmt`. Build `*Block` from a slice of statements.

### Block

```go
// hir.Blk is a convenience constructor
body := hir.Blk(stmt1, stmt2, stmt3)
// or:
body := &hir.Block{Body: []hir.Stmt{stmt1, stmt2, stmt3}}
```

### Variable declaration

```go
// var x: u16 = 0
&hir.VarDeclStmt{Name: "x", Ty: mir2.TyU16,
    Init: &hir.IntLitExpr{Val: 0, Ty: mir2.TyU16}}

// var x: u16  (zero-initialised)
&hir.VarDeclStmt{Name: "x", Ty: mir2.TyU16}

// var buf: [8]u8  (array)
&hir.VarDeclStmt{Name: "buf", Ty: mir2.TyU8, ArrayLen: 8}

// var buf: [8]u8 = {1,2,3,4,5,6,7,8}
&hir.VarDeclStmt{Name: "buf", Ty: mir2.TyU8, ArrayLen: 8,
    Initial: []hir.Expr{
        &hir.IntLitExpr{Val: 1, Ty: mir2.TyU8},
        // ...
    },
}
```

### Assignment

```go
// x = expr
&hir.AssignStmt{
    Target: hir.Var("x", mir2.TyU16),
    Val:    someExpr,
}

// *ptr = val
&hir.AssignStmt{
    Target: &hir.DerefExpr{Ptr: hir.Var("ptr", mir2.TyPtr), Ty: mir2.TyU8},
    Val:    &hir.IntLitExpr{Val: 42, Ty: mir2.TyU8},
}
```

### Return

```go
// return              (void)
&hir.ReturnStmt{}

// return expr
&hir.ReturnStmt{Val: someExpr}

// return (a, b)       (multi-return)
&hir.ReturnStmt{Vals: []hir.Expr{exprA, exprB}}
```

### If / else

```go
// if cond { then }
&hir.IfStmt{
    Cond: condExpr,
    Then: hir.Blk(stmt1, stmt2),
}

// if cond { then } else { els }
&hir.IfStmt{
    Cond: condExpr,
    Then: hir.Blk(thenStmt),
    Else: hir.Blk(elseStmt),
}
```

### While

```go
// while cond { body }
&hir.WhileStmt{
    Cond: condExpr,
    Body: hir.Blk(bodyStmt),
}
```

### For range (counted loop)

```go
// for i in 0..n { body }
// Lowers to DJNZ loop when count is known.
&hir.ForRangeStmt{
    Var:   "i",
    Start: &hir.IntLitExpr{Val: 0, Ty: mir2.TyU8},
    End:   hir.Var("n", mir2.TyU8),
    Body:  hir.Blk(bodyStmt),
}
```

### For-each (iterator over memory)

```go
// for x: u8 in ptr[0..len] { body }
// Lowers to tight DJNZ pointer-walk in MIR2.
&hir.ForEachStmt{
    Var:    "x",
    ElemTy: mir2.TyU8,
    Ptr:    hir.Var("arr", mir2.TyPtr),
    Start:  &hir.IntLitExpr{Val: 0, Ty: mir2.TyU8},
    Len:    hir.Var("n", mir2.TyU8),
    Body:   hir.Blk(bodyStmt),
}
```

### Expression statement (call for side effects)

```go
// print(x)
&hir.ExprStmt{Expr: hir.Call("print", mir2.TyVoid,
    hir.Var("x", mir2.TyU8),
)}
```

### Pointer store

```go
// *ptr = val  (when ptr^ = val parses to StoreStmt)
&hir.StoreStmt{
    Ptr: hir.Var("ptr", mir2.TyPtr),
    Val: &hir.IntLitExpr{Val: 42, Ty: mir2.TyU8},
}
```

### Switch

```go
// switch val { case 0: ...; case 1: ...; default: ... }
&hir.SwitchStmt{
    Val: hir.Var("state", mir2.TyU8),
    Cases: []*hir.SwitchCase{
        {Val: 0, Body: hir.Blk(stmt0)},
        {Val: 1, Body: hir.Blk(stmt1)},
    },
    Default: hir.Blk(defaultStmt),
}
```

### Break / Continue

```go
&hir.BreakStmt{}
&hir.ContinueStmt{}
```

---

## Expressions

All expressions implement `hir.Expr` and carry a type via `ExprTy()`.

### Integer and bool literals

```go
&hir.IntLitExpr{Val: 42, Ty: mir2.TyU8}
&hir.IntLitExpr{Val: 1000, Ty: mir2.TyU16}
&hir.BoolLitExpr{Val: true}
```

### Variable reference

```go
hir.Var("x", mir2.TyU16)
// expands to: &hir.VarRefExpr{Name: "x", Ty: mir2.TyU16}
```

### Binary operation

```go
// a + b : u16
hir.Bin("+", hir.Var("a", mir2.TyU16), hir.Var("b", mir2.TyU16), mir2.TyU16)
// expands to: &hir.BinExpr{Op: "+", L: ..., R: ..., Ty: mir2.TyU16}

// Operators: + - * / % & | ^ << >> == != < <= > >=
```

### Unary operation

```go
// -x
&hir.UnaryExpr{Op: "-", X: hir.Var("x", mir2.TyU8), Ty: mir2.TyU8}
// !flag
&hir.UnaryExpr{Op: "!", X: hir.Var("flag", mir2.TyBool), Ty: mir2.TyBool}
```

### Function call

```go
// result = add(a, b) : u16
hir.Call("add", mir2.TyU16, hir.Var("a", mir2.TyU16), hir.Var("b", mir2.TyU16))
// expands to: &hir.CallExpr{Fn: "add", Args: [...], Ty: mir2.TyU16}
```

### Cast (type coercion)

```go
// (u16)(n)  — zero-extend u8 to u16
&hir.CastExpr{X: hir.Var("n", mir2.TyU8), Ty: mir2.TyU16}
```

### Address-of global

```go
// &counter
&hir.AddrOfExpr{Sym: "counter"}
// ExprTy() returns mir2.TyPtr
```

### Field access

```go
// p.x  (byte offset 0)
&hir.FieldExpr{
    X:      hir.Var("p", mir2.TyPtr),  // pointer to struct
    Field:  "x",
    Offset: 0,   // byte offset into struct
    Ty:     mir2.TyU8,
}
```

### Load through pointer

```go
// *ptr  (load u8 at ptr)
&hir.LoadExpr{Ptr: hir.Var("ptr", mir2.TyPtr), Ty: mir2.TyU8}
```

### Deref (left-hand side)

```go
// *ptr  used on left side of assignment
&hir.DerefExpr{Ptr: hir.Var("ptr", mir2.TyPtr), Ty: mir2.TyU8}
```

### Struct literal

```go
// Color{ r: 255, g: 0, b: 0 }
&hir.StructLitExpr{
    St: colorStructTy,   // *mir2.StructTy
    Fields: []hir.FieldInit{
        {Name: "r", Val: &hir.IntLitExpr{Val: 255, Ty: mir2.TyU8}},
        {Name: "g", Val: &hir.IntLitExpr{Val: 0,   Ty: mir2.TyU8}},
        {Name: "b", Val: &hir.IntLitExpr{Val: 0,   Ty: mir2.TyU8}},
    },
}
// In assignment context (ptr^ = StructLit), lowers to chained LD (HL),n; INC HL.
```

---

## Convenience constructors in hir.go

Several shorthands are available (check `pkg/hir/hir.go`):

```go
hir.Var(name, ty)          // *VarRefExpr
hir.Bin(op, l, r, ty)      // *BinExpr
hir.Call(fn, ty, args...)  // *CallExpr
hir.Blk(stmts...)          // *Block
hir.Ret(expr)              // *ReturnStmt
hir.Int(val, ty)           // *IntLitExpr
```

---

## Complete example: `abs_diff`

```go
package main

import (
    "fmt"
    "github.com/minz/minzc/pkg/hir"
    "github.com/minz/minzc/pkg/mir2"
    "github.com/minz/minzc/pkg/pipeline"
)

func main() {
    m := &hir.Module{Name: "abs_diff_example"}

    // fun abs_diff(a: u8, b: u8) -> u8 {
    //     if a > b { return a - b }
    //     return b - a
    // }
    a := hir.Var("a", mir2.TyU8)
    b := hir.Var("b", mir2.TyU8)

    f := &hir.Func{
        Name: "abs_diff",
        Params: []hir.Param{
            {Name: "a", Ty: mir2.TyU8},
            {Name: "b", Ty: mir2.TyU8},
        },
        RetTy: mir2.TyU8,
        Body: hir.Blk(
            &hir.IfStmt{
                Cond: hir.Bin(">", a, b, mir2.TyBool),
                Then: hir.Blk(hir.Ret(hir.Bin("-", a, b, mir2.TyU8))),
            },
            hir.Ret(hir.Bin("-", b, a, mir2.TyU8)),
        ),
    }
    m.Funcs = append(m.Funcs, f)

    asm, err := pipeline.CompileHIR(m)
    if err != nil {
        panic(err)
    }
    fmt.Println(asm)
}
```

**Output** (after all optimizer passes):
```z80
; fun abs_diff(a: u8 = A, b: u8 = B) -> u8 = A ; clobbers: F
abs_diff:
    SUB B
    RET NC
    NEG
    RET
```

4 instructions. The optimizer chain (CondRetSink → CmpSubCarry → BranchEquiv) fires automatically.

---

## HIR text format (`--emit=hir`)

Run `mz file.nanz --emit=hir` to see the HIR dump. Example output:

```
; HIR module: abs_diff_example

fun @abs_diff(a: u8, b: u8) -> u8
  if ((a:u8) > (b:u8)):bool
    return ((a:u8) - (b:u8)):u8
  return ((b:u8) - (a:u8)):u8
```

### Format rules

| Construct | Text format |
|-----------|------------|
| Module | `; HIR module: name` |
| Struct | `struct @Name { field: type, field: type }` |
| Global | `global @name : type` |
| Const global | `global const @name : type` |
| String | `string #N = "text"` |
| Function | `fun @name(p: type, ...) -> retTy` |
| Body indent | 2 spaces per nesting level |
| Var decl | `var name: type = expr` |
| Return | `return expr` / `return (e1, e2)` |
| If | `if expr` then indented block |
| Else | `else` then indented block |
| While | `while expr` then indented block |
| For range | `for var in start..end` then indented block |
| Pointer store | `*(ptr_expr) = val_expr` |
| Expression stmt | bare expression (typically a call) |
| Int literal | `N:type` e.g. `42:u8` |
| Bool literal | `true:bool` / `false:bool` |
| Variable ref | `(name:type)` |
| Binary op | `((L) op (R)):type` |
| Call | `call @fn(arg1, arg2):type` |
| Field access | `((base).field[+N]:type)` |
| Address-of | `addr @sym:ptr` |

### Text input (`.hir` files)

As of 2026-03-11, `mz` accepts `.hir` files: `mz file.hir -o file.a80`.

The parser (`pkg/hir/parse.go`, `ParseHIR(src, name string) (*Module, error)`) reads
the dump format. This enables:
- Third-party frontends that emit `.hir` text files
- Inspecting and hand-editing HIR before compiling
- Using `--emit=hir` as a round-trip debug tool

---

## Types reference

From `pkg/mir2/types.go`:

```go
// Singletons
mir2.TyVoid    // void
mir2.TyBool    // bool (1 bit, stored as u8)
mir2.TyU8      // unsigned 8-bit
mir2.TyU16     // unsigned 16-bit
mir2.TyI8      // signed 8-bit
mir2.TyI16     // signed 16-bit
mir2.TyU24     // unsigned 24-bit
mir2.TyU32     // unsigned 32-bit
mir2.TyPtr     // pointer (16-bit on Z80)

// Compound
&mir2.ArrayTy{Elem: mir2.TyU8, Len: 256}      // [256]u8
&mir2.RangedTy{Base: mir2.TyU8, Lo: 0, Hi: 64} // u8<0..63> (exclusive Hi)

// Struct (must match m.Structs entry)
&mir2.StructTy{Name: "Point", Fields: []mir2.StructField{
    {Name: "x", Ty: mir2.TyU8},
    {Name: "y", Ty: mir2.TyU8},
}}
// st.Width()        → 16  (total bits)
// st.ByteOffset(0)  → 0   (field x)
// st.ByteOffset(1)  → 1   (field y)
```

---

## What you get for free

When you write a frontend to HIR, the full MIR2 optimizer chain fires automatically:

| Optimization | What it does |
|-------------|--------------|
| CondRetSink | Hoists `if cond { return val }` into conditional return |
| CmpSubCarry | Fuses `sub(a,b) + cmpLt(a,b)` → carry already in F |
| BranchEquiv | Removes redundant `JP Z` after `CP` (VM-proven) |
| LUTGen | `u8<lo..hi>` param → compile-time lookup table |
| PBQP allocator | Weighted graph coloring, delta-sort for hot registers |
| Copy coalescing | Eliminates trampolines at block boundaries |
| Interprocedural contracts | Propagates register classes across call sites |
| Peepholes (67+) | INC/DEC consolidation, `AND A` → `CP 0`, DJNZ fusion, … |

**abs_diff goes from 8 instructions to 4 automatically.**

---

## Known limitations

| Issue | Impact | Tracking |
|-------|--------|---------|
| u16 loop variable allocation | `for i in 0..n` with u16 vars in body: dead moves at init, `ADD DE,HL` may appear despite ADD-routing fix | ADR-0006/0007 |
| ptr[i] in while loop | Invalid `EX DE,HL` / `ADD F,DE` | BUG-003 |
| Zero-size struct globals | Undefined symbol at link time | BUG-006 |
| Non-zero-lo LUT + contracts | Pipeline ordering conflict | BUG-004 |

---

## See also

- `pkg/hir/hir.go` — all type definitions
- `pkg/hir/lower.go` — HIR → MIR2 lowering
- `pkg/hir/dump.go` — HIR → text (for `--emit=hir`)
- `pkg/hir/parse.go` — text → HIR (`.hir` input files)
- `pkg/pipeline/pipeline.go` — `CompileHIR()` entry point
- `docs/MIR2_External_Frontend_Guide.md` — if you need MIR2-level control
- `docs/adr/0019-baked-sprite-codegen.md` — `@smc` parameter design
