# Report #049 — LUT Pointer Selection, Peephole Fix, and PBQP Edge Cost Architecture

**Date:** 2026-03-10
**Status:** Fix shipped (18T), BC★ roadmapped, ADR-0017 written

---

## Summary

During LUT access code review, a wasted instruction was identified in the page-aligned fast
path: `LD HL, sym` loads both H and L, but L is always immediately overwritten by the index
register. Only H (the page base) is needed. Fix: `LD H, sym^H` — saves 3T and 1 byte.

The analysis also revealed a deeper optimization opportunity (BC★) and clarified the
architectural boundary between peephole/instruction-selection and PBQP edge costs.

---

## Background: Page-Aligned LUT Fast Path

LUTGen (ADR-0015) emits page-aligned lookup tables for `u8<lo..hi>` ranged parameters.
The page-aligned fast path (`Align=256`) produces 3-instruction access because the index
*is* the low byte — no pointer arithmetic needed.

**Before this fix (21T, 6 bytes):**

```asm
LD HL, popcount_lut   ; 10T, 3 bytes — loads H=0x5B, L=0x00 (low byte of sym)
LD L, C               ; 4T, 1 byte  — immediately clobbers L with index
LD A, (HL)            ; 7T, 1 byte  — indirect load
RET                   ; 10T
; Access cost: 21T, 5 bytes (excluding RET)
```

L was computed and immediately discarded. The `LD HL, sym` is a 3-byte immediate load
of a 16-bit address — exactly one of those bytes (the low byte) was useless.

**After fix (18T, 5 bytes):**

```asm
LD H, popcount_lut^H  ; 7T, 2 bytes — only page base (high byte)
LD L, C               ; 4T, 1 byte  — index into L
LD A, (HL)            ; 7T, 1 byte  — indirect load
RET                   ; 10T
; Access cost: 18T, 4 bytes (excluding RET)
```

`sym^H` = `(address >> 8) & 0xFF`. MZA's expression evaluator already supported this suffix
with no assembler changes required.

**Saving: −3T (−14%), −1 byte per LUT access.**

---

## Analysis: All Pointer Registers for LUT Access

Expanding the question: can DE, BC, IX, or IY also perform LUT access, and are any cheaper?

| Ptr | Setup instructions | Setup T | Load instruction | Load T | Total T | Valid? |
|-----|--------------------|---------|------------------|--------|---------|--------|
| HL  | `LD H, sym^H` + `LD L, idx` | 11T | `LD A, (HL)` | 7T | **18T** | ✅ general + page-aligned |
| DE  | `LD D, sym^H` + `LD E, idx` | 11T | `LD A, (DE)` | 7T | **18T** | ✅ page-aligned only |
| BC  | `LD B, sym^H` + `LD C, idx` | 11T | `LD A, (BC)` | 7T | **18T** | ✅ page-aligned only |
| BC★ | `LD B, sym^H` *(idx already in C)* | 7T | `LD A, (BC)` | 7T | **14T** | ✅ page-aligned, idx in C |
| DE★ | `LD D, sym^H` *(idx already in E)* | 7T | `LD A, (DE)` | 7T | **14T** | ✅ page-aligned, idx in E |
| IX  | `LD IXH, sym^H` + `LD IXL, idx` | ~19T | `LD A, (IX+0)` | **19T** | **~38T** | ❌ too slow |
| IY  | `LD IYH, sym^H` + `LD IYL, idx` | ~19T | `LD A, (IY+0)` | **19T** | **~38T** | ❌ too slow |

**Why IX/IY are wrong for LUT:** The `DD`/`FD` displacement prefix costs +12T on every
indirect load (`LD A,(IX+0)` = 19T vs `LD A,(HL)` = 7T). IX is optimized for struct field
access with a compile-time constant displacement `d`, not dynamic table lookup where the
index changes on every call and the 12T penalty is never amortized.

**Why DE/BC can't do general (non-page-aligned) LUT:**
General LUT requires `ptr = table_base + runtime_index`. Z80 only has `ADD HL, DE` for
16-bit pointer arithmetic — there is no `ADD DE, xx` or `ADD BC, xx`. HL is mandatory for
non-page-aligned access.

---

## BC★ Optimization

The fastest possible page-aligned LUT access: **14T, 3 bytes.**

If the allocator assigns the index to C and B is not live at the access point:

```asm
LD B, sym^H   ; 7T — page base only
; index is already in C — no move!
LD A, (BC)    ; 7T — direct indirect load
; Total: 14T, 3 bytes
```

Saves 4T more vs the current 18T path. The pattern depends on a **correlated allocation
decision**: the optimal pointer register for `ptr_virtual` depends on where `idx_virtual` was
allocated. This is the key insight for the PBQP question below.

---

## Where Does This Optimization Belong?

### Instruction selection (current, correct)

The `LD H, sym^H` fix lives in `scanLUTPatterns` / `OpLoad` in `z80codegen.go`. This is
the right level — the LUT pattern is structurally known at this point, before assembly text
is emitted.

### PBQP node costs (current PBQP) — cannot help

PBQP's current formulation:
```
minimize Σ nodeCost[v][loc(v)]
```

Each virtual register is priced independently. PBQP cannot see that `(idx→C) + (ptr→BC)`
is jointly cheaper than `(idx→C) + (ptr→HL)`. It makes each decision without knowing the
other.

### PBQP edge costs — architecturally correct, deferred

Full PBQP:
```
minimize Σ nodeCost[v][loc(v)]  +  Σ edgeCost[u][v][loc(u)][loc(v)]
```

For a detected page-aligned LUT access, register one edge:
```go
edgeCost[idx_reg][ptr_reg][C][BC] = -4   // 4T reward for joint assignment
edgeCost[idx_reg][ptr_reg][E][DE] = -4
edgeCost[idx_reg][ptr_reg][_][_ ] =  0   // otherwise uncoupled
```

PBQP reduction rules would naturally propagate the reward and prefer `ptr→BC` when
`idx→C` is tentative. This is **exactly the problem edge costs were invented to solve**.

**Why deferred:** RN reduction with edge cost projection (bucket elimination) adds ~150 LOC
to the PBQP solver. We don't yet have enough correlated-allocation patterns to justify the
complexity. The post-allocation codegen check achieves the same result for LUT patterns
at minimal cost.

### Post-allocation codegen check (BC★, planned)

```go
// In scanLUTPatterns / OpLoad, after alloc:
idxLoc := result.Locs[pat.idxReg]
if idxLoc == LocC && !liveRegs.Has(LocB) {
    emit("LD B, %s^H", sym)   // 7T
    // LD L,C not needed — index already in correct half
    emit("LD A, (BC)")         // 7T
    // Total: 14T
}
```

This is purely local (no allocator changes) and handles the most common case.
Planned follow-up to this report.

---

## Files Changed

| File | Change |
|------|--------|
| `pkg/mir2/z80codegen.go` | LUT fast path: `LD HL, sym` → `LD H, sym^H`; `invalidate("HL")` → `invalidate("H")` |
| `docs/adr/0017-lut-pointer-selection-and-pbqp-edge-costs.md` | New ADR: full architectural analysis |

---

## Test Results

All 23/23 packages pass including:
- `TestLUTGen_*` (7 tests) — page-aligned and general LUT paths
- `TestPBQP_*` (6 tests) — allocator correctness
- `TestCoalesce_*` (3 tests) — copy coalescing

---

## T-State Summary

| LUT access variant | T-states | Bytes | Notes |
|--------------------|----------|-------|-------|
| HL, before fix     | 21T      | 6     | Wasted L load |
| HL, after fix      | 18T      | 5     | `LD H, sym^H` |
| DE/BC (page-aligned) | 18T    | 5     | Same cost, different reg |
| BC★ (idx in C)     | 14T      | 3     | Planned codegen check |
| IX/IY              | ~38T     | 6     | Never use for LUT |

---

## Next Steps

1. **BC★ codegen check** — 20 LOC in `scanLUTPatterns`, gains 4T on common patterns
2. **Iterator T-state benchmark** — run forEach/map/filter through PBQP, measure vs ~43T ideal
3. **PBQP edge costs (Phase 6e)** — when 3+ correlated-allocation patterns exist

---

*See ADR-0017 for full architectural rationale and alternatives considered.*
