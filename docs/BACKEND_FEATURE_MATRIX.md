# MinZ Backend Feature Matrix

This document provides a comprehensive comparison of features supported by each MinZ backend.

## 🎯 Core Language Features

| Feature | Z80 | 6502 | 68000 | i8080 | WASM | C | LLVM |
|---------|-----|------|-------|-------|------|---|------|
| Basic Types (u8, u16, i8, i16) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Extended Types (u32, i32) | ❌ | ❌ | ✅ | ❌ | ✅ | ✅ | ✅ |
| Boolean Type | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Arrays | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Structs | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Pointers | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Function Calls | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Recursion | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Global Variables | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| String Literals | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

## 🚀 Advanced Features

| Feature | Z80 | 6502 | 68000 | i8080 | WASM | C | LLVM |
|---------|-----|------|-------|-------|------|---|------|
| Self-Modifying Code (SMC) | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |
| TRUE SMC | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |
| Inline Assembly | ✅ | ✅ | ✅ | ✅ | ❌ | 🚧 | 🚧 |
| @abi Integration | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |
| Interrupts | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |
| Shadow Registers | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Zero Page Optimization | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Floating Point | ❌ | ❌ | 🚧 | ❌ | ✅ | ✅ | ✅ |
| SIMD Instructions | ❌ | ❌ | ❌ | ❌ | ✅ | 🚧 | ✅ |

## 💾 Memory and Addressing

| Feature | Z80 | 6502 | 68000 | i8080 | WASM | C | LLVM |
|---------|-----|------|-------|-------|------|---|------|
| Address Space | 64KB | 64KB | 16MB | 64KB | 4GB | Native | Native |
| Pointer Size | 16-bit | 16-bit | 32-bit | 16-bit | 32-bit | Native | Native |
| Stack | Hardware | Software | Hardware | Hardware | Native | Native | Native |
| Memory Banking | 🚧 | 🚧 | ❌ | 🚧 | ❌ | ❌ | ❌ |
| Direct Page Mode | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |

## 🔧 Optimization Capabilities

| Feature | Z80 | 6502 | 68000 | i8080 | WASM | C | LLVM |
|---------|-----|------|-------|-------|------|---|------|
| Register Allocation | ✅ | ✅ | ✅ | ✅ | N/A | N/A | ✅ |
| Peephole Optimization | ✅ | 🚧 | 🚧 | 🚧 | ❌ | N/A | ✅ |
| Dead Code Elimination | ✅ | ✅ | ✅ | ✅ | ✅ | N/A | ✅ |
| Constant Folding | ✅ | ✅ | ✅ | ✅ | ✅ | N/A | ✅ |
| Function Inlining | ✅ | ✅ | ✅ | ✅ | ✅ | N/A | ✅ |
| Tail Call Optimization | ✅ | ✅ | ✅ | ✅ | ✅ | N/A | ✅ |
| Loop Optimization | 🚧 | 🚧 | 🚧 | 🚧 | 🚧 | N/A | ✅ |

## 📝 Code Generation Details

| Feature | Z80 | 6502 | 68000 | i8080 | WASM | C | LLVM |
|---------|-----|------|-------|-------|------|---|------|
| Output Format | .a80 | .s | .s | .s | .wat | .c | .ll |
| Assembler | sjasmplus | ca65 | vasm | asm80 | wat2wasm | gcc/clang | llc |
| Calling Convention | Custom | Custom | Custom | Custom | Native | cdecl | Native |
| Parameter Passing | Registers/Stack | Zero Page/Stack | Registers/Stack | Registers/Stack | Stack | Stack | Varies |
| Return Values | HL/A | A/X | D0 | HL/A | Stack | Register | Varies |

## 🎮 Platform-Specific Features

### Z80 (ZX Spectrum, Amstrad CPC, MSX)
- Shadow registers (AF', BC', DE', HL')
- Block instructions (LDIR, CPIR, etc.)
- Hardware multiply via RLD/RRD tricks
- Interrupt modes (IM 0/1/2)
- OUT/IN port instructions

### 6502 (Commodore 64, Apple II, NES)
- Zero page addressing modes
- Decimal mode (BCD arithmetic)
- Indirect indexed addressing
- Hardware stack limited to 256 bytes
- Memory-mapped I/O

### 68000 (Amiga, Atari ST, Sega Genesis)
- 8 data registers (D0-D7)
- 8 address registers (A0-A7)
- Multiple addressing modes
- Hardware multiply/divide
- Supervisor/User modes

### i8080 (CP/M systems, early microcomputers)
- Subset of Z80 instructions
- No shadow registers
- No block instructions
- Basic 8-bit architecture
- Compatible with 8085

### WASM (Web browsers, Node.js)
- Stack-based VM
- Type safety
- Memory sandbox
- No direct hardware access
- Import/Export system

### C (Native compilation)
- Portable across platforms
- Relies on C compiler optimization
- Can integrate with existing C code
- No MinZ-specific optimizations
- Good for prototyping

### LLVM (Advanced optimization)
- State-of-the-art optimizations
- Cross-platform targeting
- JIT compilation possible
- Advanced analysis passes
- Integration with LLVM ecosystem

## 🔄 Backend Status

| Backend | Status | Maturity | Test Coverage | Documentation |
|---------|--------|----------|---------------|---------------|
| Z80 | ✅ Active | Production | 95% | Complete |
| 6502 | ✅ Active | Beta | 80% | Good |
| 68000 | 🚧 Development | Alpha | 60% | Basic |
| i8080 | ✅ Active | Beta | 75% | Good |
| WASM | 🚧 Development | Alpha | 50% | Basic |
| C | ✅ Active | Beta | 70% | Good |
| LLVM | 🚧 Planned | Experimental | 10% | Minimal |

## 📊 Performance Characteristics

| Backend | Code Size | Execution Speed | Memory Usage | Optimization Potential |
|---------|-----------|-----------------|--------------|----------------------|
| Z80 | Compact | Good | Efficient | High (SMC) |
| 6502 | Very Compact | Good | Very Efficient | High (Zero Page) |
| 68000 | Moderate | Excellent | Good | Very High |
| i8080 | Compact | Moderate | Efficient | Moderate |
| WASM | Moderate | Excellent | Good | Moderate |
| C | Varies | Excellent | Varies | Depends on compiler |
| LLVM | Optimal | Excellent | Optimal | Maximum |

## 🎯 Recommended Use Cases

### Z80
- ZX Spectrum games and demos
- Amstrad CPC applications
- MSX software development
- Embedded Z80 systems
- Retro computing projects

### 6502
- Commodore 64 games and demos
- Apple II software
- NES homebrew development
- Atari 8-bit programs
- 6502-based embedded systems

### 68000
- Amiga demos and games
- Atari ST applications
- Sega Genesis homebrew
- 68K embedded systems
- Performance-critical code

### i8080
- CP/M applications
- 8080/8085 embedded systems
- Historical computing
- Educational purposes
- Minimal systems

### WASM
- Web applications
- Cross-platform deployment
- Sandboxed execution
- Modern web demos
- Browser-based tools

### C
- Prototyping and testing
- Cross-platform development
- Integration with C libraries
- Learning and debugging
- Reference implementation

### LLVM
- Maximum optimization needed
- Research and experimentation
- Cross-platform deployment
- JIT compilation scenarios
- Integration with LLVM tools

## 🔮 Future Backend Plans

1. **ARM Cortex-M** - For modern embedded systems
2. **RISC-V** - For open hardware platforms
3. **AVR** - For Arduino and similar platforms
4. **x86-64** - For modern PC development
5. **PowerPC** - For vintage Mac and game consoles

## 📝 Notes

- ✅ = Fully supported
- 🚧 = In development/Partial support
- ❌ = Not supported
- N/A = Not applicable

The feature matrix is continuously updated as backends evolve. Check the individual backend documentation for the most current information.