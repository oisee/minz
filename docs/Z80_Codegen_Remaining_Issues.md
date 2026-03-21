# Z80 Codegen — Remaining Issues

**Date:** 2026-03-21
**Status:** 45 → 8 files with Z80 errors (−82%), 30 commits
**Session:** minz-parallel

## Remaining Files

| File | Errors | Root Cause |
|------|--------|-----------|
| ff.c (FatFS) | 246 | Massive spills (30+ live vars), IX↔IY cross-prefix |
| nc.nanz | 66 | FAT12 lib spills, IY routing, ADD HL,IY |
| 08_arena_allocator.nanz | 8 | F register allocated as data counter |
| sieve.pas | 2 | Pointer allocated to 8-bit register |
| editor_demo.minz | 2 | Inline asm unresolved symbols (false positive) |
| fatfs_lowlevel.c | 1 | DD cross-byte in remaining path |
| dowhile_break.c | 1 | DD NOP in remaining OpTrunc path |
| 04_lut_popcount.nanz | 1 | ^H syntax not supported by validator |

## Root Cause Analysis

### 1. Register Pressure (ff.c, nc.nanz) — 312 errors
Functions like `f_write`, `mount_volume`, `get_fat` have 20-30+ simultaneously live variables. Z80 has 7 GPRs. PBQP spills to `_spill_` labels, generating `LD r, _spill_` / `LD _spill_, r` which must route through A.

**Fix:** HIR Function Splitting — auto-split high-pressure functions into sub-functions with ≤8 live vars each. See `docs/HIR_Function_Splitting.md` (designed, not yet implemented).

### 2. PBQP Constraint Gaps (arena, sieve) — 10 errors
- **F register as data:** PBQP allocates counter to F (INC F / DEC F invalid). F should have `InfCost` for all non-ClassFlag allocations.
- **Pointer in 8-bit reg:** PBQP allocates TyPtr to single 8-bit register (C/D). Pointers must be in pairs (HL/DE/BC/IX/IY). `locCompatible` should reject single-char LocReg for TyPtr.

**Fix:** Two-line changes in `pkg/mir2/alloc.go:locCompatible` and `pkg/mir2/z80cost.go`.

### 3. DD/FD Prefix in Remaining Paths (dowhile, fatfs_lowlevel) — 2 errors
`LD L, IXL` / `LD L, IXH` from code paths not yet covered by `emitMovViaAltA`. These are in `genBinOp` 16-bit OR/AND/XOR byte-operations and `genCmp16` edge cases.

**Fix:** Add `isIXYReg` guards in remaining raw `LD %s, %s` emit sites, or use `emitLD8` universally.

### 4. Validator False Positives (editor_demo, lut_popcount) — 3 errors
- `LD A, color` / `OR mask` — inline asm with symbolic operand names
- `LD H, popcount_lut^H` — MZA doesn't support `^H` (high byte of address)

**Fix:** Add `^H`/`^L` support to MZA operand parser, and teach validator about inline asm symbolic names.

## Architectural Solutions (Priority Order)

### Priority 1: PBQP Constraints
```go
// alloc.go:locCompatible — reject F for non-flag, reject 8-bit for pointers
case LocReg:
    if loc.Name == "F" { return cls == ClassFlag }  // F is flags-only
    if ty == TyPtr && len(loc.Name) == 1 { return false }  // ptr needs pair
```

### Priority 2: HIR Function Splitting
See `project_hir_splitting.md` in memory. `pkg/hir/split.go` (~300 LOC).
Auto-split when `estimatePressure(fn) > 8`. Each sub-function gets independent PBQP allocation. PFCCO optimizes inter-function contracts automatically.

### Priority 3: TSMC Spill V2
Expand TSMC coverage to more reload patterns (genBinOp 16-bit, genCmp16 lhs/rhs setup). Currently covers OpConst and emitMov paths.

### Priority 4: Universal emitLD8
Replace ALL remaining raw `g.emitf("    LD %s, %s", ...)` with `g.emitLD8(dst, src)` which handles spills, IXY conflicts, and F register automatically.

## What Was Fixed (This Session)

| Error Class | Before | After |
|-------------|--------|-------|
| LD A,HL (pair→8bit) | 302 | ~0 |
| ADD BC/DE,rr | 752 | 0 |
| LD AF,0 / SBC HL,AF | 220 | 0 |
| LD r,F (flag read) | 470 | 0 |
| LD mem,X (unresolved spill) | 160+ | 0 |
| LD $F0xx,imm / INC $F0xx | 142 | 0 |
| TSMC label false positives | 200 | 0 |
| DD/FD NOP conflicts | 130 | ~10 |
| SBC DE,rr (non-HL) | 16 | 0 |
| Register name collision | 7 | 0 |

## New Infrastructure

- **Z80-VALIDATE** for MIR2 (`pkg/z80validate/`) — per-function validation
- **LIR validate-reject-retry** — up to 3 WFC re-collapse attempts
- **TSMC Spill** — self-modifying code register spill (world-first)
- **Named spill labels** — `_spill_{func}_r{N}` with DB/DW in data section
- **EX AF,AF' routing** — shadow A for DD/FD conflicts and spill ops
- **emitLD8** — universal 8-bit LD with spill/IXY/F guards
- **emitMovViaAltA** — DD/FD NOP conflict resolver
- **promote8toPair** — safe 8→16 promotion (A→BC, not AF)
