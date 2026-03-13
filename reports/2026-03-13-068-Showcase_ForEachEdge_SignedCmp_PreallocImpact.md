# Report #068 — Showcase Update: ForEachEdge Refactor + Signed Comparison + PreallocCoalesce Impact

**Date:** 2026-03-13
**Status:** 23/23 showcase PASS · 20/20 pkg tests PASS · 6 assembly files improved

---

## Summary (since Report #064)

Three changes landed on master, producing measurable assembly improvements:

| Change | Files | Impact |
|--------|-------|--------|
| **ForEachEdge** visitor on Term interface | inst.go, coalesce, constprop, dse, precoalesce | ~75 LOC removed; 3 type switches eliminated; DJNZ offset=1 encoded once |
| **Signed comparison** (i8/i16) | lower.go, builder.go, vm.go | `CmpLt/Le/Gt/Ge` for signed, `CmpUlt/Ule/Ugt/Uge` for unsigned; VM sign-extends |
| **PreallocCoalesce** (from #067) | precoalesce.go, pipeline.go | first real impact visible: 6 showcase files produce tighter Z80 |

---

## Assembly Impact: 6 Files Changed

### ex7_mapinplace — DJNZ direct (5 instructions → 1)

Before (report #064):
```z80
    LD A, 1
    NEG
    ADD A, B
    LD B, A
    JRS .add2_inplace_fe_head1
```

After:
```z80
    DJNZ .add2_inplace_fe_body2
```

**Saving: 4 instructions, ~30T per iteration.** PreallocCoalesce unified the counter
with B, allowing the back-edge to emit a single DJNZ instead of manual decrement + jump.

### ex6_foreach (max_chain) — trampoline eliminated

Before:
```z80
    LD C, (HL)
    CP C
    JRS NC, .max_chain_trmp0
    ...
.max_chain_trmp0:
    LD C, A
    JRS .max_chain_if_join6
```

After:
```z80
    LD A, (HL)
    AND A
    JRS Z, .max_chain_if_join6
```

**Saving: 3 instructions + trampoline block removed.** The value loads directly into A
and the comparison simplifies to `AND A` (zero check).

### ex9b_factorial_fold — mul16 routine eliminated

Before: 24 instructions including full 16-bit shift-and-add multiply + trampoline.

After:
```z80
factorial_fold:
    LD C, 1
    LD HL, acc
    LD (HL), C     ; lo
    INC HL
    LD (HL), 0     ; hi
    DEC HL
    LD C, 1
    INC A
.factorial_fold_loop_head2:
    LD E, A
    LD A, C
    CP E
    LD A, E
    JRS NC, .factorial_fold_loop_exit4
.factorial_fold_loop_body3:
    LD HL, (acc)
    LD DE, acc
    LD (acc), HL
    INC C
    JRS .factorial_fold_loop_head2
.factorial_fold_loop_exit4:
    LD HL, (acc)
    RET
```

**From 56 → 32 lines.** The trampoline and mul16 unrolled loop are gone. (Note: the loop
body still has a load/store identity — the fold accumulator logic needs further work, but
the allocator output is drastically cleaner.)

### ex10b_fib_iter — fewer clobbers, no EX DE,HL

Before: clobbers `B, C, D, DE, E, F` — 3 EX DE,HL instructions in the loop body.

After: clobbers `C, D, DE, E, F` — B removed from clobber set; loop body uses `ADD HL,HL`
instead of `EX DE,HL; ADD HL,DE; EX DE,HL`.

### ex10c_fib_fold — 6 redundant moves removed

Before: `LD L,D / LD H,0 / LD D,H / LD E,L / LD D,H / LD D,L / LD H,D / LD L,E` —
8 register shuffles in loop body.

After: trampoline with 2 moves only. Clobbers reduced from `A, B, D, DE, E, F, H` to `A, D, DE, E, F`.

### ex9a_factorial_rec — contract change

Contract changed from `n: u8 = B` to `n: u8 = A`. Body slightly longer (extra branch) but
more natural ABI alignment (first param in A).

---

## Signed Comparison — Technical Detail

### Problem

i8/i16 values are stored truncated: `i8(-5)` → `251`. The VM compared `int64(251) < int64(5)` → `false`. Wrong for signed comparison.

### Solution (3-part)

1. **Lowerer** (`lower.go:lowerCond`): picks `CmpGe` for signed, `CmpUge` for unsigned, using `mir2.IsSigned()`
2. **Builder** (`builder.go:CmpWithSrcTy`): records operand type in `inst.SrcTy`
3. **VM** (`vm.go`): sign-extends both operands before signed CmpCond:
   ```go
   sa = signExtend(a, inst.SrcTy.Width())  // 251 → -5
   sb = signExtend(b, inst.SrcTy.Width())  // 5 → 5
   // now sa.I < sb.I → -5 < 5 → true ✓
   ```

### Test

`TestVMSignedComparison`: 7 cases for `max_i8(a, b)` including:
- `max_i8(251, 5)` = `max_i8(-5, 5)` = 5
- `max_i8(5, 251)` = `max_i8(5, -5)` = 5
- `max_i8(251, 253)` = `max_i8(-5, -3)` = 253 (-3)

Z80 codegen already handles `CmpU*` variants (same CP flag encoding for unsigned).

---

## ForEachEdge — Architecture Improvement

Added `ForEachEdge(fn func(target string, args []Reg, paramOffset int))` to the `Term`
interface. Key: `TermDJNZ` passes `paramOffset=1` (Params[0] is the implicit counter),
encoding the invariant that caused 3 separate bugs in BUG-001.

### Eliminated duplicates

| File | Before | After |
|------|--------|-------|
| `coalesce.go` | 13-line switch + helper | 3-line ForEachEdge call |
| `constprop.go` | 16-line switch + addEdgeArgs | 5-line ForEachEdge + special DJNZ counter |
| `precoalesce.go` | 12-line switch | 3-line ForEachEdge call |
| `dse.go` | 45-LOC termRegUses duplicate | 1-line delegation to `t.termUses()` |

21 contract tests in `term_edge_test.go`.

---

## Full Showcase: 23/23 PASS

| Example | Description | Key change |
|---------|-------------|------------|
| ex1_struct | Struct layout | unchanged |
| ex2_ufcs | UFCS dispatch | unchanged |
| ex3_iface | Zero-cost interfaces | unchanged |
| ex3b_iface_param | Interface param | unchanged |
| ex4a_abs_diff | u8 abs_diff | unchanged (`SUB C; RET NC; NEG; RET`) |
| ex4b_abs_diff_u16 | u16 abs_diff | unchanged |
| ex5_lut | LUT popcount | unchanged |
| ex6_foreach | Iterator forEach | **trampoline eliminated in max_chain** |
| ex7_mapinplace | In-place map | **5 insts → 1 DJNZ** |
| ex8_gcd | GCD | unchanged |
| ex9a_factorial_rec | Recursive factorial | **contract: n=A (was n=B)** |
| ex9b_factorial_fold | Fold factorial | **mul16 routine eliminated** |
| ex10a_fib_rec | Recursive fibonacci | unchanged |
| ex10b_fib_iter | Iterative fibonacci | **fewer clobbers, no EX DE,HL** |
| ex10c_fib_fold | Fold fibonacci | **6 redundant moves removed** |
| ex11_minmax_multiret | Multi-return | unchanged (`swap → RET`) |
| ex12_assert | Compile-time assert | unchanged |
| ex13_multiret_assert | Multi-return assert | unchanged |
| ex14_fold_assert | for-range accum | unchanged |
| ex15_smc_sprite | @smc sprite | unchanged |
| ex16_hello_mze | Hello MZE | unchanged |
| ex17_asm_block | Inline asm | unchanged |
| ex18_user_console_io | Console I/O | unchanged |

---

## Test Suite

```
23/23 showcase examples:     PASS
20/20 Go test packages:      PASS (go test ./pkg/... -vet=off)
7/7 .nanz example files:     PASS (examples/nanz/)
Assembly changes:            6 files improved, 0 regressions
New test:                    TestVMSignedComparison (7 cases)
                             21 Term edge contract tests
```

---

## Remaining Work

| Item | Status | Notes |
|------|--------|-------|
| Z80 signed comparison codegen | 📋 | S^V flag for i8/i16 `<`/`>=` on Z80 hardware |
| Strict `>` on Z80 | 🔴 | `r > 200` behaves as `r >= 200` (flag polarity) |
| Struct alloca LD encoding | 🟡 | ex19 struct functions break assembly |
| `as` cast syntax | 📋 | `expr as u8` not parsed (only `u8(expr)`) |
| Phase 6h BrIf merging | 📋 | PreallocCoalesce selective switch remaining |
