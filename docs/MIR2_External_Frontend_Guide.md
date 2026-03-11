# MIR2 External Frontend Guide

**Audience**: Language implementers who want to target Z80 (and other retro architectures) without
writing their own register allocator, peephole optimizer, or code generator.

**The pitch**: MIR2 is to Z80 what LLVM IR is to x86. Write a frontend that lowers your language
to MIR2, and you get a production-quality Z80 backend for free.

---

## 1. What You Get for Free

By targeting MIR2 instead of writing your own Z80 codegen, you immediately get:

- **PBQP register allocator** — Partitioned Boolean Quadratic Programming, minimizes spill cost
  across the A/B/C/D/E/H/L/HL/DE/BC register set
- **Interprocedural contract optimization** — propagates calling convention choices across call
  graph, e.g. if a callee only ever needs its arg in HL, the caller keeps it in HL
- **67+ peephole patterns** — INC/DEC coalescing, CP 0 → AND A, DJNZ recognition, NEG tricks
- **LUTGen** — if a function takes a ranged integer param (e.g. `u8<0..63>`) and is pure, it is
  automatically replaced with a 64-byte lookup table (no work from you)
- **BranchEquiv** — VM-based optimizer proves redundant conditional branches and removes them
- **CondRetSink + CmpSubCarry** — `if (a < b) return b-a; return a-b;` emits as 4 instructions
- **ZX Spectrum compiled sprites** — via the SMC (`@smc`) subsystem
- **MIR2 VM** for compile-time evaluation of pure functions

---

## 2. The Three Entry Points

Choose based on how structured your language is.

### Entry Point A: HIR (easiest, recommended for new frontends)

HIR is a "typed AST": named variables, structured control flow (if/while/for), function calls. No
SSA, no basic blocks, no register assignment. If your language has those concepts, HIR is a
30-minute integration.

Both PL/M-80 (`pkg/plm`) and Nanz (`pkg/nanz`) compile to `*hir.Module`. The HIR lowerer
(`hir.LowerModule`) converts to MIR2 automatically.

**Go import path**: `github.com/minz/minzc/pkg/hir`

### Entry Point B: MIR2 Builder API (more control)

Build SSA form directly using `mir2.Builder`. You control block structure, register class hints,
and terminator types. This is the path if your language has explicit control flow graphs or you
want to experiment with DJNZ-loop generation.

**Go import path**: `github.com/minz/minzc/pkg/mir2`

### Entry Point C: `.mir2` text format

**Does not exist yet.** `mz` accepts `.mir` files (the old MIR1 text format via `pkg/mir`), but
there is no `.mir2` text parser. To add one, you would need to:

1. Write a text parser in `pkg/mir2/parse.go` that reconstructs `*mir2.Module`
2. Add `ext == ".mir2"` to the dispatch in `cmd/minzc/main.go:compile()`, routing to a new
   `compileFromMIR2()` function analogous to the existing `compileFromMIR()`
3. That function would call `pipeline.CompileMIR2Direct(m)` (also not yet written — the pipeline
   currently assumes HIR as the entry point via `CompileHIR`)

For now, the Go API (Entry Points A or B) is the only path.

---

## 3. HIR Data Structures

File: `pkg/hir/hir.go`

### Module

```go
type Module struct {
    Name       string
    Funcs      []*Func
    Globals    []mir2.Global      // module-level variables
    Structs    []*mir2.StructTy   // named struct type declarations
    Interfaces []*InterfaceDecl   // zero-cost interfaces (monomorphized)
    Strings    []string           // interned string literals
    Warnings   []string           // diagnostic warnings (use-before-init, etc.)
    Asserts    []Assert           // compile-time assertions, checked via MIR2 VM
}
```

### Function

```go
type Func struct {
    Name       string
    Params     []Param
    RetTy      mir2.Ty   // mir2.TyVoid for void functions
    RetTys     []mir2.Ty // non-empty → multiple return values
    Body       *Block    // nil for extern declarations
    IsExtern   bool
    ExternAddr uint16    // non-zero → CALL/RST to fixed address
}

type Param struct {
    Name     string
    Ty       mir2.Ty
    RegClass mir2.RegClass // 0 = auto-assign; non-zero overrides
    SMC      bool          // baked as LD HL,imm16 — not a register param
}
```

### Statements

All implement `Stmt` (via `hirStmt()`). The key ones:

```go
// Variable declaration (mutable; zero-initialized if Init == nil)
type VarDeclStmt struct {
    Name  string
    Ty    mir2.Ty
    Init  Expr    // nil = zero-initialized
}

// Assignment: target = val
// Target is VarRefExpr (named variable) or DerefExpr (*ptr write)
type AssignStmt struct {
    Target Expr
    Val    Expr
}

// Return from function
type ReturnStmt struct {
    Val  Expr   // single return; nil for void
    Vals []Expr // multi-return; non-empty overrides Val
}

// Conditional branch
type IfStmt struct {
    Cond Expr
    Then *Block
    Else *Block // nil if no else
}

// Pre-condition loop
type WhileStmt struct {
    Cond Expr
    Body *Block
}

// Counted loop: for Var in Start..End
type ForRangeStmt struct {
    Var   string
    Start Expr
    End   Expr
    Body  *Block
}

// Expression statement (call for side effects)
type ExprStmt struct{ Expr Expr }

// Block is a sequence of statements
type Block struct{ Body []Stmt }
```

### Expressions

All implement `Expr` and carry `ExprTy() mir2.Ty`.

```go
type IntLitExpr  struct { Val int64;  Ty mir2.Ty }      // integer constant
type BoolLitExpr struct { Val bool }                     // true/false
type VarRefExpr  struct { Name string; Ty mir2.Ty }      // variable reference
type BinExpr     struct { Op string; L, R Expr; Ty mir2.Ty } // +,-,*,/,%,&,|,^,<<,>>,==,!=,<,<=,>,>=
type UnaryExpr   struct { Op string; X Expr; Ty mir2.Ty }    // -, !, ~
type CallExpr    struct { Fn string; Args []Expr; Ty mir2.Ty }
type FieldExpr   struct { X Expr; Field string; Offset int; Ty mir2.Ty }
type LoadExpr    struct { Ptr Expr; Ty mir2.Ty }         // *ptr (read)
type DerefExpr   struct { Ptr Expr; Ty mir2.Ty }         // *ptr (write target)
type AddrOfExpr  struct { Sym string }                   // address of global/string symbol
type CastExpr    struct { X Expr; Ty mir2.Ty }           // zero/sign-extend or truncate
type IndexExpr   struct { Base, Idx Expr; ElemTy mir2.Ty; ElemStride int }
```

### Convenience constructors (use these)

`hir.go` exports shorthand constructors so you don't have to `&`-initialize everything:

```go
hir.U8(42)                         // *IntLitExpr{Val:42, Ty:TyU8}
hir.U16(1000)                      // *IntLitExpr{Val:1000, Ty:TyU16}
hir.Var("x", mir2.TyU8)            // *VarRefExpr{Name:"x", Ty:TyU8}
hir.Add(l, r, mir2.TyU8)           // *BinExpr{Op:"+", ...}
hir.Sub(l, r, mir2.TyU8)           // *BinExpr{Op:"-", ...}
hir.Lt(l, r)                       // *BinExpr{Op:"<", Ty:TyBool}
hir.Eq(l, r)                       // *BinExpr{Op:"==", Ty:TyBool}
hir.Call("fn", mir2.TyU8, args...) // *CallExpr
hir.Load(ptr, mir2.TyU8)           // *LoadExpr
hir.Addr("my_global")              // *AddrOfExpr
hir.Cast(x, mir2.TyU16)            // *CastExpr

hir.Ret(expr)                      // *ReturnStmt{Val:expr}
hir.RetVoid()                      // *ReturnStmt{}
hir.Decl("x", mir2.TyU8, init)    // *VarDeclStmt
hir.Assign(target, val)            // *AssignStmt
hir.If(cond, thenBlk, elseBlk)    // *IfStmt (elseBlk may be nil)
hir.While(cond, body)             // *WhileStmt
hir.For("i", start, end, body)    // *ForRangeStmt
hir.Blk(stmts...)                 // *Block
```

---

## 4. MIR2 Type System

File: `pkg/mir2/types.go`

All types are singleton pointers; use `==` for equality.

```go
// Integers
mir2.TyU8, TyU16, TyU24, TyU32  // unsigned 8/16/24/32 bit
mir2.TyI8, TyI16, TyI24, TyI32  // signed 8/16/24/32 bit
mir2.TyBool                       // 1-bit semantic; allocated to ClassFlag when possible

// Pointer (16-bit on Z80)
mir2.TyPtr

// Void (function with no return value)
mir2.TyVoid

// Constructed types
mir2.NewArray(elem, length)         // fixed-size array [N]T
mir2.NewRanged(base, lo, hi)        // e.g. NewRanged(TyU8, 0, 64) → u8<0..64>
                                    //   hi is EXCLUSIVE (Go convention)
                                    //   LUTGen fires when range ≤ 256 and function is pure

// Struct type
st := &mir2.StructTy{
    Name:   "Vec2",
    Fields: []mir2.StructField{
        {Name: "x", Ty: mir2.TyU8},
        {Name: "y", Ty: mir2.TyU8},
    },
}
offset := st.ByteOffset(1) // byte offset of field 1 (= 1 for y here)

// Fixed-point types
mir2.TyF8_8   // 8 integer + 8 fractional bits (total 16-bit)
mir2.TyF8_16  // 8 + 16 = 24-bit; common for high-precision demos
```

---

## 5. MIR2 Builder API

File: `pkg/mir2/builder.go`

The Builder emits instructions into a function. Usage pattern:

```go
m := &mir2.Module{Name: "mymodule"}

f := m.AddFunc("my_func")    // always use AddFunc, not &Func{} directly
                              // (AddFunc wires the shared module-wide reg counter)
b := mir2.NewBuilder(f)
b.SwitchToNewBlock("entry")  // create and switch to entry block

// Declare parameters BEFORE emitting any instructions
aReg := b.Param("a", mir2.TyU16, mir2.ClassPointer)
bReg := b.Param("b", mir2.TyU16, mir2.ClassIndex)

// Arithmetic
sum := b.Add(aReg, bReg, mir2.TyU16, mir2.ClassPointer)

// Return
b.Ret(sum)
```

Key Builder methods:

```go
// Parameters — call before any emit
b.Param(name string, ty Ty, cls RegClass) Reg

// Constants
b.Const(val int64, ty Ty, cls RegClass) Reg

// Arithmetic / bitwise
b.Add(l, r Reg, ty Ty, cls RegClass) Reg
b.Sub(l, r Reg, ty Ty, cls RegClass) Reg
b.Mul(l, r Reg, ty Ty, cls RegClass) Reg
b.And(l, r Reg, ty Ty, cls RegClass) Reg
b.Or (l, r Reg, ty Ty, cls RegClass) Reg
b.Xor(l, r Reg, ty Ty, cls RegClass) Reg
b.Shl(l, r Reg, ty Ty, cls RegClass) Reg
b.Shr(l, r Reg, ty Ty, cls RegClass) Reg
b.Neg(src Reg, ty Ty, cls RegClass) Reg
b.BinOp(op Op, lhs, rhs Reg, ty Ty, cls RegClass) Reg  // generic form

// Comparison (result is TyBool, prefer ClassFlag)
b.Cmp(cond CmpCond, lhs, rhs Reg, cls RegClass, clsHard bool) Reg
// CmpCond values: CmpEq, CmpNe, CmpLt, CmpLe, CmpGt, CmpGe

// Type conversions
b.Ext  (src Reg, srcTy, dstTy Ty, cls RegClass) Reg  // zero-extend
b.Sext (src Reg, srcTy, dstTy Ty, cls RegClass) Reg  // sign-extend
b.Trunc(src Reg, srcTy, dstTy Ty, cls RegClass) Reg  // truncate

// Memory
b.Load  (ptr Reg, ty Ty, cls RegClass) Reg          // read *ptr
b.Store (ptr, val Reg, ty Ty)                        // write *ptr = val
b.AddrOf(sym string, cls RegClass) Reg               // address of global symbol
b.Field (ptr Reg, byteOffset int64, cls RegClass) Reg // ptr + constant offset
b.FieldOf(ptr Reg, st *StructTy, fieldIdx int, cls RegClass) Reg  // convenience

// Function calls
b.Call      (fn string, args []Reg, retTy Ty, cls RegClass, attr CallAttrs) Reg
b.CallVoid  (fn string, args []Reg, attr CallAttrs)
b.CallMulti (fn string, args []Reg, returns []Return, attr CallAttrs) []Reg

// Control flow (terminators — seal the current block)
b.Jmp  (target string, args ...Reg)
b.BrIf (cond Reg, then string, thenArgs []Reg, else_ string, elseArgs []Reg)
b.Ret  (vals ...Reg)     // return values positionally match Contract.Returns

// DJNZ loop (Z80-specific — counter must be in ClassCounter / register B)
b.DJNZ(counter Reg, body string, bodyArgs []Reg, exit string, exitArgs []Reg)

// Block management
b.SwitchToNewBlock(label string) *Block
b.SwitchTo(blk *Block)
b.BlockParam(blk *Block, ty Ty, cls RegClass) Reg  // MIR2 uses block params, not phi nodes
```

### Block params instead of phi nodes

MIR2 uses explicit block parameters (like MLIR/WebAssembly) instead of phi nodes. When branching
to a block, pass values as arguments:

```go
loopHead := b.SwitchToNewBlock("loop_head")
iParam   := b.BlockParam(loopHead, mir2.TyU8, mir2.ClassCounter)  // loop counter

// ... emit loop body ...

// back-edge: pass updated i to loop_head
iNext := b.Sub(iParam, b.Const(1, mir2.TyU8, mir2.ClassGeneral), mir2.TyU8, mir2.ClassCounter)
b.BrIf(cond, "loop_head", []mir2.Reg{iNext}, "loop_exit", nil)
```

---

## 6. Register Classes

File: `pkg/mir2/regs.go`

Register classes are hints (soft by default) to the allocator. The Z80 calling convention uses:

| Class | Z80 Physical Register | Use When |
|-------|----------------------|----------|
| `ClassAcc` | A | Return values (u8), accumulator results |
| `ClassCounter` | B | Loop counters (required for DJNZ) |
| `ClassPointer` | HL | Pointers, 16-bit primary results |
| `ClassIndex` | DE | Secondary pointers, 16-bit secondary args |
| `ClassGeneral` | any 8-bit | C, D, E, H, L — spillable |
| `ClassFlag` | CPU flags Z/C | bool results from comparisons |
| `ClassPair` | BC or DE | 16-bit pair operations |

**Calling convention summary** for a function `fun foo(a: u16, b: u8) -> u8`:
- First u16 param → `ClassPointer` (HL)
- First u8 param → `ClassIndex` or `ClassGeneral` (DE, C, etc.)
- u8 return → `ClassAcc` (A)
- u16 return → `ClassPointer` (HL)

Setting classes on HIR params:
```go
&hir.Param{Name: "ptr", Ty: mir2.TyPtr,  RegClass: mir2.ClassPointer}
&hir.Param{Name: "n",   Ty: mir2.TyU8,   RegClass: mir2.ClassCounter}
&hir.Param{Name: "val", Ty: mir2.TyU8,   RegClass: mir2.ClassAcc}
```

If you leave `RegClass` as 0 (`ClassGeneral`), the allocator and contract optimizer will choose.
For most cases this is fine. Set explicit classes only when the generated Z80 must match a
specific ABI (e.g. calling ROM routines, BIOS entry points).

---

## 7. The Pipeline

File: `pkg/pipeline/pipeline.go`

### HIR path (recommended)

```go
import (
    "github.com/minz/minzc/pkg/hir"
    "github.com/minz/minzc/pkg/mir2"
    "github.com/minz/minzc/pkg/pipeline"
)

hm := &hir.Module{Name: "mymodule", Funcs: []*hir.Func{ /* ... */ }}

// Returns Z80 assembly text (.a80 format)
asmText, err := pipeline.CompileHIR(hm)

// Or with options
asmText, err = pipeline.CompileHIRWithOptions(hm, pipeline.Options{
    ContractOpt:     true,   // interprocedural contract optimization
    AnnotateTStates: false,  // annotate each instruction with T-state cost
})

// Or get all intermediate representations
steps, err := pipeline.CompileHIRSteps(hm, pipeline.Options{ContractOpt: true})
// steps.HIR      — HIR structural dump (before lowering)
// steps.MIR2Raw  — MIR2 dump before optimization passes
// steps.MIR2Opt  — MIR2 dump after DSE + ReorderBlocks + CondRetSink etc.
// steps.Assembly — final .a80 assembly text

// Assemble to binary
binary, errs := pipeline.Assemble(asmText, "zxspectrum") // or "cpm", "generic"
```

### MIR2 direct path

There is no `pipeline.CompileMIR2()` function yet. If you build a `*mir2.Module` directly via the
Builder API, run the pipeline stages manually:

```go
import (
    "fmt"
    "github.com/minz/minzc/pkg/mir2"
    "github.com/minz/minzc/pkg/pipeline"
)

m := &mir2.Module{Name: "mymodule"}
// ... build using m.AddFunc + Builder ...

// Run optimization passes (same sequence as pipeline.CompileHIRWithOptions)
for _, f := range m.Funcs {
    mir2.EliminateDeadBlocks(f)
    mir2.ReorderBlocks(f)
    for {
        p := mir2.PropagateConstants(f)
        c := mir2.FoldConstants(f)
        e := mir2.ConstantCallElim(m, f)
        if !p && !c && !e { break }
    }
    mir2.DeadStoreElim(f)
    if mir2.BranchEquiv(m, f) {
        mir2.EliminateDeadBlocks(f)
        mir2.DeadStoreElim(f)
    }
    if mir2.CondRetSink(f) {
        mir2.EliminateDeadBlocks(f)
    }
}

if err := mir2.Verify(m); err != nil {
    return fmt.Errorf("MIR2 verify: %w", err)
}

ct := mir2.Z80CostTable{}
cs := mir2.OptimizeContracts(m, ct)
mir2.ApplyContracts(m, cs)

combined := &mir2.AllocResult{Locs: make(map[mir2.Reg]mir2.PhysLoc)}
for _, f := range m.Funcs {
    lr := mir2.ComputeLiveness(f)
    ar := mir2.PBQPAllocate(f, lr, ct)
    for r, loc := range ar.Locs {
        combined.Locs[r] = loc
    }
}

asmText := mir2.Z80Codegen(m, combined)
```

---

## 8. Minimal "Hello World" — `add(u16, u16) -> u16`

A complete Go program that builds a HIR module, compiles it, and prints Z80 assembly:

```go
package main

import (
    "fmt"
    "github.com/minz/minzc/pkg/hir"
    "github.com/minz/minzc/pkg/mir2"
    "github.com/minz/minzc/pkg/pipeline"
)

func main() {
    // Build HIR module for: fun add(a: u16, b: u16) -> u16 { return a + b }
    m := &hir.Module{Name: "example"}

    f := &hir.Func{
        Name:  "add",
        Params: []hir.Param{
            {Name: "a", Ty: mir2.TyU16, RegClass: mir2.ClassPointer},
            {Name: "b", Ty: mir2.TyU16, RegClass: mir2.ClassIndex},
        },
        RetTy: mir2.TyU16,
        Body: hir.Blk(
            hir.Ret(
                hir.Add(
                    hir.Var("a", mir2.TyU16),
                    hir.Var("b", mir2.TyU16),
                    mir2.TyU16,
                ),
            ),
        ),
    }
    m.Funcs = append(m.Funcs, f)

    // Compile to Z80 assembly
    asm, err := pipeline.CompileHIR(m)
    if err != nil {
        panic(err)
    }
    fmt.Println(asm)
    // Expected output (approximate):
    //   add:
    //       ADD HL, DE
    //       RET
}
```

### Same function via the MIR2 Builder API

```go
m := &mir2.Module{Name: "example"}

f := m.AddFunc("add")
b := mir2.NewBuilder(f)
b.SwitchToNewBlock("entry")

aReg := b.Param("a", mir2.TyU16, mir2.ClassPointer)
bReg := b.Param("b", mir2.TyU16, mir2.ClassIndex)

// Also declare the return class in the Contract
f.Contract.Returns = []mir2.Return{
    {Ty: mir2.TyU16, Class: mir2.ClassPointer},
}

sum := b.Add(aReg, bReg, mir2.TyU16, mir2.ClassPointer)
b.Ret(sum)

// Then run the manual pipeline sequence shown in Section 7.
```

---

## 9. Globals and Memory-Mapped I/O

```go
// Simple u8 global (initialized to 0)
m.Globals = append(m.Globals, mir2.Global{
    Name: "counter",
    Ty:   mir2.TyU8,
    Init: []byte{0},
})

// Read-only lookup table
m.Globals = append(m.Globals, mir2.Global{
    Name:    "sin_table",
    Ty:      mir2.NewArray(mir2.TyU8, 256),
    Init:    sinBytes, // []byte of length 256
    IsConst: true,
})

// Memory-mapped I/O at fixed address (ZX Spectrum ULA port)
// In HIR: use ConstPtrExpr{Addr: 0xFE}
// In MIR2: b.Const(0x00FE, mir2.TyPtr, mir2.ClassPointer)
```

In HIR, reference a global by name:

```go
// Read: load value of "counter" into register
hir.Load(hir.Addr("counter"), mir2.TyU8)

// Write: *counter = 42
hir.Assign(
    &hir.DerefExpr{Ptr: hir.Addr("counter"), Ty: mir2.TyU8},
    hir.U8(42),
)
```

---

## 10. Struct Types

Struct layouts are packed (no padding). Field access uses byte offsets:

```go
color := &mir2.StructTy{
    Name: "Color",
    Fields: []mir2.StructField{
        {Name: "r", Ty: mir2.TyU8},
        {Name: "g", Ty: mir2.TyU8},
        {Name: "b", Ty: mir2.TyU8},
    },
}
// color.ByteOffset(0) == 0  (r)
// color.ByteOffset(1) == 1  (g)
// color.ByteOffset(2) == 2  (b)

// In HIR: access g field of a pointer-to-Color
// base is a TyPtr pointing at a Color
gExpr := &hir.FieldExpr{
    X:      hir.Var("base", mir2.TyPtr),
    Field:  "g",
    Offset: color.ByteOffset(1), // 1
    Ty:     mir2.TyU8,
}
```

Register `color` with the module so the HIR lowerer knows about it:

```go
m.Structs = append(m.Structs, color)
```

---

## 11. LUTGen — Free Lookup Table Optimization

If your language has functions that map a bounded integer to another value, annotate the parameter
type with `mir2.NewRanged`:

```go
// In HIR: fun square(x: u8<0..16>) -> u8 { return x * x }
f := &hir.Func{
    Name: "square",
    Params: []hir.Param{
        {Name: "x", Ty: mir2.NewRanged(mir2.TyU8, 0, 17)}, // hi=17 (exclusive)
    },
    RetTy: mir2.TyU8,
    Body: hir.Blk(
        hir.Ret(
            hir.Mul(hir.Var("x", mir2.TyU8), hir.Var("x", mir2.TyU8), mir2.TyU8),
        ),
    ),
}
```

The pipeline's `mir2.LUTGen(m)` pass automatically:
1. Evaluates `square(0)` through `square(16)` via the MIR2 VM
2. Emits a 17-byte global `square_lut: DB 0,1,4,9,16,...`
3. Replaces the function body with `LD HL, square_lut / ADD HL, DE / LD A, (HL) / RET`

No changes needed in your frontend beyond the `RangedTy` annotation.

---

## 12. What Works Today vs. What Needs Work

| Feature | Status |
|---------|--------|
| HIR path (functions, if, while, for, structs, calls) | Production — 26/26 PL/M files parse; Nanz used in active development |
| MIR2 Builder API | Production — used internally by HIR lowerer |
| PBQP register allocator | Production |
| Peephole optimizer (67+ patterns) | Production |
| LUTGen | Production — tested with popcnt, sin tables, etc. |
| BranchEquiv, CondRetSink, CmpSubCarry | Production |
| Interprocedural contract optimization | Production |
| Z80 codegen for all MIR2 ops | Production |
| Multiple return values | Production |
| DJNZ terminator | Production |
| SMC / PatchSlot | Production |
| `.mir2` text file input (`mz foo.mir2`) | **Not implemented** |
| `pipeline.CompileMIR2(m)` convenience function | **Not implemented** (use manual sequence) |
| Iterator chain fusion (DJNZ loops from forEach/map/filter) | Working E2E (11/11), ~5x T-state overhead vs ideal |
| Enumerate, reduce in iterator chains | **Broken on Z80** (register conflicts) |
| `ptr[i]` inside while loops | **Known bug** (BUG-003 — invalid EX DE,HL) |
| Zero-size struct globals | **Known bug** (BUG-006 — linker symbol not emitted) |

The HIR path is the recommended starting point for a new frontend. Budget ~1 day to get a basic
"function with arithmetic and if/while" working end-to-end from your AST to Z80 binary.

---

## 13. Worked Example: Fibonacci

```go
// fun fib(n: u8) -> u16 {
//     if n <= 1 { return (u16)n }
//     return fib(n-1) + fib(n-2)
// }

m := &hir.Module{Name: "fib_example"}
m.Funcs = append(m.Funcs, &hir.Func{
    Name:  "fib",
    Params: []hir.Param{{Name: "n", Ty: mir2.TyU8}},
    RetTy:  mir2.TyU16,
    Body: hir.Blk(
        hir.If(
            hir.Le(hir.Var("n", mir2.TyU8), hir.U8(1)),
            hir.Blk(hir.Ret(hir.Cast(hir.Var("n", mir2.TyU8), mir2.TyU16))),
            nil, // no else
        ),
        hir.Ret(
            hir.Add(
                hir.Call("fib", mir2.TyU16,
                    hir.Sub(hir.Var("n", mir2.TyU8), hir.U8(1), mir2.TyU8)),
                hir.Call("fib", mir2.TyU16,
                    hir.Sub(hir.Var("n", mir2.TyU8), hir.U8(2), mir2.TyU8)),
                mir2.TyU16,
            ),
        ),
    ),
})

asm, err := pipeline.CompileHIR(m)
```

Note: `hir.Sub` takes `(l, r Expr, ty Ty)` — see the convenience constructors in `hir.go`.

---

## 14. Module Path and Build

All packages live under `github.com/minz/minzc`. Add the repo to your Go workspace or use
`replace` in your `go.mod`:

```
require github.com/minz/minzc v0.0.0

replace github.com/minz/minzc => /path/to/minz-ts/minzc
```

Then import what you need:

```go
import (
    "github.com/minz/minzc/pkg/hir"
    "github.com/minz/minzc/pkg/mir2"
    "github.com/minz/minzc/pkg/pipeline"
)
```

Build the compiler once first to make sure all packages compile:

```bash
cd /path/to/minz-ts/minzc && make build
```
