# Session Wisdom: GPU Regalloc Pipeline + VIR Fixes

**Date:** 2026-03-25 (session 3, continuation)

---

## Key Discoveries

### 1. GPU Regalloc — Synthetic vs Real Signatures
123K synthetic patterns enumerated in 40s on dual GPU — but ZERO hits on real programs. Root cause: synthetic enumerator generates all pattern combinations, but real VIROps get specific pattern subsets from Matches() filtering. The SHA256 signatures don't match. Fix: extract real signatures from corpus compilation (`VIR_DUMP_GPU_BATCH=1`), pipe to GPU, build table from actual constraint graphs.

### 2. Width-Aware GPU Kernel
GPU kernel originally treated all vregs as 8-bit — picked H(5) for u16 vregs. Fix: add `"widths": [8, 16, ...]` field to GPU JSON. Kernel rejects 8-bit-only locs for 16-bit vregs (restrict to BC=7, DE=8, HL=9, mem0=14). One-line filter in CUDA feasibility check.

### 3. Skip Z3 for Table Hits — Direct PIR Emit
GPU assignment → Z3 fails because ParamLocs conflicts with pattern DstLocs. The correct approach: bypass Z3 entirely for table hits. GPU gives vreg→loc, linear scan ~40 patterns to find matching dst/src locs, emit PIR directly. Zero solver. Z3 only for table misses (>8 vregs).

### 4. VIR Fix Root Causes
- **ADD DE,HL**: Grace EX DE,HL sandwich elimination was swapping HL↔DE in ADD (HL-only Z80 op). Fix: skip sandwich for ADD HL/ADC HL/SBC HL.
- **Blank output**: NOT a VIR bug for simple programs — those work. Complex programs (interactive PARAMETERS) have stale A values in zext patterns. Deeper codegen issue.
- **String pool**: spliceVIRFallback missing EmitStringPool — our fix, confirmed working.
- **Width constraints**: Explored in Z3 solver but reverted (too aggressive). Fixed via cross-width findMovePattern + new truncation patterns instead.

### 5. Console I/O Port for ZX Spectrum Input
mze spectrum target: `SetConsolePort(0x23, os.Stdin, os.Stdout)`. Protocol: `IN A, ($23)` returns `0x80|byte` or `0x00`. Race condition with piped stdin resolved by poll loop (retry on 0x00). Works for interactive ABAP PARAMETERS on ZX.

### 6. ABAP Target-Aware Lowering
Must set `hm.Target` BEFORE `l.lower()`, not after. Created `LowerProgramWithTarget(prog, target)`. Guards needed for: sel_register_* (skip on ZX), sel_show (skip on ZX), sel_get_int (skip on ZX), abap_write_str/abap_write (ZX variants), abap_sel_read/abap_read_int (ZX variants with port $23).

---

## Cross-Session Coordination Protocol

Three teams worked in parallel via dedelulu messaging:
- **minz (us)**: ABAP features, testing, integration
- **minz-vir (cekgp49j)**: VIR solver fixes, GPU table integration, Grace optimization
- **z80-optimizer (x65k8mpc)**: CUDA kernel, GPU server, width support

Effective pattern: report bug with exact line context → receive fix → verify → report next issue. Cycle time: ~10 minutes per fix.

---

## GPU Regalloc Architecture (Final)

```
CodegenFunc(f):
  ops = bridge(f)           # MIR2 → VIR ops
  sig = ComputeSignature(ops)
  if entry = table.Lookup(sig):
    pir = directPIREmit(ops, entry.assignment)  # pattern select, no solver
    return emit(pir)
  else:
    # Table miss — try GPU server (if available)
    if gpuServer.available:
      json = extractConstraintJSON(ops)
      result = gpuServer.solve(json)
      table.Add(sig, result)
      pir = directPIREmit(ops, result.assignment)
      return emit(pir)
    # Fallback to Z3
    return z3Solve(ops)
```
