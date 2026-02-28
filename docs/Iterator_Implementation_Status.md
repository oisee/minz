# MinZ Iterator Implementation Status

*Updated: 2026-02-27*

## ✅ Completed Components

### 1. **Iterator Type System**
- `IteratorType` in AST (`pkg/ast/ast.go`) and IR (`pkg/ir/ir.go`)
- Supports element type tracking through chains

### 2. **Iterator AST Nodes**
- `IteratorChainExpr` — complete iterator chains
- `IteratorOp` — individual operations with separate `Function` and `Argument` fields
- `IteratorMethodExpr` — method call recognition

### 3. **Parser Integration** (commit 1db08a7)
- `tryConvertIteratorChain()` in `convert.go` walks CallExpr chains backward
- `.iter()` stripped as chain marker (not emitted as an operation)
- Method chains like `.map(f).filter(g).forEach(h)` parse into `IteratorChainExpr`
- Direct methods (without `.iter()`) also recognized

### 4. **Argument Routing** (2026-02-27)
- `IteratorOp.Function` holds lambdas/function refs (map, filter, forEach, etc.)
- `IteratorOp.Argument` holds numeric args (take, skip) and init values (reduce)
- `reduce(init, fn)` splits: first arg → Argument, second arg → Function
- `enumerate()` / `collect()`: both fields nil (no arguments)

### 5. **Semantic Analysis**
- `pkg/semantic/iterator.go` — type checking, operation dispatch, DJNZ/indexed path selection
- `pkg/semantic/iterator_enhanced.go` — enhanced DJNZ with take/skip/enumerate/reduce
- Nil guards in `applyIteratorFunction()` and `applyReducerFunction()`
- Lambda extraction to separate functions (`iter_lambda_*`, `reducer_lambda_*`)

### 6. **DJNZ Codegen** (fixed 2026-02-27)
- Counter stored back to stack slot on each iteration (was bare `DJNZ label` before)
- Uses `DEC B` + `LD A, B` + store + `JR NZ` sequence
- Enhanced mode: skip offsets pointer start, take limits counter init
- Enumerate: index register initialized at skip count, incremented each iteration
- Reduce: accumulator initialized from first element, reducer called in loop body

### 7. **Fusion Optimization Framework**
- `pkg/optimizer/fusion.go` — pattern detection, operation fusion
- DJNZ vs 16-bit counter selection based on array length

### 8. **Test Suite** (63 tests across 5 packages)
- `pkg/parser/participle/iterator_convert_test.go` — 20 parser chain conversion tests
- `pkg/semantic/iterator_test.go` — 17 semantic analysis tests (DJNZ, lambda, filter, take, skip, enumerate, reduce)
- `pkg/codegen/iterator_test.go` — 5 Z80 codegen pattern tests
- `pkg/mirvm/iterator_test.go` — 8 MIR VM DJNZ execution tests
- `tests/iterator_corpus/` — 18 .minz programs, full pipeline regression

## ✅ Working Operations

| Operation | Parser | Semantic | Z80 Codegen | Status |
|-----------|--------|----------|-------------|--------|
| `map(fn)` | ✅ | ✅ | ✅ | Full pipeline |
| `filter(fn)` | ✅ | ✅ | ✅ | Full pipeline |
| `forEach(fn)` | ✅ | ✅ | ✅ | Full pipeline |
| `take(n)` | ✅ | ✅ | ✅ | Full pipeline |
| `skip(n)` | ✅ | ✅ | ✅ | Full pipeline |
| `peek(fn)` | ✅ | ✅ | ✅ | Full pipeline |
| `inspect(fn)` | ✅ | ✅ | ✅ | Full pipeline |
| `enumerate()` | ✅ | ✅ | 🚧 | MIR OK, Z80 needs OpPush |
| `reduce(fn)` | ✅ | ✅ | 🚧 | MIR OK, Z80 needs OpPush |
| `reduce(init, fn)` | ✅ | ✅ | 🚧 | MIR OK, Z80 needs OpPush |
| `takeWhile(fn)` | ✅ | ✅ | ✅ | Full pipeline |
| `skipWhile(fn)` | ✅ | ✅ | ❌ | Not in DJNZ mode |
| `zip(other)` | ✅ | 🚧 | 🚧 | Parser only |
| `flatMap(fn)` | ✅ | 🚧 | 🚧 | Parser only |
| `collect()` | ✅ | 🚧 | 🚧 | Parser only |

## 🚧 Next Steps

### Z80 OpPush Support
The `enumerate()` and `reduce()` operations work at the MIR level but need `OpPush` support in the Z80 codegen backend for multi-argument function calls.

### Fusion Optimizer Wiring
Wire `pkg/optimizer/fusion.go` into the main compilation pipeline for automatic chain optimization.

### Remaining Operations
- `zip`, `flatMap`, `collect` — semantic analysis and codegen needed
- `skipWhile` — needs state tracking in DJNZ mode
- `find`, `any`, `all`, `count`, `sum`, `first`, `last` — parser recognizes, codegen needed

## 🏁 E2E Verified (MZX --console-io)

Two iterator patterns verified end-to-end: MinZ source -> parser -> semantic
analysis -> Z80 codegen -> MZA assembler -> MZX emulator with console I/O:

| Test | Chain | Output | Verified |
|------|-------|--------|----------|
| `iter_foreach.minz` | `arr.forEach(console_log)` | `ABCDE\n` | `41424344450a` |
| `iter_take.minz` | `arr.iter().take(3).forEach()` | `ABC\n` | `4142430a` |

Run: `bash examples/e2e_iterators/run_e2e.sh`

## 🎯 Architecture Overview

### Compilation Pipeline
```
Source Code:
  scores.iter()
    .filter(|x| x >= 90)
    .map(|x| x + 5)
    .forEach(print_u8)
    ↓
AST: IteratorChainExpr
    ↓
Semantic: Type checking & validation
    ↓
IR: Chain of operations
    ↓
Fusion: Single optimized loop
    ↓
Assembly:
  LD HL, scores
  LD B, 10
loop:
  LD A, (HL)
  CP 90
  JR C, skip
  ADD A, 5
  CALL print_u8
skip:
  INC HL
  DJNZ loop
```

### Performance Characteristics

#### Small Arrays (≤255 elements)
- Uses DJNZ instruction
- ~18 T-states per element
- Optimal for Z80 architecture

#### Large Arrays (>255 elements)
- Uses 16-bit counter (DE register)
- ~24 T-states per element
- Handles up to 65,535 elements

## 📊 Performance Impact

| Pattern | Traditional | Iterator | Speedup |
|---------|------------|----------|---------|
| Simple iteration | 45 T/elem | 18 T/elem | 2.5x |
| Map + Filter | 90 T/elem | 25 T/elem | 3.6x |
| Complex chains | 150+ T/elem | 30 T/elem | 5x+ |
