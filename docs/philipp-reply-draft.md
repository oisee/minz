# Reply to Philipp Klaus Krause — Draft

Dear Philipp,

Thank you very much for the detailed feedback — every point is helpful and will improve the paper.

## SDCC version (4.2.0 → 4.5.0)

We'll update the comparison to SDCC 4.5.0. As you note, the calling convention hasn't changed, so the PFCCO comparison should hold. The mos6502 backend in 4.5.0/4.6.0 is very interesting — we do have a 6502 backend and adding SDCC 6502 as a second comparison point would strengthen the evaluation. We'll look into this for the revision.

## Nanz citation

You're right — the paper needs a proper citation. The compiler is open-source:

> MinZ/Nanz Compiler. https://github.com/oisee/minz — Multi-frontend optimizing compiler for Z80, 6502, and other 8-bit architectures. ~90K LOC Go, 8 language frontends (Nanz, C89, Pascal, PL/M, ABAP, Frill, Lizp, Lanz), shared HIR→MIR2→Z80 pipeline with PFCCO, PBQP register allocation, and Z3 SMT-based optimal allocation.

We'll add this as a citation in the revision.

## "17x smaller" terminology

Agreed — this is mathematically imprecise. We'll change to "code size ratio SDCC:Nanz = 17:1" or "SDCC output is 17 times as large" throughout.

## Multi-return: struct vs pointer

To clarify what was compared: we tested both approaches in SDCC — struct return (hidden pointer argument) and explicit output pointer parameters. On the Nanz side, the compiler performs a **struct-return promotion** pass: when a returned struct is immediately destructured by the caller (all fields read, struct itself never stored or address-taken), the struct is promoted to a tuple return where each field maps to a separate register via PFCCO.

We've also identified a related optimization for **output pointer parameters** — the pattern `void f(uint8_t *out_a, uint8_t *out_b)` where the function only writes through the pointers, never reads. At HIR level we can prove these are write-only, making the true contract equivalent to `(u8, u8) f()` — eliminating pointer indirection entirely. This is not yet implemented but is architecturally straightforward given our HIR analysis infrastructure.

We'll state both approaches more clearly in the revision.

## Separate compilation and function pointers

Important practical point. Our system addresses this through three mechanisms:

**Pinned conventions.** Functions with `@extern` annotations or explicit register constraints get a fixed calling convention that PFCCO cannot change. This is the equivalent of your observation about hand-written asm and pre-compiled libraries — any ABI-boundary function has a locked convention, and optimization proceeds only within the compiler-controlled interior.

**Function pointers.** When a function's address is taken, PFCCO computes a **weighted-average optimal convention** across all possible targets of that function pointer, weighted by estimated call frequency. All functions assignable to the same pointer variable share this computed convention. This is more nuanced than a single global convention but less flexible than per-function — it's the optimal trade-off given the constraint that indirect calls must agree on convention.

**Separate compilation boundary.** We agree this fundamentally limits PFCCO in C's compilation model. Our approach is whole-program — more like LTO than traditional separate compilation. For SDCC, your observation is exactly right: `static` functions whose address is not taken would be the natural scope for per-function optimization. This is a smaller scope than whole-program but still captures the most impactful cases.

## Inlining vs. PFCCO

This is a subtle point and we should address it more carefully in the paper. Our argument is that PFCCO and inlining are **complementary**, not competing:

**First**, even after inlining, PFCCO-style register assignment helps. When function B is inlined into A, B's parameters become A's local variables. If PFCCO has already assigned B's parameters to registers that don't conflict with A's live variables, the inlined code runs without spills. Without PFCCO, the inliner must either insert register saves (adding overhead that partially negates the inlining benefit) or hope the register allocator finds a good assignment post-hoc.

**Second**, on Z80 with 7 GPR, aggressive inlining rapidly increases register pressure. A caller with 3 live values calling a function with 3 parameters creates 6 simultaneously live values when inlined — nearly exhausting the register file. PFCCO's inter-procedural contract means that sometimes a `CALL` is cheaper than inlining: if the callee's convention places parameters exactly where the caller already has them, the call overhead is zero (just `CALL` + `RET` = 27T) while inlining would require spills.

We'll add a section discussing this trade-off with concrete examples.

## sum_chain / do-while comparison

Your rewrite is fair — the `n > 0` guard in our original C source adds overhead unrelated to the calling convention. We'll use the do-while form for a cleaner comparison.

Your SDCC trunk output is better than 4.2.0 — the DJNZ loop is tight. The remaining difference is exactly calling convention overhead:

```z80
; SDCC trunk (your do-while):
xor a, a
ld iy, #2          ; ← convention: load n from stack
add iy, sp         ;
ld b, 0 (iy)       ;
00101$:
ld c, (hl)
inc hl
add a, c
djnz 00101$
pop hl             ; ← convention: return via stack
inc sp             ;
jp (hl)            ;

; Nanz with PFCCO (buf=HL, n=B, result=A):
XOR A
.loop:
ADD A, (HL)
INC HL
DJNZ .loop
RET                ; ← 6 bytes, 5 instructions
```

The difference — `IY` stack frame setup, `POP HL / INC SP / JP (HL)` return — is pure calling convention cost. With PFCCO, `buf` arrives in HL, `n` in B, result returns in A. No stack frame, no shuffling.

We acknowledge your point that this is a particularly favorable example for PFCCO. In the revision we'll include larger functions where the convention overhead is a smaller fraction of total code size, to give a more balanced picture.

---

Thank you again for the thorough review. The compiler continues to evolve — we now have Z3 SMT-based optimal register allocation alongside PFCCO, equality saturation for instruction combining, and 95% pass rate on our E2E test corpus across 8 language frontends. We'd be happy to share updated results when the revision is ready.

Best regards,
Alice
