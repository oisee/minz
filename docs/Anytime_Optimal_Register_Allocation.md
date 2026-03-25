# Anytime-Optimal Register Allocation: A Graceful Degradation Pipeline

## The Problem

Register allocation — assigning virtual registers to physical locations — is NP-complete [Chaitin, 1981]. Every compiler must solve it, and every compiler makes tradeoffs: speed vs quality, optimality vs reliability. Most choose heuristics (graph coloring, linear scan) that are fast but produce mediocre code.

We take a different approach: **start with provably optimal, degrade gracefully**.

## The Pipeline

The MinZ VIR backend implements a 5-level anytime register allocator. Each level is a complete fallback for the one above. The compiler **never fails** — it always produces correct code. The question is only *how optimal* that code is.

```
                    ┌──────────────────────┐
                    │   Function arrives   │
                    │     (N vregs)        │
                    └──────────┬───────────┘
                               │
                    ┌──────────▼───────────┐
             ┌──── │  L1: Table lookup    │ ────┐
             │ hit │  O(1), optimal       │miss │
             │     └──────────────────────┘     │
             │                                   │
             │     ┌──────────▼───────────┐     │
             │  ┌──│  L2: Z3 SMT solver   │──┐  │
             │  │ok│  seconds, optimal    │TO │  │
             │  │  └──────────────────────┘   │  │
             │  │                              │  │
             │  │  ┌──────────▼───────────┐   │  │
             │  │  │  L3: Backtracking    │   │  │
             │  │  │  minutes, optimal    │   │  │
             │  │  │  1000x pruning       │   │  │
             │  │  └──────────┬───────────┘   │  │
             │  │          too│slow            │  │
             │  │  ┌──────────▼───────────┐   │  │
             │  │  │  L4: Islands         │   │  │
             │  │  │  split at bottleneck │   │  │
             │  │  │  solve each part     │   │  │
             │  │  │  bounded overhead    │   │  │
             │  │  └──────────┬───────────┘   │  │
             │  │        no   │split          │  │
             │  │  ┌──────────▼───────────┐   │  │
             │  │  │  L5: PBQP fallback   │   │  │
             │  │  │  heuristic           │   │  │
             │  │  │  always succeeds     │   │  │
             │  │  └──────────┬───────────┘   │  │
             │  │             │               │  │
             ▼  ▼             ▼               ▼  ▼
        ┌─────────────────────────────────────────┐
        │         Emit Z80 Assembly               │
        │    (correct code, guaranteed)            │
        └─────────────────────────────────────────┘
```

### Optimality at Each Level

```
   Quality ▲
           │ ★ ★ ★ ★ ★  Provably Optimal
   100%    │━━━━━━━━━━━━━━━━━━━━━━━┓
           │  L1: Table  L2: Z3    ┃  L3: Backtrack
           │  (0.001s)   (5s)      ┃  (300s)
           │                       ┃
    ~94%   │                       ┗━━━━━━━━━━━┓
           │                       L4: Islands  ┃ (bounded loss:
           │                       (locally     ┃  ≤6% shuffle)
           │                        optimal)    ┃
    ~85%   │                                    ┗━━━━━━━━━━━┓
           │                                    L5: PBQP    ┃
           │                                    (heuristic) ┃
           │                                                ┃
           └────────────────────────────────────────────────┻──▶ Time
              O(1)        seconds      minutes         ms
```

### Coverage Waterfall

```
   Functions ▲
   covered   │
             │ ██████████████████████████████████████  87% ← L1: Table (≤8v)
             │ ████████████████████████████████████████████████  99% ← L2: Z3
             │ █████████████████████████████████████████████████████  99.9% ← L3
             │ ██████████████████████████████████████████████████████████ 99.99% ← L4
             │ ████████████████████████████████████████████████████████████ 100% ← L5
             └──────────────────────────────────────────────────────────────▶
```

## Level 1: GPU Precomputed Table (O(1), Provably Optimal)

**How it works.** Before the compiler ships, we enumerate all possible register allocation constraint shapes for small functions and solve them exhaustively on GPU. The Z80 has 15 usable physical locations. For a function with N virtual registers, there are 15^N possible assignments. For N≤8, this is at most 2.56 billion — searchable on a modern GPU in seconds.

The results are stored as a lookup table: `hash(constraint_signature) → optimal_assignment`. At compile time, the compiler hashes the function's constraints and looks up the answer. No solver runs. No search happens. Just a hash lookup and pattern emit.

**What's a constraint signature?** A deterministic hash of everything that matters for register allocation: how many virtual registers, which instructions use them, which registers can hold each operand (location constraints from the ISA), which vregs are simultaneously alive (interference), and whether operands must share a register (tied constraints). Two functions with identical constraint signatures — regardless of what they compute — get the same optimal allocation.

**Why this works on Z80.** The Z80's irregular register file (accumulator-only ALU, register pairs, DD/FD prefix conflicts) severely constrains valid assignments. This is usually seen as a disadvantage. For exhaustive search, it's an advantage: fewer valid assignments means a smaller table and higher reuse across programs.

**Empirical result.** 1,645 functions from 8 language frontends produce only 315 unique constraint signatures (80.9% reuse). Adding an entire standard library produces fewer than 3% new signatures — the vocabulary has converged.

**Optimality guarantee.** Every table entry was found by evaluating ALL possible assignments. The stored solution is the global minimum-cost allocation. No heuristic can do better.

## Level 2: Z3 SMT Solver (Seconds, Provably Optimal)

**When it triggers.** Table miss — the function has a constraint signature not in the precomputed table.

**How it works.** The compiler encodes the allocation problem as an SMT formula and submits it to Z3. Per-instruction variables (`lv{vreg}_i{inst}`) allow the solver to plan register moves as part of the optimal solution, not as a post-pass fixup. The `(minimize total_cost)` directive finds the provably optimal solution.

**Key innovations:**
- **Z3-PFCCO (Per-Function Calling Convention Optimization):** Z3 considers all call sites in the module simultaneously, choosing calling conventions that minimize total register shuffle overhead. This is interprocedural — SDCC and GCC cannot do this.
- **Per-instruction variables:** Unlike global allocation (one location per vreg), per-instruction variables let a vreg change location during its lifetime. The solver inserts moves only when they reduce total cost.
- **Joint isel+regalloc:** Instruction selection and register allocation are solved in a single Z3 query. The solver picks the cheapest instruction pattern AND the best register assignment simultaneously.

**Optimality guarantee.** When Z3's optimizer converges (which it does for functions with <100 variables), the solution is provably optimal with respect to the T-state cost model.

**Result on ZSQL corpus.** 31/31 functions compiled via Z3. 112/112 ABAP functions compiled via Z3. Zero heuristic fallback.

## Level 3: CPU Backtracking with Pruning (Minutes, Provably Optimal)

**When it triggers.** Z3 times out or returns "unknown" (typically for functions with >100 SMT variables, i.e., >15 vregs with dense interference).

**How it works.** A CPU-based backtracking solver enumerates assignments, but with aggressive pruning:

- **Interference-aware pruning:** If vregs A and B interfere, and A is assigned to register R, immediately eliminate R from B's candidates. This propagates through the constraint graph, often eliminating 99.99%+ of the search space.
- **Most-constrained-first ordering:** Assign the vreg with the fewest remaining valid locations first. This fails fast on impossible branches.
- **Pattern-aware location masks:** Each instruction's pattern constraints pre-filter which locations are valid for each vreg at each program point.

**Pruning effectiveness.** On a 15-vreg island with 7 interference pairs:
- Brute force: 437 trillion assignments (15^15)
- Backtracking: 587 million nodes explored
- **Pruning factor: 745,000×**

With pattern-aware masks added: 350K nodes — **1,680× faster still**, under 1 second.

**Optimality guarantee.** Backtracking is exhaustive — it explores the entire pruned search tree. The best solution found IS the global optimum. Same guarantee as GPU brute-force, just slower.

## Level 4: Island Decomposition (Locally Optimal + Bounded Overhead)

**When it triggers.** Function has too many vregs for any single solver (>15 vregs, dense interference graph).

**How it works.** Large functions are split into smaller "islands" at **liveness bottlenecks** — program points where few virtual registers are simultaneously alive. On Z80, these occur naturally at CALL instructions (calling convention forces most registers to be dead).

1. **Find bottlenecks:** Compute per-program-point liveness via backward dataflow. Identify call sites where the live set drops below the table capacity.
2. **Split at bottlenecks:** Cut the function into islands, each containing a subset of instructions and vregs.
3. **Solve each island:** Each island is a self-contained allocation problem. Because it has ≤15 vregs, it can be solved optimally by any of Levels 1-3.
4. **Stitch at boundaries:** At split points, registers may need to be shuffled between islands. The shuffle cost is bounded.

**Visual: Island decomposition of a 28-vreg function**

```
  Live vregs ▲
   at point  │
         15  │─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ GPU table limit
             │
         10  │    ████
             │   ██████        ████
          5  │  ████████      ██████       ████
             │ ██████████    ████████     ██████
          2  │─████▼█████──▼█████████──▼████████──── bottleneck
          0  │━━━━━╋━━━━━━╋━━━━━━━━━━╋━━━━━━━━━━━▶ Program point
             │     │      │          │
             │ Island 0   │ Island 1 │ Island 2
             │  (14v)     │  (15v)   │  (12v)
             │ OPTIMAL    │ OPTIMAL  │ OPTIMAL
             │            │          │
             │     ◄──────┤          │
             │     2 vregs│shuffled  │
             │     (8T)   │  (8T)    │
```

Each island is solved optimally by GPU table or backtracking. Shuffle cost at boundaries is bounded: ≤ |boundary_vregs| x 4T per LD instruction.

**Why bottlenecks exist.** Real programs don't keep all variables alive simultaneously. A function with 37 total vregs may have at most 10 alive at any point. The other 27 have non-overlapping lifetimes. Bottleneck points (where the live set is smallest) are natural cut points.

**Optimality guarantee.** Each island is provably optimal (solved by Levels 1-3). Total cost = Σ(optimal island costs) + Σ(boundary shuffle costs). The shuffle cost per boundary is bounded by:

```
shuffle_cost ≤ |live_at_boundary| × 4T    (LD r,r' = 4 T-states each)
```

For a typical function with 3 islands and 2-3 boundary vregs: overhead ≈ 24T out of ~400T total = **6% overhead** for guaranteed solvability.

**The more you split, the easier it gets.** In the limit, every instruction is its own island (1 vreg = trivially solvable). The tradeoff is more shuffle overhead. The greedy merge algorithm maximizes island size while staying within the solver's capacity.

**Empirical result on ZSQL:**
- `main` (18 vregs): 2 islands, 4T shuffle, total 184T — provably optimal
- `_prompt` (21 vregs): 2 islands, 8T shuffle, total 183T — provably optimal
- `_prow` (28 vregs): 3 islands (14v + 15v + 12v), ~8T shuffle per boundary
- `_sel_rows` (37 vregs): 3 islands (14v + 14v + 9v) with threshold=8

## Level 5: PBQP / Graph Coloring (Heuristic, Always Succeeds)

**When it triggers.** All optimal paths exhausted — no table hit, Z3 timeout, backtracking too slow, islands don't decompose cleanly.

**How it works.** Standard heuristic register allocation: PBQP (Partitioned Boolean Quadratic Programming) or Chaitin-style graph coloring with spilling. This is what SDCC and most production compilers use as their *only* allocator.

**Quality.** Heuristic — no optimality guarantee. But produces correct, reasonable code. The same quality as SDCC 4.2.0.

**When does this actually happen?** In practice, almost never. On the ZSQL corpus (31 functions) and ABAP corpus (112 functions), Level 5 is triggered for **zero** functions. Every function is solved optimally by Levels 1-3, or near-optimally by Level 4.

## The Anytime Property

This pipeline has the **anytime property**: you can stop at any level and get a valid result. More compute time → better result, but you always have *something*.

| Level | Time | Quality | Coverage |
|-------|------|---------|----------|
| 1 (Table) | O(1) | Provably optimal | 87% of functions |
| 2 (Z3) | seconds | Provably optimal | 99%+ |
| 3 (Backtrack) | minutes | Provably optimal | 99.9%+ |
| 4 (Islands) | minutes | Near-optimal (bounded loss) | 99.99% |
| 5 (PBQP) | milliseconds | Heuristic | 100% |

The table grows over time. Every Z3 or backtracking solution can be cached and added to the table. The compiler gets better with every compilation — it's a **learning system** without machine learning.

## The Additive Table Property

The precomputed table is **additive**: new entries can be added without invalidating existing ones. This enables:

1. **Ship with ≤6v complete table** (~1.25M entries, 23 min GPU time). Zero misses for small functions.
2. **Grow to 7v** over a weekend (2-3 days GPU time). Coverage jumps to 93%.
3. **Add 8v corpus-derived entries** on-demand. Each new program may contribute a few new signatures.
4. **Community contributions.** Anyone with a GPU can solve new signatures and submit them. The table is a shared, append-only artifact.

The table file is ~10KB per 100 entries (JSON). A complete ≤6v table would be ~125KB — fits in L2 cache.

```
  Table       GPU time to
  entries     compute          Coverage    Miss rate
  ─────────   ──────────       ────────    ─────────
  ≤6v  1.25M  23 min           62.8%       0% (complete)
  +7v  ~15M   2-3 days         87%         0% (complete)
  +8v  ~100M  ~1 week (A100)   93%         corpus-dependent
  +corpus     +315 sigs        88.2%       on-demand Z3 fills gaps
  ────────────────────────────────────────────────────
  Each new entry = one fewer Z3 call at compile time.
  Table is additive — never invalidates existing entries.
```

## Comparison with Other Compilers

| Feature | SDCC | GCC | LLVM | MinZ VIR |
|---------|------|-----|------|----------|
| Allocator | Graph coloring | PBQP | Greedy | 5-level anytime |
| Optimality | Heuristic | Heuristic | Heuristic | Provable (L1-L3) |
| Precomputed | No | No | No | Yes (GPU table) |
| Cross-function | No | Limited | LTO | Z3-PFCCO |
| Worst case | Graph coloring | PBQP | Greedy | PBQP (same as GCC) |
| Best case | Graph coloring | PBQP | Greedy | O(1) table lookup |

## The "Solved Game" Analogy

Chess endgame tablebases solve all positions with ≤7 pieces by exhaustive retrograde analysis. The result: when a chess engine reaches a 7-piece position, it plays perfectly — not approximately, not heuristically, but *provably optimally*.

Our GPU register allocation table is the compiler equivalent. For functions with ≤6 virtual registers (62.8% of our corpus), the compiler plays perfectly. It doesn't search, doesn't approximate, doesn't guess. It looks up the answer and emits optimal code.

For larger functions, we decompose into solved subgames (islands) with bounded transition costs (shuffles). The analogy extends: a chess engine decomposes a 20-piece game into a sequence of captures that eventually reach a solved endgame. We decompose a 37-vreg function into a sequence of islands that each reach a solved allocation.

The compiler becomes not a search engine, but a **retrieval engine** operating on precomputed optimal solutions.

---

*MinZ VIR backend: provably optimal register allocation for Z80 and constrained architectures.*
*Implementation: `pkg/vir/` — solver.go, cfgsolver.go, regalloc_table.go, gpu.go, pipeline.go*
