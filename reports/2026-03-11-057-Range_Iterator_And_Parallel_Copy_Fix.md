# Report 057 — `range()` Iterator Source + emitParallelCopy Cycle Fix

**Date**: 2026-03-11
**Status**: Complete — 23/23 packages green, E2E emulator verified

---

## Summary

Two bugs fixed, one new language feature shipped:

1. **`PropagateConstants` infinite loop** on DJNZ loops — already committed `54450e2`, blocking all range-fold compilation
2. **`emitParallelCopy` 8-bit A↔B cycle** — incorrect swap emitted when A is in the cycle; data lost
3. **`range(lo..hi)` iterator source** — counter-based source, no memory/pointer, compiles to optimal `DJNZ` loop

---

## Bug 1: PropagateConstants Infinite Loop (`constprop.go`)

### Symptom

Compiling any `range(0..n).fold(...)` caused the compiler to hang forever — `go test` timeout with stack trace pointing to `PropagateConstants` spinning.

### Root Cause

`PropagateConstants` collects incoming values for each block param by iterating over all predecessor edges. For `TermDJNZ`, the body block has:

```
body.Params[0]  = counter (B)   — driven by DJNZ itself, implicit
body.Params[1]  = accumulator   — driven by BodyArgs[0]
body.Params[2]  = ...           — driven by BodyArgs[1]
```

The code was registering `BodyArgs[i]` at index `i` instead of `i+1`:

```go
// BEFORE (wrong):
for i, a := range t.BodyArgs {
    k := paramKey{t.Body, i}   // off by 1 — feeds Params[0] (counter), not Params[1]
    incoming[k] = append(incoming[k], a)
}

// AFTER (correct):
for i, a := range t.BodyArgs {
    k := paramKey{t.Body, i + 1}   // Params[0] is counter (implicit), BodyArgs → Params[1:]
    incoming[k] = append(incoming[k], a)
}
```

### Consequence

`incoming[{body, 1}]` (the accumulator param) only had one entry: the `Const(0)` seed from the entry `BrIf`. `PropagateConstants` "proved" the accumulator was always 0, materialized a fresh `Const(0)` each iteration (`changed=true`), causing infinite loop.

---

## Bug 2: `emitParallelCopy` 8-bit A↔B Cycle (`z80codegen.go:3380`)

### Symptom

`range(0..n).fold(0, |acc,i| acc+i)` compiled and generated this trampoline:

```z80
.trmp0:
    LD A, B      ; B=0 (Const(0)), A becomes 0 — n is LOST
    LD B, A      ; B=0 still — swap is a no-op
    JRS .rng_body1
```

Both A and B become 0. The counter never loads `n`. The loop body adds 0 every iteration and exits with 0.

### Root Cause

The parallel copy emitter uses A as a scratch register for 8-bit cycles. The backward-walk algorithm:

1. Save `first.src` to A
2. Walk the cycle backward, emitting `LD dest, src`
3. Emit `LD firstDst, A` at the end

When A is **itself in the cycle** (A→B, B→A), step 1 is skipped (`if m.src != "A"`), but the walk in step 2 overwrites A (with B's value) before step 3 needs the original A. The final `LD B, A` restores B's *new* value, not A's original value — data loss.

### The three-way truth table

The bug is invisible to the MIR2 VM (which is ABI-agnostic): MIR2 VM says `sum_range(5)=15`, Z80 binary says `sum_range(5)=0`. This is exactly the class of codegen bug that **dual-VM asserts** would catch automatically (see "Future: dual-VM asserts" below).

### Fix

When A is part of the cycle, collect the full set of registers involved and pick the first 8-bit register **not** in the cycle as scratch:

```go
cycleRegs := map[string]bool{}
for i := range moves {
    if !moves[i].done {
        cycleRegs[moves[i].src] = true
        cycleRegs[moves[i].dst] = true
    }
}
scratch := "A"
if cycleRegs["A"] {
    for _, r := range []string{"C", "E", "H", "L", "D", "B"} {
        if !cycleRegs[r] {
            scratch = r
            break
        }
    }
}
```

For A↔B (2-node cycle), scratch=C:

```z80
.trmp0:
    LD C, A      ; C = n  (save A's original value)
    LD A, B      ; A = 0  (acc init was in B)
    LD B, C      ; B = n  (counter)
    JRS .rng_body1
```

The backward-walk algorithm is unchanged — only the scratch register selection is fixed. Handles arbitrary-length cycles correctly.

---

## New Feature: `range(lo..hi)` Iterator Source

### Overview

`range(lo..hi)` is a **counter-based iterator source** — no memory pointer, no `Load` instruction. Elements are the current DJNZ counter value (counting down from `hi-lo` to 1).

```nanz
// sum of 1..n = n*(n+1)/2
fun sum_range(n: u8) -> u8 {
    return range(0..n).fold(0, |acc: u8, i: u8| { return acc + i })
}
```

### Generated Z80

```z80
; fun sum_range(n: u8 = A) -> u8 = A ; clobbers: B, C, F
sum_range:
    LD B, 0              ; acc init = 0
    AND A                ; pre-check: n == 0?
    JRS NZ, .trmp0
    JRS .rng_exit2
.rng_body1:
    ADD A, B             ; acc += counter  ← 1 instruction per iteration
    DJNZ .rng_body1      ; B--; branch if B ≠ 0
    LD B, A
    JP .rng_exit2
.rng_exit2:
    LD A, B
    RET
.trmp0:
    LD C, A              ; save n
    LD A, B              ; A = 0 (acc init)
    LD B, C              ; B = n (counter)
    JRS .rng_body1
```

**Body: 1 instruction per iteration** — `ADD A, B`. No spills, no loads, no stores.

### Architecture

```
range(lo..hi)  →  RangeSourceExpr{Lo, Hi, Rev}   (HIR)
                         │
                   recognizeIterChain()
                         │
                  iterChain{isRange: true, ...}
                         │
              lowerRangeLoop() → TermDJNZ loop
```

`RangeSourceExpr` is a new HIR node that signals "no memory pointer, counter drives iteration". The `iterChain.isRange` flag routes to `lowerRangeFold` / `lowerRangeForEach` instead of the pointer-based `lowerFusedFold` / `lowerFusedForEach`.

### Counting semantics

`range(0..n)` counts DOWN: the DJNZ counter starts at `n` and decrements to 0. Element values are `n, n-1, ..., 1`. This means:

- `range(0..n).fold(0, |acc,i| acc+i)` = sum(1..n) = **n*(n+1)/2**
- `range(n..0)` sets `Rev=true` — direct back-counting, no subtraction needed

This is optimal for the Z80 DJNZ instruction which naturally counts down. Forward-counting (elements 0..n-1) would require a subtract, adding one instruction per iteration.

### forEach variant

```nanz
@extern fun cb(v: u8)

fun call_each(n: u8) -> void {
    range(0..n).forEach(|i: u8| { cb(i) })
}
```

```z80
call_each:
    AND A
    JRS NZ, .trmp0
    JRS .rng_exit2
.rng_body1:
    LD A, B
    CALL cb
    DJNZ .rng_body1
.rng_exit2:
    RET
.trmp0:
    LD B, A
    JRS .rng_body1
```

No trampoline register cycle here (no accumulator) — just `LD B, A` to load the counter.

---

## Test Coverage

| Test | Type | Verifies |
|------|------|---------|
| `TestRangeFold_Compiles` | compile check | DJNZ + ADD A present; no `LD A,B / LD B,A` no-op swap |
| `TestRangeForEach_Compiles` | compile check | DJNZ present, compiles without panic |
| `TestRangeFold_E2E_SumRange` | emulator (MZE) | sum_range(0,1,4,5,10) correct via Z80 execution |

The E2E test assembles the generated asm + a bootstrap, loads into the Z80 emulator, and reads register A at `HALT`:

```
sum_range(0)  = 0  ✓
sum_range(1)  = 1  ✓
sum_range(4)  = 10 ✓   (4+3+2+1)
sum_range(5)  = 15 ✓   (5+4+3+2+1)
sum_range(10) = 55 ✓   (10*(10+1)/2)
```

---

## Future: Dual-VM Asserts

The A↔B swap bug is the canonical example of a bug class that:

- Passes the MIR2 VM (ABI-agnostic, abstract registers)
- Fails the Z80 binary (physical registers, parallel copy matters)

Proposed `assert` levels:

```nanz
assert sum_range(5) == 15            // default: both MIR2 VM + Z80 binary
assert sum_range(5) == 15 via mir2   // fast: skip Z80 assembly
assert sum_range(5) == 15 via z80    // thorough: skip MIR2 (rare)
```

When both run by default, the compiler distinguishes:

| MIR2 | Z80 | Expected | Diagnosis |
|------|-----|----------|-----------|
| 15 | 15 | 15 | ✓ all good |
| 7 | 7 | 15 | algorithm bug |
| 15 | 0 | 15 | **codegen bug** ← the A↔B swap |

Assert blocks with sandbox annotation:

```nanz
@sandbox(mir2, z80)
assert {
    sum_range(0)  == 0
    sum_range(5)  == 15
    sum_range(10) == 55
}
```

---

## Files Changed

| File | Change |
|------|--------|
| `pkg/mir2/constprop.go` | `TermDJNZ` BodyArgs offset fix (`i` → `i+1`) |
| `pkg/mir2/z80codegen.go` | `emitParallelCopy` cycle scratch selection |
| `pkg/hir/hir.go` | `RangeSourceExpr` node |
| `pkg/hir/lower.go` | `lowerRangeLoop`, `lowerRangeFold`, `lowerRangeForEach` |
| `pkg/nanz/parse.go` | `range(lo..hi)` primary expression parsing |
| `pkg/nanz/nanz_test.go` | `TestRangeFold_Compiles`, `TestRangeForEach_Compiles` |
| `pkg/nanz/range_e2e_test.go` | `TestRangeFold_E2E_SumRange` (emulator) |
