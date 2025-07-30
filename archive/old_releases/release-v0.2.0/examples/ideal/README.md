# Ideal MinZ Code Generation Examples

This directory contains hand-optimized assembly code showing what the MinZ compiler should ideally generate for each example. These demonstrate perfect SMC (Self-Modifying Code) style with minimal overhead.

## Key Optimization Principles

1. **Direct Parameter Usage** - SMC parameters are used immediately after loading, no intermediate storage
2. **No Stack Frames** - No IX register usage or stack frame setup
3. **Efficient Register Usage** - Values kept in registers as long as possible
4. **Tail Call Optimization** - Tail recursive calls become jumps
5. **Smart Type Handling** - u8 parameters handled efficiently without unnecessary conversions

## Examples

### simple_add_ideal.a80
Shows perfect parameter passing and arithmetic:
- Parameters loaded directly into registers
- No memory stores/loads
- Caller modifies SMC slots before calling
- Total: 8 instructions for add function

### fibonacci_ideal.a80
Efficient iterative Fibonacci:
- Uses DJNZ for fast loop counting
- Clever register swapping with PUSH/POP
- Handles edge cases (n=0,1) efficiently

### tail_sum_ideal.a80
Demonstrates tail recursion optimization:
- Tail call becomes JR (jump) not CALL
- Parameters updated in-place
- No stack usage for recursion

### game_sprite_ideal.a80
Efficient sprite drawing:
- Complex address calculation optimized
- Uses DJNZ for sprite loop
- Direct memory-to-screen copying

### screen_color_ideal.a80
Attribute manipulation example:
- Bit manipulation for color attributes
- Efficient memory filling
- Hardware I/O for border color

## Comparison with Current Compiler

Current compiler generates:
```asm
add_param_a:
    LD HL, #0000   ; Load parameter
    LD ($F006), HL ; Store to memory (wasteful!)
    ; ... later ...
    LD HL, ($F006) ; Load back (wasteful!)
```

Ideal code:
```asm
add_param_a:
    LD HL, #0000   ; Load parameter
    LD D, H        ; Use directly!
    LD E, L
```

The ideal code eliminates unnecessary memory traffic and uses registers efficiently.