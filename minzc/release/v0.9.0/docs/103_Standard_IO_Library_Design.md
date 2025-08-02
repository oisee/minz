# MinZ Standard I/O Library Design

## Overview

MinZ provides a platform-agnostic I/O system that enables portable code across all Z80-based platforms while maintaining zero-cost abstractions and optimal performance.

## Architecture

### Three-Layer Design

```
┌─────────────────────────────────────────┐
│         Application Code                 │
│    (uses std.io portable interface)      │
├─────────────────────────────────────────┤
│         Standard I/O Library             │
│    (std.io - platform-agnostic API)      │
├─────────────────────────────────────────┤
│      Platform Implementation Layer       │
│ (zx.io, cpm.io, msx.io - hardware API)  │
└─────────────────────────────────────────┘
```

### Core Design Principles

1. **Zero-Cost Abstractions**: Interface dispatch resolves at compile time
2. **Platform Optimization**: Each platform uses native ROM routines
3. **Consistent API**: Same code works across all platforms
4. **Extensibility**: Easy to add new platforms

## Standard I/O Interface

### Reader Trait
```minz
interface Reader {
    fun read_byte(self) -> u8;
    fun read_bytes(self, buffer: *u8, len: u16) -> u16;
    fun available(self) -> bool;
}
```

### Writer Trait
```minz
interface Writer {
    fun write_byte(self, byte: u8) -> void;
    fun write_bytes(self, buffer: *u8, len: u16) -> u16;
    fun flush(self) -> void;
}
```

### Global Streams
- `stdin: Reader` - Standard input (keyboard)
- `stdout: Writer` - Standard output (screen)
- `stderr: Writer` - Standard error (varies by platform)

## Platform Implementations

### ZX Spectrum
- **ROM Integration**: Uses RST 0x10 for character output
- **Keyboard**: ROM routines for key scanning
- **Colors**: Direct attribute memory manipulation
- **Sound**: BEEP routine from ROM
- **Special Features**: Border color, flash, bright

### CP/M
- **BDOS Calls**: All I/O through BDOS function calls
- **File I/O**: Full FCB-based file system support
- **Console**: Automatic CR/LF translation
- **Compatibility**: Works with all CP/M 2.x systems
- **Special Features**: Command line arguments, file operations

### MSX
- **BIOS Calls**: Uses standard MSX BIOS routines
- **Graphics**: VDP access for sprites and VRAM
- **Sound**: PSG integration for music/effects
- **Input**: Joystick and keyboard support
- **Special Features**: Screen modes, sprite handling

## Core Functions

### Output Functions
```minz
pub fun print(message: *u8) -> void;
pub fun println(message: *u8) -> void;
pub fun print_char(ch: u8) -> void;
pub fun print_u8(value: u8) -> void;
pub fun print_u16(value: u16) -> void;
pub fun print_i8(value: i8) -> void;
pub fun print_i16(value: i16) -> void;
pub fun print_bool(value: bool) -> void;
pub fun print_hex_u8(value: u8) -> void;
pub fun print_hex_u16(value: u16) -> void;
```

### Input Functions
```minz
pub fun read_char() -> u8;
pub fun read_line(buffer: *u8, max_len: u16) -> u16;
```

### Utility Functions
```minz
pub fun printf(format: *u8, arg1: u16) -> void;
pub fun panic(message: *u8) -> void;
pub fun assert(condition: bool, message: *u8) -> void;
```

## Performance Characteristics

### ZX Spectrum Performance
- **Character Output**: 1 RST instruction (11 T-states)
- **Keyboard Input**: ~50 T-states for scan
- **No Buffering**: Direct hardware access

### CP/M Performance
- **Character Output**: BDOS call overhead (~200 T-states)
- **Buffered Input**: Efficient for line input
- **File I/O**: 128-byte record-based

### MSX Performance
- **BIOS Calls**: ~100 T-states overhead
- **VDP Access**: Wait states for video timing
- **PSG Access**: Direct port I/O

## Usage Examples

### Platform-Agnostic Code
```minz
import std.io;

fun main() -> u8 {
    println("Hello from MinZ!");
    print("Enter your name: ");
    
    let mut name: [u8; 32];
    let len = read_line(&name, 32);
    
    print("Hello, ");
    print(&name);
    println("!");
    
    0
}
```

### Platform-Specific Features
```minz
// ZX Spectrum specific
import zx.io;

fun colorful_hello() -> void {
    cls();
    set_ink(COLOR_YELLOW);
    set_paper(COLOR_BLUE);
    at(10, 10);
    println("Colorful Hello!");
    beep(1000, 500);
}

// CP/M specific
import cpm.io;

fun file_demo() -> void {
    let file = File::create("test.txt");
    file.write("Hello, CP/M!", 12);
    file.close();
}

// MSX specific
import msx.io;

fun game_input() -> void {
    if get_joystick(0) == JoystickDirection::Up {
        println("Joystick up!");
    }
    if get_trigger(0) {
        play_tone(0, 440, 15);  // A4 note
    }
}
```

## Implementation Details

### Compile-Time Interface Resolution
```minz
// This code:
stdout.write_byte('A');

// Compiles to direct call on ZX:
RST 0x10

// Compiles to BDOS call on CP/M:
LD C, 2
LD E, 'A'
CALL 0x0005

// Zero overhead - no vtable lookup!
```

### Memory Efficiency
- **No Heap Allocation**: All I/O uses stack buffers
- **Minimal State**: Platform implementations are zero-size structs
- **Direct Hardware Access**: No intermediate buffering

### Error Handling
- **Panic**: Halts system with message
- **Assert**: Debug-time checks (can be optimized out)
- **Return Values**: Functions return actual bytes read/written

## Extending the System

### Adding a New Platform

1. Create platform module (e.g., `amstrad/io.minz`)
2. Implement Reader trait for input
3. Implement Writer trait for output
4. Define platform-specific features
5. Export global instances

Example skeleton:
```minz
// amstrad/io.minz
import std.io;

struct AmstradStdin {}
impl Reader for AmstradStdin { ... }

struct AmstradStdout {}
impl Writer for AmstradStdout { ... }

pub let stdin = AmstradStdin {};
pub let stdout = AmstradStdout {};
pub let stderr = AmstradStdout {};
```

### Custom I/O Devices

```minz
// Serial port implementation
struct SerialPort {
    port: u8,
    baud: u16,
}

impl Writer for SerialPort {
    fun write_byte(self, byte: u8) -> void {
        // Wait for transmit ready
        while !self.tx_ready() {}
        // Send byte
        @asm {
            LD A, (byte)
            OUT (self.port), A
        }
    }
}
```

## Best Practices

### 1. Use Platform-Agnostic Code
```minz
// Good - works everywhere
import std.io;
println("Hello!");

// Avoid - platform specific
import zx.io;
set_ink(2);  // Only works on ZX
```

### 2. Check Platform Features
```minz
// Use conditional compilation
@if(platform == "zx") {
    import zx.io;
    set_border(COLOR_RED);
}
```

### 3. Buffer Appropriately
```minz
// Good - reasonable buffer
let mut line: [u8; 80];
read_line(&line, 80);

// Avoid - huge stack allocation
let mut buffer: [u8; 32768];
```

### 4. Handle Line Endings
```minz
// The library normalizes \r\n to \n
// But be aware when writing files
```

## Future Enhancements

### Planned Features
1. **Formatted Output**: Full printf implementation
2. **Color Abstraction**: Portable color API
3. **Sound Abstraction**: Cross-platform audio
4. **Async I/O**: Non-blocking operations
5. **Stream Redirection**: Pipe support

### Research Areas
1. **Zero-Copy I/O**: Direct DMA where available
2. **Compression**: On-the-fly compression
3. **Network I/O**: TCP/IP for enhanced systems
4. **Graphics Abstraction**: Portable graphics API

## Conclusion

MinZ's standard I/O library achieves the impossible: a portable, high-level I/O abstraction that compiles to optimal platform-specific code with zero overhead. This design enables developers to write once and run efficiently everywhere, from the ZX Spectrum to CP/M systems to MSX computers.

The key innovation is compile-time interface resolution, ensuring that high-level code becomes direct hardware access without any runtime indirection. This maintains MinZ's promise of zero-cost abstractions while providing the convenience of modern programming.