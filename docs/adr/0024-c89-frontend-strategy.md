# ADR-0024: C89 Frontend Strategy

## Status
Proposed (2026-03-15)

## Context

C is the lingua franca of embedded/retro computing. Massive existing codebase for Z80: SDCC projects, z88dk libraries, CP/M utilities, game engines. MinZ already has 5 frontends (Nanz, Lanz, Lizp, PL/M-80, Pascal) all converging on HIR. Adding C89 completes the picture — lets users import existing C code into MinZ programs with zero rewrite, benefiting from PFCCO, PBQP register allocation, and all MIR2 optimizations that SDCC cannot do.

Key insight: `modernc.org/cc/v4` provides a complete C preprocessor, parser, and type-checker in pure Go. This eliminates ~80% of the work — we only need `cc.AST → *hir.Module` lowering.

## Decision

### Use modernc.org/cc/v4 as the C parser

```
C source → modernc.org/cc.Translate() → cc.AST (fully typed) → lower.go → *hir.Module
```

Benefits over hand-written parser:
- Preprocessor included (`#include`, `#define`, `#ifdef`, `__LINE__`, etc.)
- Full C type system already resolved (implicit conversions, integer promotions)
- Handles all C89 syntax + selective C99/C11/C23 extensions
- Battle-tested on millions of lines of C code

### Package structure

```
pkg/c89/
  lower.go          // cc.AST → *hir.Module (main work, ~500-800 LOC)
  lower_test.go     // unit tests
  types.go          // C type → MIR2 type mapping
  builtins.go       // __builtin_memcpy, __builtin_memset → LDIR
  sdcc_compat.go    // __sfr, __at, __interrupt annotation mapping
```

### C type → MinZ type mapping

| C type | MinZ type | Z80 size |
|---|---|---|
| `char`, `unsigned char`, `uint8_t` | `u8` | 1 byte |
| `signed char`, `int8_t` | `i8` | 1 byte |
| `int`, `short`, `unsigned short`, `uint16_t` | `u16` | 2 bytes |
| `signed int`, `signed short`, `int16_t` | `i16` | 2 bytes |
| `long`, `unsigned long`, `uint32_t` | `u32` | 4 bytes (if needed) |
| `_Bool` | `bool` | 1 byte |
| `void` | `void` | 0 |
| `T *` | `ptr` (u16) | 2 bytes |
| `struct S` | `hir.StructType` | sum of fields |
| `enum E` | `u8` or `u16` | depends on range |

**Note:** Z80 `int` = 16 bits (same as SDCC). This is correct for the target.

### Supported C features (Level 1 — MVP)

| Feature | HIR mapping | Effort |
|---|---|---|
| Functions | `hir.FuncDecl` | trivial |
| Local variables | `hir.VarDeclStmt` | trivial |
| Global variables | `hir.Global` | trivial |
| Arithmetic/bitwise ops | `hir.BinExpr` | trivial |
| if/else | `hir.IfStmt` | trivial |
| while/do-while | `hir.WhileStmt` | trivial |
| for loops | `hir.ForRangeStmt` or `hir.WhileStmt` | easy |
| switch (no fallthrough) | `hir.IfStmt` chain or `hir.CaseStmt` | easy |
| return | `hir.ReturnStmt` | trivial |
| struct (declaration + access) | `hir.StructType` + `hir.FieldExpr` | easy |
| typedef | alias map in lowering | trivial |
| #define / #include | handled by modernc.org/cc | free |
| Casts | `hir.CastExpr` | easy |
| sizeof | `hir.SizeofExpr` | trivial |
| Array declaration + indexing | `hir.ArrayType` + `hir.IndexExpr` | easy |

### Deferred C features (Level 2+)

| Feature | Difficulty | Why defer |
|---|---|---|
| `goto` | medium — need `hir.GotoStmt` | rare in embedded C |
| `union` | hard — overlapping storage | needs MIR2 alias analysis |
| Function pointers | hard — indirect calls | needs MIR2 `OpCallIndirect` |
| `switch` fallthrough | medium — desugar to goto chain | uncommon in clean code |
| `static` locals | easy — lower to globals with mangled names | 1 day |
| Comma operator | easy — desugar to sequence | trivial but rare |
| Variadic functions | see ADR-0026 | complex, @print is better |
| `long long` (64-bit) | hard — no native Z80 support | rare on Z80 |

### Integration with compileViaHIR

```go
// cmd/minzc/main.go — add ".c" to the extension switch
case ".c", ".h":
    hirMod, err = c89.Compile(string(src), sourceFile)
```

Cross-language import automatically works:
```nanz
import clib { c_function }   // finds clib.c, parses via modernc.org/cc
```

### Compile-time assert for C

```c
// C syntax — leverages _Static_assert
int double_val(int x) { return x + x; }

_Static_assert(double_val(5) == 10, "double_val broken");
// OR:
// #pragma minz assert double_val 5 == 10
```

Maps to `hir.Assert` — same dual-VM verification as all other frontends.

## Consequences

### Positive
- Import existing SDCC/z88dk C code without rewriting
- PFCCO + PBQP optimizations applied to C code (SDCC can't do this)
- Struct-return promotion (ADR-0025) → 4x faster multi-value returns
- @print replaces printf with zero-overhead compile-time expansion (ADR-0026)
- Cross-language interop: C functions callable from Nanz/Pascal/PL/M

### Negative
- modernc.org/cc is a large dependency (~50K LOC)
- Some C idioms (goto spaghetti, union type punning) hard to optimize
- Function pointers break whole-program analysis (deferred)

### Neutral
- C backend (output) in pkg/codegen/c.go unaffected — different direction
- mir2c (verification oracle) unaffected — different purpose

## Alternatives Considered

1. **Hand-written C parser** — rejected: 1-2 weeks vs 2-3 days, preprocessor alone is a week
2. **Tree-sitter C grammar** — rejected: no type information, need separate type checker
3. **SDCC fork** — rejected: different architecture, no HIR, maintenance nightmare
4. **Only support C via @extern FFI** — rejected: defeats purpose of cross-frontend optimization

## Effort Estimate

| Phase | Effort | Deliverable |
|---|---|---|
| modernc.org/cc integration + scaffold | 0.5 day | `go get`, basic test |
| Level 1 MVP (functions, vars, control flow, structs) | 2-3 days | hello.c → Z80 |
| Cross-language import | 0.5 day | `import clib { func }` |
| Compile-time assert | 0.5 day | `_Static_assert` → dual-VM |
| SDCC compat annotations | 1 day | `__sfr`, `__at`, `__interrupt` |
| **Total Level 1** | **~5 days** | |

## References

- [Report #077: Multi-Frontend Feasibility](../reports/2026-03-15-077-Multi_Frontend_Feasibility_And_Compiler_DSL.md)
- [Report #070: Language Frontends Gaps & Roadmap](../reports/2026-03-14-070-Language_Frontends_Gaps_Roadmap.md)
- ADR-0025: Struct-Return Promotion to Tuple
- ADR-0026: C stdlib, @print, and Variadics
- [modernc.org/cc/v4 documentation](https://pkg.go.dev/modernc.org/cc/v4)
- ADR-0014: PL/M-80 Frontend Strategy (template for frontend ADRs)
