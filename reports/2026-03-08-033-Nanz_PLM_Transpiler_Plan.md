# Report #033 — Nanz Language & PL/M→Nanz Transpiler

**Date:** 2026-03-08
**Status:** Nanz MVP done; PLM→Nanz transpiler in progress

---

## 1. What Is Nanz?

**Nanz** ("nano MinZ") is a tiny HIR-native language.  It serves two purposes:

1. **Standalone**: write programs that compile directly to HIR → MIR2 → Z80.
2. **Transpilation target**: PL/M-80 source → HIR → Nanz gives human-readable modern source.

Nanz is deliberately minimal — every construct maps 1:1 to a HIR node.
The syntax is MinZ/Swift-flavoured, making transpiled PL/M-80 immediately readable.

---

## 2. Nanz Syntax Reference

```
struct Point {
    x: u8
    y: u8
}

global counter: u8
global vram: u8 at(0x4000)           // AT() — absolute address
global screen: [u8; 6912] at(0x4000) // array at fixed address

fun fill(buf: ^u8, len: u8) {        // ^T = pointer to T (prefix)
    var i: u8 = 0
    while (i < len) {
        buf[i] = 0                   // array index via ptr
        i = (i + 1)
    }
}

fun sum_bytes(buf: ^u8, len: u8) -> u16 {
    var total: u16 = 0
    for b: u8 in buf[0..len] {      // ForEachStmt — DJNZ-friendly loop
        total = (total + u16(b))
    }
    return total
}

fun io_example() {
    let port = @ptr(u8, 0xFE)       // typed const ptr (meta-function)
    let val = port^                  // postfix ^ = dereference (load)
    port^ = 255                      // postfix ^ = store through ptr
}
```

### Key Syntax Decisions

| Construct | Syntax | HIR node |
|-----------|--------|----------|
| Pointer type | `^T` | `mir2.TyPtr` (prefix) |
| Load through pointer | `ptr^` | `LoadExpr` (postfix) |
| Store through pointer | `ptr^ = val` | `StoreStmt` |
| Absolute-address ptr | `@ptr(T, 0xFE)` | `ConstPtrExpr` |
| Array type | `[T; N]` | `mir2.ArrayTy` |
| Array index | `arr[i]` | `IndexExpr` |
| Array global | `global a: [u8; N] at(0x4000)` | `Global{At, Ty=ArrayTy}` |
| Local array | `var a: [u8; N]` | `VarDeclStmt{ArrayLen=N}` |
| Typed iterator | `for x: T in ptr[0..len]` | `ForEachStmt` |
| Range loop | `for i in 0..n` | `ForRangeStmt` |
| Type inference | `let x = expr` | `VarDeclStmt{Init=expr, Ty=inferred}` |

---

## 3. What Is Implemented (as of this report)

### 3.1 HIR Extensions (`pkg/hir/hir.go`)

| Addition | Description |
|----------|-------------|
| `Module.Structs []*mir2.StructTy` | Named struct declarations |
| `VarDeclStmt.ArrayLen int` | Local array: `[ArrayLen]Ty` |
| `VarDeclStmt.Initial []Expr` | Array initializer values |
| `VarDeclStmt.At *uint16` | AT(addr) absolute address |
| `ConstPtrExpr{ElemTy, Addr}` | `@ptr(T, addr)` — typed const pointer |
| `ForEachStmt{Var, ElemTy, Ptr, Start, Len, Body}` | DJNZ-friendly element iterator |
| `StoreStmt{Ptr, Val}` | Write through a pointer |

### 3.2 HIR Lowering Extensions (`pkg/hir/lower.go`)

| Addition | Description |
|----------|-------------|
| `lowerForEach` | ForEachStmt → MIR2 DJNZ-friendly block |
| `ConstPtrExpr` case | → `OpConst(addr, TyPtr, ClassPointer)` |
| `StoreStmt` case | → `OpStore` |

### 3.3 MIR2 Extension (`pkg/mir2/module.go`)

| Addition | Description |
|----------|-------------|
| `Global.At *uint16` | Absolute address placement (→ EQU in asm) |

### 3.4 Nanz Parser (`pkg/nanz/parse.go`)

Full hand-written lexer + recursive descent parser.  Key features:
- `^T` in type position → `TyPtr`
- `ptr^` postfix → `LoadExpr`
- `ptr^ = val` → `StoreStmt`
- `@ptr(T, addr)` → `ConstPtrExpr`
- `let x = expr` with type inference from RHS
- `for x: T in ptr[start..end]` → `ForEachStmt`
- `for i in start..end` → `ForRangeStmt`
- `global`, `var`, `struct`, `fun`, `@extern`, `switch/case/default`, `break`, `continue`
- `at(addr)` on globals and locals
- Array types `[T; N]`, array initializers `= [1,2,3]`

**Bug fixed (this session):** `parseFor` used `parseUnary` which internally called `parsePostfix`, which greedily ate `buf[` as an index expression before we could detect the `[start..end]` slice form.  Fix: use `parsePrimary + parsePostfixNoBrack` so `[` is left for the slice disambiguation.

### 3.5 Nanz Printer (`pkg/nanz/print.go`)

HIR → Nanz text. Fully round-trippable (parse → print → parse succeeds).
Handles all HIR nodes including ForEachStmt, ConstPtrExpr, StoreStmt, struct declarations, globals with AT.

### 3.6 Test coverage

| Test | Status |
|------|--------|
| `TestParse` | ✅ struct, globals with at(), 4 functions |
| `TestPrint` | ✅ key constructs present in output |
| `TestParseRoundtrip` | ✅ parse→print→parse (no error) |
| `TestForEach` | ✅ for x: T in ptr[0..len], shape check, roundtrip |
| `TestAtDecl` | ✅ global port: u8 at(0xFE), global screen: [u8; 6912] at(0x4000) |

---

## 4. What Is NOT Yet Implemented

### 4.1 PL/M Lowerer Gaps (`pkg/plm/lower.go`)

| Feature | Current state | Target state |
|---------|--------------|--------------|
| `DECLARE x BYTE AT(addr)` | AT address ignored | `Global.At = addr` |
| `DECLARE arr(N) BYTE` | Size ignored, emitted as scalar | `Global.Ty = ArrayTy{u8, N}` |
| `DECLARE x BASED y BYTE` | Emitted as regular global | Pointer alias: access via `^y` |
| `arr(i)` in expr | Treated as `CallExpr` | Detected in lowerer → `IndexExpr` |
| `arr(i) = val` in stmt | Index lost, emitted as plain assign | `StoreStmt{Ptr: Index(arr,i), Val}` |
| Local AT variables | Ignored | `VarDeclStmt.At = addr` |
| Local arrays | Emitted as scalar | `VarDeclStmt{ArrayLen=N}` |

### 4.2 CLI (`cmd/minzc/main.go`)

No `--from=plm` or `--emit=nanz` flags yet.
Plan: detect `.plm` extension automatically; add `--emit=nanz` to print HIR as Nanz source.

---

## 5. Implementation Plan

### Phase 1: PL/M Lowerer Improvements

**Step 1** — Track array metadata in lowerer:
- `modLowerer`: add `globalArrays map[string]int`, `globalBased map[string]string`, `globalAt map[string]*uint16`
- `procLowerer`: add `localArrays map[string]int`, `localBased map[string]string`

**Step 2** — Fix global variable lowering:
```go
// AT + arrays:
g := mir2.Global{Name: name, Ty: elemTy}
if d.Size != nil {
    g.Ty = &mir2.ArrayTy{Elem: elemTy, Len: *d.Size}
}
if d.AtAddr != nil { g.At = d.AtAddr }
// BASED: skip global emit; track in globalBased
```

**Step 3** — Fix local variable lowering:
```go
// Arrays:
if d.Size != nil {
    stmts = append(stmts, &hir.VarDeclStmt{Name: name, Ty: elemTy, ArrayLen: *d.Size})
} else {
    stmts = append(stmts, hir.Decl(name, elemTy, nil))
}
// AT:
if d.AtAddr != nil { decl.At = d.AtAddr }
// BASED: add to localBased, no decl
```

**Step 4** — Fix array subscript reads in `lowerExpr`:
```go
case *CallExpr:
    if size, ok := pl.arraySize(e.Fn); ok && size > 0 && len(e.Args)==1 {
        // Array subscript read: arr(i) → arr[i]
        idx, _ := pl.lowerExpr(e.Args[0])
        base := hir.Addr(e.Fn)  // &arr
        return hir.Index(base, idx, pl.elemTy(e.Fn)), nil
    }
    // ... normal call lowering
```

**Step 5** — Fix array subscript writes in `lowerStmt(AssignStmt)`:
- Add `Idx Expr` field to `plm.AssignStmt`
- Parser: save `args[0]` as `Idx` when detecting `NAME(idx) = val`
- Lowerer: if `s.Idx != nil`, emit `StoreStmt` through indexed pointer

**Step 6** — BASED variable lowering:
- `lowerExpr(VarRef{X})` where X is BASED Y → `LoadExpr{Ptr: Var(Y, TyPtr), Ty: elemTy}`
- `lowerStmt(AssignStmt{Name:X})` where X is BASED Y → `StoreStmt{Ptr: Var(Y, TyPtr), Val}`

### Phase 2: CLI Integration

Add `--emit=nanz` flag and `.plm` input detection:

```go
// cmd/minzc/main.go
var emitFormat string  // "nanz", "hir", "mir", ...
rootCmd.Flags().StringVar(&emitFormat, "emit", "", "emit format: nanz")

// In run:
if emitFormat == "nanz" || filepath.Ext(sourceFile) == ".plm" {
    m, err := plm.Compile(src)
    if emitFormat == "nanz" {
        fmt.Print(nanz.Print(m))
        return
    }
    // ... continue to codegen
}
```

---

## 6. PL/M → Nanz Mapping Summary

| PL/M-80 | Nanz |
|---------|------|
| `DECLARE x BYTE;` | `var x: u8` |
| `DECLARE x WORD;` | `var x: u16` |
| `DECLARE x ADDRESS;` | `var x: ^u8` |
| `DECLARE arr(N) BYTE;` | `var arr: [u8; N]` |
| `DECLARE x BYTE AT(0x4000);` | `var x: u8 at(0x4000)` |
| `DECLARE x BASED y BYTE;` | *(no decl; access via `y^`)* |
| `arr(i)` | `arr[i]` |
| `arr(i) = val;` | `arr[i] = val` |
| `PROCEDURE f (a, b) BYTE;` | `fun f(a: u8, b: u8) -> u8` |
| `RETURN (expr);` | `return expr` |
| `DO WHILE cond; ... END;` | `while cond { ... }` |
| `DO i = 0 TO n-1; ... END;` | `for i in 0..n { ... }` |
| `DO CASE sel; arm0; arm1; END;` | `switch sel { case 0: ... }` |
| `ENABLE;` | `@ei()` |
| `DISABLE;` | `@di()` |
| `HALT;` | `@halt()` |
| `.name` (address-of) | `&name` |
| `BYTE(x)` | `u8(x)` |
| `WORD(x)` | `u16(x)` |
| `ADDRESS(x)` | `ptr(x)` |
| `.HIGH.(x)` | `(x >> 8)` |
| `.LOW.(x)` | `(x & 0xFF)` |

---

## 7. Conclusion

Nanz is now a fully functional HIR-native language with parser, printer, and HIR lowering.
The next step is wiring the PL/M→HIR lowerer to use all the new HIR fields (AT, arrays, BASED) and adding the `mz --emit=nanz` CLI flag, giving us a complete PL/M-80 → Nanz transpiler.
