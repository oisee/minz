# Report 046 — Nanz Week 1: Achievements, RCA of Z80 Codegen Gaps, Phase 6 Plan

**Date:** 2026-03-09
**Status:** Active development

---

## 1. What Was Shipped This Session

### 1.1 Nanz Week 1 Features (pkg/nanz, pkg/hir)

| Feature | Status | Notes |
|---------|--------|-------|
| Struct field offsets | ✅ | `makeFieldExpr` computes byte offsets at parse time from struct layout |
| Struct methods `fun Dog.speak(self: Dog)` | ✅ | Stored as `Dog_speak`, registered in `methodTable["Dog"]["speak"]` |
| UFCS type-aware dispatch `a.speak()` | ✅ | `exprTy(base)` → struct type → `methodTable` lookup → mangled name |
| Operator overloading `fun +(a: Vec2, b: Vec2)` | ✅ | `opTable`, dispatched in `parseBinary` when lhs is struct type |
| Go-style interfaces `interface Animal { speak }` | ✅ | Parsed, stored in `hir.Module.Interfaces`; zero codegen impact |
| Struct-typed params `fun f(a: Dog)` → ClassPointer | ✅ | Fixed `classForParam(*StructTy)` → ClassPointer |
| ADR-0016: error propagation `-> T ! E` / `?` | ✅ | Design documented; implementation deferred to Week 2 |

### 1.2 MIR2→QBE Backend Fixes (pkg/mir2qbe)

| Fix | Impact |
|-----|--------|
| `OpMove`: use `g.regTy[dst]` for destination type | Pointer-promoted regs (`l`) no longer get narrowed to `w` |
| `buildCallExpr`: use `g.regTy[arg]` per arg | Pointer args emitted as `l` at call sites, matching callee declarations |

### 1.3 E2E Tests Added (pkg/mir2qbe/e2e_test.go)

- `TestE2E_Nanz_StructFields` — global Color{r,g,b:u8}, field offsets 0/1/2 verified ✓
- `TestE2E_Nanz_UFCS` — `acc_g.add(a)` → `Acc_add(addr_of_acc_g, a)`, struct pointer as `l` ✓
- `TestE2E_Nanz_Interface_ZeroCost` — `g_animal.speak()` → `call $Animal_speak(l %r)`, direct CALL ✓

### 1.4 Test Suite

All **23/23** Go test packages pass. 1335/1335 FUSE Z80 emulator tests pass.

---

## 2. Interface Dispatch: What Zero-Cost Actually Means

```nanz
interface Speaker { speak }          // compile-time contract — zero bytes emitted

fun Dog.speak(self: Dog) -> u8  { self[0] = 1; return self[0] }
fun Cat.speak(self: Cat) -> u8  { self[0] = 2; return self[0] }
fun Bird.speak(self: Bird) -> u8{ self[0] = 3; return self[0] }

fun all_speak() -> u8 {
    dog.speak(); cat.speak(); bird.speak()
    return dog.sound + cat.sound + bird.sound
}
```

**QBE IL (correct, verified running):**
```
call $Dog_speak(l %r26)    ← direct CALL, struct address passed
call $Cat_speak(l %r29)    ← direct CALL
call $Bird_speak(l %r32)   ← direct CALL
```

**Ideal Z80 (what the codegen SHOULD produce):**
```asm
Dog_speak:           ; HL = &dog (ClassPointer param)
    LD (HL), 1       ; dog.sound = 1   — 10T
    LD A, (HL)       ; return value    —  7T
    RET              ;                 — 10T  → 27T total

all_speak:
    LD HL, dog
    CALL Dog_speak   ; ← direct CALL, no vtable
    LD HL, cat
    CALL Cat_speak   ; ← direct CALL
    LD HL, bird
    CALL Bird_speak  ; ← direct CALL
    ...
```

Go would emit indirect calls through an interface header (2 pointer loads + indirect CALL ≈ 45–60T).
Nanz emits: `CALL Dog_speak` — one instruction, 17T.

**Actual Z80 output today** has two classes of bugs detailed in §3.

---

## 3. Root Cause Analysis: Z80 Codegen Gaps

### Bug A — Invalid 8-bit moves of 16-bit values (`LD A, HL` / `LD HL, A`)

**Symptom** (in `all_speak`):
```asm
LD HL, dog    ; OpAddrOf → HL (16-bit ClassPointer) ✓
LD A, HL      ; INVALID — A is 8-bit, HL is 16-bit ✗
LD HL, A      ; INVALID ✗
CALL Dog_speak
```

**Cause chain:**

1. `parsePrimary` in parse.go (line 1618) always creates `VarRefExpr{Name: "dog", Ty: TyU8}` —
   **`VarRefExpr.Ty` is hardcoded to `TyU8` regardless of the variable's actual type.**

2. In `lowerCallExpr`, arg type is queried via `a.ExprTy()` → `TyU8` (not `*StructTy`, not `TyU16`).

3. `classForParam(TyU8, pos=0)` → `ClassAcc` (A, 8-bit).

4. Since `classForExpr(TyU8)` = `ClassGeneral` ≠ `ClassAcc`, a coercion `OpMove(addr_reg, TyU8, ClassAcc)` is emitted.

5. `addr_reg` was produced by `OpAddrOf("dog")` → it is 16-bit (ClassPointer, HL).

6. `emitMov("A", "HL", widthBits=8)` → `LD A, HL` — Z80 doesn't have this instruction.

**Fix required:**
`parsePrimary` must set `VarRefExpr.Ty` from `varTypes` / `globalTypes` at parse time, not hardcode `TyU8`.
Alternatively: `lowerCallExpr` must use the MIR2-level reg type (from `bld.RegTy(r)`) rather than `a.ExprTy()` when computing the coercion target class.

The `exprTy()` helper already returns the correct struct type for UFCS dispatch — the same logic must be applied to `VarRefExpr.Ty` at construction time.

---

### Bug B — ClassPointer self-param spilled to `$F0xx` (`PUSH mem` / `POP HL`)

**Symptom** (in `Dog_speak` body):
```asm
Dog_speak:
    LD C, 1           ; constant 1
    LD D, 0           ; index 0
    PUSH mem          ; reload self from $F0xx ← SPILLED
    POP HL
    LD DE, 0          ; ...
    ADD HL, DE        ; HL = self + 0 (redundant when offset is 0)
    LD (HL), C        ; self[0] = 1 ✓
```

**Cause chain:**

1. `Dog_speak` has one ClassPointer param → allocator assigns it to HL (only ClassPointer location).

2. `self[0]` lowers as `OpPtrAdd(self_reg, Ext(Const(0)))`. `OpPtrAdd` produces a NEW ClassPointer reg (needs HL) that is live simultaneously with `self_reg` (also ClassPointer, also needs HL).

3. Two simultaneously-live ClassPointer regs → classic register pressure → allocator spills one to `LocMem($F0xx)` → `PUSH mem` / `POP HL` reload.

4. Additionally, `PropagateConstants` + `FoldConstants` do NOT fold `PtrAdd(x, 0) → x`, so the zero-offset add is never eliminated; both regs stay live.

**Two independent sub-fixes required:**

**B1 (optimizer):** Teach `FoldConstants` that `OpPtrAdd(x, 0) = x` (zero-offset fold). This eliminates the second ClassPointer reg entirely.

**B2 (allocator — Phase 6a):** Even after B1, any function that creates two pointer values simultaneously (e.g. two struct fields) will face this pressure. Phase 6a must model spill costs accurately so the allocator can choose ClassIX/ClassIY (tier 1, +8T/insn) over `$F0xx` spill (tier 2, +21T/access). ClassIX/ClassIY are already defined in `regs.go` (lines 66–68) but not wired into codegen.

---

### Bug C — `isSimpleReg` rejects 16-bit registers in copy-tracking

Subsidiary to Bug A: `emitMov` calls `setCopy` only when `isSimpleReg(dst) && isSimpleReg(src)`. HL/DE/BC are not "simple regs" in the current definition, so copy tracking misses aliases and emits redundant reloads.

---

## 4. Phase 6 Plan

### Phase 6a — Spill Cost Model + ClassIX/ClassIY as Overflow Pool

**Goal:** When HL is fully occupied, the allocator picks IX/IY (tier 1, cost +8T/insn) rather than `$F0xx` (tier 2, cost ~21T/access). IX/IY are only chosen when PBQP proves it is cheaper than alternatives — never hardcoded.

**Tasks:**
1. Wire `ClassIX`/`ClassIY` into Z80 codegen: `LD IX, nn` (14T), `LD (IX+d), r` (19T), `LD r, (IX+d)` (19T)
2. Add `LocIX`/`LocIY` to physical location table with correct costs in `z80cost.go`
3. Add undocumented `LD IXh, r` / `LD IXl, r` to MZA (already has the opcodes — DD prefix table)
4. Verify that existing PBQP cost model selects IX only under genuine pressure (HL + DE + BC all live)

**Acceptance:** `Dog_speak(self: Dog)` with 2 simultaneous pointer regs → one gets IX, no `$F0xx` spill.

### Phase 6a′ — Immediate Pre-req: Fix `VarRefExpr.Ty` and `PtrAdd(x, 0)` fold

Two targeted fixes that unblock the visible bugs without Phase 6 machinery:

1. **`parsePrimary`:** Set `VarRefExpr.Ty` from `varTypes`/`globalTypes` at construction time (same data that `exprTy()` consults). Cost: ~5 lines.

2. **`FoldConstants`:** Add `OpPtrAdd(x, Const(0)) → x` fold case. Cost: ~3 lines.

These are prerequisites for Phase 6a to be measurable.

### Phase 6b — Graph Coloring with Class Constraints (Full PBQP)

Current allocator: greedy colouring ordered by interference degree. Works well for low-pressure functions; breaks under high pressure because it doesn't look ahead.

**Goal:** Replace greedy pass with full Partitioned Boolean Quadratic Program (PBQP) solver:
- Each virtual reg = a node; each pair of interfering regs = an edge constraint
- Node cost = sum of `z80cost.Cost(cls, loc)` for each use/def in the function
- Edge cost = infinity when two interfering regs are assigned the same physical location
- Solve to global minimum — allows IX/IY/shadow-reg choices to interact correctly

**Tasks:**
1. Build interference graph (already exists: `BuildInterferenceGraph` in `alloc.go`)
2. Build PBQP cost vectors per virtual reg (per-use costs from liveness + z80cost)
3. Implement PBQP solver (reductive approach: eliminate degree-1 nodes first; for RN nodes use heuristic or exhaustive for small arity)
4. Replace `Allocate()` with PBQP solve + colour assignment

### Phase 6c — Copy Coalescing Across Block Boundaries

**Goal:** Eliminate trampolines inserted for BrIf/TermJmp parallel copies. When the source and destination of a copy have compatible classes and no interference, merge them into the same physical location.

**Tasks:**
1. After initial colouring, identify `OpMove` pairs where both endpoints can use the same physical location
2. Re-colour to unify; verify no new interferences introduced
3. Run a final dead-copy elimination pass

### Phase 6d — `ptr[i]` in Loops: HL Conflict Fix

**Root cause:** Loop body uses HL for both the base pointer (ClassPointer) and intermediate 16-bit arithmetic. With Phase 6a (ClassIX), the PBQP solver will naturally place the pointer in IX when HL is occupied by arithmetic.

**This is not a separate fix** — it falls out of Phase 6a correctly. Tracking here for completeness.

---

## 5. Priority Order

```
NOW (pre-req, small):
  Fix VarRefExpr.Ty in parsePrimary        — eliminates Bug A (LD A, HL)
  Fix FoldConstants: PtrAdd(x, 0) → x     — eliminates half of Bug B

Phase 6a (medium, 1–2 sessions):
  Wire ClassIX/ClassIY into codegen + cost
  Verify PBQP selects IX only under pressure

Phase 6b (large, multiple sessions):
  Full PBQP solver replacing greedy allocator

Phase 6c (medium, after 6b):
  Copy coalescing → eliminate trampolines

PARALLEL (already unblocked):
  Error propagation -> T ! E / ? (ADR-0016 Week 2)
  Nanz hover/goto-def in LSP (symbol table)
  MinZ → HIR lowering (retire MIR1)
```

---

## 6. Metrics

| Metric | Value |
|--------|-------|
| Go test packages | 23/23 ✅ |
| FUSE Z80 emulator | 1335/1335 ✅ |
| Nanz features (Week 1) | struct methods, UFCS, op overload, interfaces, struct params |
| QBE E2E tests | 7 tests, all pass (PLM×2, Nanz×5) |
| Z80 codegen: interface dispatch | Correct structure (direct CALLs) — param-passing bugs block assembly |
| Z80 codegen: Dog_speak ideal | 27T — actual broken (spill overhead ~60T extra) |
| Phase 6 items | 0/4 done |

---

## 7. AI Consultation Log

- **2026-03-09, this session:** IX/IY for ptr[i] loops — conclusion: IX/IY only when PBQP proves cheaper than alternatives (never hardcoded). (IX+d) = 19T vs (HL) = 7T — not a win for sequential access. Real fix is Phase 6a PBQP.
- **2026-03-09:** Interface dispatch design — zero-cost via compile-time monomorphisation; Go uses indirect dispatch through interface header. Nanz: direct CALL always, 17T. Go equivalent: ~55T.
