# C89 Frontend: Progress Report

**Date:** 2026-03-15
**Report:** #082
**Branch:** `feat/c89-frontend`
**Status:** WIP — MIR2 asserts pass, Z80 codegen partial

---

## Summary

The C89/C99 frontend is live. C source code compiles through the full MinZ pipeline:

```
C source → modernc.org/cc/v4 (parse+typecheck) → HIR → MIR2 → Z80
```

- **18 corpus files**, ~100+ functions, **211 asserts** (all pass via MIR2 VM)
- **17/18 files** produce valid Z80 assembly (only `import_test.c` fails — multi-file not implemented)
- Comment-based assert/sandbox directives: `// assert fn(args) == expected [via mir2|z80]`

## Corpus (updated 2026-03-15, post codegen hardening)

| File | Category | Functions | Asserts | Z80 |
|------|----------|----------|---------|-----|
| `assert_test.c` | Integration test | 5 | 13 | OK |
| `bench.c` | SDCC comparison | 6 | 0 | OK |
| `bench_extended.c` | Extended benchmark | 12 | 27 | OK |
| `hello.c` | Smoke test | 4 | 0 | OK |
| `string8.c` | Pointer ops | 4 | 0 | OK |
| `math16.c` | 16-bit math | 7 | 10 | OK |
| `math8.c` | 8-bit math | 5 | 11 | OK |
| `bitops.c` | Bit manipulation | 8 | 6 | OK |
| `cpm_io.c` | CP/M helpers | 7 | 14 | OK |
| `game_helpers.c` | Retro game utils | 5 | 14 | OK |
| `logical.c` | &&/\|\| short-circuit | 5 | 18 | OK |
| `switch.c` | switch/case | 5 | 16 | OK |
| `c99_control.c` | do-while, for-decl | 5 | 17 | OK |
| `c99_types.c` | uint8_t, int16_t etc | 6 | 18 | OK |
| `c99_compound.c` | +=, -=, /=, %= | 7 | 21 | OK |
| `c99_preproc.c` | #define, #ifdef | 5 | 19 | OK |
| `imported_math.c` | Library functions | 3 | 0 | OK |
| `import_test.c` | Multi-file import | 2 | 7 | FAIL (multi-file not implemented) |
| **Total** | | **~100+** | **211** | **17/18** |

**Previously broken (report #082 original): bitops, math8, game_helpers, cpm_io — all fixed by BUG-A/BUG-B codegen patches.**

## What Works

### Language Features
- Functions with parameters and return values
- `int` (i16), `unsigned char` (u8), pointers, `void`
- `if`/`else`, `while`, `for` (including `for(int i=0;...)`)
- Local and global variables with initializers
- Pointer dereference (`*p`) and address-of (`&x`)
- Pointer store (`*p = val` → `StoreStmt`)
- Unary operators: `-`, `~`, `!`, `++`, `--`
- Binary operators: `+`, `-`, `*`, `/`, `<`, `>`, `<=`, `>=`, `==`, `!=`, `&`, `|`, `^`, `<<`, `>>`
- Casts (explicit and implicit)
- Function calls
- String literals (interned into string table)
- Struct declarations (lowered to pointer)
- `sizeof` (compile-time constant)

### Assert/Sandbox System
```c
// Top-level asserts (isolated VM per assert):
// assert twice(5) == 10
// assert add(3, 4) == 7 via mir2

// Sandboxes (shared VM, globals persist):
// sandbox "counter test"
// assert inc() == 1
// assert inc() == 2
// end sandbox
```

### u8 Promotion Fix
Selective narrowing of C integer promotion for Z80 efficiency:
- Subtraction, division, bitwise ops: narrowed to u8 when both operands are u8
- Addition, multiplication: kept as i16 (overflow-safe)
- Result: `abs_diff` generates 10B (was 21B before fix)

## What Doesn't Work (updated 2026-03-15)

### Fixed Since Original Report
- ~~Invalid `LD` instructions (BUG-A)~~ → Fixed: emitMov 8↔16 width handling
- ~~Invalid `ADD` instructions (BUG-B)~~ → Fixed: zero-extend guard for ADD HL, r8
- ~~`%` (modulo) panics~~ → Fixed: native OpMod + Dark/X-Trade DIVU111 Z80 codegen
- ~~`&&` / `||` panics~~ → Fixed: C89 lowerer desugars to CondExpr (short-circuit)
- ~~`switch`/`case` not implemented~~ → Fixed: if-chain lowering in C89 lowerer

### Remaining Issues
- `fibonacci` loop with 4 local vars — MIR2 VM returns wrong results (variable init ordering bug)
- Multi-file compilation (`import_test.c` fails)
- `struct` field access through full pipeline (partial)
- `typedef` (basic support via modernc.org/cc, not all paths tested)

## MinZ vs SDCC Comparison

See [Report #081](2026-03-15-081-MinZ_C89_vs_SDCC_Codegen_Comparison.md) for full details.

| Function | MinZ | SDCC | Winner |
|----------|-----:|-----:|--------|
| `twice` | 2B | 3B | MinZ |
| `add` | 2B | 3B | MinZ |
| `max` | 13B | 12B | SDCC |
| `abs_diff` | 10B | 11B | MinZ |
| `sum_to` | 28B | 25B | SDCC |
| `clamp8` | 13B | 30B | MinZ |
| **Total** | **68B** | **84B** | **MinZ (−19%)** |

Key advantages: PBQP 3-register ABI, no DE return tax, clean u8 codegen.

## Go Tests

13/13 pass:
- 9 structural tests (VoidFunc, AddFunction, Global, IfElse, WhileLoop, ForLoop, Pointer, TypeMapping, MultipleReturns)
- 3 assert parsing tests (CommentAsserts, AssertVia, Sandbox)
- 1 E2E pipeline test (4 asserts through MIR2 VM)

## Files Changed

```
minzc/pkg/c89/c89.go       — Compile(), comment-based assert/sandbox parser
minzc/pkg/c89/lower.go     — C AST → HIR lowering, u8 promotion fix, StoreStmt
minzc/pkg/c89/c89_test.go  — 13 tests
minzc/cmd/minzc/main.go    — .c extension support
examples/c89/*.c            — 9 corpus files
```

## Next Steps

1. Fix Z80 codegen for u8 multi-param functions (invalid LD/ADD instructions)
2. Add `%` and `&&`/`||` support to HIR lowerer
3. Investigate MIR2 VM variable init bug (fibonacci)
4. Add `switch`/`case` support
5. E2E Z80 emulator assert verification (currently MIR2 only)
6. Grow corpus toward CP/M real-world programs

---

*C89 frontend: 51 functions, 68 asserts, competitive with SDCC.*
