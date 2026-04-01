# Next Session Seed — 2026-04-02

## READY TO TEST (from VIR backend, on minz-vir repo)

### P1+P2 commits (need merge from minz-vir)
- arithmetic idiom: div/256→LD L,H, mul*2048→ADD HL×11
- MIR2 DAG inliner: --vir-inline, lfsr_step inlined in loop
- enriched loader: Z80T v2 graceful reject
- parallel-move ordering fix
Test: `git pull minz-vir && mz --vir-inline fun/che_nanz.nanz`

### 11-loc enriched table (61M entries, 86% feasible)
- data/enriched_5v.enr: 17.4M entries, load via VIR_ENRICHED_5V=path
- data/ix_expanded_5v.jsonl: 61M (11-loc with IXH/IXL/IYH/IYL)
- 6v_dense: running (~66M shapes, ETA minutes)

## ARCHITECTURE (decided)

### 18-Register Model
- 11 GPR active: LocCost IXH=1, ADD IX/IY patterns ✅
- IXH/IXL = zero-cost inter-zone bridges (survive EXX+AF swap) ✅
- EXX pragmatic path: S1+S2 independent table lookups + IX bridges
- Formula: total = cost(S1) + cost(S2) + 8T×bridges + 4T×EXX_count
- P5 infrastructure EXISTS: FindCutVertices+IsBipartite in regalloc_table.go

### Che Gap Roadmap (240ms target, beats hand-written 290ms)
- P1 arithmetic idiom: ✅ committed (saves 459ms)
- P2 MIR2 DAG inliner: ✅ committed (saves ~90K T-states)
- P3 IYL counter: pending (DEC IYL sets Z flag, confirmed)
- P4 LICM / embedded globals: pending
- P5 EXX zone: infrastructure ready, wire into Lookup() needed

### Cross-Zone Channels
- IX halves: 0T (survive EXX+AF)
- A: 0T (EXX pass-through)
- TSMC tunnel: 20T (8-bit, code-memory patching)
- PUSH/POP: 21T (stack, always safe)
- R register: 26T boolean spill (bit7, zero named regs)

## FUN/ STATUS (all pass)
- 13 files, 185+ asserts, 9/9 regression ✓
- Che decoder: 394B (3.3× vs 121B asm)
- With P1+P2: expected ~300B (~2.5×)

## ARTICLES
- docs/2026-03-31-PRNG-Image-Decoding-On-Z80.md (3 chapters + CP codec)
- docs/2026-03-31-Closing-The-Gap.md
- docs/2026-03-31-Nanz-Codegen-Quality.md (9 sections)
- docs/2026-04-01-Che-Gap-Proposal.md (397 lines, compiler beats hand-written)
- docs/z80_opref.md (415 lines, complete Z80 reference)
- reports/2026-03-31-Cross-Team-Sprint.md

## GPU Tables Available
| Table | Entries | Status |
|-------|---------|--------|
| mul8 A×K→A | 254 | ✅ in codegen |
| mul16 HL×K→HL | 254 | ✅ in codegen (7.7× speedup) |
| div8 A÷K→A v3 | 254 | ✅ loaded (carry_compare for K≥128) |
| u32 ops (DEHL+HLH'L') | 16 | ✅ loaded |
| enriched 4v | 156K | ✅ default |
| enriched 5v | 17.4M | ✅ via VIR_ENRICHED_5V |
| ix_expanded 5v (11-loc) | 61M | ✅ ready |
| ix_expanded 6v_dense | ~66M | ⏳ running |
| OFB flags | ready | ✅ ComputeOFB() API |
