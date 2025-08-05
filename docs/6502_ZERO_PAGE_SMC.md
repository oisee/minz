# 6502 Zero-Page SMC Optimization

## Overview
The 6502 processor's zero-page addressing mode provides a unique opportunity for highly efficient self-modifying code (SMC). By placing function parameters and frequently accessed variables in zero-page ($00-$FF), we can achieve:

1. **Faster Access**: Zero-page addressing is 1 cycle faster than absolute
2. **Smaller Code**: 2-byte instructions instead of 3-byte
3. **Natural SMC**: Self-modifying zero-page locations is a common 6502 idiom

## Zero-Page Memory Map

```
$00-$0F: Scratch/temporary registers
$10-$1F: Function parameters (SMC)
$20-$3F: Fast global variables (@zeropage)
$40-$7F: User-defined zero-page variables
$80-$8F: Iterator state (for zero-cost iterators)
$90-$9F: String/array pointers
$A0-$EF: Platform-specific (varies by system)
$F0-$FF: System use (stack pointer region)
```

## Implementation Strategy

### 1. Function Parameter Passing
Instead of stack-based parameter passing, use zero-page SMC:

```minz
fun draw_sprite(x: u8, y: u8, sprite: *u8) -> void {
    // Parameters are pre-loaded in zero-page
}
```

Generated 6502 code:
```asm
; Caller patches zero-page before call
LDA #10
STA $10      ; x parameter
LDA #20
STA $11      ; y parameter
LDA #<sprite_data
STA $12      ; sprite pointer low
LDA #>sprite_data
STA $13      ; sprite pointer high
JSR draw_sprite

; Function reads from zero-page
draw_sprite:
    LDA $10  ; x coordinate
    STA screen_x
    LDA $11  ; y coordinate
    STA screen_y
    ; ... rest of function
    RTS
```

### 2. Fast Local Variables
Allocate frequently used locals in zero-page:

```minz
fun game_loop() -> void {
    @zeropage let player_x: u8;
    @zeropage let player_y: u8;
    @zeropage let score: u16;
    
    while true {
        // Direct zero-page access
        player_x = player_x + 1;  // INC $20
    }
}
```

### 3. Iterator Optimization
Zero-page is perfect for iterator state:

```minz
// Iterator chain
enemies.filter(|e| e.alive)
       .map(|e| e.update())
       .forEach(|e| e.draw());
```

Generated code uses zero-page for iterator:
```asm
; Iterator state in zero-page
; $80 = current pointer low
; $81 = current pointer high  
; $82 = remaining count
; $83 = stride (element size)

iterator_loop:
    LDY #0
    LDA ($80),Y    ; Load enemy.alive field
    BEQ skip       ; Skip if not alive
    
    ; Process enemy...
    
skip:
    ; Advance pointer using zero-page arithmetic
    LDA $80
    CLC
    ADC $83        ; Add stride
    STA $80
    BCC no_carry
    INC $81
no_carry:
    DEC $82        ; Decrement count
    BNE iterator_loop
```

### 4. SMC Call Optimization
For frequently called functions, use zero-page SMC for the call address:

```asm
; Self-modifying JSR in zero-page
call_vector = $40

; Initialize
LDA #<function_address
STA call_vector+1
LDA #>function_address
STA call_vector+2

; Zero-page contains: JSR $0000
call_vector:
    JSR $0000      ; This address is patched
    
; To call different functions dynamically:
LDA #<new_function
STA call_vector+1
LDA #>new_function  
STA call_vector+2
JSR call_vector
```

## MinZ Language Extensions

### @zeropage Attribute
```minz
@zeropage global fast_counter: u8;
@zeropage global sprite_ptr: *u8;

fun game_tick() -> void {
    @zeropage let temp: u8;
    // Compiler allocates these in zero-page
}
```

### Zero-Page Arrays
```minz
@zeropage global lookup: [u8; 16];  // 16-byte lookup table in ZP
```

### Platform-Specific Allocation
```minz
@target("6502") {
    @zeropage(0x40) global custom_var: u8;  // Force specific address
}
```

## Code Generation Changes

### 1. Parameter Allocation
```go
func (g *M6502Generator) allocateParameters(fn *ir.Function) {
    zpOffset := 0x10  // Start of parameter area
    for _, param := range fn.Params {
        param.Location = ZeroPageLocation(zpOffset)
        zpOffset += param.Type.Size()
    }
}
```

### 2. Zero-Page Tracking
```go
type ZeroPageAllocator struct {
    used      [256]bool
    variables map[string]uint8  // Variable -> ZP address
}

func (zp *ZeroPageAllocator) Allocate(size int) (uint8, error) {
    // Find contiguous free space in zero-page
}
```

### 3. Instruction Selection
```go
// Prefer zero-page addressing when possible
if isZeroPage(addr) {
    // LDA $10 instead of LDA $1000
    emit("LDA $%02X", addr)
} else {
    emit("LDA $%04X", addr) 
}
```

## Performance Benefits

### Cycle Savings
- Zero-page load: 3 cycles vs 4 cycles (absolute)
- Zero-page store: 3 cycles vs 4 cycles (absolute)
- Zero-page RMW: 5 cycles vs 6 cycles (absolute)

### Code Size Savings
- Zero-page instruction: 2 bytes
- Absolute instruction: 3 bytes
- 33% code size reduction for memory operations

### Example: Array Sum
```minz
fun sum_array(arr: [u8; 100]) -> u16 {
    @zeropage let sum: u16 = 0;
    @zeropage let i: u8 = 0;
    
    while i < 100 {
        sum = sum + arr[i];
        i = i + 1;
    }
    return sum;
}
```

Traditional: ~2000 cycles
With zero-page: ~1400 cycles (30% faster)

## Integration with MinZ Features

### 1. Error Propagation
Use zero-page for error state:
```asm
ERROR_FLAG = $0F  ; Global error flag in zero-page

; Fast error checking
BIT $0F
BMI handle_error  ; Branch if error (bit 7 set)
```

### 2. String Operations
Zero-page pointers for string manipulation:
```asm
STR_PTR = $90    ; String pointer
STR_LEN = $92    ; String length

; Fast string iteration
LDY #0
loop:
    LDA (STR_PTR),Y
    ; Process character
    INY
    CPY STR_LEN
    BNE loop
```

### 3. Metaprogramming
Generate optimal zero-page layouts at compile time:
```minz
@minz[[[
    // Analyze variable usage and generate optimal ZP layout
    let hot_vars = analyze_usage();
    for (var, addr) in allocate_zeropage(hot_vars) {
        @emit("@zeropage({addr}) global {var.name}: {var.type};")
    }
]]]
```

## Platform Considerations

Different 6502 systems have different zero-page usage:

### Commodore 64
- $00-$01: I/O port registers
- $02-$8F: Available
- $90-$FF: KERNAL workspace

### Apple II
- $00-$1F: Monitor workspace
- $20-$4F: Available
- $50-$FF: System use

### NES
- $00-$07: Temporary
- $08-$0F: Available
- $10-$FF: Various PPU/APU temps

The compiler should have platform profiles to avoid conflicts.

## Future Enhancements

1. **Auto-promotion**: Automatically promote hot variables to zero-page
2. **Lifetime Analysis**: Reuse zero-page locations based on variable lifetime
3. **Interprocedural Optimization**: Share zero-page across functions
4. **Profile-Guided**: Use runtime profiling to optimize zero-page allocation

## Conclusion

Zero-page SMC optimization can provide 20-40% performance improvement for typical 6502 programs. Combined with MinZ's high-level abstractions, this enables writing clean, maintainable code that rivals hand-optimized assembly in performance.