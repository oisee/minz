# Benchmark data: MinZ, SDCC 4.6.0 and llvm-z80 (2026-11-27)

- MinZ commit `4a49d0dfe`; SDCC 4.6.0 #16555; llvm-z80 clang 23.1.5
  (`llvm-z80/llvm-z80` `3363179afe9a`, release 23.1.5-r1, Linux x86-64).
- `all-summary.json`, `fusion-summary.json`: raw rows of `research/c89-benchmark/run.py`.
- Same commands as [the 2026-11-26 run](../c89-benchmark-2026-11-26-llvm/README.md);
  only the MinZ commit differs.

A ratio is left / right; below 1 means the left side needs fewer T-states or bytes.
