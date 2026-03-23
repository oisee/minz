# Session Extract: Nanz ADT + Self-Hosting Foundations (2026-03-23 evening)

## Wisdom Earned

### 1. `^` is NOT XOR in Nanz
The most insidious bug of the session. `^` is pointer dereference. XOR is the `xor` keyword.
```nanz
var a: u8 = 1
var b: u8 = 38
return (a ^ b)    // WRONG: dereferences pointer b, chaos
return (a xor b)  // RIGHT: bitwise XOR = 39
```
**Rule:** When porting code from C/Rust/Frill to Nanz, mechanically replace `^` with `xor` in all bitwise contexts.

### 2. HIR Lowerer Can't Handle `x op (x + N)`
Using a variable twice in one expression panics: `"undefined variable"`.
```nanz
var h: u16 = key ^ (key + 37)   // PANIC: lowerer loses `key`
var shifted: u16 = key + 37     // OK: temp var
var h: u16 = key xor shifted    // OK
```
**Rule:** Always decompose into temps when a variable appears twice in an expression.

### 3. MIR2 VM Assert System Is Better Than We Thought
Cross-function calls work. Globals work. Structs work. The "unknown function" errors were caused by using Z80-absolute addresses (0xC000) that are outside VM heap — not by missing functions.
```nanz
// BAD: Arena at Z80 address — VM heap is only ~200 bytes
Arena.init(&arena, 0xC000, 4096)

// GOOD: Use global arrays — VM allocates them in its heap
global buf: [u8; 64]
```
**Rule:** For MIR2 VM asserts, use global arrays as backing storage. Never use absolute Z80 addresses.

### 4. `via mir2` vs `via z80` — Choose Wisely
- `via mir2` — fast, works with globals and cross-function calls, BUT no Z80 codegen bugs caught
- `via z80` — catches codegen bugs, BUT some patterns fail (computed indices, `LD A, (A)` bug)
- No `via` annotation — runs BOTH (default), fails if either fails
**Rule:** Use `via mir2` for stdlib/algorithm tests. Use `via z80` (or both) for codegen-critical tests.

### 5. ADT Encoding: u16 = tag<<8 | payload
Brilliant on Z80: tag in H, payload in L. `__tag(x) = LD A, H` (4T). `__payload(x) = LD A, L` (4T).
But: payload limited to u8. For larger payloads, need structs.

### 6. Match on LIR Backend Is Broken
LIR (ISLE+WFC) generates straight-line code for nested CondExpr — no branches. Production MIR2 backend (`--lir=false`) generates correct `CP`+`JR` chains. This is a known LIR gap.
**Rule:** Always test match expressions with `--lir=false` until LIR is fixed.

### 7. Self-Hosting Is Feasible — On The Right Hardware
- ZX Spectrum 48K: too tight (compiler ~80-120KB)
- **Agon Light 2 (512KB, 18MHz eZ80):** entire compiler fits comfortably
- **ZX Spectrum 128K:** staged compilation across 8 memory banks
- **MZV (MIR2 VM):** no constraints, easiest path

### 8. Frill Was The Proving Ground
Every feature we added to Nanz today (ADT, match, payload binding) was already implemented and tested in Frill. Porting was mostly parser work — HIR, MIR2, and codegen already support everything because Frill uses them.

## What Was Built

| Component | Lines | Tests |
|-----------|-------|-------|
| ADT enum parser (payload, constructors) | ~200 | 27 Go tests |
| Match expression parser (exhaustive, wildcard, binding) | ~200 | integrated |
| `tokFatArrow` (`=>`) lexer token | 5 | — |
| 4 Nanz showcase examples | ~200 | 37 compile-time asserts |
| stdlib/core/strref.nanz (intern table) | 170 | 6 asserts |
| stdlib/core/hashmap.nanz (u16→u16 map) | 130 | 8 asserts |
| Nanz Book v6.0 (Ch.16 rewrite + Ch.22 new) | ~350 | — |
| README update (Frill + ADT/match) | ~80 | — |
| Frill Language Guide PDF/HTML/EPUB regen | — | — |

7 commits, all pushed to master.

## Files Changed

```
minzc/pkg/nanz/parse.go              — ADT enum, match expr, tokFatArrow, grammar comment
minzc/pkg/nanz/nanz_test.go          — 10 new ADT/match tests
minzc/pkg/nanz/enum_e2e_test.go      — 7 new E2E tests with Z80 emulator
stdlib/core/strref.nanz              — NEW: StrRef + InternTable
stdlib/core/hashmap.nanz             — NEW: HashMap (open addressing)
examples/nanz/10_adt_option.nanz     — NEW: Option pattern showcase
examples/nanz/11_match_expression.nanz — NEW: match expression showcase
examples/nanz/12_state_machine.nanz  — NEW: state machine showcase
examples/nanz/13_result_error.nanz   — NEW: Result/Error pattern
examples/nanz/20_strref_test.nanz    — NEW: strref E2E tests
examples/nanz/21_hashmap_test.nanz   — NEW: hashmap E2E tests
docs/Nanz_Language_Book_v5.md        — v6.0: Ch.16 ADT/match, Ch.22 self-hosting
docs/book/Frill_Language_Guide.*     — regenerated PDF/HTML/EPUB
README.md                            — Frill frontend, ADT/match, v0.23.0
```
