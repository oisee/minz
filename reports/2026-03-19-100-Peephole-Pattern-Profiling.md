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

## Z80 Superoptimizer (~/dev/z80-optimizer)

Our own CUDA-accelerated brute-force superoptimizer has already proven
**602,008 instruction equivalences** for length-2 Z80 sequences.

Architecture:
- 455 opcodes enumerated (all Z80 instructions including undocumented)
- 3-level verification: QuickCheck (8 vectors) → MidCheck (32) → Exhaustive (2^24)
- 2.7ns/op Go executor, CUDA GPU search for bulk enumeration
- Full state equivalence (registers + flags + memory byte)

### Results Database

```
Rules by bytes saved:
  −3 bytes:    1,212 rules (e.g. SLA A; RR A → OR A)
  −2 bytes:  580,937 rules
  −1 byte:   19,859 rules
  Total:    602,008 proven rules
```

### Cross-Reference with Our Hot Patterns

| Our pattern | Superoptimizer match | Saving |
|-------------|---------------------|--------|
| `LD A, 0; *` | 2,252 rules (→ XOR A family) | −1-3B |
| `SBC A, *` | 19,934 rules | varies |
| `CP 0; *` | 206,289 rules (most are trivial) | −1-3B |

### Next: Length-3 Search

Length-2 is complete. Length-3 (74.8B targets) requires GPU:
- CUDA v2 batched pipeline ready (`z80_search_v2.cu`)
- Expected: 10-100× more optimizations in non-obvious 3-instruction patterns
- Will find patterns like our SBC+ADD+branch fusion automatically

## Methodology

Patterns identified by profiling **our own generated assembly** across
57 Nanz + C89 files (~7000 instructions). This avoids GPL contamination
while targeting the patterns that actually matter for our compiler.

## Superoptimizer Validation

Ran `z80opt target "OR A : SBC HL, DE : ADD HL, DE"` — no shorter
equivalent exists at instruction level (69M checks/s, CPU). This confirms
that the SBC+ADD optimization must be done at **MIR2 level** (omit the
ADD when only flags are needed), not as a peephole rewrite.

Key insight: **most of our hot patterns are already optimal at instruction
level** (length-2 superoptimizer confirms). The wins are at higher levels:
- MIR2: flags-only comparison (skip ADD restore)
- MIR2: CSE / value numbering (already done with redundant-load)
- Regalloc: better register targeting (DE vs HL choice)

## Next Steps

1. **SBC+ADD flags-only** — Grace rule at MIR2 level (~20 LOC)
   - Detect `cmp u16` where result is only used as branch condition
   - Mark as flags-only so codegen skips ADD HL,rr restore
   - Expected: −115 instructions across corpus
2. **DEC SP pair** — ISLE rule at LIR/peephole level (~5 LOC)
3. **Length-3 GPU search** — run CUDA v2 on targeted 3-instruction sequences
   from our compiler output (not full enumeration)
4. **STOKE** — for length-4+ optimization of hot functions
