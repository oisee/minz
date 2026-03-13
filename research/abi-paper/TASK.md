# Research: Per-Function ABI Optimization for Register-Constrained Architectures

## Goal

Prepare material for an academic publication (CC or LCTES venue) on Nanz's
per-function calling convention optimization — a direct continuation of
Krause 2013 (optimal intraprocedural RA) and Krause 2022 (empirical global ABI).

## What Makes This Novel

Krause 2013 solved optimal register allocation **within** a function, assuming
fixed ABI. Krause 2022 empirically searched thousands of global ABIs for SDCC.
Neither addresses:

> Jointly optimize register assignment and calling convention **per-function**
> across the call graph, minimizing total register-transfer cost.

Nanz does this. The `min_of: EQU minmax` result — a function that compiles to
zero bytes because its ABI matches the callee's — is a direct consequence.

## Task Breakdown

### Phase 1: Code Analysis ✅

Find and document the ABI optimizer in the Nanz/MinZ compiler:
**Artifact:** [FINDINGS.md](FINDINGS.md) — verified against source 2026-03-12

1. **Locate** the component that assigns registers to function parameters and
   return values. Key files likely in `pkg/mir2/`, `pkg/hir/`, or `pkg/pipeline/`.

2. **Trace** the algorithm: how does it decide which register gets which param?
   - Is it bottom-up on the call graph? Top-down? Iterative?
   - Does it consider call frequency or just topology?
   - How are recursive functions handled?
   - How are multi-call-site functions handled?

3. **Classify**: exact optimum, bounded approximation, or heuristic?

4. **Document** as pseudocode suitable for a paper.

### Phase 2: Formalization ✅

**Artifact:** [FORMALIZATION.md](FORMALIZATION.md)

1. **Problem statement**: PFCCO formalized with cost function, constraints, variables.
2. **Complexity analysis**: O(n) for bounded params on DAG call graphs.
3. **Optimality proof sketch**: Greedy DP on topo order is optimal for DAGs.
4. **PBQP/ILP connection**: PFCCO has PBQP structure; ILP formulation noted.

### Phase 3: Examples ✅

**Artifact:** [EXAMPLES.md](EXAMPLES.md)

4 examples with source, optimizer decisions, Z80 output, SDCC comparison, T-state analysis:
1. Call chain forwarding (transitive ABI propagation, 3 levels → EQU collapse)
2. abs_diff (ALU register class selection, 22T savings vs stack ABI)
3. min_of/minmax (zero-byte EQU alias — the crown jewel, ~120T savings)
4. Iterator DJNZ loop (ClassCounter for loop count, 4T×N savings)

**Note:** Multi-return syntax (`-> (T1, T2)`) does not parse in current compiler;
examples 1-2 compile, example 3 is hand-traced from the optimizer logic.

### Phase 4: Related Work Gap ✅

**Artifact:** [RELATED_WORK.md](RELATED_WORK.md)

Positioning vs 6 prior works with gap analysis table.
Key positioning paragraph ready for paper insertion.
Unique contribution = per-function × irregular register file × formal optimality × production.

### Phase 5: Paper Draft ✅

**Artifact:** [DRAFT.md](DRAFT.md) — v0.1 complete structure with all sections
**Maintainer notes:** [NOTE_TO_MAINTAINER.md](NOTE_TO_MAINTAINER.md) — prioritized action items

Structure (as implemented):
```
1. Introduction — contributions, motivation
2. Background — Z80 register file, fixed vs per-fn CC, Nanz pipeline, register classes
3. Problem Formulation — PFCCO definition, naturalClass, constraints
4. Algorithm — Greedy DP, three-layer system, complexity, optimality proof
5. Implementation — 455 LOC Go, cost table, pipeline position
6. Evaluation — 7 examples vs SDCC 4.2.0 with T-state analysis
7. Related Work — Krause 2013/2022, De Bus, LLVM IPRA, Wall, MSVC, PBQP
8. Future Work — shadow registers, recursion, PGO, benchmarks
9. Conclusion
```

## Key Questions To Answer

1. Is the ABI decision made at HIR→MIR2 lowering time, or during regalloc?
2. Are @z80_a / @z80_hl annotations the mechanism, or is there inference?
3. Does the compiler ever change its mind (iterative refinement)?
4. What happens at module boundaries / @extern functions?
5. Multi-return (HL, DE, B) — fixed convention or optimized?

## References

- Krause 2013: "Optimal Register Allocation in Polynomial Time" (CC 2013)
- Krause 2022: "Bytewise Register Allocation" (ACM TACO)
- Wall 1986: "Global Register Allocation at Link Time" (SIGPLAN)
- US5428793: Interprocedural register allocation patent
- SDCC source: sdcc.sourceforge.net (z80 backend, ralloc2.cc)
- Scholz & Eckstein 2002: "Register Allocation for Irregular Architectures" (PBQP)

### Local PDF copies

**`/mnt/safe/optimal-register-allocation-in-polynomial-time-1yfn9r69k7.pdf`**
- Krause 2013: "Optimal Register Allocation in Polynomial Time" (CC 2013) — **KEY REFERENCE**

**`/mnt/safe/z80-compiler/`:**
- `2112.01397v2.pdf` — Krause 2022: "Efficient Calling Conventions for Irregular Architectures" — **KEY REFERENCE**
- `2010.04633v2.pdf` — Krause & Lesser: "C for a tiny system" (SDCC on Padauk) — **relevant**
- `2011.10789v1.pdf` — Krause: "lospre in linear time" (redundancy elimination) — background
- `1310.2378v4.pdf` — Adler, Kolliopoulos, Krause et al.: Disjoint Paths (graph theory, not RA)
- `1011.2136v1.pdf` — Adler, Krause: tree-width lower bounds (graph theory)
- `1901.04325v1.pdf` — Adler, Krause: tree-width lower bounds (graph theory)
- `2010.04527v2.pdf` — Krause: "Constant-time connectivity tests" (unrelated)
