# MinZ v0.9.5 Release Notes - "Multi-Backend Revolution"

## 🎉 Major Features

### 🌍 Multi-Backend Architecture
MinZ now supports multiple target platforms through a clean, extensible backend system:
- **Z80** - Original backend with full SMC/TSMC support
- **6502** - Apple II, Commodore 64, NES (SMC capable)
- **WebAssembly** - Run MinZ in browsers (no SMC)
- **Game Boy** - Sharp LR35902 CPU support (NEW!)

### 📊 MIR Visualization
Generate beautiful control flow graphs from your MinZ programs:
```bash
minzc program.minz --viz program.dot
dot -Tpng program.dot -o program.png
```
- Visualize function control flow
- See basic blocks and edges
- Debug optimizations visually
- Perfect for documentation

### 🔢 Extended Type System
- **24-bit integers**: `u24`, `i24` for eZ80 addressing
- **Fixed-point types**: `f8.8`, `f.8`, `f.16`, `f16.8`, `f8.16`
- Full arithmetic support for all new types
- Efficient code generation

### 🎯 Platform-Specific Code
New `@target` directive for conditional compilation:
```minz
@target("z80") {
    asm { EXX }  // Shadow registers only on Z80
}
@target("gb") {
    asm { LDH A, [$FF44] }  // Game Boy specific
}
```

### 📦 MIR Compilation
Compile directly from intermediate representation:
```bash
# Save MIR during compilation
minzc program.minz -o program.a80  # Creates program.mir

# Compile MIR to different backends
minzc program.mir -b wasm -o program.wat
minzc program.mir -b 6502 -o program.s
```

## 🚀 Improvements

### Compiler Infrastructure
- Clean separation between frontend and backends via MIR
- Modular backend registration system
- Backend feature detection (SMC, pointer sizes, etc.)
- Improved error messages for backend-specific issues

### Documentation
- Comprehensive backend development guide
- MIR visualization guide
- Updated examples for all backends
- Platform-specific tutorials

### Testing
- Multi-backend test suite
- Visualization regression tests
- Cross-platform compatibility tests

## 🐛 Bug Fixes
- Fixed const keyword parsing and code generation
- Corrected global struct variable handling
- Fixed MIR parser for various instruction types
- Improved type inference for new extended types

## 📝 Breaking Changes
None - all existing MinZ code continues to work!

## 🔧 Technical Details

### Backend Capabilities
| Backend | SMC | Interrupts | Shadow Regs | Pointer Size |
|---------|-----|------------|-------------|--------------|
| Z80     | ✅  | ✅         | ✅          | 16-bit       |
| 6502    | ✅  | ✅         | ❌          | 16-bit       |
| WASM    | ❌  | ❌         | ❌          | 32-bit       |
| GB      | ✅  | ✅         | ❌          | 16-bit       |

### New Compiler Flags
- `--viz <file>` - Generate MIR visualization
- `-b <backend>` - Select target backend
- `--list-backends` - Show available backends

## 🎮 Game Boy Backend
Special support for Nintendo's handheld:
- RGBDS assembler syntax
- Proper memory layout (ROM0, WRAM0)
- VBlank-aware print routines
- No IX/IY or shadow registers

## 🙏 Acknowledgments
Thanks to the MinZ community for testing and feedback, especially on the new backend system!

## 📚 Documentation
- [Backend Development Guide](docs/BACKEND_DEVELOPMENT_GUIDE.md)
- [MIR Visualization Guide](docs/MIR_VISUALIZATION_GUIDE.md)
- [Multi-Backend Architecture](docs/128_Multi_Backend_Architecture_Complete.md)

## 🚀 What's Next
- 68000 backend for Amiga/Atari ST/Genesis
- Enhanced Game Boy support
- MIR-level optimizations
- Assembly correlation visualization

---
*MinZ - Modern abstractions, vintage performance, zero compromises!*