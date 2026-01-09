# ABI and Assembly Integration in MinZ

**The Power of @abi Annotations for Seamless Assembly Integration**

## The Brilliant Insight

Using `@abi("register")` and other ABI annotations creates a **perfect bridge** between MinZ high-level code and hand-written assembly!

## Assembly Integration Patterns

### 1. Direct Assembly Function Replacement

```minz
// MinZ declaration with explicit register ABI
@abi("register: A=x, E=y")
@extern
fun fast_multiply(x: u8, y: u8) -> u16;

// Can be implemented in pure assembly:
; fast_multiply.asm
fast_multiply:
    ; A = x, E = y (guaranteed by ABI)
    LD D, 0
    LD H, D
    LD L, D         ; HL = 0
    LD B, 8         ; 8 bits
mul_loop:
    ADD HL, HL      ; HL *= 2
    RLCA            ; Check MSB of A
    JR NC, no_add
    ADD HL, DE      ; HL += DE
no_add:
    DJNZ mul_loop
    RET             ; Result in HL
```

### 2. Inline Assembly with Register Guarantees

```minz
@abi("register: HL=ptr, BC=count")
fun fast_fill(ptr: *u8, count: u16, value: u8) -> void {
    // We KNOW ptr is in HL, count is in BC!
    asm {
        ; A already contains 'value' from compiler
        LD E, A         ; Save fill value
fill_loop:
        LD (HL), E      ; Fill byte
        INC HL          ; Next address
        DEC BC          ; Decrement count
        LD A, B         ; Check if BC = 0
        OR C
        JR NZ, fill_loop
    }
}
```

### 3. System Call Wrappers

```minz
// ZX Spectrum ROM calls expect specific registers
@abi("register: A=color")
fun set_border(color: u8) -> void {
    asm {
        OUT ($FE), A    ; A guaranteed to have color
    }
}

@abi("register: HL=addr, DE=dest, BC=length")
fun rom_copy(addr: u16, dest: u16, length: u16) -> void {
    asm {
        CALL $1F00      ; ROM LDIR wrapper
    }
}
```

### 4. Optimized Math Library

```minz
// Entire math library with optimal register usage
module fastmath;

@abi("register: HL=a, DE=b")
pub fun add32(a: u32, b: u32) -> u32 {
    // HL:HL' = a, DE:DE' = b
    asm {
        ADD HL, DE      ; Low 16 bits
        EXX
        ADC HL, DE      ; High 16 bits with carry
        EXX
        ; Result in HL:HL'
    }
}

@abi("register: A=angle")
pub fun sin(angle: u8) -> i8 {
    asm {
        LD HL, sine_table
        LD L, A         ; HL = sine_table + angle
        LD A, (HL)      ; Table lookup
    }
}
```

### 5. Hardware Driver Integration

```minz
// AY-3-8912 sound chip driver
@abi("register: A=reg, C=value")
fun ay_write(reg: u8, value: u8) -> void {
    asm {
        LD BC, $FFFD    ; AY register port
        OUT (C), A      ; Select register
        LD BC, $BFFD    ; AY data port
        OUT (C), C      ; Write value
    }
}

// DMA controller
@abi("register: HL=src, DE=dst, BC=count, A=mode")
fun dma_transfer(src: u16, dst: u16, count: u16, mode: u8) -> void {
    asm {
        ; Registers already set up perfectly!
        OUT ($6B), A    ; DMA mode
        ; ... DMA setup sequence
    }
}
```

## Advanced Integration Patterns

### 1. Conditional ABI Selection

```minz
// Use different ABIs based on build configuration
#if ROM_BUILD
    @abi("stack")  // ROM can't use SMC
#else
    @abi("smc")    // RAM build uses fast SMC
#endif
fun configure_system(params: SystemConfig) -> void {
    // Implementation
}
```

### 2. ABI Constraints for Assembly

```minz
// Tell compiler which registers assembly code uses
@abi("register: HL=data, DE=size")
@clobbers("AF", "BC")  // Assembly will destroy these
fun crc16(data: *u8, size: u16) -> u16 {
    asm {
        ; Complex CRC calculation
        ; Can freely use AF, BC
        ; Must preserve IX, IY
    }
}
```

### 3. Naked Functions

```minz
// No prologue/epilogue - pure assembly
@abi("naked")
@abi("register: HL=vector")
fun set_interrupt_vector(vector: u16) -> void {
    asm {
        DI
        LD ($5C00), HL  ; Set IM2 vector
        IM 2
        EI
        RET
    }
}
```

### 4. Multiple Entry Points

```minz
// Different ABIs for different entry points
@abi("register: A=byte")
fun putchar(c: u8) -> void;

@abi("register: HL=string")
@entry("print_string")  // Alternate entry
fun print(s: *u8) -> void {
    loop {
        let c = *s;
        if c == 0 { break; }
        putchar(c);
        s = s + 1;
    }
}
```

## Compiler Benefits

### 1. Register Allocation Hints

```minz
@abi("register: HL=important, DE=temp")
fun process(important: *Data, temp: u16) -> void {
    // Compiler knows 'important' should stay in HL
    // Can use DE as scratch register
}
```

### 2. Calling Convention Documentation

```minz
// ABI annotations serve as documentation
@abi("register: HL=buffer, A=flags, BC=size")
@returns("HL=newbuffer, A=status")
fun compress(buffer: *u8, flags: u8, size: u16) -> (*u8, u8);
```

### 3. Binary Compatibility

```minz
// Match external libraries exactly
@abi("cdecl")  // C calling convention
@extern
fun malloc(size: u32) -> *u8;

@abi("pascal")  // Pascal calling convention  
@extern
fun TurboPascalFunc(a: u16, b: u16) -> u16;

@abi("z80_romcall")  // Custom ROM convention
@extern
fun ROM_PrintChar(c: u8) -> void;
```

## Implementation Strategy

### Phase 1: Basic Register ABIs
```rust
// Compiler implementation
fn parse_abi_string(abi: &str) -> AbiSpec {
    // Parse "register: A=x, HL=ptr, BC=count"
    match abi {
        "register" => parse_register_spec(abi),
        "stack" => StackAbi::default(),
        "smc" => SmcAbi::default(),
        _ => Custom(abi),
    }
}
```

### Phase 2: Assembly Verification
```rust
// Verify assembly matches ABI contract
fn verify_assembly_abi(asm: &AsmBlock, abi: &AbiSpec) {
    // Check register usage matches declaration
    // Warn about ABI violations
    // Optimize register allocation around assembly
}
```

### Phase 3: Cross-Language Support
```minz
// Import C functions with correct ABI
@abi("c")
import "libc" {
    fun printf(fmt: *u8, ...) -> i32;
    fun strcmp(s1: *u8, s2: *u8) -> i32;
}

// Export MinZ functions for C
@abi("c")
@export
fun minz_function(x: i32) -> i32 {
    return x * 2;
}
```

## Real-World Example: Graphics Library

```minz
// High-performance graphics with assembly integration
module gfx;

@abi("register: HL=vram, D=x, E=y, C=color")
pub fun plot_pixel(vram: *u8, x: u8, y: u8, color: u8) -> void {
    asm {
        ; Ultra-optimized pixel plotting
        ; Inputs already in perfect registers!
        PUSH HL
        
        ; Calculate address: vram + (y<<8) + (x>>3)
        LD L, E         ; L = y
        LD H, 0
        ADD HL, HL      ; HL = y*2
        ADD HL, HL      ; HL = y*4
        ADD HL, HL      ; HL = y*8
        ADD HL, HL      ; HL = y*16
        ADD HL, HL      ; HL = y*32
        
        LD A, D         ; x
        RRCA
        RRCA
        RRCA
        AND $1F
        LD E, A
        LD D, 0
        ADD HL, DE      ; HL = offset
        
        POP DE          ; DE = vram base
        ADD HL, DE      ; HL = final address
        
        ; Set pixel
        LD A, D         ; x
        AND 7           ; bit position
        LD B, A
        LD A, $80
        JR Z, no_shift
shift_loop:
        RRCA
        DJNZ shift_loop
no_shift:
        LD B, A         ; B = pixel mask
        LD A, (HL)      ; Current byte
        
        ; Apply color
        BIT 0, C
        JR Z, clear_pixel
        OR B            ; Set pixel
        JR store
clear_pixel:
        CPL
        AND B
        CPL             ; Clear pixel
store:
        LD (HL), A
    }
}

// Can now be called from MinZ with zero overhead!
fun draw_line(x1: u8, y1: u8, x2: u8, y2: u8, color: u8) -> void {
    let vram = 0x4000 as *u8;
    // Bresenham's algorithm
    // Each plot_pixel call has optimal register usage!
    plot_pixel(vram, x1, y1, color);
    // ...
}
```

## Conclusion

The `@abi` annotation system transforms MinZ from just another high-level language into a **precision tool** for Z80 development:

1. **Zero-overhead assembly integration** - registers exactly where assembly expects them
2. **Self-documenting interfaces** - ABI is part of the function signature
3. **Optimal performance** - no register shuffling needed
4. **Seamless interop** - call assembly from MinZ, MinZ from assembly
5. **Hardware-friendly** - perfect for drivers and system programming

This is the "missing link" that makes MinZ truly powerful for systems programming! 🚀