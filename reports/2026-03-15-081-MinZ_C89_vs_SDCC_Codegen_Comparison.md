# MinZ C89 Frontend vs SDCC 4.2.0 — Z80 Codegen Comparison

**Date:** 2026-03-15 (updated)
**Report:** #081
**Compiler versions:** MinZ v0.19.6 (MIR2 backend) vs SDCC 4.2.0 #13081

---

## Results at a Glance

Assembled binary sizes (code only, no headers/vectors):

| Function | MinZ C89 | SDCC 4.2.0 | Delta | Notes |
|----------|-------:|-------:|------:|-------|
| `twice(i16)→i16` | 2B | 3B | −1B | SDCC: `EX DE,HL` return tax |
| `add(i16,i16)→i16` | 2B | 3B | −1B | SDCC: `EX DE,HL` return tax |
| `max(i16,i16)→i16` | 12B | 12B | TIE | Both use clever compare tricks |
| `abs_diff(u8,u8)→u8` | 9B | 11B | −2B | MinZ: `RET Z/RET C` conditional return |
| `sum_to(i16)→i16` | 21B | 25B | −4B | MinZ: no trampoline, `ADD HL` restore |
| `clamp8(u8,u8,u8)→u8` | 10B | 30B | −20B | MinZ: 3-reg ABI + `RET Z/C` |
| **Scalar total** | **56B** | **84B** | **−33%** | |
| | | | | |
| `minmax(u16,u16)→(u16,u16)` | 19B | 61B | −42B | MinZ: tuple return + `RET C/Z` |
| `smaller` (uses lo) | 0B | 34B | −34B | MinZ: `EQU minmax` (degenerate!) |
| `larger` (uses hi) | 6B | — | — | |
| **Pair return total** | **25B** | **95B** | **−74%** | |
| | | | | |
| **GRAND TOTAL** | **81B** | **179B** | **−55%** | |

**Verified**: Nanz native and C89/MinZ produce **byte-for-byte identical** Z80 binary code for **pair-return functions** (minmax, smaller, larger). Scalar functions use the same pipeline but may differ in HIR lowering between frontends.

```
Binary verification (xxd, minmax code section):
Nanz: e5b7 ed52 e1d5 c1e5 dde1 d8c8 6069 dd54 dd5d c9
C89:  e5b7 ed52 e1d5 c1e5 dde1 d8c8 6069 dd54 dd5d c9
      ^^^^ identical ^^^^
```

---

## Methodology

Identical C89 source compiled through both toolchains targeting Z80.
MinZ uses its new C89 frontend (`modernc.org/cc/v4` parser → HIR → MIR2 → Z80`).
SDCC uses its standard Z80 backend with default optimization (`-mz80`).

**Key ABI differences:**
- **MinZ**: Per-function ABI via PBQP + PFCCO, returns in HL (16-bit) or A (8-bit), passes up to 3 params in registers, **multi-return** (HL + DE)
- **SDCC**: Fixed ABI, returns in DE (16-bit) or A (8-bit), passes 1st param in HL, 2nd in DE, 3rd+ on stack; **no multi-return** (requires pointer-based out-params)

---

## Part 1: Scalar Functions (bench.c)

### Source Code

```c
int twice(int x) { return x + x; }
int add(int a, int b) { return a + b; }
int max(int a, int b) { if (a > b) return a; return b; }
unsigned char abs_diff(unsigned char a, unsigned char b) {
    if (a > b) return a - b; return b - a;
}
int sum_to(int n) {
    int total = 0; int i = 0;
    while (i < n) { total = total + i; i = i + 1; }
    return total;
}
unsigned char clamp8(unsigned char val, unsigned char lo, unsigned char hi) {
    if (val < lo) return lo; if (val > hi) return hi; return val;
}
```

---

### `twice` — double a 16-bit value

**MinZ (2B):**
```asm
twice:
    ADD HL, HL          ; 1B
    RET                 ; 1B
```

**SDCC (3B):**
```asm
_twice::
    add  hl, hl         ; 1B
    ex   de, hl         ; 1B  (return convention: DE)
    ret                 ; 1B
```

**Verdict:** MinZ −1B. SDCC wastes 1B on `EX DE,HL` because it returns 16-bit in DE.

---

### `add` — add two 16-bit values

**MinZ (2B):**
```asm
add:
    ADD HL, DE          ; 1B
    RET                 ; 1B
```

**SDCC (3B):**
```asm
_add::
    add  hl, de         ; 1B
    ex   de, hl         ; 1B
    ret                 ; 1B
```

**Verdict:** MinZ −1B. Same `EX DE,HL` overhead.

---

### `max` — signed 16-bit maximum

**MinZ (12B):**
```asm
max:
    OR A                ; 1B  ← ADD HL,DE restore (no PUSH/POP needed)
    SBC HL, DE          ; 2B
    ADD HL, DE          ; 1B  ← restores HL without PUSH/POP
    JR Z, .cret0        ; 2B
    JR C, .cret0        ; 2B
    RET                 ; 1B
.cret0:
    LD H, D             ; 1B
    LD L, E             ; 1B
    RET                 ; 1B
```

**SDCC (12B):**
```asm
_max::
    ld   a, e           ; 1B
    sub  a, l           ; 1B
    ld   a, d           ; 1B
    sbc  a, h           ; 1B
    jp   PO, 00110$     ; 3B
    xor  a, #0x80       ; 2B
00110$:
    ret  P              ; 1B  (conditional RET)
    ex   de, hl         ; 1B
    ret                 ; 1B
```

**Verdict:** TIE at 12B. MinZ's `SBC HL,DE / ADD HL,DE` restore trick matches SDCC's `JP PO / XOR 0x80` signed overflow trick — both arrive at 12B through entirely different strategies. Previously MinZ was 13B (`PUSH/POP` pattern); the `ADD HL,rr` restore optimization saved 1B.

---

### `abs_diff` — unsigned 8-bit absolute difference

**MinZ (9B):**
```asm
abs_diff:
    CP C                ; 1B
    NEG                 ; 2B
    ADD A, C            ; 1B
    RET Z               ; 1B  ← conditional return (was JR Z)
    RET C               ; 1B  ← conditional return (was JR C)
    NEG                 ; 2B
    RET                 ; 1B
```

**SDCC (11B):**
```asm
_abs_diff::
    ld   c, a           ; 1B
    ld   a, l           ; 1B
    sub  a, c           ; 1B
    jr   NC, 00102$     ; 2B
    ld   a, c           ; 1B
    sub  a, l           ; 1B
    ret                 ; 1B
00102$:
    ld   a, l           ; 1B
    sub  a, c           ; 1B
    ret                 ; 1B
```

**Verdict:** MinZ −2B. The `elimJrToRet` peephole replaces `JR Z/JR C` to bare `RET` with `RET Z/RET C` conditional returns, saving 3B (12B→9B). Optimal codegen would be `SUB C / RET NC / NEG / RET` at 6B — room for improvement.

---

### `sum_to` — 16-bit loop with accumulator

**MinZ (21B):**
```asm
sum_to:
    LD DE, 0            ; 3B  (i=0)
    LD HL, 0            ; 3B  (total=0)
.head:
    OR A                ; 1B
    SBC HL, BC          ; 2B
    ADD HL, BC          ; 1B  ← restore without PUSH/POP
    JR NC, .exit        ; 2B
    EX DE, HL           ; 1B
    ADD HL, DE           ; 1B
    EX DE, HL           ; 1B
    INC HL              ; 1B
    JR .head            ; 2B
.exit:
    LD H, D             ; 1B
    LD L, E             ; 1B
    RET                 ; 1B
```

**SDCC (25B):**
```asm
_sum_to::
    ex   de, hl         ; 1B
    ld   hl, #0x0000    ; 3B  (total)
    ld   bc, #0x0000    ; 3B  (i)
00101$:
    ld   a, c           ; 1B
    sub  a, e           ; 1B
    ld   a, b           ; 1B
    sbc  a, d           ; 1B
    jp   PO, 00117$     ; 3B
    xor  a, #0x80       ; 2B
00117$:
    jp   P, 00103$      ; 3B
    add  hl, bc         ; 1B
    inc  bc             ; 1B
    jr   00101$         ; 2B
00103$:
    ex   de, hl         ; 1B
    ret                 ; 1B
```

**Verdict:** MinZ −4B! Previously MinZ was 28B (trampoline bug), now 21B — cleaner than SDCC's 25B. SDCC's signed loop comparison costs `JP PO + XOR 0x80 + JP P` = 8B per iteration; MinZ's unsigned path is 5B (`OR A + SBC + ADD + JR NC`).

---

### `clamp8` — unsigned 8-bit clamp to range

**MinZ (10B):**
```asm
clamp8:
    CP B                ; 1B
    JR NC, .join        ; 2B
    LD A, B             ; 1B
    RET                 ; 1B
.join:
    CP C                ; 1B
    RET Z               ; 1B  ← conditional return (was JR Z)
    RET C               ; 1B  ← conditional return (was JR C)
    LD A, C             ; 1B
    RET                 ; 1B
```

**SDCC (30B):**
```asm
_clamp8::
    ld   c, a           ; 1B
    sub  a, l           ; 1B
    jr   NC, 00102$     ; 2B
    ld   a, l           ; 1B
    jr   00105$         ; 2B
00102$:
    ld   hl, #2         ; 3B  (stack access!)
    add  hl, sp         ; 1B
    ld   a, (hl)        ; 1B
    sub  a, c           ; 1B
    jr   NC, 00104$     ; 2B
    ld   iy, #2         ; 4B  (IY for stack!)
    add  iy, sp         ; 2B
    ld   a, 0 (iy)      ; 3B
    jr   00105$         ; 2B
00104$:
    ld   a, c           ; 1B
00105$:
    pop  hl             ; 1B
    inc  sp             ; 1B
    jp   (hl)           ; 1B
```

**Verdict:** MinZ −20B (67% smaller!). The killer advantage: MinZ's PBQP allocator passes all 3 u8 params in A, B, C registers. SDCC can only pass 2 params in registers — the 3rd goes to stack, requiring expensive `IY`-indexed loads (4B per access). Plus `elimJrToRet` replaces `JR Z/JR C` → `RET Z/RET C`, saving 3B.

---

### Scalar Summary

| Function   | v0.19.5 | v0.19.6 | **current** | SDCC | Winner |
|------------|-----:|-----:|-----:|-----:|--------|
| `twice`    |   2B |   2B |   2B |   3B | MinZ −1B |
| `add`      |   2B |   2B |   2B |   3B | MinZ −1B |
| `max`      |  13B |  13B | **12B** |  12B | **TIE** (was SDCC) |
| `abs_diff` |  10B |  10B | **9B** |  11B | **MinZ −2B** (was SDCC) |
| `sum_to`   |  28B |  22B | **21B** |  25B | **MinZ −4B** |
| `clamp8`   |  13B |  13B | **10B** |  30B | **MinZ −20B** |
| **TOTAL**  |**68B**|**62B**|**56B**|**84B**| **MinZ −33%** |

Key improvements since v0.19.5:
- `max`: `PUSH/POP` → `ADD HL,DE` restore (−1B), now TIES SDCC
- `sum_to`: dead trampoline eliminated (−7B), now BEATS SDCC by 4B
- `abs_diff`: `elimJrToRet` peephole (−3B), now BEATS SDCC by 2B
- `clamp8`: `elimJrToRet` peephole (−3B), 67% smaller than SDCC
- Score: MinZ wins 4 functions, SDCC wins 0, 1 tie, 1 near-tie

---

## Part 2: Pair Return — minmax(a, b) → (lo, hi)

This is the flagship comparison: struct/pair returns demonstrate MinZ's per-function ABI advantage over C's lack of multi-return.

### Source Code

**Nanz (native):**
```nanz
fun minmax(a: u16, b: u16) -> (u16, u16) {
    if a <= b { return (a, b) }
    return (b, a)
}
fun smaller(x: u16, y: u16) -> u16 {
    let (lo, _) = minmax(x, y)
    return lo
}
fun larger(x: u16, y: u16) -> u16 {
    let (_, hi) = minmax(x, y)
    return hi
}
```

**C89 (via MinZ C89 frontend, struct→tuple promotion):**
```c
typedef struct { uint16_t lo; uint16_t hi; } Pair;

Pair minmax(uint16_t a, uint16_t b) {
    if (a <= b) { Pair r = { a, b }; return r; }
    Pair r = { b, a };
    return r;
}
uint16_t smaller(uint16_t a, uint16_t b) {
    Pair p = minmax(a, b);
    return p.lo;
}
uint16_t larger(uint16_t a, uint16_t b) {
    Pair p = minmax(a, b);
    return p.hi;
}
```

**SDCC (pointer-based, no multi-return in C):**
```c
void minmax(uint16_t a, uint16_t b, uint16_t *lo, uint16_t *hi) {
    if (a <= b) { *lo = a; *hi = b; }
    else        { *lo = b; *hi = a; }
}
uint16_t min_of(uint16_t a, uint16_t b) {
    uint16_t lo, hi;
    minmax(a, b, &lo, &hi);
    return lo;
}
```

---

### Assembly Comparison

#### `minmax` body

**Nanz AND C89/MinZ — IDENTICAL output (19B):**
```asm
; fun minmax(a: u16 = HL, b: u16 = DE) -> (u16 = HL, u16 = DE)
minmax:
    PUSH HL             ; 1B  save a
    OR A                ; 1B  clear carry
    SBC HL, DE          ; 2B  a - b (compare)
    POP HL              ; 1B  restore a
    PUSH DE             ; 1B  save b
    POP BC              ; 1B  BC = b
    PUSH HL             ; 1B  save a
    POP IX              ; 2B  IX = a
    RET C               ; 1B  a < b → return (a, b) ← was JR C
    RET Z               ; 1B  a == b → return (a, b) ← was JR Z
    LD H, B             ; 1B  ┐ swap: HL = b
    LD L, C             ; 1B  ┘
    LD D, IXH           ; 2B  ┐ DE = a
    LD E, IXL           ; 2B  ┘
    RET                 ; 1B  return (b, a)
```

**SDCC `_minmax` (61B):**
```asm
_minmax::
    push ix             ; 2B  ┐
    ld   ix, #0         ; 4B  │ frame setup (8B overhead!)
    add  ix, sp         ; 2B  ┘
    push af             ; 1B
    ld   c, l           ; 1B  ┐ save params from registers to C,B
    ld   b, h           ; 1B  ┘
    ld   l, 4 (ix)      ; 3B  ┐ load pointer params from stack
    ld   h, 5 (ix)      ; 3B  │ (3rd and 4th params = *lo, *hi)
    ld   a, 6 (ix)      ; 3B  │
    ld   -2 (ix), a     ; 3B  │
    ld   a, 7 (ix)      ; 3B  │
    ld   -1 (ix), a     ; 3B  ┘
    ld   a, e           ; 1B  ┐ compare b vs a
    sub  a, c           ; 1B  │
    ld   a, d           ; 1B  │
    sbc  a, b           ; 1B  ┘
    jr   C, 00102$      ; 2B
    ld   (hl), c        ; 1B  ┐ *lo = a (write through pointer)
    inc  hl             ; 1B  │
    ld   (hl), b        ; 1B  ┘
    pop  hl             ; 1B  ┐ *hi = b (write through pointer)
    push hl             ; 1B  │
    ld   (hl), e        ; 1B  │
    inc  hl             ; 1B  │
    ld   (hl), d        ; 1B  ┘
    jr   00104$         ; 2B
00102$:                         ; else branch: swap
    ld   (hl), e        ; 1B
    inc  hl             ; 1B
    ld   (hl), d        ; 1B
    pop  hl             ; 1B
    push hl             ; 1B
    ld   (hl), c        ; 1B
    inc  hl             ; 1B
    ld   (hl), b        ; 1B
00104$:
    ld   sp, ix         ; 2B  ┐ frame cleanup (7B overhead!)
    pop  ix             ; 2B  │
    pop  hl             ; 1B  │
    pop  af             ; 1B  │
    pop  af             ; 1B  ┘
    jp   (hl)           ; 1B  return via stack (no direct RET!)
```

#### `smaller` / `min_of`

**Nanz AND C89/MinZ — IDENTICAL (0B, degenerate):**
```asm
; fun smaller(a: u16 = HL, b: u16 = DE) -> u16 = HL
smaller    EQU minmax
```

PFCCO detects that `smaller` only uses the first return value (HL), which is exactly where `minmax` already returns it. The function is **eliminated entirely** — zero bytes, zero overhead.

**SDCC `_min_of` (34B):**
```asm
_min_of::
    push ix             ; 2B  ┐ frame setup
    ld   ix, #0         ; 4B  │
    add  ix, sp         ; 2B  ┘
    push af             ; 1B
    push af             ; 1B
    ld   c, l           ; 1B
    ld   b, h           ; 1B
    ld   hl, #2         ; 3B  ┐ compute &hi on stack
    add  hl, sp         ; 1B  ┘
    push hl             ; 1B
    ld   hl, #2         ; 3B  ┐ compute &lo on stack
    add  hl, sp         ; 1B  ┘
    push hl             ; 1B
    ld   l, c           ; 1B  ┐ restore params
    ld   h, b           ; 1B  ┘
    call _minmax        ; 3B  call
    pop  de             ; 1B  ┐ read result from stack
    push de             ; 1B  ┘
    ld   sp, ix         ; 2B  ┐ frame cleanup
    pop  ix             ; 2B  ┘
    ret                 ; 1B
```

#### `larger`

**Nanz AND C89/MinZ — IDENTICAL (6B):**
```asm
larger:
    CALL minmax         ; 3B
    LD H, D             ; 1B  move 2nd return (DE) → HL
    LD L, E             ; 1B
    RET                 ; 1B
```

---

### Pair Return Summary

| Function | Nanz | C89/MinZ | SDCC | MinZ/SDCC ratio |
|----------|-----:|-----:|-----:|-----:|
| `minmax` body | 19B | **19B** | 61B | **3.2× smaller** |
| `smaller`/`min_of` | 0B (EQU) | **0B** (EQU) | 34B | **∞× smaller** |
| `larger` | 6B | **6B** | — | — |
| **TOTAL** | **25B** | **25B** | **95B** | **3.8× smaller** |

**Key finding: C89 through MinZ generates BYTE-FOR-BYTE IDENTICAL Z80 binary as native Nanz for pair-return functions** (verified via `xxd` and `cmp`).

The struct→tuple promotion pass (ADR-0025) completely erases the semantic gap between C's `typedef struct { ... } Pair` and Nanz's native `(u16, u16)` multi-return.

---

### How Struct→Tuple Promotion Works

The promotion pass (`pkg/c89/promote.go`) transforms C struct patterns into HIR tuple returns in 4 phases:

```
C source                          HIR (after promotion)
─────────────────────────         ──────────────────────
Pair minmax(u16 a, u16 b) {       fun minmax(a, b) -> (u16, u16) {
  if (a <= b) {                     if a <= b {
    Pair r = { a, b };    ────→       return (a, b)     // tuple!
    return r;                       }
  }                                 return (b, a)       // tuple!
  Pair r = { b, a };      ────→   }
  return r;
}

u16 smaller(u16 a, u16 b) {      fun smaller(a, b) -> u16 {
  Pair p = minmax(a, b);  ────→     let (p_lo, p_hi) = minmax(a, b)
  return p.lo;             ────→     return p_lo
}                                 }
```

**4 supported patterns:**
1. Direct struct literal return: `return (Pair){a, b};`
2. Indirect via brace init: `Pair r = {a, b}; return r;`
3. Out-param: `void f(..., Pair *out) { out->lo = a; out->hi = b; }`
4. Pointer return: `Pair *f() { tmp.lo = a; tmp.hi = b; return &tmp; }`

**Dead struct decl elimination** removes leftover `VarDeclStmt` with `StructLitExpr` init after promotion, preventing unnecessary stack allocation in codegen.

**Scoped variable resolution** correctly handles if/else branches that declare the same variable name with different struct literal values.

---

## Combined Summary

| Category | MinZ | SDCC | MinZ advantage |
|----------|-----:|-----:|-----:|
| 6 scalar functions | 56B | 84B | **−33%** |
| minmax pair return | 25B | 95B | **−74%** (3.8× smaller) |
| **ALL 9 functions** | **81B** | **179B** | **−55%** |

---

## Analysis

### MinZ Strengths
1. **Per-function ABI (PFCCO)** — each function gets optimal register assignment; no fixed convention overhead
2. **Multi-return via tuple** — `(HL, DE)` pair return eliminates pointer-based out-params entirely
3. **PBQP register allocator** — 3-register u8 calling convention eliminates stack spills (`clamp8`: 57% savings)
4. **Degenerate function elimination** — `smaller EQU minmax` = 0 bytes when caller's return register matches callee's
5. **Struct→tuple promotion** — C struct returns automatically promoted to register-based tuple returns
6. **`ADD HL,rr` restore** — avoids `PUSH/POP` around `SBC HL,DE` compare (safe: `ADD HL,rr` doesn't touch S/Z/PV flags)
7. **`elimJrToRet` peephole** — replaces `JR cc, .label` → bare `RET` with `RET cc` (1B vs 2B+1B), then removes dead label+RET pairs

### MinZ Weaknesses
1. **`abs_diff` codegen** — `NEG / ADD A,C` pattern (9B) vs optimal `SUB C / RET NC / NEG / RET` (6B)
2. **IX usage in `minmax`** — `PUSH HL / POP IX` for temporary storage costs 3B; optimal hand-written code could use register pairs more efficiently

### SDCC Strengths
1. **Mature signed arithmetic** — decades of Z80-specific peephole patterns (`JP PO / XOR 0x80`)

### SDCC Weaknesses
1. **No multi-return** — C language limitation forces pointer-based out-params (+50B for minmax!)
2. **3rd param → stack** — no 3-register ABI; IY-indexed stack access costs 4-7B per parameter
3. **DE return convention** — every 16-bit function pays 1B `EX DE,HL` tax
4. **Frame setup overhead** — `PUSH IX / LD IX,0 / ADD IX,SP` = 8B per function with stack params

---

## Conclusion

MinZ's C89 frontend produces Z80 code that is **byte-for-byte identical** to native Nanz for pair-return functions, and **33% smaller overall** for scalar functions. The struct→tuple promotion pass completely bridges the gap between C's lack of multi-return and Nanz's native tuple support.

For the full 9-function benchmark, MinZ is **55% smaller** than SDCC (81B vs 179B) — not because of better peephole optimization, but because of a fundamentally better ABI design: per-function register contracts, multi-return values, and conditional-return peepholes eliminate the structural overhead that C compilers cannot avoid.

**The implication:** C code compiled through MinZ's frontend can achieve the same codegen quality as a purpose-built language — the ABI advantage is in the backend, not the frontend.

---

*Generated by MinZ C89 frontend benchmarking pipeline. All byte counts verified from actual compiler output.*
