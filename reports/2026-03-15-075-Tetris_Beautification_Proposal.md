# Report #075 — Tetris Beautification Proposal

**Date:** 2026-03-15
**Status:** Proposal
**File:** `examples/zx/tetris.nanz` (852 lines) → `examples/zx/tetris_v2.nanz`

---

## Summary

Analysis of the existing Tetris implementation reveals significant opportunities to
make the code more idiomatic Nanz while preserving identical runtime behavior.
The original code works and compiles to 2190 lines of Z80 asm — this proposal
focuses purely on source-level readability and language idiom usage.

---

## 1. While → For Range (18 of 25 loops)

The original Tetris uses `while` for all loops, including trivial counted loops.
Nanz has `for i in 0..n` range syntax which is cleaner and eliminates manual
counter variables.

### Category A: Direct `for i in 0..n` (13 loops)

```
Line 459:  while c < 4       → for c in 0..4
Line 475:  while x < 10      → for x in 0..10
Line 483:  while sx < 10     → for sx in 0..10
Line 490:  while sx2 < 10    → for sx2 in 0..10
Line 555:  while line < 8    → for line in 0..8
Line 616:  while py < 4      → for py in 0..4
Line 618:  while px < 4      → for px in 0..4
Line 628:  while c < 4       → for c in 0..4
Line 640:  while y < 20      → for y in 0..20
Line 642:  while x < 10      → for x in 0..10
Line 657:  while c < 4       → for c in 0..4
Line 673:  while c < 4       → for c in 0..4
Line 841:  while flash < 20  → for flash in 0..20
```

### Category B: Offset range `for i in a..b` (5 loops)

```
Line 566:  addr = 0x4000; while addr < 0x5800  → for addr in 0x4000..0x5800
Line 571:  addr = 0x5800; while addr < 0x5B00  → for addr in 0x5800..0x5B00
Line 578:  cy = 2; while cy < 22               → for cy in 2..22
Line 589:  cy = 2; while cy < 22               → for cy in 2..22
Line 603:  bx = 10; while bx <= 21             → for bx in 10..22
```

### Category C: Not transformable (7 loops)

| Line | Reason |
|------|--------|
| 427  | `ghost_y()` — iterate until collision, unknown N |
| 445  | `can_place()` — early `return false` inside body |
| 472  | `clear_lines()` — reverse iteration with row deletion |
| 481  | nested shift-down with decrement |
| 504  | modulo via subtraction (replaced with `& 7`) |
| 776  | game loop — infinite with exit condition |
| 812  | hard drop — iterate until collision |

**Note:** `can_place()` (line 445) uses `while c < 4` with early return. This
*could* be `for c in 0..4` if the language supports early return from for-loops
(which it does). Keeping as while in the proposal for safety.

---

## 2. Enums for Piece Types and Colors

### Before (magic numbers everywhere)
```nanz
global cur_type: u8 = 0       // 0-6: I, O, T, S, Z, L, J
if t == 0 { return 0x28 }     // cyan???
if cur_type != 2 { return 0 } // 2 = T... or was it S?
```

### After (self-documenting)
```nanz
enum Piece { I = 0, O, T, S, Z, L, J }
enum Attr {
    BLACK   = 0x00,
    BLUE    = 0x08,
    RED     = 0x10,
    MAGENTA = 0x18,
    GREEN   = 0x20,
    CYAN    = 0x28,
    YELLOW  = 0x30,
    WHITE   = 0x38,
    BRIGHT_YELLOW = 0x70
}

if cur_type != Piece::T { return 0 }  // crystal clear
```

---

## 3. Array LUTs for Piece Geometry (-400 lines)

The biggest win. `piece_dx()` and `piece_dy()` are ~210 lines each of nested
if/else encoding 7×4×4 = 112 values. Replace with array lookups:

### Before (210 lines per function)
```nanz
fun piece_dx(t: u8, r: u8, c: u8) -> u8 {
    if t == 0 {
        if (r == 0) | (r == 2) { return c }
        return 0
    }
    if t == 1 {
        if c == 0 { return 0 }
        if c == 1 { return 1 }
        ...  // 200+ more lines
```

### After (compact table + 3-line function)
```nanz
// 7 pieces × 4 rotations × 4 cells = 112 entries
global PIECE_DX: [u8; 112] = [
    // I: rot0    rot1    rot2    rot3
    0,1,2,3,  0,0,0,0,  0,1,2,3,  0,0,0,0,
    // O: all rotations identical
    0,1,0,1,  0,1,0,1,  0,1,0,1,  0,1,0,1,
    ...
]

fun piece_dx(t: u8, r: u8, c: u8) -> u8 {
    return PIECE_DX[u16(t) * 16 + u16(r) * 4 + u16(c)]
}
```

**Impact:** -400 lines of if/else, +30 lines of data tables. Net: **-370 lines**.

---

## 4. Bitwise AND for Random Piece

### Before
```nanz
fun random_piece() -> u8 {
    var rval: u8 = rng_next()
    while rval >= 7 { rval = rval - 7 }  // O(n) subtraction loop
    return rval
}
```

### After
```nanz
fun random_piece() -> u8 {
    let r: u8 = rng_next() & 7     // AND 0x07 — one Z80 instruction
    if r >= 7 { return 0 }          // reject 7 → remap to I-piece
    return r
}
```

**Impact:** O(1) instead of O(n), compiles to `AND 7` + `CP 7` + `JR`.

---

## 5. Named Constants for Screen Layout

### Before
```nanz
zx_poke(zx_attr_addr(11 + x, 2 + y), attr)  // what is 11? what is 2?
if cx >= 10 { return false }                   // why 10?
```

### After
```nanz
// Screen layout constants
global BOARD_X: u8 = 11       // playfield left column (screen chars)
global BOARD_Y: u8 = 2        // playfield top row
global BOARD_W: u8 = 10       // board width (cells)
global BOARD_H: u8 = 20       // board height (cells)
global CELLS_PER_PIECE: u8 = 4

zx_poke(zx_attr_addr(BOARD_X + x, BOARD_Y + y), attr)
if cx >= BOARD_W { return false }
```

---

## 6. For-Each on Piece Cells (minor, aesthetic)

Several functions iterate over 4 cells of a piece identically. With `for`:

### Before
```nanz
var c: u8 = 0
while c < 4 {
    let cx: u8 = cur_x + piece_dx(cur_type, cur_rot, c)
    let cy: u8 = cur_y + piece_dy(cur_type, cur_rot, c)
    ...
    c = c + 1
}
```

### After
```nanz
for c in 0..4 {
    let cx: u8 = cur_x + piece_dx(cur_type, cur_rot, c)
    let cy: u8 = cur_y + piece_dy(cur_type, cur_rot, c)
    ...
}
```

---

## Expected Impact

| Metric | Original | Beautified | Change |
|--------|----------|------------|--------|
| Total lines | 852 | ~480 | -44% |
| While loops | 25 | 7 | -72% |
| For loops | 0 | 18 | +18 |
| Magic numbers | ~30 | 0 | -100% |
| Enum types | 0 | 2 | +2 |
| Array LUTs | 0 | 3 | +3 |
| Asm inserts | 5 | 5 | same |
| Behavior | identical | identical | same |

---

## Risk Assessment

| Change | Risk | Notes |
|--------|------|-------|
| while → for | Low | `for i in 0..n` is well-tested |
| Enums | Low | Enum values are ✅ DONE |
| Array LUTs | Medium | Array literal init codegen is 🚧 WIP — may not optimize well |
| `& 7` random | None | Bitwise AND is fully implemented |
| Constants | None | Just globals with names |
| u16 ranges | Medium | `for addr in 0x4000..0x5800` needs u16 range support |

**Recommendation:** Ship as `tetris_v2.nanz` alongside original. Original stays
as the "known-working" reference; v2 demonstrates idiomatic Nanz style.

---

## File

See: `examples/zx/tetris_v2.nanz`
