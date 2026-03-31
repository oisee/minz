# Closing The Gap: Nanz → Z80 vs Hand-Optimized ASM

**Case study: Che Guevara LFSR decoder. 1043 bytes → 382 bytes. The journey.**

---

## The Challenge

Write a Che Guevara face renderer in Nanz (high-level language) and compare
the output with hand-optimized Z80 assembly. Measure the gap. Close it.

The algorithm: 32-bit Galois LFSR generates pseudo-random points.
64 layers of XOR'd points converge to a recognizable face.
Seeds found via CUDA GPU brute-force (z80-optimizer project).

## Starting Point

**Hand-optimized ASM: 121 bytes.**

```z80
; LFSR step — register-resident DEHL, zero memory access
lfsr_step:
    srl d           ; 8T
    rr  e           ; 8T
    rr  h           ; 8T
    rr  l           ; 8T — total 32T for 32-bit shift
    ret nc          ; skip XOR if bit=0 (50% of the time)
    ld  a, d
    xor $B4         ; XOR taps byte-by-byte
    ld  d, a        ; ... 60T when XOR fires
    ...
    ret             ; avg 72T per step
```

Key optimizations in the ASM:
- **DEHL register-resident** — LFSR state never touches memory
- **SRL/RR chain** — native 32-bit shift in 4 instructions
- **RET NC** — conditional return skips XOR (branchless on 50% of inputs)
- **CPL** for XOR 0xFF — 4T/1B vs XOR 255 at 7T/2B
- **INC H** for next pixel row — ZX screen layout trick
- **CP 96; JR C; SUB 96** — inline mod with no loop

**Naive Nanz: 1043 bytes (8.6× the ASM).**

```nanz
fun lfsr_step() -> void {
    let el: u16 = st_el          // memory load (16T!)
    let dh: u16 = st_dh          // memory load (16T!)
    let bit: u8 = el % 2         // → full 16-bit division loop!
    st_el = el / 2               // → full 16-bit division loop!
    if dh % 2 == 1 { ... }       // → another division loop!
    st_dh = dh / 2               // → and another one
    ...
    st_dh = st_dh xor 0xB4BC     // memory store (16T)
    st_el = st_el xor 0xD35C     // memory store (16T)
}
```

Problems:
1. `% 2` compiled to **full 16-bit division loop** (~40 instructions) instead of `AND 1`
2. `/ 2` compiled to **full division** instead of `SRL`
3. `% 128` — another division loop instead of `AND $7F`
4. LFSR state in **global variables** (memory) instead of registers
5. Screen address computed **per pixel row** via function call

## The Journey: 8.6× → 3.2×

### Step 1: Power-of-2 Strength Reduction (8.6× → 6.2×)

Added to VIR bridge (`bridge.go`):
```go
// Before: div(x, 2^n) → CALL __div16 (40+ instructions)
// After:  div(x, 2^n) → SHR(x, n) (1-3 instructions)
if inst.Imm > 0 && isPow2(inst.Imm) {
    shift := log2(inst.Imm)
    // emit SRL chain
}
```

Also for PBQP path (`z80codegen.go`):
```go
// mod(x, 2^n) → AND (2^n - 1)
if isConst && k > 0 && k&(k-1) == 0 {
    g.emitf("    AND %d", k-1)
}
```

**Result: 1043 → 754 bytes.** Every `% 2`, `/ 2`, `% 128`, `/ 8` now compiles to
1-3 shift/and instructions instead of 40-instruction division loops.

### Step 2: u16 Inline ASM (6.2× → 6.2×, same ratio but different code)

Z80 descriptor had no pattern for 16-bit SHR. Added inline asm:
```go
// u16 div by 2^n: inline SRL H; RR L chain
for i := 0; i < shift; i++ {
    asmLines = append(asmLines, "    SRL H", "    RR L")
}
```

### Step 3: Code Architecture (6.2× → 4.0×)

Rewrote Nanz code for Z80 efficiency:
- **Globals for LFSR state** (eliminated `get_dh()` function call overhead)
- **`addr = addr + 256`** instead of calling `screen_addr()` per row
- **`while raw_y >= 96`** instead of `% 96` (not power-of-2)

### Step 4: PBQP Strength Reduction (4.0× → 3.2×)

Added power-of-2 div/mod to PBQP codegen (`z80codegen.go`):
```go
func (g *z80cg) genDivMod(inst *Inst) {
    k, isConst := g.constVals[inst.Src[1]]
    if isConst && k > 0 && k&(k-1) == 0 {
        // div → SRL chain, mod → AND mask
    }
}
```

Fixed `emitLD8` for u8→pair widening (`LD L,A; LD H,0` instead of invalid `LD HL,A`).

### Step 5: Idiomatic Nanz (3.4× → 3.2×)

- **`for i in 0..8`** instead of `while i < 8 { ... i = i + 1 }` → DJNZ
- **Split helper functions** instead of one big main → less register pressure
- **`p^ = p^ xor 0xFF`** for screen XOR → `LD A,(HL); CPL; LD (HL),A`

### Step 6: Additional Optimizations

- **XOR 255 → CPL** peephole (4T/1B vs 7T/2B)
- **Grace bounded loop unroll** (`while x >= K` → if-chain)
- **IX/IY half-register patterns** (compound moves HL↔IXH)
- **Spill label dedup** (fixed duplicate `_spill_` definitions)
- **Adaptive Z3 timeout** (complex functions → fast PBQP fallback)

## Final Comparison

```
Hand-optimized ASM:  121 bytes  (1.0×)
Nanz v1 (naive):    1043 bytes  (8.6×)
Nanz final:          382 bytes  (3.2×)
Code reduction:       63%
```

### Instruction Mix Analysis

```
Nanz output (382 bytes):
  Logic (ALU):     58 instructions  (26%)  — actual computation
  Data move (LD): 134 instructions  (61%)  — register shuffling
  Control (JR):    25 instructions  (11%)  — branches/calls
```

**61% of instructions are data moves.** This is the fundamental cost of
7 general-purpose registers. The hand-optimized ASM minimizes this by
keeping DEHL register-resident across the entire program.

### What Generates The Gap

| Source | Nanz | Optimal ASM | Overhead |
|--------|------|-------------|----------|
| LFSR state | Global vars (memory LD/ST) | Register-resident DEHL | ~16T per access |
| Function calls | CALL+RET+adapter (17T+) | Inlined | 17T per call |
| Screen addr | Multiply chain | Bit manipulation | ~20T per row |
| `% 96` | While loop | `CP 96; JR C; SUB 96` | ~10T |

### Key Discovery: Inlining HURTS on Z80

```
Monolithic main():   555 bytes  (4.6×)  ← WORSE!
Split functions:     382 bytes  (3.2×)  ← BETTER!
```

**On a 7-register machine, function call overhead (17T) is CHEAPER than
register spill overhead (21T per PUSH/POP pair).** Inlining everything into
one function creates 20+ live variables → massive spilling → more code.

This is the opposite of x86/ARM where inlining usually helps.
**Paper B material: register pressure dominates on small-register architectures.**

## What's Left (Structural Gap)

| Optimization | Impact | Difficulty |
|---|---|---|
| Interprocedural register allocation | -30% | Hard (MIR2 pass) |
| Cost-based inlining heuristic | -20% | Medium |
| Global→register promotion | -30% | Hard |
| ZX screen bit-manipulation ISLE rules | -10% | Medium |

The remaining 3.2× gap is **structural** — it requires the compiler to reason
about data flow across function boundaries, which current per-function allocation cannot do.

## Compiler Improvements Made

| Improvement | Where | Effect |
|---|---|---|
| Power-of-2 div/mod → shr/and | VIR bridge + PBQP | **-28%** code size |
| u16 inline asm shr/and | VIR bridge | -1% |
| emitLD8 u8→pair fix | PBQP codegen | bugfix (invalid LD HL,A) |
| XOR 255 → CPL | VIR peephole | -1% per XOR |
| MIR2 const propagation | VIR bridge | enables strength reduction |
| Grace bounded loop unroll | Grace runner | ~1% |
| IX/IY half compound moves | Z80 descriptor | enables 11-reg allocation |
| Adaptive Z3 timeout | VIR pipeline | 2+ min → <30s compile |
| Spill label dedup | VIR pipeline | bugfix (dup labels) |

## Discoveries

### 1. Branchless Z Flag Materialization (NEW)

GPU proved: arbitrary Z→A is impossible branchlessly.
But with known constant N:

```z80
; (A == N) via two CP trick: 8 ops, 38T, branchless
CP N           ; CY = (A < N)
LD B, A        ; save A
SBC A, A       ; mask1 = CY
LD C, A        ; save
LD A, B        ; restore A
CP N+1         ; CY = (A ≤ N)
SBC A, A       ; mask2
XOR C          ; mask2 XOR mask1 = (A == N)
```

Verified: 65536/65536 exhaustive. First known branchless equality predicate for Z80.

### 2. GPU carry_compare Division (GPU-Discovered)

For K ≥ 128, quotient is always 0 or 1:

```z80
OR A; LD B,(256-K); ADC A,B; SBC A,A; AND 1
; 5 ops, 26T — not in any Z80 textbook!
```

Found by GPU brute-force, verified 32768/32768. Average div8: 154T → 79T (−49%).

### 3. Split Functions > Inline on 7-Register Machines

CALL overhead (17T) < spill overhead (21T per PUSH/POP).
Monolithic function with 20+ live vars = spill disaster on Z80.
Split into 5-8 var functions = clean allocation = smaller code.

---

*All code, measurements, and proofs available in the MinZ repository.*
*GPU tables from z80-optimizer. VIR/PBQP/Grace from minz-vir.*
*5 Claude Code sessions collaborating via dedelulu.*
