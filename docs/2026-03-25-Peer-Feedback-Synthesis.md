# Peer Feedback Synthesis — 2026-03-25

Three AI researchers reviewed our Superoptimizer Research Ideas document.
Sources: `docs/_in/chat-gpt-feedback.md`, `docs/_in/gemini-feedback-01.md`, `docs/_in/claude-feedack.md`

---

## Consensus: What All Three Agree On

1. **GPU exhaustive table + corpus-driven enumeration = publishable.** Paper A is ready to write.
2. **80% signature reuse is the killer result.** Not GPU speed — the REUSE RATE.
3. **Need corpus transfer experiment.** → DONE: 88.2% (z80-optimizer ran it)
4. **"Compile the compiler" / "Solved games" framing is strong** but needs careful scoping
5. **Island-of-Optimality needs either proof or large empirical study** to be a standalone paper

## Unique Insights Per Reviewer

### ChatGPT — Strongest Practical Guidance
- Paper A title: "Precomputed Optimal Register Allocation for the Z80 via Corpus-Driven Exhaustive GPU Search"
- Narrow claim: "low-register irregular architectures with small live-set envelopes"
- 14 research angles including "Effective Entropy", "Retrieval-Based Compilation", "Phase Transitions"
- Need: cost model sensitivity, honest failure cases, benchmark protocol for -60% SDCC claim
- Dissertation structure: 7 parts from Problem Reframing to Limits
- "Irregularity as Structure" — conterthesis that irregularity HELPS by reducing ambiguity

### Claude Desktop — Strongest Theoretical Insights
- **Futamura projection** applied to register allocator: partial evaluation of allocator on constraint patterns → table
- **80% reuse is potentially a theorem**, not just statistics: "For fixed ISA with N registers, # of constraint signatures bounded by f(N) << N^K"
- **Island-split = treewidth decomposition** (Bodlaender 1993): CALL boundaries are graph separators, optimality gap has known theoretical bounds
- **PFCCO is ISA-independent** — separate paper, applies to modern architectures
- **Divmod negative result is a theorem**: exhaustive search certificate proves lower bound ≥13 instructions
- Three CUDA kernels = three different search geometries (assignment, synthesis, approximation)

### Gemini — Strongest Engineering/Visualization Ideas
- "Periodic Table of Z80 Constraints" — atlas of 312 signatures
- WFC as "Entropy-Reduced Register Allocation" — formalize
- Pareto front: return both min-T-states AND min-bytes per signature
- GameBoy LR35902 as validation target
- 6502 Zero Page as knapsack problem (different kernel entirely)
- "Endgame Tablebases for Compilers" as paper title

## Key Cautions (All Three)

1. **"Provably optimal"** — always qualify: "with respect to modeled cost function and legal assignment space"
2. **"Z3 cannot find these"** — reframe as "different search geometry", not Z3 limitation
3. **"No compiler uses IXH/IXL"** — weaken to "we are not aware of mainstream Z80 compilers..."
4. **"-60% vs SDCC"** — needs benchmark protocol, metric definition, semantic equivalence criteria
5. **Corpus transfer risk** — ADDRESSED: 88.2% cross-frontend (z80-optimizer experiment)

## Action Items from Feedback

### Immediate
- [x] Corpus transfer experiment → 88.2% (done by z80-optimizer)
- [ ] Pareto front in GPU kernel (--pareto flag) — z80-optimizer next
- [ ] GameBoy LR35902 validation — z80-optimizer next

### For Paper
- [ ] Signature algebra formalization (canonical form, symmetry group)
- [ ] Marginal coverage curve (functions covered vs table entries)
- [ ] Cost model sensitivity analysis
- [ ] Honest failure cases section
- [ ] Phase transition hypothesis: Z80=88%, 6502≈99%, ARM Thumb≈5%

### Theoretical
- [ ] Prove or disprove: 80% reuse as theorem about constraint signature cardinality
- [ ] Treewidth decomposition connection for island-split bounds
- [ ] Futamura projection formalization

## Paper Priority (Consensus)

1. **Paper A** (strongest, ready now): "Exhaustive GPU Regalloc + Corpus-Driven Enumeration"
2. **Paper B** (more fundamental): "Corpus-Driven Constraint Enumeration — general methodology"
3. **Paper C** (needs proof): "Island-of-Optimality with treewidth bounds"
4. **Paper D** (ISA-independent): "Z3-PFCCO Module-Level Calling Convention"

## One-Paragraph Research Statement (ChatGPT formulation, refined)

> Our main result is not merely that GPUs accelerate superoptimization. Rather, they make it practical to empirically probe the structure of backend search spaces on small irregular architectures. What emerges is not unstructured combinatorial explosion, but a sharply compressed vocabulary of recurring allocation signatures and arithmetic idioms. This suggests a broader paradigm: parts of code generation may be shifted from online search to offline knowledge compilation, with the compiler acting as a retrieval engine over precomputed optimality artifacts. We demonstrate this on the Z80 with 88.2% cross-frontend transfer, but the principle applies to any architecture where the effective entropy of the constraint space is low.

## Notable Quotes from Reviewers

- "This is not a Z80 paper. It's a 'solved games in compilers' paper." — z80-optimizer
- "Register allocation is a database query problem, not a search problem." — Claude Desktop
- "The best online solver may be no online solver at all." — ChatGPT (refined)
- "Irregularity hurts local freedom but reduces ambiguity, making the space more compressible." — ChatGPT
- "Backend search spaces may be far more finite than compiler folklore assumes." — ChatGPT
