# BUG-009: ABAP u16 WHILE loop codegen — invalid Z80 instructions

**Status:** 🔴 Open
**Discovered:** 2026-03-16
**Affects:** fibonacci, fizzbuzz, bubble_sort, guessing_game, sysinfo
**Does NOT affect:** hello, hello_simple, hello_input, hello_param, selscreen, forms, oop, mara_report, sqlite_demo

---

## Symptoms

Three categories of invalid Z80 instructions in WHILE loops with u16 (`TYPE i`) arithmetic:

```
1. LD C, HL    — width mismatch: 16-bit value into 8-bit register
2. SBC HL, F   — F (flags) register used as 16-bit operand
3. ADD HL, C   — 8-bit register where 16-bit pair expected
```

## Root Cause Analysis

**The allocator assigns u16 variables to 8-bit physical locations.**

The PBQP allocator has 3 u16 locations: HL, DE, BC (plus IX, IY).
In a WHILE loop with multiple live u16 variables (loop var, limit, accumulator),
pressure exceeds available pairs. The allocator falls back to 8-bit registers
(C, D, E) or even LocFlag (F) for u16 values.

### Why LocFlag is assigned to u16

`locCompatible(TyU16, LocFlag)` returns `false` (width 16 != 1).
But `costGeneral` for LocFlag returns `InfCost`.
So LocFlag should NOT be assigned to u16 under normal circumstances.

**Hypothesis:** The u16 comparison result (`CmpLe`) is ClassFlag (TyBool, width=1),
which correctly goes to LocFlag. But the codegen then uses this flag result
in a context that expects a 16-bit operand (e.g., the SBC HL,rr pattern
for 16-bit comparison).

### Why 8-bit regs are assigned to u16

`locCompatible(TyU16, LocReg{"C"})` returns `false` (len("C")==1, so w<=8 required).
So single 8-bit regs should NOT be assigned to u16 values.

**Hypothesis:** The HIR lowerer or ABAP frontend may emit incorrect types.
The WHILE condition `lv_i <= lv_n` may not properly propagate TyU16 through
the comparison chain. If comparison operands are emitted as TyU8, the allocator
correctly assigns 8-bit regs — but then the codegen sees the operands are
actually 16-bit and generates invalid instructions.

## Where to Look

1. **ABAP lowerer** (`pkg/abap/lower.go`): Check `lowerExpr` for comparison ops.
   Does `<=` with u16 operands produce `BinExpr{Op: "<=", Ty: TyBool}` with
   u16 sub-expressions? Or does something truncate to u8?

2. **HIR lowerer** (`pkg/hir/lower.go`): Check `lowerCond` and `lowerBinExpr` for
   comparison with u16 operands. The CmpLe should use `SrcTy: TyU16` so the
   codegen knows to use SBC HL,rr.

3. **Codegen** (`pkg/mir2/z80codegen.go`): `genCmp16` at line ~5400 — handles
   16-bit comparison. Check if it correctly handles cases where operands ended
   up in unexpected locations.

## Impact

9/13 ABAP examples compile and assemble correctly. The 4 failures all involve
WHILE loops with u16 arithmetic (same pattern). Simple programs work perfectly.

## Fix Approach

1. Add debug logging in PBQP to detect u16-in-8bit assignments
2. Check ABAP comparison expression lowering for correct type propagation
3. May need `classForExpr` override for u16 comparison operands
