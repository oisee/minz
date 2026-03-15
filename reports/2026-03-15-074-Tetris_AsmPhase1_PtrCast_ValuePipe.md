# Report #074 — Tetris + Asm Phase 1 + ptr() Cast + Value Pipe

**Date:** 2026-03-15
**Status:** All features complete, all tests pass (26/26 packages)

---

## Summary

Four major deliverables in one session:

1. **ZX Spectrum Tetris** — 853 lines of Nanz, compiles to 2176 lines Z80 assembly (48 functions)
2. **Asm Phase 1** — `(ret REG)`, `(clob REG,...|auto|all)`, register-style `(in REG)`
3. **`ptr()` cast** — language-level peek/poke without inline asm
4. **`|>` value pipe operator** — F#/Elixir-style function chaining with constant folding

---

## 1. ZX Spectrum Tetris

Full playable Tetris for ZX Spectrum 48K, written entirely in Nanz.

### Controls
| Key | Action |
|-----|--------|
| O | Move left |
| P | Move right |
| Q | Rotate (with wall kicks) |
| A | Soft drop |
| SPACE | Hard drop |
| H | Hold piece |

### Features
- 7 standard tetrominoes (I, O, T, S, Z, L, J) with 4 rotations each
- SRS-lite wall kicks — 7 offset tests per rotation + I-piece ±2
- Hold piece with swap (once per lock cycle)
- Next piece preview
- Ghost piece (drop shadow at landing position)
- T-spin detection (3+ diagonal corners) with bonus scoring
- Speed progression every 8 lines
- Game over detection with flashing border

### Architecture
- **Attribute-based rendering**: pre-fill 8×8 cells with `0xFF` pixels, change only attribute bytes (fast — no pixel redraw per frame)
- **Zero-cost ZX primitives**: `zx_poke` = `LD (HL), A`, `zx_peek` = `LD A, (HL)`, `zx_key_row` = `IN A, (0xFE)` — 2 instructions each via `@z80_*` annotations + implicit asm return
- **Function-based piece tables**: `piece_dx(t,r,c)` and `piece_dy(t,r,c)` — no lookup arrays needed
- **LFSR PRNG**: `rng_state ^ 0xB400` for pseudo-random piece selection

### Compilation
```bash
cd minzc && ./mz ../examples/zx/tetris.nanz -o /tmp/tetris.a80
# Assemble and run:
./mza /tmp/tetris.a80 -o /tmp/tetris.bin
./mzx --run /tmp/tetris.bin@8000
```

### Z80 Output Quality
| Function | Z80 Instructions | Notes |
|----------|-----------------|-------|
| `zx_poke` | `LD (HL), A / RET` | Zero overhead via @z80_hl + @z80_a |
| `zx_peek` | `LD A, (HL) / RET` | Same pattern |
| `zx_key_row` | `IN A, (0xFE) / RET` | Direct port I/O |
| `color_attr` | 14 inst (if-chain) | 7 colors → constant returns |
| `board_get` | ~8 inst | u16 multiply + array index |
| `can_place` | ~30 inst | While loop, 4 cells check |
| `try_rotate` | ~100 inst | 7-8 wall kick tests |
| `ghost_y` | ~15 inst | Downward scan loop |

Known: 13 `LD A, ?` unresolved register artifacts (pre-existing allocator BUG-001).

---

## 2. Asm Phase 1 — ret, clob, register-style in

### New Syntax
```nanz
// Full form:
asm z80 (in HL) (ret A) (clob A, F) { LD A, (HL) }

// All clauses optional:
asm z80 (ret A) { ADD A, A }         // in = auto, clob = auto
asm z80 { HALT }                      // void, clob = auto

// Escape hatch for opaque code:
asm z80 (clob all) { CALL unknown }
```

### Design
| Clause | Takes | Default | Purpose |
|--------|-------|---------|---------|
| `(in REG,...)` | Physical regs | Auto-infer from @z80_* params | Liveness hint |
| `(ret REG)` | Physical reg | void | Return register |
| `(out REG)` | — | — | Alias for `ret` |
| `(clob REG,...\|auto\|all)` | Physical regs or keyword | `auto` | Clobber specification |

### Auto-clobber Analysis
When no `clob` clause is given (or `clob auto`), the compiler parses the asm text and computes which registers are written:
- Always includes F (flags) — almost every Z80 instruction touches F
- `CALL`/`RST` → `clob all` (unknown callee effects)
- Unknown mnemonics → `clob all` (conservative)
- Known instructions (LD, ADD, INC, etc.) → extract destination register

### Backward Compatibility
- Old code without `clob` gets `auto` instead of `all` — strictly better (fewer spills)
- Old `(in varname)` still works via fallback path

### Generated Code (ex31)
```nanz
fun port_read(@z80_a port_hi: u8) -> u8 {
    asm z80 (ret A) { IN A, (0xFE) }
}
fun double(@z80_a x: u8) -> u8 {
    asm z80 (ret A) (clob A, F) { ADD A, A }
}
```
```z80
port_read:
    IN A, (0xFE)
    RET
double:
    ADD A, A
    RET
main:
    LD A, 223
    CALL port_read
    JP double           ; tail call optimization!
```

---

## 3. ptr() Cast — Language-Level Memory Access

### Syntax
```nanz
ptr(addr)^           // read byte at address (peek)
ptr(addr)^ = val     // write byte to address (poke)
```

### How It Works
- `ptr(expr)` → `CastExpr{X: expr, Ty: TyPtr}` — u16 to pointer cast (no-op at MIR2 level, same width)
- `ptr(addr)^` → `LoadExpr{Ptr: cast}` → `LD A, (HL)` via standard deref
- `ptr(addr)^ = val` → `StoreStmt{Ptr: cast, Val: val}` → `LD (HL), val`
- Lookahead: `ptr` followed by `(` → cast; otherwise → variable name (no conflict)

### Generated Code (ex30)
```nanz
fun peek(addr: u16) -> u8 { return ptr(addr)^ }
fun poke(addr: u16, val: u8) -> void { ptr(addr)^ = val }
```
```z80
peek:
    LD A, (HL)      ; 2 instructions — identical to asm version!
    RET
poke:
    LD (HL), C      ; 2 instructions
    RET
```

This eliminates the need for `asm z80 { LD A, (HL) }` wrappers for memory-mapped I/O. Pure language, zero overhead.

---

## 4. `|>` Value Pipe Operator

### Syntax
```nanz
expr |> f            // → f(expr)
expr |> f(a, b)      // → f(expr, a, b)    (first-arg insertion)
```

### Generated Code (ex32)
```nanz
fun piped() -> u8 {
    return 5 |> double |> inc    // double(5)=10, inc(10)=11
}
```
```z80
piped:
    LD A, 11        ; constant-folded at compile time!
    RET
```

The entire chain `5 |> double |> inc` is evaluated at compile time → single `LD A, 11`.

### Lambda Type Inference (ex33)
```nanz
fun sum_doubled() -> u8 {
    return range(0..5).map(|x| x + x).fold(0, add_acc)
}
```
Lambda `|x| x + x` has no type annotation — the compiler infers `u8` from the chain context. Compiles to fused DJNZ loop.

---

## Showcase Examples

| # | File | Feature | Z80 Output | Quality |
|---|------|---------|-----------|---------|
| 30 | `ex30_ptr_cast` | ptr() peek/poke | `LD A,(HL)/RET`, `LD (HL),C/RET` | **Optimal** |
| 31 | `ex31_asm_ret_clob` | (ret A) + (clob A,F) | `IN A,(0xFE)/RET`, `ADD A,A/RET` | **Optimal** |
| 32 | `ex32_value_pipe` | `\|>` pipe operator | `LD A,11/RET` (constant folded!) | **Optimal** |
| 33 | `ex33_lambda_inference` | Untyped lambda in chain | DJNZ fused loop | **Good** |
| 34 | `ex34_tetris` | Full Tetris (853 LOC) | 2176 lines, 48 functions | **Complete** |

---

## Test Results

```
ok  github.com/minz/minzc/pkg/nanz    0.218s
ok  github.com/minz/minzc/pkg/lanz    0.329s
ok  github.com/minz/minzc/pkg/hir     0.503s
```

All 26/26 packages pass. 9 new tests added:
- `TestAsmRetReg`, `TestAsmOutAlias`, `TestAsmClobExplicit`, `TestAsmClobAll`, `TestAsmClobAutoDefault`
- `TestAsmBlock_InRegister`
- `TestPtrCast`, `TestPtrCastAsVariableName`
- `TestValuePipe`, `TestValuePipeWithArgs`, `TestLambdaTypeInference` (from previous session)

---

## Metrics Update

| Metric | Before | After |
|--------|--------|-------|
| Nanz examples | 131/173 | 136/173+ |
| Showcase examples | 29 | 34 |
| Asm clauses | `(in var)` only | `(in REG)`, `(ret REG)`, `(out REG)`, `(clob ...)` |
| ptr() cast | N/A | `ptr(addr)^` read+write |
| Value pipe | N/A | `\|>` with constant folding |
| Tetris LOC | — | 853 Nanz → 2176 Z80 asm |

---

## How Tetris Works

### Game Loop (60 Hz)
```
main() → init board → init screen → fill preview areas → spawn first piece
  └─ while game_over == 0:
       1. HALT (wait for VBlank — 50 Hz on PAL Spectrum)
       2. read_input() → 6-bit bitmask from keyboard matrix
       3. Edge-detect: new_keys = keys & (keys ^ prev_keys)
       4. Handle input:
          - Left/Right: can_place() check, move cur_x ±1
          - Rotate: try_rotate() with SRS wall kicks (7 tests)
          - Soft drop: set drop_timer = drop_speed (instant gravity tick)
          - Hard drop: loop can_place downward, +2 score/row, lock immediately
          - Hold: swap cur_type ↔ hold_type (once per piece)
       5. Gravity: drop_timer++, if >= drop_speed → move down or lock
       6. render():
          a. Draw board (200 cells → attribute bytes at 0x5800+)
          b. Ghost piece (dim color at ghost_y() landing position)
          c. Current piece (bright color)
          d. Next/hold piece previews (mini 4×4 at screen edges)
       7. Loop
```

### Rendering Strategy
ZX Spectrum stores screen in two areas:
- **Pixel data** (0x4000-0x57FF): 256×192 monochrome pixels
- **Attribute data** (0x5800-0x5AFF): 32×24 color cells (8×8 each)

Tetris pre-fills the playfield pixels with `0xFF` (all white) during `init_screen()`. Then each frame only writes **attribute bytes** — color changes are instant, no pixel redraw needed. This is extremely efficient on Z80: one `LD (HL), attr` per cell per frame.

### Piece Representation
Instead of lookup tables (arrays), pieces are encoded as **function lookups**:
```nanz
fun piece_dx(t: u8, r: u8, c: u8) -> u8 { ... }
fun piece_dy(t: u8, r: u8, c: u8) -> u8 { ... }
```
7 types × 4 rotations × 4 cells = 112 positions, encoded as nested if-chains. This avoids array indexing overhead and lets the compiler constant-fold when arguments are known.

### T-Spin Detection
When a T-piece is rotated and locked:
1. Check 4 diagonal corners around the T center
2. If 3+ corners are filled (by board cells or walls) → T-spin
3. T-spin line clears score 2× normal (800/1200/1600 vs 100/300/500)

### Wall Kicks (SRS-lite)
When rotation fails (collision), try 7 offset positions in order:
1. Basic rotation (no offset)
2. Right (+1, 0)
3. Left (-1, 0)
4. Up (0, -1)
5. Right+Up (+1, -1)
6. Left+Up (-1, -1)
7. I-piece only: ±2 horizontal

---

*Report #074 — MinZ Compiler v0.21.x*
