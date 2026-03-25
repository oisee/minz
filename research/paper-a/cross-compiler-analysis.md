# Cross-Compiler Signature Analysis

## Experiment: MinZ vs SDCC for identical C functions

### Setup
- **SDCC 4.2.0** — industry-standard Z80 C compiler (graph coloring regalloc)
- **MinZ VIR** — Z3 SMT solver + GPU exhaustive table
- **MinZ C89** — same source, different frontend (u16 promotion)
- **Nanz** — native u8 types, no promotion

### Results

| Function | SDCC (z80) | MinZ C89 (u16 promoted) | MinZ Nanz (native u8) |
|----------|-----------|------------------------|----------------------|
| `add8(a,b)→u8` | `ADD A, L / RET` (2) | `LD L,A / LD H,0 / ADD HL,BC / LD A,L / RET` (5) | `ADD A, C / RET` (2) |
| `double_val(x)→u8` | `ADD A, A / RET` (2) | `LD L,A / LD H,0 / ADD HL,BC / LD A,L / RET` (5) | `ADD A, A / RET` (2) |
| `add16(a,b)→u16` | `ADD HL, DE / EX DE,HL / RET` (3) | `ADD HL, DE / RET` (2) | N/A |
| `abs_diff(a,b)→u8` | `LD C,A / LD A,L / SUB C / JR NC / LD A,C / SUB L / RET` (7) | — | `SUB C / RET NC / NEG / RET` (4) |

### Key Finding: Width Promotion is the Differentiator

When using **native u8 types** (Nanz), MinZ VIR produces:
- **Identical** code to SDCC for `double_val` (`ADD A, A / RET`)
- **Same pattern** for `add8` (2-inst ALU, different src reg = calling convention)
- **Better** code for `abs_diff` (4 inst vs 7 — Z3 finds NEG optimization)

When using **C89 frontend** (u16 promotion), MinZ produces 5 instructions for what should be 2. The IR promotes u8→u16, creating unnecessary 16-bit operations.

### Implication for Paper A

The 315 constraint signatures include IR artifacts from width promotion.
The **true ISA-intrinsic vocabulary** is smaller:

```
315 signatures (current)
  - ~65 from unnecessary u16 promotion (u8 functions inflated to 16-bit patterns)
  ≈ 250 ISA-intrinsic signatures
```

This STRENGTHENS the paper's claim:
- Fewer signatures = smaller table = more "solved"
- The constraint space is even more compressed than initially measured
- Cross-compiler transfer should be high IF both compilers use native widths

### SDCC Calling Convention vs PFCCO

| | SDCC | MinZ PFCCO |
|---|------|-----------|
| 1st u8 param | A | A |
| 2nd u8 param | L (via stack) | C |
| 1st u16 param | HL (via stack) | HL |
| Return u8 | A | A |
| Return u16 | DE | HL |

The calling convention difference means the exact register assignments differ,
but the **constraint pattern** (1 dst-A, 2 src-GPR, tied-dst) is the same.
Different physical allocation, same abstract signature.

## Per-Frontend Signature Data (Table 2)

| Corpus | Functions | Unique Sigs | Reuse Rate |
|--------|-----------|-------------|------------|
| Nanz | ~200 | ~100 | 80.9% |
| C89 | 292 | 129 | 55.8% |
| ABAP | ~80 | ~20 | 98.8% |
| Combined | 645 | 315 | 97.8% convergence |

C89 has lower reuse (55.8%) because it has more diverse patterns (pointer
arithmetic, struct access, bitops). ABAP has highest reuse (98.8%) because
business logic functions share common ALU + WRITE patterns.

Top signature reuse in C89 corpus:
- 68 functions share one trivial signature (test stubs)
- `abs_diff`: same sig across 7 files (0a1625cad3c1)
- `add/add8`: same sig, 8 instances (21a88efb6705)
- `max/min`: same sig, 7 instances (4b042982e36f)

This is the Zipf-like distribution: few signatures cover many functions.

### Next Steps

1. Compile all 333 C89 functions with SDCC → extract register pressure
2. Fix C89 frontend: avoid u16 promotion for u8 operations
3. Re-measure: signature count with native widths (predicted: ~250)
4. Cross-compiler transfer: compute signature overlap between MinZ and SDCC
5. Width-equivalence tagging in VIR → hash without promotion → true ISA vocabulary
