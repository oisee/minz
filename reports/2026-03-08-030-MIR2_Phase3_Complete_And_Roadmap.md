# MIR2 Phase 3 Complete + Architectural Roadmap

**Date**: 2026-03-08  
**Sprint**: #030  
**Status**: Phase 3 complete (53/53 tests), roadmap written

---

## Phase 3 Completed This Session

### Features
- **OpPtrBump**: iterator-style pointer advance (semantically distinct from OpField)
- **DEC/INC × N peephole**: `sub r, N where N≤3 → DEC r × N` (12T vs 15T at N=3)
- **AND A peephole**: `CP 0 → AND A` (1B/4T vs 2B/7T)
- **String pool emission**: `DB bytes, 0` with `_mir2_str_N` labels
- **Multi-function modules**: `OpCall` first tested in MIR2, `double`/`quad` verified
- **Live-across-call spill pattern**: `double_sum` demonstrates canonical `LD D,A / LD A,C` spill
- **strlen**: NUL-terminated string walk via OpPtrBump + AND A

### Bug Fixes (critical)
1. **Module-wide reg numbering** (`Module.nextReg` shared across all `AddFunc` calls)  
   Root cause of all multi-function codegen corruption: both funcs started at reg=1,
   AllocResult keys collided, wrong physical regs assigned.

2. **TermRet redundant move elimination** (`holdsValue(retLoc, src)` check)  
   After `ADD A,A / LD C,A`, A already holds C's value — TermRet was emitting
   `LD A,C` redundantly. Now eliminated via copy-tracking.

### Test Count
- Session start: 51 tests  
- Session end: 53 tests  
- All pass, 19/20 packages green (z80testing: pre-existing TSMC failures)

---

## Generated Code Quality (double_sum example)

**MIR2:**
```
fun @double(%r1: u8 [acc]) -> u8 [acc]
  block @entry:
    %r2 = add %r1, %r1 : u8 [acc]
    ret %r2

fun @double_sum(%r3: u8 [acc], %r4: u8 [general]) -> u8 [acc]
  block @entry:
    %r5 = call @double(%r3) : u8 [acc]
    %r6 = move %r5 : u8 [general]     ; explicit spill
    %r7 = move %r4 : u8 [acc]         ; reload b into A
    %r8 = call @double(%r7) : u8 [acc]
    %r9 = add %r8, %r6 : u8 [acc]
    ret %r9
```

**Z80:**
```asm
double:
    ADD A, A
    RET

double_sum:
    CALL double    ; A = 2a;  C = b (ClassGeneral, survives call)
    LD D, A        ; spill 2a → D
    LD A, C        ; b → A for second call
    CALL double    ; A = 2b
    ADD A, D       ; A = 2a + 2b
    RET
```

---

## Architectural Decisions Made (→ ADR-0013)

### Pipeline: AST → TypedHIR → MIR2 (chosen over MIR1→MIR2)

MIR1 is suspect: semantic analyzer directly builds `ir.Module`, feature tests 9/11 FAIL,
pipeline was unstable during MIR1 development. Lowering from broken MIR1 would
propagate bugs. Decision: new `pkg/hir/` package (TypedAST: structured CF, named vars,
types resolved) → MIR2 lowering assigns RegClass from rules.

### SMC + Universal Currying

Any function curriable via 5-7 byte runtime stub: `LD A, [patched]; JP fn`.
`@curry(fn, val)` allocates stub, patches value. Zero overhead on hot path.
MIR2 already has `OpPatchSlot` + `OpCallIndirect` — just needs the intrinsic.

### Backend Portability

RegClass is ISA-neutral. 6502/68000/eZ80 backends: new `<target>codegen.go` + cost table.
ClassCounter (DJNZ) lowers to `DEC X/BNE` (6502) or `DBF Dn` (68000) — correct, not zero-cost.

---

## Phase Status

### Phase 3 ✅ DONE (53 tests)
- Core Z80 patterns: load/store, ALU, branches, loops, globals, structs
- Multi-function: OpCall, live-across-call spill pattern
- String pool, OpPtrBump, AND A / INC×N peepholes

### Phase 3 Deferred (parked, not blocking)
- SoA256 `INC L` path for OpPtrBump  
- @print intrinsic
- OpForRange → DJNZ

### Phase 4: MIR2 Optimization Passes
Priority order:
1. **DSE** — kills `LD C,0` dead stores from CP-imm8 peephole
2. **Copy coalescing** — eliminates non-interfering `move` instructions entirely
3. **LICM** — hoist `addr_of`/const out of loops
4. **Pressure-aware scheduling** — reorder insns to shrink live ranges
5. **LDIR collapse** — memcpy/fill loops → LDIR or LDI×N

### Phase 5: HIR Package + Frontend Connection
- `pkg/hir/` node types (HFunc/HBlock/HStmt/HExpr, typed)
- New `Analyzer2` emitting HIR for a subset of MinZ (start small)
- HIR→MIR2 lowering: linearize expressions, CFG construction, SSA insertion
- RegClass assignment rules: position-based + liveness-based

### Phase 6: Full MinZ Coverage via HIR Path
- Iterators, lambdas, UFCS monomorphization
- SMC/currying via OpPatchSlot
- Parallel to MIR1 path until parity reached; then deprecate MIR1

---

## Register Pressure: Independent Chunk Ideas

For future HIR→MIR2 lowering, register pressure reduction techniques:

1. **Expression reordering**: evaluate sub-expressions that kill registers first
2. **Live range splitting**: if peak pressure > N physical regs, split at the peak
3. **LICM at HIR level**: `addr_of` / constant loads hoisted before entering loops
4. **Block sinking**: push computation into the branch that needs it (avoid holding values across branches that don't use them)
5. **Shrink wrapping**: alloca/prologue only on paths that actually use the stack
