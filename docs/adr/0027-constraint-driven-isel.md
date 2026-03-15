# ADR-0027: Constraint-Driven Instruction Selection

**Status:** Proposed (2026-03-15)

## Context

Current MIR2→Z80 codegen uses ~500 lines of if-chains: for each MIR2 opcode, hardcoded pattern matching selects a Z80 instruction. This works but has three systemic problems:

1. **Locality**: each decision is isolated — inst N doesn't know what inst N+2 needs
2. **Fragility**: adding a new pattern = finding the right place in if-chains without breaking others
3. **Self-host**: 500 lines of code in RAM is expensive for a compiler targeting 64K TPA

This ADR proposes replacing if-chains with a **table-driven constraint solver** using forward+backward propagation (conceptually close to WFC — Wave Function Collapse — applied to instruction selection).

## Problem: Greedy Local Selection Loses

### Example 1: missed register placement

```nanz
let x = a + b     // inst 1
call foo(x)        // inst 2 — PFCCO contract: foo expects arg in C
```

**Forward-only (current):**
```asm
ADD A, B          ; x → A (natural choice)
LD C, A           ; move to C — 4T wasted
CALL foo
```

**Bidirectional:** backward pass knows inst 2 needs x in C → plans inst 1 accordingly.

### Example 2: bit operations

```nanz
flags.carry = 1   // set bit 0 of flags
```

**Forward-only:**
```asm
LD A, (HL)        ; 7T
OR 0x01           ; 7T
LD (HL), A        ; 7T   — total: 21T, 3 instructions
```

**Constraint solver knows the pattern:**
```asm
SET 0, (HL)       ; 15T, 1 instruction — Z80 hardware bit op
```

SET/RES/BIT are hardware bitfield instructions the current codegen doesn't use at all.

### Example 3: self-host size

| | Code in RAM | Data in ROM |
|---|---|---|
| Current if-chains | ~3-5KB | — |
| Table + solver | ~200B interpreter + ~512B state | ~2-3KB pattern table |

For Z80 TPA (60KB), this is the difference between self-host being feasible or not.

## Decision

### Component 1: Pattern Table (data, not code)

Each row = one way to implement a MIR2 operation:

```go
type InstPattern struct {
    Op       MIR2Op
    Srcs     []LocSet        // allowed locations for each operand
    Dst      LocSet          // allowed locations for result
    Template string          // Z80 asm template
    Cost     int             // T-states
    Flags    PatternFlags    // SetsFlags, NeedsA, etc.
}

var addPatterns = []InstPattern{
    {OpAdd, [{RegA}, {Reg8}],    RegA,   "ADD A, %s",    4,  SetsAllFlags},
    {OpAdd, [{RegA}, {MemHL}],   RegA,   "ADD A, (HL)",  7,  SetsAllFlags},
    {OpAdd, [{RegA}, {Imm8}],    RegA,   "ADD A, %d",    7,  SetsAllFlags},
    {OpAdd, [{RegHL}, {RegPair}], RegHL, "ADD HL, %s",   11, SetsCarry},
}

// Z80 hardware bit manipulation:
var bitPatterns = []InstPattern{
    {OpBitSet, [{MemHL}, {Bit0_7}], MemHL, "SET %d, (HL)", 15, NoFlags},
    {OpBitSet, [{Reg8},  {Bit0_7}], Reg8,  "SET %d, %s",   8, NoFlags},
    {OpBitClr, [{MemHL}, {Bit0_7}], MemHL, "RES %d, (HL)", 15, NoFlags},
    {OpBitTst, [{MemHL}, {Bit0_7}], Flags, "BIT %d, (HL)", 12, SetsZero},
    {OpBitTst, [{Reg8},  {Bit0_7}], Flags, "BIT %d, %s",   8, SetsZero},
}
```

### Component 2: Constraint State

Per basic block — which physical locations are **possible** for each MIR2 value:

```go
type LocSet uint16  // bit field: bit N = location N available
                    // A=0, B=1, C=2, D=3, E=4, H=5, L=6,
                    // BC=7, DE=8, HL=9, (HL)=10, $addr=11

const (
    LocReg8 = LocA | LocB | LocC | LocD | LocE | LocH | LocL
    LocPair = LocBC | LocDE | LocHL
)
```

### Component 3: Forward Pass

For each MIR2 instruction in execution order — narrow possible sets by intersecting with compatible patterns.

### Component 4: Backward Pass

From end of block to start — propagate what's **needed**:
- Live-out constraints from PFCCO contracts
- Propagate through assignment chains
- When intersection is empty → insert move, widen to LocAny

### Component 5: Collapse

After forward + backward — select minimum-cost pattern for each instruction. Insert moves where operands aren't in required locations.

## Bidirectional = PFCCO Inside Basic Blocks

```
PFCCO:              optimizes ABI contracts BETWEEN functions
                    backward: from callee to caller

Constraint solver:  optimizes instruction selection WITHIN basic block
                    backward: from end of block to start

Together:           end-to-end optimization from HIR to Z80 with no gaps
```

PFCCO says: "function foo wants argument in C."
Constraint solver inside caller says: "value x needed in C → plan x's computation so result lands in C."

Without bidirectional solver, PFCCO gains are partially lost to intra-block moves.

## Bitfields as First-Class Nodes

### New MIR2 opcodes

```go
OpBitSet  // set bit N in value
OpBitClr  // clear bit N in value
OpBitTst  // test bit N, result in ZF
OpBitExt  // extract bit field (shift + mask)
OpBitIns  // insert bit field (mask + shift + or)
```

### HIR: @packed struct

```nanz
struct Flags @packed {
    carry: u1   // bit 0
    zero:  u1   // bit 1
    sign:  u1   // bit 2
}

flags.carry = 1    // → OpBitSet → SET 0, (HL)  — 15T
flags.zero = 0     // → OpBitClr → RES 1, (HL)  — 15T
if flags.carry     // → OpBitTst → BIT 0, (HL)  — 12T
```

### Pascal `set of` mapping

```pascal
Include(f, ACTIVE);   // → OpBitSet(f, ord(ACTIVE)) → SET 0, (HL)
Exclude(f, VISIBLE);  // → OpBitClr(f, ord(VISIBLE)) → RES 1, (HL)
if ACTIVE in f then   // → OpBitTst(f, ord(ACTIVE)) → BIT 0, (HL) / JR NZ
```

## Self-Host: Table = ROM, Interpreter = Tiny RAM

```
ROM (read-only):
  Pattern tables (instruction selection)
  Grammar tables (parser, if table-driven)

RAM:
  ~200 bytes interpreter
  ~512 bytes constraint state (per block, reused)
  ~700 bytes total for instruction selection

Historical precedent: Turbo Pascal 3 for CP/M (1983)
  Entire compiler: ~33KB, table-driven codegen, fit in TPA with room for programs.
```

## Datalog / Cypher Connections

Two related ideas for future exploration:

- **Datalog** — natural fit for compiler analysis (liveness, points-to, reaching defs). Rules like `live(X, B) :- use(X, B). live(X, B) :- live(X, B2), succ(B, B2), !def(X, B2).` express what we write manually in Go. Soufflé (Datalog engine) is used in production compilers (Doop for Java analysis).

- **Cypher-like graph queries** — query IR/CFG as a graph: `MATCH (b:Block)-[:SUCC]->(b2) WHERE b.has_call RETURN b2`. Useful for diagnostics and debugging, not hot-path compilation.

Both are analysis tools, not codegen. Useful when IR becomes complex enough that manual graph traversal becomes fragile.

## Phased Implementation

| Phase | Scope | Effort | Blocks |
|---|---|---|---|
| 1: Pattern table + forward-only | Refactor if-chains to table, identical output | ~1 week | nothing |
| 2: Backward propagation | PFCCO-aware constraint propagation in blocks | ~1 week | Phase 1 |
| 3: Bitfield HIR/MIR2 nodes | @packed struct, OpBitSet/Clr/Tst | ~3-5 days | Phase 1 |
| 4: Self-host ROM layout | Pattern table in ROM for Z80 self-host | ~1-2 days | self-host milestone |

Phase 1 is independent and can run in parallel with other work.

## References

- **BURG / iburg** (Fraser & Hanson, 1992) — tree pattern matching for instruction selection
- **Superoptimization** (Massalin, 1987) — exhaustive search for optimal instruction sequences
- **Equality Saturation** (egg/egglog) — apply all rules to fixpoint, extract minimum cost
- **Turbo Pascal 3** — historical precedent for table-driven Z80 codegen in 64K
- **Soufflé** — Datalog engine used in production compilers

## Related ADRs

- ADR-0020 (PBQP) — constraint solver complements PBQP: PBQP inter-function, solver intra-block
- ADR-0025 (struct promotion) — promoted struct returns create more tuple values for constraint propagation
- Self-host milestone — table-driven codegen critical for compiler size in 64K TPA
