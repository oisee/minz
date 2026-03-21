# Session 2026-03-21: Z80 Codegen — 99.6% Error Reduction

## Stats
- **60 commits**, ~12 hours
- **~3000 → 12 errors** (−99.6%)
- **45 → 3 files** with errors (−93%)
- **ff.c (FatFS 7K LOC) = ZERO errors**
- **97% clean rate** (104/107 compilable files)

## Remaining 12 errors

### arena (8) — PBQP ClassFlag vs ClassAcc
Bool vreg gets ClassFlag (from CMP) → LocFlag (F=0 cost).
Same vreg feeds block params → INC/DEC/SBC → F can't ALU.
Fix: locCompatible needs RegClass param, or block-param class
propagation must respect ALU constraints.

### nc.nanz (2) — genCmp16 LD L, IYH
Two identical functions (get_dir_entry, get_dir_entry_full).
lhs = IYH (8-bit half), genCmp16 zero-extends to HL.
Guard exists but code takes different path — needs trace.

### sieve.pas (2) — Pascal IndexExpr TyPtr
IndexExpr.ExprTy() returns ElemTy (u8) not TyPtr.
When used as StoreStmt.Ptr, ptr vreg gets wrong type.
PBQP allocates to D (8-bit) → LD (D), A invalid.
Fix: either IndexExpr returns TyPtr when used as ptr,
or Pascal lowerer wraps in explicit addr computation.

## Key Innovations
1. TSMC Spill (world-first SMC for register spilling)
2. HIR Function Splitting (21 auto-splits)
3. EX AF,AF' shadow register routing
4. Named spill labels (_spill_{func}_r{N})
5. emitLD8/emitADDHL/emitSBCHL universal helpers
6. Z80 39 virtual registers strategy
7. Z80-VALIDATE for MIR2 + LIR retry loop
