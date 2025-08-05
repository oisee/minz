# MinZ Type System

## Core Philosophy
MinZ provides a rich type system optimized for 8-bit and 24-bit processors (Z80/eZ80), with zero-cost abstractions and compile-time type safety.

## Numeric Types

### Integer Types
- `u8` - Unsigned 8-bit (0 to 255)
- `i8` - Signed 8-bit (-128 to 127)
- `u16` - Unsigned 16-bit (0 to 65,535)
- `i16` - Signed 16-bit (-32,768 to 32,767)
- `u24` - Unsigned 24-bit (0 to 16,777,215) - Perfect for eZ80 addressing
- `i24` - Signed 24-bit (-8,388,608 to 8,388,607)
- `bool` - Boolean (true/false, stored as u8)

### Fixed-Point Types
MinZ provides efficient fixed-point arithmetic for systems without floating-point hardware:

#### Standard Fixed-Point
- `f8.8` - 8-bit integer, 8-bit fraction
  - Range: -128.0 to 127.996
  - Precision: 1/256 (≈0.0039)
  - Use cases: Positions, velocities in games

- `f16.8` - 16-bit integer, 8-bit fraction
  - Range: -32,768.0 to 32,767.996
  - Precision: 1/256
  - Use cases: World coordinates, large-scale physics

- `f8.16` - 8-bit integer, 16-bit fraction
  - Range: -128.0 to 127.999985
  - Precision: 1/65,536 (≈0.000015)
  - Use cases: High-precision calculations, audio processing

#### Pure Fractional
- `f.8` - 8-bit pure fraction (0.0 to 0.996)
  - Range: 0.0 to 0.996
  - Precision: 1/256
  - Use cases: Percentages, alpha blending, probability

- `f.16` - 16-bit pure fraction (0.0 to 0.9999847)
  - Range: 0.0 to 0.9999847
  - Precision: 1/65,536 (≈0.0000153)
  - Use cases: High-precision interpolation, audio mixing, fine gradients

### Fixed-Point Operations
```minz
// Automatic fixed-point arithmetic
let velocity: f8.8 = 1.5;
let acceleration: f8.8 = 0.25;
let new_velocity = velocity + acceleration;  // Result: 1.75

// Mixing precision levels
let pos: f16.8 = 100.5;
let delta: f8.8 = 1.25;
let new_pos = pos + delta;  // Automatic promotion to f16.8

// Pure fractional for blending
let alpha: f.8 = 0.75;  // 75% opacity
let color1 = 255;
let color2 = 128;
let blended = color1 * (1.0 - alpha) + color2 * alpha;
```

## String Types

### Design Philosophy
MinZ provides two distinct string types to eliminate ambiguity and optimize for different use cases:

### String (Short String)
- Structure: `{ len: u8, data: [u8] }`
- Maximum length: 255 characters
- Memory overhead: 1 byte
- Use cases: UI text, short messages, filenames

```minz
struct String {
    len: u8,
    data: [u8; 255],  // Actual size is dynamic
}
```

### LString (Long String)
- Structure: `{ len: u16, data: [u8] }`
- Maximum length: 65,535 characters
- Memory overhead: 2 bytes
- Use cases: Text buffers, file contents, network data

```minz
struct LString {
    len: u16,
    data: [u8; 65535],  // Actual size is dynamic
}
```

### String Literals
```minz
let short_msg: String = "Hello, World!";  // Automatically String (< 256 chars)
let long_text: LString = l"Very long text...";  // Explicit LString with 'l' prefix
```

### Zero-Cost String Conversion
Both string types implement the `Printable` interface for seamless interoperability:

```minz
interface Printable {
    fun print(self) -> void;
    fun len(self) -> u16;
}

impl Printable for String {
    fun print(self) -> void {
        print_string(&self.data[0]);
    }
    
    fun len(self) -> u16 {
        return self.len as u16;
    }
}

impl Printable for LString {
    fun print(self) -> void {
        print_string(&self.data[0]);
    }
    
    fun len(self) -> u16 {
        return self.len;
    }
}

// Usage - zero-cost abstraction
fun display(text: &Printable) -> void {
    text.print();  // Works with both String and LString
}
```

## Composite Types

### Arrays
Fixed-size arrays with compile-time bounds checking:
```minz
let buffer: [u8; 256];
let matrix: [f8.8; 16];  // 4x4 matrix of fixed-point values
```

### Structs
```minz
struct Sprite {
    x: f16.8,        // World position with sub-pixel precision
    y: f16.8,
    vel_x: f8.8,     // Velocity
    vel_y: f8.8,
    frame: u8,       // Animation frame
    alpha: f.8,      // Transparency (0.0 to 1.0)
}
```

### Bit-Packed Structures
Memory-efficient bit packing:
```minz
type Flags = bits_8 {
    visible: 1,      // 1 bit
    collision: 1,    // 1 bit
    priority: 3,     // 3 bits (0-7)
    state: 3,        // 3 bits (0-7)
};
```

## Type Conversion

### Explicit Casting
```minz
let a: u8 = 255;
let b: u16 = a as u16;  // Zero-extension
let c: i8 = a as i8;    // Reinterpretation (-1)

// Fixed-point conversions
let int_val: u8 = 10;
let fixed_val: f8.8 = int_val as f8.8;  // 10.0
let back: u8 = fixed_val as u8;         // Truncates fraction
```

### Automatic Promotion
MinZ automatically promotes types in mixed arithmetic to prevent precision loss:
```minz
let a: u8 = 100;
let b: u16 = 1000;
let c = a + b;  // Result is u16

let x: f8.8 = 1.5;
let y: f16.8 = 100.25;
let z = x + y;  // Result is f16.8
```

## Memory Layout

All types are designed for efficient Z80/eZ80 memory access:
- 8-bit types: Direct register operations
- 16-bit types: Register pair operations (HL, DE, BC)
- 24-bit types: eZ80 native or Z80 with extra byte
- Fixed-point: Stored as integers, interpreted as fixed-point

## Performance Considerations

### Zero-Cost Abstractions
- Interfaces compile to direct function calls
- String type selection is compile-time
- Fixed-point operations compile to integer arithmetic

### Optimization Tips
1. Use `u8` when possible - fits in single register
2. Use `String` for UI text - less memory overhead
3. Use `f8.8` for most game physics - good balance
4. Use `f.8` for percentages - efficient and clear
5. Pack flags into bit structures - 8x memory savings

## Future Extensions

### Planned Types
- `f4.4` - Nibble-precision for very compact storage
- `String8` - 8-character fixed string for identifiers
- `Vector2`, `Vector3` - SIMD-style vector types
- `Color` - Optimized color representation

### Under Consideration
- Arbitrary precision integers
- Decimal fixed-point (BCD)
- Quaternions for 3D rotation