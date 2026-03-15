# Report #076 — Tetris V2 Handoff Notes for Neighbor

**Date:** 2026-03-15
**Status:** Ready for review
**Context:** Analysis & idiomatic rewrite of `examples/zx/tetris.nanz`

---

## What Was Done

1. Analyzed original tetris.nanz (852 lines) for idiomaticity
2. Wrote `tetris_v2.nanz` — idiomatic rewrite (488 lines)
3. Wrote formal loop reversal proof for DJNZ optimization
4. Fixed stale `~/bin/mz` binary (was from Mar 13, missing enum support)
5. **Both versions compile and produce comparable asm output**

---

## Files To Review

### New (today)

| # | File | What | Status |
|---|------|------|--------|
| 1 | `examples/zx/tetris_v2.nanz` | Idiomatic rewrite: enums, LUT arrays, for-range, `& 7` | ✅ Compiles (2188 asm lines) |
| 2 | `reports/2026-03-15-075-Tetris_Beautification_Proposal.md` | Full analysis: 25 while-loops categorized, all changes documented | ✅ |
| 3 | `docs/Loop_Reversal_Equivalence_Proof.md` | Formal proof: when forward→reverse iteration is safe for DJNZ | ✅ |

### Existing (context)

| # | File | Why |
|---|------|-----|
| 4 | `docs/Open_Bugs_RCA.md` | BUG-001 (PBQP spills) — main perf blocker for Tetris |
| 5 | `reports/2026-03-14-071-Arena_Z80_Codegen_IXL_Bug.md` | BUG-008 (IX/IY) — blocks struct methods in Tetris |
| 6 | `docs/adr/ADR-0006*`, `ADR-0007*` | Register allocator design decisions |

---

## Key Discoveries

### 1. Enum syntax is DOT, not double-colon
```nanz
// ✅ Correct (compiles):
if cur_type != Piece.T { return 0 }
zx_poke(addr, Attr.WHITE)

// ❌ Wrong (parse error):
if cur_type != Piece::T { return 0 }
```

Nanz uses `Enum.Variant` (like Ruby/Crystal), not `Enum::Variant` (Rust).
Verified with showcase examples `ex21_enum.nanz`, `ex26_enum_assert.nanz`.

### 2. Stale binary trap
`make install-user` installs to `~/.local/bin/`, but `~/bin/` had old
binaries that took precedence in PATH. After `make clean && make all`,
must copy to `~/bin/` manually or fix PATH order.

**Action item:** Consider adding `~/bin/` to the Makefile `install-user`
target, or remove stale `~/bin/mz*` to avoid future confusion.

### 3. Array LUT init works
112-element `global PIECE_DX: [u8; 112] = [...]` compiles correctly.
The codegen handles large array literal initialization without issues.
Output is 17 lines *shorter* than the if-chain version (2188 vs 2205).

### 4. For-range with offset works
`for cy in 2..22`, `for bx in 10..22`, etc. all compile fine.
No u16 range tested (screen init still uses while for 0x4000..0x5800).

---

## What Changed in V2 (summary)

| Change | Lines saved | Codegen impact |
|--------|-------------|----------------|
| `piece_dx`/`piece_dy` if-chains → LUT arrays | -370 | -17 asm lines, table lookup instead of branches |
| 18× `while` → `for i in range` | -36 | Same asm (DJNZ when possible) |
| Magic numbers → `enum Piece`, `enum Attr` | ~0 | Same asm (constants folded) |
| `random_piece()` subtraction → `& 7` | -2 | `AND 7` + `CP 7` instead of loop |
| Screen addresses → named constants | ~0 | Same asm |
| **Total** | **-364** | **-17 asm lines** |

---

## What V2 Does NOT Change

- 5 asm inserts (zx_poke, zx_peek, zx_key_row, zx_border, zx_halt) — identical
- Game logic (wall kicks, T-spin, hold, ghost piece) — identical
- Rendering approach (attribute-only, no compiled sprites) — identical
- 7 while loops that can't be for-ranges (game loop, ghost_y, hard drop, clear_lines, screen init)

---

## Open Questions for You

1. **Run tetris_v2 on MZX** — does it actually play correctly? (We verified compilation, not runtime)
2. **$0000 spills** — are they still present in v2's main loop? (BUG-001)
3. **Loop reversal in MIR2** — the proof doc proposes a `canReverse()` pass. Worth adding to the pipeline? Or wait for BUG-001 fix first?
4. **Struct for game state** — blocked by BUG-008 (IX/IY). Once fixed, `tetris_v3.nanz` could group globals into `struct GameState`
5. **Screen init** — `for addr in 0x4000..0x5800` would need u16 range. Currently still uses while. Worth implementing u16 ranges?

---

## Compilation Quick-Check

```bash
# Make sure binary is fresh!
cd minzc && make clean && make all
cp mz ~/bin/mz   # if ~/bin is in PATH before ~/.local/bin

# Compile both versions
mz ../examples/zx/tetris.nanz -o /tmp/tetris_orig.a80    # 2205 lines
mz ../examples/zx/tetris_v2.nanz -o /tmp/tetris_v2.a80   # 2188 lines

# Run on MZX
./mzx --run /tmp/tetris_v2.bin@8000
```
