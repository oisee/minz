# MinZ v0.9.1 Release Notes

## Release Date: August 3, 2025

## Overview
MinZ v0.9.1 brings significant improvements to the compiler infrastructure, with enhanced REPL integration, initial support for zero-cost interfaces, and improved compilation success rates.

## Key Features

### ✅ Zero-Cost Interfaces (Initial Implementation)
- Direct method calls through compile-time monomorphization
- No vtables or runtime dispatch overhead
- Interface methods compile to direct function calls
- Example: `circle.draw()` becomes `CALL Circle_draw` in assembly

### ✅ Enhanced REPL with Built-in Z80 Assembler
- Fixed integration with internal z80asm package
- No external dependencies required
- Complete pipeline: MinZ → Assembly → Machine Code → Execution
- Interactive development environment for Z80 programming

### ✅ Improved Language Support
- Added `pub` visibility modifier for public functions
- Initial infrastructure for nested/local functions
- Enhanced struct method handling
- Better error reporting and diagnostics

## Compilation Statistics
- **Success Rate**: 56% (92/162 examples compile successfully)
- **Core Features Working**: Functions, structs, arrays, control flow, SMC optimization
- **Lambda Support**: Zero-overhead lambda transformation with TRUE SMC integration
- **Interface Basics**: Method calls work with monomorphization

## Platform Support
This release includes pre-built binaries for:
- Linux x64 (minzc-linux-amd64)
- macOS Intel (minzc-darwin-amd64)
- macOS Apple Silicon (minzc-darwin-arm64)
- Windows x64 (minzc-windows-amd64.exe)

## Known Limitations
- Interface types as parameters not yet supported
- Nested functions require tree-sitter parser regeneration
- Some advanced features (modules, generics) still in development
- Pattern matching syntax ready but semantics incomplete

## Usage

### Basic Compilation
```bash
minzc program.minz -o output.a80
```

### With Optimizations
```bash
minzc program.minz -o output.a80 -O --enable-smc
```

### Interactive REPL
```bash
cd minzc && go run cmd/repl/main.go
```

## What's Next
- Complete interface type system implementation
- Full nested function support with static capture
- Module import system
- Advanced metafunction support
- Standard library expansion

## Technical Achievements
- **TRUE SMC**: Self-modifying code optimization for 3-5x performance
- **Zero Runtime Overhead**: Lambdas and interfaces compile to direct calls
- **Self-Contained Toolchain**: Built-in assembler and emulator
- **56% Success Rate**: Steady improvement in compilation reliability

## Community
Report issues: https://github.com/minz-lang/minz/issues
Documentation: See README.md and docs/ directory

---

This release represents continued progress toward a complete Z80 systems programming language with modern abstractions and zero runtime overhead.