# Reply to Philipp — March 25, 2026 (v2)

Dear Philipp,

Since our last exchange, we've stumbled onto something that may interest you beyond the MinZ/SDCC comparison — a structural result about the nature of register allocation on small architectures.

## The Surprising Finding: 88.2% Cross-Program Transfer

We built a GPU brute-force register allocator that exhaustively searches all 15^N possible register assignments for the Z80 (N = number of virtual registers). For N≤8, this is tractable — seconds on a modern GPU. The result is provably globally optimal: not heuristic, not approximate, but the actual minimum-cost assignment.

The surprising part isn't the GPU. It's what the GPU revealed about the structure of the problem.

We compiled 1,605 functions from 8 source language frontends (Nanz, C89, Pascal, PL/M, ABAP, Frill, Lizp, Lanz) through our shared HIR→MIR2→VIR pipeline, and extracted the register allocation constraint signature for each function at the VIR level. A signature captures: which patterns are available per instruction, which virtual registers interfere, and what the tied-operand/accumulator constraints are. It does NOT capture function names, immediates, or source language.

**Result: 1,605 functions collapse to 312 unique signatures. 80% reuse.**

We then ran a transfer experiment: built the lookup table from one subset of programs (Nanz core + PL/M), tested on a disjoint set (ABAP + SQLite + Screen rendering).

**88.2% hit rate on unseen programs.** ABAP business logic: 98.8%. Screen rendering: 89.6%.

**Important caveat:** All 8 frontends share the same IR pipeline (HIR→MIR2→VIR), so the 80% reuse and 88.2% transfer reflect the constraint vocabulary of *our IR and backend*, not necessarily a universal property of the Z80 ISA. Different compilers with different IRs and instruction selection strategies may produce different constraint signatures.

This is exactly why we'd like to test on SDCC — it would tell us whether the low signature entropy is a property of the Z80 itself, or an artifact of our particular IR design.

## An Invitation: Test This on SDCC's Corpus

This leads to a concrete, testable hypothesis — but one we cannot test alone:

> *For a fixed ISA with N physical locations, the number of distinct allocation constraint signatures encountered in practice is bounded by a function much smaller than the theoretical N^K. Open question: is this bound a property of the ISA, or of the compiler's IR?*

You're uniquely positioned to help answer this. SDCC has a completely different IR, different instruction selection, and a much larger real-world C corpus. The experiment:

1. For each function SDCC compiles, extract the interference graph + register class constraints
2. Hash them into a canonical signature
3. Count unique signatures vs total functions
4. Measure: does the 80% reuse hold? Is the signature vocabulary similar to ours?

If SDCC also shows high signature reuse — the low entropy is a property of the Z80 ISA itself. If SDCC shows low reuse — the constraint vocabulary depends heavily on IR design, which is equally interesting and worth reporting honestly.

We'd be happy to share our signature extraction code and GPU solver. The JSON protocol is simple (one JSON per function, stdin/stdout).

## A Second Hypothesis: Irregularity Helps

One of our peer reviewers (we consulted three independent AI researchers) made an observation that inverts conventional wisdom:

> *Architecture irregularity doesn't just make compilation harder — it makes the constraint space more compressible. Fewer valid assignments means smaller tables and higher reuse.*

The Z80's messy register file (accumulator-only ALU, tied operands, pair constraints, DD/FD prefix conflicts) drastically reduces the number of valid allocations. A regular architecture like ARM with 16 interchangeable registers has a much larger effective search space — every register is equally valid, so there are more distinct constraint patterns.

This predicts a **phase transition**: as the number of physical locations grows, "tableability" (fraction of functions solvable by precomputed table) drops sharply.

We're testing this now by parametrizing our GPU kernel:

| Architecture | Locations | Predicted Tableability |
|-------------|-----------|----------------------|
| 6502 | 3 (A,X,Y) | ~99% |
| Z80 GPR only | 7 | ~92% |
| Z80 full | 15 | 88.2% (measured) |
| ARM Thumb | 16 | ~5%? |
| RISC-V | 32 | ~0%? |

If the phase transition is real, it explains why exhaustive allocation has never been tried on modern architectures — not because the idea is wrong, but because the ISA doesn't support it. The Z80 sits in a sweet spot: complex enough to be interesting, constrained enough to be solvable.

## The z80-superoptimizer Project

We've also built an open-source Z80 superoptimizer that runs entirely on CUDA (github.com/oisee/z80-optimizer). It emulates the Z80 on GPU — thousands of candidate instruction sequences executing in parallel, verified against a reference for all inputs.

Results so far:
- **Multiply-by-constant table:** 103 entries for 8-bit, 51+ for 16-bit. GPU-discovered patterns like `NEG` as ×255 (8T).
- **divmod10:** 27 instructions, 124 T-states, verified all 256 inputs. Uses Hacker's Delight reciprocal with RRA+AND optimization.
- **A negative result as theorem:** Exhaustive GPU search proves no instruction sequence of length ≤12 (over a set of 18 Z80 opcodes) computes floor(n/10) for all n ∈ {0..255}. This is a lower bound via exhaustive search certificate — 37.8 billion sequences verified at length 8, 794 billion at length 9.

The superoptimizer also powers the register allocation brute-force: same CUDA kernel, different cost function.

## Three Concrete Proposals

**1. Joint experiment on SDCC corpus.** You extract constraint signatures from SDCC's compilation of real Z80 programs (CP/M utilities, embedded firmware, games). We provide the GPU solver. Together we measure: does the signature vocabulary overlap? Is the reuse rate ISA-dependent or compiler-dependent?

**2. Phase transition validation.** SDCC also targets several architectures (Z80, 8051, STM8, etc). Running the same experiment across targets would map the "tableability frontier" — where exhaustive precomputation transitions from practical to impractical.

**3. SDCC integration prototype.** For `static` functions whose address is not taken (as you suggested in your review), SDCC could use a precomputed table for allocation. The table is small (~300 entries for Z80), the lookup is O(1), and the result is provably optimal. Even a partial integration would be interesting — replacing graph coloring for leaf functions with table lookup.

## Our Current Understanding

We've had the benefit of extensive discussion and peer review. The emerging picture:

- **Register allocation on constrained ISAs may be a "solved game"** — like chess endgame tablebases, the space is finite and the optimal answers can be precomputed.
- **The compiler becomes a retrieval engine**, not a search engine. For 87% of functions (those with ≤8 virtual registers), we look up the answer. For the rest, we decompose into solvable subproblems at call boundaries.
- **Calling conventions are part of the optimization space**, not external constraints. Z3-PFCCO jointly optimizes conventions across all functions in a module. This is where most of the -60% vs SDCC comes from.

We're now writing Paper A: "Precomputed Optimal Register Allocation via Corpus-Driven Exhaustive GPU Search" — focused on the Z80 as a case study for the broader principle. Your input on whether this generalizes to SDCC's perspective would be invaluable.

The full research document with all data, hypotheses, and peer feedback: github.com/oisee/minz, `docs/2026-03-25-Superoptimizer-Research-Ideas.md`.

Best regards,
Alice

P.S. — All your original review points are addressed in the paper revision. The do-while comparison, SDCC 4.5.0 update, and "17x smaller" terminology fix are all incorporated. The calling convention analysis now includes concrete T-state breakdowns showing exactly where the overhead lives.
