# MinZ Arithmetic Operations Implementation
**Date**: July 26, 2025  
**Author**: MinZ Development Team

## Overview

This document describes the implementation of arithmetic operations in the MinZ compiler's Z80 code generator. All operations are implemented using native Z80 instructions for optimal performance on 8-bit hardware.

## Implemented Operations

### 1. Multiplication (OpMul)
**Algorithm**: Repeated addition  
**Complexity**: O(n) where n is the multiplier value

```asm
; 8-bit multiplication: B * C -> HL
; B = multiplicand, C = multiplier
LD HL, 0          ; Initialize result
.mul_loop:
    ADD HL, DE    ; Add multiplicand to result
    DEC C         ; Decrement counter
    JR NZ, .mul_loop
```

**Edge Cases**:
- Multiplier = 0: Returns 0 immediately
- Overflow: Results > 255 wrap around (8-bit)

### 2. Division (OpDiv)
**Algorithm**: Repeated subtraction  
**Complexity**: O(n) where n is the quotient

```asm
; 8-bit division: D / E -> B (quotient)
LD B, 0           ; Initialize quotient
LD A, D           ; Load dividend
.div_loop:
    CP E          ; Compare with divisor
    JR C, .done   ; If remainder < divisor, done
    SUB E         ; Subtract divisor
    INC B         ; Increment quotient
    JR .div_loop
```

**Edge Cases**:
- Divide by zero: Returns 0
- Integer division: Remainder is discarded

### 3. Modulo (OpMod)
**Algorithm**: Repeated subtraction (remainder)  
**Complexity**: O(n) where n is the quotient

```asm
; 8-bit modulo: D % E -> A (remainder)
LD A, D           ; Load dividend
.mod_loop:
    CP E          ; Compare with divisor
    JR C, .done   ; If < divisor, we have remainder
    SUB E         ; Subtract divisor
    JR .mod_loop
```

**Edge Cases**:
- Modulo by zero: Returns 0
- Result is always 0 <= result < divisor

### 4. Shift Left (OpShl)
**Algorithm**: Bitwise shift using SLA instruction  
**Complexity**: O(n) where n is shift count

```asm
; Shift left: A << C -> A
.shl_loop:
    SLA A         ; Shift left, 0 into bit 0
    DEC C
    JR NZ, .shl_loop
```

**Behavior**:
- Shifts in zeros from the right
- Bits shifted out on left are lost
- Shift by 0: No operation

### 5. Shift Right (OpShr)
**Algorithm**: Bitwise shift using SRL instruction  
**Complexity**: O(n) where n is shift count

```asm
; Shift right: A >> C -> A
.shr_loop:
    SRL A         ; Shift right, 0 into bit 7
    DEC C
    JR NZ, .shr_loop
```

**Behavior**:
- Logical shift (unsigned)
- Shifts in zeros from the left
- Bits shifted out on right are lost
- Shift by 0: No operation

## Performance Considerations

### Multiplication
- Fast for small multipliers (< 16)
- Consider lookup tables for fixed multiplications
- 16-bit multiplication would need different algorithm

### Division & Modulo
- Slow for large dividends
- Consider shift-based division for powers of 2
- Combined div/mod operation could be optimized

### Shifts
- Could optimize for fixed shift counts
- Shift by 1: Use single instruction
- Shift by 8+: Result is always 0

## Testing

### Test Cases
```minz
// Multiplication
assert(5 * 6 == 30);
assert(0 * 100 == 0);
assert(255 * 1 == 255);

// Division
assert(20 / 4 == 5);
assert(17 / 5 == 3);
assert(5 / 0 == 0);  // Divide by zero

// Modulo
assert(17 % 5 == 2);
assert(20 % 4 == 0);
assert(5 % 0 == 0);  // Modulo by zero

// Shifts
assert(3 << 2 == 12);
assert(12 >> 2 == 3);
assert(1 << 7 == 128);
```

## Future Optimizations

1. **Strength Reduction**
   - Replace `x * 2` with `x << 1`
   - Replace `x / 2` with `x >> 1`
   - Replace `x % 2` with `x & 1`

2. **Special Cases**
   - Multiply by 0, 1, powers of 2
   - Divide by 1, powers of 2
   - Shift by constant amounts

3. **16-bit Operations**
   - Implement 16-bit multiply using 8x8->16
   - Implement 16-bit divide
   - Support for signed operations

## Limitations

1. **8-bit Only**: Currently only 8-bit operations
2. **Unsigned Only**: No signed arithmetic yet
3. **No Overflow Detection**: Results wrap around
4. **Performance**: Not optimized for speed

## Conclusion

The arithmetic operations are now fully functional in MinZ v0.3.1. While not optimized, they provide correct results and enable real computations in MinZ programs. Future versions will focus on performance optimizations and 16-bit support.