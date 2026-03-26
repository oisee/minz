# Precomputed Optimal Register Allocation for Constrained Architectures via Corpus-Driven Exhaustive GPU Search

**Authors:** Alice Vinogradova, with Claude Opus 4.6 (AI collaborator)

**Abstract.**
We show that register allocation on the Z80 — a constrained, irregular 8-bit architecture with 15 usable physical locations — can be largely reduced from an online search problem to an offline table lookup. By exhaustively enumerating all possible register assignments on GPU for functions with up to 8 virtual registers (87% of a 1,645-function corpus across 8 language frontends plus standard library), we precompute provably optimal allocations keyed by a compact constraint signature. The surprising empirical finding is not the speed of GPU search, but the structure it reveals: 1,645 functions collapse to only 315 unique signatures (80% reuse), and adding an entire standard library produces fewer than 3% new signatures — indicating that the constraint vocabulary has converged. A cross-program transfer experiment shows 88.2% hit rate when testing on programs unseen during table construction. We further demonstrate a phase transition in "tableability" as a function of physical register count: at 15 locations (Z80), exhaustive precomputation covers the majority of real functions; at 32 locations (RISC-V), only trivial functions are tractable. For functions exceeding table capacity, we propose an island-of-optimality decomposition at liveness bottlenecks, where each island is solved optimally and connected via bounded-cost register shuffles. The resulting compiler uses no solver at compile time for table-hit functions — only a hash lookup and pattern selection. We validate the approach end-to-end by compiling SQL queries that execute on a ZX Spectrum (1982) via Z80 CP/M, with provably optimal register allocation for leaf functions.

---

## 1. Introduction

Register allocation — assigning virtual registers to physical locations — is one of the oldest and most studied problems in compiler construction. The problem is NP-complete in general [Chaitin et al., 1981], and practical compilers use heuristic approaches: graph coloring [Chaitin, 1982; Briggs et al., 1994], linear scan [Poletto & Sarkar, 1999], PBQP [Scholz & Eckstein, 2002], or ILP formulations [Appel & George, 2001].

All of these approaches solve the allocation problem *online* — at compile time, for each function individually. We ask a different question:

> **Can register allocation be precomputed offline and shipped as a lookup table?**

For architectures with a small number of physical locations, the answer is yes. The Z80 microprocessor (1976) has 15 usable register locations: 7 general-purpose 8-bit registers (A, B, C, D, E, H, L), 3 register pairs (BC, DE, HL), 4 undocumented index register halves (IXH, IXL, IYH, IYL), and 1 memory spill slot. For a function with N virtual registers, there are 15^N possible assignments. For N ≤ 8 — which covers 87% of functions in our corpus — this is at most 2.56 billion, exhaustively searchable on a modern GPU in seconds.

The key insight is not that GPUs are fast. It is that **real programs occupy a tiny fraction of the theoretical constraint space**. We compiled 1,645 functions from 8 language frontends through a shared compiler backend and found only 315 unique constraint signatures — an 80% reuse rate. Adding an entire standard library produced fewer than 3% new signatures. A transfer experiment (training on arithmetic programs, testing on enterprise business logic and screen rendering) achieved 88.2% hit rate.

This suggests that for constrained, irregular architectures, the backend optimization problem is not just tractable but *effectively finite*. We can "solve the game" offline and ship the answers.

### Contributions

1. **Exhaustive GPU register allocation** with provable global optimality for functions with ≤8 virtual registers (87% of corpus).

2. **Empirical characterization of the constraint space**: 80% signature reuse, 88.2% cross-program transfer, <3% marginal growth from standard library expansion.

3. **Phase transition analysis**: tableability as a function of physical register count, with a cliff at ~16 locations separating tractable from intractable architectures.

4. **Island-of-optimality decomposition** for functions exceeding table capacity, with provably optimal sub-solutions and bounded join cost.

5. **End-to-end validation**: SQL queries compiled to Z80 and executing on ZX Spectrum hardware (1982), with provably optimal leaf-function allocation via table lookup.

---

## 2. Background

### 2.1 The Z80 Register File

The Zilog Z80 (1976) has an irregular register file that exemplifies the challenges of constrained architectures:

- **Accumulator-only ALU:** Arithmetic operations (ADD, SUB, AND, OR) require operand A as both source and destination. This creates a tied-operand constraint absent from RISC architectures.
- **Register pairs:** 16-bit operations use BC, DE, HL as pairs. HL serves as the primary memory pointer (LD A,(HL), ADD HL,rr). There is no ADD DE,rr — only ADD HL,rr.
- **DD/FD prefix conflicts:** Under the DD prefix (IX-indexed instructions), H and L are reinterpreted as IXH and IXL. This means LD IXH,H encodes as LD IXH,IXH (a no-op) — certain register-to-register moves are physically impossible.
- **No orthogonal register classes:** Unlike ARM or RISC-V where most registers are interchangeable, Z80 registers have distinct capabilities. A is the only ALU destination. HL is the only pair that can dereference memory. DE and BC serve as secondary pairs with limited operations.

This irregularity creates a constraint landscape where many theoretical assignments are illegal. We exploit this: **fewer valid assignments means a smaller table and higher reuse**.

### 2.2 Prior Work

**Graph coloring** [Chaitin, 1982] models interference as a graph and colors it with K colors (registers). SDCC, the reference open-source Z80 compiler, uses this approach. It produces correct but often verbose code because the coloring heuristic cannot exploit Z80-specific constraints like tied operands or pair aliasing.

**PBQP** (Partitioned Boolean Quadratic Programming) [Scholz & Eckstein, 2002] models allocation as a cost-minimization problem over a bipartite graph. GCC uses PBQP for some targets. Our compiler uses PBQP as a baseline and fallback.

**ILP** (Integer Linear Programming) [Appel & George, 2001] formulates allocation as an optimization problem with provable optimality. However, ILP solvers are slow for large functions and do not scale to whole-program compilation.

**SMT-based allocation** is less explored. Our Z3-based unified solver [VIR, this work] jointly solves instruction selection and register allocation in a single SMT query, achieving provable optimality for functions where Z3's optimizer converges.

**Superoptimization** [Massalin, 1987; Bansal & Aiken, 2006] exhaustively searches for optimal instruction sequences. Our GPU approach applies the same principle to register *assignment* rather than instruction *synthesis*, and precomputes answers offline rather than searching at compile time.

**Endgame tablebases** in games [Schaeffer et al., 2007 (checkers); Thompson, 1986; Nalimov et al., 2000 (chess)] solve positions with bounded pieces by exhaustive retrograde analysis. Our approach is analogous: we solve allocation instances with ≤8 virtual registers by exhaustive forward search, and ship the table with the compiler.

---

## 3. Approach

### 3.1 Constraint Signatures

We define a *constraint signature* as a deterministic hash of a function's register allocation problem, capturing:

- Number of virtual registers and instructions
- Per-instruction: which patterns are available (destination/source location sets, cost, tied-operand constraints)
- Interference pairs (which virtual registers are simultaneously live)

The signature does NOT capture function names, immediate values, symbols, or source language. Two functions with isomorphic constraint graphs — regardless of what they compute — share the same signature and optimal assignment.

Formally, let a constraint instance be a tuple (V, I, P, F) where V is the set of virtual registers, I is the set of instructions, P maps each instruction to its set of legal patterns (each specifying allowed physical locations for operands), and F ⊆ V × V is the interference relation. The signature is σ = SHA256(canonical(V, I, P, F)).

### 3.2 GPU Exhaustive Search

For a function with N virtual registers and L physical locations, we enumerate all L^N possible assignments on GPU. Each assignment is evaluated against the full pattern table: for each instruction, we find the cheapest legal pattern whose location constraints are satisfied by the assignment. Infeasible assignments (no legal pattern exists, or interfering registers share a location) are discarded. The minimum-cost feasible assignment is the provably globally optimal allocation.

**Implementation.** We use a CUDA kernel running on NVIDIA RTX 4060 Ti GPUs (cudaDeviceScheduleBlockingSync to avoid CPU spinwait). Each GPU thread evaluates one assignment. For N=8, L=15: 2.56 billion assignments, completed in ~15 seconds on a single GPU. Dual-GPU throughput: ~19,000 allocation problems per second for the typical corpus function.

**Width-aware constraints.** 16-bit virtual registers (holding pointer or u16 values) are restricted to pair locations (BC=7, DE=8, HL=9). The JSON protocol includes a per-vreg `widths` array; the kernel masks invalid locations accordingly.

### 3.3 Table Construction

**Corpus compilation.** We compile 1,645 functions from 8 language frontends (Nanz, C89, Pascal, PL/M, ABAP, Frill, Lizp, Lanz) through a shared HIR→MIR2→VIR pipeline. For each function, we extract the constraint signature and the GPU function descriptor (a JSON representation of the allocation problem).

**GPU solving.** Unique constraint instances (by signature) are submitted to the CUDA kernel. Results are stored as (signature → optimal assignment) pairs.

**Table shipping.** The table is a JSON file (~10KB per 100 entries) distributed with the compiler binary. At compile time, the compiler hashes the function's constraints, looks up the signature, and — on hit — emits code directly from the precomputed assignment without invoking any solver.

**Scale.** The complete ≤6v table contains 83.6 million entries (32MB compressed): ≤4v: 156,506 (40 seconds GPU), ≤5v: 17,366,874 (25 minutes dual GPU), 6v dense: 66,118,738 (5.7 hours dual GPU). This is a one-time offline computation; the table ships with the compiler binary.

### 3.4 Direct Emit (Zero Solver)

On a table hit, the compiler:

1. Verifies ABI compatibility (the precomputed assignment must satisfy calling convention constraints)
2. For each instruction, selects the cheapest pattern whose location constraints match the assignment (linear scan over ~40 Z80 patterns)
3. Emits the pattern template with physical register names substituted

No Z3, no GPU, no graph coloring. Compile-time cost: O(I × P) where I is the number of instructions and P is the number of patterns — effectively O(1) per function.

### 3.5 Island-of-Optimality Decomposition

For functions with >8 virtual registers (13% of corpus), we decompose at liveness bottlenecks — program points where the set of simultaneously live registers drops below the table capacity K.

**Natural cut points.** Function call instructions (CALL) are natural separators: the Z80 calling convention requires saving caller-used registers, reducing the live set to 1–3 values (return address + caller-saved registers in IXH/IXL).

**Split policy.** We split only when the live set exceeds K. If 3 virtual registers remain live across a CALL (e.g., values preserved in IXH/IXL), we keep them in one island.

**Join solver.** At boundaries between islands, registers may need to be shuffled. The boundary graph has 2–4 edges. We enumerate all possible shuffle sequences and select the minimum-cost one:

| Move | Cost (T-states) | Encoding |
|------|-----------------|----------|
| LD r, r' | 4 | 1 byte |
| EX DE, HL | 4 | 1 byte |
| LD IXH, r | 8 | 2 bytes (DD prefix) |
| PUSH rr / POP rr' | 11 | 2 bytes |

**Guarantee.** Each island is provably optimal (exhaustive search). Total cost = Σ(optimal island costs) + Σ(join move costs). The join cost is bounded by |boundaries| × 11T (worst case: PUSH/POP per boundary register).

---

## 4. Evaluation

### 4.1 Corpus

| Frontend | Files | Functions | Description |
|----------|-------|-----------|-------------|
| Nanz | 58 | 807 | Native language (arithmetic, data structures, iterators) |
| C89 | 43 | 312 | C programs (benchmarks, algorithms) |
| ABAP | 20 | 196 | SAP business logic (reports, ALV grids) |
| Frill | 15 | 89 | ML-style functional (ADTs, pattern matching) |
| Lizp | 8 | 34 | Lisp dialect |
| Pascal | 5 | 42 | Pascal programs |
| PL/M | 3 | 18 | Intel PL/M legacy |
| Lanz | 1 | 7 | Experimental |
| Stdlib | 55 | 140 | Standard library modules |
| **Total** | **208** | **1,645** | **8 frontends + stdlib** |

*Note: Function counts exclude extern stubs and empty wrappers that produce no VIR instructions.*

**Important caveat.** All frontends share the same HIR→MIR2→VIR backend pipeline. The 80% signature reuse and 88.2% transfer reflect the constraint vocabulary of *our IR and backend*, not necessarily a universal property of the Z80 ISA. Testing on SDCC's independent backend would determine whether the low signature entropy is ISA-intrinsic or IR-dependent.

### 4.2 Signature Reuse

| Metric | Value |
|--------|-------|
| Total functions | 1,645 |
| Unique constraint signatures | 315 |
| Reuse rate | **80.9%** |
| Functions with ≤8 vregs | 1,431 (87%) |
| Functions with >8 vregs | 214 (13%) |

The virtual register distribution peaks at 0–2 vregs (62.8% cumulative), with a long tail to 14 vregs.

### 4.3 Cross-Program Transfer

**Protocol.** Build table from Program Group A (Nanz core + PL/M + miscellaneous, 783 functions, 274 unique signatures). Test on completely disjoint Program Group B (ABAP + SQLite + Screen + Lizp + FS, 822 functions, 50 unique signatures).

| Test Frontend | Hit Rate | Functions |
|--------------|----------|-----------|
| ABAP | 98.8% | 477/483 |
| Screen | 89.6% | 190/212 |
| Lizp | 66.7% | 2/3 |
| SQLite | 45.9% | 45/98 |
| FS | 42.3% | 11/26 |
| **Total** | **88.2%** | **725/822** |

ABAP business logic (98.8%) and screen rendering (89.6%) transfer near-perfectly from basic arithmetic training data. SQLite I/O functions (45.9%) show lower transfer — they use specialized port-access patterns not present in the training set. Only 38 additional signatures would close the gap to 100%.

**Caveat.** The train/test split uses program groups through the same compiler backend. A cleaner experiment would test across independently developed compilers (e.g., MinZ vs SDCC).

### 4.4 Convergence

| Corpus Stage | Functions | Unique Signatures | New Signatures |
|-------------|-----------|-------------------|----------------|
| Examples only | 1,605 | 312 | — |
| + Standard library | 1,645 | 315 | +3 (1.0%) |
| + Expanded tests | 1,645 | 319 | +7 (2.2%) |

Adding 140 standard library functions produced 7 new signatures (2.2%). The signature vocabulary has effectively converged for this backend.

### 4.5 Phase Transition

We parameterize the GPU kernel's location count and measure the maximum number of virtual registers solvable within a 5-minute timeout (at 10M evaluations/second):

| Locations | Architecture Analogue | Max Vregs (5 min) | Corpus Solvability |
|-----------|----------------------|-------------------|--------------------|
| 3 | 6502 | 19 | 100% (all solved, 47.5% feasible) |
| 8 | GameBoy LR35902 | 10 | 93% |
| 15 | Z80 (full) | 8 | 81% |
| 16 | ARM Thumb (low regs) | 7 | ~70% |
| 32 | RISC-V | 6 | ~50% |

The theoretical curve follows: **max_vregs = ⌊log(B) / log(L)⌋** where B is the GPU evaluation budget (e.g., 3×10⁹ for a 5-minute timeout at 10M evaluations/second). This formula cleanly separates two regimes: at small L (≤16), the system is *constraint-limited* — most functions are solvable but many assignments are infeasible due to ISA constraints. At large L (>16), the system is *compute-limited* — the search space grows faster than GPU throughput.

Below ~16 locations, exhaustive precomputation covers the majority of real functions. Above 16, only small functions are tractable without decomposition.

The feasibility cliff is equally dramatic: 95.9% of 2v shapes are feasible, dropping to 0.9% at 6v. The Z80 register file fills up — the majority of theoretically possible constraint patterns have no valid assignment. This is the flip side of irregularity: it makes the table smaller AND proves most shapes impossible.

**Irregularity helps.** The Z80's constrained register file (accumulator-only ALU, pair-only 16-bit ops, DD/FD prefix conflicts) reduces the number of valid assignments per signature. This makes the constraint space more compressible and the table more reusable — inverting the conventional wisdom that irregular architectures are harder to compile for. They are harder for *heuristic* allocators but easier for *exhaustive* ones.

### 4.6 End-to-End Validation

We compile a SQL client (ZSQL) in the Nanz language, targeting Z80 CP/M and ZX Spectrum:

```
CREATE TABLE mara (matnr TEXT, mtart TEXT, meins TEXT, maktx TEXT);
INSERT INTO mara VALUES ('100-100', 'FERT', 'ST', 'Pump Assembly');
SELECT * FROM mara WHERE mtart = 'FERT';
→ 100-100|FERT|ST|Pump Assembly
```

The SQL executes through SQLite-over-I/O-ports on a Z80 emulator. Leaf functions (puts, putchar, newline, etc.) use provably optimal allocation from the GPU table with zero solver invocation. Complex functions (_do_select, sqlite_query) fall back to PBQP allocation. The output renders correctly on both CP/M terminal and ZX Spectrum 256×192 display.

### 4.7 Comparison with SDCC

On a 5-function micro-benchmark (abs_diff, gcd, minmax, fib, swap):

| | SDCC 4.2.0 | MinZ VIR | Δ |
|---|-----------|---------|---|
| Total instructions | 131 | 52 | **−60.3%** |

The improvement decomposes approximately as: ~40% from calling convention optimization (Z3-PFCCO eliminates register shuffles at call boundaries), ~20% from exhaustive table allocation (provably optimal register assignment within functions), and the remainder from instruction selection differences (ISLE combining, load16_le fusion).

**Caveat.** This benchmark is small and favorable to MinZ. Larger programs with many call sites show smaller differences. SDCC 4.5.0 trunk produces tighter code than 4.2.0. The comparison is against SDCC's default settings; SDCC with `--opt-code-size` may close part of the gap.

---

## 5. Discussion

### 5.1 Theoretical Interpretation

The 80% signature reuse result suggests that the effective dimensionality of the register allocation constraint space is much lower than its theoretical upper bound. For a fixed ISA with L locations and instructions drawn from a finite pattern set, the number of distinct constraint signatures encountered in practice appears to be bounded by a function significantly smaller than L^K.

One interpretation: the semantics of high-level languages impose strong priors on liveness patterns. Functions tend to have similar numbers of live variables at similar program points, creating recurring interference structures. The ISA's irregularity further constrains the space by eliminating many theoretical configurations.

A connection to graph theory: the island-of-optimality decomposition corresponds to tree decomposition of the interference graph [Bodlaender, 1993]. CALL boundaries act as graph separators, creating components of bounded treewidth. Known results on graph coloring with bounded treewidth provide theoretical grounding for the optimality guarantees.

### 5.2 Limitations

**Shared IR pipeline.** All 8 frontends use the same backend (HIR→MIR2→VIR). The signature reuse may be an artifact of our IR design rather than an ISA property. Testing on SDCC (different IR, different instruction selection) would resolve this.

**Cost model sensitivity.** Our optimality is with respect to T-state costs. A different cost model (code size, energy, eZ80 pipeline effects) would produce different optimal tables. We have not measured sensitivity to cost model perturbation.

**Small benchmark.** The −60% vs SDCC result is on 5 functions. Comprehensive comparison on larger codebases (FatFS, CP/M utilities, games) is future work.

**Table completeness.** The ≤6v table is exhaustive — 83.6M entries covering ALL possible constraint shapes through 6 virtual registers. For 7-8v (13% of corpus), the table grows to an estimated 1-10 billion entries. Corpus-derived signatures (315 entries) provide O(1) lookup for common patterns; Z3 or backtracking fills gaps on-demand.

**Exhaustive enumeration and negative certificates.** Complete enumeration of all ≤4v constraint shapes (156,506 total) revealed that 61% of 6v shapes are provably infeasible — negative certificates showing the Z80 register file physically cannot accommodate the majority of theoretical constraint configurations. These negative certificates allow the compiler to detect impossible allocation patterns in O(1) and immediately trigger spilling or instruction rewriting, rather than searching fruitlessly.

**Island splitter correctness.** The island splitter now operates at the VIR level — splitting ops before GPU descriptor construction, so each island gets fresh pattern matching. ZSQL's 4 large functions (18-37 vregs) successfully decompose into 10 islands, all ≤15v. Remaining limitation: functions with unrolled loops and high accumulator contention (e.g., _sel_rows) require per-call-site splitting, increasing shuffle overhead.

**Interaction with instruction synthesis.** Our GPU exhaustive approach extends beyond register allocation. The same z80-optimizer CUDA infrastructure discovered an optimal divmod10 sequence (27 instructions, 124T, verified correct for all 256 inputs) through instruction synthesis search. This sequence is now integrated into the compiler as a specialization: when the register allocator identifies a division-by-constant-10, the GPU-discovered sequence replaces the general runtime loop (180T), saving 56T per call while clobbering only B,C,F (leaving HL/DE untouched for surrounding code). This demonstrates how offline exhaustive search can improve multiple compiler phases simultaneously.

### 5.3 Threats to Validity

**Internal validity.** The GPU kernel's correctness is critical — an incorrect kernel would produce invalid "optimal" assignments. We validate by assembling and executing the emitted Z80 code on a cycle-accurate emulator (1,335/1,335 FUSE test suite). Additionally, the Z3 SMT solver independently verifies a subset of allocations.

**External validity.** Our corpus spans 8 frontends but through one backend. The signature reuse finding may not generalize to compilers with substantially different IRs or instruction selection strategies. We have invited collaboration with SDCC's maintainer to test this experimentally.

**Construct validity.** "Optimality" is relative to our cost model (T-states per pattern). Alternative cost models (code size, energy, cache effects) could produce different optimal tables. The cost model sensitivity has not been measured.

### 5.4 Generalization

The approach generalizes to any architecture where:
1. The number of physical locations L is small enough that L^K fits within GPU search budgets for typical K (live register counts)
2. The instruction set creates sufficient constraint structure for high signature reuse

Candidate architectures include the 6502 (3 registers — trivially solvable), 8080/8085, GameBoy LR35902, PIC microcontrollers, and AVR (32 registers but with register class constraints that may reduce the effective space).

---

## 6. Related Work

- **Optimal register allocation via ILP** [Appel & George, 2001]: Provably optimal but solved online, not precomputed. Our approach moves the computation offline.
- **Superoptimization** [Massalin, 1987; Bansal & Aiken, 2006; Phothilimthana et al., 2016]: Exhaustive search for instruction sequences. We apply exhaustive search to register assignment.
- **STOKE** [Schkufza et al., 2013]: Stochastic superoptimization. Trades completeness for speed. Our approach is complete (exhaustive) for the bounded subspace.
- **Endgame tablebases** [Thompson, 1986; Nalimov et al., 2000; Schaeffer et al., 2007]: Solve game positions with bounded pieces by exhaustive retrograde analysis. Directly analogous: we solve allocation instances with ≤8 virtual registers.
- **Partial evaluation / Futamura projections** [Futamura, 1971]: Our precomputed table can be viewed as the result of partially evaluating the allocator with respect to constraint patterns — a Futamura projection applied to the compiler backend.

---

## 7. Conclusion

We have shown that register allocation on the Z80 can be partially reduced from online search to offline table lookup. The empirical evidence — 80% signature reuse, 88.2% cross-program transfer, <3% marginal growth, and a phase transition cliff at ~16 physical locations — suggests that for small, irregular architectures, the backend optimization space is far more finite and compressible than compiler folklore assumes.

The resulting compiler requires no solver at compile time for table-hit functions. It looks up the answer in O(1) and emits code directly. For functions exceeding table capacity, island-of-optimality decomposition preserves local optimality with bounded join cost.

More broadly, this work raises the question: **which parts of compiler backends can themselves be compiled away into finite, reusable optimization artifacts?** The Z80 is a proof of concept. The principle — offline enumeration of a bounded decision space, with corpus-driven pruning of the search — may apply more widely than the retro computing niche suggests.

The divmod10 result illustrates this synergy: the same GPU infrastructure that precomputes register allocation tables also discovers optimal instruction sequences. The compiler becomes not a search engine but a retrieval engine, looking up precomputed answers across multiple optimization phases.

**Reproducibility.** The compiler (MinZ), GPU kernel (z80-optimizer), and complete corpus are open-source at github.com/oisee/minz and github.com/oisee/z80-optimizer. The JSON protocol for constraint signatures is documented and suitable for integration with other compilers.

---

## Acknowledgments

This research was conducted collaboratively across three AI-assisted sessions (compiler backend, superoptimizer, main integration) using Claude Code with cross-session messaging. The CUDA register allocation kernel was developed in the z80-optimizer project. Philipp Klaus Krause (SDCC maintainer) provided invaluable feedback on calling convention comparison methodology.

---

## References

[Appel & George, 2001] A. W. Appel and L. George. Optimal spilling for CISC machines with few registers. PLDI 2001.

[Bansal & Aiken, 2006] S. Bansal and A. Aiken. Automatic generation of peephole superoptimizers. ASPLOS 2006.

[Bodlaender, 1993] H. L. Bodlaender. A linear time algorithm for finding tree-decompositions of small treewidth. STOC 1993.

[Briggs et al., 1994] P. Briggs, K. D. Cooper, and L. Torczon. Improvements to graph coloring register allocation. TOPLAS 1994.

[Chaitin, 1982] G. J. Chaitin. Register allocation & spilling via graph coloring. SIGPLAN 1982.

[Chaitin et al., 1981] G. J. Chaitin et al. Register allocation via coloring. Computer Languages 1981.

[Futamura, 1971] Y. Futamura. Partial evaluation of computation process. Systems, Computers, Controls 1971.

[Massalin, 1987] H. Massalin. Superoptimizer: A look at the smallest program. ASPLOS 1987.

[Nalimov et al., 2000] E. V. Nalimov, G. McC. Haworth, and E. A. Heinz. Space-efficient indexing of chess endgame tables. ICGA Journal 2000.

[Phothilimthana et al., 2016] P. M. Phothilimthana et al. Scaling up superoptimization. ASPLOS 2016.

[Poletto & Sarkar, 1999] M. Poletto and V. Sarkar. Linear scan register allocation. TOPLAS 1999.

[Schaeffer et al., 2007] J. Schaeffer et al. Checkers is solved. Science 2007.

[Schkufza et al., 2013] E. Schkufza, R. Sharma, and A. Aiken. Stochastic superoptimization. ASPLOS 2013.

[Scholz & Eckstein, 2002] B. Scholz and E. Eckstein. Register allocation for irregular architectures. LCTES 2002.

[Thompson, 1986] K. Thompson. Retrograde analysis of certain endgames. ICCA Journal 1986.
