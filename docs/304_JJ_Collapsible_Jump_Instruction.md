# JJ: Collapsible Jump Pseudo-Instruction

## Overview

JJ ("Jump Judiciously") is a pseudo-instruction for the MinZ assembler that intelligently chooses between JR (relative) and JP (absolute) jump instructions based on the target distance and optimization preferences.

## Problem Statement

The Z80 has two jump instruction families:

| Instruction | Size | Timing (taken) | Timing (not taken) | Range |
|------------|------|----------------|-------------------|-------|
| JR d       | 2 bytes | 12 T-states | 7 T-states | ±127 bytes |
| JP nn      | 3 bytes | 10 T-states | 10 T-states | Full 64KB |

The programmer faces a dilemma:
- **JR** is smaller but has limited range and slower when taken
- **JP** is larger but has unlimited range and consistent timing

Manually choosing between them is error-prone:
- JR can fail with "relative jump out of range" if code changes
- Optimal choice depends on branch likelihood and optimization goals

## Solution: JJ

JJ automatically handles the JR/JP selection:

```asm
    JJ .target          ; Unconditional
    JJ Z, .target       ; If zero flag set
    JJ NZ, .target      ; If zero flag not set
    JJ C, .target       ; If carry flag set
    JJ NC, .target      ; If carry flag not set
```

### Branch Hints

JJ supports branch prediction hints for future optimization:

```asm
    JJ.L .target        ; Likely taken - prefer JP for speed
    JJ.U .target        ; Unlikely taken - prefer JR for size

    JJ.L Z, .loop       ; Likely zero - use JP
    JJ.U NZ, .rare_case ; Unlikely non-zero - use JR
```

## Z80 Branch Timing Analysis

```
                    JR        JP
Taken:           12 T      10 T    (JP is 2T faster)
Not taken:        7 T      10 T    (JR is 3T faster)
```

**Breakeven Analysis:**
- At 60% taken rate: JR avg = 0.6(12) + 0.4(7) = 10 T = JP
- >60% taken: JP is faster
- <60% taken: JR is faster

This justifies the hints:
- `.L` (likely): Branch usually taken → prefer JP (10 T-states)
- `.U` (unlikely): Branch rarely taken → prefer JR (7 T-states when not taken)

## Implementation: Multi-Pass Convergence

The MinZ assembler uses **multi-pass convergence** to handle JJ's variable-sized output:

### How It Works

1. **Pass 1**: Calculate label addresses assuming worst-case (JP = 3 bytes)
2. **Pass 2**: Emit code with actual JR/JP choices, track instruction sizes
3. **Pass 3+**: If any sizes changed, recalculate labels and re-emit
4. **Converge**: Repeat until no size changes (typically 2-3 passes)

### Example Convergence

```asm
    JJ .skip      ; Pass 1: reserves 3 bytes (JP)
    LD A, 1
.skip:
    LD A, 2
```

- **Pass 1**: JJ reserves 3 bytes, .skip = $8005
- **Pass 2**: Offset calculated, JR fits (2 bytes), emits JR
- **Pass 3**: Labels recalculated, .skip = $8004
- **Pass 4**: JR offset still valid, no changes → converged!

### Optimization Modes

The assembler's `OptMode` affects JJ behavior:

| Mode | Forward Jumps | Backward Jumps | Size Savings |
|------|---------------|----------------|--------------|
| **Balanced** (default) | JR (2 bytes) | JP (3 bytes) | ~1 byte per forward |
| **Size** (`-Os`) | JR when possible | JR when possible | Maximum |
| **Speed** (`-O2`) | JP always | JP always | None |

### Why Backward Jumps Use JP?

In balanced mode, backward jumps (loops) use JP because:
- Loops execute many times
- JP is 10 T-states (always)
- JR is 12 T-states when taken
- Over N iterations: JP saves 2N T-states!

## Usage Examples

### Loop Optimization
```asm
render_frame:
    LD B, 192           ; 192 scanlines
.loop:
    CALL render_line
    DJNZ .loop          ; DJNZ for counted loops

    ; For conditional loops, use JJ:
    LD A, (game_over)
    OR A
    JJ Z, render_frame  ; Continue while not game over
```

### Error Handling
```asm
validate_input:
    LD A, (input)
    CP 128
    JJ.U NC, .invalid   ; Unlikely to be invalid

    ; Normal processing
    ...
    RET

.invalid:
    LD A, ERROR_INVALID
    RET
```

### Jump Tables (Long Distance)
```asm
    ; JJ safely handles any distance
    JJ .handler_table + 0x100  ; Would fail with JR
```

## Benefits

1. **No Range Errors**: JJ never fails due to distance limitations
2. **Future-Proof**: Code remains correct as other code changes
3. **Self-Documenting**: `.L` and `.U` hints document expected behavior
4. **Optimization Ready**: When multi-pass is implemented, full optimization kicks in

## Technical Details

### Instruction Patterns
```
JJ target          → JP target (3 bytes)
JJ cond, target    → JP cond, target (3 bytes)
JJ.L target        → JP target (3 bytes)
JJ.U target        → JP target (3 bytes, future: JR)
JJ.L cond, target  → JP cond, target (3 bytes)
JJ.U cond, target  → JP cond, target (3 bytes, future: JR)
```

### Supported Conditions
- `Z` / `NZ` - Zero/Non-zero
- `C` / `NC` - Carry/No-carry

Note: `PO`, `PE`, `P`, `M` are not supported for JJ (these don't have JR equivalents).

## Comparison with Other Assemblers

| Assembler | Feature | Notes |
|-----------|---------|-------|
| SJASMPLUS | No equivalent | Manual JR/JP only |
| Z88DK | No equivalent | Uses JR with error on overflow |
| PASMO | No equivalent | Manual selection |
| **MZA** | **JJ** | Automatic with hints |

## Roadmap

1. **v0.15**: JJ with multi-pass convergence for JR/JP optimization
2. **v0.16+**: Zero-distance collapse (JJ to nothing when target is next instruction)
3. **Future**: Profile-guided JJ optimization based on runtime branch statistics

---

*Part of the MinZ Toolchain - Zero-Cost Abstractions for Z80*
