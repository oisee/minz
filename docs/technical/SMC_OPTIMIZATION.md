# Self-Modifying Code (SMC) Optimization in MinZ

## Overview

MinZ supports an advanced optimization technique using self-modifying code (SMC) that can significantly improve performance for Z80 programs. This optimization embeds frequently accessed values directly into instruction opcodes, eliminating memory accesses and reducing register pressure.

## How It Works

Traditional approach:
```asm
; Load parameter from stack
LD L, (IX+4)
LD H, (IX+5)
; Use the value
LD A, L
```

SMC approach:
```asm
smc_param:
LD HL, #1234    ; The immediate value #1234 can be modified
; Use the value directly
```

The immediate value in the instruction becomes the storage location itself, accessible via:
```asm
parameter_addr EQU smc_param+1
```

## Benefits

1. **Eliminated Memory Access**: No need to load from stack or memory
2. **Reduced Register Pressure**: Values are immediate, freeing registers
3. **Faster Execution**: Fewer instructions in hot paths
4. **Smaller Code Size**: In some cases, reduces overall code size

## Use Cases

### 1. Function Parameters (Non-Recursive)

```minz
@smc_optimize
fn draw_line(y: u8, color: u8) -> void {
    // Parameter 'y' is embedded in the code
    for x in 0..32 {
        set_pixel(x, y, color);  // No stack access for 'y'
    }
}
```

### 2. Frequently Read, Rarely Modified Values

```minz
@smc_optimize
fn game_loop() -> void {
    let player_lives: u8 = 3;    // Embedded in code
    let score: u16 = 0;          // Embedded in code
    
    loop {
        display_score(score);     // Direct immediate load
        display_lives(player_lives);
        
        if hit_enemy() {
            player_lives = player_lives - 1;  // SMC modifies the code
        }
    }
}
```

### 3. Animation and Counters

```minz
@smc_optimize
fn animate() -> void {
    let frame: u8 = 0;  // Embedded as immediate value
    
    draw_sprite(x, y, sprite_base + frame * 8);
    frame = (frame + 1) & 7;  // Modifies the immediate value
}
```

## Enabling SMC Optimization

### Compiler Flag

```bash
minzc program.minz -O --enable-smc
```

### Function Attribute

```minz
@smc_optimize
fn my_function() -> void {
    // This function will use SMC optimization
}
```

### Automatic Detection

The compiler automatically applies SMC optimization to:
- Non-recursive functions
- Constants that are modified infrequently
- Function parameters that aren't modified
- Local variables with limited scope and few modifications

## Limitations

### 1. Code Must Be in RAM
SMC requires code to be in writable memory:
```minz
#[section(".ram_code")]
@smc_optimize
fn fast_routine() -> void {
    // This function must be copied to RAM
}
```

### 2. No Recursion
Recursive functions cannot use SMC for parameters:
```minz
fn factorial(n: u16) -> u16 {
    if n <= 1 { return 1; }
    return n * factorial(n - 1);  // Parameters must be on stack
}
```

### 3. Debugging Complexity
- Breakpoints may behave unexpectedly
- Values change in the code itself
- Memory dumps don't show current values

## Implementation Details

### IR Extensions

New IR opcodes:
- `OpSMCLoadConst`: Load with self-modifying capability
- `OpSMCStoreConst`: Modify a previous SMC constant

### Code Generation

```asm
; Generated code for: let counter: u8 = 0
smc_1:
    LD A, 0        ; SMC constant
    
; Generated code for: counter = counter + 1
    INC A
    LD (smc_1+1), A  ; Modify the immediate value
```

### Safety Measures

1. **Recursion Detection**: Automatic analysis prevents SMC in recursive functions
2. **Memory Safety**: Compiler ensures SMC targets are valid
3. **Relocation Support**: SMC addresses are tracked for relocation

## Performance Analysis

### Memory Access Comparison

Traditional (stack-based):
```asm
; Load parameter: 19 cycles
LD L, (IX+4)    ; 19 cycles
```

SMC optimized:
```asm
; Load parameter: 10 cycles
LD HL, #1234    ; 10 cycles
```

**Savings: 9 cycles per access**

### Real-World Example: Line Drawing

Drawing a 32-pixel horizontal line:

Traditional: ~640 cycles
SMC optimized: ~352 cycles
**Performance gain: 45%**

## Best Practices

### 1. Profile First
Use SMC for hot paths identified by profiling:
```minz
@smc_optimize
fn inner_loop() -> void {
    // Optimize the most frequently executed code
}
```

### 2. Group SMC Values
Minimize code cache impact:
```minz
@smc_optimize
fn render() -> void {
    // Group related SMC values together
    let x_offset: u8 = 0;
    let y_offset: u8 = 0;
    let frame: u8 = 0;
    
    // Use them in rendering
}
```

### 3. Document SMC Usage
Make it clear when using SMC:
```minz
// WARNING: This function uses self-modifying code
// Must be loaded in RAM at address 0xC000
@smc_optimize
#[ram_address(0xC000)]
fn critical_routine() -> void {
    // Performance-critical code here
}
```

## Advanced Techniques

### 1. SMC with Interrupt Handlers
```minz
@interrupt
@smc_optimize
fn vblank_handler() -> void {
    let frame_counter: u8 = 0;  // Modified each frame
    frame_counter = frame_counter + 1;
    
    // Fast sprite multiplexing with embedded values
}
```

### 2. Dynamic Code Generation
```minz
fn generate_unrolled_copy(count: u8) -> void {
    let mut addr = code_buffer;
    
    for i in 0..count {
        // Generate: LD A, (HL) / LD (DE), A / INC HL / INC DE
        emit_instruction(addr, 0x7E);  // LD A, (HL)
        emit_instruction(addr, 0x12);  // LD (DE), A
        // ...
    }
}
```

### 3. Conditional SMC
```minz
@lua_if(PLATFORM == "ZX_SPECTRUM_128")
@smc_optimize  // Only on 128K with RAM banking
fn sound_routine() -> void {
    // SMC optimization for 128K only
}
@lua_endif
```

## Troubleshooting

### Issue: Code Crashes When Moved
**Solution**: Ensure SMC code is relocated properly or use position-independent addressing

### Issue: Debugger Shows Wrong Values
**Solution**: Use the generated symbol file that maps SMC locations

### Issue: Performance Degradation
**Solution**: Too many SMC modifications can cause cache issues; profile and limit usage

## Future Enhancements

1. **Automatic RAM Placement**: Compiler automatically manages RAM code sections
2. **SMC Profiling**: Track SMC modification frequency
3. **Hardware Detection**: Runtime detection of ROM vs RAM execution
4. **SMC Optimization Hints**: Compiler suggestions for SMC candidates