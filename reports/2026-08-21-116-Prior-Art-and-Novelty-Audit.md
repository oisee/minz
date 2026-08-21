# Prior-Art and Novelty Audit

**Date:** 2026-08-21
**Method:** adversarial review by a dedicated skeptic agent, briefed to find prior art and to be the honest counterweight to the project's own enthusiasm. Web sources cited inline. Load-bearing claims spot-verified independently against the working tree (see *Verification* below).
**Scope:** the backend claims in `CLAUDE.md` §4–6 and the `reports/` series.

---

## Why this document exists

The project's *research* documents are well-calibrated. `research/abi-paper/RELATED_WORK.md`,
`research/abi-paper/philipp-reply-2026-03-25.md` and `docs/2026-03-25-Peer-Feedback-Synthesis.md`
already anticipate most of what follows, including the −60% caveat, the IXH/IXL survey gap, and
the IR-vs-ISA confound in the signature-reuse figure.

The problem is not the research. **It is that `CLAUDE.md` and the reports carry the enthusiastic
version while the honest version lives in a letter to a correspondent.** Promoting those caveats
into the top-level docs resolves most of the items below at zero cost, because the correct
wording is already written.

---

## Verification status

Three load-bearing claims were re-checked directly rather than taken on the agent's word:

| Claim | Check | Result |
|---|---|---|
| `Entropy()` is dead code | `grep -rn "Entropy()" minzc/pkg/` | One hit: the definition at `wfc.go:40`. Zero call sites. **Confirmed.** |
| No backtracking in the LIR allocator | `grep -rniE "backtrack\|snapshot\|undo" minzc/pkg/lir/*.go` | Only a comment at `lir.go:276` reading "without backtracking". **Confirmed.** |
| Shipped regalloc table size | parsed `regalloc_table.json` | JSON list, **36 entries**. **Confirmed.** |

Remaining claims below are the agent's, with the file:line evidence it reported. They are
consistent with the three that were verified, but have not each been independently re-derived.

---

## 1. WFC as a register allocator — **RENAMING**

Wave Function Collapse is Gumin's 2016 reimplementation of Merrell's *model synthesis*
(i3D 2007); Merrell's own comparison states the two "use nearly the same algorithm."
Mechanically WFC is **arc consistency (AC-3) + a variable-ordering heuristic + backtracking on
contradiction**. Boris the Brave, the most-cited expositor, states plainly that "the underlying
solver WaveFunctionCollapse came with is called Arc Consistency 3."

The min-entropy rule is a probability-weighted variant of the standard CSP
**minimum-remaining-values (MRV / "fail-first")** heuristic — Haralick & Elliott, 1980. With
unweighted domains the two coincide exactly.

### What the code does

`minzc/pkg/lir/wfc.go` (1,052 LOC) and `wfc_interblock.go` (658 LOC):

- `Propagate()` runs five monotone `LocSet`-narrowing passes (forward, backward,
  vregConsistency, clobber, operandInterference) to fixpoint with a 20-iteration cap.
  **This is a correct, well-built arc-consistency engine.**
- `WFCCell.Entropy()` at `wfc.go:40` — *never called*.
- `Collapse()` is `for i := range s.Cells`, strict program order. Its own doc comment says
  "Sequential order: process instructions in program order." The inter-block variant uses
  fixed RPO.
- No backtracking. Contradictions trigger spill recovery; failure aborts the whole module to
  a PBQP fallback.

### Assessment

What is implemented is textbook AC-3 followed by greedy assignment with PBQP hints — a
legitimate allocator design. But the two features that distinguish WFC from generic constraint
propagation are respectively dead code and absent.

Our domains are unweighted register sets, so entropy ≡ domain size ≡ MRV. Even wired up, the
correct citation would be Haralick & Elliott 1980, not Gumin 2016.

The architecture doc already gives the game away: `Collapse | → | Sequential assignment with
pickPreferred(hints)`.

**Recommendation:** describe it as *"arc-consistency propagation over register-location bitsets,
with PBQP-derived value ordering."* That is accurate and unembarrassing. Keeping "WFC" invites a
reviewer to check for entropy ordering and backtracking, find neither, and discount everything
else.

**"Dimensions."** Inter-block propagation across CFG edges with a back-edge fixpoint is standard
iterative dataflow analysis. "Dimension 2" is branding. Any future "3D/4D/5D" framing should be
grounded in a real change of quantifier (intra-block → inter-block → inter-procedural →
whole-module co-design), not in vocabulary.

Sources: [Model synthesis](https://paulmerrell.org/model-synthesis/) ·
[MS vs WFC comparison](https://paulmerrell.org/wp-content/uploads/2021/07/comparison.pdf) ·
[WFC Explained](https://www.boristhebrave.com/2020/04/13/wave-function-collapse-explained/) ·
[Arc Consistency Explained](https://www.boristhebrave.com/2021/08/30/arc-consistency-explained/) ·
[AC-3](https://en.wikipedia.org/wiki/AC-3_algorithm)

---

## 2. PBQP → WFC hint passing — **NEW COMBINATION, weakly**

PBQP register allocation is Scholz & Eckstein (2002), extended to SSA by Hames & Scholz
(CC 2011), shipped in LLVM as `RegAllocPBQP` since 2008 — and explicitly designed for irregular
architectures.

Hint-passing between a global and a local allocator is ubiquitous: LLVM's greedy allocator
consumes copy hints; GCC's IRA is explicitly two-tier with preference propagation.

Using PBQP output as *value ordering* for a propagation-based repair pass is sensible
engineering, but thin as a research contribution — close to "run a heuristic, then legalize."
The interesting content is in what the second stage enforces (Z80 encoding constraints), not in
the fact that hints were passed.

Sources: [PBQP](https://www.complang.tuwien.ac.at/scholz/pbqp.html) ·
[SSA-Based RA with PBQP](https://link.springer.com/chapter/10.1007/978-3-642-19861-8_4) ·
[LLVM RegAllocPBQP](https://llvm.org/doxygen/RegAllocPBQP_8cpp_source.html)

---

## 3. Z3 joint isel + regalloc (VIR) — **NEW COMBINATION**

This is a crowded field:

- **Unison** (Castañeda Lozano, Carlsson, Hjort Blindell, Schulte; TOPLAS 2019) — constraint
  programming integrating register allocation with instruction scheduling, solving functions up
  to ~1,000 instructions optimally. This is what VIR does, one solver technology removed.
- **Goodwin & Wilken** (1996) — 0-1 ILP optimal global register allocation.
- **Appel & George**, *"Optimal spilling for CISC machines with few registers"* (PLDI 2001) —
  the title is almost our thesis statement; their target was x86 with 6 usable registers.
- **Krause** (CC 2013), *"Optimal Register Allocation in Polynomial Time"* — tree-decomposition
  DP handling register aliasing, and **the default allocator in SDCC's Z80 backend today.**

Our per-instruction encoding (`lv{vreg}_i{inst}`), so the solver plans moves as part of the
solution rather than as a fixup, is essentially Unison's per-operation temporary model.

**The technique is well-trodden; the target is not.** No published constraint/SMT-based joint
isel+regalloc backend for the Z80 was found. Applying Unison-class methods to an accumulator
machine with 7 GPRs and tied operands is a genuine, unoccupied niche — but the framing must be
"we apply known combinatorial codegen methods where nobody has," not "we invented joint solving."

**Missing citations a reviewer will ask for first:** Unison, Appel & George, Goodwin & Wilken.
`RELATED_WORK.md` cites Krause, Wall, Chow, LLVM IPRA, MSVC `/GL`, Scholz & Eckstein — good for
PFCCO, but **none** of the joint-solving literature.

Sources: [Unison (TOPLAS)](https://arxiv.org/pdf/1804.02452) ·
[Survey on Combinatorial RA & IS](https://arxiv.org/pdf/1409.7628) ·
[Optimal spilling for CISC machines](https://dl.acm.org/doi/10.1145/381694.378854) ·
[Krause CC 2013](https://link.springer.com/chapter/10.1007/978-3-642-37051-9_1)

---

## 4. Z3-PFCCO — **NEW COMBINATION, with a real delta**

The best-researched claim in the project; `RELATED_WORK.md` already maps it correctly.
Prior art: Wall (1986), Chow (PLDI 1988), De Bus et al. (2004), Caldwell & Chiba (2017),
LLVM IPRA (2016), GCC `-fipa-ra`, MSVC `/GL`, and Krause's
*Efficient Calling Conventions for Irregular Architectures* (arXiv 2112.01397) — which found
improvements large enough that **SDCC broke ABI compatibility to adopt them** (`__sdcccall(1)`,
default since 4.1.12).

The claimed delta is precise and correct: Krause selects **one globally optimal convention per
architecture**; Wall/Chow/De Bus do per-function conventions on **regular RISC register files**.
Solving per-function *register-class* assignment jointly across all call sites on an irregular
register file appears unoccupied.

**But "The SDCC killer" in `CLAUDE.md` must not survive contact with a reviewer.** SDCC's Z80
backend ships Krause's optimal allocator *and* an empirically-searched calling convention from
the same author — the strongest 8-bit codegen baseline in existence — and its maintainer is
already a correspondent on this project. The line is inaccurate and needlessly antagonistic.
Position as complementary, which `RELATED_WORK.md` already does: PFCCO sets ABIs, Krause-style
allocation runs per-function underneath.

Sources: [Krause, calling conventions](https://arxiv.org/abs/2112.01397) ·
[Chow, PLDI 1988](https://www.cs.tufts.edu/~nr/cs257/archive/fred-chow/p85-chow.pdf) ·
[Wall](https://dl.acm.org/doi/10.1145/93548.93551)

---

## 5. O(1) regalloc via precomputed table — **NEW, methodologically**

Closest analogue: **Bansal & Aiken, *Automatic Generation of Peephole Superoptimizers***
(ASPLOS 2006) — exhaustive offline enumeration, canonicalization into a signature, and an
"indexed structure for efficient lookup… much closer to a search engine database than to a
traditional optimizer." Precisely our architecture, applied to peepholes rather than allocations.

No prior work was found precomputing a lookup table of *optimal register allocations* keyed on
interference-graph shape.

### The genuinely interesting result

From `~/dev/z80-optimizer/data/README.md`: exhaustive enumeration for ≤5 vregs (17.5M entries;
4v 78.9% feasible, 5v 67.7% feasible), and feasibility collapsing to **0.9% at 6 vregs** — the
Z80 register file measurably saturates. Plus: 1,605 functions across 8 frontends collapse to
**312 unique allocation signatures** (80% reuse), **88.2%** transferring to a disjoint program set.

The hypothesis worth publishing: **architecture irregularity makes the constraint space *more*
compressible, not less**, because fewer assignments are legal. This predicts a phase transition
in "tableability" as location count grows, and explains why nobody tried this on RISC. It is
falsifiable and has an experimental protocol attached. The project already states the right
caveat — 80% reuse may reflect *our IR's* constraint vocabulary rather than the Z80 ISA — which
is exactly what makes the proposed SDCC cross-validation load-bearing.

### But the published numbers do not hold up

| Published | Reality |
|---|---|
| "37.6M precomputed entries" | Appears once, credited to a sibling repo. That repo documents **83.6M** (4v 156K, 5v 17.4M, 6v-dense 66.1M). 37.6M matches neither figure nor any subtotal. A fourth number (298.7M) appears in a code comment. |
| Shipped table | `regalloc_table.json` = 5,941 bytes, **36 entries** (verified). |
| Report 111 "61 entries" | Report 111 says 59; Report 109 says 61; commit `37e5affa` says 59; the file has 36. Four numbers for one table. |
| Enriched tables auto-loaded | `regalloc_table.go:251-322` looks for `enriched_regalloc.jsonl`, `enriched_4v.z80t`, `merged_ix_5v.bin`, `ix_expanded_6v_dense.bin`. **None exist on this machine.** What exists is `exhaustive_*.bin.zst` — zstd-compressed, while `LoadEnrichedBinary` does a plain `os.Open`. Every load loop `continue`s silently on error, so **the O(1) path is inert in this checkout.** |
| "91% of real functions hit" | **Not a hit rate.** It is static corpus *eligibility* analysis (functions with ≤6 vregs or cut-vertex-decomposable), from `reports/2026-03-28-session-13-O1-regalloc.md:117-122`. **No hit/miss counters exist anywhere in the compiler.** That report's own "What's Next" concedes the pipeline was not yet wired when 91% was published. |

Sources: [Bansal & Aiken](https://theory.stanford.edu/~aiken/publications/papers/asplos06.pdf) ·
[STOKE](https://theory.stanford.edu/~aiken/publications/papers/asplos13.pdf) ·
[Souper](https://arxiv.org/pdf/1711.04422)

---

## 6. Spill-tier lattice (IXH/IXL + I/R + SMC) — **NEW COMBINATION, one new element**

**IXH/IXL/IYH/IYL** are decades-old Z80 folklore. For compilers: SDCC feature request #791 asked
for them; SDCC snapshots beyond 4.2 allow IYL/IYH via eZ80 encodings; the **z80n target
officially supports iyl/iyh and SDCC uses them there** to skip stack usage. An SDCC developer
estimated the general-purpose payoff at "0.1% improvement in code size."

> **`CLAUDE.md`'s "No existing Z80 compiler uses IX/IY halves for register allocation" is stated
> flatly, twice (§4 and `LIR_Backend_Architecture.md`), and is contradicted by SDCC's z80n
> support.** This is the single most falsifiable sentence in our docs.
>
> Use the wording we already wrote to Krause: *"we are not aware of mainstream Z80 compilers
> using IXH/IXL as general-purpose spill locations, but we have not done a systematic survey."*

**Extended storage as an allocator tier** is what **llvm-mos** does for the 6502: zero-page
locations modelled as "imaginary registers" fed to LLVM's normal allocator. Same idea, different
substrate.

**SMC as storage:** Massalin's **Synthesis kernel** (1988–92) is the canonical citation —
runtime code generation that "overwrote specific bytes in the generated code with pointers."
Immediate-field patching on 6502/Z80/x86 is documented folklore. What was *not* found is prior
work where **the register allocator itself models an SMC immediate field as a cost-ranked spill
destination and the solver chooses it.**

**The novelty is the lattice, not the locations.** Compilers normally have two tiers (register,
stack slot). A seven-tier cost-ranked lattice — L0 GPR → L1 IX/IY halves → L2 I/R → L3 SMC
tunnel → L3b shadow → L4 stack → L5 memory — with per-tier T-state costs *and per-tier safety
predicates* (recursion-safety for SMC, interrupt-mode-safety for `I`) exposed as a first-class
solver decision variable is not done elsewhere. The T-state arithmetic (SMC tunnel 20T beats
PUSH/POP 21T for 8-bit; PUSH/POP 21T beats 44T two-byte SMC for pairs) is exactly the concrete,
checkable result that makes it credible.

Sources: [Z80 undocumented](http://www.z80.info/z80undoc.htm) ·
[SDCC FR #791](https://sourceforge.net/p/sdcc/feature-requests/791/) ·
[SDCC z80 wiki](https://sourceforge.net/p/sdcc/wiki/z80/) ·
[llvm-mos codegen](https://llvm-mos.org/wiki/Code_generation_overview) ·
[The Synthesis Kernel](https://www.usenix.org/legacy/publications/compsystems/1988/win_pu.pdf)

---

## 7. "−60% vs SDCC" — real measurement, indefensible headline

To our credit the SDCC output is genuine and committed: 11 `.asm` files in
`research/abi-paper/sdcc-output/` with real SDCC 4.2.0 #13081 headers, plus their `.c` sources.
Nothing is fabricated. The *presentation* is the problem:

1. **n = 5 functions** — abs_diff, gcd, minmax, fib, swap. 131 vs 51 instructions.
2. **Metric is instruction count**, not bytes and not T-states. On Z80 these diverge sharply
   (a `DD`-prefixed IXH op is 2 bytes / 8T; `LD A,(nn)` is 3 bytes / 13T).
3. **Two of five comparisons are not semantically equivalent.** `swap` (−90%) and `minmax`
   (−81%) use C pointer out-params against MinZ register multi-return. The C source itself
   carries the comment `// SDCC can't return structs — must use out-params or globals`.
   **Excluding those two: 51 vs 38 → −25%.**
4. **No optimization flags.** Committed asm shows `.optsdcc -mz80`; `EXAMPLES.md` documents
   `sdcc -mz80 --std-c11 -S`. No `--opt-code-size`, no `--max-allocs-per-node` — SDCC's principal
   code-size knob, and the one driving Krause's optimal allocator. A reviewer reads this as
   comparing against SDCC with optimization off.
5. **SDCC 4.2.0 is from 2022**; current is 4.5, and the maintainer has already told us trunk
   output is tighter.
6. **No reproducible script.** Nothing in the repo regenerates these numbers.
7. **Three incompatible values circulate:** −61% (`vir-e2e-report.md`, 131→51), −60%
   (`CLAUDE.md`), −71% (Report 109, 131→38). `abs_diff` is recorded as a 12–12 tie in one place
   and 4 vs 12 in another.

The C89 benchmark (Report 104, "−81%") is worse: **112 of the 145-instruction delta comes from
four functions that MinZ constant-folds to `LD HL,55 / RET`.** That measures interprocedural
constant propagation, not codegen quality.

**One item to check before publishing anything:** Report 109 lists `add` compiling to
`ADD A, C / RET`, but the source declares `int add(int a, int b)`. If we narrow C `int` to 8
bits, the comparison against SDCC's conformant 16-bit `int` is invalid and the C17-conformance
claims need revisiting.

---

## 8. "Rewrite Triad" — **RENAMING** (ISLE, Datalog) / **overstated** (Grace)

ISLE is Cranelift's by name and design; our architecture doc's "inspired by Cranelift" is the
right attribution. Datalog-over-IR is bddbddb (Whaley & Lam), Doop, Soufflé. Declarative graph
queries over compiler IR is **CodeQL/Semmle .QL**.

The **"Cypher-equivalent"** claim does not hold. In `minzc/pkg/rewrite/grace/`: 3 match kinds
(`MatchBlock`, `MatchEdge`, `MatchNode`), 8 predicates, 7 actions — a fixed vocabulary. Missing
versus openCypher: variable-length paths, `RETURN`/projections, `WITH`, `ORDER BY`/`LIMIT`, real
aggregation, `OPTIONAL MATCH`, arbitrary property maps, `CREATE`/`MERGE`/`SET`, path variables,
the type system. **12 tests, not 13**, run against a hand-rolled `mockGraph` rather than real IR;
the "7 Cypher categories" are hand-written comments asserting analogy, not a conformance suite.
**1,051 non-test LOC**, not 1,255.

Other files already hedge correctly — `lir/rules.go:10` says "Cypher-**like**", and
`LIR_Backend_Reference.md` marks the row "Declared". Adopt that wording everywhere, and cite
CodeQL as prior art.

Sources: [Cranelift ISLE](https://cfallin.org/blog/2023/01/20/cranelift-isle/) ·
[egg (POPL 2021)](https://dl.acm.org/doi/pdf/10.1145/3434304) ·
[Doop / Datalog](https://yanniss.github.io/doop-datalog2.0.pdf) ·
[CodeQL CFG library](https://codeql.github.com/codeql-standard-libraries/cpp/semmle/code/cpp/controlflow/ControlFlowGraph.qll/module.ControlFlowGraph.html)

---

## Where the defensible novelty actually is

Strip the naming and four things survive with no prior art found. Ranked by strength.

### 1. Precomputed exhaustive allocation tables + the compressibility result

Not the GPU — that is an implementation detail, and offline-enumeration-into-a-table is Bansal &
Aiken's architecture. What is new is the **empirical structural result**: 312 signatures for
1,605 functions, 88.2% transfer, and the measured feasibility cliff from 67.7% (5v) to 0.9% (6v).
The inverted-intuition hypothesis — irregularity increases compressibility — is falsifiable,
predicts a phase transition, and explains why this was never tried on RISC.
**Get the SDCC cross-validation and this is a paper.**

### 2. The spill-tier lattice as a solver decision variable

Seven cost-ranked tiers with safety predicates, SMC immediate fields as an allocator-visible
spill destination. Massalin used SMC for specialization, not as an allocator target.

### 3. Per-function calling-convention co-design on an irregular register file

The gap between Krause (one global convention) and the link-time per-function work (regular RISC
files) is real and correctly identified in `RELATED_WORK.md`.

### 4. The target itself

Unison/Appel-George-class combinatorial codegen on a 7-register accumulator machine with tied
operands and DD/FD prefix conflicts is unoccupied — as an *engineering case study*: "here is what
optimal codegen looks like on the most hostile architecture still in use, and where it breaks."

### Not defensible as novelty

WFC (AC-3 + MRV, minus the MRV), PBQP (Scholz & Eckstein 2002), ISLE (Cranelift), Datalog
(bddbddb/Doop/CodeQL), Grace-as-Cypher, SMT-for-codegen in general (Unison), and IXH/IXL as
merely *existing*.

---

## Claims needing softening, ordered by damage

### Would cause immediate reviewer rejection

| # | Claim | Problem | Fix |
|---|---|---|---|
| 1 | "Wave Function Collapse register allocator" | `Entropy()` dead; collapse is program-order/RPO; zero backtracking. The name promises exactly what is missing. | Rename to "arc-consistency register allocation with PBQP value ordering." Cite AC-3 and MRV. |
| 2 | "−60% vs SDCC" | n=5; instruction count; 2/5 semantically inequivalent (excluding them −25%); no opt flags; SDCC 4.2.0; no script; three conflicting values. | State n, metric, flags, version. Report bytes *and* T-states. Split out the multi-return cases as "features C cannot express" — `EXAMPLES.md` already does this correctly. Ship a script. |
| 3 | "948/948 pipeline completion (100%)" | `Match: r.OK` means "codegen returned no error" (`pipeline.go:505`); failing files are `continue`d out of the denominator (`lir_test.go:44,54`). 948 = ~237 funcs × 4 **abstract** machines, not Z80. The test never calls `t.Error` — it always passes. | Rename to "compiles without error." Report skips. Never write "100%" for a denominator that excludes failures. |
| 4 | "No existing Z80 compiler uses IX/IY halves" | Contradicted by SDCC z80n. Trivially falsifiable. | Use our own Krause-letter wording, or do the survey. |

### Would draw "citation needed"

| # | Claim | Problem | Fix |
|---|---|---|---|
| 5 | "37.6M precomputed entries" | Four different numbers in circulation; shipped table has 36. | Name the table, artifact, entry count, coverage. |
| 6 | "O(1) regalloc for 91% of corpus" | Eligibility analysis, not a hit rate. No counters exist. Loader's target files absent, so the path is inert here. | Say "eligible for lookup." Add counters. Ship or document the tables. |
| 7 | "97.9% VM-verified" | Appears only in `CLAUDE.md` and a weekly. Measured figures are 94.6% and 96.1%. | Use a measured number or drop it. |
| 8 | "645/645 (100%), zero PBQP fallback" | Retroactively invalidated: `reports/2026-03-28-session-13-O1-regalloc.md:45` says all 645 were compiled leaf-only with CALL args never loaded. The milestone was declared over code that mis-compiled every non-leaf function. | Restate post-fix. This is the clearest evidence that coverage ≠ correctness. |
| 9 | "55/55 Z80-verified" | The best evidence we have — genuinely executed on a cycle-accurate emulator. But ~19 hand-written cases, and allocation comes from **PBQP** via `FuncParamLocs`, not the Z3 solver (`assert_test.go:37-86`). | Say so precisely, then grow it. This is the metric worth investing in. |
| 10 | "Grace: Cypher-equivalent, 1255 LOC, 13 tests" | 3/8/7 vocabulary; no paths, `RETURN`, `WITH`, aggregates; 12 tests on a mock graph; 1,051 LOC. | "Cypher-**like**", fix counts, cite CodeQL. |
| 11 | "The SDCC killer" | SDCC ships Krause's optimal allocator and searched conventions; the maintainer corresponds with us. | Delete. Position as complementary. |
| 12 | Report 104's "−81%" | 112 of 145 instructions of delta are constant-folding. | Separate consteval wins from codegen wins. |
| 13 | Missing joint-solving citations | Nothing from Unison / Appel & George / Goodwin & Wilken / Bansal & Aiken / STOKE / Souper / egg / model synthesis. | Add them. The PFCCO section shows we can do this well. |

---

## The one structural recommendation

Promote the "Honest Caveats and Limitations" section of
`research/abi-paper/philipp-reply-2026-03-25.md` into `CLAUDE.md` and the report headers. It
already anticipates most of items 1–13. The correct wording exists; it is just filed in the
wrong place.
