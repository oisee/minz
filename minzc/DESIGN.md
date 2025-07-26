# MinZ Design Document

## Overview

MinZ is a systems programming language specifically designed for Z80-based computers, with a focus on the ZX Spectrum. It combines modern language features with aggressive optimization techniques, particularly self-modifying code (SMC).

## Core Philosophy

1. **Zero-Cost Abstractions**: High-level features compile to optimal assembly
2. **SMC-First**: Self-modifying code is a first-class optimization
3. **Hardware-Aware**: Language features designed for 8-bit constraints
4. **Modern Syntax**: Familiar to Rust/Zig programmers
5. **Predictable Performance**: No hidden costs or surprises

## Language Features

### Type System

**Basic Types**:
- `u8`, `u16` - Unsigned integers
- `i8`, `i16` - Signed integers  
- `bool` - Boolean
- `void` - No value

**Composite Types**:
- Arrays: `[T; N]` - Fixed size
- Pointers: `*T`, `*mut T`
- Structs: Named fields
- Enums: Tagged unions
- Bit structs: Hardware register abstraction

**Special Types**:
- Function pointers: `fn(T) -> U`
- Lambdas: `|T| -> U` (via TRUE SMC)

### Memory Model

```
$0000-$3FFF: ROM (usually)
$4000-$5AFF: Screen memory
$5B00-$FFFF: RAM (program + data)

Stack: Grows down from top
Heap: NONE - static allocation only
```

### TRUE SMC (истинный SMC)

Our revolutionary approach to parameter passing:

```minz
fn draw_pixel(x: u8, y: u8) -> void {
    // Parameters are patched directly into code
}

// Compiles to:
draw_pixel:
    LD A, 0     ; x anchor - patched before call
    LD B, 0     ; y anchor - patched before call
    ; ... rest of function
```

Benefits:
- 3-4x faster than stack passing
- Enables zero-cost closures
- Reduces register pressure
- Smaller code size

### Bit Structs

Zero-cost bit field manipulation:

```minz
type ScreenAttr = bits {
    ink: 3,      // Bits 0-2
    paper: 3,    // Bits 3-5  
    bright: 1,   // Bit 6
    flash: 1     // Bit 7
};
```

Compiles to optimal AND/OR/shift sequences.

### Module System

Simple and effective:

```minz
// In graphics.minz
export fn draw_line(x1: u8, y1: u8, x2: u8, y2: u8);

// In main.minz
import graphics;
graphics.draw_line(0, 0, 255, 191);
```

Internal representation uses prefixing: `graphics_draw_line`

### Control Flow

- `if`/`else` - Standard conditionals
- `while` - Pre-tested loops
- `loop` - Infinite loops with `break`
- `for` - Iterator-based loops (planned)
- `match` - Pattern matching (planned)

### Inline Assembly

Direct hardware control when needed:

```minz
asm {
    DI              ; Disable interrupts
    LD HL, $4000    ; Screen start
    LD (HL), 0      ; Clear first byte
    EI              ; Enable interrupts
}
```

## Compilation Pipeline

1. **Parse**: Source → AST (via simple parser)
2. **Semantic Analysis**: Type checking, symbol resolution
3. **IR Generation**: AST → Three-address code
4. **Optimization**: Multiple passes including SMC
5. **Code Generation**: IR → Z80 assembly
6. **Assembly**: Via sjasmplus → Z80 binary

## Optimization Strategy

### Register Allocation
- Intelligent use of 8-bit registers (A, B, C, D, E, H, L)
- Shadow register optimization for interrupts
- Register pair operations for 16-bit values

### SMC Optimizations
1. **Parameter Patching**: TRUE SMC for function calls
2. **Constant Propagation**: Patch constants at call sites
3. **Lambda Captures**: Direct value patching
4. **Loop Unrolling**: With patched iteration counts

### Memory Optimizations
- Static allocation only
- Buffer pooling for dynamic-like behavior
- Compile-time memory layout
- Page-aligned data for banking

## Standard Library

### Core Module
- Basic types and operations
- Memory operations
- Math utilities

### Platform Modules
- `zx.screen` - Screen memory access
- `zx.input` - Keyboard/joystick
- `zx.sound` - BEEP/AY chip
- `zx.tape` - Tape loading/saving

### Planned Modules
- `math` - Fixed-point, trig tables
- `game` - Sprites, collision
- `compression` - RLE, simple LZ

## Error Handling

Two-tier approach:
1. **System Calls**: Carry flag convention
2. **User Code**: `Result<T, E>` types

```minz
fn read_byte(addr: u16) -> Result<u8, Error> {
    if addr >= 0x4000 {
        Ok(peek(addr))
    } else {
        Err(Error::RomAddress)
    }
}
```

## Future Directions

### Near Term
1. Fix parser reliability
2. Complete struct support
3. Implement for loops
4. Lambda expressions via TRUE SMC

### Medium Term  
1. Pattern matching
2. Const functions
3. Better error messages
4. LSP implementation

### Long Term
1. Other Z80 platforms (Game Boy, MSX)
2. Profile-guided optimization
3. Formal verification subset
4. Educational materials

## Design Principles

Every feature must:
1. Have zero or negative runtime cost
2. Map cleanly to Z80 assembly
3. Solve real problems for retro developers
4. Be understandable without surprises
5. Maintain the "feel" of programming close to metal

## Non-Goals

MinZ explicitly does NOT aim to:
- Support dynamic memory allocation
- Provide memory safety guarantees
- Abstract away platform details
- Support every modern language feature
- Target non-Z80 processors

## Conclusion

MinZ represents a new approach to retro development - modern language design meeting extreme optimization. By embracing constraints and focusing on what makes Z80 special, we can write better code than ever before possible.