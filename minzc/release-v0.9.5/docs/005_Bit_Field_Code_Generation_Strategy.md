# 025: Bit Field Code Generation Strategy

## Z80 Bit Manipulation Instructions Analysis

### BIT/SET/RES Instructions
- **BIT n,r**: Test bit n of register r (4 T-states)
- **SET n,r**: Set bit n of register r to 1 (8 T-states)
- **RES n,r**: Reset bit n of register r to 0 (8 T-states)
- **Limitation**: Only work on single bits, bit position must be constant

### AND/OR/Shift Approach
- **AND mask**: Clear bits (4 T-states)
- **OR mask**: Set bits (4 T-states)
- **SRL/RLC/RRC**: Shift/rotate operations (8 T-states each)

## Comparison for Common Operations

### Reading a 3-bit field at position 2
```asm
; Using shifts and AND (12 T-states)
LD A, source     ; Get value
SRL A           ; Shift right twice
SRL A
AND 0x07        ; Mask to 3 bits

; BIT/SET/RES cannot extract multi-bit fields!
```

### Writing a 3-bit field at position 2
```asm
; Using AND/OR (12 T-states)
LD A, source     ; Get current value
AND 0xE3        ; Clear bits 2-4 (11100011)
LD B, A         ; Save cleared value
LD A, new_value ; Get new 3-bit value
SLA A           ; Shift left twice
SLA A
OR B            ; Combine

; Using SET/RES would need 3 operations per bit = 24 T-states!
```

### Single bit operations
```asm
; Setting bit 6 - SET is actually good here
SET 6, A        ; 8 T-states

; vs AND/OR
OR 0x40         ; 7 T-states (slightly faster!)

; Testing bit 6
BIT 6, A        ; 8 T-states, sets Z flag

; vs AND
AND 0x40        ; 7 T-states, but modifies A
```

## Recommended Strategy

### For MinZ Bit Fields:

1. **Multi-bit fields (common case)**: Always use AND/OR with shifts
   - More flexible (bit width can be variable)
   - Often faster
   - Can handle any bit width

2. **Single-bit fields**: Use OR/AND for set/clear, BIT for testing
   - OR to set: `OR mask`
   - AND to clear: `AND ~mask`
   - BIT to test without modifying: `BIT n, r`

3. **Optimization opportunities**:
   - Fields at bit 0: No shift needed
   - Power-of-2 aligned fields: Can use rotate instead of shift
   - Consecutive fields: Can be processed together

## Code Generation Examples

### 8-bit ScreenAttr Example
```minz
type ScreenAttr = bits {
    ink: 3,      // Bits 0-2
    paper: 3,    // Bits 3-5
    bright: 1,   // Bit 6
    flash: 1     // Bit 7
};
```

#### Reading ink (bits 0-2, no shift needed):
```asm
LD A, (attr_addr)
AND 0x07         ; Mask for 3 bits
; Result in A
```

#### Reading paper (bits 3-5):
```asm
LD A, (attr_addr)
SRL A
SRL A
SRL A
AND 0x07         ; Mask for 3 bits
; Result in A
```

#### Setting paper to value in B (bits 3-5):
```asm
LD A, (attr_addr)
AND 0xC7         ; Clear bits 3-5 (11000111)
LD C, A          ; Save
LD A, B          ; New value
SLA A
SLA A
SLA A
OR C             ; Combine
LD (attr_addr), A
```

#### Setting bright bit:
```asm
LD A, (attr_addr)
OR 0x40          ; Set bit 6
LD (attr_addr), A
```

### 16-bit Fields
For 16-bit bit structs, we'll use HL register pair and similar techniques, but with 16-bit operations where possible.

## Conclusion

**AND/OR with shifts is the better choice** for MinZ bit field implementation because:
1. Works for any bit width (not just single bits)
2. Often faster or equal performance
3. More consistent code generation
4. Can be optimized for special cases
5. Maps better to how programmers think about bit fields

BIT/SET/RES are really only optimal for single-bit boolean flags, and even then, OR/AND can be just as fast.