# MinZ Research Programme

**Three papers, one insight: backend decision spaces are finite and solvable.**

---

## The Core Insight

Z80's irregular register file (7 GPRs + 4 IX/IY halves + 3 pairs + mem spill = 15
locations) creates a constraint space with **low effective entropy**. The number of
unique constraint patterns is bounded (~315 for the entire MinZ corpus). This makes
exhaustive GPU enumeration practical — and turns register allocation into a
**solved game** for architectures below the tractability cliff (~16 locations).

## Papers

### [Paper A: Register Allocation as a Solved Game](paper-a/)
**Status: Data complete, draft reviewed.**

GPU exhaustive tables. 315 signatures. 97.8% convergence. Phase transition at 16
register locations. 88.2% cross-frontend transfer. O(1) compilation via direct PIR
emit. Zero solver dependency.

*Key claim: "For sub-cliff architectures, register allocation is a solved game."*

### [Paper B: Exact Inlining via Exhaustive Cost Oracle](paper-b/)
**Status: Architecture designed, prototype building.**

GPU table as exact cost oracle for inlining decisions. Bottom-up DP on call graph.
`cost(merged) < cost(A) + cost(B) + shuffle` → provably optimal inline decision.
36% T-state savings on real corpus.

*Key claim: "Inlining decisions become exact when register allocation costs are known."*

### [Paper C: Exhaustive Search Certificates](paper-c/)
**Status: Data available, framing refined.**

GPU exhaustive search produces certificates: proofs that no solution exists below a
given length. DIVMOD lower bound ≥13 instructions. 686 infeasibility certificates
from regalloc corpus. Every GPU table entry is a miniature theorem.

*Key claim: "Exhaustive search proves what DOESN'T work — equally valuable to proving what does."*

## Shared Infrastructure

All three papers share the same infrastructure:

```
                     ┌─────────────────┐
                     │  GPU Kernel      │
                     │  (CUDA, 15-loc)  │
                     └────────┬────────┘
                              │
              ┌───────────────┼───────────────┐
              │               │               │
        ┌─────┴─────┐  ┌─────┴─────┐  ┌─────┴─────┐
        │  Paper A   │  │  Paper B   │  │  Paper C   │
        │  regalloc  │  │  inlining  │  │  negative  │
        │  table     │  │  oracle    │  │  certs     │
        └───────────┘  └───────────┘  └───────────┘
```

- **GPU kernel:** z80-optimizer (CUDA, `--server` mode, width-aware, 15-loc)
- **VIR backend:** MinZ compiler (Z3 + GPU table, direct PIR emit)
- **Corpus:** Nanz + ABAP + C89 + PL/M + Frill (8 frontends, 645+ functions)
- **Demo:** ZSQL on CP/M, ABAP ALV on ZX Spectrum

## Phase Transition (Figure 1, shared across papers)

```
Register Locations │ Max Solvable Vregs │ Feasibility Rate
──────────────────│───────────────────│─────────────────
3   (6502)        │ 19                │ 100%
8   (GameBoy)     │ 10                │ 93%
15  (Z80)         │ 8                 │ 81%
16  (Thumb)       │ 7                 │ ~70%
32  (RISC-V)      │ 6                 │ ~50%
```

Below the cliff: exhaustive tables dominate.
Above the cliff: heuristics needed.

## Reviewers Asked, We Answered

| Reviewer Concern | Our Answer |
|-----------------|-----------|
| "Shared IR bias" | 88.2% transfer across 8 frontends |
| "Only Z80" | Phase transition data for 3-32 locs |
| "Corpus too small" | 97.8% convergence = table is complete |
| "Cost model sensitivity" | GPU uses exact T-state counts from Z80 manual |
| "How does this help ARM?" | It doesn't — that's the point (above cliff) |
| "SDCC comparison" | Our table lookup is O(1); SDCC is heuristic |

## Peer Reviews

- **GPT-5.4:** Strongest on practical guidance. "Corpus-driven enumeration is more fundamental than GPU tables."
- **Claude Desktop:** Strongest on theory. "Island-split = treewidth decomposition (Bodlaender 1993)."
- **Gemini:** Strongest on framing. "Periodic table of Z80 constraints."

All three agree: publishable. Paper A first.
