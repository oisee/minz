# Z80 Exhaustive Register Allocation Tables

Complete tables of provably optimal register allocations for the Z80 CPU.
Every possible constraint shape is enumerated and solved.

## Files

### compressed/ (for distribution)
Binary format + zstd compression. ~41MB total for ≤6v.

| File | Entries | Feasible | Size |
|------|---------|----------|------|
| exhaustive_4v.bin.zst | 156,506 | 123,453 (78.9%) | 0.1MB |
| exhaustive_5v.bin.zst | 17,366,874 | 11,762,983 (67.7%) | 8.5MB |
| exhaustive_6v_dense.bin.zst | 66,118,738 | TBD | ~32MB |

Binary format: fixed-size records in enumerator order.
- Header: 4 bytes magic "Z80T" + 4 bytes version (LE uint32)
- Feasible: 1 byte nVregs + 2 bytes cost (LE uint16) + nVregs bytes assignment
- Infeasible: 1 byte 0xFF marker

Enumerator order is deterministic: `regalloc-enum --max-vregs N`
Shape K in table = shape K from enumerator output.

### raw/ (for analysis)
Original JSONL from GPU solver. One JSON object per line:
`{"cost": N, "assignment": [loc0, loc1, ...], "searchSpace": S, "feasible": F}`
Cost -1 = infeasible (no valid assignment exists).

## Coverage

| Level | Method | Coverage |
|-------|--------|---------|
| ≤5v | Exhaustive GPU enumeration | 100% (zero misses) |
| 6v tw≤3 | Composition from ≤5v table | 98.3% of 6v shapes |
| 6v tw≥4 | Direct GPU solve | 1.7% of 6v shapes |
| 7v+ | On-demand GPU/Z3 + caching | corpus-driven |

## Key Findings

- Feasibility cliff: 95.9% (2v) → 88.5% (3v) → 78.7% (4v) → 67.7% (5v) → 0.9% (6v)
- 99.5% of random interference graphs have treewidth ≤3 (classically tractable)
- Real compiler graphs are denser: 53.7% of dense corpus functions have tw≥4
- Composition from ≤4v table verified on 13.2M 5v shapes: max 12T overhead, 0 misses

## Generation

```bash
# Enumerate shapes
./regalloc-enum --max-vregs 5 > shapes.jsonl

# Solve on GPU
cat shapes.jsonl | ./cuda/z80_regalloc --server > results.jsonl

# For 6v dense only (tw≥4 interference graphs):
./regalloc-enum --max-vregs 6 --only-nv 6 --dense-masks dense_6v_masks.txt > shapes_6v.jsonl
```

## Repository
https://github.com/paigeadelethompson/z80-optimizer (or wherever this ends up)

Generated: 2026-03-26
Hardware: 2x NVIDIA RTX 4060 Ti 16GB
