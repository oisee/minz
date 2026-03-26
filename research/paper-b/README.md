# Paper B: Exact Inlining via Exhaustive Cost Oracle

**Optimal Program Partitioning across Function Boundaries**

---

## Thesis

When a compiler has an exact cost oracle for register allocation (GPU exhaustive
table), inlining decisions become provably optimal: compare `cost(merged)` vs
`cost(A) + cost(B) + shuffle`. No heuristics, no thresholds. The call graph
partition that minimizes total cost can be found via bottom-up DP.

## Key Insight

Traditional inlining (GCC, LLVM) uses heuristic cost estimates. We use
**exact costs from the GPU table**. For each pair of caller/callee:

```
cost(separate) = table[sig(A)] + table[sig(B)] + shuffle_moves(A→B)
cost(merged)   = table[sig(A ∪ B)]
if cost(merged) < cost(separate): inline
```

Both lookups are O(1). The decision is provably optimal.

## Architecture

```
Call Graph → Topological Sort → Bottom-up DP
  For each edge (caller → callee):
    - Compute sig(merged) from GPU table
    - Compare vs sig(caller) + sig(callee) + shuffle
    - If merged wins: inline
  Prune by K constraint (merged vregs ≤ table max)
```

## Connection to Paper A

Paper A provides the cost oracle. Paper B consumes it:

| Paper A | Paper B |
|---------|---------|
| GPU exhaustive table | Cost oracle input |
| 315 signatures | Function costs |
| Phase transition | Tractability boundary for merging |
| O(1) lookup | O(1) inline decision |

## Relation to PFCCO

Z3-PFCCO (Per-Function Calling Convention Optimization) is a special case:
choosing the optimal calling convention for each function = choosing the
optimal boundary registers. Paper B generalizes this to choosing the optimal
function boundaries themselves.

## Experiments (Planned)

### Exp 1: Pairwise Merge Costs
For all call edges in the ZSQL corpus, compute merged vs separate costs.
Expected: ~30% of edges benefit from inlining.

### Exp 2: DP Partition
Run bottom-up DP on the full call graph. Compare total T-states:
- No inlining (SDCC-style)
- Heuristic inlining (GCC-style threshold)
- Optimal inlining (our DP + GPU oracle)

### Exp 3: Island Decomposition
For functions >8 vregs: split at call boundaries into islands ≤K vregs.
Each island solved optimally from GPU table. Join cost = shuffle moves.

## Data Files (To Generate)

| File | Description |
|------|-------------|
| `data/call_graph.json` | Full MinZ corpus call graph |
| `data/pairwise_costs.csv` | Merge vs separate for each edge |
| `data/dp_partition.json` | Optimal partition result |
| `data/island_splits.csv` | Island decomposition statistics |

## Citation

```bibtex
@inproceedings{minz2026inlining,
  title={Exact Inlining via Exhaustive Cost Oracle:
         Optimal Program Partitioning across Function Boundaries},
  author={Alice and Claude and Contributors},
  year={2026}
}
```
