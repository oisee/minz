# MIR2 Open Bugs — Root Cause Analysis

**Last updated:** 2026-03-13
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

## BUG-002 ✅ FIXED forEach entry scheduling — constant rematerialization in block params

**Fixed in:** 2026-03-12 (parallelCopy isImm/immVal deferred emission)

**Symptom:** `forEach` iterator prologue emits `LD D,0; LD B,C; LD C,D` —
a constant `0` is being passed as a block parameter through the parallel-copy
mechanism instead of being rematerialized inline.

**RCA:** The loop head receives `(counter, accumulator=0)` as block params.
The `accumulator=0` is a constant in the IR (`OpConst 0`), but the allocator
treated it as any other virtual register.  The parallel-copy resolver emitted
`LD D, 0` then a swap to get the constant into the expected register.

**Fix:** Extended `parallelCopy` struct with `isImm bool` and `immVal int64`.
In `buildBlockCopies`, when the source is a constant (`constVals` map), set
`isImm=true` instead of emitting inline.  In `emitParallelCopy`, all register
moves are resolved first, then `LD dst, imm` emissions follow — preventing
constant rematerialization from clobbering live source registers mid-swap.
Also added `src == dst` early-exit to handle pre-coalesced or same-location pairs.

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

## BUG-004 ✅ FIXED Non-zero-lo LUT (e.g. `u8<10..20>`) — contract opt class mismatch

**Fixed in:** 2026-03-12 (pipeline reordering — LUTGen moved after OptimizeContracts)

**Symptom:** LUTGen correctly builds the lookup table and emits `Sub(x, Const(lo))`
before the table lookup.  But the interprocedural contract optimizer ran AFTER
LUTGen and inferred a different register class for the parameter (e.g. ClassAcc
instead of ClassCounter), conflicting with the hardcoded `Sub(lo)` that assumed
ClassAcc.  Unit tests passed (bypass contract opt), pipeline broken.

**RCA:** `LUTGen` rebuilt the function body with a fixed IR structure.  The
contract optimizer ran after LUTGen and reassigned the param class, producing
a class mismatch.

**Fix:** In `pipeline.go` (`CompileHIRSteps` and `CompileHIRWithOptions`), moved
`mir2.LUTGen(m)` to run AFTER `OptimizeContracts` + `ApplyContracts`.  Contract
optimizer sees the original function signatures; LUTGen synthesizes after
contracts are frozen.

---

## BUG-005 ✅ FIXED `applySubSwapNeg` missing u16 guard

**Fixed in:** 2026-03-12 (condret.go — width-conditional ClassPointer vs ClassAcc)

**Symptom:** For u16 subtraction in the "swapped" branch of abs_diff-style code,
`applySubSwapNeg` set `ClassAcc` on the hoisted instruction.  For u16, ClassAcc
maps to A (8-bit), but the result needs HL (16-bit, ClassPointer).

**RCA:** `applySubSwapNeg` in `condret.go` replaced `sub(y,x)` with `neg(r_h)`
and inherited the class from the hoisted instruction without checking width.

**Fix:** Added `h.Ty.Width() > 8` guard: emit `inst.Cls = ClassPointer` for
16-bit, `ClassAcc` for 8-bit.  One-line fix.

---

## BUG-006 ✅ FIXED Zero-size struct globals (`struct Dog {}`) not emitted

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

## BUG-007 ✅ FIXED Spurious adapter LD when caller/callee share convention

**Symptom:** When two functions share an identical PFCCO contract, the codegen
emits a spurious `LD` that overwrites the first argument with the second.

**Reproducer:**
```nanz
fun add(a: u8, b: u8) -> u8 { return a + b }
fun add_then_double(a: u8, b: u8) -> u8 {
    let s = add(a, b)
    return s + s
}
```

Both functions get contract `(a=A, b=C)`.  `add_then_double` should call
`add(a, b)` with no setup (args already in place), but codegen emits:
```z80
add_then_double:
    LD A, C         ; WRONG: overwrites a with b
    CALL add        ; computes add(b, b) instead of add(a, b)
    ADD A, A
    RET
```

**Masked by:** Constant folding in `main`.  If `add_then_double(3, 4)` is
called with constants, `main` is folded to `LD A, 14; RET` (correct).  But
the `add_then_double` function body is wrong for dynamic inputs.

**RCA:** The call-site lowering in `z80codegen.go` emits argument setup
moves based on the *default* convention expectation, not the *actual*
convention assigned by PFCCO.  When PFCCO assigns an identical convention
to both caller and callee, the setup code should be a no-op, but the
codegen doesn't check for this case.

**Fix options:**
1. **Skip identity moves:** Before emitting call-site argument setup, check
   if the source and destination registers are the same.  Skip the `LD` if so.
2. **Post-RA dead move elimination:** A peephole pass that removes `LD X, X`
   (literal same register) and identity parallel copies.

**Priority:** Medium.  Masked by constant folding in most small programs, but
will cause incorrect results in larger programs with dynamic call chains.

**Discovered:** 2026-03-13, during PFCCO paper validation.

---

## Summary table

| Bug | Category | Severity | Fix size | Status |
|-----|----------|----------|----------|--------|
| BUG-001 GCD parallel-copy | Allocator (affinity) | 🟡 | Large | Open (Phase 6e nudge applied) |
| BUG-002 forEach const rematl | Codegen (parallel copy) | 🟡 | Small | ✅ Fixed 2026-03-12 |
| BUG-003 ptr[i] in while loop | HIR/codegen (PtrAdd) | ~~🔴~~ | Medium | ✅ Fixed (37b934d) |
| BUG-004 Non-zero-lo LUT | Pipeline ordering | 🟡 | Small | ✅ Fixed 2026-03-12 |
| BUG-005 SubSwapNeg u16 guard | condret.go | 🟡 | Trivial | ✅ Fixed 2026-03-12 |
| BUG-006 Zero-size struct global | Global emitter | 🔴 | Small | Open |
| BUG-007 Spurious adapter LD | Codegen (PFCCO+RA) | 🟡 | Medium | Open |
