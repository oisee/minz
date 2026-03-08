# ADR-0011: MIR2 Codegen — DSE Pass and Shadow Register Guard

**Date:** 2026-03-07
**Status:** Accepted

## Context

The MIR2 register allocator + Z80 codegen pipeline is working for arithmetic,
branches, and loops (fib, clamp, abs_diff, sum_range, mul8 all verified).
Two codegen quality issues were identified:

### Problem 1 — Dead Stores from OpConst

When the CP-immediate peephole fires (e.g. `CP 0` instead of `LD A, x; CP x`),
the `OpConst` that materialised the constant register is no longer needed, but
the codegen still emits it:

```asm
; sum_range inner loop — before DSE:
    LD C, 0      ; ← dead: only used as RHS of CP, peephole made it unnecessary
    LD A, B
    CP 0
    JP NZ, loop
    LD C, 1      ; ← dead: only used as DEC-by-1, INC/DEC peephole eliminated it
    DEC B
```

### Problem 2 — Silent Shadow Register Allocation

`LocShadow` had cost 10 (< stack 21, < mem 28) for `ClassGeneral`, `ClassAcc`,
and `ClassCounter`. The allocator would silently assign vregs to `B'`, `C'`,
`D'`, `E'`, `A'` under register pressure — but the codegen emits zero `EXX` /
`EX AF,AF'` instructions. Result: broken code with no compile-time error.

## Decision

### Immediate (this ADR)

1. **Shadow guard**: Set `LocShadow = InfCost` for `ClassGeneral`, `ClassAcc`,
   `ClassCounter` in `z80cost.go`. Shadow allocation is now impossible for
   standard classes until EXX codegen is implemented. `ClassShadow` and
   `ClassAccShadow` are unaffected (they are the explicit shadow classes).

2. **DSE pass in codegen**: Before emitting an `OpConst` instruction, check
   whether the vreg is *only* consumed by instructions where a constant-folding
   peephole would already have fired (CP imm8, INC/DEC). If so, skip emission.
   Implementation: single pre-pass over the block, O(n) per block.

### Future (not in this ADR)

- **EXX region detection** (ADR-0012): SESE-based analysis to find regions
  where B'/C'/D'/E' are safe to use without EXX within a block. Re-enable
  shadow locs for `ClassGeneral` with proper EXX emission.
- **minz → MIR2 lowering**: after MIR2 feature coverage matches the old IR
  (structs, arrays, globals, multi-file calls). Estimated: Phase 3 of MIR2
  roadmap (see `docs/MIR2_Roadmap.md`).

## Consequences

- Shadow allocation bugs are prevented at the cost table level — hard failure
  rather than silent miscompilation.
- DSE eliminates ~1–3 dead `LD reg, imm8` per function in typical arithmetic
  code, saving 7–10 T-states per occurrence.
- `TestZ80CostShadowCheaperThanMemory` renamed to
  `TestZ80CostShadowDisabledForStandardClasses` to document intent.
- `TestZ80CostHierarchy` updated: shadow tier removed from ClassGeneral chain.
