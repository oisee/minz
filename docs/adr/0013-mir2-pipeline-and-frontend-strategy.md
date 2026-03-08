# ADR-0013: MIR2 Pipeline and Frontend Strategy

**Date**: 2026-03-08  
**Status**: Proposed  
**Context**: MIR2 backend is proven (53 tests). Decision: how to connect MinZ frontend?

---

## Problem

The semantic analyzer (`pkg/semantic/analyzer.go`, ~12K lines) directly produces
`ir.Module` (MIR1). MIR1 has 118 opcodes, many Z80-specific (OpDJNZ, 14 SMC ops,
RegisterHint). MIR1 optimizations (13 passes) were written during pipeline turbulence
and may be partially incorrect — feature tests show 9/11 FAIL on advanced features.

Three strategies:

### Option A: MIR1 → MIR2 lowering
Keep semantic as-is. Write a lowering pass.

**Pro**: Reuses existing 13 optimization passes, iterator fusion, CTIE, SMC.  
**Con**: Propagates MIR1 bugs into MIR2. Z80-specific opcodes in MIR1 must be
translated (OpDJNZ→loop+ClassCounter, etc.). If MIR1 is wrong, lowering inherits it.

### Option B: MinZ AST → MIR2 directly (rewrite semantic)
Replace `Analyzer` with a new pass that emits MIR2.

**Pro**: Clean slate, no inherited bugs.  
**Con**: ~12K lines of semantic work (type checking, UFCS, overloading, lambdas,
iterators, SMC) must be re-implemented. High risk, high effort.

### Option C: AST → typed-HIR → MIR2  ← RECOMMENDED
Factor semantic into two layers:

```
AST  →  Semantic Analysis  →  TypedAST (HIR)  →  MIR2 Lowering  →  MIR2
         (type checking,         (structured          (linearize,
          name resolution,        control flow,        CFG, SSA,
          UFCS, overloads,        named vars,          block params)
          lambdas)                types resolved)
```

**HIR properties**:
- Structured control flow (if/while/for — not basic blocks)
- Named variables (not SSA virtual regs)
- Types fully resolved
- No register classes yet (assigned during HIR→MIR2 lowering)
- No Z80-specific operations

**HIR→MIR2 lowering assigns classes by rules**:
- param[0] → ClassAcc (A)
- param[ptr] → ClassPointer (HL)
- loop counter → ClassCounter (B, for DJNZ)
- value live across call → ClassGeneral (C/D/E via Move spill)

**Pro**: Clean separation of concerns. Semantic analysis is reusable for other backends.
HIR is simple to validate. Incremental: can start HIR with a subset of MinZ features.  
**Con**: Still requires refactoring semantic (~6K LOC to decouple from ir.Module).

---

## Decision

**Chosen: Option C (AST → TypedHIR → MIR2)**, with pragmatic incremental path:

### Phase 4a (parallel, non-breaking):
Start with simple MinZ programs. New `pkg/hir/` package:
- HIR node types: `HFunc`, `HBlock`, `HStmt`, `HExpr` (typed)
- New `Analyzer2` that emits HIR (subset of MinZ: no iterators, no SMC initially)
- `HIR → MIR2` lowering: expression linearizer + CFG builder + SSA inserter

### Phase 4b:
Expand HIR to cover full MinZ. Migrate optimizer passes to MIR2 level.

### Phase 4c:
Deprecate MIR1 path. Remove `pkg/ir/` when all features covered by HIR path.

### Rationale for NOT choosing Option A:
MIR1 feature tests (9/11 FAIL) indicate the existing pipeline has significant correctness
issues. Lowering from broken MIR1 to MIR2 would require debugging both layers simultaneously.
Better to build a clean HIR path and validate incrementally.

---

## MIR2 Optimization Roadmap (to implement at MIR2 level, not in lowering)

| Pass | Priority | Effect |
|------|----------|--------|
| DSE (dead store elim) | HIGH | removes `LD C, 0` dead consts after CP-imm8 |
| Copy coalescing | HIGH | merges non-interfering move src/dst → eliminates `LD D,A; LD A,D` |
| LICM | MEDIUM | hoists loop-invariant `addr_of` / consts before loop |
| Strength reduction | MEDIUM | `mul×2→add`, `mul×4→shl 2`, `div÷2→shr` |
| Pressure-aware scheduling | MEDIUM | reorders insns to minimize simultaneously-live regs |
| Pattern collapse (LDIR) | LOW | memcpy/fill loop → LDIR / LDI×N |
| Live range splitting | LOW | splits long ranges at pressure peaks |

---

## SMC + Currying (related decision)

With SMC, universal currying requires no explicit curried function variants.
A `@curry(fn, arg)` intrinsic can generate a runtime stub (5-7 bytes in RAM):

```asm
curry_stub:
    LD A, [patched_val]   ; PatchSlot — value written at curry() call time
    JP target_fn
```

Calling convention: stub sets up the curried args, caller provides remaining args
in ClassGeneral/ClassCounter as normal. Zero overhead on the hot path.
MIR2 ops needed: OpPatchSlot, OpCallIndirect (both already exist).

---

## Backend Portability

MIR2 RegClass is ISA-neutral by design. Adding a backend:
- Write `<target>codegen.go` + `<target>cost.go`
- Map RegClass to ISA physical roles:

| RegClass | Z80 | 6502 | 68000 | eZ80 |
|----------|-----|------|-------|------|
| ClassAcc | A | A | D0 | A |
| ClassCounter | B (DJNZ) | X/Y | D7 (DBF) | B |
| ClassPointer | HL | ZP+1:ZP | A0 | HL (24-bit) |
| ClassIndex | DE | X or Y | A1 | DE |
| ClassGeneral | C/D/E | ZP scratch | D1-D6 | C/D/E |

ClassCounter (DJNZ) has no direct equivalent on 6502/68000 — lowers to
`DEC X / BNE` (6502) or `DBF Dn, label` (68000). Correct, not zero-cost.
