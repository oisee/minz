# Context: mzd --regs IN/OUT/CLOBBER annotations
- **Date:** 2026-03-15
- **Host:** macbookairm2
- **Branch:** master
- **Status:** Feature complete, all tests pass

## What was done

### 1. `mzd --regs` — per-function register annotations
Added `--regs` flag to mzd that computes and displays IN/OUT/CLOBBER annotations for every detected function.

**Files created/modified:**
- `minzc/pkg/disasm/analysis/regtrack.go` — NEW: register analysis engine
- `minzc/pkg/disasm/analysis/regtrack_test.go` — NEW: 13 tests
- `minzc/pkg/disasm/analysis/analysis.go` — added `FuncRegs` field
- `minzc/cmd/mzd/main.go` — added `--regs` flag, rendering, analysis call

### 2. Provenance tracking
Instead of simple "read before write" bitmask, tracks the **origin** of each register's value through the function:
- `EX DE,HL` → swaps provenance (D↔H, E↔L), no consumption
- `LD r, r'` → transfers provenance from source to dest
- Non-move instructions → consume provenance (record as input), produce new values
- PUSH/POP pairs → save/restore provenance (register preservation detection)

### 3. ABI-aware CALL consumption
For known syscalls (BDOS, Spectrum ROM, MSX BIOS, Agon MOS):
- Resolves dispatch register (e.g., C for BDOS)
- Looks up function params from ABI profile
- Only marks actually-consumed registers as inputs

For unknown callees: conservative (all regs consumed).

## Example output
```
; ---- sub_011A ($011A) ----
; IN: HL  OUT: —  CLOBBER: BC, DE, A, F
011A: EB          EX DE,HL
011B: 0E 09       LD C,$09
011D: CD 05 00    CALL BDOS
0120: C9          RET
```
Note: IN is correctly `HL` (not `HL, DE`) because provenance traces through EX DE,HL.

## Known limitations
1. **Linear scan** — doesn't handle control flow splits (conditional branches with different register usage on each path)
2. **Unknown callees** — conservatively assumes all regs consumed (sub_0100/main shows false inputs)
3. **OUT detection** — backward scan from RET, heuristic only
4. **IX/IY** — not tracked in RegSet (effects on A-L are tracked)

## Next steps (discussed with neighbor)
1. **Inter-procedural analysis** — use analyzed function's IN set as callee ABI for internal CALLs
2. **SLD integration** — feed source-level labels from compiler into mzd for function names
3. **ABI comparison** — compare mzd-computed IN/OUT/CLOBBER with MIR2 contract from HIR→MIR2 lowering for automated codegen verification:
   ```
   HIR → MIR2 → contract (expected ABI)
                     ↕ compare
   Z80 asm → mzd analysis (actual ABI)
   ```
   Mismatch = codegen bug.

## Test results
```
ok  github.com/minz/minzc/pkg/disasm          0.004s
ok  github.com/minz/minzc/pkg/disasm/analysis  0.005s
```
All 13 regtrack tests pass including provenance-specific tests.
