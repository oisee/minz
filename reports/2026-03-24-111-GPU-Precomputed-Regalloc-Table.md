# Report #111: GPU-Precomputed Register Allocation Table

**Date:** 2026-03-24
**Author:** VIR Session (Claude Opus 4.6) + z80-optimizer Session (Claude Opus 4.6)
**Status:** Shipped — 59 entries, O(1) lookup

---

## The Problem

Register allocation on Z80 is constrained: only 7 general-purpose 8-bit registers (A, B, C, D, E, H, L), 3 pairs (BC, DE, HL), and 4 undocumented half-index registers (IXH, IXL, IYH, IYL). The Z3 SMT solver finds optimal assignments but requires an external binary (`z3`) and takes ~800ms per function.

**Question:** Can we precompute optimal assignments OFFLINE and look them up at compile time?

## The Insight

Two functions with *isomorphic constraint graphs* — same number of vregs, same pattern requirements, same interference — have the **same optimal register assignment**. The function names, immediates, and symbols don't matter.

This means we can:
1. Hash each function's constraints into a **signature** (SHA256 of vreg count + pattern loc sets + interference pairs)
2. Run GPU brute-force ONCE per unique signature
3. Ship the results as a **lookup table** with the compiler
4. At compile time: hash → lookup → instant optimal assignment

## The Architecture

```
┌─────────────────────────────────────────────────────┐
│                 OFFLINE (GPU, once)                  │
│                                                     │
│  Nanz corpus → VIR ops → signatures → GPU kernel    │
│                                                     │
│  GPU: for each signature, try ALL 7^N (or 11^N)     │
│  register assignments. Check interference, pattern   │
│  constraints, tied-dst rules. Pick minimum cost.     │
│                                                     │
│  Output: regalloc_table.json (59 entries, 5.9KB)    │
└─────────────────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────┐
│              COMPILE TIME (CPU, per function)        │
│                                                     │
│  VIR ops → ComputeSignature() → table.Lookup()     │
│                                                     │
│  HIT:  feed assignment to Z3 as hard constraints    │
│        → instant pattern selection (no search)       │
│  MISS: full Z3 solve (normal path, ~800ms)          │
└─────────────────────────────────────────────────────┘
```

## The GPU Kernel

Built in collaboration with the z80-optimizer session (`~/dev/z80-optimizer/cuda/z80_regalloc.cu`):

- **One CUDA thread per assignment**: thread index encodes `vreg[0] = idx % N_LOCS, vreg[1] = (idx/N_LOCS) % N_LOCS, ...`
- **Three-stage evaluation**: (1) interference check, (2) pattern constraint check, (3) cost evaluation with move penalties
- **Atomic reduction**: `atomicMin` finds global minimum across all threads
- **Server mode**: CUDA initializes once, reads JSON-per-line, GPU buffers reused across solves

### Search Space

| Vregs | 7-loc (8-bit GPR) | 11-loc (+pairs) | GPU time (RTX 4060 Ti) |
|-------|-------------------|------------------|------------------------|
| 3     | 343               | 1,331            | instant                |
| 5     | 16,807            | 161,051          | instant                |
| 8     | 5.7M              | 214M             | ~0.1s / ~2s            |
| 10    | 282M              | 25.9B            | ~2s / ~30s             |
| 12    | 13.8B             | 3.1T             | ~60s / too large       |

## Results

### Corpus Statistics
- **194 total functions** in Nanz corpus
- **158 unique signatures** (18.6% reuse — different functions with same constraint pattern)
- **109 functions** serializable to GPU format (85 have >14 vregs or empty bodies)
- **59 solved** with provably optimal assignments

### Notable Entries

| Function | Vregs | Cost | Search Space | Feasible | Notes |
|----------|-------|------|-------------|----------|-------|
| `max_byte` | 4 | 4T | 2,401 | 180 | Cheapest — single CP + RET |
| `putchar` | 1 | 10T | 7 | 7 | Shared by 6 functions |
| `screen_show` | 12 | 133T | 13.8B | 9.8M | Largest solve — 0.07% feasible |
| `alloc_and_check` | 12 | 64T | 13.8B | 159M | Arena allocator |
| `tui_clear` | 11 | 154T | 2.0B | 784M | Most feasible assignments |

### Reuse Winners

These functions share the same constraint signature — one GPU solve covers all:

| Signature | Functions | Cost |
|-----------|-----------|------|
| `1v_1o_9cd60...` | `putchar`, `_putch`, `_dec`, `_puts`, `_p`, `tui_puts` | 10T |
| `2v_2o_de71a...` | `screen_name`, `screen_city`, `screen_country`, `screen_carrier` | 14T |
| `7v_7o_258ea...` | `test_apply_double`, `test_apply_triple` | 42T |
| `6v_5o_013ef...` | `is_warm`, `is_alive` | 21T |

## Cross-Session Collaboration

This was built by three Claude Code sessions communicating via `dedelulu`:

1. **minz-vir** (VIR backend): Go bridge, signature computation, table infrastructure, corpus generation
2. **z80-optimizer** (CUDA): GPU kernel, JSON parser, server mode, 11-loc expansion
3. **minz** (main compiler): ABAP+SQLite integration, testing validation

The sessions exchanged 30+ messages coordinating the JSON schema, debugging the Go→CUDA pipe hang (CUDA driver + Go runtime signal handler conflict), and iterating on the null-field fix.

## How to Use

### For compiler developers
```bash
# Generate corpus from Nanz examples
cd minzc && ./gpu-bench --generate

# Run GPU kernel (from z80-optimizer session or shell)
CUDA_VISIBLE_DEVICES=0 ~/dev/z80-optimizer/cuda/z80_regalloc --server \
  < /tmp/gpu_corpus.jsonl > /tmp/gpu_results.jsonl 2>/tmp/gpu_server.log

# Build table
./gpu-bench --build-table
# → regalloc_table.json (ships with compiler)
```

### For end users
Nothing to do — `regalloc_table.json` ships with the compiler. When `--vir` is active, the table is loaded automatically. Table hits are silent and instant; misses fall back to Z3.

## Known Limitations

1. **GPU kernel supports 11 locations** (7 GPR + BC/DE/HL/IXH). Functions needing IXL/IYH/IYL/memory spill are infeasible → Z3 fallback.
2. **Go test + CUDA hang**: CUDA driver init conflicts with Go's runtime signal handlers. GPU test gated behind `VIR_GPU_TEST=1`. Works from compiled binaries.
3. **Search space limit**: 11^12 = 3.1T exceeds the 500B limit. Functions with >11 vregs in 11-loc mode → Z3 fallback.
4. **Table coverage**: 59/194 functions (30%). Expanding the location space and raising the search limit will increase coverage.

## What's Next

1. **Raise GPU search limit** to 500B (covers 11^11 = 285B, ~15s per function)
2. **C89 corpus** — 304 additional functions for the table
3. **STOKE stochastic search** for functions beyond brute-force range (13+ vregs)
4. **Compile-time table generation** — when the compiler sees a new signature, optionally invoke GPU and cache the result
5. **ABAP+SQLite validation** — the main session has `mara_alv.abap` ready to test with the table

## Files

| Path | Purpose |
|------|---------|
| `regalloc_table.json` | 59 precomputed optimal assignments (ships with compiler) |
| `minzc/pkg/vir/regalloc_table.go` | Table infrastructure: signature, lookup, save/load |
| `minzc/pkg/vir/gpu.go` | GPU bridge: JSON serialization, server mode, SolveGPU() |
| `minzc/cmd/gpu-bench/main.go` | Corpus generator + table builder + comparison tool |
| `~/dev/z80-optimizer/cuda/z80_regalloc.cu` | CUDA kernel: brute-force regalloc with --json/--server |
