# BUG: Z80 Inline ASM Calling Convention Issues

**Status:** Open
**Found:** 2026-03-21
**Affects:** Any function with `asm z80 (in ...)` and 2+ parameters
**Severity:** Medium — workaround exists but fragile

---

## Bug 1: `@z80_c` maps to ClassGeneral, not physical register C

### Symptom
```nanz
fun f(@z80_a col: u8, @z80_b row: u8, @z80_c line: u8) -> u16 { ... }
```
Expected: col=A, row=B, line=C.
Actual:   col=A, row=C, line=B (PBQP swaps B and C).

### Root Cause
In `pkg/hir/lower.go:z80RegNameToClass`:
```go
case "C":
    return mir2.ClassGeneral  // NOT "register C"!
```
`ClassGeneral` means "any general register" — PBQP is free to put it in B, C, D, or E.
Only `@z80_a` (ClassAcc) and `@z80_hl` (ClassPointer) pin to specific physical registers.

### Fix Options
1. Add `ClassRegC`, `ClassRegB`, `ClassRegD`, `ClassRegE` — explicit single-register classes
2. Add `LocPin` constraint that forces PBQP to use a specific physical register
3. Document that `@z80_b`/`@z80_c` are hints, not guarantees

---

## Bug 2: Caller may not emit `LD A, imm` for first constant argument

### Symptom
```nanz
assert test_conv(0x11, 0x22, 0x33) == 0x2211 via z80
```
Expected: A=0x11. Actual: A=0 (not loaded).

Caller asm shows `LD C, 34 / LD B, 51` but no `LD A, 17`.

### Root Cause
When first argument is a compile-time constant and destination is ClassAcc (A),
the codegen may optimize away the load assuming A already has the value,
or the assert test harness doesn't emit the argument setup for param[0].

### Workaround
Use `(in param_name)` in asm block — the `in` clause forces the lowerer to
route the parameter value into the register the asm expects, regardless of
calling convention.

---

## Bug 3: `screen_addr(15, 12, 3)` off by one (0x4B8F vs 0x4C8F)

### Symptom
6/7 screen_addr asserts pass. The 7th:
```
screen_addr(15, 12, 3) == 0x4C8F  →  got 0x4B8F
```
Difference: H = 0x4B vs 0x4C. That's bit 0 of H wrong.

### Analysis
- row=12 → TT = 12>>3 = 1, sub-row = 12&7 = 4
- line=3
- Expected H: 010 01 011 = 0x4B... wait, that IS 0x4B.

Let me recalculate: row=12, line=3
- pixel_y = 12*8 + 3 = 99
- third = 99/64 = 1 → TT = 01
- pixel_line_in_third = 99 - 64 = 35
- char_row_in_third = 35/8 = 4 → SSS = 011 (line=3, NOT row sub-row)

Actually the formula is:
- H = 010 | (row & 0x18) | line = 0x40 | (12 & 0x18) | 3 = 0x40 | 0x08 | 0x03 = 0x4B
- L = ((row & 7) << 5) | col = (4 << 5) | 15 = 0x80 | 0x0F = 0x8F

So 0x4B8F is CORRECT! The expected value 0x4C8F was wrong in the assert.

### Resolution
Fix the assert: `screen_addr(15, 12, 3) == 0x4B8F`

---

## Recommended Testing Pattern

Until calling convention is stabilized, test asm functions by:
1. Compiling to .a80 and checking the `; fun name(params = REGS)` annotation
2. Writing asm body to match the actual convention shown in the annotation
3. Using `assert ... via z80` to verify correctness

```nanz
// Check what convention PBQP chose:
// ; fun screen_addr(col: u8 = A, row: u8 = C, line: u8 = B) -> u16 = HL
// Then write asm matching A=col, C=row, B=line
```
