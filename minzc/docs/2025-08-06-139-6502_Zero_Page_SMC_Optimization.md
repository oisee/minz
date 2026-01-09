# 139: 6502 Zero-Page SMC Optimization

## Date: 2025-08-06

This document describes the implementation of zero-page SMC (Self-Modifying Code) optimization for the 6502 backend, leveraging the 6502's unique zero-page addressing mode for significant performance gains.

## Overview

The 6502 processor has a special "zero page" (addresses $00-$FF) that can be accessed with shorter, faster instructions. By placing SMC parameters and frequently-used virtual registers in zero page, we achieve:

1. **Faster access**: 3 cycles vs 4 cycles for absolute addressing
2. **Smaller code**: 2-byte instructions vs 3-byte instructions
3. **Special instructions**: INC/DEC work directly on zero page
4. **Indirect addressing**: (zp),Y mode for efficient pointer operations

## Zero-Page Memory Layout

```
$00-$7F: Virtual Registers (128 bytes)
  - 64 virtual registers, 2 bytes each
  - Most frequently used IR registers mapped here
  
$80-$9F: SMC Parameters (32 bytes)
  - 16 parameter slots, 2 bytes each
  - Function parameters for SMC-enabled functions
  
$A0-$BF: TSMC Anchors (32 bytes)
  - 16 TRUE SMC anchor points
  - Self-modifying instruction operands
  
$C0-$EF: Reserved (48 bytes)
  - Future use: stack frame, temporaries
  
$F0-$FF: Pointer Operations (16 bytes)
  - 8 pointer pairs for indirect addressing
```

## Implementation Details

### 1. Zero-Page Allocation

The `M6502SMCOptimizer` manages zero-page allocation:

```go
type M6502SMCOptimizer struct {
    virtualRegBase   byte  // $00
    smcParamBase     byte  // $80
    tsmcAnchorBase   byte  // $A0
    
    regToZeroPage    map[ir.Register]byte
    paramToZeroPage  map[string]byte
    anchorToZeroPage map[string]byte
}
```

### 2. Enhanced Parameter Passing

SMC-enabled functions receive parameters directly in zero page:

```asm
; Traditional parameter passing
lda param_value
sta param_location
jsr function

; Zero-page SMC parameter passing
lda param_value
sta $80          ; Direct to zero page
jsr function     ; Function reads from $80
```

### 3. Optimized Instruction Patterns

#### Pattern 1: Direct INC/DEC
```asm
; Traditional increment
lda counter
clc
adc #1
sta counter

; Zero-page increment
inc $00          ; Single instruction!
```

#### Pattern 2: Fast Loops
```asm
; Traditional loop counter
loop:
    lda i
    cmp limit
    bcc continue
    
; Zero-page loop
loop:
    dec $01      ; Counter in zero page
    bne loop     ; Automatic flag setting
```

#### Pattern 3: Indirect Addressing
```asm
; Array access with zero-page pointer
lda #<array
sta $F0
lda #>array
sta $F1
ldy index
lda ($F0),y      ; Efficient array access
```

## Performance Analysis

### Cycle Savings

| Operation | Traditional | Zero-Page | Savings |
|-----------|------------|-----------|---------|
| Load variable | 4 cycles | 3 cycles | 25% |
| Store variable | 4 cycles | 3 cycles | 25% |
| Increment | 10 cycles | 5 cycles | 50% |
| Compare | 4 cycles | 3 cycles | 25% |
| Parameter pass | 8 cycles | 3 cycles | 62.5% |

### Code Size Savings

| Instruction | Traditional | Zero-Page | Savings |
|-------------|------------|-----------|---------|
| LDA abs | 3 bytes | 2 bytes | 33% |
| STA abs | 3 bytes | 2 bytes | 33% |
| INC abs | 3 bytes | 2 bytes | 33% |
| CMP abs | 3 bytes | 2 bytes | 33% |

## Example: Optimized Function

```minz
fun sum_range_smc(start: u8, count: u8) -> u8 {
    var sum: u8 = 0;
    var i: u8 = 0;
    
    while i < count {
        sum = sum + start + i;
        i = i + 1;
    }
    
    return sum;
}
```

Generates optimized 6502 code:
```asm
sum_range_smc:
    ; Parameters already in zero page
    ; start at $85, count at $86
    
    lda #0
    sta $00      ; sum in zero page
    sta $01      ; i in zero page
    
loop:
    lda $01      ; Load i
    cmp $86      ; Compare with count
    bcs done
    
    ; sum = sum + start + i
    lda $00      ; Load sum
    clc
    adc $85      ; Add start
    adc $01      ; Add i
    sta $00      ; Store back to sum
    
    inc $01      ; i++
    jmp loop
    
done:
    lda $00      ; Return sum
    rts
```

## Advanced Optimizations

### 1. Parameter Usage Analysis

The enhancement tracks how parameters are used:
- Frequently accessed parameters kept in X/Y registers
- Loop variables optimized for Y register indexing
- Read-only parameters can stay in zero page

### 2. Zero-Page to Zero-Page Transfers

Direct transfers without using accumulator:
```asm
; Planned optimization for 65C02
lda $80
sta $81
; Becomes:
; Future: Direct ZP-to-ZP transfer instruction
```

### 3. Compound Operations

Combining multiple operations on zero-page variables:
```asm
; Multiple increments detected
inc $00
inc $00
; Could become:
lda $00
clc
adc #2
sta $00
```

## Integration with TSMC

TRUE SMC on 6502 can patch zero-page locations for ultimate performance:

```asm
; TSMC anchor in zero page
modify_increment:
    lda $A0      ; Self-modifying zero-page load
    clc
    adc #$00     ; This immediate will be patched
    sta $A0
    rts
```

## Future Enhancements

1. **65C02 Instructions**: Use enhanced opcodes like STZ, BRA
2. **Stack in Zero Page**: Fast local variables
3. **Coroutine Support**: Context switching via zero page
4. **Interrupt Handlers**: Ultra-fast ISRs using zero page

## Conclusion

Zero-page SMC optimization provides significant performance improvements for 6502 targets:
- 25-62% cycle reduction for common operations
- 33% code size reduction
- Natural fit with 6502 architecture
- Foundation for advanced optimizations

This positions MinZ as a highly competitive language for 6502-based systems (Apple II, Commodore 64, NES), offering modern language features with assembly-level performance.