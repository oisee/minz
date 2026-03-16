# ADR-0031: Rule Discovery, Verification, and WFC Solver

**Status:** Draft
**Date:** 2026-03-16
**Builds on:** ADR-0009 (superoptimizer), ADR-0027 (constraint isel), ADR-0030 (LIR)

## Problem

ADR-0030 defined three DSL layers (Facts, Patterns, Rewrites) and a WFC-based
solver. But WFC only **applies** rules — it doesn't **discover** them.

The full system needs three capabilities:
1. **Discovery** — find optimization opportunities and encoding constraints
2. **Verification** — prove rules are correct across all inputs
3. **Application** — use rules during compilation (WFC constraint propagation)

Currently, all three are done by humans writing Go code. We need to automate
discovery and verification, then express results as declarative rules that
the WFC solver applies mechanically.

## Architecture: Three Systems

```
DISCOVER ──→ VERIFY ──→ APPLY
   │            │          │
   │            │          └─ WFC solver (bitfield AND/OR)
   │            └─ Convergence testing + exhaustive state enum
   └─ Superoptimizer + convergence failure analysis
```

### System 1: Rule Discovery

**Source A: ISA specification (manual, one-time)**
Human reads Z80/6502/etc manual and writes Datalog facts:
```prolog
% From Z80 manual, section "Undocumented Opcodes"
forbidden(dst=IXH, src=ix_indirect).
alias(HL, H). alias(HL, L).
acc_only(ADD8). acc_only(SUB8).
```
~50-100 facts per target. Written once, rarely changes.

**Source B: Superoptimizer (automatic, assembly-level)**
Brute-force search over all 2-instruction pairs × all register states:
```
For each (inst_a, inst_b) in ISA²:
  For each state in {0..2^24}:
    result_original = execute(inst_a; inst_b, state)
    For each inst_c in ISA:
      result_candidate = execute(inst_c, state)
      if result_original == result_candidate for ALL states:
        emit_rule(inst_a + inst_b → inst_c)
```
Already done: 602K proven rules in z80-optimizer (ADR-0009).
Future: GPU-accelerated length-3 search.

**Source C: Convergence failure analysis (automatic, IR-level)**
When MIR2-VM produces result X but LIR-VM(CISC) produces Y:
1. The divergence itself is the "rule discovery signal"
2. Binary search narrows to the specific instruction where divergence starts
3. Log the operand locations that caused the failure
4. Generate a ConstraintRule that forbids that combination

```
MIR2-VM: fib(10) = 55
LIR-VM(CISC): fib(10) = 21  ← DIVERGENCE!

Binary search → divergence starts at inst 7 (ADD HL, IX)
Z80 has no ADD HL,IX → generate:
  ConstraintRule{Name: "no_add_hl_ix", Op: OpAdd, DstSet: HL, SrcSet: IX, Cost: MaxCost}
```

This is **automated rule discovery from test failures**. Every convergence
failure becomes a new constraint rule.

### System 2: Rule Verification

**Assembly-level rules:** Already verified by superoptimizer (exhaustive state
enumeration, 2^8 to 2^24 states per rule). Zero false positives.

**IR-level rules:** Verified by convergence testing across 4+ architectures:
```
rule_correct(R) :-
  for all programs P in test_suite:
    for all machines M in {RISC32, RISC8, CISC, MICRO, Z80, 6502}:
      result(P, M, with_rule(R)) == result(P, M, without_rule(R))
```

**Constraint rules:** Verified by showing that forbidden combinations
cause assembly errors or wrong results:
```
constraint_necessary(C) :-
  exists program P, machine M:
    without_constraint(C): codegen(P, M) produces invalid instruction
    with_constraint(C): codegen(P, M) produces valid instruction
```

### System 3: WFC Solver (Rule Application)

WFC operates on **LocSet bitfields** representing superpositions:

```
Cell = (instruction_position, operand_slot)
State = LocSet (bitfield of possible physical locations)
Constraint = pattern from Datalog fact

Propagation:
  1. Initialize: every cell has full LocSet (all locations possible)
  2. Apply facts: AND each cell with its pattern's DstLocs/SrcLocs
  3. Propagate: if cell N.dst collapses to {A}, then cell N+1's
     src that reads N's result also collapses to {A}
  4. Backward: if cell N+2 requires src in {HL}, propagate backward
     to cell N+1's dst → must include {HL}
  5. Repeat until fixed point (no more changes)
  6. Collapse: choose minimum-entropy cell, pick lowest-cost option
  7. If contradiction (empty LocSet): insert spill/move and retry

Total iterations: typically 2-4 passes for straight-line code.
Each pass: O(n) ANDs on uint64 bitfields = nanoseconds.
```

**WFC dimensions:**
- **1D:** linear instruction sequence (basic block)
- **2D:** instruction × operand (register file view)
- **3D:** + flag state (which flags are live at each point)
- **Loop dimension:** loop-carried values wrap constraints around the back-edge

**Why WFC beats PBQP for retro targets:**
- PBQP: O(n²) edge matrix construction, O(n) R0/R1/RN reduction
- WFC: O(n) per propagation pass, 2-4 passes = O(n) total
- Z80 has 7 primary 8-bit regs: LocSet = uint8, AND = 1 cycle
- 6502 has 3 regs: LocSet = uint4, AND = 1 cycle

PBQP is better for large register files (ARM/x86). WFC is better for
small, constrained register files (Z80/6502/GB).

## DSL Syntax Proposals

### Datalog Rules (text file format)
```prolog
% z80.rules — loaded at compile time
target(z80).
reg(A, 8, acc). reg(B, 8, gen). reg(C, 8, gen).
reg(HL, 16, ptr). reg(DE, 16, idx). reg(BC, 16, pair).
alias(HL, H). alias(HL, L).
forbidden(IXH, ix_indirect).
forbidden(H, IXH).       % DD prefix NOP
pair_only(add16, BC, DE, HL, SP).
acc_only(add8).
clobber(add8, F). clobber(sbc16, F).
sets_flag(srl, ZF). sets_flag(srl, CF).
preserves_flag(inc, CF).  % INC doesn't touch carry!
```

### Cypher-like Patterns (text file format)
```cypher
% z80.patterns — loaded at compile time

-- Sub+CmpLt fusion (eliminates redundant CP)
RULE sub_cmp_fusion
MATCH (s:Sub)-[:NEXT]->(c:Cmp {cond: lt})
WHERE s.src0 == c.src0 AND s.src1 == c.src1
REWRITE c AS CmpSubCarry(s.dst)

-- INC peephole
RULE inc_peephole
MATCH (a:Add)
WHERE a.src1.is_const AND a.src1.val == 1 AND a.dst.loc == a.src0.loc
REWRITE a AS Inc(a.src0)

-- Redundant CP after flag-setting instruction
RULE redundant_cp
MATCH (i1:FlagSetter)-[:NEXT]->(cp:Cmp {imm: 0})-[:NEXT]->(br:Branch)
WHERE br.flag IN i1.flags_set
DELETE cp

-- Tail call optimization
RULE tail_call
MATCH (c:Call)-[:NEXT]->(r:Ret)
WHERE c.is_last_in_block AND r.vals.len == 0
REWRITE c AS Jump(c.target)
DELETE r
```

### WFC Configuration (embedded in MachineDesc)
```go
type WFCConfig struct {
    MaxPasses     int  // max propagation iterations (default: 10)
    CollapseOrder string // "min_entropy" | "sequential" | "critical_first"
    SpillStrategy string // "memory" | "push_pop" | "shadow_regs"
}
```

## Implementation Status

| Component | Status | Location |
|-----------|--------|----------|
| LocSet bitfield | ✅ Done | pkg/lir/lir.go |
| MachineDesc + patterns | ✅ Done | pkg/lir/machines.go |
| LIR-VM (convergence oracle) | ✅ Done | pkg/lir/vm.go |
| 4-way convergence tests | ✅ Done | pkg/lir/convergence_test.go |
| Greedy isel + setup moves | ✅ Done | pkg/lir/isel.go |
| Datalog fact DB | ✅ Done | pkg/lir/rules.go (Z80Facts) |
| 10 known IR patterns | ✅ Done | pkg/lir/rules.go (KnownPatterns) |
| 4 rewrite rules | ✅ Done | pkg/lir/rules.go (KnownRewrites) |
| WFC forward propagation | 📋 Next | — |
| WFC backward propagation | 📋 Next | — |
| Rule file parser | 📋 Planned | — |
| Convergence failure → rule | 📋 Planned | — |
| Superoptimizer integration | 📋 Planned | ADR-0009 |

## Decision

Proceed with this layered architecture. WFC forward+backward propagation
is the next implementation step. Rule files (Datalog + Cypher) will be
parsed from text initially, compiled to Go for release builds.

The key insight: **WFC is the solver, not the whole system**. Discovery
(superoptimizer + convergence analysis) and verification (exhaustive testing)
are equally important. All three must work together.
