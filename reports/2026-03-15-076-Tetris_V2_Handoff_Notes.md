# Report #076 — Complete Handoff Notes for Neighbor

**Date:** 2026-03-15
**Status:** Ready for review
**Context:** Tetris beautification, loop reversal proof, register allocator research

---

## What Was Done Today

1. Analyzed original `tetris.nanz` (852 lines) for idiomaticity
2. Wrote `tetris_v2.nanz` — idiomatic rewrite (488 lines), **compiles to 2188 asm**
3. Wrote formal loop reversal equivalence proof for DJNZ optimization
4. Fixed stale `~/bin/mz` binary (was from Mar 13, missing enum support)
5. Verified: enums, array LUT init, offset ranges all work in .nanz
6. Compiled full reading list of register allocator constraint research

---

## Part 1: Tetris V2 — Files & Findings

### New Files (today, all verified)

| # | File | What | Status |
|---|------|------|--------|
| 1 | `examples/zx/tetris_v2.nanz` | Idiomatic rewrite: enums, LUT arrays, for-range, `& 7` | ✅ 488 lines → 2188 asm |
| 2 | `reports/2026-03-15-075-Tetris_Beautification_Proposal.md` | Full analysis: 25 while-loops categorized, all changes documented | ✅ |
| 3 | `docs/Loop_Reversal_Equivalence_Proof.md` | Formal proof: when forward→reverse iteration is safe for DJNZ | ✅ |

### Key Discoveries

**Enum syntax is DOT, not double-colon:**
```nanz
// ✅ Correct (compiles):
if cur_type != Piece.T { return 0 }
zx_poke(addr, Attr.WHITE)

// ❌ Wrong (parse error):
if cur_type != Piece::T { return 0 }
```
Nanz uses `Enum.Variant` (like Ruby/Crystal), not `Enum::Variant` (Rust).
Verified with showcase examples `ex21_enum.nanz`, `ex26_enum_assert.nanz`.

**Stale binary trap:**
`make install-user` installs to `~/.local/bin/`, but `~/bin/` had old
binaries that took precedence in PATH. Fixed by copying fresh binaries.
**Action item:** Add `~/bin/` to Makefile `install-user` target.

**Array LUT init works:**
112-element `global PIECE_DX: [u8; 112] = [...]` compiles correctly.
Output is 17 lines *shorter* than the if-chain version (2188 vs 2205).

**Offset ranges work:** `for cy in 2..22`, `for bx in 10..22` — all compile fine.

### What Changed in V2

| Change | Lines saved | Codegen impact |
|--------|-------------|----------------|
| `piece_dx`/`piece_dy` if-chains → LUT arrays | -370 | -17 asm lines, table lookup instead of branches |
| 18× `while` → `for i in range` | -36 | Same asm (DJNZ when possible) |
| Magic numbers → `enum Piece`, `enum Attr` | ~0 | Same asm (constants folded) |
| `random_piece()` subtraction → `& 7` | -2 | `AND 7` + `CP 7` instead of loop |
| Screen addresses → named constants | ~0 | Same asm |
| **Total** | **-364** | **-17 asm lines** |

### Loop Reversal Proof (summary)

For `for i in 0..n { body(i) }`, reverse iteration (n-1→0) produces
identical results when all iteration effects commute:

- **Independent indexed stores:** `array[i] = expr` where `i ≠ j` → disjoint addresses → commute ✅
- **Commutative accumulator:** `sum = sum + f(i)` → `+` commutes ✅
- **Loop-carried dependency:** `a[i] = a[i-1] + 1` → does NOT commute ❌

Static check: if `W(i) ∩ R(j) = ∅` for all `i ≠ j`, loop is reversible.
Nanz makes this easier than C: no pointers, no aliasing, explicit array indexing.

**Proposal:** Add `canReverse()` analysis pass to MIR2 → emit DJNZ for reversed loops.

---

## Part 2: Register Allocator Constraint Research

### ADR Documents

| # | File | Topic | Status |
|---|------|-------|--------|
| 4 | `docs/adr/0017-lut-pointer-selection-and-pbqp-edge-costs.md` | PBQP affinity edges for LUT: HL vs BC★ vs DE★, IX/IY exclusion | **Accepted** |
| 5 | `docs/adr/0020-instruction-level-constraints.md` | `EdgeCost(op, dst, src)` — pairwise operand constraints for PBQP | **Proposed** |

### Reports

| # | File | Topic |
|---|------|-------|
| 6 | `reports/2026-03-10-049-LUT_Pointer_Selection_And_PBQP_Edge_Costs.md` | LUT fast path: `LD H, sym^H` (-3T/-1byte), BC★/DE★ opportunity |
| 7 | `reports/2026-03-14-071-Arena_Z80_Codegen_IXL_Bug.md` | BUG-008 full RCA: impossible `LD IXL,(IX+0)`, DD prefix conflict |
| 8 | `reports/2026-03-13-067-BUG001_PreallocCoalesce_Wired.md` | BUG-001: PBQP spills, pre-allocation coalescing |

### Bug & Roadmap Files

| # | File | Why |
|---|------|-----|
| 9 | `docs/Open_Bugs_RCA.md` | BUG-001 (PBQP spills) + BUG-008 (IX/IY) — both directly related |
| 10 | `docs/Allocator_Roadmap.md` | Overall allocator roadmap |
| 11 | `docs/adr/ADR-0006*`, `ADR-0007*` | Register allocator design decisions |

### ADR-0020: Instruction-Level Constraints (key details)

**Problem:** PBQP allocator assigns operand combinations that are impossible
to encode. `LD IXL, (IX+d)` can't exist — DD prefix can't simultaneously
remap destination (L→IXL) and source ((HL)→(IX+d)).

**Solution:** Three-layer defense:

```
Layer 1 — PBQP EdgeCost     allocator AVOIDS bad combinations     (prevention)
Layer 2 — Codegen guards     emitter FIXES remaining edge cases    (mitigation)
Layer 3 — MZA reject         assembler CATCHES anything missed     (detection)
```

**Four Z80 EdgeCost rules:**

| Rule | Pattern | Cost | Why |
|------|---------|------|-----|
| 1 | `LD IXL, (IX+d)` | ∞ | DD prefix conflict — can't remap both dest and source |
| 2 | `LD H, (IX+d)` | +4 | Valid but H is "real H", not IXH — confusion risk |
| 3 | `LD IX, (IX+d)` (16-bit) | ∞ | Self-clobber: first byte load corrupts IX base |
| 4 | `LD H, IXH` | ∞ | DD makes it `LD IXH, IXH` (NOP) |

**API:**
```go
type CostTable interface {
    Locs() []PhysLoc
    Cost(cls RegClass, loc PhysLoc) int          // existing: move/storage cost
    EdgeCost(op Op, dstLoc, srcLoc PhysLoc) int  // NEW: pairwise constraint
}
```

**Why pairwise, not per-register:** `IXL` as destination is fine for
`LD IXL, A`, but forbidden for `LD IXL, (IX+0)`. Removing IXL globally
is too aggressive. PBQP edge cost matrices handle this naturally.

### ADR-0017: LUT Pointer Selection (key details)

**All LUT access options:**

| Ptr | Total T | When to use |
|-----|---------|-------------|
| HL | 18T | General case — `LD H, sym^H; LD L, idx; LD A,(HL)` |
| BC★ | 14T | If idx already in C and B free — `LD B, sym^H; LD A,(BC)` |
| DE★ | 14T | If idx already in E and D free — `LD D, sym^H; LD A,(DE)` |
| IX/IY | ~38T | **Never** — 19T per `LD A,(IX+0)`, displacement penalty not amortized |

**Fix shipped:** `LD HL, sym` → `LD H, sym^H` saves 3T + 1 byte per LUT access.
**BC★ planned:** Post-allocation codegen check for affinity optimization.

---

## Part 3: Open Questions

### Must verify (runtime)
1. **Run tetris_v2 on MZX** — plays correctly? (compilation verified, not runtime)
2. **$0000 spills in v2?** — check main loop asm for BUG-001 memory-backed registers

### Design decisions
3. **Implement ADR-0020 EdgeCost?** — prevents BUG-008 class of bugs at source
4. **`canReverse()` MIR2 pass?** — enables DJNZ for reversed counted loops
5. **BC★ LUT affinity?** — saves 4T per LUT access when idx already in C

### Blocked by BUG-001
6. **DJNZ everywhere** — proof is ready, but B spills to $F0xx kill the benefit
7. **Iterator fusion perf** — ~5x overhead from memory-backed registers
8. **Tetris main loop perf** — $0000 spills in game loop

### Future (post bug fixes)
9. **Struct game state** — blocked by BUG-008 (IX/IY). `tetris_v3.nanz` once fixed
10. **u16 ranges** — `for addr in 0x4000..0x5800` would clean up screen init
11. **`?` operator** — sugar for `@error` propagation (80% of monad benefit, 0% overhead)

---

## Compilation Quick-Check

```bash
# Make sure binary is fresh!
cd minzc && make clean && make all
cp mz ~/bin/mz   # if ~/bin is in PATH before ~/.local/bin

# Compile both versions
mz ../examples/zx/tetris.nanz -o /tmp/tetris_orig.a80    # 2205 lines asm
mz ../examples/zx/tetris_v2.nanz -o /tmp/tetris_v2.a80   # 2188 lines asm

# Run on MZX
./mzx --run /tmp/tetris_v2.bin@8000
```
