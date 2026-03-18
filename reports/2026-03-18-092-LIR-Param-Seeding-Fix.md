# Report 092 — LIR Param Seeding Fix & Codegen Analysis

**Date:** 2026-03-18
**Branch:** `feat/lir-backend`
**Status:** Bug #1 (params) fixed, Bug #3 (ld_r_hl cost) fixed, Bug #2 (OpCall) deferred

---

## Problem

The flat codegen path (`lirCodegenFlat`) ignored `f.Contract.Params`, so WFC had no idea which vreg should be in A vs C vs B. Result: `add(a, b)` → `ADD A, A` instead of `ADD A, C`.

Three bugs identified:

1. **Flat path ignores function params** — both params collapse to A (FIXED)
2. **OpCall not lowered** — CALL instructions vanish entirely (DEFERRED)
3. **Multiple loads to A overwrite each other** — `ld_r_hl` had same cost as `ld_a_hl` (FIXED)

## Root Cause

`lirCodegenFlat` called `SelectInstructions` (no param awareness) + `NewWFCState` (no param cells). The multi-block path already had `SelectBlockInstructions` + `NewWFCStateWithParams` which pre-seed vregs from block params — the flat path just wasn't using them.

## Fix (4 files)

### `bridge.go` — `ContractParamsToBlockParams` helper

New function that converts `f.Contract.Params` → `[]BlockParam`, reusing `regClassToLocSet`:

```go
func ContractParamsToBlockParams(f *mir2.Func, desc *MachineDesc) []BlockParam
```

Applied in 3 call sites: `LowerMIR2Func`, `lirCodegenFlat`, `checkFuncConvergenceFlat`.

### `pipeline.go` — Wire params into flat paths

All flat codegen paths now use:
```go
params := ContractParamsToBlockParams(f, desc)
sel, err := SelectBlockInstructions(desc, combResult.Ops, params)
wfc := NewWFCStateWithParams(desc, sel.Insts, params)
```

### `wfc.go` — `ToInsts()` filters synthetic cells

`NewWFCStateWithParams` prepends synthetic param cells (Pat==nil). `ToInsts()` now skips them, matching what the multi-block path already does in `ProgWFC.Collapse()`.

### `z80.go` — `ld_r_hl` cost 7 → 9

Higher cost than `ld_a_hl` (7) so isel prefers A when possible, but uses any GPR when A is occupied.

---

## Codegen Results

### Leaf functions (no calls) — correct, optimal

| Function | Production (PBQP) | LIR (ISLE+WFC) | Status |
|----------|-------------------|-----------------|--------|
| `add(a,b)` | `ADD A, C` | `ADD A, B` | correct (1 inst) |
| `sub(a,b)` | `SUB C` | `SUB B` | correct (1 inst) |
| `double(x)` | `ADD A, A` | `ADD A, A` | correct (1 inst) |
| `add3(a,b,c)` | `ADD A, C; ADD A, D` | `ADD A, B; ADD A, C` | correct (2 inst) |
| `mul_add(a,b,c)` | `ADD A, C; SUB D` | `ADD A, B; SUB C` | correct (2 inst) |

All leaf functions produce **identical instruction count** to production. Register assignment differs (B/C vs C/D) because LIR and PBQP use different allocation strategies, but both are correct.

### Functions with calls — Bug #2 (OpCall not lowered)

| Function | Production (PBQP) | LIR (ISLE+WFC) | Status |
|----------|-------------------|-----------------|--------|
| `triple(x) = double(x) + x` | `ADD A,A; LD B,A; ADD A,B` (3 inst, correct: x*3) | `ADD A,A; ADD A,A` (2 inst, **WRONG: x*4**) | BUG |

`OpCall` is skipped in `translateInst` (bridge.go:441-442), so calls vanish. After inlining, `triple` sees two independent `add(self,self)` operations instead of `double(x)` then `add(result, x)`.

**Fix requires:** calling convention modelling, clobber set tracking, callee metadata. Separate task.

---

## Test Added

`TestLIRCodegen_TwoParams` — builds `add(a: ClassAcc, b: ClassGeneral)`, runs `LIRCodegenFunc`, asserts output contains `ADD A, <non-A-reg>` and does NOT contain `ADD A, A`.

---

## MIR2 Instruction Count: Nanz vs C89 (FatFS functions)

Side analysis of MIR2 efficiency across frontends:

| Function | Nanz | C89 | Delta | Winner |
|----------|------|-----|-------|--------|
| ld_word | 9 | 9 | 0 | TIE |
| st_word | 10 | 8 | +2 | C89 |
| read_fat12 | 15 | 16 | -1 | Nanz |
| classify_fat12 | 21 | 21 | 0 | TIE |
| clst2sect | 9 | 9 | 0 | TIE |
| is_deleted | 11 | 11 | 0 | TIE |
| sfn_checksum | 18 | 19 | -1 | Nanz |
| dbc_1st | 2 | 2 | 0 | TIE |
| dbc_2nd | 2 | 2 | 0 | TIE |
| chain_length/follow_chain | 36 | 35 | +1 | C89 |

**Totals:** 6 TIE, 2 Nanz wins, 2 C89 wins. Practically identical at MIR2 level.

### Root Causes of Differences

- **st_word (C89 wins by 2):** Nanz does explicit `& 0xFF` mask (2 extra `const 255 + and`), C89 uses `(BYTE)val` cast → single `trunc` (free on Z80). **Optimization opportunity:** Nanz could emit `trunc` for u16→u8 narrowing.

- **read_fat12 (Nanz wins by 1):** C89 emits redundant `move` for pointer coercion before `call ld_word`. Nanz passes `add` result directly.

- **sfn_checksum (Nanz wins by 1):** C integer promotion forces `i16` arithmetic + `trunc` back to `u8`. Nanz keeps narrow `u8` arithmetic throughout.

- **chain_length (C89 wins by 1):** `do-while + break` (C89) is more efficient than `while + flag variable` (Nanz) — 1 fewer block parameter to thread through the loop.

---

## Next Steps

1. **OpCall lowering (Bug #2)** — requires calling convention, clobber sets, callee metadata
2. **Nanz `trunc` optimization** — emit `trunc` instead of `& 0xFF` for u16→u8
3. **C89 redundant move elimination** — remove unnecessary `move` before calls
