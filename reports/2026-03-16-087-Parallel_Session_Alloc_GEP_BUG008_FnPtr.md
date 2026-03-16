# Report #087 — Parallel Session: Allocator, GEP, BUG-008, Function Pointers

**Date:** 2026-03-16
**Machine:** parallel session (this machine)
**Commits:** 4 (1ee08de1, 3ffd5b6b, ba2b04b2, 47072bdd)

---

## Summary

Major session covering allocator correctness, HIR infrastructure, Z80 codegen
safety, and function pointer support across 3 frontends.

## Changes

### 1. PBQP R1 Back-Assignment Fix
**Files:** `pkg/mir2/pbqp.go`, `pkg/mir2/alloc.go`, `pkg/mir2/pbqp_test.go`

R1 reduction in PBQP removes a degree-1 node from the interference graph. On
back-assignment, only the single R1 neighbor was checked — other already-assigned
neighbors were ignored, allowing silent register aliasing.

**Fix:** Save snapshot of ALL neighbors before `removeNode`. Back-assignment
checks blocked locations from every neighbor (including sub-register aliases).

**Result:** 9+ live u8 regs correctly allocated to A,B,C,D,E,H,L + IXH,IXL,IYH.
Tetris compiles with zero $F0xx memory spills.

### 2. GEP Builder + Embedded Struct Support
**Files:** `pkg/hir/hir.go`, `pkg/hir/lower.go`, `pkg/hir/lower_test.go`

- `hir.GEP(base, steps...)` convenience builder with automatic fold of consecutive
  constant-offset fields: `a.inner.y` → single `LD A, (obj + 2)`.
- FieldExpr/IndexExpr with StructTy now return address (no Load), enabling nested
  struct and array-of-struct access chains.
- Backward goto via variable threading through block parameters.

### 3. BUG-008 Closed — DD Prefix Conflicts
**Files:** `pkg/mir2/z80codegen.go`

Five categories of invalid Z80 instructions eliminated:

| Pattern | Problem | Fix |
|---------|---------|-----|
| `LD IXL,(IX+d)` | DD can't remap both | Route through A |
| `LD H,IXH` | Encodes as NOP | PUSH/POP for IX↔HL |
| `LD (IX+d),H` | H substituted to IXH | Route through A |
| `ADD DE,DE` | Only ADD HL,rr valid | SLA E; RL D |
| `obj__offN` | Label undefined | Use `obj + N` |

Arena_alloc and all struct methods now generate valid Z80 assembly.

### 4. Function Pointer ABI (ADR-0029)
**Files:** `docs/adr/0029-function-pointer-abi.md`, `pkg/mir2/z80codegen.go`

Standard callable ABI: param0=A/HL, param1=C/DE, param2=B/BC. OpCallIndirect
now builds synthetic Contract and uses emitCallArgs for proper arg setup.

### 5. Lanz/Lizp Function Pointers
**Files:** `pkg/lanz/lower.go`, `pkg/lizp/lizp.go`, tests

- Lanz: `(funcall fn-expr args...)` → CallIndirectExpr
- Lizp: `#'name` → `(addr name)` (Common Lisp style)
- Lanz: `compileCall` now looks up callee return type (was always TyVoid)
- Verified: `apply(#'double, 5) == 10` via MIR2 VM

## Test Results

| Package | Status |
|---------|--------|
| pkg/hir | ✅ PASS |
| pkg/mir2 | ✅ PASS (2 pre-existing) |
| pkg/c89 | ✅ PASS |
| pkg/lanz | ✅ PASS |
| pkg/lizp | ✅ PASS |

## Function Pointer Status Across Frontends

| Frontend | Status | Syntax |
|----------|--------|--------|
| C89 | ✅ | `int (*fp)(int) = add; fp(3)` |
| Lanz | ✅ | `(funcall (addr double) 5)` |
| Lizp | ✅ | `(funcall #'double 5)` |
| Nanz | 📋 | `type T = fun(u8) -> u8` (parser ready) |
| Pascal | 📋 | `type T = function(x: byte): byte` |
| PL/M | 📋 | `DECLARE F ADDRESS` |
| ABAP | 📋 | `PERFORM dynamic` |
