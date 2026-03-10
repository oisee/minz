# MIR2 Open Bugs — Root Cause Analysis

**Last updated:** 2026-03-10
**Status key:** 🔴 blocking | 🟡 degraded (correct but slow) | 🟢 tracked/deferred

---

## BUG-001 🟡 GCD parallel-copy bloat (CFG diamond, block params)

**Symptom:** `gcd(a,b)` compiles correctly but with 8+ redundant `LD A,x; LD x,A`
moves at every loop-back and conditional branch edge.

**Example generated:** `.loop_back` emits `LD A,C / LD C,D / LD D,A` just to
re-synchronize `a` and `b` into the registers the loop header expects.

**RCA:** PBQP allocator solves each virtual register independently.  It assigns
`a→C, b→D` in the loop body but `a_param→D, b_param→C` for the block
parameters in the loop header (or vice versa) because there are no *affinity
edges* between a block-arg and the corresponding block-param.  The parallel-copy
resolver must emit swap sequences on every CFG edge where source and destination
register assignments differ.

**Architecture note:** MIR2 uses Cranelift-style block parameters (not φ-nodes).
A block edge `Jmp(head, [a_result, b_result])` creates an implicit parallel copy
`(head_param_a ← a_result), (head_param_b ← b_result)`.  Without a Union-Find
pre-coalescing pass or PBQP affinity edges, the allocator is blind to this
preferred alignment.

**Fix options:**
1. **Pre-allocation coalescing (Union-Find):** Before PBQP, union block-arg and
   block-param virtual registers if their live ranges are non-overlapping
   (they always "touch" at the branch — safe to merge).  This eliminates the
   interference and the parallel copy.
2. **PBQP affinity edges:** Add a soft-cost edge between each block-arg and its
   block-param with cost = T-states of the `LD` sequence needed to resolve the
   mismatch.  PBQP then picks the globally cheapest coloring.
3. **Post-allocation coalescing (Phase 6c already live):** Partially handles
   simple cases (single-block functions) but leaves diamond CFGs unsolved.

**Priority:** Medium.  GCD is ~25% slower than hand-written Z80 due to this.

---

## BUG-002 🟡 forEach entry scheduling — constant rematerialization in block params

**Symptom:** `forEach` iterator prologue emits `LD D,0; LD B,C; LD C,D` —
a constant `0` is being passed as a block parameter through the parallel-copy
mechanism instead of being rematerialized inline.

**Example:**
```z80
forEach:
    LD D, 0      ; unnecessary — D=0 should be rematerialized at use site
    LD B, C      ; move counter to B
    LD C, D      ; C = 0 (was D)
```

**RCA:** The loop head receives `(counter, accumulator=0)` as block params.
The `accumulator=0` is a constant in the IR (`OpConst 0`), but the allocator
treats it as any other virtual register.  The parallel-copy resolver emits
`LD D, 0` then a swap to get the constant into the expected register.  A
*rematerialization* optimization would recognize that constants can be
re-computed at any use site and never need a "copy" instruction.

**Fix:** In the parallel-copy resolver, when the source of a copy is a constant
value (tracked via `constVals` map), emit `LD dst, imm` directly instead of
`LD dst, src_reg`.  Estimated: ~20-30 lines.

**Priority:** Low.  Cosmetic for simple cases; more impactful for longer chains.

---

## BUG-003 🔴 `ptr[i]` inside while loop — broken EX DE,HL / ADD F,DE

**Symptom:** Accessing `ptr[i]` inside a while loop produces invalid Z80:
`EX DE,HL` at wrong points, `ADD F,DE` (F is not a general register).

**Example:**
```nanz
while i < n {
    val = ptr[i]    // ← breaks codegen
    i = i + 1
}
```

**RCA:** The HIR loop variable threading pass adds ALL variables read/written in
the loop body to the set of loop block parameters.  When both a pointer `ptr`
(u16, ClassPointer→HL) and an index `i` (u8, ClassCounter→B) are loop params,
the parallel-copy resolver must emit a sequence to simultaneously update HL and
B across the back-edge.  The `PtrAdd` instruction (`ADD HL, DE`) requires HL=ptr
and DE=i_extended.  If the copy resolution moves HL before DE is set (or uses F
as if it were a register), the sequence is invalid.

**Sub-issues:**
- `OpPtrAdd` on Z80 requires `HL += DE`; if `ptr` was in DE before the copy,
  using `EX DE,HL` leaves the wrong register pair in HL.
- Loop-back parallel copy walks BACKWARD (correct per design) but the PtrAdd
  register constraints aren't respected during cycle resolution.

**Fix:** In `emitParallelCopy`, when a cycle involves HL and DE, check if any
live instruction between the copies requires HL as a pointer base.  Use a
scratch register (BC or stack) to break the cycle safely.

**Priority:** High.  Blocks any loop that iterates over an array.

---

## BUG-004 🟡 Non-zero-lo LUT (e.g. `u8<10..20>`) — contract opt class mismatch

**Symptom:** LUTGen correctly builds the lookup table and emits `Sub(x, Const(lo))`
before the table lookup.  But the interprocedural contract optimizer runs AFTER
LUTGen and infers a different register class for the parameter (e.g. ClassAcc
instead of ClassCounter), which conflicts with the hardcoded `Sub(lo)` that
assumes a specific class.  Unit tests pass (bypass contract opt), pipeline broken.

**RCA:** `LUTGen` rebuilds the function body with a fixed IR structure:
`Param(lo_sub) → Sub(lo) → Ext(u16) → AddrOf → PtrAdd → Load → Ret`.
The contract optimizer runs after LUTGen and reassigns the param class.  The
new class is incompatible with the Sub instruction's expected ClassAcc.

**Fix:** After LUTGen transforms a function, mark it as `Contract.Fixed = true`
(or equivalent) to prevent the contract optimizer from re-inferring the class.
Alternatively, have LUTGen annotate the Sub instruction with the class it needs.

**Priority:** Low.  Only affects ranged types with non-zero lower bound.

---

## BUG-005 🟡 `applySubSwapNeg` missing u16 guard

**Symptom:** For u16 subtraction in the "swapped" branch of abs_diff-style code,
`applySubSwapNeg` sets `ClassAcc` on the hoisted instruction.  For u16, ClassAcc
maps to A (8-bit), but the result needs to be in HL (16-bit, ClassPointer).
This produces incorrect codegen for u16 abs_diff via the classic pattern.

**RCA:** `applySubSwapNeg` in `condret.go` replaces `sub(y,x)` with `neg(r_h)`
and inherits the class from the hoisted instruction.  The hoisted instruction
is in ClassAcc for u8, which is correct.  For u16, the hoisted instruction
should be ClassPointer.  No width guard exists.

**Fix:** In `applySubSwapNeg`, check `h.Ty.Width() > 8` and set `ClassPointer`
instead of `ClassAcc`.  One line.

**Workaround:** The IAR-style pattern (Sub then Neg directly, commit 96598d2)
avoids `applySubSwapNeg` entirely for u16 by using `fusionSubCmpInBlock` +
`CmpSubCarryNot` instead.

**Priority:** Low.  Workaround available.

---

## BUG-006 🔴 Zero-size struct globals (`struct Dog {}`) not emitted

**Symptom:** `global g_dog: Dog` where `Dog` has no fields emits no bytes.
Subsequent `LD HL, g_dog` references an undefined symbol, causing MZA to fail.

**RCA:** The global emitter checks `byteWidth(ty) == 0` and skips the global
entirely.  For zero-size types (structs with no fields), the symbol should still
be emitted as a bare label (EQU or zero-byte reserve) so that address-of
operations remain valid.

**Fix:** In the global emitter, if `byteWidth(ty) == 0`, emit `g_dog: EQU $`
(or `DEFS 0`) so the symbol exists in the symbol table without consuming bytes.

**Priority:** Medium.  Affects interface/polymorphism examples with marker types.

---

## Summary table

| Bug | Category | Severity | Fix size | Status |
|-----|----------|----------|----------|--------|
| BUG-001 GCD parallel-copy | Allocator (affinity) | 🟡 | Large | Open |
| BUG-002 forEach const rematl | Codegen (parallel copy) | 🟡 | Small | Deferred |
| BUG-003 ptr[i] in while loop | HIR/codegen (PtrAdd) | 🔴 | Medium | Open |
| BUG-004 Non-zero-lo LUT | Pipeline ordering | 🟡 | Small | Open |
| BUG-005 SubSwapNeg u16 guard | condret.go | 🟡 | Trivial | Deferred (workaround) |
| BUG-006 Zero-size struct global | Global emitter | 🔴 | Small | Open |
