# MinZ VIR Solver — E2E Codegen Report

Z3 joint isel+regalloc with ISLE fusion + Grace PIR optimization.
Every function compiled → assembled → valid Z80 binary.

## Leaf & Conditional Functions (Nanz)

| Function | Instructions | Bytes | Z80 Assembly |
|----------|-------------|-------|--------------|
| `add` | 2 | 2 | `ADD A, C / RET` |
| `sub` | 2 | 2 | `SUB C / RET` |
| `double` | 2 | 2 | `ADD A, A / RET` |
| `inc` | 2 | 2 | `INC A / RET` |
| `identity` | 1 | 1 | `RET` |
| `const5` | 2 | 3 | `LD A, 5 / RET` |
| `neg` | 2 | 3 | `NEG / RET` |
| `band` | 2 | 2 | `AND C / RET` |
| `bor` | 2 | 2 | `OR C / RET` |
| `ret_b` | 2 | 2 | `LD A, C / RET` |
| `max` | 6 | 8 | `CP C / JR Z, .max_if_join2 / JR C, .max_if_join2 /...` |
| `min` | 5 | 6 | `CP C / JR NC, .min_if_join2 / RET / LD A, C / RET` |
| `abs_diff` | 11 | 13 | `CP C / JR Z, .abs_diff_if_join2 / JR C, .abs_diff_...` |

## Paper Benchmarks vs SDCC 4.2.0

| Program | SDCC | VIR | Delta | Winner |
|---------|------|-----|-------|--------|
| abs_diff | 12 | 12 | 0% | tie |
| gcd | 17 | 15 | -11% | VIR |
| minmax | 60 | 11 | -81% | VIR |
| fib | 22 | 11 | -50% | VIR |
| swap | 20 | 2 | -90% | VIR |
| **TOTAL** | **131** | **51** | **-61%** | **VIR** |

## FatFS Low-Level Functions (C89)

| Function | VIR Insts | SDCC Bytes | Notes |
|----------|-----------|------------|-------|
| `ld_word` | 5 | 29 | SDCC 29B |
| `st_word` | 4 | 4 | SDCC 4B |
| `dbc_1st` | 2 | n/a |  |
| `dbc_2nd` | 2 | n/a |  |
| `classify_fat12` | 21 | n/a |  |
| `is_deleted` | 10 | n/a |  |
| `clst2sect_simple` | 12 | n/a |  |
| `sfn_checksum` | 17 | n/a |  |
| `read_fat12` | 11 | n/a |  |
| `follow_chain` | 30 | n/a |  |
| `bpb_bytes_per_sect` | 4 | n/a |  |
| `bpb_sects_per_clust` | 5 | n/a |  |
| `bpb_reserved_sects` | 4 | n/a |  |
| `bpb_num_fats` | 5 | n/a |  |
| `bpb_root_entry_count` | 4 | n/a |  |
| `bpb_fat_size_16` | 4 | n/a |  |
| `root_dir_start` | 9 | n/a |  |

Total binary: 611 bytes

## Corpus Coverage

| Corpus | Functions | Pass Rate |
|--------|-----------|----------|
| Nanz | 216/216 | 100% |
| C89 | 304/304 | 100% |
| **Total** | **520/520** | **100%** |

Zero PBQP fallback. Previous gaps closed by:
- **16-bit div/mod inline runtime** — `__div16`/`__mod16` shift-and-subtract long division, inlined per call site
- **OpAsmBlock passthrough** — inline asm emitted verbatim, surrounding code still Z3-optimized
