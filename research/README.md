# MinZ VIR Research Papers

Provably optimal register allocation for Z80 via GPU exhaustive search, Z3 SMT solving, and compositional decomposition.

## Papers

### Paper A: Precomputed Optimal Register Allocation via GPU Exhaustive Search
**[paper-a-draft.md](paper-a-draft.md)**

The foundation: exhaustive GPU enumeration of all possible register assignments for small functions. 156,506 shapes solved for ≤4v (40 seconds), 17.2M for ≤5v (overnight). 315 unique constraint signatures from a 1,645-function corpus (80.9% reuse, 88.2% cross-program transfer). 21.1% of shapes are provably infeasible — negative certificates that save the compiler from fruitless search.

**Key result:** Register allocation on Z80 is a solved game for ≤5 virtual registers. The compiler looks up the answer in O(1).

### Paper B: Cross-Function Merging via Island-of-Optimality Decomposition
**[paper-b-outline.md](paper-b-outline.md)**

Extends Paper A across function boundaries. Merge caller+callee into a single GPU optimization problem — eliminates CALL/RET overhead (~35T per boundary). 5 merges save 210 T-states (36%) on ZSQL. For large functions: island decomposition at liveness bottlenecks, each island solved optimally, stitched with bounded shuffle cost. CPU backtracking with 745,000x interference pruning solves 15v islands in <1 second.

**Key result:** Every eliminated call boundary saves exactly ~35T. Composition is linear.

### Paper C: Compositional Register Allocation — Decomposing Large Problems from Solved Atoms
**[paper-c-compositional-regalloc.md](paper-c-compositional-regalloc.md)**

The meta-insight: instead of enumerating all 1.9B 6v shapes (infeasible), COMPOSE solutions from the complete ≤5v table. Split interference graphs at cut vertices, look up each half in the table, stitch with bounded cost. 76.8% of 5v shapes are decomposable; ~77% at 6v (extrapolated). Three strategies: graph-cut, treewidth-bounded DP (exact for tw≤4), shape alteration (spill-to-fit).

**Key result:** The ≤4v table (156K entries) is the "alphabet." All larger problems are "sentences" composed via graph decomposition.

## The Three-Paper Arc

```
Paper A: The Atoms        Paper B: The Molecules      Paper C: The Grammar
(exhaustive table)        (cross-function merge)       (compositional rules)
                          (island decomposition)
     │                          │                           │
     ▼                          ▼                           ▼
  ≤5v: O(1) lookup     Functions: merge or split      ≤6v+: compose from
  156K-17.2M entries    35T per boundary saved          solved sub-shapes
  21% infeasible        184T/183T proven optimal        77% decomposable
```

## Data

| Dataset | Location | Description |
|---------|----------|-------------|
| Exhaustive ≤4v | z80-optimizer `research/paper-a/data/` | 156,506 shapes, GPU-solved |
| Exhaustive ≤5v | z80-optimizer (overnight run) | 17.2M shapes |
| Corpus signatures | `minzc/pkg/vir/regalloc_exhaustive.json` | 56 entries (subset) |
| ZSQL call graph | `VIR_DUMP_CALLGRAPH` output | 31 functions |
| Merge candidates | `VIR_DUMP_MERGED` output | 5 pairs, 210T savings |
| Island sub-problems | `VIR_DUMP_ISLANDS` output | 10 islands, all ≤15v |
| C89 signatures | `/tmp/c89_signatures.json` | 292 functions → 129 unique |
| Decomposition analysis | z80-optimizer | 76.8% decomposable at 5v |

## Companion Documents

- [Anytime-Optimal Register Allocation](../docs/Anytime_Optimal_Register_Allocation.md) — 5-level graceful degradation pipeline (technical writeup with diagrams)
- [ADR-0040: Island-of-Optimality](../docs/adr/0040-island-of-optimality-regalloc.md) — Architecture decision record
- [Philipp Krause letter](abi-paper/) — Correspondence with SDCC maintainer

## Reproducibility

All results are reproducible via environment variables:

```bash
# Dump GPU constraint signatures
VIR_DUMP_GPU_BATCH=1 ./minzc program.nanz --vir

# Dump call graph
VIR_DUMP_CALLGRAPH=callgraph.json ./minzc program.nanz --vir

# Dump merged caller+callee problems
VIR_DUMP_MERGED=merged.json ./minzc program.nanz --vir

# Dump per-point liveness
VIR_DUMP_LIVENESS=liveness.json ./minzc program.nanz --vir

# Dump island sub-problems (VIR-level splitting)
VIR_DUMP_ISLANDS=islands.json ./minzc program.nanz --vir

# Enumerate all constraint shapes
vir-enumerate --max-vregs=4 | z80_regalloc --server > results.jsonl
```

GPU kernel: [z80-optimizer](https://github.com/oisee/z80-optimizer) — `z80_regalloc --server`
Compiler: [MinZ](https://github.com/oisee/minz) — `--vir` flag
