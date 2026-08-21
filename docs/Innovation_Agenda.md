# Innovation Agenda

**Created:** 2026-08-21
**Basis:** the four audits of 2026-08-21 ([116](../reports/2026-08-21-116-Prior-Art-and-Novelty-Audit.md),
[117](../reports/2026-08-21-117-Solver-Architecture-Audit.md),
[118](../reports/2026-08-21-118-GPU-Superoptimizer-Artifacts-Audit.md),
[119](../reports/2026-08-21-119-Language-and-Codegen-Audit.md)),
which included a deliberate prior-art search against the compiler literature.
**Companion:** [Codegen Remediation Roadmap](Codegen_Remediation_Roadmap.md) — the engineering
debt this agenda depends on.

---

## What the prior-art search actually established

Most of our headline items have heavy prior art, and pretending otherwise costs us the items that
don't. Stated plainly so we stop re-litigating it:

**Not novel.** SMC parameter baking (1980s 8-bit practice; Massalin's Synthesis kernel for the
systematic version) · deforestation and stream fusion (Wadler; Coutts et al., 1990s Haskell) ·
devirtualizing a unique implementor (`-fdevirtualize`; Rust static dispatch) · UFCS (D, Nim) ·
constant folding · PBQP allocation (Scholz & Eckstein 2002; LLVM since 2008) · SMT/CP for joint
instruction selection and register allocation (**Unison**, TOPLAS 2019; Appel & George PLDI 2001;
Goodwin & Wilken 1996) · ISLE (Cranelift) · Datalog over IR (bddbddb, Doop, CodeQL) · e-graphs
(egg) · superoptimization by exhaustive search (Massalin 1987; Bansal & Aiken ASPLOS 2006; STOKE;
Souper) · and "WFC", which is arc-consistency plus a variable-ordering heuristic — and we
implement neither the entropy ordering nor the backtracking.

**That leaves four things with no prior art found.** They are the agenda.

---

## Track A — Retrieval-based code generation *(strongest; publishable)*

**The claim.** Backend constraint spaces on small irregular ISAs have low effective entropy, so
register allocation can move from *online search* to *offline knowledge compilation* — enumerate
exhaustively once, index, look up in O(1) at compile time.

**Why it is not just "Bansal & Aiken for registers."** Their architecture (enumerate → canonicalize
→ index) is the same, and we should cite it. The new part is the **empirical structural result**:

- 1,605 functions across 8 frontends collapse to **312 unique allocation signatures** — 80% reuse.
- **88.2%** of signatures transfer to a disjoint program set.
- Z80 register-file feasibility falls from 78.9% at 4 vregs to 67.7% at 5 — and the enumeration is
  self-consistent to the unit (83,485,612 records, 37,535,076 feasible, formula reproduces counts
  exactly).

**The thesis worth defending: irregularity makes the constraint space *more* compressible, not
less** — because fewer assignments are legal. This predicts a phase transition in "tableability" as
location count grows, and explains why nobody tried this on RISC. It is falsifiable and has an
experimental protocol attached.

**Before this can be published, three things must be true** (Roadmap §3.2–3.3, §4.1):

1. The tables must load — today the format gap makes the whole path inert, and the loader
   *silently* misparses.
2. A real hit/miss counter must exist. "91%" is static eligibility, not a hit rate.
3. Aliasing must be modelled in the generator, or some "feasible optimal" entries are unrealizable.

**And one number must be re-derived.** The "6v: 0.9% feasible" phase-transition figure is
contradicted by our own data — the 6v file covers only treewidth ≥ 4, the hardest 1.7%, measured
at 39.0%. The cliff may still be real; the published figure is not.

**The load-bearing experiment** is the SDCC cross-validation the reviewers already proposed: does
the 80% signature reuse reflect the *Z80 ISA*, or merely *our IR's* constraint vocabulary? Get that
measurement and this is a paper. Without it, it is an artifact.

---

## Track B — Per-function calling conventions on irregular register files *(closest to done)*

**Status:** implemented (`pkg/mir2/contracts.go` — greedy DP on the topo-sorted call graph, 4-pass
convergence), written up in `research/abi-paper/`, and *correctly positioned* — which is rare
enough here to be worth saying.

**The gap is real and precisely stated.** Krause 2013 solves intraprocedural allocation given a
fixed ABI. Krause 2022 searches for the best *single global* ABI — successfully enough that SDCC
broke ABI compatibility to adopt it. Wall (1986), Chow (1988), De Bus (2004), Caldwell & Chiba
(2017) do per-function conventions at link time on **regular RISC register files**. Per-function
*register-class* assignment solved jointly across all call sites on an **irregular** file is
unoccupied.

**What it needs, both flagged by our own reviewers:**

1. **A stated benchmark protocol** — metric (bytes *and* T-states, not instruction counts), SDCC
   version and flags (`--opt-code-size`, `--max-allocs-per-node`), and a semantic-equivalence
   criterion that excludes comparisons where C cannot express the construct.
2. **Edge-weight-aware costs** — `NOTE_swap_degenerate.md` already self-critiques the unit-weight
   cost function.
3. **Retire "−60%"/"−71%".** The harness prints **−29%** on eight functions. Publish that.

Position as complementary to SDCC, not against it: PFCCO chooses ABIs, Krause-style allocation runs
per-function underneath. Delete "the SDCC killer."

---

## Track C — The spill-tier lattice *(most original, least built)*

**The idea.** Compilers have two storage tiers: register and stack slot. We describe seven — GPR →
IX/IY halves → I/R → SMC immediate → shadow bank → stack → memory — each with a T-state cost *and a
safety predicate* (recursion-safety for SMC, interrupt-mode-safety for `I`), exposed as a
first-class solver decision variable.

**No prior art found for the lattice.** Massalin used SMC for specialization, not as a spill
destination an allocator chooses. llvm-mos models 6502 zero-page as "imaginary registers" — the
same instinct, one tier deep.

**Honest status: one real tier plus a wish-list.**

| Tier | Allocatable today? | In real output? |
|---|---|---|
| GPR A–L | yes | everywhere |
| **IX/IY halves** | **yes** | **32-35 of 142 files** |
| I / R | not declared anywhere | 0 |
| SMC slot | declared, then `NonAllocatable` | 0 of 314 files |
| Shadow / EXX | declared, then `NonAllocatable` | `EXX` never emitted |
| Stack | `InfCost` | ad-hoc pass only |
| Memory | yes | 11 of 142 files |

The costs are declared in `vir/z80.go:107-120` and removed at `:150`. There is no `LocTSMC` in
`pkg/mir2` at all. **`grep "tunnel"` over the Go sources returns nothing.**

**The honest sequence:**

1. Say so in the docs first (Roadmap §2.3). A seven-tier lattice we describe and do not implement
   is worse than a two-tier one we do.
2. **Land one more tier, properly.** Shadow registers via `EXX` are the best candidate: 4T for a
   batch swap of seven registers. They need a genuine *batch* constraint (`shadow_active(t)` gating
   all six jointly) rather than per-register moves that do not exist in the ISA — which is exactly
   why the current per-location model cannot express them. Doing this right is a real contribution:
   **an allocator whose storage tiers include ones that can only be switched as a set.**
3. Only then revisit SMC-as-spill. `pkg/mir2/tsmc_spill.go` already implements the idea and cannot
   fire; the 20T-vs-21T arithmetic that motivates it is sound and checkable.

Meanwhile, credit what is real: IX/IY halves as costed allocation targets with DD/FD legality
rules (a pressure test with 11 live values spilled to **zero** memory slots), and `EX AF,AF'` as a
scratch bracket for spill-to-spill moves in 10 of 142 files.

---

## Track D — Differential compile-time asserts *(most under-sold)*

`assert f(x) == y` written in the source, executed **at compile time by both the MIR2 VM and a
real cycle-accurate Z80 emulator**, with arguments placed using the actual register allocation.
1,366 asserts across eight language corpora.

The audits called this the strongest engineering idea in the repo, and it is mentioned nowhere in
`CLAUDE.md`. It is also the mechanism that found nearly every bug in reports 117–119 — when it was
pointed at Z80.

**Why it is more than a test harness.** It is a *differential oracle across abstraction levels*
built into the language: the same source text is executed by an IR interpreter and by silicon
semantics, and disagreement localizes the bug to the backend. For a compiler whose entire thesis is
"solvers produce optimal code on a hostile architecture", having the correctness oracle in the
language is the thing that makes the thesis testable at all.

**What it needs** (Roadmap §2.4): fix `buildAssertBootstrap` to read the emitted ABI rather than the
PBQP `AllocResult`, and move the corpus off `via mir2`. Then the metric to publish is not "524/524
asserts" but "N asserts executed on cycle-accurate hardware semantics across 8 source languages",
which is a genuinely unusual claim.

---

## Two smaller ideas worth keeping

**Range-refined types driving automatic tabulation.** `fun popcount(x: u8<0..255>)` → LUTGen
evaluates all 256 inputs on the MIR2 VM and replaces the body with a table lookup. ADR-0015 cites
Ada subtypes, SPARK and Zig honestly. This turns a manual retro technique into a type-system
feature. Small gap: emit `ALIGN 256` so it costs `LD L,A` instead of `ADD HL,BC`.

**Metafunctions in the target language.** ADR-0023: compile-time metafunctions are ordinary Nanz
functions run on the MIR2 VM, with typed AST introspection (`for node: ^BlockNode in nodes[0..n]`).
One language for compile time and runtime, no external interpreter. Strictly better than the
`@define`/`@lua` design `CLAUDE.md` still documents, and it works today.

---

## Sequencing

Tracks are gated on the Roadmap, not on each other:

| Track | Blocked by | Ready when |
|---|---|---|
| **B — PFCCO** | nothing but the benchmark protocol | Roadmap §2.1 (retract the figures) |
| **D — Differential asserts** | assert bootstrap ABI bug | Roadmap §2.4 |
| **A — Retrieval codegen** | format gap, hit counters, aliasing | Roadmap §3.2–3.3, §4.1 |
| **C — Spill lattice** | honest docs, then batch constraints | Roadmap §2.3, then §4 |

**Track B is closest to a paper. Track A is the most interesting result. Track D is the one that
makes either of them checkable — and it is the cheapest.** Start there.

---

## The framing to adopt

The current framing — *"three cooperating solvers produce provably optimal code"* — is not
demonstrated: the legacy PBQP path is correct on loops that both newer backends miscompile.

The defensible framing is stronger *because* it survives checking:

> Eight source languages share one Z80 backend, a differential compile-time assert harness that
> executes claims on cycle-accurate hardware semantics, and an experiment in replacing online
> register allocation with offline knowledge compilation on an architecture irregular enough that
> the constraint space compresses.

That is a real contribution, and every clause of it is something we can hand a reviewer.
