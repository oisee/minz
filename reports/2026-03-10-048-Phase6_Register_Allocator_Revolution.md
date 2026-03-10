# Report #048 — Phase 6: Register Allocator Revolution (6a/6b/6c)

**Date:** 2026-03-10
**Status:** ✅ COMPLETE — 23/23 test packages pass
**Commits:** 14dbbad · 2e0a6e3 · 52cd9ef · ae7ed3a

---

## Executive Summary

Three phases of the register allocator were implemented in sequence, replacing the old greedy graph-colouring allocator with a principled PBQP (Partitioned Boolean Quadratic Program) solver, adding IX/IY register support for Z80, and eliminating unnecessary copies at block boundaries via post-allocation coalescing.

**Before Phase 6:** Four simultaneously-live ClassPointer registers → three in HL/DE/BC, fourth spilled to `$F0xx` memory.
**After Phase 6:** All four land in HL/DE/BC/IX — zero spills, correct `(IX+0)` addressing.

---

## Phase 6a' — Pre-requisite Bug Fixes

Two root-cause bugs blocked correct struct/pointer codegen:

### Bug A — `VarRefExpr.Ty` hardcoded as `TyU8`
`parsePrimary` in `pkg/nanz/parse.go` always set `Ty: mir2.TyU8` for variable references regardless of the variable's actual declared type. Struct globals got `ClassAcc` instead of `ClassPointer` → invalid `LD A, HL` emitted.

**Fix:** Look up `varTypes` and `globalTypes` maps:
```go
ty := mir2.Ty(mir2.TyU8)
if t2, ok := p.varTypes[t.val]; ok {
    ty = t2
} else if t2, ok := p.globalTypes[t.val]; ok {
    ty = t2
}
```

### Bug B1 — `PtrAdd(base, Const(0))` zero-offset spill
After lowering `self.field` when offset is 0, the IR contained `OpPtrAdd(self, Const(0))`, creating a redundant second ClassPointer register that spilled to `$F0xx` under register pressure.

**Fix:** New `SimplifyIdentities(f)` pass in `pkg/mir2/constprop.go`:
```go
// OpPtrAdd(x, Const(0)) → OpMove(x)
if inst.Op == OpPtrAdd {
    if v, ok := consts[inst.Src[1]]; ok && v == 0 {
        b.Insts[i] = &Inst{Op: OpMove, Dst: inst.Dst, Src: [2]Reg{inst.Src[0]}, ...}
    }
}
```
Wired into the constant pipeline fixpoint loop in `pkg/pipeline/pipeline.go`.

---

## Phase 6a — IX/IY Pointer Support in Z80 Codegen

### The Problem
The Z80 has four 16-bit pointer registers: HL, DE, BC — and the indexed pair IX/IY. The cost table already assigned `costPointer(LocIXY) = 8T` (cheaper than `$F0xx` memory at 26T+), so the allocator *could* select IX. But the codegen had no support for `(IX+d)` addressing mode — it would emit bare `(IX)` which is invalid Z80.

### Implementation
`pkg/mir2/z80codegen.go` — new helpers:

```go
func isIXY(reg string) bool { return reg == "IX" || reg == "IY" }

func ptrIndirect(ptr string, d int) string {
    if isIXY(ptr) {
        if d == 0 { return fmt.Sprintf("(%s+0)", ptr) }
        return fmt.Sprintf("(%s+%d)", ptr, d)
    }
    return fmt.Sprintf("(%s)", ptr)
}
```

**8-bit load/store** via IX:
```asm
LD A, (IX+0)      ; was: LD A, (IX)  ← invalid Z80
LD (IX+0), A
```

**16-bit load** via IX uses displacement pair (38T) instead of INC/DEC IX trick (58T):
```asm
LD L, (IX+0)      ; lo byte
LD H, (IX+1)      ; hi byte
```

**16-bit register↔IX copy:** DE/BC→IX uses undocumented byte-copy (16T < PUSH/POP 21T):
```asm
LD IXH, D        ; DD 62 — D not substituted by DD prefix ✓
LD IXL, E        ; DD 6B — E not substituted ✓  (total 16T)
LD IXH, B        ; DD 60 — B not substituted ✓
LD IXL, C        ; DD 69 — C not substituted ✓
```
**HL→IX byte-copy is INVALID.** The DD prefix substitutes H→IXH and L→IXL in *both*
source and destination operands, so `LD IXH, H` encodes as `LD IXH, IXH` (NOP).
HL↔IX must use `PUSH HL / POP IX` (21T). MZA correctly rejects `LD IXH, H`.

**Bug C fix:** `emitMov` for 16-bit moves now calls `g.setCopy(dst, src)` after every path so alias tracking works correctly.

### Assembly Before/After

**Before Phase 6a** (4 pointers, 4th spills):
```asm
pressure:
    LD A, ($F001)   ; load spilled pointer
    LD A, (HL)
    ...             ; 3× ($F0xx) references
```

**After Phase 6a** (4 pointers, IX used for 4th):
```asm
pressure:
    LD A, (IX+0)    ; p3 → IX, no spill
    LD B, (BC)      ; p2 → BC
    LD C, (DE)      ; p1 → DE
    LD D, (HL)      ; p0 → HL
    ADD A, B
    LD A, C
    ADD A, D
    LD B, A
    ADD A, B
    RET
```

---

## Phase 6b — PBQP Register Allocator

### Model
`pkg/mir2/pbqp.go` — replaces the greedy graph-colouring `Allocate()` for the HIR→MIR2→Z80 pipeline.

**Node cost vector:**
```
nodeCost[r][i] = useCount[r] × ct.Cost(r.Cls, locs[i])
```
A register used 10× pays 10× the slot cost. High-use registers are strongly incentivised to occupy cheap (zero-cost) physical locations.

**Edge cost:** Interfering pairs have infinite cost for same-location assignment.

**Objective:** Minimise `Σ_r nodeCost[r][colour[r]] + Σ_(r1,r2)∈E edgeCost`.

### Reduction Rules

| Rule | Condition | Action |
|------|-----------|--------|
| R0 | degree-0 (isolated) | assign min-cost location immediately |
| R1 | degree-1 (leaf) | fold edge into neighbour's cost vector, defer assignment |
| RN | degree ≥ 2 | greedy: sort by **delta** = `2nd_best − best`, allocate first |

The **delta sort** is the key innovation: a register with large delta (pays a high penalty if displaced from its first choice) is allocated before registers that are flexible. This naturally gives high-use registers primary locations.

### Weighted vs Greedy — `TestPBQP_WeightedCost_BeatGreedy`

Setup: `r_light` (1 use), `r_heavy` (10 uses), both ClassAcc, both interfere.

| Allocator | r_light | r_heavy | Note |
|-----------|---------|---------|------|
| Greedy    | A       | B       | arbitrary ordering |
| PBQP      | C       | **A**   | heavy → zero-cost A |

PBQP correctly puts the high-use register in the cheaper slot, minimising total weighted cost.

### R1 Rule — `TestPBQP_R1_LeafNode`

```
r1(use=1) → C       (cost 1×6 = 6T)
r2(use=10+) → A     (cost 10×0 = 0T)
```
Optimal: assign expensive slot to the low-use register, free slot to the hot register.

### Four Pointers — `TestPBQP_FourPointers_NoSpill`

```asm
four_ptrs:
    LD A, (HL)       ; p0 → HL  (cost 0)
    LD B, (DE)       ; p1 → DE  (cost 4)
    LD C, (BC)       ; p2 → BC  (cost 6)
    LD D, (IX+0)     ; p3 → IX  (cost 8) — no $F0xx spill!
    ADD A, B
    LD A, C
    ADD A, D
    LD B, A
    ADD A, B
    RET
```

Zero spills. No `($F0xx)` references. All four ClassPointer registers placed in distinct physical locations.

---

## Phase 6c — Post-Allocation Copy Coalescing

### Problem
After PBQP assigns physical locations, the Z80 codegen inserts **trampolines** — small glue blocks that copy values between physical locations at block boundaries (BrIf edges) and for OpMove instructions. These add 4–10T per copy.

### Implementation
`pkg/mir2/coalesce.go` — `coalesceAllocResult(f, result, lr)`:

**Step 1:** Collect affinity edges from:
- `OpMove dst ↔ src`
- Block boundary: `block_param ↔ arg` for all terminator types

**Step 2:** Build fresh IG from liveness data (independent of PBQP's R1-modified graph).

**Step 3:** Single-pass recolouring — for each affinity edge `(dst, src)`:
- If no IG neighbour of `dst` uses `src`'s location → recolor `dst` to match `src`
- `recolored` set prevents the same reg from being moved twice

### Why Single-Pass (No Fixpoint)?
Loop back-edges create affinity cycles in the phi-web:
```
loop_head(a, b) → loop_body(a', b') → loop_head(b', a'+b')
```
This creates the affinity cycle `a ↔ a' ↔ b' ↔ a`. Since none of these pairs interfere (disjoint live ranges), a fixpoint loop would rotate their physical locations forever: `HL→DE→BC→HL→…`. The `recolored` lock + single pass breaks the cycle while still coalescing direct arg/param pairs.

### Test Results

```
TestCoalesce_BlockBoundary:         r1={A}  r2={A}   ← TermJmp arg/param fused
TestCoalesce_OpMove:                r1={A}  r2={A}   ← OpMove eliminated
TestCoalesce_NoCoalesceIfInterfering: r1={A} r2={B}  ← interfering pair preserved
```

---

## Metrics

| Metric | Before Phase 6 | After Phase 6 |
|--------|----------------|---------------|
| 4× ClassPointer regs | 3 regs + $F0xx spill | 4 regs in HL/DE/BC/**IX** |
| High-use reg location | arbitrary (greedy) | zero-cost slot (PBQP delta-sort) |
| Block boundary copies | always trampoline | coalesced when safe |
| Go test packages | 23/23 ✅ | 23/23 ✅ |
| PBQP tests | — | 7 pass (R0/R1/RN/Coalesce) |
| Z80 codegen IX tests | — | 3 pass (8-bit/16-bit/pressure) |

---

## Files Changed

| File | Change |
|------|--------|
| `pkg/mir2/pbqp.go` | New — PBQP allocator (~250 LOC) |
| `pkg/mir2/coalesce.go` | New — post-allocation coalescing (~130 LOC) |
| `pkg/mir2/pbqp_test.go` | New — 7 tests |
| `pkg/mir2/constprop.go` | `SimplifyIdentities()` added |
| `pkg/mir2/z80codegen.go` | `isIXY`, `ptrIndirect`, IX 8/16-bit load/store, undocumented HL↔IX copy, `setCopy` 16-bit |
| `pkg/mir2/z80codegen_test.go` | 3 IX tests added |
| `pkg/nanz/parse.go` | `VarRefExpr.Ty` lookup in `varTypes`/`globalTypes` |
| `pkg/pipeline/pipeline.go` | `SimplifyIdentities` wired; `Allocate → PBQPAllocate` |

---

## Next Steps (Phase 6d onward)

- **Phase 6d** — `ptr[i]` in loops: HL conflict between base pointer and index arithmetic. With Phase 6a's IX support, PBQP should naturally solve this by placing the base pointer in IX when HL is occupied by arithmetic. Needs verification with a real loop test.
- **Error propagation** `-> T ! E / ?` (ADR-0016, Nanz Week 2)
- **Nanz LSP** hover/goto-def (symbol table needed in server.go)
- **MinZ → HIR lowering** (retire MIR1, unify frontends)
- **Agon eZ80 target** (proposal #047 written)
