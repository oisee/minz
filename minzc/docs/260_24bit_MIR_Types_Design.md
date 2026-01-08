# 24-bit MIR Types Design

## Overview

This document specifies native 24-bit integer types for MIR (MinZ Intermediate Representation), enabling efficient targeting of:
- **eZ80** (native 24-bit ADL mode)
- **68000/68040** (can use lower 24-bits of 32-bit regs)
- **Z80** (synthesized from 8/16-bit operations)
- **Modern CPUs** (trivial - just mask to 24 bits)

## Type Definitions

### New Types

| Type | Size | Range | Use Case |
|------|------|-------|----------|
| `u24` | 3 bytes | 0 to 16,777,215 | Addresses, large counters |
| `i24` | 3 bytes | -8,388,608 to 8,388,607 | Signed arithmetic |

### IR Type System (Already Implemented)

```go
// pkg/ir/ir.go - Types already exist!
const (
    TypeU24 TypeKind = ... // Unsigned 24-bit
    TypeI24 TypeKind = ... // Signed 24-bit
)

func (t *BasicType) Size() int {
    case TypeU24, TypeI24:
        return 3  // 3 bytes
}
```

## MIR Instructions

### Load/Store Operations

```mir
; 24-bit immediate load
r1 = 0x123456          ; Load 24-bit immediate

; 24-bit memory operations
r2 = *r1               ; Load 24-bit from address in r1
*r1 = r2               ; Store 24-bit to address in r1

; With explicit size annotation (for mixed-size code)
r3 = load.24 [r1]      ; Explicit 24-bit load
store.24 [r1], r3      ; Explicit 24-bit store
```

### Arithmetic Operations

```mir
; All standard ops work with 24-bit values
r3 = r1 + r2           ; 24-bit add (result masked to 24 bits)
r3 = r1 - r2           ; 24-bit subtract
r3 = r1 * r2           ; 24-bit multiply (lower 24 bits)
r3 = r1 / r2           ; 24-bit divide
r3 = r1 % r2           ; 24-bit modulo

; Overflow behavior: wrap around (modulo 2^24)
```

### Type Conversion

```mir
; Widening (zero-extend or sign-extend)
r2 = zext.24 r1        ; Zero-extend u8/u16 to u24
r2 = sext.24 r1        ; Sign-extend i8/i16 to i24

; Narrowing (truncate)
r2 = trunc.16 r1       ; Truncate u24/i24 to 16-bit
r2 = trunc.8 r1        ; Truncate to 8-bit
```

## Backend Mappings

### eZ80 (Native 24-bit) - ADL Mode

```asm
; Direct mapping - eZ80 has native 24-bit registers
; r1 = 0x123456
LD HL, $123456         ; 24-bit immediate load

; r2 = r1 + r3
ADD HL, BC             ; 24-bit add (HL = HL + BC)

; Memory access
LD HL, (addr)          ; 24-bit load
LD (addr), HL          ; 24-bit store
```

### Z80 (Synthesized from 16+8 bit)

```asm
; 24-bit value in HL (low 16) + A (high 8)
; Or use memory: [addr], [addr+1], [addr+2]

; r1 = 0x123456
LD HL, $3456           ; Low 16 bits
LD A, $12              ; High 8 bits
LD (r1_lo), HL
LD (r1_hi), A

; 24-bit add: r3 = r1 + r2
LD HL, (r1_lo)
LD DE, (r2_lo)
ADD HL, DE             ; Add low 16 bits
LD (r3_lo), HL
LD A, (r1_hi)
ADC A, (r2_hi)         ; Add high bytes with carry
LD (r3_hi), A
```

### 68000/68040 (32-bit registers, use lower 24)

```asm
; Use long registers, mask to 24 bits where needed
; r1 = 0x123456
MOVE.L #$00123456, D0  ; Load into 32-bit reg

; Arithmetic just works (mask on store if needed)
ADD.L D1, D0           ; 32-bit add
AND.L #$00FFFFFF, D0   ; Mask to 24 bits (if overflow matters)

; Memory (68000 has 24-bit address bus anyway)
MOVE.L (A0), D0        ; Load
MOVE.L D0, (A0)        ; Store
```

### MIR VM (64-bit host)

```go
// Already works - just mask results
func (vm *VM) execAdd24(dest, src1, src2 Register) {
    result := vm.registers[src1] + vm.registers[src2]
    vm.registers[dest] = result & 0xFFFFFF  // Mask to 24 bits
}
```

## MinZ Language Support

### Type Declaration

```minz
// New primitive types
let addr: u24 = 0x123456;
let offset: i24 = -1000;

// Array indexing with 24-bit
let bigArray: [u8; 1000000];  // Requires 24-bit index
let value = bigArray[addr];
```

### Platform-Specific Annotations

```minz
// Compile-time platform detection
@if(target.address_width >= 24) {
    let ptr: u24 = get_address();
} @else {
    // Fallback for 16-bit targets
    let ptr_lo: u16 = get_address_lo();
    let ptr_hi: u8 = get_address_hi();
}
```

### Automatic Type Promotion

```minz
// u16 + u16 that might overflow to u24
let a: u16 = 50000;
let b: u16 = 50000;
let c: u24 = a + b;  // Promoted to 24-bit result (100000)
```

## Implementation Plan

### Phase 1: MIR VM Support (Current)
- [x] Types defined in IR (TypeU24, TypeI24)
- [x] Size calculation (3 bytes)
- [x] Memory read/write supports variable sizes
- [ ] Add masking for arithmetic overflow
- [ ] Add explicit 24-bit instructions to VM

### Phase 2: Semantic Analyzer
- [ ] Accept u24/i24 in type declarations
- [ ] Type checking for 24-bit operations
- [ ] Automatic promotion rules

### Phase 3: Code Generation
- [ ] eZ80 backend (native 24-bit)
- [ ] Z80 backend (synthesized)
- [ ] 68K backend (use 32-bit regs)

### Phase 4: Optimization
- [ ] Constant folding for 24-bit
- [ ] Strength reduction
- [ ] Register allocation for 24-bit values on Z80

## Testing Strategy

```minz
// test_24bit.minz
fun test_24bit_arithmetic() -> bool {
    let a: u24 = 0x800000;  // 8,388,608
    let b: u24 = 0x800000;
    let c: u24 = a + b;     // Should be 0x000000 (overflow)
    return c == 0;
}

fun test_24bit_address() -> u8 {
    // Access memory beyond 64KB
    let addr: u24 = 0x100000;  // 1MB
    return peek(addr);
}
```

## Compatibility Notes

### Backward Compatibility
- Existing 8/16-bit code unchanged
- 24-bit types opt-in via explicit declaration
- MIR files without 24-bit ops run on all backends

### Forward Compatibility
- 32-bit types (u32/i32) can reuse this infrastructure
- Size field in instructions already supports arbitrary sizes
- VM registers are 64-bit internally

## Related Documents
- [198_VM_Bytecode_Targets_and_MIR_Runtime_Vision.md](198_VM_Bytecode_Targets_and_MIR_Runtime_Vision.md)
- [067_MinZ_Multi_Processor_Target_Analysis.md](067_MinZ_Multi_Processor_Target_Analysis.md)
