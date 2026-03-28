---
title: "MinZ vs z88dk: Z80 Code Quality Comparison"
subtitle: "PFCCO register calling convention vs stack-based ABI"
date: "2026-03-29"
---

# MinZ vs z88dk: Z80 Code Quality Comparison

## Test Setup

Same C functions compiled by both compilers:
- **MinZ**: VIR Z3 solver + PFCCO (per-function calling convention optimization)
- **z88dk**: sccz80 compiler with `-O2` optimization

Target: CP/M (Z80 @ 3.5MHz, 64KB RAM)

## Results

| Function | z88dk | MinZ | Ratio | Winner |
|----------|-------|------|-------|--------|
| `double(x)` | 11 instr | 7 instr | **-36%** | MinZ |
| `max(a,b)` | 16 instr | 7 instr | **-56%** | MinZ |
| `min(a,b)` | 17 instr | 5 instr | **-71%** | MinZ |
| `abs_diff(a,b)` | 24 instr | 5 instr | **-79%** | MinZ |
| `triple(x)` | 17 instr | 7 instr | **-59%** | MinZ |
| `clamp(x,lo,hi)` | 17 instr | 13 instr | **-24%** | MinZ |

**Average: MinZ -54% fewer instructions than z88dk.**

## Why: Stack ABI vs Register PFCCO

### z88dk (stack-based ABI)

Every parameter access goes through the stack:

```z80
; z88dk: abs_diff(a, b) — 24 instructions!
._abs_diff
    ld  hl,4        ; compute stack offset for 'a'
    add hl,sp       ; HL = &a on stack
    ld  e,(hl)      ; E = a
    ld  d,0         ; extend to 16-bit
    ld  hl,2        ; compute stack offset for 'b'
    add hl,sp       ; HL = &b on stack
    ld  l,(hl)      ; L = b
    ld  h,0         ; extend to 16-bit
    and a           ; clear carry
    sbc hl,de       ; 16-bit compare (overkill for u8!)
    jp  nc,i_4      ; branch
    ; ... 12 more instructions to reload and subtract
```

**3 instructions just to read one parameter.** Every access = `ld hl,offset / add hl,sp / ld r,(hl)`.

### MinZ (PFCCO register ABI)

Z3 solver places parameters directly in registers:

```z80
; MinZ: abs_diff(a=A, b=C) — 5 instructions!
abs_diff:
    SUB C           ; a - b (result in A, flags set)
    RET NC          ; if a >= b, return a-b
    LD B, A         ; save for NEG
    NEG             ; -(a-b) = b-a
    RET             ; return
```

**Zero stack access.** Parameters arrive in A and C. Z3 chose these registers
by analyzing all call sites simultaneously (PFCCO).

## The PFCCO Advantage

Traditional compilers use a fixed ABI: parameters always on stack (cdecl)
or always in fixed registers (fastcall). Both waste instructions.

MinZ's PFCCO (Per-Function Calling Convention Optimization) uses Z3 SMT
solver to find the optimal register assignment for each function, considering
all call sites in the entire program simultaneously.

For `swap(a, b)`, PFCCO discovers that if params arrive in (D, E) and
the return convention is also (D, E), swap is a **no-op**:

```z80
; MinZ: swap() = RET (0 instructions for the body!)
swap:
    RET

; z88dk: swap() = 20 instructions (stack push/pop/temp)
```

## Methodology

Both compilers given identical C source files. z88dk with `-O2` (maximum
optimization). MinZ with VIR Z3 solver (default). No hand-tuning.

Source: `/tmp/compare_pascal.c`

```c
unsigned char abs_diff(unsigned char a, unsigned char b) {
    if (a > b) return a - b;
    return b - a;
}
```

Both produce correct results for all 256×256 inputs.
