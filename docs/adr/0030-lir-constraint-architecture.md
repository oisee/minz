# ADR-0030: LIR + Constraint-Driven Backend Architecture

**Status:** Draft — design discussion
**Date:** 2026-03-16
**Supersedes/Unifies:** ADR-0009, ADR-0020, ADR-0027

## Problem

z80codegen.go is 6K+ lines of fragile if-chains. Every edge case (DD prefix,
width mismatch, IX arithmetic, parallel copies) requires a new guard. Fixing
one bug creates another. The 6502 backend has its own set of problems (A-only ALU).
Both share the same root cause: **the codegen has no formal model of target constraints**.

Three separate ADRs proposed partial solutions:
- ADR-0009: superoptimizer peephole (post-emit, assembly-level)
- ADR-0020: instruction-level EdgeCost (PBQP, pre-allocation)
- ADR-0027: table-driven constraint solver (WFC, replaces if-chains)

This ADR proposes a **unified architecture** that subsumes all three.

## Core Insight

The entire MIR2→assembly pipeline is a **constraint satisfaction problem**:

```
Given:  MIR2 program (operations + types + virtual registers)
Find:   Sequence of target instructions + physical register assignments
Such that:
  1. Semantics preserved (correctness)
  2. All encoding constraints satisfied (no impossible instructions)
  3. Total cost minimized (T-states, bytes, or weighted)
```

Currently this is solved by three separate phases (alloc → codegen → peephole),
each blind to the others. Unifying them exposes global optimization opportunities.

## Proposed Architecture

```
MIR2
  ↓
LIR (Low IR) — target-specific, 1:1 with machine instructions
  ↓
Constraint Solver — Datalog/Prolog-style rules
  ↓
Register Allocation — integrated with isel
  ↓
Emission — trivial template expansion
```

### Level 1: LIR — Low-Level IR

Between MIR2 (target-independent) and assembly (text), add **LIR** — a typed IR
where each operation maps 1:1 to a machine instruction, but operands are still
virtual (or constrained sets of physical locations).

```go
type LIRInst struct {
    Pattern   *InstPattern    // which target instruction
    Operands  []LIROperand    // virtual regs with location constraints
    Cost      int             // T-states
    Bytes     int             // encoded size
}

type LIROperand struct {
    VReg      Reg             // virtual register
    Allowed   LocSet          // bitset of allowed physical locations
    Preferred LocSet          // bitset of preferred locations (affinity)
}

type LocSet uint64            // bitfield: one bit per PhysLoc in the target
```

**LIR for Z80:**
```
MIR2: %r3 = add %r1, %r2 : u8 [acc]
   ↓ isel
LIR:  ld_a  %r1 {A}           ; load lhs to A (if not already)
      add_a %r2 {A,B,C,D,E,H,L,(HL)}  ; ADD A, src
      ; result in A → %r3 constrained to {A}
```

**LIR for 6502:**
```
MIR2: %r3 = add %r1, %r2 : u8
   ↓ isel
LIR:  lda   %r1 {A}           ; LDA src
      clc                      ; CLC (must precede ADC)
      adc   %r2 {A,X,Y,zp}    ; ADC src
      ; result in A → %r3 constrained to {A}
```

### Level 2: Instruction Pattern Tables

Each target defines its instructions as **pattern records**, not code:

```go
type InstPattern struct {
    Name      string         // "add_a_r"
    MIR2Op    Op             // OpAdd
    Width     int            // 8
    Template  string         // "ADD A, {src}"
    DstLocs   LocSet         // {A}
    SrcLocs   []LocSet       // [{A,B,C,D,E,H,L}]
    Cost      int            // 4 T-states
    Bytes     int            // 1
    Clobbers  LocSet         // {F}
    Flags     PatternFlags   // FlagCommutative, FlagClobberA, etc.
}
```

Z80 has ~100 patterns. 6502 has ~40. Each pattern is a **fact** in the constraint DB.

### Level 3: Constraint Rules (Datalog-style)

Target constraints expressed as **rules**, not if-chains:

```prolog
% Z80 DD prefix conflict
forbidden(Dst, Src) :- ixhalf(Dst), ixindexed(Src).
forbidden(Dst, Src) :- ixhalf(Src), dest_is(Dst, h).
forbidden(Dst, Src) :- ixhalf(Src), dest_is(Dst, l).

% Z80 ADD HL only
requires_pair(add_hl, rhs, {bc, de, hl, sp}).

% Z80 SBC HL only
requires_pair(sbc_hl, rhs, {bc, de, hl, sp}).

% 6502 ALU is A-only
requires_acc(Op) :- alu_op(Op).
```

These rules are **data** — loaded from files, not compiled into code.
Adding a new target = writing new rule file, not new Go code.

### Level 4: Graph Query DSL (Cypher-like)

For pattern matching across instruction sequences:

```cypher
-- Find sub(a,b) followed by branch on carry → abs_diff pattern
MATCH (s:Inst {op: OpSub})-[:FEEDS_FLAG]->(br:Term {kind: BrIf})
WHERE br.cond = CmpUlt
  AND s.src[0] = br.then_arg[0]  -- same operands
RETURN s, br AS abs_diff_candidate
```

Or for the superoptimizer integration:

```cypher
-- Find any 2-instruction window that matches a known optimization
MATCH (a:Inst)-[:NEXT]->(b:Inst)
WHERE NOT EXISTS (a)-[:LABEL]->()  -- no label between
  AND superopt_match(a.asm, b.asm) IS NOT NULL
RETURN a, b, superopt_match(a.asm, b.asm) AS replacement
```

### Level 5: Verification — Multi-Backend Convergence

```
               ┌─→ LIR-VM (interpret LIR directly)
MIR2 → isel → ├─→ Z80 emit → MZA → MZE (Z80 emulator)
               ├─→ 6502 emit → assembler → 6502 emu
               ├─→ QBE → native (correctness oracle)
               └─→ C99 → gcc (correctness oracle)

All must produce same result for same input.
```

**LIR-VM** is new: executes LIR instructions with simulated register file.
Catches isel bugs before emission. Much faster than full Z80 emulation.

## Implementation Plan

### Phase 0: Feature Branch + LIR Type Definitions (1-2 days)
- Create `feat/lir-backend` branch
- Define `LIRInst`, `LIROperand`, `LocSet`, `InstPattern` types
- Define LIR-VM interpreter skeleton

### Phase 1: Pattern Tables for Z80 (3-5 days)
- Extract Z80 instruction patterns from z80codegen.go into data tables
- ~100 patterns covering all MIR2 ops
- Verify: table-driven emit produces identical assembly to current codegen

### Phase 2: Constraint Rules + PBQP Integration (1-2 weeks)
- Implement EdgeCost from ADR-0020
- Wire into PBQP as edge matrices
- Constraint propagation (forward + backward) from ADR-0027
- Result: impossible allocations prevented, not just patched in codegen

### Phase 3: LIR-VM + Convergence Testing (3-5 days)
- LIR-VM that interprets pattern tables
- Test harness: MIR2-VM result == LIR-VM result == Z80 result == QBE result
- Run against existing 24 E2E tests + 241 C89 corpus

### Phase 4: 6502 Pattern Tables (2-3 days)
- Port 6502 patterns to same framework
- Same constraint rules, different table
- Convergence testing with 6502 E2E tests

### Phase 5: Superoptimizer Integration (ADR-0009) (2-3 days)
- Post-LIR peephole using 602K proven rules
- Hash-lookup on emitted instruction pairs
- Now operates on LIR (structured) not assembly (text)

### Phase 6: Graph DSL + Datalog (future research)
- Evaluate Datalog engines for Go (e.g. rego, souffle FFI)
- Pattern matching on MIR2/LIR graphs
- Rule files per target

## Key Design Decisions

1. **LIR is per-target** — no shared LIR across targets (unlike LLVM). Each target
   defines its own patterns. The shared part is the pattern/constraint framework.

2. **Alloc + isel unified** — register allocation and instruction selection happen
   together, not sequentially. LocSet constraints flow bidirectionally.

3. **Rules are data** — Prolog/Datalog rules loaded from files, not compiled Go code.
   Adding a target = writing rules, not code.

4. **Convergence testing is mandatory** — every change must pass all 4 backends.
   LIR-VM makes this fast (no external assembler/emulator needed).

5. **Feature branch** — all work on `feat/lir-backend`. Merge to master only when
   convergence tests pass and output matches current codegen.
