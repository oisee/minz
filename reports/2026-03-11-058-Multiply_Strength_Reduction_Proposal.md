# Report 058 — Z80 Multiplication: From TestTSMC Failure to Optimal Codegen

**Date**: 2026-03-11
**Status**: Proposal (not yet implemented)
**Author**: Claude Opus 4.6 (analysis session with Alice)
**Companion**: `docs/plan_multiply_strength_reduction.md` (detailed fix plan)
**References**: antique-toy/chapters/ch04-maths/draft.md (Dark / X-Trade, Spectrum Expert #01, 1997)

---

## The Problem

`TestTSMCRealWorldBenchmark` in `pkg/z80testing/e2e_tsmc_verification_test.go:348`
silently **skips** 3 of 5 benchmarks (`fill_rect_small`, `fill_rect_large`, `blit_sprite`)
because compiled code exceeds the hard-coded **100K instruction limit** in the emulator
harness (`e2e_harness.go:379`, `CallFunction()`).

The remaining 2 benchmarks (`draw_hline`, `draw_vline`) run but show **0% TSMC improvement**,
causing the test to FAIL. This is the only test failure in the suite (besides `codegen` build
failure from frozen MIR1).

---

## Root Cause Analysis

Three layers contribute to the failure, from most impactful to least:

### Layer 1: Missing `* 256` Strength Reduction (40x speedup available)

The `fill_rect` test function computes:

```minz
let offset: u16 = (y + row) * 256 + x;
```

`genMul16()` (z80codegen.go:2263) handles power-of-2 constants via repeated `ADD HL,HL`.
For `* 256` this emits **8 consecutive** `ADD HL,HL` instructions (56T, 16 bytes).

But `* 256` has a trivial byte-swap implementation:

```z80
LD H, L       ; 4T, 1 byte
LD L, 0       ; 7T, 2 bytes
              ; total: 11T, 3 bytes (vs 56T, 16 bytes)
```

Per inner iteration: 45T saved. Over 768 iterations: **~34,560T saved** — roughly
a third of the entire execution budget.

**Other byte-boundary constants with trivial codegen:**

| Constant | Optimal | T-states | Current (`ADD HL,HL` chain) |
|----------|---------|----------|-----------------------------|
| `* 256`  | `LD H,L / LD L,0` | 11T | 8 × `ADD HL,HL` = 56T |
| `* 512`  | above + `ADD HL,HL` | 22T | 9 × `ADD HL,HL` = 63T |
| `* 128`  | `SRL L / LD H,L / LD L,0 / RR L` | ~26T | 7 × `ADD HL,HL` = 49T |

### Layer 2: Register Allocator Spills to $F0xx

The PBQP allocator (`pkg/mir2/pbqp.go`) spills loop variables to sequential memory
addresses starting at `$F000`. Each spill costs **26T round-trip** (LD (nn),A + LD A,(nn)).

In `fill_rect`, 5 live variables in the inner loop (row, col, offset, screen, color)
compete for 7 primary 8-bit registers (A,B,C,D,E,H,L). The RN phase of PBQP greedily
spills when interference exceeds available registers.

Per iteration overhead: ~130T (5 vars × 26T). Over 768 iterations: **~100K T-states wasted**.

Known as "memory-backed registers" issue — tracked in CLAUDE.md and
`docs/Iterator_Implementation_Status.md`. Root cause: PBQP lacks **affinity edges**
(BUG-001 in `docs/Open_Bugs_RCA.md`) and has no pre-allocation coalescing.

### Layer 3: Test Harness Limit

`CallFunction()` in `e2e_harness.go:379` uses `maxInstr := 100000`.
The older `z80_test_framework.go:247` uses `maxCycles = 1000000` (10x higher).
The 100K limit is undocumented and not mentioned in `E2E_TESTING.md`.

---

## What Was Already Fixed (commits 5b95cda → 5b0d4cc)

These fixes landed today and address **related** issues from the same analysis session:

| Commit | Fix | Impact |
|--------|-----|--------|
| `d2fdda1` | Peephole: double `EX DE,HL` elimination | -2 insts, -8T per occurrence |
| `d2fdda1` | Peephole: single-`JP` → `EQU` alias (tail-call) | 0 bytes, 0T for wrapper functions |
| `797b8f7` | Auto-infer `asm z80` inputs from `@z80_X` annotations | Correct register liveness for inline asm |
| `54450e2` | Fix `PropagateConstants` infinite loop on DJNZ | Unblocks range-fold compilation |
| `5b0d4cc` | `emitParallelCopy` A↔B cycle scratch selection | Correct swap for 8-bit register cycles |
| `5b0d4cc` | `range(lo..hi)` iterator source | Counter-based iteration, DJNZ loops |

**Not yet fixed:** multiplication codegen, regalloc spills, test harness limit.

---

## Proposed Fixes

### Fix A: `* 256` and Byte-Boundary Strength Reduction

**File:** `pkg/mir2/z80codegen.go`, `genMul16()` (~line 2285)
**Effort:** ~20 lines | **Impact:** High — directly unblocks `fill_rect`

Insert before the existing power-of-2 `ADD HL,HL` chain:

```go
// * 256: byte swap (low → high, zero low)
if cv == 256 {
    g.emit("    LD H, L")
    g.emit("    LD L, 0")
    g.invalidate("HL")
    return
}
// * N where N ≥ 256 and N is power-of-2: byte swap + remaining shifts
if cv >= 256 && cv&(cv-1) == 0 {
    g.emit("    LD H, L")
    g.emit("    LD L, 0")
    k := bits.TrailingZeros64(uint64(cv)) - 8
    for i := 0; i < k; i++ {
        g.emit("    ADD HL, HL")
    }
    g.invalidate("HL")
    return
}
```

### Fix B: General Constant Decomposition (≤3 set bits)

**File:** same function
**Effort:** ~40 lines | **Impact:** Medium — covers `* 10`, `* 12`, `* 40`, `* 320`, etc.

For any constant N with `popcount(N) ≤ 3`, decompose into shift+add sequence
instead of the 16-iteration Russian-peasant loop. Already done for 3,5,6,9 as
special cases — generalize.

Decision rule:
```
popcount(N) ≤ 3  →  shift+add decomposition (~20-60T)
popcount(N) > 3  →  Russian-peasant loop (~320T)
```

### Fix C: Variable u8×u8 Multiply (Dark's Method 1)

**File:** `pkg/mir2/z80codegen.go`, `genMul()` (line ~2235, currently `TODO`)
**Effort:** ~30 lines | **Impact:** High — closes a codegen gap

Replace TODO with Dark's mulu112 (shift-and-add from LSB):

```z80
; B = lhs, C = rhs → result in A:C (high:low)
LD  A, 0
LD  D, 8
.loop:
    RR  C
    JR  NC, .noadd
    ADD A, B
.noadd:
    RRA
    DEC D
    JR  NZ, .loop
```

196-204T depending on set bits. From Dark / X-Trade, Spectrum Expert #01 (1997).
For programs with multiple u8 multiplies, emit as CALL to shared routine to save code size.

### Fix D: Square Table Lookup (Dark's Method 2) — Optional

**File:** runtime table + `genMul()` alternate path
**Effort:** ~80 lines | **Impact:** High for hot loops — 61T vs 200T (3.3x)

Trades 512 bytes of ROM for `A * B = ((A+B)² - (A-B)²) / 4`.
Two lookups + one subtraction = ~61T.

**Trade-off** (quoting Dark): *"Choose: speed or accuracy."*
Rounding error ≤ 0.25 — negligible for screen addresses, scrolling, sprites.
Not suitable for fixed-point 3D rotation where individual vertex jitter is visible.

Recommendation: auto-enable when the program has ≥3 variable u8 multiplies, or via pragma.

### Fix E: Signed Multiply

**File:** `genMul()` + new `genMulSigned()`
**Effort:** ~50 lines | **Impact:** Medium — enables 3D math, physics

Two approaches from ch04-maths:

1. **2×abs** (mul_signed): XOR signs, abs both, unsigned mul, negate if negative. ~240-260T.
2. **Fix-unsigned** (Ped7g): Do unsigned mul, subtract corrections from high byte. Same cost, cleaner code, no PUSH/POP.

Recommendation: fix-unsigned — shorter, branchless correction.

### Fix F: Raise Test Harness Limit

**File:** `pkg/z80testing/e2e_harness.go:379`
**Effort:** 1 line | **Impact:** Unblocks benchmarks immediately

```go
maxInstr := 1000000  // was 100000; align with z80_test_framework.go:247
```

Better: make configurable via parameter or method variant.

### Fix G: PBQP Affinity Edges (longer term)

**File:** `pkg/mir2/pbqp.go`
**Effort:** ~200-400 lines | **Impact:** Fixes root cause of memory-backed registers

Add preference edges between:
- Block parameter ↔ argument at jump site (loop back-edge coalescing)
- CALL argument ↔ ABI physical register
- Binary op result ↔ input (2-address form preference)

This is BUG-001 in `docs/Open_Bugs_RCA.md`. Fixing it would drop the ~5x
iterator overhead to ~1.2-1.5x across all programs, not just multiply.

---

## Execution Order

```
Phase 1 — Quick wins (1-2 days):
  [A]  * 256 → LD H,L / LD L,0             ~20 lines
  [F]  maxInstr 100K → 1M                   1 line
  → Verify: fill_rect benchmark passes, T-states drop

Phase 2 — Complete multiply codegen (3-5 days):
  [C]  Variable u8×u8 (mulu112)             ~30 lines
  [B]  General ≤3-bit constant decomp       ~40 lines
  → Verify: all examples compile, Go tests green

Phase 3 — Advanced multiply (3-5 days):
  [E]  Signed multiply (fix-unsigned)        ~50 lines
  [D]  Square table lookup (opt-in)          ~80 lines
  → Verify: new test cases, accuracy bounds

Phase 4 — Regalloc (2-4 weeks):
  [G]  PBQP affinity edges                  ~200-400 lines
  → Verify: iterator benchmarks, fill_rect cycle counts
```

---

## Verification Checklist

After each phase:

- [ ] `cd minzc && go test ./...` — all packages pass (currently 1 FAIL: z80testing)
- [ ] `go test -run TestTSMCRealWorldBenchmark -v ./pkg/z80testing/` — benchmarks run, not skipped
- [ ] `./compile_all_examples.sh` — ≥71/73 core examples pass
- [ ] Manual: compile fill_rect, inspect `.a80` for `LD H,L / LD L,0` (not 8×`ADD HL,HL`)
- [ ] Manual: compile program with u8 variable multiply, verify mulu112 emitted (not TODO comment)

---

## Key Source Locations

| Component | File | Lines |
|-----------|------|-------|
| genMul (u8) | `pkg/mir2/z80codegen.go` | 2139-2236 |
| genMul16 (u16) | `pkg/mir2/z80codegen.go` | 2263-2359 |
| genMul32 (u32) | `pkg/mir2/z80codegen.go` | 2367-2471 |
| MIR strength reduction | `pkg/optimizer/mir_peephole.go` | 305-476 |
| PBQP allocator | `pkg/mir2/pbqp.go` | 113-320 |
| Spill region ($F0xx) | `pkg/mir2/pbqp.go` | 274 |
| Test harness limit | `pkg/z80testing/e2e_harness.go` | 379 |
| Test: TSMC benchmark | `pkg/z80testing/e2e_tsmc_verification_test.go` | 348-532 |
| fill_rect source | `pkg/z80testing/e2e_tsmc_verification_test.go` | 361-373 |
| stdlib fast_mult | `stdlib/mathlib/multiply.minz` | (not compiler-integrated) |

---

## References

- **Dark / X-Trade**, "Programming Algorithms", *Spectrum Expert* #01, 1997
  — antique-toy/chapters/ch04-maths/draft.md
  — Method 1: shift-and-add (mulu112), 196-204T
  — Method 2: square table lookup (mulu_fast), ~61T, 512-byte table
  — Signed multiply, division, sine/cosine tables

- **Ped7g** (Peter Helcmanovsky)
  — Fix-unsigned signed multiply technique
  — Z80N `MUL DE` variant (~70T, 16 bytes)

- **BUG-001** — `docs/Open_Bugs_RCA.md`
  — GCD parallel-copy bloat, PBQP lacks affinity edges

- **Iterator overhead** — `docs/Iterator_Implementation_Status.md`
  — ~207T actual vs ~43T ideal per element (pre-v0.19.5)
  — ~26T after v0.19.5 overhaul, still above ideal
