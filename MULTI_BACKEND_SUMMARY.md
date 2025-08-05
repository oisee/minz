# MinZ Multi-Backend Architecture Summary

## Overview
MinZ now supports multiple target platforms through a flexible backend architecture. The compiler can generate code for Z80, 6502, and WebAssembly from the same MinZ source code.

## Implemented Features

### 1. Extended Type System ✅
- **24-bit integers**: `u24`, `i24` for eZ80 addressing
- **Fixed-point types**: `f8.8`, `f.8`, `f.16`, `f16.8`, `f8.16`
- **Native implementation** for zero overhead
- **Correct register allocation**: 8-bit types use A, 16-bit use HL, 24-bit use A+HL

### 2. Backend Architecture ✅
- **Backend Interface**: Clean abstraction for code generation
- **Backend Registry**: Dynamic backend registration and selection
- **Feature Detection**: Each backend reports supported features
- **Configurable Options**: Optimization levels, SMC support, target addresses

### 3. Implemented Backends

#### Z80 Backend (Full Support) ✅
- Complete instruction set implementation
- Self-modifying code (SMC and TRUE SMC)
- Shadow register optimization
- Register allocation and optimization passes
- Production-ready for ZX Spectrum

#### 6502 Backend (Basic Support) ✅
- MIR to 6502 assembly translation
- Basic arithmetic and data movement
- Function calls and returns
- Print operations (character output)
- Platform-agnostic (C64, Apple II, etc.)

#### WASM Backend (Basic Support) ✅
- MIR to WebAssembly Text (WAT) format
- Local variable management
- Import/export handling
- Basic arithmetic operations
- Print function imports

### 4. Compiler Enhancements ✅
- `--backend` flag to select target platform
- `--list-backends` to show available backends
- Automatic file extension based on backend
- Debug output showing selected backend
- MIR file generation for all backends

### 5. Metaprogramming Support ✅
- Basic `@minz[[[]]]` block parsing
- Simple `@emit` functionality for code generation
- Foundation for compile-time code execution

## Usage Examples

```bash
# Compile for Z80 (default)
minzc program.minz -o program.a80

# Compile for 6502
minzc program.minz -b 6502 -o program.s

# Compile for WebAssembly
minzc program.minz -b wasm -o program.wat

# List available backends
minzc --list-backends

# With optimizations
minzc program.minz -b z80 -O --enable-smc
```

## Example Program
```minz
// Works on all backends!
fun main() -> void {
    let x: u8 = 42;
    let y: u16 = 1000;
    let z: u24 = 0x100000;  // 24-bit value
    let fixed: f8.8 = 3.14;  // Fixed-point
    
    @print("Multi-backend test: x={x}, y={y}");
}
```

## Current Limitations

### 24-bit Types (Partially Implemented)
- Regular SMC works correctly
- TRUE SMC only patches 16 bits (needs separate A and HL anchors)
- Recommended to avoid 24-bit parameters until fully implemented

### Backend-Specific Limitations
- **6502**: No register allocation, basic instruction support only
- **WASM**: No control flow structures (if/else, loops), string handling incomplete
- **All backends**: Limited standard library support

## Future Enhancements
1. Complete register allocation for 6502/WASM
2. Control flow implementation for all backends
3. String and array handling
4. Backend-specific optimizations
5. More target platforms (68000, ARM, RISC-V)
6. Full MinZ interpreter for `@minz` blocks

## Technical Implementation
- **Backend Interface**: `pkg/codegen/backend.go`
- **Z80 Backend**: `pkg/codegen/z80_backend.go`
- **6502 Backend**: `pkg/codegen/m6502_backend.go`
- **WASM Backend**: `pkg/codegen/wasm_backend.go`
- **Main Compiler**: Updated to use backend interface

The multi-backend architecture makes MinZ a truly portable language while maintaining its focus on efficient code generation for resource-constrained systems.