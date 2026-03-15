# MZD Register Analysis & ABI Verification

**Date:** 2026-03-15
**Status:** Shipped (2 commits: `1679c5a`, `1fc4cfa`)

---

## Summary

Added two new capabilities to `mzd` disassembler:

1. **`--regs`** — Per-function IN/OUT/CLOBBER register annotations with provenance tracking
2. **`--verify-abi`** — Automated comparison of detected register usage against compiler-declared ABI from `.a80` comments

Together they form a **closed-loop codegen verification pipeline**:

```
MinZ source
    ↓ mzc
program.a80 (ABI comments: '; fun name(x: u8 = A) -> u8 = A ; clobbers: BC')
    ↓ mza
program.bin
    ↓ mzd --regs --verify-abi program.a80
    ↓
"ABI verify: 5/5 functions matched, all OK"   ← or mismatch = codegen bug
```

---

## Feature 1: `mzd --regs` — Register Annotations

### What it does
For every detected function, computes and displays:
- **IN** — registers whose entry values are consumed (function inputs)
- **OUT** — registers written before RET (return values)
- **CLOBBER** — registers modified but not preserved

### Example
```
; ---- sub_011A ($011A) ----
; IN: HL  OUT: —  CLOBBER: BC, DE, A, F
011A: EB          EX DE,HL
011B: 0E 09       LD C,$09
011D: CD 05 00    CALL BDOS
0120: C9          RET
```

### Provenance tracking
Simple "read before write" analysis fails on register transfers:
- `EX DE,HL` swaps registers — naive analysis reports both DE and HL as inputs
- `LD E, A` copies A to E — need to trace the value, not just the register

**Solution:** Each register carries a *provenance tag* — which entry-state register's value it currently holds.

| Instruction | Effect on provenance |
|-------------|---------------------|
| `EX DE,HL` | Swap D↔H, E↔L provenance (no consumption) |
| `LD r, r'` | Transfer provenance from source to dest |
| `ADD A, B` | Consume A and B provenance, produce new A (prov=nil) |
| `LD A, $42` | Produce new A (prov=nil) |
| `PUSH/POP` pair | Save/restore provenance (register preservation) |

### ABI-aware CALL consumption
When a CALL targets a known syscall (BDOS, Spectrum ROM, MSX BIOS, Agon MOS), only the callee's declared parameter registers are consumed. This prevents false inputs.

Example: `EX DE,HL; LD C,$09; CALL BDOS`:
- BDOS #9 reads C and DE
- Provenance: D=H, E=L (from EX), C=nil (immediate)
- Consumed: D(prov=H) + E(prov=L) → **IN: HL** (correct, not DE)

---

## Feature 2: `--verify-abi` — Automated ABI Comparison

### What it does
1. Parses `; fun name(param: type = REG) -> type = REG ; clobbers: REG, ...` comments from compiler-generated `.a80` files
2. Assembles the `.a80` with the built-in assembler to get symbol→address mapping
3. Compares declared IN/OUT/CLOBBER against detected values
4. Reports mismatches

### Usage
```bash
# Compile
mzc program.minz -o program.a80
mza program.a80 -o program.bin

# Verify
mzd --regs --verify-abi program.a80 program.bin
```

### Output on match
```
ABI verify: 5/5 functions matched, all OK
```

### Output on mismatch
```
ABI verify: 5/5 functions checked, 2 mismatches:
  board_set            IN: extra=D (declared=A, C, B detected=A, C, B, D)
  can_place            CLOBBER: extra=BC (declared=E, H, L detected=E, H, L, BC)
```

This means `board_set` in the binary reads D (which the compiler didn't declare as a parameter) — possible codegen bug where D carries a stale dependency.

---

## Test Results

| Suite | Result |
|-------|--------|
| `pkg/disasm` | PASS |
| `pkg/disasm/analysis` | PASS (35 tests incl. 13 regtrack + 9 abi_verify) |
| ex29_lanz | 3/3 match |
| ex30_ptr_cast | 3/3 match |
| ex31_asm_ret_clob | 3/3 match |
| ex32_value_pipe | 5/5 match |

---

## Files Changed

### New files
| File | LOC | Purpose |
|------|-----|---------|
| `pkg/disasm/analysis/regtrack.go` | ~850 | Register analysis engine with provenance |
| `pkg/disasm/analysis/regtrack_test.go` | ~200 | 13 tests |
| `pkg/disasm/analysis/abi_verify.go` | ~250 | .a80 ABI parser + comparison |
| `pkg/disasm/analysis/abi_verify_test.go` | ~180 | 9 tests |
| `contexts/2026-03-15_macbookairm2_mzd_regs.md` | — | Session context |

### Modified files
| File | Change |
|------|--------|
| `pkg/disasm/analysis/analysis.go` | Added `FuncRegs` field |
| `cmd/mzd/main.go` | Added `--regs`, `--verify-abi` flags + rendering |

---

## Known Limitations

1. **Linear scan** — doesn't handle control flow merges (conditional branches with different register usage per path)
2. **Unknown callees** — internal function calls conservatively assume all regs consumed (inter-procedural analysis TODO)
3. **OUT detection** — heuristic backward scan from RET, may miss complex return patterns
4. **IX/IY** — not tracked in RegSet (but effects on A-L are tracked through DD/FD opcodes)
5. **Tetris .a80** — some advanced instructions fail mza assembly (LD IX+d syntax), verify-abi needs mza fixes

---

## Future Work

1. **Inter-procedural analysis** — use analyzed function's IN set as callee ABI for internal CALLs (eliminates false positives in `main()`)
2. **CI integration** — `mzd --verify-abi` as post-compile check in test pipeline
3. **SLD integration** — source-level labels for richer disassembly output
4. **Graph export** — call graph with IN/OUT/CLOBBER edges for visualization
