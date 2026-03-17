# Report #088 — LIR Backend Foundation

**Date:** 2026-03-16 → 2026-03-17
**Branch:** `feat/lir-backend`
**Machine:** parallel session

---

## Summary

Built the foundation for a constraint-driven, data-driven compiler backend
that replaces 6K lines of if-chains with declarative rules and WFC solver.

## What Was Built

### pkg/lir/ — 3,018 LOC, 24 tests, all pass

| Component | LOC | Purpose |
|-----------|-----|---------|
| lir.go | 165 | LocSet bitfield, MachineDesc, Pattern, ConstraintRule |
| machines.go | 350 | 4 architectures: RISC32, RISC8, CISC, MICRO |
| isel.go | 170 | Pattern-table instruction selection + setup moves |
| vm.go | 120 | LIR interpreter for convergence testing |
| wfc.go | 230 | Wave Function Collapse: forward+backward+collapse |
| rules.go | 300 | Datalog facts (30 Z80 facts), 10 IR patterns, 4 rewrites |
| isle/isle.go | 290 | ISLE term-rewriting DSL: parser+matcher+rewriter |
| tests | ~850 | 24 tests across 6 test files |

### Architecture Zoo — 4-way Convergence

```
RISC32 (32 regs, symmetric)     ── all produce same ──┐
RISC8  (8 regs, pressure)       ── result for same  ──┤ ← convergence = correctness
CISC   (Z80-like, asymmetric)   ── MIR program       ──┤
MICRO  (6502-like, 3 regs)      ──────────────────────┘
```

Verified: add(10,32)=42, sub(100,37)=63, (0xFF&0x0F)|0xF0=255, chain(40+3-1)=42

### ISLE DSL

Z80 instruction selection as declarative rules:
```lisp
(rule 10 (lower (add u8 ?x (const 1)))
         (z80_inc (gpr8 ?x)))

(rule (lower (add u8 ?x ?y))
      (z80_add_a (put_in_a ?x) (gpr8 ?y)))
```
Priority system, pattern variables, recursive rewriting, nested matching.

### WFC Solver

LocSet bitfield constraint propagation:
- Forward: def→use (what CAN be here)
- Backward: use→def (what MUST be here)
- Liveness-aware collapse (no double-assignment)
- Converges in 2 iterations on tested programs

### ADRs Written

- ADR-0030: LIR + constraint-driven backend architecture
- ADR-0031: Rule discovery, verification, and WFC solver
- ADR-0032: DSL landscape (ISLE, TableGen, Datalog, egg)

## Earlier Session Work (same day)

### Allocator & Codegen Fixes (master)
- PBQP R1 back-assignment fix — zero spills on 9+ live regs
- BUG-008 CLOSED — all DD prefix conflicts eliminated
- Z80 SHL fix, genCmp16 guards for F/8-bit operands
- GEP builder + embedded struct + backward goto

### Function Pointers — 6/7 frontends (master)
C89, Lanz, Lizp, Nanz, Pascal, PL/M — all emit CallIndirectExpr
ADR-0029: standard callable ABI

### Emulator Tooling (master)
- MZX true headless (build tag, no GLFW)
- MZV frame dump (SCR format)
- RZX parser/writer + replay integration

## Next Steps

1. **MIR2→LIR bridge** — convert MIR2 Func → ISLE terms → LIR insts
2. **5-way convergence** — MIR2-VM == LIR-VM(×4) on real programs
3. **Z80 MachineDesc** — CISC + DD prefix rules + shadow regs
4. **Spill support** — for MICRO when regs < live vars
5. **ISLE rule files** — .isle files per target, loaded at compile time
