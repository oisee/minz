# Iterator Chain Optimization Status (Report #2)

*2026-03-01 | MinZ v0.19.4 | Report #016*

---

## Summary

The iterator chain pipeline is **production-quality for 9 core operations** with 78 tests across 6 layers, all passing. Two critical bugs were resolved in this session: the pointer-walk regression (stale binary) and the inline filter constant bug (DCE missing `OpJumpIfFlag`). The pipeline now compiles MinZ iterator chains to efficient Z80 DJNZ loops with correct output, verified end-to-end via hex comparison.

---

## Test Pyramid — 78 Tests, 6 Layers, 100% Pass

| Layer | Tests | Status | What it validates |
|-------|------:|--------|-------------------|
| **E2E shell** (hex-verified) | **7** | **7 pass** | Full pipeline: MinZ → MIR → Z80 → MZA → MZX → output bytes |
| Corpus (compile) | 18 | 18 pass | 18 real .minz programs compile through Z80 backend |
| MIR VM | 8 | 8 pass | DJNZ loop execution in register-based VM |
| Codegen (Z80 patterns) | 7 | 7 pass | Correct Z80 instruction selection |
| Semantic (IR generation) | 20 | 20 pass | Correct IR generation for all op combinations |
| Parser (chain conversion) | 18 | 18 pass | Method chain → `IteratorChainExpr` AST |

### E2E Test Details

Each test compiles a `.minz` program, assembles to binary, runs in MZX with `--console-io`, and compares hex output:

| Test | Input | Chain | Expected Hex | Status |
|------|-------|-------|-------------|--------|
| `iter_foreach` | `[A,B,C,D,E]` | `forEach(console_log)` | `41424344450a` | PASS |
| `iter_take` | `[A,B,C,D,E]` | `take(3).forEach(...)` | `4142430a` | PASS |
| `iter_skip` | `[A,B,C,D,E]` | `skip(2).forEach(...)` | `4344450a` | PASS |
| `iter_map_foreach` | `[1,2,3,4,5]` | `map(double).forEach(...)` | `020406080a0a` | PASS |
| `iter_filter_foreach` | `[A,B,C,D,E]` | `filter(is_big).forEach(...)` | `44450a` | PASS |
| `iter_inline_filter` | `[A,B,C,D,E]` | `filter(\|x\| x > 67).forEach(...)` | `44450a` | PASS (NEW) |
| `iter_lambda_map` | `[A,B,C,D,E]` | `map(\|x\| x + 1).forEach(...)` | `42434445460a` | PASS |

---

## Operations — Full Support Matrix

### Z80-Ready (9 operations + inline filter)

| Operation | Z80 | E2E | Inline? | Z80 Pattern |
|-----------|:---:|:---:|:-------:|-------------|
| `forEach(fn)` | yes | yes | — | `CALL fn` inside DJNZ loop |
| `map(fn)` | yes | yes | — | `CALL fn`, store result, continue |
| `filter(fn)` | yes | yes | — | `CALL predicate`, `OR A`, `JR Z continue` |
| `filter(\|x\| x > N)` | yes | yes | `CP N+1` | Inline: `CP N+1`, `JR C continue` — no CALL |
| `take(n)` | yes | yes | — | Second counter, `DEC`+`JR Z end` |
| `skip(n)` | yes | yes | — | Second counter, `DEC`+`JR NZ continue` |
| `map(\|x\| x + 1)` | yes | yes | — | Lambda → direct `CALL` (zero-cost transform) |
| `peek(fn)` | yes | — | — | `CALL fn`, pass element through unchanged |
| `inspect(fn)` | yes | — | — | Alias for peek |
| `takeWhile(fn)` | yes | — | — | `CALL predicate`, `JR Z end` on false |

### MIR-Only (need OpPush fix for Z80)

| Operation | MIR VM | Z80 Blocker | Notes |
|-----------|:------:|-------------|-------|
| `enumerate` | yes | OpPush routes through HL | Needs direct `PUSH BC`/`PUSH DE` |
| `reduce(init, fn)` | yes | OpPush routes through HL | Both pushes become `PUSH HL` (wrong values) |

### Inline Lambda Filter — How It Works

For simple comparison lambdas like `filter(|x| x > 3)`, the compiler:

1. **Detects** the pattern via `isSimpleComparisonLambda()` — supports `>`, `>=`, `<`, `<=`, `==`, `!=`
2. **Adjusts** the constant for Z80 CP semantics: `x > 3` → `CP 4` (via `invertedFlagForComparison()`)
3. **Emits** `OpJumpIfFlag` instead of `OpCall` — saves ~27 T-states per iteration (no function call overhead)
4. **Z80 output**: `CP 4` + `JR C, filter_continue` — 2 instructions instead of `CALL` + `OR A` + `JR Z`

---

## Bugs Fixed

### 1. Pointer-Walk Regression — RESOLVED (not a bug)

**Symptom:** `iter_foreach` output `AAAAA` instead of `ABCDE`.

**Investigation:** Added debug traces to MIR copy propagation and semantic analyzer. Confirmed IR was correct via `--dump-mir` — `OpLoad.Src1=ptrReg=r10` (advancing pointer), not `sourceReg=r8` (array base).

**Root cause:** Stale `mz` binary built 5 hours earlier from a different code state.

**Lesson:** Always rebuild before E2E testing. The `mz` binary in the working directory is not automatically rebuilt by `go test`.

### 2. Inline Filter Constant (`CP 0` instead of `CP 68`) — FIXED

**Symptom:** `filter(|x| x > 67)` compiled to `OR A` (compare against 0) instead of `CP 68`. All elements passed through — no filtering occurred.

**Investigation path:**
1. Confirmed Z80 codegen's `OpJumpIfFlag` handler falls back to `CP 0` when `constantValues[Src2]` is empty
2. Checked if `constantValues` was cleared by `OpLabel` — but `OpLoadConst` is emitted AFTER the loop label, so this wasn't the issue
3. Added traces to `generateInstruction()` — discovered `OpLoadConst` for the filter constant was **completely missing** from the instruction list
4. Checked MIR peephole's DCE — only removed the array literal, not the filter constant
5. Checked main optimizer's DCE (`dead_code_elimination.go`) — **found it**

**Root cause:** `DeadCodeEliminationPass.markUsedRegisters()` Phase 1 was missing `OpJumpIfFlag`. Since the DCE didn't know `OpJumpIfFlag` reads `Src2`, it never marked the constant register as used. The `OpLoadConst` instruction was then removed as a "dead store." By the time Z80 codegen ran, the constant was gone and `constantValues` was empty.

**Fix:** Added to `dead_code_elimination.go`:
- `OpJumpIfFlag` → marks `Src1` and `Src2` as used
- `OpJumpIf`, `OpJumpIfZero`, `OpJumpIfNotZero` → marks `Src1` as used
- `OpPush` → marks `Src1` as used
- All missing jump opcodes added to `markReferencedLabels()`

**Verification:** `filter(|x| x > 67)` now generates `CP 68` + `JR C, filter_continue`. New E2E test `iter_inline_filter` produces correct hex output `44450a` (D, E, newline).

---

## Architecture — The DJNZ Iterator Pipeline

```
MinZ Source                    Z80 Assembly
─────────────                  ─────────────
arr.filter(|x| x > 67)        djnz_loop:
   .forEach(console_log);         LD HL, (ptr)     ; Load pointer
                                   LD A, (HL)       ; Load element
                                   PUSH HL          ; Save pointer
                                   CP 68            ; filter: x > 67?
                                   JR C, continue   ; Skip if ≤ 67
                                   CALL console_log ; forEach
                               continue:
                                   POP HL           ; Restore pointer
                                   INC HL           ; Advance
                                   LD (ptr), HL     ; Save pointer
                                   DEC B            ; DJNZ counter
                                   JR NZ, djnz_loop
```

### Pipeline Stages

```
Source → Parser → Semantic → MIR Optimizer → Z80 Codegen → MZA → Binary
         │         │           │                │
         │         │           │                └─ OpJumpIfFlag → CP N + JR
         │         │           └─ DCE, copy prop, constant fold
         │         └─ generateDJNZIteration(): OpLoadConst + OpJumpIfFlag
         └─ tryConvertIteratorChain(): method chain → IteratorChainExpr
```

### Key Design Decisions

- **DJNZ for ≤255 elements**: Uses Z80's dedicated loop instruction (13 T-states) instead of compare-and-jump (25+ T-states)
- **Pointer walk**: `INC HL` (6 T-states) instead of index computation (15+ T-states)
- **PUSH/POP HL around CALLs**: Functions clobber HL, so the array pointer is saved/restored per iteration
- **Inline lambda filter**: `CP N` + `JR CC` (14 T-states) instead of `CALL` + `OR A` + `JR Z` (~44 T-states)
- **Flag-based predicate ABI** (ADR-0008): Filter predicates set/clear CY flag, avoiding boolean register allocation

---

## Remaining Work

### Small Fixes

| # | Issue | File | Fix | Effort |
|---|-------|------|-----|--------|
| 4 | OpPush routes through HL | `pkg/codegen/z80.go` | Direct `PUSH BC`/`PUSH DE` when source already in register pair | Small |
| 5 | Counter waste (~12 T/iter) | `pkg/codegen/z80.go` | Peephole to collapse `LD A,B` / `LD B,A` shuffle | Small |
| 6 | Unused result saves | `pkg/codegen/z80.go` | Skip `LD D,L` after void function calls | Small |

### Larger Items

| Item | Status | Effort |
|------|--------|--------|
| Multi-stage chain E2E tests | `map(f).filter(g).forEach(h)` compiles but no hex-verified test | Small |
| Fusion optimizer | Skeleton in `pkg/optimizer/fusion.go`, detection-only | Medium |
| enumerate/reduce on Z80 | Blocked by OpPush routing (#4) | Small (after #4) |
| Generator syntax (`gen`/`yield`) | Design doc exists, not started | Large |

---

## Performance Characteristics

| Metric | Iterator Chain | Hand-Written DJNZ | Naive Indexed |
|--------|:--------------:|:-----------------:|:-------------:|
| Loop overhead | 13 T (DJNZ) | 13 T (DJNZ) | 25+ T (CP+JR) |
| Element access | 6 T (INC HL) | 6 T (INC HL) | 15+ T (index calc) |
| Filter (inline) | 14 T (CP+JR) | 14 T (CP+JR) | 14 T (CP+JR) |
| Filter (function) | ~44 T (CALL+OR+JR) | — | ~44 T |
| Pointer save/restore | 22 T (PUSH+POP HL) | 0 T (manual) | 0 T (no pointer) |
| Memory overhead | O(1) | O(1) | O(n) if multi-pass |

The main overhead vs hand-written code is the `PUSH/POP HL` per iteration (22 T-states) needed because CALLed functions clobber HL. This could be eliminated by the fusion optimizer (inlining the function body and using shadow registers or stack discipline).

---

## Timeline

| Date | Milestone |
|------|-----------|
| 2026-02-27 | Iterator parser + semantic + DJNZ codegen working, 53 unit tests |
| 2026-02-28 | HL clobber fix (PUSH/POP HL), TRUE SMC anchor fix, superoptimizer ADR |
| 2026-03-01 | All 6 E2E tests wired and passing (pointer-walk was stale binary) |
| 2026-03-01 | **Inline filter constant fix** — DCE bug found and fixed, 7/7 E2E pass |

---

## References

- [Iterator Implementation Status](../docs/Iterator_Implementation_Status.md) — pipeline details
- [Iterator E2E Testing Report (#014)](2026-03-01-014-Iterator_E2E_Testing_Report.md) — previous report (6 E2E tests)
- [Project Status (#015)](2026-03-01-015-Project_Status_And_Next_Steps.md) — full project status
- [ADR-0008](../docs/adr/0008-flag-based-boolean-abi-for-iterators.md) — flag-based boolean ABI
- [ADR-0009](../docs/adr/ADR-0009-superoptimizer-peephole-rules.md) — superoptimizer peephole rules
- [GenPlan](../docs/GenPlan.md) — canonical development roadmap

---

*MinZ: Zero-cost iterator chains on Z80 — 78 tests, 7 E2E, all green.*
