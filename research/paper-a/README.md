# Paper A: Register Allocation as a Solved Game

**Exhaustive GPU Tables for Sub-Cliff Architectures**

---

## Thesis

For architectures with fewer than ~16 register locations, the register allocation
decision space has low effective entropy. Exhaustive GPU enumeration produces a
finite lookup table that covers >97% of all functions. Register allocation becomes
an O(1) lookup — a solved game.

## Key Results

| Metric | Value |
|--------|-------|
| Unique constraint signatures | 315 |
| Corpus convergence | 97.8% |
| Cross-frontend transfer (Nanz→ABAP) | 88.2% |
| ABAP-specific hit rate | 98.8% |
| Phase transition cliff | ~16 register locations |
| GPU enumeration time (2-5 vreg) | 20 min (dual RTX 4060 Ti) |
| Entries (5-vreg exhaustive) | 11.6M feasible |
| Direct PIR emit (zero solver) | O(1) per function |

## Reproduce

### Prerequisites

```bash
# MinZ compiler
git clone https://github.com/oisee/minz
cd minz/minzc && make install-user

# z80-optimizer (GPU kernel)
git clone https://github.com/oisee/z80-optimizer
cd z80-optimizer && make
```

### Experiment 1: Signature Convergence

Compile the full corpus and count unique signatures:

```bash
# Compile all Nanz examples + ABAP examples
VIR_DUMP_GPU_BATCH=1 mz --vir examples/nanz/*.nanz > sigs_nanz.jsonl
VIR_DUMP_GPU_BATCH=1 mz --vir examples/abap/*.abap > sigs_abap.jsonl

# Count unique signatures
cat sigs_nanz.jsonl sigs_abap.jsonl | jq -r '.sig' | sort -u | wc -l
# Expected: ~315
```

### Experiment 2: Cross-Frontend Transfer

Train on Nanz+PL/M, test on ABAP+Screen+SQLite:

```bash
# Build table from Nanz corpus only
cat sigs_nanz.jsonl | gpu-server > nanz_table.jsonl

# Test against ABAP corpus
python3 research/paper-a/transfer_test.py \
  --table nanz_table.jsonl \
  --test sigs_abap.jsonl
# Expected: 88.2% hit rate
```

### Experiment 3: Phase Transition

Parametrize MAX_LOCS from 3 to 32:

```bash
for locs in 3 7 8 15 16 32; do
  gpu-server --max-locs $locs < corpus_sigs.jsonl > results_$locs.jsonl
  echo "L=$locs: $(grep SOLVED results_$locs.jsonl | wc -l) feasible"
done
# Expected cliff at L=16
```

### Experiment 4: Live Demo

Real SQL on Z80 CP/M with provably optimal register allocation:

```bash
# Build ZSQL
mz examples/nanz/zsql.nanz --vir --target=cpm -o zsql.a80
mza zsql.a80 -o ZSQL.COM

# Execute real SQL
printf "CREATE TABLE users(name TEXT, age TEXT)\n\
INSERT INTO users VALUES('Alice','30')\n\
INSERT INTO users VALUES('Bob','25')\n\
SELECT * FROM users\n.quit\n" | mze ZSQL.COM -t cpm

# Output: Alice | 30 / Bob | 25
```

## Data Files

| File | Description |
|------|-------------|
| `data/regalloc_table.json` | 315-entry production lookup table |
| `data/corpus_sigs.jsonl` | 1325 function signatures from full corpus |
| `data/phase_transition.csv` | Feasibility rate vs MAX_LOCS (3-32) |
| `data/transfer_matrix.csv` | Cross-frontend hit rates |
| `data/gpu_timing.csv` | GPU enumeration time vs nVregs |

## Figures

| Figure | Source |
|--------|--------|
| Fig 1: Phase transition | `data/phase_transition.csv` → `scripts/plot_phase.py` |
| Fig 2: ZSQL on ZX Spectrum | `media/zsql_mara_zx_spectrum.png` |
| Fig 3: Marginal coverage curve | `data/corpus_sigs.jsonl` → `scripts/plot_marginal.py` |
| Fig 4: Signature reuse heatmap | `data/transfer_matrix.csv` → `scripts/plot_transfer.py` |

## Architecture

```
Source (.nanz / .abap / .c89)
  → Frontend → HIR → MIR2 → VIR ops
  → ComputeSignature(ops) → SHA256 hash
  → Table lookup (O(1))
    HIT  → Direct PIR emit (pattern select, zero solver)
    MISS → GPU server (solve, cache) or Z3 fallback
  → Z80 assembly → binary
```

## Citation

```bibtex
@inproceedings{minz2026regalloc,
  title={Register Allocation as a Solved Game:
         Exhaustive GPU Tables for Sub-Cliff Architectures},
  author={Alice and Claude and Contributors},
  year={2026}
}
```
