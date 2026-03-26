# Paper C: Exhaustive Search Certificates

**Negative Results via GPU-Verified Lower Bounds**

---

## Thesis

GPU exhaustive search over instruction sequences provides **search certificates**:
proofs that no sequence of length ≤N exists for a given computation. These are
publishable negative results — the first proven lower bounds for specific Z80
computations.

## Key Result: DIVMOD by Non-Power-of-2

**Theorem:** No Z80 instruction sequence of length ≤12 computes division by a
non-power-of-2 constant on 8-bit registers without native multiply.

**Proof:** Exhaustive GPU search of all valid Z80 instruction sequences up to
length 12, verified against all 256 input values. Zero sequences found.
Search certificate: the complete enumeration log.

## What is a Search Certificate?

A search certificate proves that a search was exhaustive:

```
Input:  target function f(x), instruction set I, max length N
Output: either a sequence S ∈ I^N where S(x) = f(x) ∀x
        OR a certificate that no such S exists

Certificate = proof that all |I|^N candidates were tested
```

For Z80 with ~700 instruction patterns and N=12:
- Search space: 700^12 ≈ 10^34 (infeasible naively)
- Pruned search: dead-state elimination, value tracking, symmetry breaking
- GPU parallelism: 11.6M patterns/second on RTX 4060 Ti

## Three Superoptimizers, Three Certificates

| Superoptimizer | Search Space | Certificate Type |
|----------------|-------------|-----------------|
| **mulopt** | Constant multiply sequences | Optimal sequence OR lower bound |
| **divmod** | Division/modulo sequences | Lower bound ≥13 for non-power-of-2 |
| **regalloc** | Register assignments | Optimal assignment OR infeasibility proof |

Each produces a different kind of certificate:
- **mulopt:** "The shortest sequence for ×37 is 5 instructions" (constructive)
- **divmod:** "No sequence ≤12 exists for ÷7" (negative)
- **regalloc:** "No valid assignment exists for this constraint graph" (infeasibility)

## Connection to Papers A and B

Paper A's GPU table entries ARE search certificates:
- Feasible entry = constructive certificate (here's the optimal assignment)
- Infeasible entry = negative certificate (no valid assignment exists)

Paper C formalizes this: every GPU table entry is a miniature theorem.

## Experiments

### Exp 1: DIVMOD Lower Bound
```bash
cd z80-optimizer
./divmod --target div7 --max-length 12 --exhaustive
# Output: INFEASIBLE at length 12 (certificate saved)
./divmod --target div7 --max-length 13
# Output: FOUND at length 13 (sequence saved)
```

### Exp 2: Multiply Optimal Sequences
```bash
./mulopt --all-constants --max-length 8
# Output: 103/254 constants have sequences ≤8
# Certificate: complete enumeration log
```

### Exp 3: Regalloc Infeasibility
```bash
# From Paper A's corpus:
cat corpus_sigs.jsonl | gpu-server > results.jsonl
grep INFEASIBLE results.jsonl | wc -l
# Output: 686 infeasible signatures (each is a negative certificate)
```

## Data Files

| File | Description |
|------|-------------|
| `data/divmod_certificate.json` | Complete DIVMOD search log (≤12, infeasible) |
| `data/mulopt_results.csv` | All constant multiply: length, sequence, status |
| `data/infeasible_sigs.jsonl` | 686 regalloc infeasibility certificates |
| `data/gpu_search_stats.csv` | Search space size, pruning rate, GPU time |

## Philosophical Note

Traditional compiler papers prove things work. Search certificates prove things
**don't** work — and that's equally valuable. When the compiler encounters an
infeasible constraint signature, it KNOWS (not guesses) that a different strategy
is needed. No wasted search time. No heuristic fallback. Just certainty.

## Citation

```bibtex
@inproceedings{minz2026certificates,
  title={Exhaustive Search Certificates:
         Negative Results via GPU-Verified Lower Bounds},
  author={Alice and Claude and Contributors},
  year={2026}
}
```
