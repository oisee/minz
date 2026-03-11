# Plan: From fill_rect Bug to Optimal Z80 Multiplication

**Author:** Claude (for maintainer review)
**Date:** 2026-03-11
**Status:** Proposal
**References:** antique-toy/chapters/ch04-maths/draft.md (Dark / X-Trade, Spectrum Expert #01)

---

## 1. The Bug

`TestTSMCRealWorldBenchmark` in `pkg/z80testing/e2e_tsmc_verification_test.go:348`
silently skips all benchmarks because `fill_rect(screen, 0, 0, 32, 24, 0x55)` exceeds
the hard-coded `maxInstr := 100000` instruction limit in `e2e_harness.go:379`.

The test compiles MinZ to Z80 machine code, then runs it on the emulator.
Compilation succeeds. The binary is valid. But the generated code is so bloated
that 768 iterations of a nested loop (32×24) don't finish in 100K instructions.

---

## 2. Root Cause Analysis (three layers)

### Layer 1: Strength Reduction Gap — `* 256` not folded

The inner expression `(y + row) * 256 + x` compiles to a full 16-bit
Russian-peasant shift-and-add loop (~320T, ~25 bytes, 16 iterations of
SRL/RR/ADD HL,BC/SLA/RL/DEC/JR).

But `* 256` is simply "put the low byte into the high byte":

```z80
; (y + row) * 256: just move L into H, zero L
LD H, L
LD L, 0
; then ADD HL, <x>
```

Cost: 8T, 2 bytes. **40x faster than the current codegen.**

Current state in `z80codegen.go:2148-2244` (`genMul16`):
- Power-of-2 constants are handled via N × `ADD HL,HL` (line ~2172)
- For `* 256` this means 8 × `ADD HL,HL` = 56T, 16 bytes
- But `* 256` has a trivial special case: byte swap + zero

Missing special cases for 16-bit multiply by constant:
| Constant | Optimal codegen | T-states | Current codegen | Current T |
|----------|----------------|----------|-----------------|-----------|
| 256 | `LD H,L / LD L,0` | 8T | 8×`ADD HL,HL` | 56T |
| 128 | `SRL L / LD H,L / LD L,0 / RR L` | ~24T | 7×`ADD HL,HL` | 49T |
| 512 | `LD H,L / LD L,0 / ADD HL,HL` | 19T | 9×`ADD HL,HL` | 63T |

### Layer 2: Register Allocator Spills to $F0xx Memory

PBQP allocator (`pkg/mir2/pbqp.go`) spills loop variables to `$F0xx` memory
when the interference graph has too many simultaneously-live registers.

Each spill costs 26T round-trip (`LD (nn),A` + `LD A,(nn)`).
In the `fill_rect` loop body, if all 5 variables (row, col, offset, screen, color)
spill, that's ~130T extra per iteration × 768 iterations = ~100K T-states wasted.

This is the known "memory-backed registers" issue (CLAUDE.md, Iterator status doc).
Root cause: PBQP lacks affinity edges (BUG-001) and the RN phase greedily spills
when >3 registers interfere in a loop.

### Layer 3: Test Harness Limit Too Low

`CallFunction()` in `e2e_harness.go:379` uses `maxInstr := 100000`.
The older `z80_test_framework.go:247` uses `maxCycles = 1000000` (10x higher).
The limit is undocumented, has no TODO, and is not mentioned in E2E_TESTING.md.

---

## 3. Fix Plan (ordered by impact / effort ratio)

### Fix A: Strength Reduction for `* 256` and Byte-Boundary Powers of 2

**Impact:** Eliminates the multiply entirely for `fill_rect` and similar screen-address code.
**Effort:** ~20 lines in `genMul16()`.
**Risk:** Low — additive, existing tests cover other constant cases.

**File:** `pkg/mir2/z80codegen.go`, function `genMul16()` (~line 2165)

Before the existing power-of-2 `ADD HL,HL` chain, add:

```go
// Special case: * 256 = byte position swap
if cv == 256 {
    g.emit("LD H, L")
    g.emit("LD L, 0")
    return
}

// Special case: * N where N = 2^k and k >= 8
// e.g. * 512 = * 256 then * 2
if cv >= 256 && cv&(cv-1) == 0 {
    g.emit("LD H, L")
    g.emit("LD L, 0")
    k := bits.TrailingZeros64(uint64(cv)) - 8
    for i := 0; i < k; i++ {
        g.emit("ADD HL, HL")
    }
    return
}
```

Also consider `* 128`:
```go
if cv == 128 {
    // L * 128 = (L >> 1) in H, (L & 1) << 7 in L
    g.emit("SRL L")      // L >>= 1, bit0 → carry
    g.emit("LD H, L")
    g.emit("LD L, 0")
    g.emit("RR L")       // carry → L bit7
    return
}
```

### Fix B: General Strength Reduction via Decomposition (from ch04-maths)

**Impact:** Covers `* 10`, `* 12`, `* 40`, `* 80` etc. common in screen address calculations.
**Effort:** ~40 lines.
**Risk:** Low.

For any constant N, decompose into shifts and adds:
- `* 10 = * 8 + * 2` → 3×`ADD HL,HL` + `PUSH HL/POP DE` + `ADD HL,HL` + `ADD HL,DE`
- `* 40 = * 32 + * 8` → similar
- `* 320 = * 256 + * 64` → `LD H,L / LD L,0` + save + 6×`ADD HL,HL` + `ADD HL,saved`

Already partially done for 3, 5, 6, 9 in `genMul16()` (lines 2180-2205).
Generalize: for any constant with ≤3 set bits, emit shift+add sequence.
For constants with >3 set bits, keep the Russian-peasant loop.

**Decision rule:**
```
popcount(N) <= 3  →  shift+add decomposition
popcount(N) >  3  →  Russian-peasant loop (current)
```

### Fix C: Square Table Multiply (from ch04-maths, Dark's Method 2)

**Impact:** 61T vs 200T for arbitrary u8×u8. 3.3x speedup.
**Effort:** ~80 lines codegen + 512-byte table generation.
**Risk:** Medium — requires 512 bytes of ROM. Opt-in via pragma or auto for hot loops.

From Dark / X-Trade, Spectrum Expert #01 (1997):

```
A * B = ((A+B)² - (A-B)²) / 4
```

Two lookups + one subtraction = ~61T.

**Implementation:**
1. Add `__sq_table` to runtime (512-byte page-aligned table of n²/4)
2. In `genMul()` for u8×u8 variable case (currently `TODO: general mul` at line 2120):
   - Emit inline square-table lookup (12 instructions, ~61T)
   - Auto-include table when any variable u8 multiply exists in the program
3. Table generation: emit in `.data` section, compute at assemble time or as `DB` literals

**Trade-off (quoting Dark):** *"Choose: speed or accuracy."*
Rounding error ≤ 0.25 per lookup — negligible for screen addresses, sprites, scrolling.
For fixed-point 3D math where precision matters, keep shift-and-add (Method 1).

### Fix D: Variable u8×u8 Multiply (currently TODO)

**Impact:** Unblocks any program that multiplies two runtime u8 values.
**Effort:** ~30 lines.
**Risk:** Low.

`genMul()` line 2120 currently emits `// TODO: general mul`.
Implement Dark's Method 1 (mulu112) inline:

```z80
; B = lhs, C = rhs (or A = lhs loaded first)
LD A, 0
LD D, 8
.loop:
    RR C
    JR NC, .noadd
    ADD A, B
.noadd:
    RRA
    DEC D
    JR NZ, .loop
; result in A:C (high:low)
```

196-204T depending on set bits. Could also emit as CALL to shared routine
to save code size when multiply appears multiple times.

### Fix E: Signed Multiply (from ch04-maths)

**Impact:** Enables signed arithmetic for 3D, physics, etc.
**Effort:** ~50 lines.
**Risk:** Low.

Two approaches from the book:

1. **2×abs method** (mul_signed, ~240-260T): XOR signs, abs both, unsigned mul, negate if needed
2. **Fix-unsigned** (Ped7g, ~same cost on Z80, cleaner): Do unsigned mul, then subtract corrections

Recommendation: implement fix-unsigned — shorter code, no PUSH/POP for sign flag.

### Fix F: Raise Test Harness Limit

**Impact:** Unblocks TSMC benchmarks even before codegen improvements.
**Effort:** 1 line.
**Risk:** None — only affects test execution time.

**File:** `pkg/z80testing/e2e_harness.go:379`

```go
// Before:
maxInstr := 100000
// After:
maxInstr := 1000000  // align with z80_test_framework.go:247
```

Also: make it a parameter of `CallFunction()` or add `CallFunctionWithLimit()`.
Document in E2E_TESTING.md.

### Fix G: PBQP Affinity Edges (longer term)

**Impact:** Fixes the root cause of memory-backed registers.
**Effort:** ~200-400 lines (BUG-001 in Open_Bugs_RCA.md).
**Risk:** Medium — core allocator change, needs careful testing.

Add affinity (preference) edges between:
- Block parameter and its argument at the jump site
- CALL argument and the ABI-required physical register
- Binary op result and one of its inputs (2-address form on Z80)

This guides PBQP to keep values in registers across loop back-edges
instead of spilling to $F0xx.

---

## 4. Recommended Execution Order

```
Phase 1 — Quick wins (1-2 days):
  [A] * 256 → LD H,L / LD L,0 special case
  [F] Raise maxInstr to 1M
  → Test: fill_rect benchmark should pass, show T-state reduction

Phase 2 — Complete multiply (3-5 days):
  [D] Variable u8×u8 (mulu112 inline)
  [B] General decomposition for ≤3-bit constants
  → Test: all examples recompile, Go test suite green

Phase 3 — Signed + fast path (3-5 days):
  [E] Signed multiply (fix-unsigned method)
  [C] Square table for hot u8×u8 (opt-in or auto)
  → Test: new test cases for signed mul, square-table accuracy

Phase 4 — Regalloc (2-4 weeks):
  [G] PBQP affinity edges
  → Test: iterator chain benchmarks, fill_rect cycle counts
```

---

## 5. Verification

After each phase:

1. `cd minzc && go test ./...` — all 22 packages pass
2. `go test -run TestTSMCRealWorldBenchmark -v ./pkg/z80testing/` — benchmarks run, not skipped
3. `go test -run TestDollarMangledSymbols -v ./pkg/z80asm/` — no regression
4. `./compile_all_examples.sh` — ≥71/73 core examples pass
5. Manual: compile fill_rect, inspect .a80 output for `LD H,L / LD L,0` (not 8×ADD HL,HL)

---

## 6. References

- **Dark / X-Trade**, "Programming Algorithms", Spectrum Expert #01, 1997
  - Method 1: shift-and-add (mulu112), 196-204T
  - Method 2: square table lookup, ~61T, 512-byte table
  - Division, sine tables, signed multiply
  - Source: `antique-toy/chapters/ch04-maths/draft.md`

- **Ped7g** (Peter Helcmanovsky): fix-unsigned signed multiply, Z80N MUL variant

- **BUG-001** (Open_Bugs_RCA.md): PBQP affinity edges for parallel-copy coalescing

- **Current codegen:** `pkg/mir2/z80codegen.go`
  - genMul: lines 2021-2121
  - genMul16: lines 2148-2244
  - genMul32: lines 2252-2356

- **Current MIR peephole:** `pkg/optimizer/mir_peephole.go`
  - x*0, x*1, x*2..x*16 strength reduction: lines 305-476

- **Test harness:** `pkg/z80testing/e2e_harness.go:379` (maxInstr=100000)
