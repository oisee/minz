# ADR-0020: Instruction-Level Constraints for Register Allocation

## Status
Proposed

## Context

### The Problem

BUG-008 exposed a class of codegen bugs where the PBQP register allocator assigns
operand combinations that are **impossible to encode** on the target CPU.  The Z80's
arena allocator generates `LD IXL, (IX+0)` — an instruction that cannot exist because
the DD prefix can't simultaneously remap the destination register (L→IXL) and the
source addressing mode ((HL)→(IX+d)).

The current architecture has a gap:

```
CostTable (move costs)  →  PBQP (allocation)  →  Codegen (emit)  →  Assembler (validate)
     knows reg↔reg            decides                blind              catches, but
     move costs               assignments             to encoding        too late
                                                      constraints
```

The **cost model** (`z80cost.go`) knows that moving between H and IXH costs infinity
(DD prefix conflict).  But it only models **register-to-register moves**, not
**instruction operand constraints**.  The codegen (`z80codegen.go:1662`) blindly
combines `lowByte(dst)` with `ptrIndirect(ptr, d)` without checking whether the
resulting instruction can be encoded.

### This Is Not Z80-Specific

Every CPU has impossible operand combinations that aren't captured by move costs:

| CPU | Impossible Combination | Why |
|-----|----------------------|-----|
| Z80 | `LD IXL, (IX+d)` | DD prefix conflict — can't remap both dest and source |
| Z80 | `LD H, IXH` | DD prefix remaps H→IXH — becomes `LD IXH, IXH` (NOP) |
| Z80 | Sequential `LD IXL,(IX+0); LD IXH,(IX+1)` | First load clobbers IX base for second |
| 6502 | `ADC X` | ALU operations can only target A accumulator |
| 6502 | `STA (ind),X` | Indirect-indexed only works with Y, not X |
| eZ80 | 16-bit op on 24-bit register in ADL mode | Mode mismatch |
| ARM | `ADD PC, R1, R2` | PC as ALU destination is restricted/unpredictable |
| GB (SM83) | `LD (HL+), (HL)` | Auto-increment and source can't both be HL |

These are **not move costs** — they're constraints on which physical locations can
appear together as operands of a specific MIR2 opcode.

### What We Need

A way for each backend to declare: "for opcode X, if operand A is in location P
and operand B is in location Q, this combination is forbidden/expensive."  The
allocator must respect these constraints **before** code generation, not after.

## Decision

### 1. Extend CostTable with `EdgeCost` — Pairwise Operand Constraints

Add a new method to the `CostTable` interface:

```go
type CostTable interface {
    Locs() []PhysLoc
    Cost(cls RegClass, loc PhysLoc) int               // existing: move/storage cost

    // NEW: instruction-level constraint.
    // Returns additional cost (or InfCost) when an instruction with the given
    // opcode has dst at dstLoc and a source operand at srcLoc.
    // Zero means "no constraint".
    EdgeCost(op Op, dstLoc, srcLoc PhysLoc) int
}
```

Each backend fills in its forbidden combinations.  The default implementation
returns 0 for everything (no constraints — backwards compatible).

### 2. Z80 EdgeCost Implementation

```go
func (t *Z80CostTable) EdgeCost(op Op, dst, src PhysLoc) int {
    switch {

    // Rule 1: DD prefix conflict — IXY8 dest with IX-indexed source.
    // LD IXL, (IX+d) is uncodable: DD prefix can't remap both.
    case isMemoryOp(op) && isIXY8(dst) && isIXY(src):
        return InfCost

    // Rule 2: H/L dest with IX-indexed source.
    // LD H, (IX+d) is valid (DD 66 dd), but H in the result is "real H",
    // not IXH.  If the allocator later needs IXH, this creates confusion.
    // Allow but add penalty to prefer A/B/C/D/E destinations.
    case isMemoryOp(op) && isHorL(dst) && isIXY(src):
        return 4 // soft penalty, not forbidden

    // Rule 3: Self-clobber — loading 16-bit value into IX from (IX+d).
    // First byte load changes IXL, corrupting (IX+d) for second byte.
    case op == OpLoad && dst.Kind == LocIXY && src.Kind == LocIXY:
        if pairOf(dst) == pairOf(src) {
            return InfCost
        }

    // Rule 4: H↔IXH in same instruction — DD prefix makes them identical.
    // LD H, IXH → DD 64 → LD IXH, IXH (NOP).  Already caught by move cost
    // InfCost, but belt-and-suspenders for instruction contexts.
    case isRegOp(op) && isHorL(dst) && isIXY8(src):
        if sameHalf(dst, src) { return InfCost }
    case isRegOp(op) && isIXY8(dst) && isHorL(src):
        if sameHalf(dst, src) { return InfCost }
    }

    return 0 // no constraint
}
```

### 3. 6502 EdgeCost Implementation (for reference)

```go
func (t *M6502CostTable) EdgeCost(op Op, dst, src PhysLoc) int {
    switch {

    // ALU ops (add, sub, and, or, xor, cmp) — only A as destination.
    case isALUOp(op) && dst.Name != "A":
        return InfCost

    // Indexed addressing: zero-page,X and abs,X — index must be X.
    case isIndexedLoad(op) && src.Name == "Y":
        return InfCost  // (addr,X) only, not (addr,Y)

    // Indirect-indexed: (zp),Y — index must be Y, not X.
    case isIndirectIndexed(op) && src.Name == "X":
        return InfCost
    }

    return 0
}
```

### 4. Wire EdgeCost into PBQP Solver

EdgeCost creates **PBQP edge matrices** between operand pairs of the same instruction:

```go
// In PBQP setup, for each instruction with dst and src:
func addInsnEdges(f *Func, states map[Reg]*regState, ct CostTable, locs []PhysLoc) {
    for _, b := range f.Blocks {
        for _, inst := range b.Insts {
            if inst.Dst == NoReg { continue }
            for _, src := range inst.Src {
                if src == NoReg { continue }
                // Build edge matrix: cost[i][j] for all (dstLoc[i], srcLoc[j]) pairs
                matrix := make([][]int, len(locs))
                hasConstraint := false
                for i, dLoc := range locs {
                    matrix[i] = make([]int, len(locs))
                    for j, sLoc := range locs {
                        c := ct.EdgeCost(inst.Op, dLoc, sLoc)
                        matrix[i][j] = c
                        if c != 0 { hasConstraint = true }
                    }
                }
                if hasConstraint {
                    addPBQPEdge(states, inst.Dst, src, matrix)
                }
            }
        }
    }
}
```

PBQP already supports edge cost matrices — the solver minimizes total cost
including both node costs (current) and edge costs (new).  No solver changes
needed.

### 5. Three-Layer Defense

```
Layer 1 — PBQP EdgeCost     allocator AVOIDS bad combinations     (prevention)
Layer 2 — Codegen guards     emitter FIXES remaining edge cases    (mitigation)
Layer 3 — Assembler reject   MZA CATCHES anything that slipped     (detection)
```

**Layer 1** prevents 95%+ of cases by making bad combinations expensive/infinite.
**Layer 2** handles forced spill scenarios where PBQP had no better option — the
codegen routes through scratch registers instead of emitting impossible instructions.
**Layer 3** is already implemented (MZA rejects invalid instructions) and serves
as final safety net.

### 6. Codegen Guard (Layer 2) — Safety Net

Even with PBQP constraints, edge cases may remain (e.g., all registers occupied,
allocator forced into bad combination).  The codegen must never emit uncodable
instructions:

```go
// z80codegen.go — guarded IX-indexed load
} else if isIXY(ptr) {
    lo, hi := lowByte(dst), highByte(dst)
    if isIXYHalf(lo) || isHorL(lo) {
        // Route through scratch — can't LD IXL,(IX+d) or LD H,(IX+d)->IXH confusion
        g.emitf("    LD A, %s", ptrIndirect(ptr, 0))   // A is always safe with (IX+d)
        g.emitf("    LD B, %s", ptrIndirect(ptr, 1))   // B is always safe with (IX+d)
        g.emitf("    LD %s, A", lo)                     // LD IXL, A — valid undocumented
        g.emitf("    LD %s, B", hi)                     // LD IXH, B — valid undocumented
    } else {
        g.emitf("    LD %s, %s     ; lo", lo, ptrIndirect(ptr, 0))
        g.emitf("    LD %s, %s     ; hi", hi, ptrIndirect(ptr, 1))
    }
}
```

## Consequences

### Positive

- **Generalized**: Same `EdgeCost` interface works for Z80, 6502, eZ80, ARM, GB.
  Each backend declares its own constraints.  Allocator and solver are architecture-
  independent.
- **Prevents bugs at source**: Bad combinations eliminated during allocation, not
  patched after emission.  No more impossible instructions reaching the assembler.
- **Composable with existing PBQP**: Edge cost matrices are native to PBQP — no
  solver changes, just additional edges.
- **Self-documenting**: Constraint rules are explicit, readable, testable.  New
  backend authors can see exactly what combinations are forbidden.
- **Enables new backends**: 6502, eZ80, GB (SM83) backends can declare their
  constraints before writing a single line of codegen.  Allocator does the work.
- **Testable**: `EdgeCost(OpLoad, IXL, IX) == InfCost` is a unit test.

### Negative

- **Edge matrix overhead**: For N physical locations, each instruction edge adds
  an N×N matrix.  With ~30 Z80 locations, that's ~900 ints per edge.  Mitigated by
  only creating matrices when `hasConstraint == true` (most instructions have no
  constraints).
- **PBQP solve time**: More edges means more work for the solver.  But instruction
  edges are sparse (only memory ops and ALU ops have constraints), and PBQP is
  already fast for Z80-scale problems (~30 nodes).
- **Two places to maintain**: Constraints declared in EdgeCost AND guarded in
  codegen.  But this is intentional defense-in-depth, and the codegen guards are
  small (~10 LOC each).

### Neutral

- Move costs in `Cost()` remain unchanged — EdgeCost is additive, not replacing.
- Existing allocation results for programs that don't hit IX-indexed loads will
  not change (EdgeCost returns 0 for unconstrained combinations).
- MZA validation (Layer 3) remains as final safety net regardless.

## Alternatives Considered

### A. Hardcoded codegen guards only (no PBQP changes)

Add `if isIXYHalf(lo)` checks in codegen and route through scratch registers.

**Rejected because:** This is a bandaid.  The allocator still assigns IXL as
destination for IX-indexed loads — the codegen just works around it with extra
instructions.  Every new backend would need its own set of ad-hoc guards.
The constraint is better expressed once in the cost model.

Retained as **Layer 2** safety net, but not as primary solution.

### B. Forbidden register sets per opcode (pre-allocation)

Before PBQP, scan all instructions and remove forbidden locations from each
virtual register's candidate set.

**Rejected because:** Operand constraints are **pairwise** — `IXL` as destination
is fine for `LD IXL, A`, but forbidden for `LD IXL, (IX+0)`.  Removing IXL from
the candidate set globally is too aggressive and loses valid allocations.  PBQP
edge costs handle pairwise constraints naturally.

### C. Post-allocation rewrite pass

After PBQP assigns locations, scan for impossible combinations and rewrite them
(insert scratch register moves).

**Rejected because:** Post-allocation rewrites are fragile — inserting new
instructions changes liveness, which may invalidate other allocations.  Better
to prevent the problem than to patch it afterward.

Partially retained as **Layer 2** (codegen guards), which is simpler than a full
rewrite pass since it operates instruction-by-instruction during emission.

### D. Extend move cost matrix to cover all instruction types

Instead of a separate `EdgeCost`, add more dimensions to the existing `Cost()`
function.

**Rejected because:** `Cost(cls, loc)` is a clean 1D lookup per register.
Instruction constraints are inherently 2D (pairwise).  Forcing them into a 1D
model would require enumerating all (opcode, loc) combinations — combinatorial
explosion.  A separate `EdgeCost(op, dst, src)` is cleaner.

## References

- [BUG-008](../Open_Bugs_RCA.md) — Arena codegen impossible `LD IXL, (IX+d)`
- [Report #071](../../reports/2026-03-14-071-Arena_Z80_Codegen_IXL_Bug.md) — Full RCA
- [ADR-0010](0010-register-first-calling-convention.md) — PFCCO calling convention
- [ADR-0017](0017-lut-pointer-selection-and-pbqp-edge-costs.md) — PBQP edge costs (LUT affinity)
- PBQP paper: Scholz & Eckstein, "Register Allocation for Irregular Architectures"
- Z80 undocumented instructions: Sean Young, "The Undocumented Z80 Documented"

## Appendix: Complete Z80 DD-Prefix Conflict Table

For reference, the full set of Z80 instruction combinations affected by DD/FD
prefix conflicts:

```
VALID (DD remaps ONE thing):
  LD A, (IX+d)      DD 7E dd     — DD remaps source memory operand
  LD B, (IX+d)      DD 46 dd     — DD remaps source memory operand
  LD IXH, A         DD 67        — DD remaps dest register H→IXH
  LD IXH, 42        DD 26 2A     — DD remaps dest register H→IXH
  LD IXH, IXL       DD 65        — DD remaps both H→IXH and L→IXL (same prefix)

INVALID (DD would need to remap TWO DIFFERENT things):
  LD IXL, (IX+d)    —            — can't remap both dest L→IXL AND source (HL)→(IX+d)
  LD IXH, (IX+d)    —            — same conflict
  LD (IX+d), IXL    —            — can't remap both dest (HL)→(IX+d) AND source L→IXL
  LD (IX+d), IXH    —            — same conflict

LOOKS VALID BUT ISN'T (DD makes H/L become IXH/IXL):
  LD H, IXH         DD 64        — DD remaps H→IXH: becomes LD IXH, IXH (NOP)
  LD L, IXL         DD 6D        — DD remaps L→IXL: becomes LD IXL, IXL (NOP)
  LD IXH, H         DD 64        — same encoding, same NOP
  LD IXL, L         DD 6D        — same encoding, same NOP

SELF-CLOBBER (valid individually, broken in sequence):
  LD IXL, r         DD 68+r      — changes IX.lo
  LD IXH, (IX+1)    —            — IX base already corrupted by previous IXL write
```
