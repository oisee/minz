# Report 100 — Z80 Peephole Pattern Profiling

**Date:** 2026-03-19
**Method:** Profiled all 57 Nanz + C89 example files, counted instruction pair frequencies
**Purpose:** Find highest-value peephole optimizations by frequency, not guesswork

---

## Top Instruction Pairs in MinZ Z80 Output

| Count | Pattern | Optimization | Saving |
|-------|---------|-------------|--------|
| **61** | `SBC HL, DE; ADD HL, DE` | 16-bit CP emulation — remove ADD if only flags needed | −1 inst, −11T each |
| **54** | `SBC HL, HL; ADD HL, HL` | Same: zero-test via SBC, then restore | −1 inst each |
| **53** | `DEC SP; DEC SP` | → `PUSH AF` (allocate 2 bytes of stack) | −1 inst each |
| **37** | `ADD HL, DE; EX DE, HL` | Swap could be avoided if regalloc targets DE | arch-level |
| **29** | `EX DE, HL; OR A` | EX before SBC — might be avoidable | arch-level |
| **130** | `LD A, HL; RET` | Normal u8 return pattern | ✓ optimal |
| **99** | `LD L, A; LD H, 0` | u8→u16 zero-extension | ✓ optimal |
| **81** | `OR A; SBC HL, DE` | 16-bit subtract (clear carry) | ✓ optimal |

## Priority Analysis

### 1. SBC+ADD removal (115 occurrences, ~115 instructions saveable)

The pattern `OR A; SBC HL, rr; ADD HL, rr` is a **16-bit comparison**:
SBC sets flags (Z, C), then ADD restores HL. If the value of HL isn't
needed after the comparison (only flags matter), the ADD is dead.

**Grace rule:** detect at MIR2 level — if `cmp u16` result is only used
as a branch condition, mark the comparison as "flags only" so codegen
skips the ADD restore.

**Expected impact:** −115 instructions across the corpus.

### 2. DEC SP; DEC SP → PUSH AF (53 occurrences)

Two `DEC SP` instructions allocate 2 bytes of stack. A single `PUSH AF`
does the same in 1 instruction (1 byte, 11T vs 2×6T=12T).

**ISLE rule:** `(rule (dec_sp (dec_sp)) (push_af))`

**Expected impact:** −53 instructions across the corpus.

### 3. ADD HL, DE; EX DE, HL (37 occurrences)

Result of ADD needed in DE, not HL. If regalloc assigned the destination
to DE directly, both ADD and EX would be unnecessary.

**Fix:** PBQP cost model adjustment — prefer DE for results that flow
into DE-consuming operations. Not an ISLE rule.

## SDCC Peephole Analysis

SDCC's `peeph-z80.def` contains 217 rules (2448 lines), GPL-v2 licensed.
**We cannot copy their rules.** But the Z80 instruction set semantics
are mathematical facts, not copyrightable.

Our approach: profile our own output, identify patterns from Z80 ISA
knowledge, write independent ISLE/Grace rules.

### SDCC Rule Categories (for reference, not copying)

| Category | ~Rules | Our coverage | Gap |
|----------|--------|-------------|-----|
| Dead load elimination | ~50 | DSE at MIR2 level | Post-regalloc text peephole |
| EX DE,HL optimization | ~20 | Double-EX cancel | EX avoidance |
| Flag optimization | ~15 | CP 0→OR A, XOR flag | Flag propagation |
| PUSH/POP optimization | ~15 | Same-reg removal | Cross-reg transfer |
| Jump optimization | ~10 | Trivial branch, chain | Jump redirect |
| INC/DEC patterns | ~5 | ISLE inc/dec | Pair increment |
| Specialized patterns | ~30 | Partial | LDIR, carry tricks |

## Methodology

Patterns identified by profiling **our own generated assembly** across
57 Nanz + C89 files (~7000 instructions). This avoids GPL contamination
while targeting the patterns that actually matter for our compiler.

## Next Steps

1. **SBC+ADD flags-only** — Grace rule at MIR2 level (~20 LOC)
2. **DEC SP pair** — ISLE rule at LIR/peephole level (~5 LOC)
3. **Copy propagation** — bypass intermediate loads (~Grace rule)
4. **PUSH HL; POP DE** → `LD E,L; LD D,H` — ISLE peephole (~5 LOC)
