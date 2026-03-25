# ADR-0040: Island-of-Optimality Register Allocation

**Status:** Accepted
**Date:** 2026-03-25
**Deciders:** Alice, VIR session, z80-optimizer session, main session

## Context

The VIR backend uses Z3 SMT solver for joint instruction selection + register allocation. Z3 produces provably optimal results for small functions but struggles with >8 virtual registers (minutes, or "unknown" from `(minimize)`). The Z80 has only 15 usable physical locations (7 GPR8 + 3 pairs + 4 IX/IY halves + 1 memory spill).

GPU brute-force (CUDA) can exhaustively search all 15^N assignments for N≤8 vregs in milliseconds. For N>8, the search space exceeds practical limits (15^9 = 38B).

87% of functions in the 8-frontend corpus have ≤8 vregs. The remaining 13% (206/1605 functions) need a strategy beyond brute-force.

## Decision

**Three-tier register allocation with provable optimality guarantees:**

### Tier 1: GPU Exhaustive Table (≤8 vregs, 87% of functions)

Precomputed lookup table from CUDA brute-force. O(1) at compile time.

- **Guarantee:** Provably globally optimal (all assignments enumerated)
- **Method:** `ComputeSignature(ops)` → table lookup → direct PIR emit
- **No solver invocation.** Zero Z3, zero GPU at compile time.
- **Table growth:** Compile corpus with `VIR_DUMP_GPU_BATCH=1` → GPU solve → merge into table

### Tier 2: Island-Split + GPU (9-20 vregs, ~10% of functions)

Split function into islands at liveness bottlenecks. Solve each island via Tier 1.

- **Guarantee:** Each island provably optimal. Total cost = Σ(optimal islands) + join cost.
- **Join cost bounded:** ≤ |boundaries| × 11T (worst case PUSH/POP per boundary vreg)

#### Split Algorithm

1. Compute liveness at each program point
2. Find **cut points** where live set ≤ K (K = table max vregs, currently 8)
3. Natural cut points: after CALL instructions (clobber reduces live set to 1-3 caller-saved vregs)
4. **Only split when live set exceeds K.** If live set stays ≤K across a CALL (e.g., values in IXH/IXL), keep in one island. Minimizes join moves.

#### Join Algorithm (Exact)

At each boundary between islands A and B:
1. Island A assigns vregs to physical locations: `{v1→DE, v3→IXH}`
2. Island B needs: `{v1→HL, v3→B}`
3. **Enumerate all shuffle sequences** (boundary graph has 2-4 edges → ~20 options max)
4. Pick minimum-cost shuffle

Cost matrix for boundary moves:

| Move type | Cost | When |
|-----------|------|------|
| LD r,r' | 4T, 1B | Single register transfer |
| EX DE,HL | 4T, 1B | Swap DE↔HL (two vregs simultaneously) |
| PUSH rr / POP rr' | 11T, 2B | No free intermediate register |
| LD IXH/IXL, r | 8T, 2B | Call-safe storage across boundary |
| TSMC tunnel | 20T, 3B | Patch immediate in next island (constants only) |

**Exact enumeration is instant** for 2-4 edge graphs. Greedy would miss EX DE,HL swaps that save 7T.

### Tier 3: Spill-to-Fit + GPU (20+ vregs, ~3% of functions)

Choose spill candidates to reduce live set to ≤8, then Tier 1.

- **Guarantee:** Optimal allocation for the chosen spill set. Enumerate all C(N, N-8) spill sets → GPU each → pick best.
- **Example:** 13 vregs → C(13,5) = 1287 spill sets × GPU solve (ms each) = seconds total
- **Spill cost:** Known and fixed per tier (IXH=8T, PUSH/POP=11T, memory=26T)
- **Strictly better than heuristic spill selection** — all options explored

### Z3 Fallback

Z3 remains as fallback for:
- Table misses (new signature not yet in table)
- Functions where island-split produces no valid cut points
- Verification mode (`--verify-gpu` compares table result with Z3)

## Architecture

```
CodegenFunc(f):
  ops = bridge(f)
  sig = ComputeSignature(ops)

  if table.Lookup(sig):           # Tier 1: O(1)
    return directPIREmit(assignment)

  if nVregs > K:                  # Tier 2: Island-Split
    islands = splitAtCutPoints(ops, K)
    for island in islands:
      island.assignment = table.Lookup(island.sig)  # or GPU online
    joins = solveJoins(islands)   # exact, instant
    return emitIslands(islands, joins)

  return Z3Solve(ops)             # Fallback
```

## Comparison with SDCC

| Aspect | SDCC | MinZ Island Architecture |
|--------|------|--------------------------|
| Regalloc method | Graph coloring (heuristic) | GPU exhaustive (optimal) |
| Scope | Global (whole function) | Per-island (each optimal) + exact join |
| Spill selection | Heuristic (cost estimate) | Enumerate all options (exact) |
| Guarantee | Correct, quality unknown | Provably optimal per island |
| Inter-block | Graph coloring across CFG | Min-cost matching at boundaries |
| Compile time | Fast (ms) | O(1) table lookup (ns) |

## Consequences

### Positive
- 87% of functions get provably optimal allocation with zero solver
- Remaining functions get near-optimal via island decomposition
- Table grows organically from corpus compilation
- No external tool dependency at compile time (table ships with compiler)
- Compile speed limited by codegen, not solving

### Negative
- GPU hardware needed for table generation (offline only, not at compile time)
- Island join moves add 8-16T overhead vs theoretical global optimum
- Table size grows with corpus diversity (~10KB per 100 entries)
- Signature system must stay in sync between compiler and GPU enumerator

### Risks
- Z80's constrained register file may create functions with no valid cut points (all instructions have high live sets). Mitigation: Tier 3 spill-to-fit.
- Signature hash collisions (SHA256-based, effectively impossible)

## Implementation Plan

1. ✅ GPU exhaustive table + direct PIR emit (done, 46 entries, 598 functions covered)
2. ✅ Width-aware GPU kernel (done, z80-optimizer 2089c42)
3. ⬜ Realistic enumerator integration (z80-optimizer running 156K real loc sets)
4. ⬜ LivenessIslandSplit pass
5. ⬜ ExactJoinSolver (boundary min-cost matching)
6. ⬜ Spill-to-fit for Tier 3
7. ⬜ Table auto-growth (solve on miss, merge result)

## References

- [Report #115: Exhaustive GPU Regalloc Table](../../reports/2026-03-25-115-Exhaustive-GPU-Regalloc-Table.md)
- [ADR-0039: Unified VIR Solver](0039-unified-vir-solver.md)
- [Report #111: GPU Precomputed Regalloc Table](../../reports/2026-03-24-111-GPU-Precomputed-Regalloc-Table.md)
