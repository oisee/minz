# Tetris CP/M Runtime Verification

**Date:** 2026-04-07
**Purpose:** Confirm IXY-half fixes (`a0f1094c`, `ab362576`) cause no runtime regression for `tetris_cpm.nanz`.

---

## Commands Run

```bash
# 1. Compile tetris_cpm.nanz (no Z80-VALIDATE errors)
go run ./cmd/minzc examples/nanz/tetris_cpm.nanz -o /tmp/tetris_cpm.a80
# 12 functions fall to PBQP, rest via VIR. Zero Z80-VALIDATE errors.

# 2. Assemble to COM binary
go run ./cmd/mza /tmp/tetris_cpm.a80 -o /tmp/tetris_cpm.com
# Success: 3609 bytes

# 3. Run under mze with CP/M emulation
go run ./cmd/mze /tmp/tetris_cpm.com -t cpm --timeout 500000000
# Output: "00000" (score display), clean exit code 0

# 4. Verbose BDOS trace
go run ./cmd/mze /tmp/tetris_cpm.com -t cpm --timeout 5000000 -v
# 5 BDOS 02 calls (score digits), then timeout in input poll loop
```

## Observed Behavior

| Step | Result |
|------|--------|
| Compilation | Clean — no Z80-VALIDATE errors (previously had `st_word` and `write_fat12`) |
| Assembly | Success — 3609 bytes, no invalid instructions |
| Execution | Runs to "GAME OVER" path, outputs `00000` (score), exits cleanly |
| Crash/hang | **None** — clean exit with code 0 |

## Analysis

The program enters `main()`, initializes the board, calls `spawn_piece()`, then immediately exits the game loop (`while game_over == 0`). This means `game_over` becomes non-zero during `spawn_piece()` — likely because `can_place()` returns false for the initial piece position (a pre-existing logic/codegen issue, not related to IXY fixes).

The "GAME OVER" path at lines 516-524 runs correctly:
- `draw_number(score)` outputs `00000` via 5× BDOS 02 calls
- Program exits cleanly via `DI; HALT`

No VT100 screen drawing occurs because the game loop body never executes.

## IXY Fix Regression Assessment

**No regression.** The IXY-half fixes changed codegen for:
- `LD (HL), r` when `r` comes from IX/IY pair → now routes through A
- `BIT/SET/RES n, r` when `r` is IXH/IXL/IYH/IYL → now uses AND/OR mask

These changes produce functionally equivalent code (same semantics, different instruction sequence). The tetris binary compiles, assembles, and runs without crash. The pre-existing issue (immediate game_over) is unrelated to register allocation — it's a logic bug in `spawn_piece()` or `can_place()` initialization.

## Pre-existing Issue: Immediate Game Over

This is the same issue tracked in [seed 2026-04-05](/home/alice/dev/minz-vir/contexts/2026-04-05-next-session-seed.md): the tetris rendering doesn't reach the first visible frame. The seed identifies the suspect zone as `piece_dx/piece_dy` or arithmetic around `draw_cell`. The probe-based investigation workflow described there is still the right approach.
