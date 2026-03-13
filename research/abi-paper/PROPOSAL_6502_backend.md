# Proposal: MIR2 6502 Backend — PFCCO on the Most Irregular Architecture

**Status:** Draft
**Scope:** New backend, ~3.5-5K LOC new code, zero changes to MIR2 core
**Motivation:** Validate PFCCO's architecture-independence; publishable result
**Branch:** `feature/6502-backend`

---

## Why 6502?

### The most irregular popular architecture

| | Z80 | 6502 | Ratio |
|---|---|---|---|
| General registers | 7 (A,B,C,D,E,H,L) | 3 (A,X,Y) | 2.3x fewer |
| ALU operand regs | 1 (A) | 1 (A) | Same |
| Register pairs (16-bit) | 3 (BC,DE,HL) | 0 | 6502 uses zero-page |
| Addressing modes | ~8 | 13 | More modes, fewer regs |
| Index registers | 2 (IX,IY) | 2 (X,Y) but asymmetric | X!=Y capabilities |

The 6502 has **extreme register pressure** — only A can do arithmetic,
X and Y are not interchangeable (X for `(zp,X)` indirect, Y for `(zp),Y`
indirect-indexed), and 16-bit values must live in zero-page pairs.

This makes PFCCO **even more impactful**: choosing the wrong register
for a parameter on 6502 is catastrophic. There's no "just use another
general register" fallback.

### Publishable novelty

No published work describes per-function calling convention optimization
for 6502. The closest is llvm-mos's link-time whole-program optimization,
which optimizes *frame placement* (static vs dynamic stack), not *parameter
passing conventions per function*. PFCCO would be novel.

### Community interest

NES homebrew, C64 demoscene, Apple II retro — massive community.
The 6502 is the most iconic retro CPU. A compiler that generates
better code than cc65 (the standard 6502 C compiler) would get
significant attention.

---

## Architecture: What Changes, What Doesn't

### ZERO changes to MIR2 core

| Component | LOC | Reusable? |
|-----------|-----|-----------|
| MIR2 IR (ops, types, regs) | ~900 | 100% |
| Constant propagation + folding | ~500 | 100% |
| Dead store elimination | ~120 | 100% |
| Trivial function inlining | ~350 | 100% |
| Copy propagation | ~100 | 100% |
| PBQP register allocation | ~430 | 100% |
| PFCCO contract optimization | ~600 | 100% |
| Liveness analysis | ~300 | 100% |
| **Total reusable** | **~3,300** | **100%** |

### New code needed

| Component | Est. LOC | Complexity |
|-----------|----------|-----------|
| `pkg/mir2/m6502cost.go` | 200-300 | Medium |
| `pkg/mir2/m6502codegen.go` | 2500-4000 | High (main effort) |
| `pkg/m6502timing/timing.go` | 100-150 | Low |
| Pipeline wiring | 50-100 | Low |
| Tests + E2E verification | 200-400 | Medium |
| **Total new** | **~3,500-5,000** | |

---

## 6502 Register Classes for PFCCO

```go
// m6502cost.go

const (
    ClassAcc6502    RegClass = iota  // A — sole ALU register
    ClassIdxX                        // X — zero-page indexing, TAX/TXA
    ClassIdxY                        // Y — indirect-indexed (zp),Y, TAY/TYA
    ClassZP8                         // Zero-page byte (LDA zp = 3 cycles)
    ClassZP16                        // Zero-page pair (2 bytes, e.g. $00-$01)
    ClassStack6502                   // Hardware stack (PHA/PLA = 3 cycles each)
    ClassMem6502                     // Absolute memory (LDA abs = 4 cycles)
)
```

### Cost table (6502 cycles)

| From \ To | Acc | IdxX | IdxY | ZP8 | ZP16 |
|-----------|-----|------|------|-----|------|
| Acc | 0 | 2 (TAX) | 2 (TAY) | 3 (STA zp) | 6 (STA zp;STA zp+1) |
| IdxX | 2 (TXA) | 0 | 4* | 4 (STX zp) | — |
| IdxY | 2 (TYA) | 4* | 0 | 4 (STY zp) | — |
| ZP8 | 3 (LDA zp) | 3 (LDX zp) | 3 (LDY zp) | 5** | — |
| ZP16 | — | — | — | — | 0 |

\* X↔Y requires going through A: TXA;TAY or TYA;TAX (4 cycles)
\** ZP→ZP requires LDA zp1; STA zp2 (6 cycles, through A)

### plausibleClasses for PFCCO

| Type | Plausible classes |
|------|------------------|
| u8 | {ClassAcc6502, ClassIdxX, ClassIdxY, ClassZP8} |
| u16 | {ClassZP16} (only option — no register pairs!) |
| ptr | {ClassZP16} |
| bool | {ClassAcc6502} (flags-based) |

**Key insight for PFCCO:** u8 has 4 plausible classes on 6502 (vs 3 on Z80).
The search space per function is slightly larger, but still O(1) bounded.
u16 has only 1 class (ZP16), so 16-bit parameter placement is really about
*which* zero-page pair, not which class.

---

## Calling Convention Comparison

### cc65 (state of the art)

```
Param 1 (u8):  A register
Param 2+ :     software stack (slow! PHA/PLA per byte)
Return (u8):   A
Return (u16):  A (lo), X (hi)    [or via stack pointer]
```

cc65 uses a **software stack** for parameter passing — extremely slow
(~20 cycles per parameter vs 2-3 for register passing).

### llvm-mos (modern)

```
Param 1-2:     A, X/Y, then "imaginary registers" (zero-page pairs)
Return:        A (u8), A:X (u16)
Optimization:  Link-time static frame allocation for non-recursive functions
```

llvm-mos uses up to 16 zero-page "imaginary registers" ($00-$1F),
assigned globally at link time.

### Nanz with PFCCO (proposed)

```
Param:         Per-function optimal: A, X, Y, or ZP pair — chosen by PFCCO
Return:        Per-function optimal: A, X, or ZP pair
Optimization:  Compile-time per-function contract optimization
```

**PFCCO advantage over llvm-mos:** llvm-mos assigns imaginary registers
*globally* (same $00-$01 is always "rc0" for all functions). PFCCO
assigns *per-function* — function A might use ZP $00-$01 for its first
parameter while function B uses A register for the same slot, based on
how each function body uses the value.

---

## Example: abs_diff on 6502

```nanz
fun abs_diff(a: u8, b: u8) -> u8 {
    if a < b { return b - a }
    return a - b
}
```

### cc65 output (expected ~12 bytes)
```asm
_abs_diff:
    sta  tmp1       ; 3T — save a to temp (software stack overhead)
    jsr  _get_param ; ~20T — get b from software stack
    cmp  tmp1       ; 3T — compare
    bcs  @ge        ; 2T
    sec             ; 2T
    sbc  tmp1       ; 3T — b - a
    rts             ; 6T
@ge:
    lda  tmp1       ; 3T
    sec             ; 2T
    sbc  ???        ; need b back...
    ; ... more stack juggling
```

### Nanz PFCCO output (expected ~8 bytes)
```asm
; fun abs_diff(a: u8 = A, b: u8 = X) -> u8 = A
abs_diff:
    stx  tmp        ; 3T — save b (or use ZP)
    sec             ; 2T
    sbc  tmp        ; 3T — A = a - b
    bcs  .done      ; 2T — if a >= b, done
    eor  #$FF       ; 2T — negate (two's complement)
    adc  #1         ; 2T — A = b - a
.done:
    rts             ; 6T
; 8 bytes, 20T worst
```

**PFCCO chose X for second parameter** — `STX tmp; SBC tmp` is cheaper
than fetching from software stack. On Z80, PFCCO chose C for the same
reason (`SUB C` = 1 byte vs `SUB L` not available).

---

## Toolchain: External Dependencies

### Assembler: 64tass

```bash
# Install (Ubuntu/Debian)
sudo apt install 64tass

# Or build from source
wget https://sourceforge.net/projects/tass64/files/binaries/64tass-1.58.2974-src.tar.gz
tar xf 64tass-*.tar.gz && cd 64tass-* && make && sudo make install

# Usage (simple, single binary)
64tass -o output.bin input.asm
64tass -o output.prg --cbm-prg input.asm   # C64 .PRG format
```

**Why 64tass:** Simplest CLI invocation, single binary, no config files
needed. Community standard for C64/NES development.

**Alternative:** ca65 (from cc65 suite) — more features but requires
linker config files. Use if we need cc65 interop.

### Emulator: Go library

**Option A: `beevik/go6502`** — CPU + assembler + disassembler + debugger
```go
import "github.com/beevik/go6502/cpu"

c := cpu.New(mem)
c.SetPC(0x0200)
for !c.Stopped() {
    c.Step()    // execute one instruction
}
result := c.Reg.A   // read accumulator
```

**Option B: `ariejan/i6502`** — designed for embedding
```go
import "github.com/ariejan/i6502"

cpu := i6502.NewCPU(bus)
cpu.Reset()
cpu.Step()
```

**Recommendation:** Start with `beevik/go6502` — most complete, includes
assembler (could replace 64tass for testing).

### I/O Trapping (like Z80 --console)

6502 has no I/O ports like Z80. Instead, use **memory-mapped I/O**:

```go
// Memory-mapped I/O handler
type ConsoleMem struct {
    ram [65536]byte
}

func (m *ConsoleMem) Write(addr uint16, val byte) {
    if addr == 0xFFF0 {
        fmt.Printf("%c", val)  // Console output
        return
    }
    if addr == 0xFFF1 {
        // Halt CPU
        return
    }
    m.ram[addr] = val
}
```

Map `$FFF0` = console output, `$FFF1` = halt. Same concept as Z80's
`OUT (1), A` but memory-mapped. Assert verification works the same way:
compile, assemble, emulate, check A register at halt.

---

## Implementation Plan

### Phase 1: Foundation (1 session, ~500 LOC)

1. Create `feature/6502-backend` branch
2. `pkg/mir2/m6502cost.go` — CostTable with 6502 physical locations
3. `pkg/mir2/m6502codegen.go` — skeleton: emit `.org`, function labels,
   RTS. Handle OpConst, OpMove, OpAdd, OpSub, OpReturn.
4. `pkg/pipeline/pipeline.go` — add `--backend=6502` flag
5. **Goal:** `fun add(a: u8, b: u8) -> u8 { return a + b }` compiles to
   valid 6502 assembly

### Phase 2: Core Codegen (2-3 sessions, ~2000 LOC)

1. Arithmetic: OpMul (software), OpDiv (software), shifts, bitwise
2. Comparisons + branches: OpCmp → CMP/BCC/BCS/BEQ/BNE
3. Memory: OpLoad, OpStore via zero-page and absolute addressing
4. Function calls: JSR/RTS, parameter setup via register/ZP contracts
5. 16-bit operations: zero-page pair arithmetic (ADC/SBC chains)
6. **Goal:** GCD, abs_diff, factorial compile and run correctly

### Phase 3: Optimization + Testing (1-2 sessions, ~1500 LOC)

1. Wire PFCCO + PBQP with 6502 cost table
2. Peephole patterns (TAX;TXA elimination, redundant LDA, etc.)
3. Zero-page allocation strategy (which pairs for which functions)
4. E2E test suite: compile → 64tass → emulate → verify
5. **Goal:** 10+ examples passing, PFCCO demonstrably choosing better
   conventions than cc65's fixed ABI

### Phase 4: Paper Integration (1 session)

1. Add 6502 column to PFCCO paper evaluation table
2. Show same algorithm, same code, different architecture
3. Compare vs cc65 (fixed software-stack ABI) — expect 3-10x wins
4. **Goal:** "PFCCO generalizes across architectures" claim validated

---

## What Stays the Same (the whole point)

```
Nanz source → Parser → HIR → MIR2 → [PFCCO] → [PBQP] → {Z80 codegen}
                                                         {6502 codegen}  ← NEW
                                                         {C codegen}
                                                         {QBE codegen}
```

Everything above the dotted line is identical. Same parser, same HIR
lowering, same MIR2 optimizations, same PFCCO algorithm, same PBQP
solver. Only the cost table and codegen change.

**This is the paper's strongest architectural claim:** PFCCO is not a
Z80 trick. It's a general technique for irregular register architectures.
6502 proves it.

---

## Risk Assessment

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| 6502 codegen harder than expected | Medium | Start with simple examples; reference existing m6502_backend.go |
| Zero-page allocation conflicts | Medium | Use PFCCO to assign ZP pairs per-function; non-overlapping live ranges |
| 16-bit arithmetic too verbose | Low | Expected — 6502 is an 8-bit CPU. Show the overhead honestly. |
| External assembler/emulator integration | Low | Go libraries exist; 64tass is single binary |
| No measurable PFCCO improvement | Very Low | 6502 is MORE irregular than Z80 — improvements should be LARGER |

---

## References

- llvm-mos C calling convention: https://llvm-mos.org/wiki/C_calling_convention
- cc65 documentation: https://cc65.github.io/
- Millfork calling convention: https://karols.github.io/millfork/abi/calling-convention.html
- beevik/go6502: https://github.com/beevik/go6502
- 64tass assembler: https://sourceforge.net/projects/tass64/
- 6502.org efficient calling convention discussion: http://forum.6502.org/viewtopic.php?f=2&t=6181
- gingerBill, "Multiple Return Values Research" (polyadic design)
- Existing MinZ 6502 stub: pkg/codegen/m6502_backend.go (~260 LOC)
