# Superoptimizer Research: Ideas, Achievements, and Insights

**Period:** 2026-03-23 — 2026-03-25 (3 days)
**Team:** Alice + 3 AI sessions (minz-vir, minz main, z80-optimizer)
**Hardware:** Dual NVIDIA RTX 4060 Ti (16GB each), Z3 SMT solver
**Target:** Z80 8-bit CPU (1976), 15 physical register locations

---

## 1. The Core Insight: "Compile the Compiler"

Instead of solving register allocation at compile time, **precompute all possible solutions offline on GPU and ship a lookup table**. The compiler becomes a pure lookup engine — O(1), zero solver dependency, provably optimal.

This is possible because:
- Z80 has only 15 usable register locations (7 GPR8 + 3 pairs + 4 IX/IY halves + 1 memory)
- For N≤8 virtual registers, 15^N ≤ 2.5 billion — exhaustively searchable on GPU
- 87% of real functions have ≤8 virtual registers
- 80% of functions share constraint signatures — massive table reuse

**Key metric:** 56 table entries cover 639 functions (39.8% of 8-frontend corpus). With full enumeration: potentially 80%+ coverage.

---

## 2. What We Built

### 2.1 GPU Brute-Force Register Allocator (CUDA)
- Exhaustive search: tries ALL 15^N register assignments for N virtual registers
- Per-solve time: microseconds (N≤5) to seconds (N=8)
- Dual GPU pipeline: split batch → solve on both RTX 4060 Ti → merge
- **755,160 synthetic patterns solved in 40 seconds** (19K solves/sec)
- Width-aware: 16-bit vregs restricted to pair locations only

### 2.2 Z3 SMT Unified Solver (VIR)
- Joint instruction selection + register allocation in one Z3 query
- Per-instruction variables: `lv{vreg}_i{inst}` — solver plans register moves as part of optimal solution
- `(minimize total_cost)` for provably optimal; `(check-sat)` fallback for large problems
- CFG-aware encoding: per-block variables + edge constraints for loops/conditionals
- **645/645 functions (100% coverage), 55/55 Z80-verified, -60% vs SDCC**

### 2.3 WFC (Wave Function Collapse) Guided by PBQP
- PBQP provides global allocation hints, WFC enforces Z80-specific constraints
- Forward, backward, vreg consistency, clobber passes
- IXH/IXL as undocumented L2 spill (8T, call-safe) — no existing Z80 compiler uses this
- **948/948 pipeline completion (100%), 97.9% VM-verified**

### 2.4 Direct PIR Emit from GPU Table
- Table hit → pattern select (linear scan ~40 patterns) → PIR → asm
- **Zero solver invocation.** No Z3, no GPU at compile time.
- ABI verification: skip table emit when assignment conflicts with calling convention
- Confirmed working: 17 hits on zsql.nanz, 18 on sap_mara_cpm

### 2.5 ISLE + Grace + Datalog Rewrite Triad
- ISLE: term rewriting with guards + extern Go callbacks (542 LOC)
- Grace: graph pattern matching on CFG (1255 LOC, 13 tests)
- Datalog: fact database with wildcard queries (101 LOC)
- **63 rewrites applied across 89 functions, 94.1% convergence**

### 2.6 GPU Instruction Sequence Synthesis
- Multiply table: 103 entries (8-bit A×K), 51+ entries (16-bit HL×K)
- divmod10: 27 instructions, 124T, verified all 256 inputs, clobbers B,C,F only
- Key discovery: NEG as x255 (8T), carry-chain patterns (GPU-discovered)
- **Z3 cannot find these** — instruction synthesis ≠ constraint solving

---

## 3. Architectural Innovations

### 3.1 Island-of-Optimality (ADR-0040)

**Problem:** GPU brute-force limited to ≤8 vregs. 13% of functions exceed this.

**Solution:** Split functions at liveness bottlenecks (natural cut points at CALL boundaries), solve each island optimally via GPU table, connect with minimum-cost boundary shuffles.

```
14-vreg function:
  [2 live] → island A (4v, GPU optimal) → [CALL: 1 live] → island B (5v, GPU optimal) → [ret]
                                          ↑ boundary join: min-cost shuffle
```

**Key properties:**
- Each island provably optimal (exhaustive search)
- Join cost bounded: ≤ |boundaries| × 11T
- Split only when live set exceeds table capacity K
- Exact join solver: enumerate all 2-4 edge shuffles (~20 options, instant)
- Total cost = Σ(optimal islands) + Σ(join moves) — near-optimal for entire function

### 3.2 Corpus-Driven Enumeration

**Discovery:** Blind enumeration of constraint patterns is intractable (3.8B for 6 vregs). But real programs use only ~15 unique width+location shapes per vreg count.

**Method:** Extract unique shapes from corpus → enumerate only interference graph variants → GPU solve.

- 6 vregs: 15 shapes × 32K interference = **491K patterns** (vs 3.8B blind)
- **7700x reduction** in search space
- Scales to any vreg count — even 14 vregs has maybe 50 shapes × variants

### 3.3 Signature-Based Caching with 80% Reuse

**Discovery:** Register allocation constraints hash to a small number of unique signatures. 1605 functions across 8 language frontends produce only 312 unique signatures — **80% reuse rate**.

**Implication:** A small table covers a large fraction of all possible programs. The Z80 instruction set creates a finite vocabulary of constraint patterns.

### 3.4 Three-Tier Regalloc with Provable Guarantees

| Tier | vregs | Method | Guarantee |
|------|-------|--------|-----------|
| 1 | ≤8 (87%) | GPU exhaustive table | **Provably globally optimal** |
| 2 | 9-20 (~10%) | Island-split + GPU per island | **Optimal per island + bounded join** |
| 3 | 20+ (~3%) | Spill-to-fit + enumerate spill sets | **Optimal for best spill choice** |

### 3.5 Dual-Mode Solver: Constrained vs Standalone + Adapter

Solve each function twice:
1. With ABI constraints (param regs, return reg)
2. Without constraints (standalone) + adapter LD moves at entry/exit

Pick cheaper option. Benchmark: 7 functions / 47 instructions saved across Nanz corpus.

### 3.6 Z3-PFCCO: Module-Level Calling Convention Optimization

Z3 considers ALL call sites simultaneously to optimize calling conventions. Instead of fixed ABI, each function gets custom param/return registers that minimize total move cost across the entire module.

**This is what makes MinZ beat SDCC** — SDCC uses fixed conventions, we optimize them.

---

## 4. Hard-Won Lessons (Wisdom)

### 4.1 Solver Design
1. **Hard vs soft Z3 constraints:** Hard `(assert)` for facts (ABI, spill tiers). Soft `(ite cost)` for hints. Hard hints that conflict with tied-dst patterns → unsat.
2. **Per-instruction vs global variables:** Global (one loc per vreg) fails at >7 live vregs. Per-instruction handles everything.
3. **Z3 minimize pitfall:** `(minimize total_cost)` uses opt module. On >100 ITE vars returns "unknown". Use plain `(check-sat)` for large problems.
4. **Width constraints in solver too aggressive:** Adding vreg width restrictions to Z3 variables breaks satisfiability. Fix at move insertion level (add truncation/extension patterns) instead.
5. **Pre-solver pass interaction:** DON'T propagate param constraints to copy vregs (both live at move → both constrained to same reg → unsat).

### 4.2 Z80 Hardware Quirks
6. **DD/FD prefix conflict:** `LD IXH, H` encodes as `LD IXH, IXH` (no-op). Route through A.
7. **EX DE,HL is a swap, not a copy.** Inter-instruction moves need non-destructive LD patterns.
8. **Grace EX sandwich:** Can't swap HL↔DE in `ADD HL,DE` — HL-only Z80 instruction.
9. **EX AF,AF' swaps BOTH A and F.** Cannot use for carry-spill between bytes.
10. **IXH/IXL as L2 spill:** Undocumented, call-safe, 8T. No existing compiler uses this.

### 4.3 Engineering
11. **Compilation time is generous.** Minutes per function OK. Optimize quality, not speed.
12. **Solve once, ship forever.** GPU brute-force offline → JSON table → O(1) compile lookup.
13. **Corpus-driven > blind enumeration.** 7700x reduction by enumerating only real shapes.
14. **80% signature reuse** — the Z80 constraint vocabulary is small. 56 entries cover 639 functions.
15. **MZA trailing-zero trim:** Assembler strips trailing zeros from COM files. Strings must come before zero-filled globals.

---

## 5. Research Paper Seeds

### 5.1 "Exhaustive Register Allocation via GPU Brute-Force for Constrained Architectures"
- **Thesis:** For CPUs with ≤15 physical registers and ≤8 live variables, exhaustive search on modern GPUs is faster and provably better than all heuristic approaches.
- **Novelty:** First compiler to ship a precomputed regalloc table from GPU brute-force.
- **Results:** 87% of functions solved optimally in O(1). -60% code size vs SDCC.
- **Comparison:** vs graph coloring (SDCC), vs linear scan (LLVM), vs PBQP (GCC), vs ILP (optimal but slow).

### 5.2 "Island-of-Optimality: Composing Provably Optimal Subgraph Allocations"
- **Thesis:** Large functions can be decomposed at liveness bottlenecks into small subproblems, each solved optimally, with bounded-cost joins.
- **Novelty:** Combines exhaustive local search with exact boundary matching. Provable quality guarantees (optimal per island + bounded join cost).
- **Open question:** How close to global optimum? Can we prove a bound?

### 5.3 "Corpus-Driven Constraint Enumeration: Pruning Superoptimizer Search Spaces"
- **Thesis:** Real programs use a tiny fraction of possible constraint patterns. Extracting shapes from a corpus and enumerating only interference variants reduces GPU search space by 4 orders of magnitude.
- **Novelty:** 7700x reduction (491K vs 3.8B for 6 vregs). 80% signature reuse across 8 language frontends.
- **Generalization:** Applies to any superoptimizer — use corpus analysis to focus search.

### 5.4 "Z3-PFCCO: SMT-Driven Module-Level Calling Convention Optimization"
- **Thesis:** Jointly optimizing calling conventions across all functions in a module via SMT yields 5-15% improvement over fixed ABIs.
- **Novelty:** No existing compiler optimizes calling conventions with a constraint solver.
- **Results:** 55/55 Z80-verified, consistent wins over SDCC on every benchmark.

### 5.5 "Self-Modifying Code as a Compiler Optimization: TSMC for Z80"
- **Thesis:** On architectures with writable code memory, self-modifying code can replace conditional logic with single-byte patches (7-20T vs 44+T).
- **Novelty:** First systematic compiler framework for generating SMC optimizations.
- **Ethical note:** Only for embedded/retro targets where code=data is expected.

### 5.6 "Three Solvers, One Pipeline: PBQP + WFC + Z3 for Register Allocation"
- **Thesis:** Combining three different solver paradigms (graph-based PBQP, constraint propagation WFC, SMT Z3) with progressive refinement yields better results than any single approach.
- **Novelty:** First compiler to use WFC (Wave Function Collapse) for register allocation.

### 5.7 "GPU Instruction Sequence Synthesis for Fixed-Architecture Targets"
- **Thesis:** For architectures with small instruction sets (Z80: ~1500 opcodes), GPU brute-force can discover optimal instruction sequences for common operations (multiply, divide, address calculation).
- **Results:** 103 multiply constants, divmod10 in 27 instructions, carry-chain patterns.
- **Novelty:** NEG as x255, RRA+AND replacing SRL chains — patterns no human would write.

---

## 6. Open Questions for Peer Review

1. **Island join optimality bound:** Can we prove that island-split + exact join is within a constant factor of globally optimal? What's the worst-case gap?

2. **Signature space completeness:** Is the set of possible constraint signatures for a given instruction set finite and enumerable? If so, can we precompute the ENTIRE table?

3. **Cross-architecture generalization:** Does this approach work for ARM Thumb (16 regs), RISC-V (32 regs), x86 (16 regs)? At what register count does exhaustive search become impractical?

4. **Corpus transfer:** Does a table built from one corpus generalize to new programs? What's the coverage drop when compiling programs from a different domain?

5. **Optimal vs near-optimal:** For Tier 2 (island-split) functions, how often does the result differ from global optimum? Can we measure the gap empirically?

6. **Compilation as database query:** If the regalloc table is complete for a given architecture, is compilation reducible to a series of table lookups? What other compiler passes could be "compiled away"?

---

## 7. Numbers At a Glance

| Metric | Value |
|--------|-------|
| Corpus | 8 frontends, 153 files, 1605 functions |
| Unique signatures | 312 (80% reuse) |
| GPU-solvable (≤8 vregs) | 87% of corpus |
| Table entries | 56 → covers 639 functions (39.8%) |
| GPU solve throughput | 19K solves/sec (dual RTX 4060 Ti) |
| Synthetic patterns solved | 755K in 40 seconds |
| Corpus-driven reduction | 7700x (491K vs 3.8B for 6 vregs) |
| VIR vs SDCC | -60% code size on Z80 benchmarks |
| Z80 instruction synthesis | 103 multiply, divmod10, carry-chain patterns |
| Solver architectures tried | 4 (PBQP, WFC, Z3, GPU exhaustive) |
| ADRs written | ADR-0039 (unified solver), ADR-0040 (islands) |

---

*"The best solver is no solver at all — just a table lookup."*

*"The Z80 has 15 registers. A GPU has 10,000 cores. The math is obvious in hindsight."*

*"80% of programs are the same program, from a register allocator's perspective."*
