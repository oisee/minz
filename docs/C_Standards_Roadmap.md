# C Standards Roadmap: C89 → C99 → C11 → C17 → C23

MinZ C frontend progression toward modern C standards on Z80.

**Parser:** `modernc.org/cc/v4` — handles preprocessing, parsing, type checking.
**Lowerer:** `pkg/c89/lower.go` — walks typed AST, emits HIR.
**libc:** `pkg/c89/libc/` — minimal headers for Z80 freestanding environment.

---

## Summary

| Standard | Features Added | Supported | Skipped (Z80) | TODO |
|----------|---------------|-----------|---------------|------|
| **C99**  | ~15 major     | 12        | 2             | 1    |
| **C11**  | ~10 major     | 4         | 4             | 2    |
| **C17**  | 0 (bugfix)    | ✅ full   | —             | —    |
| **C23**  | ~15 major     | 2         | 3             | 10   |

---

## C99 (ISO/IEC 9899:1999)

### ✅ Supported

| Feature | Implementation | Notes |
|---------|---------------|-------|
| `//` single-line comments | `modernc.org/cc` parser | |
| `_Bool` type | `lower.go` type mapping + `boolNormalize()` | Any non-zero → 1 |
| `for (int i = 0; ...)` | `IterationStatementForDecl` case | |
| Mixed declarations and code | `lowerCompound()` processes interleaved decl/stmt | |
| Designated struct initializers `{.x = 1}` | `lowerStructInit()` | Out-of-order, mixed positional |
| Compound literals `(Type){...}` | `PostfixExpressionComplit` case | Struct + scalar |
| `inline` functions | Parsed, treated as regular functions | No actual inlining at C level |
| `restrict` keyword | Parsed, ignored | Z80: no aliasing optimizer |
| `<stdint.h>` | `libc/stdint.h` | uint8_t through uint32_t |
| `<stddef.h>` | `libc/stddef.h` | NULL, size_t, offsetof |
| `<stdbool.h>` | `libc/stdbool.h` | bool, true, false macros |
| Predefined `bool`/`true`/`false` | `z80Predefined` macros | C23-style, available without include |

### ❌ Skipped (Z80 limitations)

| Feature | Reason |
|---------|--------|
| Variable-length arrays (VLAs) | No runtime stack allocation on Z80. C11 made optional anyway. |
| `_Complex` / `_Imaginary` | No FPU on Z80. |

### ✅ Recently Added

| Feature | Implementation | Notes |
|---------|---------------|-------|
| Array designated init `[i] = val` | `lowerArrayInit()` in lower.go | Positional, designated, mixed, sparse — all work |

---

## C11 (ISO/IEC 9899:2011)

### ✅ Supported

| Feature | Implementation | Notes |
|---------|---------------|-------|
| `_Static_assert` | `modernc.org/cc` parser | Compile-time assertions |
| `_Generic` | `modernc.org/cc` parser | Type-generic expressions |
| `_Noreturn` | Parsed, ignored | No codegen effect |
| `<stdalign.h>` | `libc/stdalign.h` | Z80: always 1-byte aligned |
| `<stdnoreturn.h>` | `libc/stdnoreturn.h` | `noreturn` → `_Noreturn` |
| `<assert.h>` | `libc/assert.h` | assert() macro, NDEBUG support |

### ❌ Skipped (Z80 limitations)

| Feature | Reason |
|---------|--------|
| `<threads.h>` | Z80: single core, no OS threads |
| `<stdatomic.h>` | Z80: single core, all ops are atomic |
| `u8""`/`u""`/`U""` string prefixes | Z80: ASCII only, no Unicode |
| `_Alignas(N)` | Z80: byte-addressed, alignment always 1 |

### 📋 TODO

| Feature | Difficulty | Notes |
|---------|-----------|-------|
| Anonymous structs/unions | Medium | Flatten unnamed member fields into parent namespace. Affects `lowerStructDecl()` + `resolveFieldOffset()`. |
| `_Alignof` operator | Small | Return 1 for all types on Z80 (byte-addressed). |

---

## C17 (ISO/IEC 9899:2018)

**C17 is a bugfix release** — no new language features. Only clarifications and defect resolutions from C11.

✅ **Fully supported** by supporting C11.

`__STDC_VERSION__` is set to `201710L` in `z80Predefined`.

---

## C23 (ISO/IEC 9899:2024)

### ✅ Supported

| Feature | Implementation | Notes |
|---------|---------------|-------|
| `bool`/`true`/`false` as keywords | `z80Predefined` macros | Available without `#include <stdbool.h>` |
| `static_assert` (no message) | `assert.h` macro → `_Static_assert` | |

### ❌ Skipped (Z80 limitations)

| Feature | Reason |
|---------|--------|
| `_BitInt(N)` | Arbitrary-width integers — no Z80 support |
| `_Decimal32/64/128` | Decimal floating point — no FPU |
| `char8_t` | UTF-8 type — Z80 is ASCII |

### 📋 TODO

| Feature | Difficulty | Priority | Notes |
|---------|-----------|----------|-------|
| `nullptr` | Small | Nice | Map to `(void*)0`. Check if `cc.PrimaryExpressionNullptr` exists. |
| `typeof` / `typeof_unqual` | Medium | Nice | Depends on `modernc.org/cc` support. |
| `constexpr` | Medium | Nice | Compile-time constant evaluation. Extend `evalConstInit`. |
| Enum underlying type `enum E : uint8_t` | Small | Nice | Useful for Z80 — saves bytes vs default int (2 bytes). |
| `[[]]` attributes | Small | Nice | Depends on parser. `[[noreturn]]`, `[[maybe_unused]]`. |
| Digit separators `1'000` | Trivial | Nice | Depends on `modernc.org/cc` lexer. |
| `#embed` | Medium | **High** | Binary include — perfect for Z80 sprites, fonts, LUTs. |
| `<stdbit.h>` | Small | Nice | Bit manipulation: `stdc_popcount`, `stdc_leading_zeros`, etc. |
| `<ctype.h>` functions | ✅ Done | — | `libc/ctype.h` — inline implementations, no lookup table. |
| `auto` type inference | Medium | Nice | Depends on parser. `auto x = expr;` |

### `#embed` — The Z80 Killer Feature

```c
// C23: include binary data at compile time
const unsigned char font[] = {
    #embed "font.bin"    // 768 bytes of ZX Spectrum font data
};

const unsigned char sprite[] = {
    #embed "player.spr"  // sprite pixel data
};

const unsigned char sin_table[] = {
    #embed "sin256.bin"  // precomputed sine lookup table
};
```

This replaces hand-typed `DB` sequences in assembly. High priority for Z80 development.

---

## libc Headers Status

| Header | Status | Contents |
|--------|--------|----------|
| `<stdint.h>` | ✅ Done | uint8_t..uint32_t, INT8_MIN..UINT32_MAX |
| `<stddef.h>` | ✅ Done | NULL, size_t, ptrdiff_t, offsetof |
| `<limits.h>` | ✅ Done | CHAR_BIT, INT_MAX, LONG_MAX, etc. |
| `<stdbool.h>` | ✅ Done | bool, true, false, __bool_true_false_are_defined |
| `<stdarg.h>` | ✅ Done | va_list, va_start, va_arg (basic) |
| `<stdlib.h>` | ✅ Done | abs, atoi, malloc/free stubs |
| `<string.h>` | ✅ Done | memcpy, strlen, strcmp, etc. |
| `<math.h>` | ✅ Done | fabs, fmod stubs |
| `<assert.h>` | ✅ Done | assert() macro + NDEBUG |
| `<ctype.h>` | ✅ Done | Inline isdigit..toupper (no LUT) |
| `<stdalign.h>` | ✅ Done | alignas, alignof (always 1 on Z80) |
| `<stdnoreturn.h>` | ✅ Done | noreturn → _Noreturn |
| `<stdio.h>` | ❌ N/A | Use `@print` metafunction instead |
| `<errno.h>` | 📋 TODO | Error codes |
| `<signal.h>` | ❌ N/A | No OS signals on Z80 |
| `<locale.h>` | ❌ N/A | No locale support |
| `<time.h>` | ❌ N/A | No OS time on bare Z80 |
| `<stdbit.h>` | 📋 TODO | C23 bit manipulation |

---

## Predefined Macros

```c
#define __Z80__ 1              /* Target architecture */
#define __MINZ__ 1             /* MinZ compiler */
#define __STDC__ 1             /* ISO C conformant */
#define __STDC_VERSION__ 201710L  /* C17 conformance level */
#define __STDC_HOSTED__ 0      /* Freestanding (no OS) */
#define __SIZEOF_INT__ 2       /* 16-bit int */
#define __SIZEOF_POINTER__ 2   /* 16-bit pointers */
#define bool _Bool             /* C23 keyword */
#define true 1
#define false 0
```

---

## Implementation Priority

### Phase 1 — Done ✅
1. ~~stdbool.h~~ — `bool`/`true`/`false`
2. ~~assert.h~~ — `assert()` macro
3. ~~ctype.h~~ — character classification (inline, no LUT)
4. ~~stdalign.h~~ — C11 alignment (trivial on Z80)
5. ~~stdnoreturn.h~~ — C11 noreturn
6. ~~Predefined macros~~ — `__STDC_VERSION__`, `bool`/`true`/`false`

### Phase 2 — Next
7. Array designated initializers `[i] = val`
8. Anonymous structs/unions (C11)
9. `nullptr` (C23)

### Phase 3 — Future
10. `#embed` (C23) — binary includes for sprites/fonts/LUTs
11. `typeof` (C23)
12. `constexpr` (C23)
13. Enum underlying type (C23)
14. `<stdbit.h>` (C23)

---

## Test Examples

| File | Features Tested |
|------|----------------|
| `examples/c89/*.c` | 38 files — C89/C99 core features, 350 asserts |
| `examples/c/c99_stdbool.c` | `<stdbool.h>`, `_Bool`, logical ops |
| `examples/c/c99_ctype.c` | `<ctype.h>`, isdigit..toupper |
| `examples/c/c11_features.c` | Designated init, compound literals, mixed decls, inline, for-init |
| `examples/c/c23_preview.c` | `bool` without include, ternary chains |
| `examples/c/array_desig.c` | Array designated init: positional, designated, mixed, sparse |

---

## Z80-Specific Considerations

- **`int` = 16 bits** — C integer promotion rules still apply, but `uint8_t` arithmetic stays 8-bit where safe (selective narrowing in lowerer)
- **No FPU** — `float`/`double` map to 4-byte stubs, no real arithmetic
- **No OS** — freestanding environment (`__STDC_HOSTED__ 0`), no stdio/signal/time
- **1-byte alignment** — `_Alignof(T)` = 1 for all types
- **2-byte pointers** — 16-bit address space (64 KB)
- **`long` = 4 bytes** — but 32-bit arithmetic is expensive (multi-instruction sequences)

---

*Last updated: 2026-03-26*
