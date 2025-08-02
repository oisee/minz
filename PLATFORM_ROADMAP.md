# MinZ Platform Support Roadmap

## Current Status: Z80 Foundation Complete

MinZ has achieved a solid foundation on Z80-based systems with:
- ✅ Complete Z80 instruction set support
- ✅ TRUE SMC optimization (world's first)
- ✅ Zero-cost abstractions (lambdas, interfaces)
- ✅ @abi integration for seamless assembly integration
- ✅ Comprehensive optimization pipeline
- ✅ 76% compilation success rate across 138 test examples

## Platform Expansion Strategy

### Phase 1: Z80 Ecosystem Completion (Next 6 Months)

#### CP/M Support 🎯 High Priority
**Target**: Complete CP/M 2.2 and CP/M 3.0 compatibility

**Standard Library Components:**
```minz
// stdlib/cpm/stdio.minz
fun printf(format: *u8, ...args) -> i16;
fun scanf(format: *u8, ...args) -> i16;
fun fopen(filename: *u8, mode: *u8) -> *FILE;
fun fclose(file: *FILE) -> i16;
fun fread(buffer: *u8, size: u16, count: u16, file: *FILE) -> u16;
fun fwrite(buffer: *u8, size: u16, count: u16, file: *FILE) -> u16;

// CP/M specific functions
fun bdos_call(function: u8, parameter: u16) -> u16;
fun get_user_area() -> u8;
fun set_user_area(area: u8) -> void;
fun get_disk() -> u8;
fun set_disk(drive: u8) -> void;
```

**Key Features:**
- Full BDOS (Basic Disk Operating System) integration
- File I/O with CP/M file system support
- Console I/O optimization for CP/M terminals
- Memory management within 64KB constraints
- Integration with CP/M command line utilities

**Implementation Approach:**
- @abi integration with CP/M BDOS calls
- Zero-overhead wrapper functions
- TRUE SMC optimization for system calls
- Optimized text processing for 80-column terminals

#### MSX Support 🎯 High Priority  
**Target**: MSX-BASIC integration and MSX-DOS compatibility

**Standard Library Components:**
```minz
// stdlib/msx/stdio.minz
fun print(text: *u8) -> void;
fun input(prompt: *u8) -> *u8;
fun locate(x: u8, y: u8) -> void;
fun screen(mode: u8) -> void;

// MSX-specific graphics and sound
fun pset(x: u16, y: u16, color: u8) -> void;
fun line(x1: u16, y1: u16, x2: u16, y2: u16, color: u8) -> void;
fun sound(channel: u8, frequency: u16, volume: u8) -> void;
fun play(music_string: *u8) -> void;

// MSX hardware access
fun vdp_write(register: u8, value: u8) -> void;
fun vdp_read(register: u8) -> u8;
fun psg_write(register: u8, value: u8) -> void;
```

**Key Features:**
- Direct VDP (Video Display Processor) integration
- PSG (Programmable Sound Generator) support
- Sprite handling and graphics primitives
- MSX-DOS file system integration
- ROM cartridge development support

#### ZX Spectrum Enhancement 🔧 Extension
**Target**: Expanded ZX Spectrum standard library

**Enhanced Components:**
```minz
// stdlib/zx/enhanced_io.minz
fun load_tape(filename: *u8) -> bool;
fun save_tape(filename: *u8, data: *u8, length: u16) -> bool;
fun microdrive_load(channel: u8, filename: *u8) -> bool;
fun microdrive_save(channel: u8, filename: *u8, data: *u8) -> bool;

// Enhanced graphics
fun draw_sprite(x: u8, y: u8, sprite_data: *u8, width: u8, height: u8) -> void;
fun scroll_screen(direction: u8, amount: u8) -> void;
fun fade_colors(steps: u8) -> void;

// Interface 2 support
fun joystick_read(port: u8) -> u8;
fun rs232_init(baud_rate: u16) -> void;
fun rs232_send(data: u8) -> void;
fun rs232_receive() -> u8;
```

#### Amstrad CPC Support 🎯 New Platform
**Target**: Complete Amstrad CPC family support

**Standard Library Components:**
```minz
// stdlib/cpc/stdio.minz
fun cpc_print(text: *u8) -> void;
fun cpc_input() -> *u8;
fun cpc_cls() -> void;
fun cpc_locate(x: u8, y: u8) -> void;

// CPC-specific graphics
fun mode(screen_mode: u8) -> void;  // Mode 0, 1, 2
fun ink(color: u8, palette: u8) -> void;
fun plot(x: u16, y: u16, ink: u8) -> void;
fun draw(x: u16, y: u16, ink: u8) -> void;

// Disk and tape I/O
fun disc_in(filename: *u8) -> bool;
fun disc_out(filename: *u8, data: *u8, length: u16) -> bool;
fun tape_in(filename: *u8) -> bool;
fun tape_out(filename: *u8, data: *u8, length: u16) -> bool;
```

### Phase 2: 8-bit Platform Expansion (6-12 Months)

#### Commodore 64 Support 🎯 Major Platform
**Target**: Complete C64 development environment

**Key Features:**
- KERNAL ROM integration
- VIC-II graphics chip support  
- SID sound chip programming
- 1541 disk drive integration
- Cartridge development support

#### Apple II Support 🎯 Major Platform  
**Target**: Apple II/IIe/IIc family support

**Key Features:**
- ProDOS file system integration
- Hi-Res and Double Hi-Res graphics
- Mockingboard sound support
- Disk II and ProFile integration
- Slot-based peripheral support

#### BBC Micro Support 🔧 Specialized Platform
**Target**: BBC Micro Model B and Master support

**Key Features:**
- BBC BASIC integration
- Acorn DFS/ADFS file systems
- Teletext mode support
- User port I/O programming
- Tube coprocessor support

### Phase 3: Advanced Z80 Systems (12+ Months)

#### RC2014 Support 🔧 Modern Hobbyist Platform
**Target**: RC2014 modular computer support

**Key Features:**
- CP/M 2.2 and CP/M 3.0 compatibility
- Modular I/O card support
- CF card and SD card file systems
- Serial console optimization
- Real-time clock integration

#### TRS-80 Support 🔧 Historical Platform
**Target**: TRS-80 Model I/III/4 support

**Key Features:**
- TRSDOS file system integration
- Cassette tape I/O
- Floppy disk support
- Level II BASIC integration
- Hi-Res graphics board support

## Implementation Strategy

### 1. Modular Standard Library Architecture

```
stdlib/
├── core/           # Platform-independent core
│   ├── types.minz
│   ├── memory.minz
│   └── string.minz
├── cpm/           # CP/M specific
│   ├── stdio.minz
│   ├── bdos.minz
│   └── filesystem.minz
├── msx/           # MSX specific
│   ├── stdio.minz
│   ├── graphics.minz
│   └── sound.minz
├── zx/            # ZX Spectrum (existing)
│   ├── screen.minz
│   └── input.minz
└── platform/      # Platform detection
    └── detect.minz
```

### 2. @abi Integration Strategy

Each platform will leverage MinZ's revolutionary @abi system for zero-overhead system call integration:

```minz
// CP/M BDOS calls
@abi("register: C=function, DE=parameter")
@extern
fun bdos(function: u8, parameter: u16) -> u16;

// MSX BIOS calls  
@abi("register: A=function, HL=parameter")
@extern
fun msxbios(function: u8, parameter: u16) -> u16;

// Direct hardware access
@abi("register: A=data, BC=port")
@extern
fun out_port(port: u16, data: u8) -> void;
```

### 3. Cross-Platform Abstraction

```minz
// Platform-independent I/O
interface Console {
    fun print(text: *u8) -> void;
    fun input() -> *u8;
    fun clear() -> void;
}

// Platform-specific implementations
impl Console for CPMConsole { ... }
impl Console for MSXConsole { ... }
impl Console for ZXConsole { ... }
```

## Development Priorities

### High Priority (Next 3 Months)
1. **CP/M stdio library** - Essential for CP/M ecosystem
2. **MSX graphics/sound library** - Major platform expansion
3. **Cross-platform build system** - Support multiple targets

### Medium Priority (3-6 Months)
1. **Amstrad CPC support** - European market expansion
2. **Enhanced ZX Spectrum library** - Complete existing platform
3. **Commodore 64 foundation** - Major platform preparation

### Low Priority (6+ Months)
1. **Apple II support** - US market expansion
2. **BBC Micro support** - UK education market
3. **Hobbyist platforms** - RC2014, TRS-80

## Technical Challenges

### 1. Memory Management
Different platforms have varying memory layouts:
- **CP/M**: 64KB with CP/M overhead
- **MSX**: Cartridge slots and memory mappers
- **C64**: Bank switching and zero page optimization
- **Apple II**: Language card and auxiliary memory

### 2. File System Integration
Platform-specific file systems require different approaches:
- **CP/M**: FCB (File Control Block) based
- **MSX-DOS**: FAT-like file system
- **ProDOS**: Block-based file system
- **ZX Spectrum**: Tape-based sequential access

### 3. Graphics Abstraction
Widely different graphics capabilities:
- **Text-only**: CP/M, some terminals
- **Character graphics**: ZX Spectrum, BBC Micro
- **Bitmap graphics**: MSX, Amstrad CPC, C64
- **Hi-res graphics**: Apple II, some MSX modes

## Success Metrics

### Platform Adoption
- **Primary Goal**: 80% of platform-specific examples compile successfully
- **Quality Goal**: Generated code within 10% of hand-optimized assembly
- **Performance Goal**: TRUE SMC optimization works on all platforms

### Community Impact
- **Documentation**: Complete platform-specific tutorials
- **Examples**: 20+ examples per major platform
- **Integration**: Seamless cross-platform development workflow

## Long-Term Vision

MinZ aims to become the premier high-level language for Z80 development across all platforms, providing:

1. **Zero-cost abstractions** that work identically across platforms
2. **TRUE SMC optimization** for revolutionary performance
3. **Seamless platform integration** through @abi annotations
4. **Modern development experience** for retro computing

By focusing on the Z80 ecosystem first, MinZ establishes itself as the definitive solution for 8-bit development before expanding to other architectures.

## Contributing to Platform Support

Developers can contribute by:
1. **Testing existing code** on target platforms
2. **Writing platform-specific examples** and tutorials
3. **Implementing standard library modules** for specific platforms
4. **Creating hardware integration examples** and drivers
5. **Documenting platform-specific optimizations** and best practices

This roadmap ensures MinZ becomes the standard for high-performance, high-level programming across the entire Z80 ecosystem and beyond.