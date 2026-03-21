# Report 105: Z80 Codegen Corpus Validation & TSMC Spill

**Date:** 2026-03-21
**Session:** minz-parallel
**Commits:** 12

## Summary

Corpus-driven Z80 codegen bug hunt: compiled all 370 source files through MIR2 backend, identified and fixed 6 classes of invalid Z80 instructions. Introduced Z80-VALIDATE for MIR2, retry loop for LIR, and world-first TSMC spill optimization.

## Results

| Metric | Before | After |
|--------|--------|-------|
| Files with Z80 errors | **45** | **22** |
| Compile regressions | — | **0** |
| Error classes eliminated | — | **6** |
| TSMC spill pairs activated | — | **16** (NC) |

## Error Classes Fixed

| Class | Before | After | Fix |
|-------|--------|-------|-----|
| `LD A, HL` (pair to 8-bit) | 302 | 58 | emitSingleCopy width guard |
| `LD DE, C` (8-bit to pair) | 70+ | 0 | genShift uses emitMov |
| `ADD BC/DE, rr` | 752 | 0 | Route through HL |
| `LD AF,0` / `SBC HL,AF` | 220 | 0 | promote8toPair(A) to BC |
| `LD r, F` (flag read) | 470 | 0 | SBC A,A materialisation |
| `LD mem, X` (unresolved spill) | 160+ | 0 | physName() helper |
| `LD $F0xx, imm` / `INC $F0xx` | 142 | 0 | OpConst spill + INC guard |
| False positives (LD label) | ~30 | 0 | Validator LD (label) support |
| DD/FD prefix conflicts | ~130 | partial | IX/IY shift + load/store |

## New Features

### 1. Z80-VALIDATE for MIR2 (`pkg/z80validate/`)
- Shared validation package (no circular imports)
- Per-function validation after emit
- Shows actual invalid instruction in error output
- Used by both MIR2 and LIR backends

### 2. LIR Validate-Reject-Retry Loop
- After Z80-VALIDATE finds errors, rejects phys assignments
- Re-runs WFC collapse with tighter constraints
- Max 3 attempts before fallback to warn-only
- `LocSet.Add()` / `LocSet.Subtract()` for constraint manipulation

### 3. TSMC Spill (World-First)
Self-modifying code as register spill mechanism:

```asm
; Traditional spill (26T round-trip):
LD ($F000), A     ; store to memory (13T)
LD A, ($F000)     ; reload from memory (13T)

; TSMC spill (20T round-trip, -23%):
LD (_tsmc+1), A   ; patch reload immediate (13T)
_tsmc: LD A, 0    ; immediate pre-patched (7T)
```

For `emit8ALU` (AND/OR with spill): **77% faster** (11T vs 47T).

**Safety:** non-recursive functions only (verified by call graph analysis).

### 4. TUI Tetris (`examples/nanz/tetris_tui.nanz`)
Full Tetris using tui_* host functions. Runs in mzv terminal and mze CP/M.
Arrow keys, Z=rotate, X=drop, C=hold, Q=quit.

### 5. ZX Console Library (`stdlib/zx/console.nanz`)
Standalone text console using ROM font at $3D00. No ROM calls, no IM1.
- `con_print()`, `con_putchar()`, `con_getchar()`, `con_input()`
- Direct screen memory writes + keyboard matrix scanning
- Scrolling, cursor, attribute colors

## Commits

| Hash | Description |
|------|-------------|
| `9c92cbb6` | TUI Tetris + ZX console lib + ClassGeneral fix |
| `cd519451` | Z80-VALIDATE for MIR2 + LIR retry loop |
| `ded93f5f` | Width mismatch: LD A,HL to LD A,L (-81%) |
| `1ed208a4` | AF pair + ADD non-HL pair (eliminated) |
| `805b0986` | F register materialisation (SBC A,A) |
| `c49e8290` | genShift width mismatch + validator shows instructions |
| `275e5269` | DD/FD prefix conflicts + validator false positives |
| `2dcd3e41` | OpTrunc DD/FD + validator LD (label) |
| `8d913096` | physName() for LocMem (eliminates LD mem, X) |
| `c8916f86` | Spill codegen: $F0xx store/INC/ALU |
| `92b35308` | TSMC spill: self-modifying code register spill |

## Remaining (22 files)

Dominated by FAT12 library (high register pressure):

1. **`LD r, $F0xx`** (load spill to register) — needs TSMC reload expansion
2. **`ADD/SBC HL, $F0xx`** — load to pair first
3. **`LD IXH, H`** / `LD IXH, IYH` — DD/FD NOP conflicts in non-emitMov paths
4. **`SBC DE, DE`** / `SBC BC, BC` — only SBC HL,HL is valid

## Architecture Notes

### Z80 Flag Materialisation Strategy
| Flag | Pattern | Cost |
|------|---------|------|
| C (carry) | `SBC A, A` | 4T |
| Z (zero) | `JR NZ,$+3; SCF; SBC A,A` | ~15T |
| Immediate consumption | `JR C,`/`JR Z,` directly | 0T |
| Temporary save | `EX AF,AF'` | 4T (1-deep) |

### TSMC Spill Eligibility
- Non-recursive (call graph verified)
- Max 3 reload sites (cost: 13T * N spill + 7T * N reload)
- 8-bit and 16-bit values supported
- Block-param spills excluded (loop-variant)

### Future: TSMC Spill to Immediate
Even more aggressive: spill directly into instruction immediate bytes.
Requires Grace/Datalog proof: `non_recursive(F) :- func(F), !calls(F, F).`
