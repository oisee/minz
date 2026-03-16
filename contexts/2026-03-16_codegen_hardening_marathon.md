# Context: Z80 Codegen Hardening Marathon

**Date:** 2026-03-15 → 2026-03-16 (overnight session)
**Machine:** macbookairm2 (alice)
**Branch:** master

---

## Session Summary

Epic codegen improvement session. MinZ vs SDCC: **−19% → −38%** (68B → 52B vs 84B).
Tetris MIR2 VM runner built, BUG-014 root-caused and partially fixed.

## Commits (chronological)

| Hash | Description |
|------|-------------|
| `145c514` | feat(mzv2): MIR2 VM runner with TUI display + ZX font OCR |
| `1fac23a` | chore: rename cmd/mzv2 → cmd/mzv |
| `9106e99` | feat(z80): OpDiv/OpSDiv/OpMod codegen (Dark/X-Trade DIVU111) |
| `2e69822` | perf(z80): caller-save liveness (−63% PUSH/POP) |
| `541f7e6` | perf(z80): ADD HL,rr restore in signed compare (max −1B) |
| `84aed4f` | perf(mir2): SimplifyIdentities Sub(0,x)→Neg, Add(x,0)→Move |
| `68f3e9f` | fix(z80): genMul16 save/restore HL when dst≠HL (BUG-014 partial) |
| `b530903` | fix(z80): exclude block params from constVals (BUG-014 partial #2) |
| `c798f02` | fix(z80): A-clobber interference + call arg alloc + instruction reorder |
| `d3acd8f` | perf(mir2): FuseAbsDiff — cmp.ugt+sub(b,a) → sub(a,b)+CmpSubCarry |
| `81a4fa2` | fix(mir2): FuseAbsDiff keep Src[1] for VM CmpSubCarry evaluation |

## Benchmark: MinZ vs SDCC (C89 identical source)

| Function   | v0.19.5 | NOW  | SDCC | Winner |
|------------|---------|------|------|--------|
| `twice`    |      2B |   2B |   3B | MinZ −1B |
| `add`      |      2B |   2B |   3B | MinZ −1B |
| `max`      |     13B |  12B |  12B | TIE |
| `abs_diff` |     10B | **5B** |  11B | **MinZ −6B (−55%)** |
| `sum_to`   |     28B |  21B |  25B | MinZ −4B |
| `clamp8`   |     13B |  10B |  30B | MinZ −20B |
| **TOTAL**  |  **68B**| **52B** | **84B** | **MinZ −38%** |

## Tetris Status

- **MZV (MIR2 VM)**: ✅ works perfectly, pieces fall, game logic correct
- **MZX (Z80)**: ❌ black screen (BUG-014)
  - Root cause 1: genMul16 HL clobber → **FIXED** (zx_attr_addr works)
  - Root cause 2: constVals treats loop counters as const → **FIXED**
  - Root cause 3: fill_cell call arg A-clobber → **FIXED** (reorderAccMoves + emitCallArgs)
  - Root cause 4: zx_screen_addr register pressure (8+ live u8, A clobbered by ALU scratch) → **OPEN**
  - Needs: PBQP spill support or zx_screen_addr inline asm workaround

## Key Technical Discoveries

1. **genMul16 clobbers HL**: when mul result allocated to DE, the shift-and-add sequence uses HL as scratch, destroying any live value in HL. Fix: PUSH/POP HL when dst≠HL.

2. **PreallocCoalesce constVals**: after coalescing, OpConst Dst may share a Reg with a block param (loop counter). constVals treated it as compile-time constant → `ADD A, 0` instead of `ADD A, D`. Fix: blockParamRegs exclusion set.

3. **8-bit ALU implicit A clobber**: Z80 8-bit ALU ops (ADD/SUB/AND/OR/XOR) always route through A. PBQP doesn't model this — assigns long-lived values to A that get clobbered by intermediate ALU ops. Partial fix: interference edges for ALU sources. Full fix needs PBQP spill.

4. **emitCallArgs canonicalReturnLoc**: was using class-based default (ClassGeneral u8 → A) for ALL params. Two u8 params both defaulted to A = conflict. Fix: use callee's actual PBQP allocation.

5. **FuseAbsDiff**: `cmp.ugt(a,b) + sub(b,a)` → `sub(a,b) + CmpSubCarry`. Enables Sub+Cmp flag fusion on Z80. C89 abs_diff: 9B → 5B. Key trick: repoint cmp sources to sub's dst to remove false live ranges → PBQP assigns sub result to A.

## C89 Corpus Status

- **17/18 OK** (import_test.c = multi-file, expected fail)
- **211 asserts** all passing
- **abs_diff fusion**: v1/v3/v4 patterns → 5B. v2 (ternary) = 14B, v5 (join-block) = 9B.

## New Tools

- **MZV** (`cmd/mzv/`): MIR2 VM runner with TUI display + ZX ROM font OCR
  - `./mzv program.nanz` — interactive play
  - `./mzv --headless --max-frames=100` — testing
  - Proves MIR2 IR correctness as oracle for Z80 codegen

## Next Priorities

1. **PBQP spill** — blocks tetris on Z80. See `memory/pbqp_spill_plan.md`
2. **BUG-008** 🔴 — Arena codegen, blocks struct methods
3. **BUG-001** — GCD parallel-copy bloat
4. **Ternary abs_diff** — C89 lowerer CondExpr → if/return conversion
5. **ADR-0027** — Constraint-driven instruction selection (WFC/table approach)

## Reports Updated

- **#081** MinZ vs SDCC → −38%
- **#082** C89 Frontend → 17/18 OK, 211 asserts
- **#083** Z80 Codegen Hardening Session (new)
