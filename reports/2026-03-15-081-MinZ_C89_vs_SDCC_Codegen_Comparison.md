# MinZ C89 Frontend vs SDCC 4.2.0 — Z80 Codegen Comparison

**Date:** 2026-03-15
**Report:** #081
**Compiler versions:** MinZ v0.19.5 (MIR2 backend) vs SDCC 4.2.0 #13081

---

## Methodology

Identical C89 source compiled through both toolchains targeting Z80.
MinZ uses its new C89 frontend (`modernc.org/cc/v4` parser → HIR → MIR2 → Z80`).
SDCC uses its standard Z80 backend with default optimization (`-mz80`).

**Key ABI differences:**
- **MinZ**: PBQP register allocator, returns in HL (16-bit) or A (8-bit), passes up to 3 params in registers
- **SDCC**: returns in DE (16-bit) or A (8-bit), passes 1st param in HL (16-bit) or A (8-bit), 2nd in DE, 3rd+ on stack; `__z88dk_fastcall` for single-param only

---

## Source Code

```c
/* MinZ vs SDCC benchmark — small Z80-relevant functions */

int twice(int x) {
    return x + x;
}

int add(int a, int b) {
    return a + b;
}

int max(int a, int b) {
    if (a > b) return a;
    return b;
}

unsigned char abs_diff(unsigned char a, unsigned char b) {
    if (a > b) return a - b;
    return b - a;
}

int sum_to(int n) {
    int total = 0;
    int i = 0;
    while (i < n) {
        total = total + i;
        i = i + 1;
    }
    return total;
}

unsigned char clamp8(unsigned char val, unsigned char lo, unsigned char hi) {
    if (val < lo) return lo;
    if (val > hi) return hi;
    return val;
}
```

---

## Assembly Output

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

**Verdict:** MinZ −1B. Same `EX DE,HL` overhead in SDCC.

---

### `max` — signed 16-bit maximum

**MinZ (13B):**
```asm
max:
    EX DE, HL           ; 1B
    PUSH HL             ; 1B
    OR A                ; 1B
    SBC HL, DE          ; 2B
    POP HL              ; 1B
    EX DE, HL           ; 1B
    JR C, .then         ; 2B
    LD H, D             ; 1B
    LD L, E             ; 1B
    RET                 ; 1B
.then:
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
    ret  P              ; 1B  (conditional RET!)
    ex   de, hl         ; 1B
    ret                 ; 1B
```

**Verdict:** SDCC −1B. SDCC's `JP PO` overflow trick for signed compare is more compact. MinZ's `PUSH/POP HL` to preserve across `SBC HL,DE` costs 2B extra.

---

### `abs_diff` — unsigned 8-bit absolute difference

**MinZ (10B):**
```asm
abs_diff:
    CP C                ; 1B
    NEG                 ; 2B
    ADD A, C            ; 1B
    JR Z, .ret          ; 2B
    JR C, .ret          ; 2B
.then:
    NEG                 ; 2B
    RET                 ; 1B  (shared with .ret fallthrough? no, .ret below)
.ret:
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

**Verdict:** MinZ −1B. After u8 promotion fix, MinZ generates clean 8-bit code. SDCC is 11B because 2nd param arrives in L (not a register-friendly position), requiring extra `LD` shuffles.

---

### `sum_to` — 16-bit loop with accumulator

**MinZ (28B):**
```asm
sum_to:
    LD DE, 0            ; 3B  (i)
    LD HL, 0            ; 3B  (total)
.head:
    PUSH HL             ; 1B
    OR A                ; 1B
    SBC HL, BC          ; 2B
    POP HL              ; 1B
    JR NC, .trmp        ; 2B
.body:
    EX DE, HL           ; 1B
    ADD HL, DE           ; 1B
    EX DE, HL           ; 1B
    INC HL              ; 1B
    JR .head            ; 2B
.exit:
    RET                 ; 1B
.trmp:
    LD DE, 0            ; 3B  (spurious!)
    LD HL, 0            ; 3B  (spurious!)
    JR .exit            ; 2B
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

**Verdict:** SDCC −3B. MinZ's trampoline re-initializes DE/HL to 0 at loop exit (8B wasted) — a known register allocator bug. Without it, MinZ would be 20B.

---

### `clamp8` — unsigned 8-bit clamp to range

**MinZ (13B):**
```asm
clamp8:
    CP B                ; 1B
    JR NC, .join        ; 2B
.lo:
    LD A, B             ; 1B
    RET                 ; 1B
.join:
    CP C                ; 1B
    JR Z, .ret          ; 2B
    JR C, .ret          ; 2B
.hi:
    LD A, C             ; 1B
    RET                 ; 1B
.ret:
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

**Verdict:** MinZ −17B (57% smaller!). This is the killer advantage: MinZ's PBQP allocator passes all 3 u8 params in A, B, C registers. SDCC can only pass 2 params in registers — the 3rd goes to stack, requiring expensive `IY`-indexed loads (4B per access).

---

## Summary

| Function   | MinZ | SDCC | Delta | Winner |
|------------|-----:|-----:|------:|--------|
| `twice`    |   2B |   3B |   −1B | MinZ   |
| `add`      |   2B |   3B |   −1B | MinZ   |
| `max`      |  13B |  12B |   +1B | SDCC   |
| `abs_diff` |  10B |  11B |   −1B | MinZ   |
| `sum_to`   |  28B |  25B |   +3B | SDCC   |
| `clamp8`   |  13B |  30B |  −17B | MinZ   |
| **TOTAL**  |**68B**|**84B**|**−16B**| **MinZ (−19%)** |

---

## Analysis

### MinZ Strengths
1. **PBQP register allocator** — 3-register u8 calling convention eliminates stack spills for small functions (`clamp8`: 57% savings)
2. **No return-convention overhead** — returns in HL directly, no `EX DE,HL` tax on every 16-bit return
3. **Clean u8 codegen** — after promotion fix, 8-bit ops stay in A register without widening

### MinZ Weaknesses
1. **Loop exit trampoline** — `sum_to` re-initializes registers at loop exit (known regalloc bug, 8B waste)
2. **Signed compare** — SDCC's `JP PO / XOR 0x80` overflow trick for signed comparison is 1B cheaper than MinZ's `PUSH/SBC/POP` pattern

### SDCC Strengths
1. **Mature signed arithmetic** — decades of Z80-specific peephole patterns
2. **Conditional RET** — uses `RET P` / `RET C` to save bytes vs branch-to-RET

### SDCC Weaknesses
1. **3rd param → stack** — no 3-register ABI; IY-indexed stack access costs 4-7B per parameter
2. **DE return convention** — every 16-bit function pays 1B `EX DE,HL` tax

---

## Known Issues (MinZ)

- `sum_to` trampoline: register allocator inserts dead `LD DE,0 / LD HL,0` at loop exit. Tracked in ADR-0006/ADR-0007.
- `abs_diff` codegen uses `NEG / ADD A,C` pattern which is correct but not minimal. Optimal would be `SUB C / JR NC,.done / NEG / .done: RET` (8B vs 10B).

---

*Generated by MinZ C89 frontend benchmarking pipeline.*
