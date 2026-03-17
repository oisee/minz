# ADR-0033: LIR Pipeline Integration

**Status:** Accepted
**Date:** 2026-03-17
**Deciders:** MinZ core team
**Relates to:** ADR-0030 (LIR Constraint Architecture), ADR-0031 (Rule Discovery & WFC Solver), ADR-0032 (DSL Landscape)

## Context

The existing MIR2 -> Z80Codegen pipeline is production-ready (71/73 core examples, 131/173 total). However, the register allocator remains a bottleneck: it overwrites operands in loops, uses memory-backed virtuals causing ~5x overhead, and struggles with Z80's asymmetric register file.

We need a next-generation backend that can:
- Perform instruction combining (fusing load sequences, strength-reducing MUL)
- Allocate registers with awareness of Z80's irregular constraints (A-only for arithmetic, HL for addressing, BC/DE as pairs)
- Prove correctness through multi-architecture convergence testing

Rather than replacing the working codegen, we build a parallel pipeline and validate through convergence.

## Decision

Build a constraint-based LIR backend that runs **in parallel** with the existing Z80Codegen for convergence testing.

### Architecture

```
MIR2 -> LowerMIR2Block -> MIROps -> Combine(ISLE) -> isel(PatternTable) -> WFC -> emit
```

### Key Design Choices

#### 1. Parallel execution, not replacement

LIR runs alongside Z80Codegen. Both consume the same MIR2 input. Results are compared for convergence: if both backends produce the same observable behavior, confidence in the new pipeline increases. The existing codegen remains the production path until LIR reaches full parity.

**Rationale:** The existing Z80Codegen works for 97% of core examples. Replacing it risks regressions. Running in parallel lets us validate incrementally.

#### 2. ISLE term-rewriting for instruction combining

Inspired by Cranelift's ISLE (Instruction Selection/Lowering Expressions), we use S-expression rules for pattern matching and rewriting at the MIR op level. Rules are parsed by the Lanz parser (self-referential: the compiler's own DSL frontend parses its optimization rules).

Example combining rules:
- `(load (add base (const off)))` -> `(load_offset base off)` — fuses address computation into load
- `(mul x (const 2))` -> `(add x x)` — strength reduction before isel
- `(mul x (const 3))` -> `(add x (add x x))` — further MUL reduction

This reduced FatFS `ld_word` from 8 ops to 2 ops.

#### 3. WFC (Wave Function Collapse) for register allocation

Each virtual register starts with a full LocSet of possible physical locations. Constraints propagate in three passes:

- **Forward propagation:** Instruction semantics constrain outputs (e.g., ADD result must be in A)
- **Backward propagation:** Uses constrain their sources (e.g., CALL argument must be in specific reg)
- **Vreg consistency:** Same virtual must resolve to the same physical across all references

When a LocSet is reduced to a single element, the register is allocated (collapsed). Ties are broken by heuristic priority.

#### 4. Spill recovery via dimension expansion

When a register's LocSet becomes empty (no physical register available), the allocator does not fail. Instead, it **expands the dimension** from register to memory: the value is spilled to a stack slot, and a reload is inserted before the next use. This is more principled than traditional "spill and retry" approaches.

#### 5. Four-way convergence testing

Every function is lowered to four machine descriptors:
- **RISC32:** 16 registers, orthogonal ISA
- **RISC8:** 8 registers, orthogonal ISA
- **CISC:** Accumulator-centric, register pairs (Z80-like)
- **MICRO:** 4 registers, very constrained

If the computed result matches across all four machines, the lowering logic is correct regardless of machine-specific register allocation. This catches bugs that only manifest under tight register pressure.

#### 6. Loop rotation

Header-tested `while` loops are rotated to bottom-tested `do-while` form:

```
// Before: while (cond) { body }
//   test cond; branch_false exit; body; jump top
//
// After: if (cond) { do { body } while (cond) }
//   test cond; branch_false exit; body; test cond; branch_true top
```

This eliminates one branch per iteration and enables DJNZ (Decrement and Jump if Not Zero) pattern matching on Z80, which is critical for tight loops.

#### 7. Z80 composite patterns

Z80 has no native 16-bit shift or multiply. The isel pattern table includes composite patterns that expand to multi-instruction sequences:

- `SHL16(rr)` -> `ADD HL,HL` (left shift by 1)
- `SHR16(rr)` -> `SRL H; RR L` (logical right shift by 1)
- `ADD16(rr1, rr2)` -> `ADD HL,rr2` (16-bit add)

The machine descriptor declares 18 locations and 29 patterns for the Z80 target.

#### 8. MUL strength reduction

Constant multipliers are reduced to shift+add sequences via ISLE rules, firing before instruction selection:

- `mul(x, 2)` -> `add(x, x)`
- `mul(x, 3)` -> `add(x, add(x, x))`
- `mul(x, 4)` -> `shl(x, 2)`
- `mul(x, 5)` -> `add(x, shl(x, 2))`

Non-constant multipliers fall through to a runtime `CALL __mul8` (not yet implemented).

## Current Status

94.6% pass rate on 948 function x machine checks across four frontend corpuses:

| Corpus | Pass | Total | Rate |
|--------|------|-------|------|
| C89    | 700  | 720   | 97.2% |
| Nanz   | 140  | 162   | 86.4% |
| Lizp   | 49   | 57    | 86.0% |
| Lanz   | 9    | 9     | 100%  |
| **Total** | **898** | **948** | **94.6%** |

## Alternatives Considered

### Replace Z80Codegen entirely

Replacing the production codegen with LIR would be the cleanest architecture, but it carries unacceptable risk. The existing pipeline handles 97% of core examples and is battle-tested. A parallel approach lets us validate without risking regressions.

**Decision:** Rejected. Too risky for a production compiler.

### LLVM-style SelectionDAG

SelectionDAG is a proven approach for instruction selection, used by LLVM for decades. However, it is designed for large, orthogonal register files (x86-64, AArch64). Z80's asymmetric constraints (A-only arithmetic, HL-only addressing, pair registers) make DAG-based selection overly complex.

**Decision:** Rejected. ISLE term-rewriting is simpler and more natural for constrained ISAs.

### Linear scan register allocation

Linear scan is fast and well-understood, used by many production compilers. However, it assumes a mostly-orthogonal register file where any register can hold any value. Z80's constraints (specific registers required for specific operations) make linear scan produce excessive spills.

**Decision:** Rejected. WFC is more natural for asymmetric architectures where each register has a distinct personality.

## Consequences

### Positive
- Instruction combining reduces op count significantly (8 -> 2 for FatFS patterns)
- WFC handles Z80 asymmetry naturally without special-casing
- Convergence testing catches lowering bugs early across 4 machine models
- ISLE rules are declarative and auditable
- Parallel execution means zero risk to production codegen

### Negative
- Two codegen paths to maintain during transition period
- WFC is non-standard; new contributors need to learn the approach
- ISLE rule ordering can be subtle (first-match semantics)

### Neutral
- ~6100 LOC added across 29 files
- Build time increase is negligible (LIR pipeline is fast)

## Related Documents

- ADR-0030: LIR Constraint Architecture
- ADR-0031: Rule Discovery and WFC Solver
- ADR-0032: DSL Landscape
- `minzc/pkg/lir/` — LIR implementation
