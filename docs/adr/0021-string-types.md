# ADR-0021: String Types for Z80

## Status
Accepted

## Context
Nanz has no string type.  This blocks:
- Any I/O beyond `@print` meta-function
- Pascal frontend (length-prefixed strings)
- C89 frontend (null-terminated strings)
- CP/M BDOS interop (`$`-terminated or null-terminated)
- ZX Spectrum ROM calls (length-prefixed)

Different Z80 targets expect different string formats.  A single string type
cannot serve all targets without runtime conversion overhead.

## Decision

### Three string types, one default

| Type | Format | Max length | `len` cost | Use case |
|------|--------|-----------|-----------|----------|
| `string` | u8-length prefix | 255 | O(1) — 7T | Default, Pascal, ZX ROM |
| `cstr` | null-terminated | unlimited | O(n) — scan | C interop, CP/M BDOS |
| `rstr` | bare bytes in ROM | compile-time | 0T (known) | Constants, titles, menus |

### `string` is the default (u8-length-prefixed)

```nanz
let greeting: string = "Hello"
// Memory: [05][H][e][l][l][o]  — 6 bytes
// greeting.len → LD A, (HL) = 7T, O(1)
```

### `cstr` for C/CP/M interop

```nanz
let msg: cstr = c"Hello"
// Memory: [H][e][l][l][o][00]  — 6 bytes
```

### `rstr` for ROM constants

```nanz
let title: rstr = r"TETRIS"
// Memory: [T][E][T][R][I][S]  — 6 bytes, no prefix, length known at compile time
```

### Conversion functions (in stdlib)

```nanz
fun to_cstr(s: string) -> cstr    // copy + append null
fun to_string(s: cstr) -> string  // scan + prepend length
fun strlen(s: string) -> u8       // O(1): return s[0]
fun cstrlen(s: cstr) -> u8        // O(n): scan for null
```

### No u16-length-prefix type

Z80 has 64K total RAM.  Strings > 255 bytes are extremely rare.  If needed
later, `lstr` can be added without breaking existing code.

## Consequences

### Positive
- O(1) length for default string type — critical for Z80 performance
- Pascal strings use the same format — frontend compatibility
- Null-terminated available for C interop without being the default
- ROM strings have zero runtime overhead
- Safe: `string` can contain null bytes (binary data)

### Negative
- 255-byte limit on default strings (acceptable for Z80)
- Three types to learn (mitigated: `string` is the only one most code uses)
- Conversion between types requires copying

### Neutral
- String interpolation (`"x=#{x}"`) deferred to CTFE phase
- String operations (strcmp, strcpy, etc.) go in stdlib, not compiler

## Alternatives Considered

### Single null-terminated type (C-style)
Rejected: O(n) length is expensive on Z80 (no `strlen` instruction).  Every
`print` call would need to scan.  Pascal frontend would need constant wrapping.

### Single u16-prefixed type
Rejected: 2 bytes overhead per string.  With 100 strings in a game, that's 100
extra bytes.  255-byte max is already generous for Z80.

### `SString` / `LString` / `String` hierarchy with inheritance
Rejected: over-engineered.  Three independent types with conversion functions
are simpler and have no runtime cost.  Inheritance adds vtable overhead.

## References
- [Report #070](../../reports/2026-03-14-070-Language_Frontends_Gaps_Roadmap.md) §3
- Turbo Pascal 3 string format: `[len:u8][chars...]`
- ZX Spectrum ROM print: length-prefixed
- CP/M BDOS function 9: `$`-terminated (we map to `cstr` + custom terminator)
