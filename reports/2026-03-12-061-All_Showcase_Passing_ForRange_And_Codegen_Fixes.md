# Report #061 — All 23 Showcase Examples Passing: ForRange & Codegen Fixes

**Date:** 2026-03-12
**Status:** 23/23 showcase PASS, 26/26 test packages PASS

---

## Summary

Four bugs fixed in this session. Together they unblock `ex14_fold_assert.nanz` (the last failing
showcase example), achieving **23/23 showcase pass** and **26/26 Go test package pass** with zero
regressions.

---

## Fixes

### Fix 1 — `lowerForRange` precomputes loop bound (`hir/lower.go`)

**Symptom:** `factorial(1) == 0`, `sum_to(4) == 0`. For-range loops with an expression bound
(`n+1`) produced wrong results.

**Root cause:** `lowerForRange` was generating:
```
while i < n+1 { ... }
```
The `n+1` ADD instruction lived inside the loop header block. The ISA output-constraint recoloring
in `coalesceAllocResult` then placed either `n` or `i` (the ADD lhs) into the A register. Any
A-clobbering operation in the loop body (loads, the `LD A, 16` mul16 counter) corrupted the
loop-invariant value before the next iteration.

**Fix:** Precompute the upper bound once before entering the while loop:
```go
// lowerForRange — for v in start..end
endReg := l.lowerExpr(st.End)         // evaluate ONCE, outside the loop
endName := l.fresh("__for_hi_")
l.bind(endName, endReg, ty)

// Synthesize: while v < hi { body; v++ }
l.lowerWhile(&WhileStmt{
    Cond: &BinExpr{Op: "<", L: ..., R: &VarRefExpr{Name: endName, ...}},
    Body: whileBody,
})
```

Result: no ADD in the loop header; `n` is not forced into A; loop runs correctly.

---

### Fix 2 — `LD HL,(sym)` for u16 global loads (`z80codegen.go`)

**Symptom:** After Fix 1, `i` (loop counter) ended up in A. `LD A,(HL)` in the u16 global load
path clobbered `i` before the `PUSH AF` in mul16 could save it.

**Root cause:** Loading a u16 global via HL pointer used:
```asm
LD A, (HL)     ; lo — clobbers A (= loop counter i)
INC HL
LD H, (HL)     ; hi
LD L, A        ; restore lo
```

**Fix:** Pattern-detect `OpLoad(u16, AddrOf(global))` with `dst=HL` in
`scanGlobalFieldPatterns`. Emit `LD HL, (sym)` (20T, no A clobber) directly:
```asm
LD HL, (acc_u16)    ; u16 direct load (20T, no A clobber)
```

Z80 instruction `$2A`: 3 bytes, 20T, loads (sym) and (sym+1) directly into L and H.

---

### Fix 3 — `LD HL,sym; LD (HL),imm` for constant u8 global stores (`z80codegen.go`)

**Symptom:** `sum_to(4) == 0`. The `acc_u8 = 0` initialization before the loop destroyed
parameter `n` (which is in A per Z80 ABI for the first u8 argument).

**Root cause:** The `globalFieldStore` non-hlChain constant path emitted:
```asm
LD A, 0           ; ← clobbers n (parameter was in A)!
LD (acc_u8), A
```

**Fix:** For constant u8 global stores, use HL-indirect instead:
```go
if cv, ok := g.constVals[inst.Src[1]]; ok {
    // LD HL, sym; LD (HL), imm — 20T, no A clobber
    g.emitf("    LD HL, %s", lbl)
    g.emitf("    LD (HL), %d", cv)
    g.invalidate("HL")
}
```

Same cost (10T + 10T = 20T vs 4T + 13T = 17T — actually 3T more, but correctness beats 3T).
The HL-chain optimizer continues to fire for consecutive field stores; only isolated constant
stores fall through to this path.

---

### Fix 4 — `OpSub` HL↔DE uses `EX DE,HL` not byte-by-byte (`z80codegen.go`)

**Symptom:** `abs_diff_u16(300, 1000)` returned 0 after the `physicalAliases` extension was
added. Regression introduced by expanded sub-register alias tracking (H/L ↔ HL, D/E ↔ DE).

**Root cause:** The OpSub codegen comment at line 2124 said "emitMov for HL↔DE emits EX DE,HL
which is a SWAP", but `emitMov("HL","DE")` actually does byte-by-byte (`LD H,D; LD L,E`) — a
one-way copy that preserves DE. The rhs adjustment logic (swapping "HL"↔"DE" in the rhs string)
was correct assuming a swap, but the emit was wrong.

With the old allocator, `diff2` was consistently placed in DE (not HL), hitting a different
special case (line 2111). With expanded aliases, PBQP more aggressively places ClassPointer
results in HL, exposing the latent bug.

**Failing assembly (b_gt_a block computing b − a):**
```asm
LD H, D        ; HL = b (byte-by-byte copy of DE)
LD L, E        ; — but DE still = b, not a!
OR A
SBC HL, DE     ; b − b = 0  ✗
```

**Fix:** Emit `EX DE, HL` explicitly in the HL↔DE move case of OpSub:
```go
if (dst == "HL" && lhs == "DE") || (dst == "DE" && lhs == "HL") {
    g.emit("    EX DE, HL")   // true swap: HL←lhs, DE←old-HL
    // adjust rhs string to reflect swapped locations
    switch rhs {
    case "HL": rhs = "DE"
    case "DE": rhs = "HL"
    }
} else {
    g.emitMov(dst, lhs, w)
}
```

**Correct assembly:**
```asm
EX DE, HL      ; HL = b (old DE), DE = a (old HL)
OR A
SBC HL, DE     ; b − a ✓
```

---

## Showcase Results

All 23 showcase examples compiled and verified (compile-time `assert` checks via MIR VM):

| Example | Status | Description |
|---------|--------|-------------|
| ex1_struct | PASS | Struct layout + field access |
| ex2_ufcs | PASS | UFCS method dispatch |
| ex3_iface | PASS | Zero-cost interfaces |
| ex3b_iface_param | PASS | Interface as parameter |
| ex4a_abs_diff | PASS | u8 absolute difference |
| ex4b_abs_diff_u16 | PASS | u16 absolute difference |
| ex5_lut | PASS | LUT lookup with BC★ |
| ex6_foreach | PASS | Iterator forEach |
| ex7_mapinplace | PASS | In-place map |
| ex8_gcd | PASS | GCD (Euclidean) |
| ex9a_factorial_rec | PASS | Recursive factorial |
| ex9b_factorial_fold | PASS | Fold-based factorial |
| ex10a_fib_rec | PASS | Recursive fibonacci |
| ex10b_fib_iter | PASS | Iterative fibonacci |
| ex10c_fib_fold | PASS | Fold-based fibonacci |
| ex11_minmax_multiret | PASS | Multiple return values |
| ex12_assert | PASS | Compile-time assert (was pre-existing fail) |
| ex13_multiret_assert | PASS | Multi-return assert (was pre-existing fail) |
| **ex14_fold_assert** | **PASS** | **for-range accumulator (was failing, now fixed)** |
| ex15_smc_sprite | PASS | @smc sprite patching |
| ex16_hello_mze | PASS | Hello world via MZE |
| ex17_asm_block | PASS | Inline assembly block |
| ex18_user_console_io | PASS | Console I/O |

**23/23 PASS. 0 regressions.**

---

## ex14 Generated Assembly Highlights

`factorial` — key loop body:
```asm
; fun factorial(n: u8 = A) -> u16 = HL ; clobbers: C, D, DE, F
factorial:
    LD C, 1
    LD HL, acc_u16
    LD (HL), C        ; lo = 1
    INC HL
    LD (HL), 0        ; hi = 0 (zero-extend u8→u16)
    DEC HL
    ...
.factorial_loop_body3:
    LD HL, (acc_u16)  ; u16 direct load (20T, no A clobber)  ← Fix 2
    ; mul16 HL * A → HL  (~320T, 16-iter shift-and-add)
    PUSH AF           ; save loop counter (A)
    ...
    POP AF            ; restore loop counter
    LD (acc_u16), HL  ; u16 direct store (16T, no A clobber)  ← Fix 2 (store)
    INC A             ; increment loop counter
    JRS .factorial_loop_head2
```

`sum_to` — constant init no longer clobbers A:
```asm
; fun sum_to(n: u8 = A) -> u8 = A ; clobbers: C, D, E, F, HL
sum_to:
    LD HL, acc_u8     ; ← Fix 3: no LD A, 0
    LD (HL), 0        ;           LD (HL), 0 preserves A = n
    LD C, 0           ; i_init = 0
    INC A             ; hi = n+1  ← Fix 1: computed ONCE here, not in loop header
    ...
```

---

## Test Suite

```
26/26 Go test packages pass (go test ./pkg/... -vet=off)
23/23 showcase examples pass
```

---

## Next Steps

With all showcase examples passing, the natural next targets from the roadmap:

1. **PBQP edge costs (Phase 6e)** — `edgeCost` map for correlated allocations (LUT BC★,
   mul16 rhs→DE, DJNZ counter→B). ~150 LOC, spec in `docs/adr/0017-...`.

2. **Fix BUG-003** — `ptr[i]` in while loop produces invalid `EX DE,HL / ADD F,DE` (PtrAdd cycle).

3. **Fix BUG-006** — Zero-size struct globals not emitted (undefined symbol at link time).

4. **lospre** (medium term) — Krause 2020 linear-time redundancy elimination to reduce live-range
   length and spill count.
