# Next Session Seed — 2026-04-02

## READY TO PICK UP

### 11-loc GPU tables (overnight run complete)
- data/ix_expanded_5v.jsonl — 2.3M+ shapes, 11 locs (A,B,C,D,E,H,L,IXH,IXL,IYH,IYL)
- Load into VIR enriched table, test on fun/ examples
- Expected: higher O(1) hit rate, fewer Z3 calls

### IXH→pair fix (41c4e41d from VIR backend)
- Pull and verify: LD BC, IXH → LD C, IXH (correct)
- sha256 and widemath should now ASSEMBLE (were blocked by this bug)

### ISLE combining rules LIR→VIR (VIR backend working on it)
- load16_le fusion, MUL strength reduction, store16_le
- Expect PR from p0gf238i

### mze screenshot for Che decoder
- Problem: mze SNA loader doesn't RETN — use raw .bin with --load/--start
- Need: longer timeout (120s+) or fewer seeds
- Alternative: fix mzv EnsureHeap for 0xC000 buffer

## ARCHITECTURE DECISIONS (recorded)

### 18-Register Model
- 11 GPR active: LocCost IXH=1, ADD IX/IY patterns ✅
- EXX-zone: separate 6-loc table + compose at boundary
- IXH/IXL = zero-cost inter-zone bridges (survive EXX+AF swap)
- Cross-zone channels: IX(0T), A(0T), TSMC(20T), PUSH/POP(21T)
- HLH'L' > DEHL when DE needed as pointer

### Che Gap: 8.6×→3.2× (structural limit without interprocedural regalloc)
- Instruction mix: 61% data moves = cost of 7 GPR
- Split functions > inline on Z80 (Paper B insight)
- Remaining: globals in memory, CALL overhead

## ARTICLES WRITTEN
- docs/2026-03-31-PRNG-Image-Decoding-On-Z80.md (3 chapters)
- docs/2026-03-31-Closing-The-Gap.md
- docs/2026-03-31-Nanz-Codegen-Quality.md (9 sections + bool returns)
- reports/2026-03-31-Cross-Team-Sprint.md

## SPRINT STATS
- ~60 commits across 5 sessions
- 800 GPU-verified arithmetic entries
- 4 novel Z80 discoveries (Z materialization, carry_compare, sat_add8, split>inline)
- 13 fun/ examples, 185+ asserts
- 4 articles, 3 reports
