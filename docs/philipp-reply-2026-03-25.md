# Reply to Philipp — March 25, 2026

Dear Philipp,

A significant architectural development since our last exchange — we've moved beyond Z3 as the primary solver.

## GPU Exhaustive Register Allocation

The key observation: the Z80 has 15 usable register locations. For a function with N virtual registers, there are 15^N possible assignments. For N≤8, this is at most 2.5 billion — exhaustively searchable on a modern GPU in seconds.

We built a CUDA kernel that brute-forces ALL register assignments for a given constraint set, evaluating each against the full Z80 pattern table (tied operands, accumulator-only ALU, pair constraints). The result is provably globally optimal — not "good enough", not "within a bound", but the actual minimum-cost assignment.

The practical workflow:

1. **Offline:** Compile a representative corpus. For each function, extract the constraint signature (which patterns are available, which vregs interfere). Send to GPU.
2. **GPU solves all of them.** Dual RTX 4060 Ti: 19,000 solves/second. Our 1,605-function corpus (8 language frontends) solved in under a minute.
3. **Ship a lookup table** with the compiler. At compile time: hash the function's constraints → table lookup → done. O(1), zero solver dependency.

The numbers:

| Metric | Value |
|--------|-------|
| Functions in corpus | 1,605 (Nanz, C89, Pascal, PL/M, ABAP, Frill, Lizp, Lanz) |
| Unique constraint signatures | 312 |
| Signature reuse rate | **80%** |
| Functions with ≤8 vregs | 87% |
| Table entries needed | 56 (covers 639 functions, 39.8%) |

The 80% reuse rate is the surprising finding. The Z80 instruction set creates a small vocabulary of constraint patterns. Different functions — across different languages — produce the same regalloc problem. A 56-entry table covers 40% of all functions we've ever compiled.

## Beyond 8 Vregs: Island-of-Optimality Decomposition

Your review raised the important question of scalability. For functions with >8 vregs (13% of our corpus), GPU brute-force is impractical (15^9 = 38 billion). Our solution:

**Split at liveness bottlenecks.** After every CALL instruction, the live register set drops dramatically (return value + caller-saved only). These are natural partition points. A 14-vreg function typically decomposes into 4-5 islands of 3-5 vregs each — all within GPU range.

Each island is solved optimally via table lookup. Between islands, we insert register shuffle moves (LD r,r' = 4T, EX DE,HL = 4T, PUSH/POP = 11T). The boundary graph has 2-4 edges — we enumerate all possible shuffles and pick the minimum.

**Guarantee:** Each island is provably optimal. Total cost = Σ(optimal islands) + bounded join cost. This is strictly better than SDCC's global graph coloring, which makes heuristic choices within what we call "islands" and across boundaries equally.

## Corpus-Driven Enumeration (Not Blind Search)

An efficiency insight: blind enumeration of all possible constraint patterns is intractable (3.8 billion for 6 vregs). But real programs use only ~15 unique constraint shapes per vreg count. By extracting shapes from the corpus and varying only the interference graph, we reduce the search space by **7,700x** — from 3.8 billion to 491,000 patterns for 6 vregs. Five minutes on dual GPU instead of eighteen hours.

This is because the Z80 instruction set constrains register usage in predictable ways. An ADD always needs A. A 16-bit load always needs HL or DE. The constraint vocabulary is finite and small.

## Relevance to Your SDCC Questions

### Separate compilation
The GPU table approach sidesteps this entirely. The table is built offline from a corpus — any corpus, any compilation model. At compile time, there's no solver to scope-limit. A function compiled separately gets the same optimal allocation as one compiled whole-program, because the table entry depends only on the constraint signature, not on context.

### Inlining vs. PFCCO
With the island architecture, this trade-off becomes concrete. If inlining a function keeps the island's vreg count ≤8, the GPU table gives the optimal allocation for the inlined code. If inlining pushes vregs above the island limit, a CALL boundary (with PFCCO-optimized convention) may be cheaper. We can now **measure** this trade-off precisely rather than estimating it.

### Practical scalability
We tested across 8 language frontends — not just Nanz. C89 programs, Pascal programs, even ABAP-on-Z80 all produce constraint signatures that hit the same table entries. The approach is language-independent.

## Current Status

- **Z3 solver:** 645/645 functions, 55/55 Z80-verified, -60% vs SDCC (unchanged)
- **GPU table:** 56 entries, 639 functions covered, direct PIR emit (zero Z3)
- **Island decomposition:** Architecture defined (ADR-0040), implementation next
- **Instruction synthesis:** GPU-discovered multiply sequences (103 constants), divmod10 in 27 instructions

The Z3 solver remains as fallback for table misses, but our goal is to make the table comprehensive enough that Z3 is never invoked during normal compilation. We're compiling the compiler itself.

I'd be very interested in your thoughts on:
1. Whether the 80% signature reuse would hold for SDCC's function corpus
2. Whether island decomposition at CALL boundaries aligns with your experience on register pressure patterns in real Z80 code
3. Whether a similar approach could benefit SDCC — even a small lookup table for the most common 2-4 vreg patterns could speed up compilation while guaranteeing optimality for those cases

## z80-superoptimizer: GPU-Accelerated Instruction Synthesis

Alongside the regalloc work, we've been building an open-source Z80 superoptimizer that runs entirely on CUDA. The core idea: emulate the Z80 on GPU — thousands of candidate instruction sequences executing in parallel, each verified against a reference implementation for all possible inputs.

What it's found so far:

- **Multiply-by-constant table:** 103 entries for 8-bit (A×K mod 256), 51+ entries for 16-bit (HL×K). Some patterns no human would write — `NEG` as ×255 (8T), carry-chain sequences like `RLA / NEG / ADD A,A` for ×252.
- **divmod10:** 27 instructions, 124 T-states, verified correct for all 256 inputs. Uses Hacker's Delight reciprocal approximation with RRA+AND trick. Clobbers only B,C,F — leaves HL/DE untouched.
- **Register allocation brute-force:** The same CUDA kernel that finds instruction sequences also solves regalloc — try all 15^N assignments, keep the cheapest valid one. This is what powers the exhaustive tables described above.

The superoptimizer treats the Z80 as a black box: any instruction sequence that produces correct output for all inputs is valid, regardless of how "weird" it looks. The GPU doesn't know about conventional programming patterns — it just searches. This is how it discovers things like using `NEG` (negate accumulator) as a multiply-by-255, or `RL B` to capture carry from 8-bit operations into the high byte of a 16-bit result.

The repository is at github.com/oisee/z80-optimizer. The CUDA kernel supports:
- Instruction sequence synthesis (find shortest program for a given function)
- Register allocation brute-force (15-location model with width constraints)
- Server mode for batch processing (stdin/stdout JSON protocol)

We think this approach generalizes to other constrained architectures — 6502, 8080, GameBoy Z80 variant. The emulation core is small (~200 lines of CUDA per target), and modern GPUs can evaluate billions of candidate programs per second.

The full technical details and research paper seeds: `docs/2026-03-25-Superoptimizer-Research-Ideas.md` in the repository. We're also preparing the paper revision with these new results.

Best regards,
Alice
