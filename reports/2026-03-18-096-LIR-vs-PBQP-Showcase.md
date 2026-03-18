# Report 096 — LIR vs PBQP Showcase Comparison

**Date:** 2026-03-18
**Branch:** master (after merge of feat/lir-backend, 38 commits)

---

## Status: LIR Default, 9/9 Nanz Showcase Compiles

Previous report: 3/8 assembled. Now: **9/9 all compile and assemble** (invalid opcodes fixed).

## Instruction Count Comparison

| Example | PBQP | LIR | Delta | Winner |
|---------|------|-----|-------|--------|
| 01_sum_array | 16 | 13 | **-18%** | LIR |
| 02_sum_idiomatic | 24 | 29 | +20% | PBQP |
| 03_filter_map_chain | 37 | 57 | +54% | PBQP |
| 04_lut_popcount | 6 | 7 | +16% | PBQP |
| 05_four_pointers | 15 | 27 | +80% | PBQP |
| 06_pbqp_weighted | 19 | 20 | +5% | PBQP |
| 07_ix_load_store | 18 | 33 | +83% | PBQP |
| 08_arena_allocator | 723 | 456 | **-36%** | LIR |
| 09_function_pointers | 21 | 19 | **-9%** | LIR |

**LIR wins: 3** (sum_array -18%, arena -36%, func_ptrs -9%)
**PBQP wins: 6** (iterator fusion, LUT tricks, IX indexing, pointer ops)

## Root Cause Analysis: Where PBQP Wins

### 1. Page-Aligned LUT Access (`LD H, label^H`)
PBQP: `LD H, popcount_lut^H; LD L, A; LD A, (HL)` — 3 instructions
LIR: `LD BC, A; EX DE, HL; LD HL, 0; ADD HL, DE; LD A, (HL)` — 5 instructions

**Fix:** Add `ld_h_imm` pattern for high-byte-of-label + recognize LUT access pattern in ISLE.

### 2. Conditional Returns (`RET NC`, `RET Z`, `RET C`)
PBQP: `SUB C; RET NC; NEG; RET` — abs_diff in 4 instructions
LIR: flat path loses control flow, no conditional returns

**Fix:** Multi-block path needs `TermCondReturn` support in block rules + emit `RET cc` patterns.

### 3. Iterator DJNZ Fusion
PBQP: hand-tuned iterator patterns fuse callback into DJNZ loop
LIR: each iterator step is a separate function call

**Fix:** ISLE combining rules for iterator patterns (forEach+lambda → DJNZ loop).

### 4. IX-Indexed Loads (`LD A, (IX+d)`)
PBQP: directly uses IX+offset for struct field access
LIR: bounces through HL with explicit pointer arithmetic

**Fix:** Add IX-indexed patterns to ISel, teach bridge to use `ld_r_ix_d` pattern.

### 5. RST Optimization (`RST 0x10` vs `CALL 0x0010`)
PBQP: recognizes extern addresses 0x00-0x38, emits 1-byte RST
LIR: always emits 3-byte CALL

**Fix:** Post-emit peephole: `CALL 0x00|0x08|...|0x38` → `RST addr`.

## Where LIR Wins

### Large Functions (Arena Allocator: -36%)
LIR's ISLE combining + save-before-overwrite produces fewer redundant moves for complex functions with many variables. PBQP's O(n²) interference graph gets expensive.

### Multiply (mul8: LIR works, PBQP has TODO stub)
LIR routes through `__mul8` runtime. PBQP had a TODO placeholder.

### Function Pointers (LIR: 19 vs PBQP: 21)
LIR's `__call_hl` trampoline is more compact than PBQP's inline approach.

## Improvement Roadmap (Priority Order)

| Priority | Fix | Impact | Effort |
|----------|-----|--------|--------|
| P0 | Conditional returns (RET cc) | -20-50% on if/else functions | Medium |
| P1 | `LD H, label^H` for LUT | -16% on LUT access | Small |
| P2 | IX-indexed load patterns | -30-80% on struct access | Medium |
| P3 | RST optimization | -2 bytes per extern call | Small |
| P4 | Iterator DJNZ fusion | -50% on iterator chains | Large |
