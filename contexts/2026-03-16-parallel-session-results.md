# Context: Parallel Session Results — 2026-03-16

**Date:** 2026-03-16
**Sessions:** 3+ parallel (macbookairm2, this machine, possibly others)
**Branch:** master

---

## What landed today (all sessions combined)

### Session A (this machine) — Allocator + HIR + BUG-008
1. **PBQP R1 back-assignment fix** — saved ALL neighbors before removeNode, checks all on back-assign. 9+ live u8 regs → IXH/IXL/IYH overflow, zero $F0xx spills. Tetris compiles clean.
2. **GEP builder** — `hir.GEP(base, steps...)` with offset folding. `a.inner.y` → `LD A, (obj + 2)`.
3. **Embedded struct support** — FieldExpr/IndexExpr with StructTy return address (no Load). Array-of-struct `pts[i].y` works.
4. **Backward goto** — loop-via-goto with variable threading through block params. C89 compliance.
5. **BUG-008 CLOSED** — all 5 DD prefix conflict categories fixed. Arena_alloc generates valid Z80. Struct methods unblocked.
6. **Z80 SHL fix** — `ADD DE,DE` → `SLA E; RL D` (only ADD HL,rr is valid).

### Session B (macbookairm2) — C89/C99/C11 + Function Pointers
1. **Function pointers** — C89 `(*fp)(args)` → HIR CallIndirectExpr → MIR2 OpCallIndirect → `CALL __call_hl`. Full pipeline.
2. **C99** — inline, _Bool normalization, restrict, designated init, compound literals
3. **C11** — _Static_assert
4. **Static local heap fix** — 1-byte init for 16-bit int caused overlap
5. **C89 corpus: 241/241** (was 222)

### Session C — ABAP Frontend (!)
1. **ABAP frontend** — 8 programs (hello, fibonacci, fizzbuzz, guessing, bubblesort, forms, oop, sysinfo)
2. **7th frontend** in the multi-frontend family

---

## Current bug status

| Bug | Status | Notes |
|-----|--------|-------|
| BUG-001 GCD bloat | 🟡 | PBQP affinity, not blocking |
| BUG-006 Zero-size struct | 🟡 | Link error, rare |
| BUG-007 Adapter LD | 🟡 | Spurious when same PFCCO |
| BUG-008 Arena IXL,(IX+d) | ✅ **FIXED** | All DD conflicts gone |

---

## Next: Function Pointer ABI Design

**Problem:** Function pointers need a FIXED calling convention. Current PFCCO
(Per-Function Calling Convention Optimization) assigns registers per-function.
But indirect calls (`CALL __call_hl`) don't know the callee → must agree on ABI.

**See:** `docs/adr/0029-function-pointer-abi.md` (to be written)

**Key questions:**
1. Which functions get standard ABI? (address-taken detection vs explicit annotation)
2. What is the standard ABI? (A/C/B for u8 params, HL/DE for u16, return in A/HL)
3. How do Nanz/Lizp/Pascal express function pointer types?
4. Can PFCCO and standard ABI coexist? (yes — direct calls use PFCCO, indirect use standard)
