# Report 104 — MinZ C89 vs SDCC 4.2.0: Extended Benchmark

**Date:** 2026-03-20
**Source:** `examples/c89/sdcc_benchmark.c` — identical C89 compiled by both
**Result:** MinZ **33 instructions** vs SDCC **178 instructions** (−81%)

---

## Results

```
Function         MinZ   SDCC  Delta  MinZ technique
──────────       ────   ────  ─────  ──────────────
fib10              2     25   -23    consteval (entire loop folded)
sum10              2     22   -20    consteval (entire loop folded)
fact6              2     28   -26    consteval (entire loop folded)
twice              2      3    -1    per-function ABI (no EX DE,HL)
add                2      3    -1    per-function ABI (no EX DE,HL)
abs_diff           4     11    -7    conditional RET + NEG
clamp              9     22   -13    3-reg ABI + conditional RET
min8               4      8    -4    RET C (carry flag)
demo               2     45   -43    cascading consteval
wrap_min           2      6    -4    tail call JP (no CALL+RET)
wrap_twice         2      5    -3    tail call JP
─────────────────────────────────────────────────────
TOTAL             33    178  -145    81% smaller
```

MinZ wins on **every single function**. No ties.

---

## Technique Breakdown

### 1. Compile-Time Evaluation (consteval) — biggest win

MinZ evaluates entire loops at compile time when arguments are known constants.
SDCC generates full runtime loop code.

```c
int fibonacci(int n) {
    int a = 0, b = 1;
    while (n > 0) { int t = a + b; a = b; b = t; n = n - 1; }
    return a;
}
int fib10(void) { return fibonacci(10); }
```

**MinZ:** `LD HL, 55; RET` — 2 instructions, 4 bytes
**SDCC:** 25 instructions — full loop with registers, jumps, compare

```c
int square(int x) { return x * x; }
int demo(void) { return square(4) + twice(5); }
```

**MinZ:** `LD HL, 26; RET` — cascading consteval: square(4)=16, twice(5)=10, 16+10=26
**SDCC:** 45 instructions — calls square(), calls twice(), adds results at runtime

### 2. Per-Function ABI (PBQP) — no return-convention tax

SDCC returns 16-bit values in DE and passes first arg in HL.
Every function that computes in HL must add `EX DE,HL` before returning.

MinZ uses PBQP to choose optimal register classes per-function:
- `twice(x)` returns in HL directly (no EX)
- `add(a, b)` takes both args and returns in HL (no shuffling)

**MinZ:** `ADD HL, HL; RET` (2 inst)
**SDCC:** `ADD HL, HL; EX DE, HL; RET` (3 inst)

### 3. Conditional Return — Z80 flag-based returns

MinZ uses `RET Z`, `RET C`, `RET NC` for early exits.
SDCC uses `JP Z, label; ... label: RET` (extra jump).

```c
unsigned char min8(unsigned char a, unsigned char b) {
    if (a < b) return a; return b;
}
```

**MinZ (4 inst):**
```z80
min8:
    CP B         ; compare a,b
    RET C        ; if a<b, return a (in A)
    LD A, B      ; else load b
    RET
```

**SDCC (8 inst):** Uses JP, temporary register, extra loads.

### 4. Tail Call Optimization — CALL+RET → JP

When a function's last action is calling another function, MinZ replaces
`CALL target; RET` with `JP target` — saves 1 byte and 17 T-states.

```c
unsigned char wrap_min(unsigned char a, unsigned char b) { return min8(a, b); }
```

**MinZ (2 inst):** `LD B, C; JP min8` — no CALL overhead, no RET
**SDCC (6 inst):** PUSH args, CALL, POP, return

### 5. Three-Register ABI — clamp uses all three

```c
unsigned char clamp(unsigned char val, unsigned char lo, unsigned char hi) {
    if (val < lo) return lo; if (val > hi) return hi; return val;
}
```

**MinZ (9 inst):** val=A, lo=B, hi=C — all in registers, conditional returns
**SDCC (22 inst):** Arguments on stack, load/compare/jump chains

---

## Methodology

- **MinZ:** C89 frontend (`modernc.org/cc/v4` parser → HIR → MIR2 → Z80)
- **SDCC:** Version 4.2.0 #13081, default optimization (`sdcc -mz80`)
- **Counting:** Z80 instructions per function (excluding labels/comments)
- SDCC numbers from manual compilation and disassembly

Both compilers receive **identical C89 source code**.

---

## Why MinZ Wins

| Technique | SDCC equivalent | Advantage |
|-----------|----------------|-----------|
| Consteval | None — SDCC cannot fold loops | Eliminates entire functions |
| PBQP ABI | Fixed DE-return convention | Saves 1 inst per 16-bit function |
| Cond RET | JP cc → label → RET | Saves 2-3 inst per early return |
| Tail call | Not implemented | Saves 2 inst per wrapper |
| 3-reg ABI | 1-2 reg params, rest on stack | Saves 3-8 inst on multi-param |
| Grace DSE | Basic dead store elimination | Removes unused computations |
