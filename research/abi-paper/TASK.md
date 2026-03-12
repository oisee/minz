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

### Phase 1: Code Analysis

Find and document the ABI optimizer in the Nanz/MinZ compiler:

1. **Locate** the component that assigns registers to function parameters and
   return values. Key files likely in `pkg/mir2/`, `pkg/hir/`, or `pkg/pipeline/`.

2. **Trace** the algorithm: how does it decide which register gets which param?
   - Is it bottom-up on the call graph? Top-down? Iterative?
   - Does it consider call frequency or just topology?
   - How are recursive functions handled?
   - How are multi-call-site functions handled?

3. **Classify**: exact optimum, bounded approximation, or heuristic?

4. **Document** as pseudocode suitable for a paper.

### Phase 2: Formalization

1. **Problem statement**: Given call graph G=(F,E), register file R,
   for each f∈F choose ABI(f) to minimize Σ cost(e) over all edges.

2. **Complexity analysis**: What is the time complexity? Is it polynomial
   for structured programs (like Krause 2013)?

3. **Correctness**: Does the algorithm preserve program semantics?
   (ABI is internal — external linkage functions keep fixed ABI.)

### Phase 3: Examples

Generate ≥3 end-to-end examples showing per-function ABI wins:

For each example, show:
- MinZ source code
- Nanz output (per-function ABI)
- What a fixed-ABI compiler would emit (HL=arg1, DE=arg2, stack for rest)
- Instruction count and T-state difference

Priority examples:
1. `min_of` / `minmax` — EQU alias (0 bytes, 0T)
2. `abs_diff` — SUB C / RET NC / NEG / RET (register choice eliminates MOVs)
3. Iterator chain — DJNZ with counter in B (ABI puts count in ClassCounter)
4. Multi-return — (HL, DE) layout matching caller's expectations

### Phase 4: Related Work Gap

One paragraph positioning vs:
- Krause 2013: optimal intraprocedural RA, fixed ABI (our lower layer)
- Krause 2022: empirical global ABI search (one ABI for all functions)
- MSVC /GL: per-function CC for internal linkage (x86, no formal theory)
- Wall 1986: interprocedural RA for RISC (many registers, regular file)

### Phase 5: Paper Draft

Structure:
```
1. Introduction
2. Background (Z80 register file, Krause 2013/2022)
3. Problem Formulation
4. Algorithm
5. Implementation (Nanz/MinZ)
6. Evaluation (vs SDCC, vs Hi-Tech C)
7. Related Work
8. Conclusion + Future Work
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
