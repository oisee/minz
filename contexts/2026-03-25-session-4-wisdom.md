# Session Wisdom: SQL on Z80 CP/M + MZA Multi-Pass Fix

**Date:** 2026-03-25 (session 4)

---

## The MZA Bug: pass == 2 vs pass >= 2

**Root cause:** MZA's multi-pass assembler only emitted DB/DW/DS data on `a.pass == 2`. When JRS (Jump Relative Short) expansion caused instruction sizes to change, the assembler ran pass 3 for convergence. Pass 3 skipped all data emission → strings and globals produced 0 bytes in the binary.

**Fix:** Change all `a.pass == 2` to `a.pass >= 2` in EmitByte, EmitWord, handleDB, handleDW, handleDW24, handleDS.

**Impact:** Binary went from 923 → 1465 bytes. ZSQL strings appear in binary. ALL VIR-compiled programs with data sections now work.

**Lesson:** Multi-pass assemblers need "final pass" semantics, not "pass N" semantics. Any pass that rebuilds output should emit data.

## VIR OpAsmBlock: CFG Solver Path

VIR's CFG solver had a separate PIROp construction path that didn't copy AsmTemplate from the VIROp. Fix: copy AsmTemplate in cfgsolver.go. Per-block solver already had this fix.

Functions affected: readline, getinput, terminate — all had asm bodies that compiled to just `RET`.

## VIR Asm Emission: Duplicate Instructions Dropped

VIR drops the second `INC HL` from `INC HL / INC HL` in asm blocks. The asm emitter may be deduplicating adjacent identical instructions. Needs investigation — affects getinput() which needs _inbuf+2 offset.

## Phase Transition: The Figure 1

z80-optimizer ran the corpus with MAX_LOCS from 3 to 32:
- 3 locs (6502): 19 vregs solvable → 100% coverage
- 8 locs (GameBoy): 10 vregs → 93%
- 15 locs (Z80): 8 vregs → 81%
- 16 locs (Thumb): 7 vregs → ~70%
- 32 locs (RISC-V): 6 vregs → ~50%

Cliff at ~16 locations. Below: exhaustive tables cover majority. Above: only small functions.

## Corpus Transfer: 88.2%

Train on Nanz+PL/M → Test on ABAP+Screen+SQLite+Lizp+FS:
- ABAP: 98.8% hit rate
- Screen: 89.6%
- SQLite: 45.9% (unique I/O patterns)
- Only 38 new signatures needed for 100%

## Research Programme

- Paper A: GPU exhaustive + entropy + phase transition (data ready)
- Paper B: Island decomposition + treewidth bounds (ADR-0040 written)
- Paper C: Exhaustive search certificates (divmod lower bound ≥13)

Key reframing: "Irregularity as Structure" — Z80's weird constraints REDUCE ambiguity, making the space MORE compressible. Architecture irregularity helps exhaustive solvers.
