# INC/DEC Optimization Analysis for Z80

## Key Discovery

The effectiveness of INC/DEC optimization varies dramatically based on which register is being modified. For the A register, only ±1 is beneficial. For all other registers (B, C, D, E, H, L), INC/DEC is beneficial up to ±3!

## Cycle Analysis

### For A Register

```asm
; A = A + 1
INC A         ; 4 cycles
; vs
ADD A, 1      ; 7 cycles
; Savings: 3 cycles (43% faster)

; A = A + 2  
INC A         ; 4 cycles
INC A         ; 4 cycles = 8 total
; vs
ADD A, 2      ; 7 cycles
; ADD is faster by 1 cycle
```

**Conclusion for A**: Only ±1 should use INC/DEC

### For Other Registers (B, C, D, E, H, L)

The situation is completely different because ADD only works with A register!

```asm
; B = B + 1
INC B         ; 4 cycles
; vs
LD A, B       ; 4 cycles
ADD A, 1      ; 7 cycles  
LD B, A       ; 4 cycles = 15 total
; Savings: 11 cycles (73% faster)

; B = B + 2
INC B         ; 4 cycles
INC B         ; 4 cycles = 8 total
; vs
LD A, B       ; 4 cycles
ADD A, 2      ; 7 cycles
LD B, A       ; 4 cycles = 15 total  
; Savings: 7 cycles (47% faster)

; B = B + 3
INC B         ; 4 cycles
INC B         ; 4 cycles
INC B         ; 4 cycles = 12 total
; vs
LD A, B       ; 4 cycles
ADD A, 3      ; 7 cycles
LD B, A       ; 4 cycles = 15 total
; Savings: 3 cycles (20% faster)

; B = B + 4
INC B × 4     ; 16 cycles
; vs
LD A, B; ADD A, 4; LD B, A  ; 15 cycles
; ADD is faster by 1 cycle
```

**Conclusion for non-A registers**: INC/DEC is beneficial up to ±3

## Register Pressure Considerations

### When A is Already in Use

If A contains important data that must be preserved:

```asm
; Option 1: Using ADD (must save/restore A)
PUSH AF       ; 11 cycles
LD A, B       ; 4 cycles
ADD A, 2      ; 7 cycles
LD B, A       ; 4 cycles
POP AF        ; 10 cycles
; Total: 36 cycles!

; Option 2: Using INC
INC B         ; 4 cycles
INC B         ; 4 cycles
; Total: 8 cycles (77% faster!)
```

### Memory Operands

For memory increments, the difference is even more dramatic:

```asm
; (HL) = (HL) + 2
INC (HL)      ; 11 cycles
INC (HL)      ; 11 cycles = 22 total

; vs
LD A, (HL)    ; 7 cycles
ADD A, 2      ; 7 cycles
LD (HL), A    ; 7 cycles = 21 total

; Nearly identical, but INC preserves A register!
```

## Optimization Strategy

### Thresholds by Register Type

| Register | Optimal INC/DEC Range | Reason |
|----------|---------------------|---------|
| A | ±1 only | ADD A,n is available |
| B,C,D,E,H,L | ±1 to ±3 | Requires 3 instructions for ADD |
| (HL) | ±1 to ±2 | Similar cycles but saves A |
| (IX+d),(IY+d) | ±1 only | Too slow (23 cycles each) |

### Decision Matrix

```
if (target == A) {
    use_inc_dec = (abs(delta) == 1)
} else if (target in [B,C,D,E,H,L]) {
    use_inc_dec = (abs(delta) <= 3)
} else if (target == (HL)) {
    use_inc_dec = (abs(delta) <= 2)
} else {  // IX+d, IY+d
    use_inc_dec = (abs(delta) == 1)
}
```

## Implementation in MinZ

### Current Implementation (Too Conservative)

```go
// Only handles ±1 for all registers
if inst1.Imm == 1 || inst1.Imm == -1 {
    // Use INC/DEC
}
```

### Optimized Implementation

```go
func shouldUseIncDec(target Register, delta int64) bool {
    absDelta := abs(delta)
    
    switch target {
    case RegA:
        return absDelta == 1
    case RegB, RegC, RegD, RegE, RegH, RegL:
        return absDelta <= 3
    case RegHL_Indirect:
        return absDelta <= 2
    case RegIX_Indirect, RegIY_Indirect:
        return absDelta == 1
    default:
        return false
    }
}
```

## Performance Impact

For typical code with many increments by 2 or 3:

- Non-A register ±2: 47% faster with INC/DEC
- Non-A register ±3: 20% faster with INC/DEC
- Preserves A register (critical for register allocation)
- Reduces register pressure significantly

## Examples

### Loop Counter
```minz
for i in 0..10 {
    // If i is in register B
    // i += 2 compiles to:
    INC B
    INC B    ; 8 cycles instead of 15
}
```

### Pointer Arithmetic
```minz
ptr += 3;  // If ptr is in HL
// Compiles to:
INC HL    ; 6 cycles
INC HL    ; 6 cycles  
INC HL    ; 6 cycles = 18 total
// vs
// LD BC, 3; ADD HL, BC = 10 + 11 = 21 cycles
```

## Conclusion

The INC/DEC optimization is much more powerful than initially implemented. By expanding it to handle up to ±3 for non-A registers, we can achieve significant performance improvements and better register allocation. This optimization is especially valuable in register-constrained situations where preserving the A register is crucial.