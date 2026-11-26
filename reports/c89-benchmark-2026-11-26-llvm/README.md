# Benchmark data: MinZ, SDCC 4.6.0 and llvm-z80 (2026-11-26)

- MinZ commit `7d1119456`; SDCC 4.6.0 #16555; llvm-z80 clang 23.1.5
  (`llvm-z80/llvm-z80` `3363179afe9a`, release 23.1.5-r1, Linux x86-64).
- `all-summary.json`, `fusion-summary.json`: raw rows of `research/c89-benchmark/run.py`.

```sh
python3 research/c89-benchmark/run.py --out <out>/all --sdcc-root <sdcc-root> \
  --llvm-z80-root <llvm-z80-root> --suite all --include-nanz --jobs 8
python3 research/c89-benchmark/run.py --out <out>/fusion --sdcc-root <sdcc-root> \
  --llvm-z80-root <llvm-z80-root> --suite fusion --jobs 8
python3 reports/c89-benchmark-2026-11-26-llvm/tables.py reports/c89-benchmark-2026-11-26-llvm > tables.md
```

A ratio is left / right; below 1 means the left side needs fewer T-states or bytes.
