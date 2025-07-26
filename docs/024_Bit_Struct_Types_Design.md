# Bit-Struct Types Design Document

**Date**: 2025-07-26  
**Document**: 024_Bit_Struct_Types_Design.md

## 1. Executive Summary

This document proposes the implementation of bit-struct types for MinZ, providing native support for bit-level manipulation crucial for Z80 hardware programming. The design focuses on compile-time resolution with optimized implementations for 8-bit and 16-bit underlying types.

## 2. Motivation

### 2.1 Hardware Register Manipulation
Z80-based systems frequently use packed bit fields for hardware control:
- **ZX Spectrum attributes**: 8 bits encoding ink, paper, bright, and flash
- **Sound chip registers**: Bit flags for channel control
- **Sprite attributes**: Packed position, palette, and control bits

### 2.2 Current Limitations
Without bit-struct types, developers must:
- Use error-prone manual bit shifting and masking
- Maintain magic constants for bit positions
- Write repetitive bit manipulation code

### 2.3 Benefits
Bit-struct types provide:
- **Type safety** for bit field access
- **Readable code** with named fields
- **Optimal code generation** through compile-time resolution
- **Hardware documentation** in code structure

## 3. Language Design

### 3.1 Basic Syntax
```minz
// 8-bit bit struct
type Attributes = bits {
    ink: 3,      // Bits 0-2
    paper: 3,    // Bits 3-5
    bright: 1,   // Bit 6
    flash: 1     // Bit 7
}

// 16-bit bit struct  
type SpritePos = bits<u16> {
    x_coord: 9,    // Bits 0-8 (0-511)
    y_fine: 3,     // Bits 9-11 (fine position)
    visible: 1,    // Bit 12
    priority: 2,   // Bits 13-14
    updated: 1     // Bit 15
}
```

### 3.2 Usage Examples
```minz
// Initialization
let attr: Attributes = { ink: 7, paper: 0, bright: true, flash: false }

// Field access
attr.paper = 2          // Sets paper color to red
let is_bright = attr.bright  // Reads bright flag

// Type conversion
let raw: u8 = attr as u8     // Get underlying value
let attr2: Attributes = 0x47 as Attributes  // From raw value

// Compound operations
attr = { ...attr, flash: true }  // Update single field
```

## 4. Implementation Strategy

### 4.1 Compile-Time Resolution
All bit-struct operations resolve at compile time to optimal Z80 instructions:

```minz
// Source code:
attr.paper = 5

// Generates:
LD A, (attr)      ; Load current value
AND 0xC7          ; Clear bits 3-5 (paper field)
OR 0x28           ; Set paper to 5 (5 << 3)
LD (attr), A      ; Store result
```

### 4.2 Optimized 8-bit Implementation
For 8-bit underlying types, use accumulator-based operations:

```minz
// Reading a field
let ink = attr.ink
// Generates:
LD A, (attr)
AND 0x07          ; Mask for 3-bit field
; Result in A

// Writing a field
attr.bright = true
// Generates:
LD A, (attr)
OR 0x40           ; Set bit 6
LD (attr), A
```

### 4.3 Optimized 16-bit Implementation
For 16-bit types, use HL register pair:

```minz
// Reading a 9-bit field
let x = sprite.x_coord
// Generates:
LD HL, (sprite)
LD A, L
LD H, 0           ; Clear high byte
; Optional: AND H, 0x01 if field crosses byte boundary

// Writing a field spanning bytes
sprite.x_coord = 256
// Generates:
LD HL, (sprite)
LD A, L
AND 0x00          ; Clear low bits of x_coord
LD L, A
LD A, H
AND 0xFE          ; Clear high bit of x_coord
OR 0x01           ; Set bit 8 (256 = 0x100)
LD H, A
LD (sprite), HL
```

## 5. Type System Integration

### 5.1 Type Checking
```minz
type ColorMode = bits {
    mode: 2,      // 0-3
    enhanced: 1,
    reserved: 5
}

let mode: ColorMode = { mode: 4 }  // Compile error: 4 > 3 (2 bits max)
let attr: Attributes = mode        // Compile error: Type mismatch
```

### 5.2 Const Evaluation
```minz
const DEFAULT_ATTR: Attributes = { ink: 7, paper: 0, bright: false, flash: false }
// Resolves to: const DEFAULT_ATTR: u8 = 0x07

const BRIGHT_WHITE: Attributes = { ...DEFAULT_ATTR, bright: true, paper: 7 }
// Resolves to: const BRIGHT_WHITE: u8 = 0x7F
```

## 6. Code Generation Examples

### 6.1 Simple Field Access
```minz
// Source:
if attr.flash {
    attr.flash = false
}

// Generated assembly:
LD A, (attr)
AND 0x80          ; Test bit 7
JR Z, skip_clear
LD A, (attr)
AND 0x7F          ; Clear bit 7
LD (attr), A
skip_clear:
```

### 6.2 Multi-Field Update
```minz
// Source:
attr = { ink: 2, paper: 5, bright: true, flash: false }

// Generated assembly:
LD A, 0x6A        ; Compile-time computed: 0x02 | (0x05 << 3) | 0x40
LD (attr), A
```

### 6.3 Bit Field Arrays
```minz
// Source:
let attrs: [Attributes; 768]
attrs[i].bright = true

// Generated assembly:
LD HL, attrs
LD DE, i
ADD HL, DE        ; Calculate address
LD A, (HL)
OR 0x40           ; Set bright bit
LD (HL), A
```

## 7. Standard Library Support

### 7.1 ZX Spectrum Module
```minz
// stdlib/zx/video.minz
type ScreenAttr = bits {
    ink: 3,
    paper: 3,
    bright: 1,
    flash: 1
}

fn set_attr(x: u8, y: u8, attr: ScreenAttr) -> void {
    let addr = 0x5800 + (y as u16) * 32 + (x as u16)
    poke(addr, attr as u8)
}
```

### 7.2 Sound Chip Module
```minz
// stdlib/zx/sound.minz
type AY_Envelope = bits<u16> {
    fine_period: 8,     // Bits 0-7
    coarse_period: 8    // Bits 8-15
}

type AY_Mixer = bits {
    tone_a_off: 1,
    tone_b_off: 1,
    tone_c_off: 1,
    noise_a_off: 1,
    noise_b_off: 1,
    noise_c_off: 1,
    port_a_input: 1,
    port_b_input: 1
}
```

## 8. Implementation Phases

### Phase 1: Basic 8-bit Support ✅
1. **Parser changes**: Add `bits` type syntax
2. **AST nodes**: BitStructType, BitFieldDecl
3. **Semantic analysis**: Type checking, field validation
4. **Code generation**: Basic read/write operations
5. **Testing**: ZX Spectrum attribute manipulation

### Phase 2: 16-bit Support ✅
1. **Extend parser**: bits<u16> syntax
2. **Multi-byte field handling**: Fields spanning byte boundaries
3. **Optimized code gen**: Use HL for 16-bit operations
4. **Testing**: Sprite position, sound period values

### Phase 3: Advanced Features
1. **Enum constraints**: `color: 2 enum { BLACK, BLUE, RED, MAGENTA }`
2. **Nested bit structs**: Bit structs containing bit structs
3. **Union types**: Overlapping bit field definitions
4. **Debug support**: Symbol information for bit fields

## 9. Performance Characteristics

### 9.1 Code Size
- **Field read**: 2-4 bytes (AND + optional shift)
- **Field write**: 4-7 bytes (load, AND, OR, store)
- **Full struct init**: 2 bytes (single LD instruction)

### 9.2 Execution Time
- **8-bit field read**: 7-14 T-states
- **8-bit field write**: 20-27 T-states
- **16-bit field access**: 14-35 T-states

### 9.3 Comparison with Manual Code
Bit-struct types generate identical or better code than hand-written bit manipulation, with the added benefits of type safety and readability.

## 10. Future Extensions

### 10.1 Bit Arrays
```minz
type Bitmap = bits {
    pixels: [1; 8]  // Array of 8 single bits
}
```

### 10.2 Conditional Fields
```minz
type Register = bits {
    value: 4,
    extended: 4 when mode == EXTENDED else 0
}
```

### 10.3 Bit Pattern Matching
```minz
match attr {
    { bright: true, flash: true } => handle_alert(),
    { ink: 0, paper: 0 } => handle_hidden(),
    _ => handle_normal()
}
```

## 11. Conclusion

Bit-struct types provide a crucial abstraction for Z80 hardware programming, offering:
- **Zero-cost abstraction** through compile-time resolution
- **Type-safe** bit manipulation
- **Readable** hardware interface code
- **Optimal** code generation for 8-bit and 16-bit cases

The implementation focuses on practical needs of Z80 developers while maintaining MinZ's philosophy of transparent, efficient compilation.

---

*"Bit-struct types transform error-prone bit twiddling into clear, maintainable, and efficient code - perfect for the precise hardware control demanded by Z80 systems."*