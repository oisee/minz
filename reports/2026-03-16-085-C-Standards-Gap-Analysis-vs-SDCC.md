# C Standards Gap Analysis: MinZ C Frontend vs SDCC

**Date:** 2026-03-16
**Report:** #085
**Status:** Plan — use as implementation roadmap

---

## Executive Summary

MinZ's C89 frontend covers ~90% of C89, ~40% of C99, ~5% of C11, and 0% of C23.
SDCC (the reference Z80 C compiler) covers C89-C23 with documented deviations.
Closing the gap requires ~6 high-impact features (Tier 1) to compile ~80% of real
embedded C code. MinZ already beats SDCC on optimization (MIR2 pipeline, struct-return
promotion) and has unique features (ObjC, multi-frontend).

---

## Current State: MinZ vs SDCC

| Feature | SDCC | MinZ C89 | Gap |
|---------|------|----------|-----|
| **C89 core** | Full | ~90% | `goto`, `union`, `static` locals, function pointers missing |
| **C99** | Full (minus VLAs) | ~40% | Designated init, compound literals, `restrict`, `inline` missing |
| **C11** | Full | ~5% | `_Generic`, `_Static_assert` (parser only), anon structs/unions |
| **C23** | Partial | 0% | `typeof`, `nullptr`, `constexpr`, enum underlying type, `<stdbit.h>` |
| **stdlib** | printf/malloc/string/math | None | No libc |
| **Calling convention** | Register-based ABI v1 | Custom PFCCO | Interop impossible |
| **Optimizer** | Peephole + some | MIR2 pipeline | **MinZ ahead** |
| **Debug info** | CDB format | None | No source-level debug |

---

## What MinZ Already Supports

### Core Language (C89)
- Functions, parameters, return values, nested calls
- Local and global variables with initializers
- `int` (i16), `unsigned int` (u16), `char` (u8), `signed char` (i8), `void`, `_Bool`
- `typedef` declarations
- Struct declarations, field access (`.` and `->`), brace init `{a, b}`
- `enum` with auto-numbering and explicit values
- Pointers: `*` dereference, `&` address-of
- `sizeof` (compile-time)
- String literals (interned)

### Control Flow
- `if`/`else`, `while`, `do-while`, `for` (including C99 `for(int i=0;...)`)
- `switch`/`case`/`default` (lowered to if-else chains, no fallthrough)
- `break`, `continue`, `return`

### Expressions & Operators
- Arithmetic: `+`, `-`, `*`, `/`, `%`
- Bitwise: `&`, `|`, `^`, `~`, `<<`, `>>`
- Relational: `<`, `>`, `<=`, `>=`, `==`, `!=`
- Logical (short-circuit): `&&`, `||`, `!`
- Assignment: `=`, `+=`, `-=`, `*=`, `/=`, `%=`, `&=`, `|=`, `^=`, `<<=`, `>>=`
- Increment/decrement: `++`, `--` (prefix and postfix)
- Ternary: `cond ? a : b`
- Comma operator, cast expressions
- Array indexing `arr[i]`

### Advanced
- Struct-return promotion to tuple returns (ADR-0025, 0T overhead vs ~60T SDCC)
- C integer promotion with Z80-specific u8 narrowing
- Multi-file import via `#import` with resolver
- Comment-based assert/sandbox system with dual-VM verification
- ObjC extensions: classes, inheritance, method chaining, protocols

### Test Corpus
- 204/204 C89 corpus asserts passing (18 files)
- 30/30 ObjC asserts passing (5 files)

---

## Implementation Plan

### Tier 1 — "Blocking Real C Code" (enables ~80% of embedded C)

These 6 features prevent compiling most real-world C programs for Z80.

#### 1.1 Designated Initializers `{.x=1, .y=2}`
- **Effort:** Small (~20 lines in lowerer)
- **Impact:** Every struct-heavy program uses these
- **Status:** Parser already recognizes `Designation` field, lowerer skips it (line 1376)
- **Plan:** In `lowerStructInit`, read `Designation.DesignatorList` to map field name → index,
  then assign to correct slot instead of sequential.
- **Test:** Add to `c99_compound.c` corpus

#### 1.2 Static Local Variables
- **Effort:** Small
- **Impact:** Counters, caches, singletons, state machines
- **Plan:** Emit as mangled global: `static int x = 0` in `foo()` → global `foo__x`.
  Detect `StorageClassSpecifierStatic` in `lowerLocalDecl`, redirect to `l.hm.Globals`.
- **Test:** New `static_locals.c` corpus file

#### 1.3 `goto` + Labels
- **Effort:** Small — lowerer stub exists (line 715, returns nil)
- **Impact:** State machines, generated code, error cleanup patterns
- **Plan:** Lower labels to HIR label nodes, `goto` to HIR jump. MIR2 already has
  unconditional branch support.
- **Test:** New `goto_test.c` corpus file

#### 1.4 Compound Literals `(Point){1, 2}`
- **Effort:** Medium — new primary expression case
- **Impact:** Extremely common in C99+, used in function args and assignments
- **Plan:** In `lowerPrimary`, handle `PrimaryExpressionCompLit`. Allocate anonymous
  local, brace-init fields, return pointer. Reuse `lowerStructInit`.
- **Test:** Add to `c99_compound.c` or new `compound_lit.c`

#### 1.5 Function Pointers `int (*fn)(int)`
- **Effort:** Medium-Large — need indirect CALL in Z80 codegen
- **Impact:** Callbacks, sort comparators, event handlers, plugin patterns
- **Plan:**
  - Type system: function pointer = `TyPtr` (already done)
  - Lowerer: handle `PostfixExpressionCall` where callee is an expression, not just identifier
  - Z80 codegen: `CALL (HL)` or `JP (HL)` for indirect calls
  - MIR2: add `OpCallIndirect` instruction
- **Test:** New `func_ptr.c` with callback patterns

#### 1.6 Union Types
- **Effort:** Medium — overlapping storage in struct alloca
- **Impact:** Variant types, protocol parsing, bit manipulation, type punning
- **Plan:**
  - Parser: already handles `union` keyword via modernc.org/cc
  - Lowerer: `union` → struct where all fields share offset 0, size = max field size
  - FieldExpr offset always 0 for union fields
- **Test:** New `union_test.c` corpus file

---

### Tier 2 — "C99 Conformance" (expected by modern C developers)

#### 2.1 `inline` Functions
- **Effort:** Small
- **Impact:** Performance hint (common in headers)
- **Plan:** Accept and ignore (or inline at HIR level for small functions)

#### 2.2 Variadic Macros `#define DBG(...) printf(__VA_ARGS__)`
- **Effort:** Free — modernc.org/cc already handles this
- **Impact:** Debug macros, logging
- **Status:** Already works (preprocessor handles it)

#### 2.3 `restrict` Qualifier
- **Effort:** Free — accept and ignore
- **Impact:** Optimizer hint (could inform alias analysis later)
- **Status:** Parser accepts it, lowerer ignores qualifiers

#### 2.4 `_Bool` Full Semantics
- **Effort:** Small
- **Impact:** Correctness for `_Bool b = 42` → `b == 1`
- **Plan:** Insert `!!` normalization on assignment to `_Bool` variables

#### 2.5 Flexible Array Members `struct { int n; char data[]; }`
- **Effort:** Medium
- **Impact:** Buffers, network protocols, variable-size records
- **Plan:** Last field with `[]` gets zero size in struct layout.
  Allocation must be explicit (malloc or stack alloca with known size).

---

### Tier 3 — "C11 Nice-to-Haves"

#### 3.1 `_Generic` Type-Dispatch
- **Effort:** Medium
- **Impact:** Type-safe macros, `<tgmath.h>` style overloading
- **Plan:** At compile time, evaluate type of controlling expression,
  select matching association. Pure lowerer logic, no codegen change.

#### 3.2 `_Static_assert` (wire up error)
- **Effort:** Small — parser already handles
- **Impact:** Build-time invariant checking
- **Plan:** Evaluate constant expression, emit compile error if false

#### 3.3 Anonymous Structs/Unions
- **Effort:** Medium
- **Impact:** Cleaner nested types, common in embedded headers
- **Plan:** Flatten anonymous member fields into parent struct with
  correct offset calculation

#### 3.4 `_Noreturn`
- **Effort:** Trivial — attribute, no codegen change
- **Plan:** Accept and annotate function (optimizer can use for dead code elimination)

#### 3.5 `_Alignof` / `_Alignas`
- **Effort:** Small on Z80 (all alignment = 1)
- **Plan:** `_Alignof(T)` → always 1. `_Alignas` → accept and ignore.

---

### Tier 4 — "C23 Future-Proofing"

#### 4.1 `typeof` / `typeof_unqual`
- **Effort:** Medium — type inference in lowerer
- **Impact:** Macro hygiene, DRY type declarations

#### 4.2 `nullptr`
- **Effort:** Trivial — alias for `((void*)0)`
- **Impact:** Cleaner null checks

#### 4.3 Enum Underlying Type `enum Color : unsigned char { R, G, B }`
- **Effort:** Small — use specified type for enum storage
- **Impact:** ABI control, space optimization

#### 4.4 Digit Separators `1'000'000` or `0xFF'FF`
- **Effort:** Lexer change (strip separators before parsing)
- **Impact:** Readability for large constants

#### 4.5 `constexpr`
- **Effort:** Large — compile-time evaluation engine
- **Impact:** Constant folding, compile-time computation
- **Note:** Low priority; `#define` and `enum` cover most use cases on Z80

#### 4.6 `<stdbit.h>` Bit Utilities
- **Effort:** Medium — implement as intrinsics
- **Impact:** Useful for Z80 bit manipulation
- **Functions:** `stdc_leading_zeros`, `stdc_popcount`, `stdc_bit_width`, etc.

---

## Features NOT Worth Implementing (SDCC also skips these)

| Feature | Reason |
|---------|--------|
| VLAs (variable-length arrays) | Not supported by SDCC either. C11+ made optional. |
| `double` / `long double` | Z80 has no FPU. Map to `float` if ever needed. |
| `_Complex` / `_Imaginary` | No use case on 8-bit targets. |
| `_Atomic` / `<stdatomic.h>` | Z80 is single-core, interrupts use DI/EI. |
| `<threads.h>` | No OS threading on Z80. |
| Full `<locale.h>` | Overkill for embedded. |

---

## MinZ's Competitive Advantages Over SDCC

These are areas where MinZ is already ahead or uniquely positioned:

1. **Struct-return promotion** — 0T overhead for multi-value returns (vs ~60T SDCC out-params)
2. **MIR2 optimization pipeline** — more sophisticated than SDCC's peephole optimizer
3. **ObjC extensions** — no other Z80 compiler has classes/inheritance/method chaining
4. **Multi-frontend** — Nanz, C89, Lizp, Pascal, PL/M all share one backend
5. **Self-modifying code (TSMC)** — unique optimization paradigm
6. **Iterator chain fusion** — zero-cost functional patterns on Z80
7. **Dual-VM verification** — MIR2 VM + Z80 emulator cross-checking
8. **Assert-in-comments** — test system requires zero test framework

---

## Implementation Order (Recommended)

```
Phase 1 (Tier 1 — core gaps):
  [1] Designated initializers ........... 1-2 hours
  [2] Static local variables ............ 1 hour
  [3] goto + labels ..................... 2-3 hours
  [4] Compound literals ................. 2-3 hours
  [5] Function pointers ................. 4-6 hours (incl. indirect CALL)
  [6] Union types ....................... 3-4 hours

Phase 2 (Tier 2 — C99 conformance):
  [7] inline (accept/ignore) ........... 30 min
  [8] _Bool normalization .............. 1 hour
  [9] Flexible array members ........... 2 hours

Phase 3 (Tier 3 — C11):
  [10] _Static_assert wiring ........... 30 min
  [11] _Generic ........................ 3-4 hours
  [12] Anonymous structs/unions ......... 3-4 hours

Phase 4 (Tier 4 — C23):
  [13] nullptr ......................... 30 min
  [14] typeof .......................... 3-4 hours
  [15] Enum underlying type ............ 1-2 hours
```

**Total estimated: ~30 hours for near-full C99 + partial C11/C23 compliance.**

After Phase 1, MinZ's C frontend would be competitive with SDCC for most real embedded
Z80 projects, with the bonus of superior optimization and multi-language support.

---

## Metrics After Each Phase

| Phase | C89 | C99 | C11 | C23 | Real Code Compilable |
|-------|-----|-----|-----|-----|---------------------|
| Current | ~90% | ~40% | ~5% | 0% | ~40% |
| After Phase 1 | ~99% | ~70% | ~5% | 0% | ~80% |
| After Phase 2 | ~99% | ~90% | ~5% | 0% | ~90% |
| After Phase 3 | ~99% | ~90% | ~60% | 0% | ~92% |
| After Phase 4 | ~99% | ~90% | ~60% | ~30% | ~95% |

---

*Sources: SDCC 4.5.x User Manual, SDCC Standard Compliance Wiki, ISO C99/C11/C23 specs.*
