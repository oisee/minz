# LIR Pipeline Integration Progress Report

**Date:** 2026-03-17
**Report:** #090
**Branch:** feat/lir-backend
**Author:** MinZ core team

## Summary

This session integrated the LIR constraint-based backend into the MinZ compiler pipeline, achieving 94.6% pass rate across 948 function x machine checks spanning four frontend corpuses (C89, Nanz, Lanz, Lizp). The LIR pipeline runs in parallel with the existing Z80Codegen for convergence testing.

## Commits (8 in this session)

1. **ISLE load combining + MIR2->LIR bridge** — Reduced FatFS `ld_word` from 8 ops to 2 ops through address computation fusion. Bridge translates MIR2 blocks into LIR MIROps.

2. **Multi-block VM + loop rotation** — Extended the LIR VM to handle multi-block functions with branches. Implemented while -> do-while loop rotation, enabling DJNZ pattern matching on Z80.

3. **Z80 machine descriptor + pipeline integration** — Defined the Z80 target with 18 register locations and 29 instruction selection patterns. Wired the machine descriptor into the isel pattern table.

4. **Wire LIR into compiler pipeline** — Connected the LIR pipeline to the existing compiler, running it alongside Z80Codegen. Initial C89 corpus result: 79% pass rate.

5. **Multi-frontend corpus tests** — Extended convergence testing to all four frontends. Results: 84% C89, 77% Nanz, 100% Lanz, 75% Lizp.

6. **WFC spill recovery + 16-bit composite ops + MUL reduction** — Three improvements in one commit:
   - Spill recovery when register LocSets empty (dimension expansion to memory)
   - 16-bit composite patterns: `ADD HL,HL` for SHL16, `SRL H; RR L` for SHR16
   - MUL strength reduction via ISLE: `mul(x,2)` -> `add(x,x)`, `mul(x,3)` -> `add(x,add(x,x))`
   - Result: 97% C89 pass rate.

7. **ISLE term-rewriting DSL for instruction selection** — S-expression rules parsed by the Lanz parser (self-referential: the compiler's own DSL frontend parses its optimization rules).

8. **E2E convergence tests + ADR-0032 DSL landscape** — Four-way convergence testing across RISC32, RISC8, CISC, MICRO machine models.

## Final Corpus Results

| Corpus | Functions | Machines | Checks | Pass | Rate |
|--------|-----------|----------|--------|------|------|
| C89    | 180       | 4        | 720    | 700  | 97.2% |
| Nanz   | ~41       | 4        | 162    | 140  | 86.4% |
| Lizp   | ~14       | 4        | 57     | 49   | 86.0% |
| Lanz   | ~2        | 4        | 9      | 9    | 100%  |
| **Total** | **~237** | **4** | **948** | **898** | **94.6%** |

The 4-way convergence model (RISC32, RISC8, CISC, MICRO) validates that lowering logic is correct independent of register pressure. All 50 failures are in constrained machines (CISC/MICRO) where spill recovery or composite pattern gaps cause mismatches.

## LOC Added

~6100 lines across 29 files, primarily in `minzc/pkg/lir/`.

## Key Innovations

### ISLE Combining Rules (S-expressions, parsed by Lanz)

The optimization rules are written as S-expressions and parsed by the Lanz frontend — the compiler's own DSL infrastructure parsing its own optimization passes. This is a satisfying example of self-referential tooling.

Rules fire before instruction selection, transforming MIR ops into simpler or more efficient forms:
- Load address fusion: `(load (add base (const off)))` -> `(load_offset base off)`
- MUL strength reduction: `(mul x (const 2))` -> `(add x x)`
- Constant folding: `(add (const a) (const b))` -> `(const (+ a b))`

### WFC Spill Recovery (Dimension Expansion)

Traditional register allocators fail when they run out of registers and must restart or use heuristics. Our WFC allocator treats this as a dimension expansion: when a register's LocSet becomes empty, the value is "promoted" to a memory location (stack slot). A reload is inserted before the next use. This is analogous to WFC tile placement expanding into a higher-dimensional search space when local constraints are unsatisfiable.

### 16-bit Composite Patterns

Z80 lacks native 16-bit shift and multiply. The pattern table includes multi-instruction expansions:
- `SHL16(rr)` -> `ADD HL,HL` (single instruction, 11 T-states)
- `SHR16(rr)` -> `SRL H; RR L` (two instructions, 16 T-states)
- `ADD16(rr1, rr2)` -> `ADD HL,rr2` (single instruction, 11 T-states)

### MUL Strength Reduction

Constant multipliers are decomposed into shift+add sequences before isel sees them. This avoids the need for a runtime multiply routine in the majority of cases:
- `x * 2` -> `x + x` (4 T-states vs ~200 T-states for CALL __mul8)
- `x * 3` -> `x + (x + x)`
- `x * 4` -> `x << 2`
- `x * 5` -> `x + (x << 2)`

## Remaining Work

1. **Runtime MUL call** — Non-constant multipliers need `CALL __mul8` / `CALL __mul16` runtime routines. The ISLE rules currently only handle constant factors.

2. **Inter-block WFC propagation** — Current WFC runs per-block. Values live across block boundaries need their LocSets propagated through the CFG to avoid redundant moves at block transitions.

3. **Full assembly emission** — The pipeline currently produces abstract LIR ops with virtual register names. Real Z80 assembly emission with physical register names (A, B, C, D, E, H, L, IX, IY) is needed.

4. **CLI flag** — Wire the LIR backend into the CLI via `--lir` or `--backend=lir` flag so users can opt into the new pipeline.

5. **Remaining failures** — The 50 failing checks (5.4%) are primarily in CISC and MICRO machine models where spill recovery or missing composite patterns cause convergence mismatches. These need targeted pattern additions and spill heuristic improvements.

## Related

- **ADR-0033:** LIR Pipeline Integration (written this session)
- **ADR-0030:** LIR Constraint Architecture
- **ADR-0031:** Rule Discovery and WFC Solver
- **ADR-0032:** DSL Landscape
- **Report #088:** LIR Backend Foundation (2026-03-17)
