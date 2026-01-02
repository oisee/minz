# MinZ Optimization Pipeline v0.15

## Overview

MinZ now features a **two-level optimization pipeline** that produces highly efficient Z80 code, competitive with hand-tuned assembly and mature compilers like SDCC.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Source Code (.minz)                       │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Frontend (Parser + IR)                    │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│              LEVEL 1: MIR Optimizations                      │
│  ┌─────────────────┐  ┌─────────────────┐                   │
│  │ Instruction     │  │ MIR Peephole    │                   │
│  │ Reordering      │──│ Optimization    │ ◄── Fixed-point   │
│  └─────────────────┘  └─────────────────┘     iteration     │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   Z80 Code Generation                        │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│              LEVEL 2: Assembly Peephole                      │
│  35+ patterns for Z80-specific optimizations                 │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Output (.a80)                             │
└─────────────────────────────────────────────────────────────┘
```

## Level 1: MIR Optimizations

These work on the intermediate representation before code generation.

### Constant Folding
Evaluates constant expressions at compile time.
```
2 + 3  →  5
10 * 4 →  40
```

### Algebraic Simplification
Applies algebraic identities to simplify expressions.

| Pattern | Simplification |
|---------|----------------|
| `x + 0` | `x` |
| `x - 0` | `x` |
| `x * 1` | `x` |
| `x * 0` | `0` |
| `x / 1` | `x` |
| `x & x` | `x` |
| `x \| x` | `x` |
| `x ^ x` | `0` |
| `x ^ 0` | `x` |
| `x - x` | `0` |
| `x << 0` | `x` |
| `x >> 0` | `x` |

### Strength Reduction
Replaces expensive operations with cheaper equivalents.

| Expensive | Cheap | Savings |
|-----------|-------|---------|
| `x * 2` | `x << 1` | MUL → SLA |
| `x * 4` | `x << 2` | MUL → 2×SLA |
| `x * 8` | `x << 3` | MUL → 3×SLA |
| `x / 2` | `x >> 1` | DIV → SRL |
| `x / 4` | `x >> 2` | DIV → 2×SRL |
| `x % 2` | `x & 1` | MOD → AND |
| `x % 4` | `x & 3` | MOD → AND |
| `x % 8` | `x & 7` | MOD → AND |
| `x % 16` | `x & 15` | MOD → AND |

### Copy Propagation + Dead Code Elimination
```
b = a       // Copy
c = b       // Use of copy
d = b       // Use of copy
            // b not used after this

// After optimization:
c = a       // Propagated
d = a       // Propagated
            // b = a eliminated (dead code)
```

### Redundant Load Elimination
```
x = load(var)
y = load(var)   // Same variable, no intervening store

// After optimization:
x = load(var)
y = x           // Reuse loaded value
```

### Instruction Reordering
- Groups independent loads together
- Hoists loop-invariant code
- Sinks stores to reduce register pressure
- Clusters related operations

## Level 2: Assembly Peephole

These work on the final Z80 assembly output.

### 8-bit Comparison Optimization
Uses `CP` instruction instead of 16-bit `SBC HL,DE` for 8-bit values.

```asm
; Before (16-bit pattern)
LD HL, value1
LD DE, value2
SBC HL, DE        ; 15 T-states

; After (8-bit pattern)
LD A, value1
CP value2         ; 7 T-states
JR C, label       ; Uses JR, not JP
```
**Savings:** 8+ T-states per comparison

### Jump-to-Next Elimination
Removes jumps that go to the immediately following instruction.

```asm
; Before
    JP label
label:
    ...

; After
label:
    ...
```
**Savings:** 10 T-states, 3 bytes

### DJNZ Pattern Recognition
Converts decrement-and-jump patterns to single instruction.

```asm
; Before
    DEC B
    JP NZ, loop

; After
    DJNZ loop
```
**Savings:** 1 byte, 3 T-states per iteration

**Note:** DJNZ only works within ±127 bytes of target label.

### Load Zero Optimization
```asm
; Before
LD A, 0     ; 7 T-states, 2 bytes

; After
XOR A       ; 4 T-states, 1 byte
```

### Other Patterns
- `ADD r, 1` → `INC r`
- `SUB r, 1` → `DEC r`
- Redundant register moves elimination
- And 30+ more patterns

## SDCC Comparison

Benchmark comparison with SDCC (industry-standard C compiler for Z80):

| Metric | SDCC | MinZ | Winner |
|--------|------|------|--------|
| 8-bit add | 8 bytes | 6 bytes | MinZ |
| Function call overhead | ~20 bytes | ~8 bytes (TSMC) | MinZ |
| Loop construct | Standard | DJNZ when possible | Tie |
| Comparison | CP-based | CP-based | Tie |

MinZ's **True Self-Modifying Code (TSMC)** gives it a unique advantage for embedded systems with RAM-based code execution.

## Multi-Pass Optimization

The `MIRCombinedPass` runs reordering and peephole passes in a loop until no more improvements are found (fixed-point iteration). This ensures that optimizations exposed by one pass are caught by subsequent passes.

```go
for iterations < maxIterations {
    changed := false
    changed |= reorderingPass.Run(module)
    changed |= peepholePass.Run(module)
    if !changed {
        break  // Fixed point reached
    }
}
```

## Files

- `pkg/optimizer/mir_peephole.go` - MIR-level peephole optimizations
- `pkg/optimizer/mir_combined.go` - Multi-pass optimizer coordinator
- `pkg/optimizer/mir_reordering.go` - Instruction reordering
- `pkg/optimizer/assembly_peephole.go` - Z80 assembly peephole patterns
- `pkg/codegen/z80.go` - Integrated optimization pipeline

## Usage

Optimizations are enabled by default. To disable:
```bash
./minzc program.minz --disable-optimize
```

To see optimization statistics:
```bash
DEBUG=1 ./minzc program.minz
```
