# ADR-0026: C stdlib, @print Meta-Function, and Variadics

## Status
Proposed (2026-03-15)

## Context

A C89 frontend (ADR-0024) needs a standard library to be useful. Three distinct concerns are often conflated under "stdlib":

```
Layer 1: Compiler builtins    — part of the compiler, not a library
Layer 2: crt0 (startup code)  — entry point, platform-specific
Layer 3: libc                 — functions linked to the program
```

Additionally, C's `printf` relies on variadic arguments (`...`), which are fundamentally at odds with Z80 register-based optimization. This ADR proposes `@print` as a compile-time meta-function that eliminates runtime format parsing entirely.

## Decision

### Layer 1: Compiler Builtins

Part of the compiler, not libc. ~50 LOC in codegen.

```c
sizeof(T)            // → SizeofExpr in HIR, already exists
_Static_assert       // → hir.Assert, already exists
__builtin_memcpy     // → emits LDIR in codegen
__builtin_memset     // → emits LDIR with fixed byte
```

### Layer 2: Platform-Specific crt0

One `.lizp` file per platform:

```asm
; CP/M crt0:
ORG 0x0100          ; TPA start
LD SP, 0xF000       ; stack
CALL _main          ; call main()
LD C, 0             ; BDOS warm boot
JP 0x0005

; ZX Spectrum crt0:
ORG 0x8000
DI
LD SP, 0xFFFF
CALL _main
JP 0x0000           ; restart
```

`argc`/`argv` for CP/M: `argv[0]` from FCB (0x005C), `argv[1..]` from DMA buffer (0x0080).

### Layer 3: Minimal libc

#### Absolute minimum (needed for any C code)

| Function | CP/M | ZX Spectrum | Z80 size |
|---|---|---|---|
| `putchar(c)` | BDOS fn 2 | RST 0x10 | ~10 bytes |
| `getchar()` | BDOS fn 1 | ROM 0x028E | ~10 bytes |
| `puts(s)` | BDOS fn 9 | via putchar | ~15 bytes |
| `exit(n)` | BDOS fn 0 | JP 0x0000 | ~5 bytes |
| `memcpy(d,s,n)` | LDIR | LDIR | ~20 bytes |
| `memset(d,c,n)` | LDIR | LDIR | ~20 bytes |
| `strlen(s)` | CPIR | CPIR | ~15 bytes |

#### String functions

| Function | Implementation | Z80 size |
|---|---|---|
| `strcmp(a,b)` | byte-by-byte loop | ~25 bytes |
| `strcpy(d,s)` | LDIR to null | ~20 bytes |
| `strcat(d,s)` | strlen + strcpy | ~30 bytes |

#### Integer math (no float — ever)

| Function | Implementation |
|---|---|
| `abs(x)` | NEG if < 0 |
| `min(a,b)` | CMP + select |
| `max(a,b)` | CMP + select |
| `div(a,b)` | → tuple (q,r) via ADR-0025 |

**`malloc`/`free` — NOT IMPLEMENTED.** On Z80 with 64K RAM, the arena allocator from Nanz stdlib replaces heap completely and does it better. C code requiring malloc gets a stub that calls `@error`.

### @print: Compile-Time printf Replacement

#### Why not runtime printf

Runtime `printf` on Z80:
- Parses format string **at runtime** — ~50T per specifier
- Type dispatch **at runtime** — switch on each `%`
- ~500 bytes of code just for specifier support
- Cannot apply PFCCO — ABI fixed to stack

#### How @print works

`@print` is a compile-time meta-function. Format string is parsed **before code generation**. Argument types are known from HIR. Each specifier → optimal call.

```c
// C code:
printf("x=%d y=%d\n", x, y);

// @print sees at compile-time:
// fmt = "x=%d y=%d\n"  (static string)
// args = [x: i16, y: i16]  (types from HIR)

// Expansion →
print_cstr("x=")    // ROM literal, BDOS fn 9
print_i16(x)        // ~20 instructions
print_cstr(" y=")   // ROM literal
print_i16(y)        // ~20 instructions
print_char('\n')    // BDOS fn 2
```

No runtime parsing. No dispatch. Each string → ROM literal, each argument → typed call.

#### Three levels of aggressiveness

**Level 1: Type known, value runtime**
```c
printf("%d", score);  // score: u16
// → CALL print_u16   // selected compile-time by type
```

**Level 2: Value is compile-time constant**
```c
printf("Score: %d\n", 42);
// → string "Score: 42\n" directly in ROM
// → one CALL print_cstr
// → zero runtime computation
```

**Level 3: String only, no arguments**
```c
printf("Hello CP/M!\n");
// → LD DE, .hello_str
// → CALL bdos_puts    // one BDOS call
// → no intermediate functions
```

#### Format specifier mapping

```
%c   → print_char(u8)          // BDOS fn 2 or OUT
%s   → print_cstr(^u8)         // BDOS fn 9 or CPIR loop
%d   → print_i16(i16)          // ~20 instructions
%u   → print_u16(u16)          // ~15 instructions
%x   → print_hex_u16(u16)      // ~10 instructions (nibble LUT)
%02x → print_hex_padded(u16,2) // with padding
%ld  → print_i32(i32)          // if needed
%%   → print_char('%')
%f   → COMPILE ERROR            // float on Z80 — no
```

#### Dynamic format string — explicit fallback

```c
const char *fmt = get_format();
printf(fmt, x);  // format string unknown at compile-time!
```

Compiler emits **warning** and uses runtime nano-printf (~100 LOC):

```
warning: dynamic format string — falling back to runtime printf
         PFCCO optimization not applicable
```

#### @print converges with Pascal WriteLn

```pascal
WriteLn('x=', x, ' y=', y);  // Pascal
```
```c
printf("x=%d y=%d\n", x, y); // C via @print
```

Both compile-time → same `print_cstr` + `print_i16` calls → identical Z80 output. Different syntax, one implementation.

### Variadic Parameters: Four Clean Variants

#### How C implements variadics — and why it's bad for Z80

```c
void printf(const char *fmt, ...) {
    va_list args;
    va_start(args, fmt);          // takes address of last named param
    char c = va_arg(args, char);  // reads argument from stack
    va_end(args);
}
```

On Z80: all variadic arguments **always on stack**, no type information, no PFCCO. `va_list` = `char *` pointer to stack. ~200T overhead just for setup.

#### Variant 1: Compile-time expansion via @meta (BEST)

```nanz
@meta fun print(fmt: @comptime ^u8, args...) {
    @for spec in @parse_format(fmt) {
        match spec {
            Literal(s)    => @emit call_print_cstr(s)
            Spec('%d', i) => @emit call_print_i16(args[i])
            Spec('%x', i) => @emit call_print_hex(args[i])
            Spec('%c', i) => @emit call_print_char(args[i])
        }
    }
}
```

`args...` is **not stack**. It's a compile-time list of typed expressions. Meta-function receives them as `@comptime []Expr`, sees types from HIR, emits optimal code for each.

#### Variant 2: Typed slice (homogeneous variadics)

When all arguments are the same type:

```nanz
fun sum(values: []u16) -> u16 {
    return values.fold(0, +)
}
sum([1, 2, 3, 4, 5])  // array literal → ROM
```

On Z80: `(HL=ptr, B=len)` → DJNZ loop. Known length compile-time → LDIR if appropriate.

#### Variant 3: Tagged union array (heterogeneous runtime)

When types differ and runtime dispatch is needed:

```nanz
enum Arg { U8(u8), U16(u16), Str(^u8) }

fun log(level: u8, args: []Arg) { ... }
log(1, [Arg.U16(score), Arg.Str("pts")])
```

Each `Arg` = 3 bytes (1 tag + 2 data). Switch on tag in loop. Explicit, predictable, ~15T per argument. Fine for debug output.

#### Variant 4: @cabi va_list (C interop only)

```nanz
@cabi  // stack ABI, no PFCCO
fun printf(fmt: ^u8, ...) -> i16
```

Only for `@extern` C functions. PFCCO not applied, stack ABI. Needed to call existing C code through FFI — not for new MinZ code.

#### Decision hierarchy

```
Compile-time known count and types:
  → @meta with args...               ← BEST: 0T overhead, type-safe

Homogeneous runtime (all args same type):
  → []T slice                        ← GOOD: DJNZ/LDIR, known length

Heterogeneous runtime (different types):
  → []Arg tagged union               ← OK: explicit, ~15T/argument

C interop (calling existing C code):
  → @cabi va_list                    ← FFI ONLY, no PFCCO

Forbidden:
  → implicit runtime variadics       ← no implicit stack pushing
```

**Key principle:** if @meta cannot expand compile-time, compiler requires explicit `[]Arg`. No stack magic that can't be optimized.

### stdlib File Layout

```
stdlib/
  c/
    crt0/
      cpm.lizp          // CP/M 0x0100, argc/argv from FCB
      zx48.lizp         // ZX Spectrum 48K 0x8000
      zx128.lizp        // ZX Spectrum 128K + bank setup
    libc/
      string.lizp       // memcpy, memset, strlen, strcmp, strcpy
      stdio.lizp        // putchar, getchar, puts, @print
      stdlib.lizp       // exit, abs, min, max, div (→ tuple)
      ctype.lizp        // isdigit, isalpha, toupper, tolower
      printf_nano.lizp  // runtime fallback for dynamic fmt
    compat/
      sdcc_compat.lizp  // __sfr, __at, __interrupt → @at, @interrupt
      z88dk_compat.lizp // z88dk specifics
  pascal/
    system.lizp         // already done! WriteLn, Write, ReadKey...
```

All in Lizp — same pattern as Pascal RTL. Cross-platform via different crt0. Cross-language import already works.

### SDCC/z88dk Compatibility

| SDCC/z88dk | MinZ | Status |
|---|---|---|
| `__sfr __at(0xFE) BORDER` | `@at(0xFE) var BORDER: u8` | maps to existing |
| `__at(0x8000) char buf[]` | `@at(0x8000) var buf: [N]u8` | maps to existing |
| `__interrupt void isr(void)` | `@interrupt fun isr()` | maps to existing |
| `__naked void f(void)` | `@naked fun f()` | maps to existing |
| `#pragma output STACKPTR` | crt0 parameter | maps to existing |

## Consequences

### Positive
- printf on Z80 with 0T runtime dispatch overhead
- Type safety at compile time (format string checked against argument types)
- Convergence with Pascal WriteLn — one implementation, two syntaxes
- Clean variadic hierarchy — @meta first, explicit fallbacks only when needed
- Minimal libc footprint — only what's used gets linked

### Negative
- `%f` not supported (no float on Z80)
- Dynamic format strings get degraded path with warning
- malloc not provided — requires code changes for heap-dependent C code

### Neutral
- nano-printf fallback exists for edge cases
- All stdlib in Lizp — consistent with Pascal RTL approach

## Effort Estimate

| Component | Estimate | Depends on |
|---|---|---|
| crt0 (CP/M + ZX) | 1 day | nearly exists |
| string.h (mem* + str*) | 1 day | pure Z80 |
| stdio.h (@print + putchar) | 2 days | platform-specific |
| stdlib.h (exit/abs/div) | 0.5 day | trivial |
| sdcc_compat annotations | 1 day | parser extensions |
| printf_nano (runtime fallback) | 1 day | dynamic fmt only |
| @meta variadics in compiler | 2-3 days | after CTFE |
| **Total** | **~9-10 days** | after C frontend |

Prerequisite for @meta variadics: CTFE (@comptime) must be ready.

## References

- ADR-0024: C89 Frontend Strategy
- ADR-0025: Struct-Return Promotion to Tuple
- ADR-0021: String Types
- ADR-0023: Compile-Time Metafunctions via Lanz
- [SDCC manual — calling conventions](https://sdcc.sourceforge.net/doc/sdccman.pdf)
- [z88dk C library reference](https://github.com/z88dk/z88dk/wiki)
