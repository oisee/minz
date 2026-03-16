# Report #086 — Overnight Marathon: Codegen −38%, C89 Tier 1, Function Pointers

**Date:** 2026-03-15 → 2026-03-16
**Status:** Complete
**Participants:** Two parallel Claude sessions + human steering

---

## Executive Summary

All-night session producing the biggest single-day improvement in MinZ compiler history:

- **MinZ vs SDCC: −19% → −38%** (68B → 52B vs 84B on 6-function benchmark)
- **C89 corpus: 68 → 222 asserts**, 9 → 25 files, 5/9 → 17/18 assembling
- **5/6 C89 Tier 1 features** implemented (designated init, static locals, goto, compound literals, unions)
- **MZV runner** — MIR2 VM with TUI display, proves IR correctness
- **BUG-014 root-caused** — 3/4 sub-bugs fixed, remaining needs PBQP spill
- **Function pointer infrastructure** — OpCallIndirect + Z80 trampoline + VM dispatch

---

## Part 1: Z80 Codegen Hardening (11 commits)

### Benchmark: MinZ vs SDCC (identical C89 source)

| Function | v0.19.5 | v0.19.6 | SDCC | Improvement |
|----------|---------|---------|------|-------------|
| `twice` | 2B | 2B | 3B | MinZ −1B |
| `add` | 2B | 2B | 3B | MinZ −1B |
| `max` | 13B | **12B** | 12B | TIE (was SDCC +1B) |
| `abs_diff` | 10B | **5B** | 11B | **MinZ −6B (−55%)** |
| `sum_to` | 28B | **21B** | 25B | MinZ −4B (was SDCC +3B) |
| `clamp8` | 13B | **10B** | 30B | MinZ −20B |
| **TOTAL** | **68B** | **52B** | **84B** | **−38%** |

**Score: MinZ wins 5, ties 1, loses 0** (was: wins 3, loses 2)

### Key Optimizations

| Optimization | Before | After | Savings |
|---|---|---|---|
| Caller-save liveness | 105 PUSH/POP | 39 | −63% |
| ADD HL restore (signed cmp) | PUSH/POP HL | ADD HL,DE | −1B per cmp |
| SimplifyIdentities | 1 pattern | 5 patterns | Sub(0,x)→Neg etc |
| FuseAbsDiff | 9B (C89) | 5B | −44% |
| CondExpr return desugar | 14B (ternary) | 5B | −64% |
| SplitJoinRet | join-block bloat | direct Ret | enables CondRetSink |
| elimJrToRet peephole | JRS cc → label:RET | RET cc | −1B per site |

### abs_diff: All Variants Optimal

| C89 Source Pattern | Before | After |
|---|---|---|
| `if (a>b) return a-b; return b-a` | 9B | **5B** |
| `(a>b) ? (a-b) : (b-a)` (ternary) | 14B | **5B** |
| `if (a<b) return b-a; return a-b` | 9B | **5B** |
| `if (a>=b) return a-b; else return b-a` | 9B | **5B** |
| `d=a-b; if(a<b) d=b-a; return d` | 9B | **7B** |

### New MIR2 Passes

- **FuseAbsDiff** (`absdiff.go`): Rewrites `cmp.ugt(a,b)+sub(b,a)` → `sub(a,b)+CmpSubCarry`. Two patterns (A: cmp-before-sub, B: sub-before-cmp). Includes neg-rewrite for then-block.
- **SplitJoinRet** (`joinret.go`): Splits trivial join-blocks (1 param, empty body, Ret) into per-predecessor Ret blocks.
- **DeadBlockArgElim** (`deadblockarg.go`, by neighbor): Removes unused block parameters.

### New Opcodes Implemented

| Opcode | Algorithm | Cost |
|---|---|---|
| OpDiv (u8) | Dark/X-Trade DIVU111 shift-and-subtract | 236–244T |
| OpDiv (u16) | Restoring long division, 16 iterations | ~1000T |
| OpMod (u8/u16) | Same as div, return remainder | same |
| OpCallIndirect | Z80: `CALL __call_hl` trampoline (`JP (HL)`) | +4T overhead |

### BUG-014 (Tetris Black Screen) — Root Cause Analysis

| Sub-bug | Root Cause | Status |
|---|---|---|
| genMul16 HL clobber | dst≠HL but shift-and-add uses HL as scratch | **FIXED** |
| constVals loop counters | PreallocCoalesce merges OpConst with block param | **FIXED** |
| emitCallArgs canonical | Two u8 params both defaulted to A | **FIXED** |
| zx_screen_addr pressure | 8+ live u8, ALU clobbers A implicitly | **OPEN** (needs PBQP spill) |

---

## Part 2: C89 Frontend (neighbor session)

### Tier 1 Features (5/6 done)

| Feature | Status | Tests |
|---|---|---|
| Designated initializers `{.x=1}` | ✅ Done | 9/9 |
| Static local variables | ✅ Done | 2/2 |
| `goto` + labels (forward) | ✅ Done | 3/3 |
| Compound literals `(Type){...}` | ✅ Done | 5/5 |
| Union types | ✅ Done (declaration) | 1/1 |
| Function pointers | Backend ready, frontend deferred | — |

### Corpus Growth

| Metric | Before | After |
|---|---|---|
| C89 files | 9 | 25 |
| MIR2 asserts | 68 → 204 → 222 | +227% |
| Files assembling to Z80 | 5/9 | 17/18 |
| ObjC asserts | 25 → 30 | +20% |

---

## Part 3: New Tools

### MZV — MIR2 VM Runner

`cmd/mzv/` — TUI display for ZX Spectrum programs on MIR2 VM.

```bash
./mzv program.nanz              # interactive play
./mzv --headless --max-frames=N  # testing
```

- Host function overrides: zx_poke/peek/key_row/halt/border
- ZX ROM font OCR: 96 glyphs, normal + inverse
- ANSI 32×24 color renderer
- Proves: MIR2 IR is correct, all bugs are in Z80 codegen only

---

## Metrics Summary (v0.19.6)

| Metric | Value |
|---|---|
| MinZ vs SDCC | **−38%** (52B vs 84B) |
| C89 corpus asserts | **222** (was 68) |
| E2E Z80 tests | **24** (was 20) |
| Go test packages | **26/26** pass |
| Frontends | 6 (Nanz, C89, PL/M, Lanz, Lizp, Pascal) |
| Toolchain binaries | 9 (mz, mza, mze, mzx, mzd, mzlsp, mzrun, mztap, mzv) |
| Tetris (MZV) | ✅ works (500+ frames verified) |
| Tetris (MZX) | ❌ BUG-014 partial (3/4 fixed) |

---

## Next Priorities

1. **PBQP spill support** — unblocks tetris on Z80, any function with >7 live u8
2. **Function pointer C89 lowerer** — backend ready, need `OpCallIndirect` emission
3. **BUG-008** — Arena codegen, blocks struct methods
4. **ADR-0027** — Constraint-driven instruction selection (WFC/table approach)
5. **Ternary abs_diff v5** — 7B → 5B (need then-block neg rewrite fix)

---

*Two sessions, one night, 16+ commits. MinZ C89 backend now generates smaller code than SDCC on 5 of 6 benchmark functions.*
