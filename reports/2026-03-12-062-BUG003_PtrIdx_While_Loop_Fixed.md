# Report #062 — BUG-003 Fixed: ptr[i] in While Loop + Codegen Correctness Fixes

**Date:** 2026-03-12
**Status:** 23/23 showcase PASS, 26/26 test packages PASS, BUG-003 resolved

---

## Summary

BUG-003 (`ptr[i]` in while loop producing invalid Z80 assembly) is fixed. Along the way four
additional codegen correctness bugs were discovered and fixed. All fixes together required changes
to codegen, the coalescer, and the PBQP/greedy allocator's alias detection.

---

## Fixes

### Fix 1 — `OpPtrAdd` always routes through HL (`z80codegen.go`)

**Symptom:** `ptr[i]` in a while loop produced `ADD DE, BC` (invalid Z80) or
`EX DE,HL / ADD F, DE` (nonsense).

**Root cause:** OpPtrAdd codegen emitted `ADD %s, %s` with the destination pair directly,
but Z80 only supports `ADD HL, rr` — no `ADD DE, rr` or `ADD BC, rr`.

**Fix:** OpPtrAdd always routes through HL:
1. Compute offset into an off-pair (DE or BC, whichever doesn't conflict with base or off)
2. Move base into HL
3. `ADD HL, off_pair`
4. If the base is live after the ptr_add (loop case): `PUSH HL` before, `POP HL` after the load
   — keeps the base pointer available for the next iteration
5. `physOverride[ptr_add_dst] = "HL"` so the subsequent load uses `LD r,(HL)` directly

---

### Fix 2 — `coalesceAllocResult` alias-aware safety check (`coalesce.go`)

**Symptom:** PBQP placed n (r27: u8) in C and ptr (r28: ^u8) in BC — a physical conflict,
since C is the low byte of BC. The coalescer recolored n to C without noticing that BC aliases C.

**Root cause:** The coalescing safety check used exact equality (`result.Locs[n] == srcLoc`)
to detect conflicts. It didn't check sub-register aliases (C ↔ BC, D ↔ DE, etc.).

**Fix:** Use `physicallyConflicts(result.Locs[n], srcLoc)` in both the ISA-recoloring pass and
the affinity-edge coalescing loop:
```go
ig.Neighbors(e.dst).Each(func(n Reg) {
    if physicallyConflicts(result.Locs[n], srcLoc) {
        safe = false
    }
})
```

After this fix, n (r27) is placed in IXL (not C), eliminating the alias conflict.

---

### Fix 3 — Block param in A clobbered by compare; restore before terminal (`z80codegen.go`)

**Symptom:** Loop compare (`LD A, i; CP n`) clobbered acc (a block param in A) before the
loop body could use it, because `materializePendingAcc` fired but physOverride[acc]="E"
was only valid within the loop-head block — not in the successor loop-body block (live-through).

**Root cause:**
1. `genBlock` now sets `pendingAccReg` from block params allocated to A (so `materializePendingAcc`
   fires before any compare in that block saves acc to scratch).
2. `accStillNeeded` didn't check the block terminal — so it returned false even when acc was
   passed via TermBrIf to successor blocks.
3. `physOverride` for a block param persisted into successor blocks (not cleared at block entry),
   but E might be reused there for unrelated code.

**Fix:**
- Extend `accStillNeeded` to also scan `b.Term.termUses()` (block terminal's register uses).
- Add restore in `genTerm`: before emitting any terminal, restore block params that were saved to
  scratch registers back to their canonical physical location:
  ```go
  for _, bp := range g.curBlock.Params {
      if scratch, ok := g.physOverride[bp.Dst]; ok {
          canon := g.ar.Loc(bp.Dst).Name
          if canon != "" && canon != scratch {
              g.emitf("    LD %s, %s    ; restore block param from scratch", canon, scratch)
          }
          delete(g.physOverride, bp.Dst)
      }
  }
  ```
  This emits `LD A, E` (restore acc from scratch) before the conditional jump, so both the
  loop-body (live-through) and loop-exit (explicit arg) blocks receive acc in A.

---

### Fix 4 — `pendingAccReg` set after plain 8-bit ADD (`z80codegen.go`)

**Symptom:** In the loop body: `acc = acc + arr[i]; i = i + 1` — the second ADD
(`LD A, IXH; ADD A, 1`) clobbered acc (in A) before the parallel copy saved it.

**Root cause:** `pendingAccReg` was only set in the OR-with-flag path, not after plain `ADD A, r`.
So `materializePendingAcc` never fired before the i+1 computation.

**Fix:** Call `materializePendingAcc(inst)` at the start of `genBinOp` for 8-bit ops (skipping
the INC/DEC peephole which doesn't touch A), and set `pendingAccReg = inst.Dst` at every
8-bit code path that leaves the result in A.

---

### Fix 5 — `buildBlockCopies` uses `g.loc()` not `g.ar.Loc()` (`z80codegen.go`)

**Symptom:** After Fix 4, acc was saved to E (`LD E, A` via materializePendingAcc) but the
back-jump parallel copy still saw acc as being in A (canonical). It emitted a no-op copy
(A→A) instead of the correct E→A restore.

**Root cause:** `buildBlockCopies` used `g.ar.Loc(arg).Name` (allocator canonical, ignores
`physOverride`) to determine the source location. When acc had `physOverride["E"]`, the canonical
was still "A" — so the copy appeared as A→A (no-op) instead of E→A.

**Fix:** Use `g.loc(arg)` (which respects `physOverride`) for the source in `buildBlockCopies`:
```go
src := g.loc(arg)  // was: g.ar.Loc(arg).Name
```

With this fix, the parallel copy correctly emits:
```asm
LD D, A        ; save i+1 (in A)
LD A, E        ; restore acc from scratch
LD IXH, D     ; i+1 to IXH
```

---

## Generated Assembly (sum_array, BUG-003 loop)

```asm
; fun sum_array(ptr: ptr = HL, n: u8 = C) -> u8 = A ; clobbers: BC, D, DE, F, IXH, IXL
sum_array:
    LD A, 0
    LD D, 0
    LD IXH, D          ; i = 0
    LD IXL, C          ; n saved to IXL (Fix 2: coalescer no longer puts n in C)
    LD B, H            ; ptr_hi → B
    LD C, L            ; ptr_lo → C (safe: n is now in IXL, not C)
.sum_array_loop_head1:
    LD E, A            ; save acc to E (Fix 3: block param A → scratch E)
    LD A, IXH          ; A = i
    CP IXL             ; compare i with n (correct: IXL = n)
    LD A, E            ; restore acc (Fix 3: restore before terminal)
    JRS NC, .sum_array_loop_exit3
.sum_array_loop_body2:
    LD E, IXH          ; E = i (offset for ptr_add)
    LD D, 0            ; zero-extend
    PUSH BC            ; save ptr (Fix 1: PUSH/POP preserves ptr across ADD HL)
    POP HL             ; HL = ptr
    ADD HL, DE         ; HL = ptr + i
    LD D, (HL)         ; D = arr[i]  (no A clobber via physOverride)
    ADD A, D           ; acc += arr[i]
    LD D, 1            ;
    LD E, A            ; save acc (Fix 4: pendingAccReg after ADD)
    LD A, IXH          ; A = i (for i+1)
    ADD A, 1           ; A = i+1
    LD D, A            ;
    LD A, E            ; restore acc (Fix 5: buildBlockCopies uses g.loc → E→A)
    LD IXH, D          ; IXH = i+1
    JRS .sum_array_loop_head1
.sum_array_loop_exit3:
    RET
```

---

## E2E Test Results

```
TestPtrWhile_E2E_SumArray — array [10,20,30,40,50,...]:
  sum_array(ptr, 0) =   0 ✓ (empty sum)
  sum_array(ptr, 1) =  10 ✓ (arr[0])
  sum_array(ptr, 2) =  30 ✓ (10+20)
  sum_array(ptr, 4) = 100 ✓ (10+20+30+40)
  sum_array(ptr, 5) = 150 ✓ (10+20+30+40+50)
```

New test file: `pkg/nanz/ptr_while_e2e_test.go`.

---

## Showcase & Test Suite

```
23/23 showcase PASS (no regressions)
26/26 Go test packages PASS
```

---

## Root Cause Summary

BUG-003 was not a single bug but a cluster of five interacting correctness issues:

| # | Component | What was wrong |
|---|-----------|----------------|
| 1 | OpPtrAdd codegen | Emitted `ADD DE, BC` — invalid Z80 instruction |
| 2 | coalesceAllocResult | Safety check missed pair/component aliases (C ↔ BC) |
| 3 | genBlock / genTerm | Block param in A clobbered by compare; no restore before terminal |
| 4 | genBinOp | pendingAccReg not set after plain ADD; acc clobbered by next ALU op |
| 5 | buildBlockCopies | Used canonical location instead of physOverride for parallel copy source |

Fixes 2–5 are general correctness improvements that benefit all code containing loops with
multiple live values, not just the specific `ptr[i]` pattern.

---

## Next Steps

1. **Phase 6e** — PBQP edge costs (~150 LOC, ADR-0017): correlated allocation for LUT BC★,
   mul16 rhs→DE, DJNZ counter→B
2. **lospre** (~500 LOC, Krause 2020): linear-time redundancy elimination for loop-hoistable
   expressions and repeated address computations
3. **Module system** — HIR merge pass (one `.nanz` file per module, `$` separator)
