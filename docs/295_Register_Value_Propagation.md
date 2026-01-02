# ADR: Register Value Propagation

**Status**: Proposed
**Date**: 2025-12-22
**Location**: `minzc/pkg/optimizer/assembly_peephole.go`

## Context

The Z80 has limited registers and frequently loads immediate values. When a register already contains a value close to what we need, we can use `INC`/`DEC` instead of loading a new immediate:

```asm
LD B, $FE       ; B = $FE (2 bytes)
; ... code that doesn't modify B ...
LD B, $FF       ; 2 bytes - but B is already $FE!
                ; Could be: INC B (1 byte)
```

This optimization builds on the "A = 0" tracking from ADR-294, extending it to track known values in all registers.

## Decision

Implement a **Register Value Propagation** pass that:

1. Tracks known constant values in registers A, B, C, D, E, H, L
2. When encountering `LD r, imm`:
   - If `r` already contains `imm`: eliminate the instruction entirely
   - If `r` contains `imm - 1`: replace with `INC r`
   - If `r` contains `imm + 1`: replace with `DEC r`
   - Otherwise: update tracking with new value
3. Invalidates tracking appropriately (labels, calls, jumps, etc.)

### Byte Savings

| Scenario | Before | After | Savings |
|----------|--------|-------|---------|
| Same value | `LD B, $42` (2B) | *eliminated* | 2 bytes |
| Value + 1 | `LD B, $43` (2B) | `INC B` (1B) | 1 byte |
| Value - 1 | `LD B, $41` (2B) | `DEC B` (1B) | 1 byte |

### Wrapping Behavior

Z80 INC/DEC wrap naturally:
- `$FF + 1 = $00` (INC from $FF gives $00)
- `$00 - 1 = $FF` (DEC from $00 gives $FF)

This is correct 8-bit arithmetic, so the optimization is safe.

## Implementation

### Data Structure

```go
type RegisterState struct {
    known bool   // Is the value known?
    value uint8  // The known value (if known == true)
}

type RegisterTracker struct {
    regs map[string]*RegisterState  // A, B, C, D, E, H, L
}
```

### Instructions That SET Register Values

| Instruction | Effect |
|-------------|--------|
| `LD r, imm` | r = imm (known) |
| `LD r, r'` | r = r' (copy known state) |
| `XOR A` | A = 0 (known) |
| `INC r` | r = r + 1 (if known, stays known) |
| `DEC r` | r = r - 1 (if known, stays known) |
| `LD r, (addr)` | r = unknown |
| `LD r, (HL)` | r = unknown |

### Instructions That INVALIDATE Tracking

| Instruction | Effect |
|-------------|--------|
| Labels | All registers unknown (jump target) |
| `CALL` | All registers unknown (callee may modify) |
| `JP`/`JR` | All registers unknown |
| `RET` | All registers unknown |
| `POP rr` | Both registers unknown |
| `EX` instructions | Affected registers unknown |
| Any instruction modifying r | r becomes unknown (unless we can track it) |

### Algorithm

```go
func (p *AssemblyPeepholePass) propagateRegisterValues(lines []string) []string {
    tracker := NewRegisterTracker()
    result := []string{}

    for _, line := range lines {
        // Check if this is LD r, imm
        if match := ldImmPattern.FindStringSubmatch(line); match != nil {
            reg, imm := match[1], parseImm(match[2])

            if state := tracker.Get(reg); state.known {
                if state.value == imm {
                    // Same value - eliminate entirely
                    continue
                } else if state.value == imm-1 {
                    // Replace with INC
                    result = append(result, fmt.Sprintf("    INC %s    ; Was LD %s, $%02X (value propagation)", reg, reg, imm))
                    tracker.Set(reg, imm)
                    continue
                } else if state.value == imm+1 {
                    // Replace with DEC
                    result = append(result, fmt.Sprintf("    DEC %s    ; Was LD %s, $%02X (value propagation)", reg, reg, imm))
                    tracker.Set(reg, imm)
                    continue
                }
            }

            // Can't optimize - update tracker
            tracker.Set(reg, imm)
            result = append(result, line)
            continue
        }

        // Handle other instructions...
        updateTrackerForInstruction(tracker, line)
        result = append(result, line)
    }

    return result
}
```

## Safety Considerations

### 1. SMC (Self-Modifying Code)

Instructions after labels might be SMC patch targets. We must NOT optimize them:
```asm
my_param$immOP:
    LD A, $00       ; DO NOT optimize - will be patched at runtime!
```

**Rule**: After any label, invalidate all tracking AND skip optimization for the next instruction.

### 2. Flag Side Effects

`INC`/`DEC` modify flags differently than `LD`:
- `LD r, imm` does NOT affect flags
- `INC r` affects S, Z, H, P/V (but NOT carry)
- `DEC r` affects S, Z, H, P/V (but NOT carry)

**Rule**: If the next instruction uses flags (conditional jump, ADC, SBC, etc.), do NOT replace `LD` with `INC`/`DEC`.

This requires look-ahead or conservative behavior:
- **Conservative**: Only optimize if next instruction is known flag-safe
- **Aggressive**: Optimize and trust that flag-dependent code is rare after LD

For v1, use **conservative** approach.

### 3. Register Pairs

`LD HL, imm16` is a 16-bit load. We track H and L separately:
```asm
LD HL, $1234    ; H = $12, L = $34
LD H, $12       ; Redundant! H already = $12
```

## Alternatives Considered

### 1. Full SSA-based Value Numbering

Build SSA form, do global value numbering, then lower back to assembly.

**Rejected**: Too complex for peephole optimization. Better suited for MIR level.

### 2. Only Track Zero

Only track "register = 0" (what we have now in ADR-294).

**Rejected**: Leaves significant optimization opportunities on the table.

### 3. Track Last N Values

Track a history of values to handle patterns like:
```asm
LD B, $10
LD B, $20
LD B, $10    ; Could use previous value if we tracked history
```

**Deferred**: Adds complexity. Single-value tracking covers most cases.

## Consequences

### Positive
- Saves 1-2 bytes per optimized instruction
- Simple tracking logic
- Builds foundation for more sophisticated optimizations

### Negative
- Must be careful with flags
- SMC requires special handling
- Tracking overhead (minimal - just a map of 7 registers)

### Neutral
- INC/DEC are same speed as LD for most registers (4 T-states)

## Test Cases

```asm
; Test 1: Eliminate redundant load
    LD B, $42
    LD B, $42       ; Should be eliminated

; Test 2: INC optimization
    LD C, $FE
    LD C, $FF       ; Should become: INC C

; Test 3: DEC optimization
    LD D, $10
    LD D, $0F       ; Should become: DEC D

; Test 4: Wrapping
    LD E, $FF
    LD E, $00       ; Should become: INC E

; Test 5: Label invalidates
label:
    LD H, $00       ; Should NOT be optimized (after label)

; Test 6: Copy propagation
    LD A, $42
    LD B, A         ; B = $42 (known)
    LD B, $42       ; Should be eliminated
```

## Implementation Phases

### Phase 1: Basic Value Tracking
- Track `LD r, imm` values
- Implement INC/DEC optimization
- Invalidate on labels

### Phase 2: Copy Propagation
- Track `LD r, r'` copies
- Handle `XOR A` (A = 0)

### Phase 3: Flag Safety
- Look-ahead for flag-using instructions
- Conservative skip when unsure

### Phase 4: 16-bit Registers
- Track HL, DE, BC as pairs
- Optimize `LD rr, imm16`

## References

- Z80 instruction set: `INC r` = 1 byte, 4 T-states
- Z80 instruction set: `DEC r` = 1 byte, 4 T-states
- Z80 instruction set: `LD r, imm` = 2 bytes, 7 T-states
- ADR-294: Consecutive LD Zero Optimization (predecessor)
