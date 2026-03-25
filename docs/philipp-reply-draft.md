# Reply to Philipp Klaus Krause — Draft

Dear Philipp,

Thank you very much for the detailed feedback and the follow-up on sum_array — every point is helpful and will improve the paper. We've re-compiled all examples and verified the results since your first message.

## SDCC version (4.2.0 → 4.5.0)

We'll update the comparison to SDCC 4.5.0. As you note, the calling convention hasn't changed, so the PFCCO comparison should hold. The mos6502 backend in 4.5.0/4.6.0 is very interesting — we do have a 6502 backend and adding SDCC 6502 as a second comparison point would strengthen the evaluation.

## Nanz citation

You're right. The compiler is open-source:

> MinZ/Nanz Compiler. https://github.com/oisee/minz — Multi-frontend optimizing compiler for Z80, 6502, and other 8-bit architectures. ~90K LOC Go, 8 language frontends, shared HIR→MIR→Z80 pipeline with PFCCO, PBQP register allocation, and Z3 SMT-based optimal allocation.

## "17x smaller" terminology

Agreed — we'll change to "code size ratio SDCC:Nanz = 17:1" or "SDCC output is 17 times as large" throughout.

## Multi-return: struct vs pointer

We tested both approaches in SDCC. On the Nanz side, the compiler performs **struct-return promotion**: when a returned struct is immediately destructured (all fields read, struct never stored or address-taken), it is promoted to a tuple return where each field maps to a register via PFCCO.

We've also identified **output-parameter promotion**: functions like `void swap(uint8_t a, uint8_t b, uint8_t *out_a, uint8_t *out_b)` where the pointers are only written to (never read through) can be transformed to `(u8, u8) swap(u8 a, u8 b)`. Our compiler detects this pattern at IR level via write-only analysis.

Here is the verified comparison for swap:

```
SDCC 4.2.0 — void swap(a, b, *out_a, *out_b):
    PUSH IX           ; frame setup
    LD IX, #0         ;
    ADD IX, SP        ;
    LD A, 4(IX)       ; load a from stack
    LD 8(IX), A       ; *out_a = a (via stack pointer)
    LD A, 5(IX)       ; load b from stack
    LD 6(IX), A       ; *out_b = b (via stack pointer)
    ...               ; frame teardown
    Total: 20 instructions, all calling-convention overhead

Nanz — fun swap(a: u8, b: u8) -> (u8, u8):
    RET               ; 0 real instructions!
    Total: 0 instructions — PFCCO placed a in C, b in A,
    return (b,a) = (A, C) — already in correct positions
```

The entire SDCC output is ABI overhead. With PFCCO, the swap is free.

## Separate compilation and function pointers

Your point about SDCC's compilation model is important. Our system handles this:

**Pinned conventions.** Functions with `@extern` or explicit register annotations get a fixed convention. PFCCO only optimizes internal functions.

**Function pointers.** When a function's address is taken, PFCCO computes a weighted-average optimal convention across all possible indirect call targets. All functions assigned to the same pointer variable share this convention. This is more nuanced than a single global convention but respects the constraint that indirect calls must agree.

**Static functions.** You mention that SDCC could optimize `static` functions whose address is not taken — this is exactly the scope where PFCCO operates. Our whole-program approach is equivalent to LTO-style visibility: internal = optimizable, external = fixed.

## Inlining vs. PFCCO

We believe PFCCO and inlining are complementary:

**First**, even after inlining, PFCCO-style register assignment helps. When B is inlined into A, B's parameters become A's local variables. If PFCCO assigned them to non-conflicting registers, the inlined code runs without extra spills.

**Second**, on Z80 with 7 GPR, aggressive inlining increases register pressure rapidly. PFCCO allows a `CALL` to be cheaper than inlining when the callee's convention already matches the caller's register state. Our verified results:

| Function | SDCC 4.2.0 | Nanz PFCCO | Ratio |
|----------|-----------|-----------|-------|
| swap | 20 inst | 0 inst | 20:0 |
| abs_diff | 13 inst | 4 inst | 3.25:1 |
| fib (recursive) | 23 inst | 12 inst | 1.9:1 |
| gcd (while loop) | 20 inst | 9 inst | 2.2:1 |
| minmax (multi-return) | 63 inst | 11 inst | 5.7:1 |

These are from real compilations with SDCC 4.2.0 and our VIR backend (Z3-based joint isel+regalloc).

## sum_array / do-while comparison

Your rewrite is fair — the `n > 0` guard adds overhead unrelated to the calling convention. We'll use the do-while form. Your SDCC trunk output:

```z80
; SDCC trunk (your do-while):
xor a, a
ld iy, #2          ; ← calling convention: load n from stack
add iy, sp         ;
ld b, 0 (iy)       ;
00101$:
ld c, (hl)
inc hl
add a, c
djnz 00101$
pop hl             ; ← calling convention: return via stack
inc sp             ;
jp (hl)            ;

; Nanz with PFCCO (buf=HL, n=B, result=A):
XOR A
.loop:
ADD A, (HL)
INC HL
DJNZ .loop
RET                ; 6 bytes, 5 instructions
```

The loop bodies are identical — both compilers find `DJNZ`. The entire difference is calling convention: `IY` stack frame setup (3 inst), `POP HL / INC SP / JP (HL)` return (3 inst). With PFCCO: buf arrives in HL, n in B, result returns in A. No stack frame.

---

Thank you again for the thorough review. The compiler has grown since the draft — we now have 8 frontends, Z3 SMT-based allocation (joint isel+regalloc), GPU-precomputed optimal register tables, and 95%+ E2E pass rate. We'd be happy to share the updated paper when the revision is ready.

Best regards,
Alice
