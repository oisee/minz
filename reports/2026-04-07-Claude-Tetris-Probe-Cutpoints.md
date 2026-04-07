# Tetris CP/M: Probe Cutpoints for Spawn Path Isolation

**Date:** 2026-04-07
**Purpose:** Map the spawn path and propose 3-step probe sequence to isolate the broken layer.
**Non-goals:** No edits, no fixes, no runtime experiments.

---

## Spawn Path Call Graph

```
main()
  ├─ board init: zero 200 bytes via pointer walk (&board)
  ├─ next_type = random_piece()         ← rng_next() & 7, guard >= 7
  ├─ spawn_piece()                      ← THE CRITICAL CALL
  │     ├─ cur_type = next_type         (0..6)
  │     ├─ cur_rot = 0, cur_x = 3, cur_y = 0
  │     └─ can_place(cur_type, 0, 3, 0) ← if false → game_over = 1
  │           ├─ piece_dx(t, 0, 0..3)   ← pointer walk into PIECE_DX[112]
  │           ├─ piece_dy(t, 0, 0..3)   ← pointer walk into PIECE_DY[112]
  │           ├─ cx = 3 + dx, cy = 0 + dy
  │           ├─ if cx >= 10 → false    ← u8 comparison
  │           ├─ if cy >= 20 → false    ← u8 comparison
  │           └─ if board_get(cx,cy)!=0 → false  ← pointer walk into board[200]
  ├─ tui_clear()                        ← NEVER REACHED if game_over=1
  └─ while game_over == 0 { ... }       ← NEVER ENTERED
```

## Suspect Layers (ordered by likelihood)

### Layer A: `&PIECE_DX` / `&PIECE_DY` address resolution

`piece_dx` starts with `var p: ^u8 = &PIECE_DX` (line 120). On Z80, `&PIECE_DX` must resolve to the label of the 112-byte global array. If `OpAddrOf("PIECE_DX")` emits the wrong address (or the global isn't laid out where the label says), every LUT lookup returns garbage.

This is the same `&global_array` mechanism as `&board` at line 444. If `&board` works (board init zeroes correctly) but `&PIECE_DX` doesn't, the issue is array-specific (alignment, naming, ordering).

### Layer B: Pointer arithmetic in `piece_dx` / `piece_dy`

The function computes `offset = t*16 + r*4 + c` via three sequential loops:
```nanz
while i < t { p = p + 16; i = i + 1 }   // skip t piece types
while i < r { p = p + 4;  i = i + 1 }   // skip r rotations
while i < c { p = p + 1;  i = i + 1 }   // skip c cells
```

If pointer addition `p = p + 16` computes incorrectly (u16 add on Z80), or the loop counter `i` (u8) wraps or doesn't increment, the pointer lands at a wrong offset. The `i = 0` reset between loops (lines 126, 131) is correct in source but could be lost in codegen.

### Layer C: `can_place` boolean return / comparison

`can_place` returns `bool` (true/false). The caller at line 315:
```nanz
if can_place(cur_type, cur_rot, cur_x, cur_y) == false {
    game_over = 1
}
```

If the bool return convention is broken (e.g., `true` returns as 0 instead of non-zero), `== false` would match and set `game_over`. This is the same bool ABI issue noted in the GPU-proven convention work (SBC A,A → 0xFF/0x00).

---

## 3-Step Probe Sequence

### Probe A: Hardcoded `piece_dx` bypass

**What:** Replace `spawn_piece()` with a version that hardcodes the piece_dx/piece_dy results for type 0, rot 0 (I-piece horizontal):

```nanz
// Expected: PIECE_DX[0..3] = 0,1,2,3  PIECE_DY[0..3] = 0,0,0,0
// So can_place(0, 0, 3, 0) checks cells (3,0),(4,0),(5,0),(6,0) — all valid
fun probe_spawn() -> void {
    cur_type = 0
    cur_rot = 0
    cur_x = 3
    cur_y = 0
    // Bypass piece_dx/piece_dy entirely:
    // Hardcode the 4 cells that I-piece-rot-0 should produce
    let cx0: u8 = 3 + 0  // = 3
    let cy0: u8 = 0 + 0  // = 0
    let cx1: u8 = 3 + 1  // = 4
    let cy1: u8 = 0 + 0  // = 0
    let cx2: u8 = 3 + 2  // = 5
    let cy2: u8 = 0 + 0  // = 0
    let cx3: u8 = 3 + 3  // = 6
    let cy3: u8 = 0 + 0  // = 0
    // All < 10 and < 20, board is empty → should NOT set game_over
    if cx0 >= 10 { game_over = 1; return }
    if cx1 >= 10 { game_over = 1; return }
    if cx2 >= 10 { game_over = 1; return }
    if cx3 >= 10 { game_over = 1; return }
    // If we get here, the comparison/control path is OK
}
```

**Cutpoint:** `spawn_piece()` call in `main()` (line 451)
**Proves:** Whether the comparison and bool path in can_place is broken, OR the bug is in piece_dx/piece_dy/board_get.
- If game loop runs → bug is in piece_dx/piece_dy or board_get
- If game loop still skipped → bug is in comparison codegen or game_over global init

### Probe B: `piece_dx` return value dump

**What:** Call `piece_dx(0, 0, 0)` and output the result via `tui_putch(48 + result)` before the game loop. Do the same for cells 1, 2, 3 and for `piece_dy`.

```nanz
fun probe_piece_values() -> void {
    // Expected output: "0123" for dx, "0000" for dy (I-piece, rot 0)
    tui_putch(48 + piece_dx(0, 0, 0))  // expect '0'
    tui_putch(48 + piece_dx(0, 0, 1))  // expect '1'
    tui_putch(48 + piece_dx(0, 0, 2))  // expect '2'
    tui_putch(48 + piece_dx(0, 0, 3))  // expect '3'
    tui_putch(48 + piece_dy(0, 0, 0))  // expect '0'
    tui_putch(48 + piece_dy(0, 0, 1))  // expect '0'
    tui_putch(48 + piece_dy(0, 0, 2))  // expect '0'
    tui_putch(48 + piece_dy(0, 0, 3))  // expect '0'
}
```

**Cutpoint:** Insert before `spawn_piece()` in `main()` (between lines 450-451)
**Proves:** Whether `piece_dx`/`piece_dy` return correct LUT values.
- If output is `01230000` → LUT functions work, bug is in can_place or bool return
- If output is garbage → bug is in `&PIECE_DX` address or pointer arithmetic

### Probe C: `&PIECE_DX` raw address dump

**What:** Print the raw pointer value of `&PIECE_DX` and the first 4 bytes at that address.

```nanz
fun probe_lut_address() -> void {
    var p: ^u8 = &PIECE_DX
    // Print address high/low as hex digits
    let addr: u16 = u16(p)
    tui_putch(48 + u8((addr >> 12) & 0xF))
    tui_putch(48 + u8((addr >> 8) & 0xF))
    tui_putch(48 + u8((addr >> 4) & 0xF))
    tui_putch(48 + u8(addr & 0xF))
    // Print first 4 bytes (should be 0,1,2,3 for I-piece dx rot 0)
    tui_putch(48 + p^)        // expect '0'
    p = p + 1
    tui_putch(48 + p^)        // expect '1'
    p = p + 1
    tui_putch(48 + p^)        // expect '2'
    p = p + 1
    tui_putch(48 + p^)        // expect '3'
}
```

**Cutpoint:** Insert before `spawn_piece()` in `main()` (between lines 450-451)
**Proves:** Whether `&PIECE_DX` resolves to the correct address AND the array data is present.
- If address is reasonable (0x01xx–0x0Fxx) and bytes are `0,1,2,3` → LUT data is fine, bug is in loop arithmetic
- If address is 0x0000 or bytes are wrong → `OpAddrOf` for global arrays is broken on Z80

---

## Recommended Execution Order

```
Probe A first:  bypass piece_dx/piece_dy with hardcoded values
  ├─ game loop runs    → Layer B/C (LUT or pointer arith broken)
  │    └─ Probe B: dump piece_dx return values
  │         ├─ correct  → can_place bool/comparison broken
  │         └─ wrong    → Probe C: dump &PIECE_DX raw address
  └─ game loop skipped → Layer C (comparison or game_over init broken)
```

Each probe is a self-contained `/tmp/` copy per the seed 2026-04-05 workflow — no repo edits.

---

## Key Expressions to Watch

| Expression | Expected | Where |
|-----------|----------|-------|
| `&PIECE_DX` | label address of 112-byte array | piece_dx line 120 |
| `piece_dx(0, 0, 0)` | 0 | PIECE_DX[0] |
| `piece_dx(0, 0, 3)` | 3 | PIECE_DX[3] |
| `can_place(0, 0, 3, 0)` | true (cells 3-6, row 0, empty board) | spawn_piece line 315 |
| `game_over` after spawn | 0 | main line 456 |
| `random_piece()` | 0..6 | rng_next() & 7, guard >= 7 |
