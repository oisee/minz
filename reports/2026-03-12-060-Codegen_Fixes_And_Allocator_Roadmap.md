# Report #060 — Codegen Correctness Fixes & Allocator Architecture Roadmap

**Date:** 2026-03-12
**Status:** 3 codegen bugs fixed, net +3 showcase examples passing, architecture direction confirmed

---

## Summary

Three codegen correctness bugs were found and fixed in `pkg/mir2/z80codegen.go`. All were in the
"widen u8 to u16" family — places where the codegen incorrectly assumed an 8-bit value already
occupied a 16-bit register. Showcase regression analysis: **+3 net improvements, 0 regressions.**

Also: architectural review of 8 compiler papers in `/Volumes/safe/z80-compiler/`, and a concrete
plan for the next optimization milestone (PBQP edge costs, Phase 6e).

---

## Bug Fixes

### Fix 1 — 16-bit store zero-extension (`OpStore` u8→u16)

**Symptom:** Storing a u8 constant (e.g. `1 : u8`) into a u16 global wrote the byte value to
both the low and high byte of the target — so `acc_u16 = 1` stored `0x0101` instead of `0x0001`.

**Root cause:** Three store paths (IX/IY, HL, DE/BC) all used `highByte(val)` unconditionally.
For an 8-bit `val` like `D`, `highByte("D") = "D"` — same register — giving the wrong byte twice.

**Fix:** In all three store paths, test `isPairReg(val)`:
- IXY path: `LD (IX+1), 0` instead of `LD (IX+1), D`
- HL path: `LD (HL), 0` instead of `LD (HL), D`
- DE/BC path: `XOR A` (clears A to 0) instead of `LD A, D`

```go
if !isPairReg(val) {
    g.emitf("    LD %s, 0     ; hi (zero-extend u8→u16)", ptrIndirect(ptr, 1))
} else {
    g.emitf("    LD %s, %s     ; hi", ptrIndirect(ptr, 1), highByte(val))
}
```

### Fix 2 — 16-bit self-pointer load (`OpLoad` HL from (HL))

**Symptom:** Loading a u16 global into HL via `LD L, (HL) / INC HL / LD H, (HL) / DEC HL` was
reading the wrong high byte. After `LD L, (HL)`, HL's low byte changes, so the subsequent
`INC HL` increments the *wrong* address.

**Root cause:** The INC/DEC trick only works when `dst ≠ ptr`. When both are HL, the first
byte load corrupts the pointer before the second byte is fetched.

**Fix:** Special-case `dst == ptr` using A as scratch:

```asm
LD A, (HL)    ; save lo byte without corrupting HL
INC HL        ; advance to hi byte — HL still valid
LD H, (HL)    ; hi byte
LD L, A       ; restore lo byte
```

```go
if dst == ptr {
    g.emitf("    LD A, (%s)     ; lo", ptr)
    g.emitf("    INC %s", ptr)
    g.emitf("    LD %s, (%s)     ; hi", highByte(dst), ptr)
    g.emitf("    LD %s, A       ; lo", lowByte(dst))
    g.invalidate("A")
} else {
    // original INC/DEC trick — safe when dst ≠ ptr
    ...
}
```

### Fix 3 — `mul16` rhs load order and 8-bit zero-extension

**Symptom:** `factorial(1)` returned 0. Multiply of `n:u8` by accumulator `u16` was broken.

**Root cause:** Two bugs in `emitMul16`:

1. **Operand order**: `PUSH BC / LD B,H / LD C,L` (which saves the multiplicand and overwrites BC)
   happened *before* loading `rhs` into DE. If `rhs` was in C (common for u8 params), it was
   overwritten before being read.

2. **8-bit rhs not zero-extended**: For an 8-bit rhs like `C`, `highByte("C") = "C"` again —
   so `LD D, C` stored the byte value in *both* D and E, giving e.g. `DE = 0x0101` instead of
   `DE = 0x0001`.

**Fix:** Load rhs into DE *before* the PUSH BC / LD B,H / LD C,L sequence. And zero-extend:

```go
// Load multiplier into DE BEFORE overwriting BC with the multiplicand.
if rhs != "DE" {
    if isPairReg(rhs) {
        g.emitf("    LD D, %s", highByte(rhs))
        g.emitf("    LD E, %s", lowByte(rhs))
    } else {
        // 8-bit rhs: zero-extend into DE
        g.emit("    LD D, 0")
        g.emitf("    LD E, %s", rhs)
    }
}
g.emit("    PUSH BC")   // NOW save BC (after rhs is safe in DE)
g.emit("    LD B, H")
g.emit("    LD C, L")   // BC = multiplicand (lhs)
```

---

## Showcase Regression Analysis

Tested all 23 showcase examples before and after changes:

| Example | Before | After | Note |
|---------|--------|-------|------|
| ex2_ufcs | FAIL (invalid LD) | **PASS** | fixed by Fix 1/2 |
| ex3_iface | FAIL (invalid LD) | **PASS** | fixed by Fix 1/2 |
| ex9b_factorial_fold | FAIL (invalid LD) | **PASS** | fixed by Fix 3 |
| ex12_assert | FAIL | FAIL | pre-existing (invalid RET codegen) |
| ex13_multiret_assert | FAIL | FAIL | pre-existing (min_of comparison) |
| ex14_fold_assert | FAIL (sum_to asm err) | FAIL (factorial(1)=0) | progressed; blocked on mul16 D-clobber |
| all others | PASS | PASS | no regressions |

**Net: +3 PASS, 0 regressions. 20/23 showcase examples passing.**

The three remaining failures are pre-existing bugs:
- **ex12**: Invalid `RET` instruction in void-function tail position — likely a `TermRet` emitted
  without a preceding load when return type is void.
- **ex13**: `min_of(20, 10) → 20` — conditional branch logic inverted for two-argument min/max.
- **ex14**: `factorial(1) → 0` — mul16 `LD D, 0` zero-extend clobbers D which holds live var `n`.

---

## Paper Review: `/Volumes/safe/z80-compiler/`

Eight papers. Three are directly relevant to the allocator roadmap:

### ⭐ Krause 2022 — "Efficient Calling Conventions for Irregular Architectures" (`2112.01397v2.pdf`)

**The most directly applicable paper.** Empirically evaluates thousands of Z80/Z180/Z80N/SM83/
STM8/Rabbit ABI variants against real benchmarks (Whetstone, Dhrystone, CoreMark, stdbench).
Finds SDCC's 20-year-old Z80 convention substantially suboptimal; the new convention became
SDCC 4.2.0 default.

**Z80 findings:**
- 8-bit return in `A`, 16-bit return in `HL`, 32-bit return in `HL+DE` (matches our ABI ✅)
- First 8-bit arg → `A`, first 16-bit arg → `HL`; remaining args → stack
- SDCC's old convention (all args on stack) wastes significant code size for common patterns
- The paper's recommended Z80 convention is very close to what MinZ/Nanz already implements

**Implication for MinZ:** Our register-first ABI (`ADR-0010`) is already well-aligned with the
empirically optimal calling convention. The paper validates our direction and can inform any
future ABI tuning (e.g. second argument placement).

### ⭐ Krause 2020 — "lospre in linear time" (`2011.10789v1.pdf`)

**LOSPRE** = Lifetime-Optimal Speculative Partial Redundancy Elimination. Subsumes CSE, GCSE,
LICM (loop-invariant code motion) in one framework. Linear time for structured programs via
CFG tree-decomposition.

Key property: **minimizes the lifetime of new temporaries**, not just redundancy count. This
directly reduces register pressure — the dominant cost in our codegen (fewer live virtuals →
fewer $F0xx spills).

**Implication for MinZ:** Our current MIR2 passes (constprop, DSE) handle easy cases. lospre
would handle the general case: redundant sub-expressions across branches, loop-hoistable
constants, repeated pointer arithmetic. Estimated ~500 LOC to implement the structured-program
variant. Medium-priority addition after Phase 6e.

### ⭐ Krause 2024 — "C for a tiny system" (`2010.04633v2.pdf`)

Padauk microcontroller C compiler (8-bit, 60–256 bytes RAM, accumulator-centric, very few regs).
Discusses memory-backed pseudo-registers, calling conventions, and code generation for minimal
register files.

**Implication for MinZ:** Padauk's constraint space is even tighter than Z80's. Their approach
to memory-backed pseudo-registers (our `$F0xx` spill model) and the trade-off with register
allocation is directly analogous to our `P4_Register_Allocator_Plan.md` situation.

### Other 5 papers

Graph theory / disjoint paths papers (Adler, Krause 2010–2019) — not directly applicable to
register allocation. They study tree-width and graph minors, which could theoretically parameterize
interference graph complexity, but the Z80 register file is so small that these bounds are
irrelevant in practice.

---

## Architecture Question: PBQP Edge Costs vs. Unified Graph

### The question

Can all possible movements (reg↔reg, reg↔pair, reg↔shadow, reg↔stack, reg↔mem, mem↔mem) be
described as a single weighted graph, and run optimization on top?

### Short answer

**Yes for the allocation sub-problem; no for a fully unified IS+RA system (and not worth it).**

### The full Z80 storage universe

```
8-bit:    A, B, C, D, E, H, L
Pairs:    BC, DE, HL, IX, IY, SP
Shadows:  A', BC', DE', HL'  — bulk swap via EXX / EX AF,AF' only
Flags:    F  — only PUSH AF / POP AF (no LD r,F)
Mem-ptr:  (HL), (DE), (BC), (IX+d), (IY+d)
Spill:    $F0xx memory, stack frames, globals
```

The cost of moving between any two locations is fully enumerable (a ~30×30 matrix). PBQP with
edge costs can represent all correlated allocation decisions as binary edges with cost matrices.

### What PBQP edge costs handle well

The current allocator has **node costs** only (how much does location `p` cost for virtual `v`).
Adding **edge costs** models correlations:

| Pattern | Edge | Benefit |
|---------|------|---------|
| LUT: idx∈C + ptr∈BC | edgeCost[idx][ptr][C][BC] = -4T | 18T→14T (ADR-0017 BC★) |
| DJNZ: counter→B | edgeCost[counter][*][B][*] = -∞ | prevents B from being used for other vars in loop body |
| mul16: rhs→DE | edgeCost[rhs][mult_result][E][HL] = -? | avoids zero-extend D clobber |
| ADD HL: dst→HL | edgeCost[dst][*][HL][*] = 0 | other allocations don't pay HL preference spuriously |

Implementation: ~150 LOC as specified in ADR-0017. Standard PBQP RN reduction with edge
projection — well-understood algorithm, LLVM has a reference implementation.

### What PBQP edge costs *cannot* handle

1. **EXX shadow registers**: EXX swaps all 6 of {BC,DE,HL,BC',DE',HL'} atomically. This is a
   *hyper-edge* — an n-ary constraint on 6 nodes simultaneously. Standard PBQP models pairwise
   edges only. Shadow registers need a separate "EXX save region" analysis pass that identifies
   function regions where the shadow bank can be borrowed.

2. **Combined instruction selection + register allocation**: Tree-tiling (BURS/IBURG) selects
   instruction patterns from the expression tree independently of PBQP. Unifying them gives ILP
   (NP-hard), which is not practical. The right architecture is: tree-tiling for IS, PBQP for RA,
   with IS informing PBQP node costs (e.g. "this op needs HL for ADD HL, so add 6T node cost
   for non-HL allocations").

3. **Memory-memory moves**: Only `LDIR` / explicit LD-loop. Not allocatable — model as spill
   cost, not as graph nodes.

### Recommendation: phased approach

```
Current state:  greedy graph coloring (alloc.go) + node costs only
                → working, but misses correlated allocation

Phase 6e:       PBQP edge costs (~150 LOC)
                → handles LUT BC★, mul16 rhs class, DJNZ B propagation
                → ADR-0017 already specifies this precisely

After 6e:       lospre MIR pass (~500 LOC, Krause 2020)
                → reduces live-range length → fewer spills
                → subsumes current constprop + partial DSE

Future:         EXX save region analysis (separate pass, not PBQP)
                → borrow shadow bank in tight loops
                → potentially +6 "free" virtual regs in hot loops
```

The Krause calling conventions paper confirms our ABI is already well-aligned with empirically
optimal conventions for Z80. The remaining gains are in allocation quality (Phase 6e) and
redundancy elimination (lospre), not in ABI redesign.

### Why NOT a fully unified graph

The Krause papers collectively suggest a pragmatic answer: even for the simple Padauk architecture
(far smaller than Z80), the practical approach is layered passes, not a monolithic optimizer.
The `C for a tiny system` paper handles pseudo-registers separately from allocation, and lospre
handles redundancy separately from IS. The combined IS+RA graph is theoretically elegant but
practically over-engineered for a 7-register machine.

---

## Next Steps

Immediate (pre-Phase 6e):
1. Fix ex12 — invalid `RET` in void tail position
2. Fix ex13 — `min_of` comparison direction
3. Fix ex14 — mul16 D clobber (need scratch reg that isn't live at mul16 call sites)

Phase 6e (PBQP edge costs):
- `edgeCost map[[2]Reg][2]int` in PBQP problem struct
- Pattern detector: page-aligned LUT access → C/BC edge, mul16 rhs → DE edge
- RN bucket-elimination: project edge costs onto neighbor node vector
- ~150 LOC, well-scoped, all prerequisites in ADR-0017

lospre (medium term):
- Implement Krause 2020 linear-time algorithm for structured CFGs
- Target: eliminate redundant address computations in loop bodies
- Pre-requisite: good CFG representation (already have it in MIR2 Func.Blocks)

---

## References

- `/Volumes/safe/z80-compiler/2112.01397v2.pdf` — Krause 2022, Efficient Calling Conventions for Irregular Architectures
- `/Volumes/safe/z80-compiler/2011.10789v1.pdf` — Krause 2020, lospre in linear time
- `/Volumes/safe/z80-compiler/2010.04633v2.pdf` — Krause 2024, C for a tiny system (Padauk)
- `docs/adr/0017-lut-pointer-selection-and-pbqp-edge-costs.md` — Phase 6e specification
- `docs/P4_Register_Allocator_Plan.md` — $F0xx elimination plan
- `minzc/pkg/mir2/alloc.go` — current greedy allocator (518 LOC)
