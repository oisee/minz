# ADR-0037: Target-Neutral Optimization Split

**Date:** 2026-03-20
**Status:** Proposed
**Related:** ADR-0033 (LIR pipeline), ADR-0036 (eZ80 independent backend)

## Context

With the eZ80 backend landing as an independent codegen (ADR-0036), the compiler
now has two active backends: Z80 and eZ80 ADL. Both consume MIR2 and share the
same frontend pipeline (Nanz/C89/PL/M -> HIR -> MIR2). Currently the eZ80
backend wraps `mir2.Z80Codegen` — this is a Phase 1 shortcut that must be
replaced with native eZ80 codegen.

The question: which optimization passes are target-neutral (shared by both
backends) and which are target-specific (need separate implementations)?

Getting this wrong means either:
- Duplicating target-neutral code (maintenance burden, drift risk)
- Leaking target assumptions into shared code (Z80-isms in MIR2 passes)

### eZ80-specific opportunities

The eZ80 has addressing modes and instructions absent from Z80 that a naive
"wrap Z80 codegen" approach cannot exploit:

**Hardware multiply (MLT)**
- `MLT rr` — 8x8 unsigned multiply in 6T (vs ~80T software on Z80)
- Operates on BC, DE, HL, SP register pairs: `B*C -> BC`, `D*E -> DE`, etc.
- Eliminates the `__mul8` runtime entirely

**Load Effective Address (LEA)**
- `LEA rr, IX+d` / `LEA rr, IY+d` — computes address in one instruction
- All 6 target pairs: `LEA BC,IX+d`, `LEA DE,IX+d`, `LEA HL,IX+d`,
  `LEA BC,IY+d`, `LEA DE,IY+d`, `LEA HL,IY+d`
- Replaces: `LD rr,IX / ADD rr,d` (2 instructions + 24-bit add)
- Critical for struct field pointer computation

**Push Effective Address (PEA)**
- `PEA IX+d` / `PEA IY+d` — push computed address directly onto stack
- Replaces: `LEA HL,IX+d / PUSH HL` (2 instructions)
- Useful for passing struct field pointers as function arguments

**Non-destructive Test (TST)**
- `TST A, r` / `TST A, n` — sets flags from `A AND r` without modifying A
- Z80 requires `PUSH AF / AND r / POP AF` or a scratch register
- Eliminates save/restore around bit tests

**Stack-relative addressing (SP+d)**
- eZ80 `LD rr, (SP+d)` — not a single instruction, BUT:
- `LEA IX, SP+0` followed by `(IX+d)` addressing replaces expensive frame
  pointer setup. On Z80 this costs `PUSH IX / LD IX,0 / ADD IX,SP` (3 instr).
  On eZ80, `LEA IX, SP+0` is one instruction (3 bytes, ~4T)
- The real win: cheaper function prologue and epilogue

**24-bit native width**
- All register pairs (BC, DE, HL, IX, IY, SP) are 24-bit
- `LD HL, nn` loads 3-byte immediate (no split hi/lo)
- PUSH/POP move 3 bytes (not 2)
- Pointer arithmetic is native (no 16-bit overflow checks for >64K addresses)

**Extended block I/O**
- `INI2`, `IND2`, `OTIR2`, `INIR`, `OTIRX`, `INIRX` — 24-bit address block
  transfers. Irrelevant for codegen but important for stdlib/driver code.

### Impact on register allocation

The eZ80's wider register file and cheaper operations change PBQP cost
tradeoffs fundamentally:

| Resource | Z80 cost | eZ80 cost | Impact |
|----------|----------|-----------|--------|
| Reg-reg move | 4T | 1T (at 18MHz) | Lower spill penalty |
| Stack round-trip | 21T (PUSH+POP) | 7T effective | Stack spill much cheaper |
| Memory slot ($F0xx) | 26T | N/A (24-bit addr space) | Absolute addressing model changes |
| 8x8 multiply | ~80T (__mul8) | 6T (MLT) | Pattern match opportunity |
| Struct field ptr | ~12T (LD+ADD) | ~4T (LEA) | Address computation |
| Frame setup | ~30T (PUSH IX/LD/ADD) | ~4T (LEA IX,SP+0) | Prologue cost |
| IXH/IXL (undoc Z80) | 4T+prefix | official on eZ80 | More Tier-1 locations |
| Shadow regs (EXX) | 8T round-trip | 4T round-trip | Cheaper call-save |

## Decision

### 1. Three-layer architecture

```
Layer 1: Target-Neutral MIR2 Passes (shared)
  |
  v
Layer 2: Target-Parameterized Allocation (CostTable interface)
  |
  v
Layer 3: Target-Specific Codegen (separate packages)
```

### 2. Layer 1 — Target-neutral MIR2 passes (pkg/mir2/)

These passes operate on MIR2 SSA structure and know nothing about physical
registers, T-states, or instruction encoding. They remain in `pkg/mir2/`:

| Pass | Why neutral |
|------|-------------|
| `EliminateDeadBlocks` | CFG structure, no target info |
| `ReorderBlocks` | Fall-through heuristic, no target info |
| `PropagateConstants` | SSA value lattice |
| `FoldConstants` | Arithmetic identity |
| `SimplifyIdentities` | Algebraic simplification |
| `ConstantCallElim` | Pure function evaluation on MIR2 VM |
| `DeadStoreElim` | SSA liveness |
| `DeadBlockArgElim` | SSA phi cleanup |
| `BranchEquiv` | CFG pattern matching |
| `SplitJoinRet` | CFG pattern matching |
| `CondRetSink` | CFG pattern matching |
| `FuseAbsDiff` | Arithmetic pattern |
| `PropagateCopies` | SSA copy propagation |
| `InlineTrivial` | Call graph analysis |
| `LUTGen` | Value table synthesis |
| `ComputeLiveness` | SSA register liveness |
| `Verify` | Structural integrity |

**Grace rules** are target-neutral — they match CFG patterns and apply
structural rewrites. Grace stays in `pkg/rewrite/`.

**ISLE term rewriting** rules are split:
- Algebraic rules (constant folding, identity) -> target-neutral
- Instruction combining rules (load16_le fusion) -> target-specific

### 3. Layer 2 — Target-parameterized allocation (pkg/mir2/)

These components use the `CostTable` interface to remain generic:

**PBQP solver** (`PBQPAllocate`) — target-neutral algorithm, target-specific
costs. Takes `CostTable` interface, returns `AllocResult`.

```go
// Already exists — no change needed:
type CostTable interface {
    Cost(cls RegClass, loc PhysLoc) int
    Locs() []PhysLoc
}

// Z80 implementation (exists):
type Z80CostTable struct{}

// eZ80 implementation (new):
type EZ80CostTable struct{}
```

**Contract optimization** (`OptimizeContracts`) — target-neutral DP solver,
target-specific class-to-physical mapping. Takes `CostTable`.

**Register classes** — the existing `RegClass` enum is already designed to be
target-abstract:

| Class | Z80 meaning | eZ80 meaning |
|-------|-------------|--------------|
| `ClassAcc` | A (8-bit) | A (8-bit, same) |
| `ClassCounter` | B (DJNZ) | B (DJNZ, same) |
| `ClassGeneral` | A,B,C,D,E,H,L | Same + official IXH,IXL,IYH,IYL |
| `ClassPointer` | HL (16-bit) | HL (24-bit) |
| `ClassIndex` | DE (16-bit) | DE (24-bit) |
| `ClassPair` | BC,DE,HL | BC,DE,HL (24-bit) |
| `ClassDWord` | HL+H'L' (shadow) | Single register pair (native 24-bit) |
| `ClassFlag` | CY/Z flags | CY/Z flags (same) |

Key difference: `ClassDWord` on Z80 requires EXX shadow pair (expensive),
on eZ80 a single 24-bit register pair suffices. This changes cost dramatically.

**DefaultClass** mapping (`Ty -> RegClass`) needs a target parameter:
- Z80: `u24/u32 -> ClassDWord` (shadow pair)
- eZ80: `u24 -> ClassPointer` (native 24-bit register pair)

### 4. Layer 3 — Target-specific codegen

**Z80** (`pkg/mir2/z80codegen.go` + `pkg/mir2/z80cost.go`):
- `Z80CostTable` — existing, 4-tier cost model
- `Z80Codegen` — existing, 6197 LOC
- `Z80PhysLocs` — existing, 26 physical locations

**eZ80** (`pkg/ez80/`):
- `EZ80CostTable` — new, different tier costs (see table above)
- `EZ80Codegen` — new, native MIR2 -> eZ80 ADL codegen
- `EZ80PhysLocs` — new, extended physical location set

eZ80 physical locations (proposed):

```go
var EZ80PhysLocs = []PhysLoc{
    // Tier 0: primary 8-bit (same as Z80)
    {Kind: LocReg, Name: "A"},
    {Kind: LocReg, Name: "B"},
    {Kind: LocReg, Name: "C"},
    {Kind: LocReg, Name: "D"},
    {Kind: LocReg, Name: "E"},
    {Kind: LocReg, Name: "H"},
    {Kind: LocReg, Name: "L"},
    // Tier 0: primary 24-bit pairs
    {Kind: LocReg, Name: "HL"},
    {Kind: LocReg, Name: "DE"},
    {Kind: LocReg, Name: "BC"},
    // Tier 0.5: index half-regs (official on eZ80, undocumented on Z80)
    {Kind: LocReg, Name: "IXH"},
    {Kind: LocReg, Name: "IXL"},
    {Kind: LocReg, Name: "IYH"},
    {Kind: LocReg, Name: "IYL"},
    // Tier 1: index 24-bit (cheaper than Z80 — no prefix penalty at 18MHz)
    {Kind: LocIXY, Name: "IX"},
    {Kind: LocIXY, Name: "IY"},
    // Tier 2: shadow (same as Z80 but relatively cheaper)
    {Kind: LocShadow, Name: "B'"}, {Kind: LocShadow, Name: "C'"},
    {Kind: LocShadow, Name: "D'"}, {Kind: LocShadow, Name: "E'"},
    {Kind: LocShadow, Name: "H'"}, {Kind: LocShadow, Name: "L'"},
    {Kind: LocShadow, Name: "A'"},
    // Tier 3: stack slot (3 bytes per PUSH/POP, cheaper at 18MHz)
    {Kind: LocStack, Name: "stack"},
    // Special: CPU flag
    {Kind: LocFlag, Name: "F"},
}
```

Key differences from Z80PhysLocs:
- IXH/IXL/IYH/IYL promoted from Tier 1 (undocumented) to Tier 0.5 (official)
- No LocMem ($F0xx) — eZ80 has 16MB address space, absolute locals don't work
- No LocDWord — u24 is native width, no shadow-pair needed
- Stack slot = 3 bytes (not 2)

### 5. LIR MachineDesc for eZ80

The LIR backend (ADR-0033) uses data-driven `MachineDesc` with `Loc`, `Pattern`,
and `ConstraintRule` arrays. eZ80 gets its own MachineDesc:

```go
var EZ80 = &MachineDesc{
    Name:     "ez80",
    WordSize: 8, // ALU still 8-bit for most ops
    Locs:     ez80Locs,
    Patterns: ez80Patterns, // includes MLT, LEA, PEA, TST patterns
    Rules:    ez80Rules,    // DD/FD prefix, ADL suffix constraints
}
```

eZ80-specific patterns (not in Z80 MachineDesc):

| Pattern | MIR2 Op | eZ80 instruction | Cost |
|---------|---------|------------------|------|
| `mul8_mlt` | OpMul (u8) | `MLT BC` | 6T |
| `lea_ix` | OpField / addr_of | `LEA rr, IX+d` | 3T |
| `lea_iy` | OpField / addr_of | `LEA rr, IY+d` | 3T |
| `pea_ix` | push(addr_of) | `PEA IX+d` | 5T |
| `tst_a_r` | OpAnd (flag-only) | `TST A, r` | 3T |
| `lea_frame` | frame_setup | `LEA IX, SP+0` | 3T |

WFC constraint rules differ from Z80:
- No DD/FD prefix mutual exclusion for IXH+IXL (they're official)
- 24-bit immediate size for LD rr,nn patterns
- ADL suffix propagation for CALL/RET/JP

### 6. What changes where

| Component | Location | Change |
|-----------|----------|--------|
| `DefaultClass(Ty)` | `pkg/mir2/regs.go` | Add `target` parameter: u24->ClassPointer on eZ80 |
| `EZ80CostTable` | `pkg/ez80/cost.go` | New file: CostTable impl with eZ80 T-state costs |
| `EZ80PhysLocs` | `pkg/ez80/cost.go` | New: extended physical location set |
| `EZ80Codegen` | `pkg/ez80/mir2codegen.go` | Replace wrapper with native MIR2->eZ80 codegen |
| `EZ80 MachineDesc` | `pkg/lir/ez80.go` | New: LIR target descriptor (Phase 3b) |
| `pipeline.Options` | `pkg/pipeline/pipeline.go` | Already done: Backend field routes to eZ80 |
| ISLE rules | `pkg/rewrite/` | Add eZ80-specific combining rules (MLT, LEA) |
| Grace rules | `pkg/rewrite/` | No change (target-neutral CFG patterns) |
| PBQP solver | `pkg/mir2/pbqp.go` | No change (already uses CostTable interface) |
| All MIR2 passes | `pkg/mir2/` | No change (target-neutral) |

### 7. Migration path

Phase 1 (current): Wrapper approach
- `MIR2Codegen` wraps `Z80Codegen` with ADL header
- Uses `Z80CostTable` (wrong costs but functional)
- Pipeline wired, E2E works

Phase 2: Native cost table
- Implement `EZ80CostTable` with correct T-state costs
- `DefaultClass` gets target parameter
- PBQP generates better allocation for eZ80

Phase 3: Native codegen
- `EZ80Codegen` replaces wrapper
- Emits MLT, LEA, PEA, TST natively
- Cheaper prologue via `LEA IX, SP+0`

Phase 4: LIR integration
- `EZ80 MachineDesc` with eZ80-specific patterns
- WFC constraints for 24-bit registers
- ISLE combining rules for MLT, LEA fusion

## Consequences

### Positive

- **Clean separation:** Target-neutral passes never import target-specific code.
  No `if isEZ80` branches in MIR2 optimization passes.
- **CostTable interface works:** PBQP is already parameterized. Adding a new
  target is one struct implementing two methods.
- **Grace untouched:** CFG pattern matching is inherently target-neutral.
- **LIR naturally extensible:** MachineDesc is already data-driven. Adding
  eZ80 is a new descriptor file, not code changes.
- **ISLE splits cleanly:** Algebraic rules shared, instruction rules per-target.

### Negative

- **EZ80Codegen is large:** Estimating ~2000-3000 LOC for native codegen.
  The Z80 version is 6197 LOC but eZ80 is simpler in some ways (no $F0xx
  absolute addressing, no SMC, simpler frame setup).
- **DefaultClass needs refactoring:** Currently a pure function of Ty. Adding
  a target parameter touches every callsite. Mitigated: there are only ~5
  callsites in the codebase.
- **Two cost tables to maintain:** Z80 and eZ80 T-state values. Mitigated:
  z80timing package already has cost constants; add ez80timing.

### Neutral

- PBQP solver unchanged — just gets different cost inputs.
- Grace rules unchanged — pure CFG rewrites.
- All existing Z80 tests unchanged — new code, not modified code.

## References

- [eZ80 CPU User Manual (UM0077)](https://www.zilog.com/docs/um0077.pdf) — instruction timing
- [ADR-0033](0033-lir-pipeline-integration.md) — LIR pipeline with MachineDesc
- [ADR-0036](0036-ez80-independent-backend.md) — eZ80 as independent backend
- [pkg/mir2/regs.go](../../minzc/pkg/mir2/regs.go) — CostTable interface, RegClass
- [pkg/mir2/z80cost.go](../../minzc/pkg/mir2/z80cost.go) — Z80CostTable reference
- [pkg/lir/z80.go](../../minzc/pkg/lir/z80.go) — Z80 MachineDesc reference
