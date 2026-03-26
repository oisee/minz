# MinZ Research Papers

Provably optimal code generation for Z80 via GPU exhaustive search, Z3 SMT solving, compositional decomposition, and instruction superoptimization.

**Author:** Alice Vinogradova
**AI Collaborators:** Claude Opus 4.6 (VIR backend, papers), GPT-5.4 (theoretical review)
**Infrastructure:** [z80-optimizer](https://github.com/oisee/z80-optimizer) (CUDA GPU kernels), [MinZ](https://github.com/oisee/minz) (compiler)

---

## Papers

### Paper A: Precomputed Optimal Register Allocation via GPU Exhaustive Search
**[paper-a-draft.md](paper-a-draft.md)** | Primary: VIR session + z80-optimizer

83.6 million register allocation shapes solved exhaustively through ≤6 virtual registers (32MB compressed table). 315 unique constraint signatures from a 1,645-function corpus (80.9% reuse). Feasibility cliff: 95.9% at 2v → 0.9% at 6v. The compiler looks up the answer in O(1) — no solver at compile time.

**Key numbers:** 83.6M entries, 32MB, ≤6v complete, 88.2% cross-program transfer, 61% of 6v infeasible.

**Who built what:**
- VIR session (minz-vir): constraint signature extraction, GPU descriptor format, table integration, direct PIR emit, Paper A text
- z80-optimizer: CUDA exhaustive kernel, CPU backtracking solver (745,000x pruning), exhaustive enumeration, data files
- Main session (minz): corpus compilation, SDCC comparison, cross-compiler analysis

---

### Paper B: Cross-Function Merging via Island-of-Optimality Decomposition
**[paper-b-outline.md](paper-b-outline.md)** | Primary: VIR session + z80-optimizer

Cross-function register allocation via GPU: merge caller+callee into single optimization, eliminating CALL/RET overhead (~35T per boundary). 5 merges save 210T (36%) on ZSQL. Island decomposition at liveness bottlenecks for functions >15v. CPU backtracking with 745,000x pruning.

**Key numbers:** 210T savings (36%), 35T invariant per boundary, 184T/183T proven optimal for main/_prompt.

**Who built what:**
- VIR session: call graph dump, merged GPU problems, liveness dump, island splitter (VIR-level), Paper B text
- z80-optimizer: GPU solving of merged pairs, backtracking solver, partopt prototype, island cost computation
- Main session: ZSQL end-to-end validation (SQL on ZX Spectrum)

---

### Paper C: Compositional Register Allocation from Solved Atoms
**[paper-c-compositional-regalloc.md](paper-c-compositional-regalloc.md)** | Primary: VIR session + z80-optimizer + GPT-5.4

Can 6v problems be composed from solved ≤5v atoms? 13.2M composition verified: zero misses, max 12T overhead. Graph separator decomposition, treewidth-bounded DP, shape alteration. Honest correction: compiler-generated graphs are denser than random (53.7% tw≥4 for dense corpus functions).

**Key numbers:** 13.2M verified, 0 misses, max 12T overhead, 99.5% random-graph tractable (46% for dense corpus).

**Who built what:**
- VIR session: composition algorithm, island sub-problem generation, Paper C text
- z80-optimizer: 5v→4v composition verification (13.2M lookups), treewidth computation, decomposability analysis
- GPT-5.4: graph separator theory, treewidth references, theoretical framework review

---

### Paper D: Constant Multiplication Superoptimization (planned)
Primary: z80-optimizer

GPU exhaustive search for optimal constant multiplication sequences. 164 8-bit solutions (A×K→A, clobber-annotated), 254 16-bit solutions (A×K→HL, complete). Pool reduction: 21→3 ops for mul16 (13,600× search space reduction). div3 lower bound ≥9 instructions (search certificate).

**Key numbers:** 254/254 mul16 complete (30s), 164/254 mul8 at len≤9, 3-op basis for all 16-bit multiplies.

---

## The Four-Paper Arc

```
Paper A: The Atoms          Paper B: The Molecules
(83.6M exhaustive table)    (cross-function merge + islands)
        │                           │
        ▼                           ▼
  ≤6v: O(1) lookup          Functions: merge or split
  83.6M entries, 32MB        35T per boundary saved
  61% infeasible at 6v       210T total savings (36%)

Paper C: The Grammar        Paper D: The Instructions
(compositional rules)       (multiply superoptimization)
        │                           │
        ▼                           ▼
  Compose from ≤5v table     164 mul8 + 254 mul16
  13.2M verified, 0 miss     Clobber-annotated for Z3
  99.5% random tractable     3-op basis (13,600× reduction)
```

## Data

| Dataset | Size | Location | Description |
|---------|------|----------|-------------|
| Exhaustive ≤6v | 83.6M / 32MB | z80-optimizer `data/` | Complete table through 6v |
| Exhaustive ≤4v | 156K | z80-optimizer `research/paper-a/data/` | Subset, first computed |
| Exhaustive ≤5v | 17.4M | z80-optimizer `data/` | Complete through 5v |
| 6v dense | 66.1M | z80-optimizer `data/` | 6v shapes requiring GPU |
| Corpus signatures | 315 | `minzc/pkg/vir/regalloc_exhaustive.json` | Compiler-shipped subset |
| mul8 clobber | 164 | z80-optimizer `data/mulopt8_clobber.json` | A×K→A with clobber masks |
| mul16 complete | 254 | z80-optimizer `data/mulopt16_complete.json` | A×K→HL, 3-op basis |
| ZSQL call graph | 31 funcs | `VIR_DUMP_CALLGRAPH` output | Cross-function analysis |
| Merge candidates | 5 pairs | `VIR_DUMP_MERGED` output | 210T savings verified |
| C89 signatures | 292→129 | `/tmp/c89_signatures.json` | 55.8% reuse |
| Composition | 13.2M | z80-optimizer | 5v→4v verified, 0 misses |

## Session Collaboration Map

```
┌─────────────┐     dedelulu      ┌──────────────────┐
│  minz-vir   │◄────────────────►│   z80-optimizer   │
│ VIR backend │  GPU descs,       │  CUDA kernels,    │
│ Z3 solver   │  signatures,      │  backtracking,    │
│ Papers A-C  │  island problems  │  exhaustive data  │
└──────┬──────┘                   └────────┬─────────┘
       │ dedelulu                          │ dedelulu
       ▼                                   ▼
┌─────────────┐                   ┌──────────────────┐
│    minz     │                   │   minz-abap      │
│ Main repo   │                   │  ABAP frontend   │
│ Frontends   │                   │  outparam/QTT    │
│ @error,ZSQL │                   │  swap/minmax     │
└─────────────┘                   └──────────────────┘
```

## Companion Documents

- [Anytime-Optimal Register Allocation](../docs/Anytime_Optimal_Register_Allocation.md) — 5-level graceful degradation pipeline
- [VIR Reliability Sprint](../docs/VIR_Reliability_Sprint.md) — P1-P5 bug fixes, E2E test plan, graduation criteria
- [ADR-0040: Island-of-Optimality](../docs/adr/0040-island-of-optimality-regalloc.md) — Architecture decision record
- [TSMC Tunnels](../CLAUDE.md#tsmc-tunnels-register-preservation-across-calls) — Self-modifying code spill tier

## Reproducibility

```bash
# Compile with VIR (uses precomputed table)
./minzc program.nanz --vir

# Dump GPU constraint signatures
VIR_DUMP_GPU_BATCH=1 ./minzc program.nanz --vir

# Dump call graph for cross-function analysis
VIR_DUMP_CALLGRAPH=callgraph.json ./minzc program.nanz --vir

# Dump merged caller+callee GPU problems
VIR_DUMP_MERGED=merged.json ./minzc program.nanz --vir

# Dump per-point liveness for island decomposition
VIR_DUMP_LIVENESS=liveness.json ./minzc program.nanz --vir

# Dump island sub-problems (VIR-level splitting)
VIR_DUMP_ISLANDS=islands.json ./minzc program.nanz --vir

# Enumerate all constraint shapes for GPU solving
vir-enumerate --max-vregs=6 | z80_regalloc --server > results.jsonl

# Strict mode (development — all warnings are errors)
VIR_STRICT=1 ./minzc program.nanz --vir  # default ON
VIR_STRICT=0 ./minzc program.nanz --vir  # production mode
```
