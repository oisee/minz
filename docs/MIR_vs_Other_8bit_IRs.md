# MinZ MIR vs Other 8-Bit Compiler IRs

How MinZ MIR compares with SDCC iCode, cc65, z88dk, QBE, and ACK — and what each does better.

---

## The Landscape

There are surprisingly few serious IRs for 8-bit targets. Most 8-bit compilers either skip the IR entirely (direct AST-to-assembly) or use minimal internal representations. MinZ MIR is unusual in having a rich, explicitly-defined IR with a VM and a multi-pass optimizer.

| IR | Compiler | Targets | Form | Opcodes | Passes | VM? |
|----|----------|---------|------|---------|--------|-----|
| **MinZ MIR** | mz | Z80, 6502, i8080, GB | Register, flat list | 118 | 13+ | Yes |
| **SDCC iCode** | sdcc | Z80, 8051, STM8, HC08, PIC | Tree → linked list | ~80 | ~10 | No |
| **cc65 pseudo-ops** | cc65 | 6502 | Pseudo-assembly | ~60 | Peephole only | No |
| **z88dk (sccz80)** | sccz80 | Z80 | Direct codegen | N/A | Minimal | No |
| **z88dk (zsdcc)** | zsdcc | Z80 | iCode (SDCC fork) | ~80 | ~10 | No |
| **QBE IL** | qbe | x86-64, aarch64, riscv64 | SSA, basic blocks | ~40 | ~5 | No |
| **ACK EM** | ack | Z80, 6502, 8086, 68000 | Stack machine | ~140 | ~8 | Yes* |

*ACK had an EM interpreter for testing/debugging.

---

## Design Philosophy Comparison

### SDCC iCode: Language-Agnostic Pragmatism

SDCC models C semantics in a target-neutral way. iCode nodes represent C operations (assignment, arithmetic, comparison, cast) with symbol-based operands. The backend translates iCode to target assembly.

```
; SDCC iCode for: int a = b + c;
iTemp0 [lr0:1] = b [lr1:3] + c [lr2:5]
a [lr3:2] = iTemp0 [lr0:1]
```

**Philosophy**: C is the language. The IR models C, not hardware. Multi-target comes from keeping the IR abstract.

**Strengths**:
- Clean multi-target: same iCode → different backends
- Proven on 6+ architectures over 20+ years
- Complete C89/C99 support

**Weaknesses**:
- No hardware-specific optimization at IR level
- Register allocation is backend's problem (often poor on register-starved targets)
- No self-modifying code, no fixed-point, no iterators

### MinZ MIR: Hardware-Aware Performance

MIR models both the language and the target. Virtual registers carry hints for physical allocation. Z80-specific opcodes (DJNZ, SMC) exist at IR level so the optimizer can reason about them.

```
; MinZ MIR for iterator: arr.iter().map(f).forEach(g)
r1 = load_const 10       ; count
r2 = load_addr arr        ; array pointer
                          ; Hint: r1 → B (DJNZ), r2 → HL (addressing)
loop:
  r3 = load_ptr r2        ; element = *HL
  r4 = call f, r3         ; mapped = f(element)
  call g, r4              ; g(mapped)
  r2 = inc r2             ; HL++
  djnz r1, loop           ; B--, jump if B != 0
```

**Philosophy**: Performance on Z80 matters more than portability. The IR should enable the best possible code for the target.

**Strengths**:
- Z80 code quality exceeds SDCC for optimized patterns (SMC, fused iterators)
- 13+ optimization passes vs SDCC's ~10
- Standalone VM for compile-time execution
- Fixed-point types (unique among 8-bit IRs)

**Weaknesses**:
- Z80 coupling makes non-Z80 backends second-class
- More complex than needed for simple programs
- 118 opcodes is a lot to implement per backend

### cc65: Direct and Simple

cc65 barely has an IR. The compiler translates C to 6502-specific pseudo-ops that are almost assembly:

```
; cc65 for: a = b + c
lda _b
clc
adc _c
sta _a
```

**Philosophy**: Simplicity. The 6502 is so constrained (3 registers, 256-byte stack) that an abstract IR adds overhead without benefit.

**Strengths**:
- Simple implementation (~30K LOC total compiler)
- Predictable output (what you write is roughly what you get)
- Excellent 6502 platform library coverage

**Weaknesses**:
- Minimal optimization (peephole only, ~40 patterns)
- No inlining, no constant propagation across functions
- Software stack for all local variables (slow)
- No way to express high-level intent (iterators, closures, etc.)

### QBE: Elegant Minimalism

QBE is not an 8-bit IR but is worth studying as a design reference — it achieves ~70% of LLVM's code quality in ~10% of the code.

```qbe
function w $add(w %a, w %b) {
@start
    %c =w add %a, %b
    ret %c
}
```

**Philosophy**: Most programs don't need LLVM's complexity. A small, well-designed IR with SSA and a few optimization passes produces good-enough code for most targets.

**Strengths**:
- 12K LOC total — incredibly small
- SSA form enables clean optimization
- Only ~40 opcodes — everything else is lowered to them
- Clean separation of concerns

**Weaknesses**:
- 64-bit targets only (word, long, single, double)
- Not designed for register-constrained architectures
- No domain-specific opcodes (no SMC, no iterators)

### ACK EM: The Multi-Language Pioneer

ACK (Amsterdam Compiler Kit, 1980s) solved the multi-language, multi-target problem with EM — a stack-based intermediate language:

```
; ACK EM for: a = b + c
LOL -2    ; load local 'b' (offset -2)
LOL -4    ; load local 'c' (offset -4)
ADI 2     ; add 2-byte integers
STL -6    ; store to local 'a' (offset -6)
```

**Philosophy**: A stack machine is language-neutral. Any language can target it. Any backend can consume it. The cost is performance (extra push/pop), but the payoff is maximum reuse.

**Strengths**:
- 5+ language frontends (C, Pascal, Modula-2, Occam, BASIC)
- 10+ backends (Z80, 6502, 8086, 68000, SPARC, ...)
- Proven multi-language architecture
- ~140 EM instructions cover everything

**Weaknesses**:
- Stack-based IR produces mediocre code on register machines
- Inactive/unmaintained since ~2005
- No modern optimization passes
- Stack overhead significant on Z80 (every operation touches memory)

---

## Feature Matrix

| Feature | MinZ MIR | SDCC iCode | cc65 | QBE | ACK EM |
|---------|----------|------------|------|-----|--------|
| **Fixed-point types** | 5 native types | No | No | No | No |
| **Self-modifying code** | 14 opcodes | No | No | No | No |
| **Iterator/closure** | DJNZ fusion | No | No | No | No |
| **Standalone VM** | 4.6K LOC | No | No | No | Yes* |
| **Register hints** | 10 hint types | No | N/A | No | No |
| **Source debugging** | SLD generation | CDB format | VICE labels | DWARF | No |
| **Multi-language** | MinZ only | C only | C only | C-like | 5+ |
| **SSA** | No | Partial | No | Yes | No |
| **Optimization depth** | 13+ passes | ~10 passes | Peephole | ~5 passes | ~8 passes |
| **Assembly peephole** | 67 patterns | ~50 patterns | ~40 patterns | N/A | ~30 patterns |
| **Inline assembly** | `OpAsm` | `__asm` | `__asm__` | No | No |

---

## What MIR Can Learn From Each

### From SDCC: Target Abstraction

SDCC's `PORT` structure cleanly separates target-specific from target-generic:

```c
typedef struct {
    const char *target;          // "z80", "stm8", "8051"
    struct { int maxRegParms; }; // How many register parameters
    struct { int fptr_size; };   // Function pointer size
    void (*genAssemblerPreamble)(FILE *);
    // ... ~30 more target hooks
} PORT;
```

Each target fills in this struct. iCode doesn't know which target it's on — it just calls through the PORT interface.

**MIR could adopt**: A `TargetInfo` struct replacing hardcoded `Z80Register` in `ir.go`. This is Layer 1 of the refactoring plan.

### From cc65: Zero-Page Strategy

cc65 treats 6502's zero page (256 bytes of fast-access RAM) as a precious resource. Variables used in inner loops get zero-page allocation; others go to main RAM.

**MIR could adopt**: A "hot register region" strategy — variables in tight loops get physical register allocation priority. MIR's `CodegenHints.BareDJNZ` is a step in this direction; generalizing it to "hot path" marking would help.

### From QBE: Minimal Opcode Set

QBE proves that ~40 opcodes suffice for real programs. MinZ MIR has 118 — many are Z80-specific (SMC, patching) or convenience aliases (OpJmp vs OpJump, OpLoadImm vs OpLoadConst).

**MIR could adopt**: Audit for redundant opcodes. Several pairs do the same thing:
- `OpJump` / `OpJmp` — identical
- `OpJumpIf` / `OpJmpIf` — identical
- `OpLoadConst` / `OpLoadImm` — identical
- `OpJumpIfNot` / `OpJmpIfNot` — identical

Removing aliases would reduce the effective opcode count to ~105 without losing any functionality.

### From ACK: The Stack-vs-Register Tradeoff

ACK proved that a stack-based IL enables multi-language support (C, Pascal, Modula-2, Occam, BASIC — all targeting the same EM). The cost: every operation touches memory, which is fatal on Z80 where memory access takes 3-4 T-states per byte.

**MIR's lesson**: Register-based IR was the right choice for Z80 performance. But if multi-language support becomes important, adding a thin stack-to-register translator (like what a Forth frontend would need) is cheaper than redesigning the IR.

---

## The Unique Things Only MIR Has

No other 8-bit IR has these:

1. **Native fixed-point types** (`f8.8`, `f.8`, `f.16`, `f16.8`, `f8.16`) — enables fixed-point arithmetic without library calls. Critical for games, graphics, audio on Z80.

2. **Self-modifying code as IR concept** — 14 opcodes model runtime code patching. The optimizer can reason about SMC (e.g., fusing constant-argument calls into patched immediates).

3. **Iterator fusion at IR level** — `OpDJNZ` + the fusion optimizer can transform `.map().filter().forEach()` chains into single DJNZ loops, eliminating function call overhead entirely.

4. **Standalone VM** — MIR can be executed without any backend. This enables compile-time evaluation, testing, and metaprogramming (`@minz[[[...]]]`).

5. **Register hints from semantic analysis** — the semantic analyzer knows that iterator counters should go in B (for DJNZ) and array pointers in HL (for `(HL)` addressing). This knowledge flows through the IR to the register allocator.

6. **Codegen hints from optimizer** — the optimizer computes `CanUseINC`, `CanUseDEC`, `CanUseXOR`, `BareDJNZ` hints that let the backend emit optimal instructions without pattern matching.

These features represent domain expertise (Z80 programming patterns) encoded into the compiler infrastructure — something that general-purpose IRs like LLVM IR or QBE IL fundamentally can't do.

---

## Summary

MinZ MIR is **overspecialized in the right direction**. For Z80 targets, its hardware awareness produces better code than any general-purpose IR could. For multi-language/multi-target use, its Z80 coupling is a real cost — but the portable core (~55 opcodes) is solid, and the path to multi-language support (target abstraction + a few new opcodes) is clear.

The right strategy isn't to become SDCC or ACK — it's to stay Z80-excellent while making the minimum changes needed to support specific additional languages (PL/M first, then BASIC/Pascal) as demand arises.

---

## Further Reading

- [MIR Architecture Guide](MIR_Architecture_Guide.md) — full opcode reference, type system, optimization pipeline
- [MIR Analysis Report](../reports/2026-03-02-020-MIR_Analysis_Multi_Language_IR_Feasibility.md) — portability scorecard, refactoring plan
- [MIR Language Compatibility](../reports/2026-03-03-021-MIR_Language_Compatibility_Deep_Dive.md) — per-language gap analysis
- [MIR vs LLVM IR Comparison](MIR_vs_LLVM_IR_Comparison.md) — three-way comparison with LLVM IR and Rust MIR
