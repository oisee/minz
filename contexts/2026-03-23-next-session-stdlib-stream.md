# Next Session Briefing: Stream Abstraction + Self-Hosting Progress

## Goal

Build the Stream I/O abstraction and push toward a self-hosted lexer.

## Priority 1: Stream — Unified Write Interface

Three backends, one API:

```nanz
struct Stream {
    write: ^u8    // function pointer (or SMC-patched target)
    buf: ^u8      // output buffer (for MemStream)
    pos: u16      // current write position
    cap: u16      // buffer capacity
}

// Stdout: writes to I/O port
fun stdout_write(s: ^Stream, byte: u8) { ... }

// Buffer: writes to arena-allocated memory
fun buf_write(s: ^Stream, byte: u8) {
    s.buf[s.pos] = byte
    s.pos = s.pos + 1
}

// File: writes via BDOS (CP/M) or MOS (Agon)
fun file_write(s: ^Stream, byte: u8) { ... }
```

**Key decision:** Function pointer dispatch (~17T per call on Z80) vs SMC-patched dispatch (faster but Z80-specific). Maybe both — `@smc` annotation on Stream makes it fast.

**Bonus:** `@sprintf` could desugar to `@print` but targeting a buffer Stream instead of stdout. Compile-time LDIR chain for static segments, runtime itoa for variables.

## Priority 2: Fix LIR Nested CondExpr

`match` expressions generate correct code on production MIR2 backend but broken straight-line code on LIR. Root cause: LIR's WFC collapse doesn't handle multi-branch CondExpr chains.

**Investigation path:**
- `minzc/pkg/lir/` — find where CondExpr is lowered
- The LIR bridge probably flattens the chain instead of creating branch blocks
- Compare with how `if-then-else` expression works (that one works on LIR)

## Priority 3: Fix HIR Lowerer Variable Reuse

`x op (x + N)` panics. The lowerer reads `x` once, uses it for the inner `x + N`, then tries to read `x` again for the outer op — but the register is gone.

**Investigation path:**
- `minzc/pkg/hir/lower.go:1569` — the panic site
- `lowerBinExpr` or `lowerExpr` — need to save L operand before lowering R when they share vars
- Fix: evaluate L into a temp reg first, then evaluate R, then combine

## Priority 4: Assert System + Import Modules

MIR2 VM asserts work with cross-function calls (proven). But haven't tested with `import` — functions from imported modules should be in the same `*mir2.Module` after lowering, so it should Just Work. Needs verification.

**Test plan:**
```nanz
// stdlib/core/math.nanz
fun double(x: u8) -> u8 { return x + x }

// test.nanz
import stdlib.core.math
assert double(21) == 42
```

## Priority 5: TinyNanz Lexer (Self-Hosting Phase 1)

Write a minimal lexer in Nanz using StrRef + HashMap:

```nanz
enum TokenKind { Ident, Int, LParen, RParen, LBrace, RBrace,
                 Comma, Colon, Arrow, FatArrow, Eq, Plus, Minus, ... }

struct Token {
    kind: u8        // TokenKind
    str_idx: u16    // index into InternTable (for Ident/Int)
    line: u16       // source line number
}

// Lexer state: global source buffer + position
global src: [u8; 4096]
global src_len: u16
global pos: u16
global line: u16

fun next_token() -> Token { ... }
```

This is the first concrete step toward self-hosting. If the lexer works on MZV, the parser follows naturally.

## Key Constraints (from this session)

1. Use `xor` keyword, not `^` (pointer deref)
2. Decompose `x op (x + N)` into temp vars
3. Use global arrays for VM-compatible storage
4. Use `--lir=false` for match expressions
5. Use `via mir2` for stdlib algorithm tests

## Files to Start With

```
stdlib/core/stream.nanz        — NEW: Stream abstraction
stdlib/core/strref.nanz        — EXISTS: extend with more operations
stdlib/core/hashmap.nanz       — EXISTS: proven working
minzc/pkg/lir/                 — LIR CondExpr investigation
minzc/pkg/hir/lower.go:1569   — variable reuse panic
minzc/pkg/pipeline/pipeline.go — assert system (for import testing)
```
