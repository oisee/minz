# MIR2 Open Bugs — Root Cause Analysis

**Last updated:** 2026-03-15
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

**ZX Spectrum impact:** The allocator also spills to `$0000` (absolute address 0), which
is **ROM on ZX Spectrum** — writes silently fail, reads return ROM bytes.  This makes
any function with spills produce incorrect results on real hardware.  The spill base
address must be relocated to RAM (e.g. `$F000+`).

**Tetris impact:** 13 `LD A, ?` (unresolved virtual register name in output) and 9
`$0000` memory spills across 48 functions.  The game compiles but is un-runnable on
real ZX Spectrum hardware.

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

## BUG-003 ✅ FIXED `ptr[i]` inside while loop — broken EX DE,HL / ADD F,DE

**Fixed in:** 2026-03-12 (5 interacting codegen fixes, report #062)

**Symptom:** Accessing `ptr[i]` inside a while loop produced invalid Z80:
`EX DE,HL` at wrong points, `ADD F,DE` (F is not a general register).

**RCA:** Not one bug but five layered codegen errors: (1) `OpPtrAdd` emitted
`ADD DE,BC` — invalid Z80; fixed with PUSH HL / ADD HL,off / LD r,(HL) / POP HL.
(2) `coalesceAllocResult` missed pair/component aliases (C vs BC). (3) Block param
in A clobbered by loop compare. (4) `pendingAccReg` not set after 8-bit ADD.
(5) `buildBlockCopies` used canonical loc instead of `physOverride`.

E2E verified: `sum_array(ptr, 4) = 100`, `sum_array(ptr, 5) = 150`.

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

## BUG-006 🟡 Zero-size struct globals (`struct Dog {}`) not emitted

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

## BUG-007 🟡 Spurious adapter LD when caller/callee share convention

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

## BUG-008 🔴 Arena codegen: impossible `LD IXL, (IX+d)` + self-pointer loss

**Symptom:** `Arena_alloc` generates `LD IXL, (IX+0)` and `LD IXH, (IX+1)` —
instructions that cannot be encoded on Z80.  MZA correctly rejects them.
MIR2 VM tests pass (don't touch Z80 codegen), masking the bug.

**Verified:** MZA outputs `unknown instruction or invalid operands: LD` for both.

**Reproducer:**
```nanz
struct Arena { ptr: u16, end: u16 }
fun Arena.alloc(self: ^Arena, n: u16) -> u16 {
    let result: u16 = self.ptr
    let next: u16 = result + n
    if next > self.end { return 0 }
    self.ptr = next
    return result
}
```

**RCA (3 compounding bugs):**

1. **PBQP missing constraint:** Allocator assigns ClassIXY8 (IXH/IXL) as
   destination for IX-indexed loads.  No constraint prevents dest=IXL when
   source=(IX+d) — the DD prefix can't remap both simultaneously.

2. **Self-clobber:** Even if the instruction existed, `LD IXL, (IX+0)` would
   change IX itself, making the subsequent `LD IXH, (IX+1)` read from a
   corrupted base address.  Must use scratch registers (A, B, etc).

3. **`LD H, IXH` invalid:** DD prefix remaps H→IXH, so `LD H, IXH` becomes
   `LD IXH, IXH` (NOP).  Valid IXH/IXL instructions require other operand
   to be A/B/C/D/E only.  MZA correctly rejects `LD H, IXH`.

4. **Self-pointer lost:** After `ADD HL, BC` (computing `next`), HL no longer
   holds `self`.  Codegen derives `self.end` from `next` via `PUSH HL; POP IX;
   INC IX; INC IX` — but `next+2 ≠ self+2`.

5. **Codegen blind combination:** `z80codegen.go:1662-1666` calls `lowByte(dst)`
   and `ptrIndirect(ptr, d)` without checking if both are IX-family.

**Location:** `pkg/mir2/z80codegen.go:1662-1666`, `pkg/mir2/pbqp.go`

**Fix options:**
1. PBQP constraint: infinite cost for IXY8 dest with IX-indexed source
2. Codegen guard: if dest is IXH/IXL and ptr is IX, route through A register
3. Both (recommended)

**Broader impact:** Any struct method that does arithmetic (clobbers HL) then
accesses another field will hit the same pattern.  Blocks non-trivial struct
methods on Z80.

**Priority:** 🔴 Blocking.  Arena and all struct-with-pointer-receiver methods
broken on Z80 path.

**Discovered:** 2026-03-14, during arena allocator Z80 code review.
See Report #071 for full analysis.

---

---

## BUG-009 ✅ FIXED Array index generates `load [N]T` instead of addr_of

**Fixed in:** 2026-03-15 (037fbc5)

**Symptom:** `board[i] = 0` (global `board: [200]u8`) generates MIR2:
`addr_of @board → load [200]u8 → ptr_add → store` — the `load [200]u8`
tries to load the ENTIRE array as a u16 value, producing garbage pointer.

**RCA:** `lowerExpr` for `VarRefExpr` on a global had a struct special-case
(return address) but no array case. Arrays fell through to "scalar global:
load and return the value" — treating `[200]u8` as a scalar to be loaded.

**Fix:** Added `ArrayTy` check alongside `StructTy` in `hir/lower.go:1400`.
Arrays return `addr_of` (pointer to first element), same as structs.

**Ultimate fix:** Same — this is the correct semantics. Arrays are always
accessed by address on Z80, never loaded as values.

---

## BUG-010 ✅ FIXED (temp) Caller-save: CALL destroys live registers

**Fixed in:** 2026-03-15 (037fbc5) — temporary PUSH/POP fix

**Symptom:** `fill_cell` calls `zx_screen_addr` which clobbers C and D.
Loop counter (in D) and y*8 (in C) destroyed → infinite loop.

**RCA:** Z80 codegen had NO caller-save mechanism. When a function CALLs
another, it assumed all registers survive. But Z80 functions freely use
B/C/D/E/H/L and there's no callee-save convention.

**Temporary fix:** `callerSavePairs()` in z80codegen.go computes which
register pairs the callee clobbers (via `computeClobbers`), checks which
are allocated to virtual regs in the caller, and emits PUSH/POP around
the CALL. Over-saves (saves all allocated regs, not just live ones) but
correct.

**Ultimate fix:** PBQP EdgeCost (ADR-0020) — add instruction-level
constraints so the allocator avoids placing live values in registers that
the callee will clobber. Eliminates PUSH/POP overhead entirely.

---

## BUG-011 🟡 computeDeadConsts suppresses u16 cmp constants

**Fixed in:** 2026-03-15 (43ddcc6)

**Symptom:** `while addr < 0x5800` compiles to `SBC HL, DE` but DE is
never loaded — the `LD DE, 22528` is suppressed by dead constant elimination.

**RCA:** `computeDeadConsts` treated `OpCmp` rhs as foldable (like `CP imm8`)
without checking operand width. For u16, `SBC HL,DE` requires the rhs in
a register — can't fold. But `inst.Ty` for cmp is `bool` (1 bit), so the
width check `inst.Ty.Width() <= 8` always passed.

**Fix:** Check lhs physical register — if it's a pair (HL/DE/BC), the rhs
cannot be folded. Only fold when lhs is an 8-bit register (A/B/C/D/E/H/L).

---

## BUG-012 ✅ FIXED InlineTrivial inlines pointer-heavy functions

**Fixed in:** 2026-03-15 (47f277c)

**Symptom:** `CALL color_attr` and `CALL piece_color` missing from render()
asm output. Their function bodies (addr_of + ext + ptr_add + load) appeared
inlined instead, but with 16-bit pointers allocated to 8-bit registers →
`LD B, (A)`, `POP A` (invalid Z80).

**RCA:** `InlineTrivial(m, 4)` in pipeline.go inlined functions with ≤4
instructions. `color_attr` has exactly 4 (addr_of, ext, ptr_add, load).
When inlined into `render()` (high register pressure, 4 CALLs per loop
iteration), PBQP ran out of pair registers and allocated pointers to
8-bit regs.

**Discovery:** Added debug comments to genInst — confirmed genInst was
NEVER called for color_attr/piece_color OpCall instructions. Traced to
InlineTrivial removing them before codegen.

**Fix:** `isTrivialFunc` now excludes functions containing OpAddrOf or
OpPtrAdd — they require 16-bit pair registers that may not be available
in register-starved callers.

---

## BUG-013 ✅ FIXED ADD HL, IX/IY (invalid Z80)

**Fixed in:** 2026-03-15 (47f277c)

**Symptom:** `ADD HL, IX` in board_set — invalid Z80 instruction.

**RCA:** 16-bit ADD codegen emitted `ADD HL, %s` without checking if rhs
is IX/IY. Z80 ADD HL only accepts BC/DE/HL/SP.

**Fix:** Three code paths in genBinOp ADD guarded — route IX/IY through
PUSH/POP + temp pair (BC or DE).

---

## BUG-014 🔴 OPEN Tetris v2 screen black — fill_cell + render output

**Status:** Under investigation (profiled 2026-03-15)

**Symptom:** Tetris v2 assembles clean (0 errors), game loop runs (1M
instructions in 100 frames), but screen is entirely black.

**Profiling data:**
- zx_screen_addr called 35 times (expected 1600+ from fill_cell)
- init_screen writes 6912 bytes (correct: 6144 pixel + 768 attr = 0x00)
- render() calls zx_poke ~12K times but writes don't reach attr area
- board likely always empty (cell=0 → color_attr(0) = black)

**Hypotheses:**
1. fill_cell loop exits early (D/C counter corrupted by zx_screen_addr clobbers despite PUSH/POP fix)
2. board_get always returns 0 (pieces never lock to board)
3. zx_attr_addr returns wrong addresses (codegen bug in address calculation)
4. spawn_piece fails silently (piece type/position never set)

**Next steps:** Add border markers, trace board_get return values, verify
fill_cell loop counter preservation.

---

## Summary table

| Bug | Category | Severity | Fix size | Status |
|-----|----------|----------|----------|--------|
| BUG-001 GCD parallel-copy | Allocator (affinity) | 🟡 | Large | Open (Phase 6e nudge applied) |
| BUG-002 forEach const rematl | Codegen (parallel copy) | 🟡 | Small | ✅ Fixed 2026-03-12 |
| BUG-003 ptr[i] in while loop | HIR/codegen (PtrAdd) | ~~🔴~~ | Medium | ✅ Fixed (37b934d) |
| BUG-004 Non-zero-lo LUT | Pipeline ordering | 🟡 | Small | ✅ Fixed 2026-03-12 |
| BUG-005 SubSwapNeg u16 guard | condret.go | 🟡 | Trivial | ✅ Fixed 2026-03-12 |
| BUG-006 Zero-size struct global | Global emitter | 🟡 | Small | Open |
| BUG-007 Spurious adapter LD | Codegen (PFCCO+RA) | 🟡 | Medium | Open |
| BUG-008 Arena IXL,(IX+d) | Allocator+Codegen | 🔴 | Medium | Open |
| BUG-009 Array load [N]T | HIR lowerer | 🔴 | Trivial | ✅ Fixed 2026-03-15 |
| BUG-010 Caller-save missing | Codegen | 🔴 | Medium | ✅ Fixed (temp) 2026-03-15 |
| BUG-011 Dead const u16 cmp | Codegen (DCE) | 🔴 | Small | ✅ Fixed 2026-03-15 |
| BUG-012 InlineTrivial ptr-op | Inliner+Allocator | 🔴 | Small | ✅ Fixed 2026-03-15 |
| BUG-013 ADD HL, IX/IY | Codegen (16-bit ADD) | 🟡 | Small | ✅ Fixed 2026-03-15 |
| BUG-014 Tetris black screen | Codegen/Logic | 🔴 | Unknown | Open (profiled) |
