# Report 101 — FlagsOnly CMP, Function Pointers, ABI Safety

**Date:** 2026-03-20
**Status:** All fixes verified, committed, showcase improved
**Builds on:** Report 098 (Grace Showcase), Report 100 (Peephole Profiling)

---

## Changes in This Session

### 1. FlagsOnly field fix (Inst struct)

`FlagsOnly` was accidentally placed inside `CallAttrs` struct — inaccessible from codegen. Moved to `Inst` where `OpCmp` can see it.

**Result:** Grace rule `flags-only-cmp` now fires correctly. Z80 codegen skips `ADD HL,rr` restore after `SBC HL,rr` when LHS is dead.

### 2. Function pointer codegen (two sub-fixes)

**Bug A — `elimSingleJpEqu` ate the indirect call trampoline:**
```
__call_hl:        →   __call_hl    EQU (HL)   ← WRONG
    JP (HL)
```
Peephole treated `JP (HL)` as a tail-call to label `(HL)`. Fix: exclude `JP` with `(` in target from single-JP-to-EQU optimization.

**Bug B — bare function name produced empty move:**
```
apply(double, 5)   →   %r10 = move        ← WRONG (no source!)
```
Nanz parser returned `VarRefExpr{Name: "double"}` for a function name used as a value. Since `double` is not a local variable, HIR lowering produced an empty move. Fix: when an identifier is a known function and NOT followed by `(`, emit `AddrOfExpr{Sym: name}` instead.

**Result:** `09_function_pointers.nanz`: FAIL → PASS.

### 3. ABI pinning for address-taken functions

Functions whose address is taken via `OpAddrOf` are now excluded from contract optimization. Without this, the optimizer could change a function's parameter from `ClassAcc` to `ClassGeneral`, breaking indirect calls that assume standard ABI.

New function `addrTakenFuncs(m *Module)` scans all `OpAddrOf` instructions and returns the set of referenced function names. These are skipped in `OptimizeContracts` alongside `IsExtern` functions.

---

## Showcase Results (Before → After)

```
Program                        R098-Go  R101-Go  R098-Grace  R101-Grace
──────────────────────────── ──────── ──────── ────────── ──────────
01_sum_array                      16       16          16         16
02_sum_array_idiomatic            24       24          24         24
03_filter_map_chain               36       36          36         36
04_lut_popcount                    5        5           5          5
05_four_pointers                  15       13          15         13
06_pbqp_weighted                  19       19          19         19
07_ix_load_store                  18       18          18         18
08_arena_allocator               704      704         702        683
09_function_pointers              19       15          19         15

TOTAL                            856      850         854        829
```

### Key improvements

| Metric | Report 098 | Report 101 | Delta |
|--------|-----------|-----------|-------|
| Go path total | 856 | **850** | **-6** |
| Grace path total | 854 | **829** | **-25** |
| Grace rules fired | 44 | **72** | +28 |
| New rules | — | `flags-only-cmp` (23), `tail-call-opt` (5) |

### Grace Rule Fire Counts (Report 101)

| Rule | Count |
|------|-------|
| flags-only-cmp | **23** |
| dead-store-elim | 19 |
| cond-ret-sink | 13 |
| dead-block-arg | 6 |
| tail-call-opt | 5 |
| split-join-ret | 4 |
| empty-block-elim | 2 |
| **Total** | **72** |

`flags-only-cmp` is now the #1 most-fired rule across the showcase.

---

## C89 vs SDCC 4.2.0 — Scalar Benchmark (unchanged)

| Function | MinZ | SDCC | Delta |
|----------|-----:|-----:|------:|
| `twice(i16)→i16` | 2B | 3B | -1B |
| `add(i16,i16)→i16` | 2B | 3B | -1B |
| `max(i16,i16)→i16` | 12B | 12B | TIE |
| `abs_diff(u8,u8)→u8` | 9B | 11B | -2B |
| `sum_to(i16)→i16` | 21B | 25B | -4B |
| `clamp8(u8,u8,u8)→u8` | 10B | 30B | -20B |
| **Scalar total** | **56B** | **84B** | **-33%** |
| `minmax(u16,u16)→(u16,u16)` | 19B | 61B | -42B |
| `smaller` (uses lo) | 0B | 34B | -34B |
| `larger` (uses hi) | 6B | — | — |
| **Pair return total** | **25B** | **95B** | **-74%** |
| **GRAND TOTAL** | **81B** | **179B** | **-55%** |

MinZ wins on **every function** where SDCC doesn't tie. The `max` function ties because both compilers use the same SBC+ADD compare trick. Note: `max` does NOT benefit from FlagsOnly because the HL value IS used after the comparison (it's the return value).

---

## FatFS Status

| Test | Status |
|------|--------|
| FatFS VM Verify (BPB, FAT, directory, read) | PASS (5/5) |
| FatFS VM Write (mount, open, read, verify) | PASS (7/7) |
| FatFS QBE LowLevel | PASS (33/33) |
| FatFS Z80 codegen | SKIP — `test_follow_chain` semantic bug |
| FatFS Z80 vs SDCC differential | SKIP — same bug blocks |

FatFS compiles through C89→HIR→MIR2 pipeline. VM and QBE backends produce correct results. Z80 backend has one remaining semantic bug in FAT chain traversal (not a codegen issue — the MIR2 VM also gets wrong answer, suggesting HIR lowering or semantic error).

---

## Test Suite Status

**39/44 packages pass** (same as before our changes — 0 regressions).

Pre-existing failures (all verified pre-existing via git stash):
- `pkg/mir2` — `TestContractScale_Long` (assertion too strict for dual-call edge case)
- `pkg/nanz` — `TestFold_Sum_ReturnValue`, `TestFold_LetBinding` (HIR lowering panic)
- `pkg/abap` — `TestSelectionScreen`, `TestParseSimple` (WASM bridge issue)
- `cmd/repl` — build failure (int→uint16 type mismatch)
- `cmd/backend-devkit` — build failure (removed backend references)

---

## Future Optimization Opportunities (FlagsOnly family)

Identified via codegen analysis, same liveness-based pattern:

| Priority | Pattern | Potential savings | Location |
|----------|---------|-------------------|----------|
| HIGH | FlagsOnly for 32-bit CMP (skip 4x PUSH/POP) | 41T, 4 inst | z80codegen.go:5676 |
| HIGH | Dead caller-save (PUSH/POP around CALL) | 21T per pair | z80codegen.go:4234 |
| MED | Mul16 HL save when HL dead | 21T, 2 inst | z80codegen.go:3289 |
| MED | Mul32 triple save (AF/DE/BC) | 42T max | z80codegen.go:3389 |
| LOW | OR A before SBC when carry known clear | 4T | z80codegen.go:5609 |

---

## Files Changed

```
minzc/pkg/mir2/inst.go          +5   FlagsOnly field on Inst
minzc/pkg/mir2/grace_runner.go  +56  flags-only-cmp Grace rule + markFlagsOnly action
minzc/pkg/mir2/z80codegen.go    +6/-16  FlagsOnly codegen + elimSingleJpEqu fix
minzc/pkg/mir2/contracts.go     +29  addrTakenFuncs + ABI pinning
minzc/pkg/nanz/parse.go         +5   Bare function name → AddrOfExpr
```
