# ADR-0039: Unified VIR Solver — Joint ISel + RegAlloc in One Pass

**Date:** 2026-03-23
**Status:** Proposed
**Contributors:** Alice Vinogradova, Claude
**Related:** ADR-0035 (constraint codegen), ADR-0037 (target-neutral split), ADR-0038 (TSMC spill tiers)

---

## Problem

The current LIR pipeline has **5 sequential phases** that make independent decisions, creating exponential edge cases at phase boundaries:

```
MIROps (virtual regs)
  ↓ isel.go          — picks pattern WITHOUT knowing which registers are available
  ↓ wfc.go           — picks registers WITHOUT knowing if the pattern is compatible
  ↓ fixInvalidZ80Template — patches TEXT because template expanded to invalid Z80
  ↓ spill_reload.go  — patches TEXT because WFC assigned to non-register location
  ↓ z80peephole.go   — patches TEXT to clean up redundancies
  ↓ validate-reject  — retries BLINDLY if any of the above produced invalid output
Z80 ASM text
```

**Root cause of every codegen bug found in this session:**

| Bug | Phase boundary where information was lost |
|-----|------------------------------------------|
| `LD HL, 0` instead of symbol | Bridge → MIROp (Sym field dropped) |
| `ADD HL,HL` → 6-inst store | MIROp → Template (fixInvalidZ80Template too greedy) |
| INC instead of double | ISel → WFC (INC pattern matched x+x) |
| `LD A, DE` (invalid) | WFC → Template (16-bit value in 8-bit load) |
| `CP B` instead of `SBC HL,DE` | Bridge → MIROp (CMP width from result not operand) |
| `SUB A` instead of `SUB B` | WFC vreg inconsistency (same vreg, different phys) |

**Each fix adds another special case. The architecture creates bugs faster than we can fix them.**

---

## Terminology

Established in this session:

```
HIR  — High IR        (pkg/hir)   — structured, typed, frontend-independent
MIR  — Mid IR         (pkg/mir2)  — SSA, target-independent, block-based
VIR  — Virtual IR     (pkg/lir)   — Z80-flavored ops, virtual registers
PIR  — Physical IR    (pkg/lir)   — concrete Z80 ops, physical registers
ASM  — Assembly text  (string)    — final output
```

**VIR** is the critical level. It knows Z80 semantics (8/16-bit ops, accumulator architecture) but has not committed to physical registers or specific instruction patterns. This is where the unified solver operates.

**PIR** is emit-ready: each instruction has a concrete Z80 pattern AND physical register assignment. No fixups needed — PIR → ASM is a trivial template expansion.

---

## Decision

### 1. Replace 5-phase pipeline with unified VIR solver

**Before (current):**
```
VIROp sequence
  → isel (choose pattern)        ← decision 1
  → WFC (choose registers)       ← decision 2
  → fixInvalidZ80Template         ← bandage 1
  → spill_reload                  ← bandage 2
  → validate-reject               ← bandage 3
PIROp / ASM text
```

**After (unified):**
```
VIROp sequence
  → Solver (choose pattern AND registers simultaneously)
PIROp sequence
  → emit (trivial template expansion)
ASM text
```

The solver sees the **complete decision space** for each VIROp:
- All applicable patterns (e.g., `add_a_r`, `add_hl_rr`, `inc_r`)
- All valid register assignments per pattern
- All spill tier options (L1-L7 including TSMC)
- All inter-instruction constraints (interference, tied operands, clobbers)

One decision. No phase boundaries. No information loss.

### 2. Unified LocSet with spill tiers

```
LocSet uint64 — 64 bits covering ALL storage locations:

Bits  0-6:   L1 GPR        {A, B, C, D, E, H, L}         cost: 0T
Bits  7-10:  L1 pairs      {BC, DE, HL, SP}               cost: 0T
Bits 11-12:  L1 index      {IX, IY}                        cost: 4T (prefix)
Bit  13:     L1 flags      {F}                              cost: 0T
Bits 14-17:  L2 IX/IY half {IXH, IXL, IYH, IYL}           cost: 8T
Bits 18-23:  L3 shadow     {B', C', D', E', H', L'}       cost: 8T (EXX)
Bit  24:     L3 shadow A   {A'}                             cost: 4T (EX AF,AF')
Bits 25-32:  L4 TSMC       {tsmc0..tsmc7}                  cost: 20T (A) / 32T (non-A)
Bits 33-36:  L7 memory     {mem0..mem3}                     cost: 26T
Bits 37-47:  L6 stack      {SP+0, SP+2, ..., SP+20}       cost: 21T
Bits 48-63:  reserved      (future: frame slots, bank-switched RAM)
```

**Every location is in one LocSet.** The solver picks the cheapest valid one.

When the solver assigns a vreg to a non-GPR location (e.g., IXH for spill across CALL), it simultaneously decides which move instructions to insert:

```
VIROp: v3 = add8 v1, v2    ; v1 live across CALL, assigned to IXH

Solver emits PIROps:
  LD IXH, B          ; save v1 to IXH before CALL
  CALL foo
  LD B, IXH          ; restore v1 from IXH after CALL
  ADD A, B            ; use v1 in ALU (now in B)
```

The solver chose IXH (cost 8T save + 8T restore = 16T) over stack (21T) because it was cheaper. It also chose to route through B for the ALU op. **One decision, no fixups.**

### 3. Solver implementation: WFC with Z3 fallback

**Fast path: WFC (< 1ms per function)**

WFC on VIROps with expanded LocSet. For each VIROp:
1. Enumerate all matching patterns → superposition of {Pattern × LocSet} cells
2. Forward propagation: narrow LocSets based on instruction constraints
3. Backward propagation: narrow based on downstream requirements
4. Collapse: pick minimum-cost {Pattern, Registers} for each cell

If WFC finds a valid solution → emit PIROps directly.

**Slow path: Z3 SMT (< 1s per function)**

If WFC contradicts (no valid assignment), encode the entire problem as SMT:

```smt2
; For each VIROp: which pattern?
(declare-const inst5_pat Int)  ; 0=add_a_r, 1=add_hl_rr, ...

; For each vreg: which physical location? (0..63 in LocSet)
(declare-const v1_loc Int)

; Pattern constrains locations:
(assert (=> (= inst5_pat 0)           ; add_a_r
             (and (= v1_loc LOC_A)    ; dst must be A
                  (or (= v2_loc LOC_B) (= v2_loc LOC_C) ...))))

; Interference:
(assert (=> (and (alive v1 inst5) (alive v2 inst5))
             (not (= v1_loc v2_loc))))

; Spill tier costs in objective:
(minimize (+ (cost v1_loc inst1) (cost v2_loc inst2) ...))
```

Z3 finds **provably optimal** joint isel + regalloc. For Z80 (37 locations, ~50 instructions per block): solves in < 1 second.

**Tier 3: Multi-seed parallel WFC**

For functions where WFC finds a solution but suboptimal: launch 5040 parallel seeds (7! initial orderings), pick best. On 8-core PC: < 10ms.

### 4. Move insertion as solver output

The solver doesn't just assign registers — it **emits move instructions** as part of its solution:

```
VIROps (input):           Solver output (PIROps):
  v1 = const 5              LD B, 5           ; v1 → B
  v2 = const 3              LD C, 3           ; v2 → C
  CALL foo                   LD IXH, B         ; spill v1 (live across CALL)
                             CALL foo
                             LD B, IXH         ; reload v1
  v3 = add v1, v2            LD A, B           ; move v1 to A (ALU requires)
                             ADD A, C          ; v3 = v1 + v2
```

Moves are **first-class solver output**, not post-hoc fixups. The solver knows the cost of each move and factors it into the optimal solution.

### 5. TSMC spill as solver option

Per ADR-0038, TSMC slots (L4/L5) are in the LocSet. The solver can choose:

```
Spill v1 across CALL:
  Option A: PUSH BC / CALL / POP BC               (21T)
  Option B: LD IXH, B / CALL / LD B, IXH          (16T)
  Option C: LD (_tsmc0+1), A / CALL / LD A, 0     (20T, TSMC tunnel)
  Option D: EXX / CALL / EXX                       (8T, if all live fit in shadow)

Solver picks cheapest valid option automatically.
```

TSMC tunnel safety (no recursion between spill/reload) is encoded as a constraint — if tunnel is unsafe, TSMC locations are removed from that vreg's LocSet.

### 6. Elimination of text fixups

With the unified solver, these files become unnecessary:

| File | What it does | Why no longer needed |
|------|-------------|---------------------|
| `fixInvalidZ80Template` | Patches `ADD HL, HL` → store | Solver never picks incompatible pattern+register |
| `spill_reload.go` | Inserts `LD DE,(mem)` for spilled operands | Solver emits move instructions |
| validate-reject loop | Retries with different registers | Solver guarantees valid output |
| `selectTemplateForPhys` | Picks different pattern post-WFC | Solver picks pattern and registers together |

`z80peephole.go` remains — peephole on PIROps is still valuable for cross-instruction optimizations (LD A,A removal, ADD A,1→INC A, etc.)

---

## Architecture

### Package layout

```
pkg/hir/     — HIR (unchanged)
pkg/mir2/    — MIR (rename to pkg/mir/ eventually)
pkg/lir/     — contains both VIR and PIR types + solver
  virop.go   — VIROp type (was MIROp)
  pirop.go   — PIROp type (was Inst)
  solver.go  — unified WFC/Z3 solver (VIROp → []PIROp)
  z80desc.go — Z80 machine descriptor (patterns, costs, constraints)
  emit.go    — PIROp → ASM text (trivial template expansion)
  bridge.go  — MIR → VIROp translation (unchanged)
  egraph.go  — e-graph for multi-variant lowering
  peephole.go — PIROp-level peephole (post-solver)
```

### Solver interface

```go
// Solver converts a basic block of VIROps into PIROps.
// It simultaneously selects patterns AND assigns registers.
type Solver interface {
    Solve(ops []VIROp, desc *MachineDesc, hints AllocHints) ([]PIROp, error)
}

// WFCSolver — fast greedy solver with constraint propagation.
type WFCSolver struct{}

// Z3Solver — optimal solver via SMT. Fallback when WFC contradicts.
type Z3Solver struct{}

// HybridSolver — WFC first, Z3 fallback, multi-seed parallel.
type HybridSolver struct {
    WFC   *WFCSolver
    Z3    *Z3Solver
    Seeds int  // 0 = no parallel seeds, >0 = try N orderings
}
```

### VIROp → PIROp example

```go
// Input: VIR level
virOps := []VIROp{
    {Op: OpConst, Dst: 1, Imm: 5, Width: 8},
    {Op: OpConst, Dst: 2, Imm: 3, Width: 8},
    {Op: OpAdd, Dst: 3, Src: [2]int{1, 2}, Width: 8},
}

// Solver output: PIR level (pattern + physical regs)
pirOps := solver.Solve(virOps, Z80, hints)
// → [
//     PIROp{Pat: "ld_r_n", Phys: {Dst: A}, Imm: 5},    // LD A, 5
//     PIROp{Pat: "ld_r_n", Phys: {Dst: B}, Imm: 3},    // LD B, 3
//     PIROp{Pat: "add_a_r", Phys: {Dst: A, Src: [A, B]}}, // ADD A, B
// ]

// Emit: trivial template expansion
for _, p := range pirOps {
    fmt.Println(p.Emit(Z80))  // "LD A, 5" / "LD B, 3" / "ADD A, B"
}
```

---

## Migration Path

**Phase 0 (now): Stabilize current pipeline**
- Fix cond_ret regression
- Keep 82/87 corpus passing
- No architectural changes

**Phase 1 (1 week): Encode text fixups as constraints**
- Every rule in `fixInvalidZ80Template` → pattern constraint in z80.go
- Every rule in `spill_reload.go` → LocSet constraint
- Delete text fixup code, verify corpus
- Result: isel + WFC still separate, but no text fixups

**Phase 2 (1 week): Merge isel into WFC**
- WFC receives VIROps directly (not Insts)
- Each cell = superposition of {Pattern × LocSet}
- Propagation considers pattern compatibility
- Result: unified solver, but WFC-only (no Z3)

**Phase 3 (1 week): Z3 fallback + TSMC tiers**
- Z3 encodes joint isel+regalloc for hard cases
- TSMC tunnel analysis enables L4/L5 spill tiers
- Multi-seed parallel WFC for near-optimal fast path
- Result: full unified solver with all spill tiers

**Phase 4 (ongoing): Superoptimization**
- E-graph saturation generates ALL equivalent VIROp sequences
- Solver picks globally optimal across all variants
- 602K superoptimizer rules feed into e-graph
- Result: provably optimal Z80 code per basic block

---

## Verification

**Invariant:** `MIR2-VM(function) == Z80-binary(function)` for all inputs.

At each phase:
1. Run MIR2 VM on original function → expected result
2. Run Z80 emulator on solver output → actual result
3. Compare. Divergence = solver bug.

Additionally:
- **RISC32 VM** as sanity check (no Z80 constraints → should always work)
- **QBE backend** as independent correctness oracle
- **Per-commit corpus test** (82/87 must not regress)

---

## Consequences

### Positive
- **Eliminates all phase-boundary bugs** — one decision, no fixups
- **Provably optimal** with Z3 — best possible Z80 code per block
- **All spill tiers available** — GPR, IXH, shadow, TSMC, stack, memory
- **Testable in isolation** — `VIROp → PIROp` unit tests without full pipeline
- **Target-retargetable** — swap MachineDesc for 6502/eZ80/GB

### Negative
- **Migration risk** — 3 weeks of work touching core codegen
- **Z3 dependency** — subprocess, not always available
- **Complexity** — unified solver is harder to debug than 5 simple phases
- **WFC correctness** — must prove WFC produces valid Z80 (currently doesn't always)

### Neutral
- Z80 peephole remains (cross-instruction opts)
- Bridge MIR→VIR unchanged
- PFCCO contracts unchanged
- HIR frontends unchanged

---

## References

- [ADR-0035](0035-algebraic-types-and-exhaustive-matching.md) — algebraic types (unrelated, same number range)
- [ADR-0036](0036-unified-constraint-codegen.md) — unified constraint codegen
- [ADR-0037](0037-target-neutral-optimization-split.md) — target-neutral split, cost tiers
- [ADR-0038](0038-tsmc-spill-tiers.md) — TSMC spill tier design
- [egg](https://egraphs-good.github.io/) — equality saturation
- [Cranelift ISLE](https://github.com/bytecodealliance/wasmtime/tree/main/cranelift/isle) — instruction selection DSL
- [Unison](http://unison-code.github.io/) — joint isel+regalloc via constraint programming
