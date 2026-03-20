# Report 102 — Tetris Compiles, FatFS RCA, LIR vs MIR2 Bug Analysis

**Date:** 2026-03-20
**Status:** Tetris verified working (MIR2). FatFS bug root-caused. LIR bugs documented.

---

## Tetris: Full ZX Spectrum Game Compiles and Runs

```
Source:    examples/zx/tetris.nanz (664 LOC Nanz)
Assembly:  2238 lines Z80 asm
Binary:    3471 bytes
Backend:   MIR2 (--lir=false)
Verified:  12/12 assertions PASS (MIR2 VM)
```

### MZV ZX Screen Output (frame 5, `--zx` mode)

```
                    ##      @@@@@@@@      ##
                    ##                    ##  @@@@@@@@
                    ##                    ##
                    ##                    ##
                    ##                    ##
                    ##                    ##
                    ##                    ##
                    ##                    ##
                    ##                    ##
                    ##                    ##
                    ##                    ##
                    ##                    ##
                    ##                    ##
                    ##                    ##
                    ##                    ##
                    ##                    ##
                    ##                    ##
                    ##                    ##
                    ##                    ##
                    ##      ........      ##
                    ########################

Legend: ## = wall (white), @@ = I-piece (cyan), .. = bottom piece (falling)
```

Board rendering is correct: 10-wide playfield with walls, I-piece (cyan) spawned
at top, next piece shown at right, bottom piece has landed. ZX Spectrum attribute
colors rendered via MZV's `--zx` ANSI terminal mode.

### Frame Dump

5 `.scr` files (6912 bytes each = ZX Spectrum screen format: 6144 bitmap + 768
attributes) dumped via `--dump-frames`. Compatible with ZX Spectrum screen
viewers and MZX emulator.

### MZE Emulator

Binary runs in Z80 emulator (`mze`). Game loop is infinite (correct for a game),
execution verified up to 500K T-states with no crashes or illegal opcodes.

### Test File Verification

`examples/zx/tetris_mir2_test.nanz` (89 LOC) — 12 compile-time assertions:

| Test | Result |
|------|--------|
| I-piece geometry (dx/dy) | PASS |
| O-piece geometry | PASS |
| Piece colors (7 types) | PASS |
| Cell color lookup | PASS |
| Board read/write | PASS |

All pass silently (exit 0) via MIR2 VM at compile time.

---

## FatFS `test_follow_chain` Bug — Root Cause Analysis

**Symptom:** `assert test_follow_chain(2, 10) == 3 via mir2` — got 1, want 3.

**Root Cause:** Test setup bug, NOT a codegen or VM bug.

```
test_follow_chain(start=2, max=10)
  → chain_length(&g_buf, start=2, max_hops=10)
    → read_fat(&g_buf, clst=2)
      → fs_type == 0 (unmounted!)        ← BUG: never initialized
      → returns 0xFFFF (fallthrough)
    → is_eoc(0xFFFF) → true
    → running = 0, loop exits after 1 iteration
    → returns 1
```

**Fix:** Add `fs_type = 1` before calling `chain_length` in `test_follow_chain`:

```minz
fun test_follow_chain(start: u8, max: u8) -> u8 {
    fs_type = 1    // ← MISSING: tell read_fat to use FAT12 path
    g_buf[0] = 0xF8
    ...
    return chain_length(&g_buf, start as u16, max)
}
```

**Impact:** All other FatFS tests pass (VM: 5/5 read, 7/7 write, QBE: 33/33).
Only `test_follow_chain` and `test_follow_chain_2` are affected.

**Severity:** Low — test-only bug. The actual FatFS filesystem operations
(`f_open`, `f_read`, `f_write`) correctly call `f_mount` which sets `fs_type`.

---

## LIR vs MIR2 — Tetris Bug Analysis

**Symptom:** `board_set` generates invalid Z80 instructions with LIR backend.

```
; LIR output for board_set(x: u8, y: u8, val: u8)
board_set:
    LD HL, board
    ; TODO: general mul A * E → E     ← multiply NOT emitted!
    LD A, E
    ADD A, C
    LD DE, 0
    LD E, A
    ADD HL, DE
    LD (HL), D
    RET
```

**Root Cause:** LIR's ISLE pattern table has no rule for 8-bit multiply
(`OpMul u8`). The `board[y * 10 + x] = val` expression requires `y * 10`
which needs `__mul8` runtime or strength reduction. LIR emits a TODO comment
and skips the multiply entirely, producing wrong code.

MIR2 backend handles this via `genMul8` which emits shift-add sequences or
calls `__mul8` runtime routine.

**Classification:**

| Issue | Backend | Category | Severity |
|-------|---------|----------|----------|
| Missing 8-bit multiply | LIR | Missing isel pattern | Medium |
| Duplicate label `board_set:` | LIR | Emit bug | Low |
| Invalid LD instructions (other funcs) | LIR | WFC constraint gaps | Medium |

**MIR2 workaround:** `--lir=false` flag. Tetris compiles and runs correctly
with MIR2 backend.

**LIR fix path:** Add ISLE rule for `(mul u8 ?x ?y)` → strength reduction
or `CALL __mul8` emission. The `__mul8` runtime (A*B→A, ~80T) already exists
in MIR2's z80codegen.go and can be shared.

---

## Summary

| Component | Status | Backend | Notes |
|-----------|--------|---------|-------|
| Tetris compile | PASS | MIR2 | 664 LOC → 3471B binary |
| Tetris MZV render | PASS | MIR2 | Board, pieces, colors correct |
| Tetris assertions | 12/12 PASS | MIR2 | Piece geometry + board ops |
| Tetris LIR | FAIL | LIR | Missing 8-bit multiply pattern |
| FatFS VM | PASS | VM | 5/5 read, 7/7 write |
| FatFS QBE | PASS | QBE | 33/33 native |
| FatFS Z80 | SKIP | MIR2 | `test_follow_chain` test setup bug |
| FatFS follow_chain fix | Identified | — | Add `fs_type = 1` in test wrapper |
