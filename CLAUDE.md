# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

MinZ is a systems programming language for Z80-based computers (ZX Spectrum). The repository contains:
- **Tree-sitter grammar** for parsing MinZ syntax
- **Go-based compiler (minzc)** that generates Z80 assembly (.a80 format)
- **Advanced optimization framework** with register allocation and self-modifying code support

## Development Commands

### Core Build Commands
```bash
# Generate tree-sitter parser
npm install
tree-sitter generate

# Build the MinZ compiler
cd minzc && make build

# Run tests
cd minzc && make test

# Build and test on sample file
cd minzc && make run

# Clean build artifacts
cd minzc && make clean
```

### Dependencies
```bash
# Install Go dependencies
cd minzc && make deps

# Install tree-sitter CLI globally
npm install -g tree-sitter-cli
```

### Testing Individual Files
```bash
# Parse a specific MinZ file  
tree-sitter parse examples/fibonacci.minz

# Compile a MinZ file to Z80 assembly
cd minzc && ./minzc ../examples/fibonacci.minz -o fibonacci.a80

# Enable optimizations
cd minzc && ./minzc ../examples/fibonacci.minz -O --enable-smc

# Test @abi assembly integration
cd minzc && ./minzc ../examples/simple_abi_demo.minz -o simple_abi_demo.a80
```

## Compiler Architecture

### Multi-Language Codebase Structure
- **Tree-sitter grammar**: JavaScript (`grammar.js`) defining MinZ syntax
- **Parser bindings**: Node.js and Rust bindings for tree-sitter integration
- **Compiler**: Go implementation in `minzc/` directory with modular packages
- **Examples**: MinZ source files demonstrating language features

### Compilation Pipeline
1. **Parsing**: Tree-sitter generates AST from MinZ source (`pkg/parser/`)
2. **Semantic Analysis**: Type checking and symbol resolution (`pkg/semantic/`)
3. **IR Generation**: Converts to intermediate representation (`pkg/ir/`)
4. **Optimization**: Advanced passes including register allocation (`pkg/optimizer/`)
5. **Code Generation**: Z80 assembly output in sjasmplus format (`pkg/codegen/`)

### Key Compiler Components

#### Register System (`pkg/ir/ir.go`)
- Physical Z80 registers (A, B, C, D, E, H, L) and 16-bit pairs (AF, BC, DE, HL, IX, IY)
- Shadow register support (Z80_*_SHADOW constants) for interrupt optimization
- RegisterSet tracking for optimization passes

#### Optimization Framework (`pkg/optimizer/`)
- **Register Analysis**: Tracks register usage patterns per function
- **Shadow Register Optimization**: Automatic use of alternative registers for performance
- **Self-Modifying Code (SMC)**: Runtime optimization of constants and parameters
- **Standard Passes**: Constant folding, dead code elimination, peephole optimization

#### Code Generation (`pkg/codegen/z80.go`)
- Generates sjasmplus-compatible `.a80` assembly files
- Lean prologue/epilogue that only saves actually used registers
- Shadow register support for interrupt handlers and performance-critical code

### Z80-Specific Features
- **Shadow Registers**: EXX and EX AF,AF' instructions for fast context switching
- **Interrupt Optimization**: Ultra-fast handlers using shadow registers (16 vs 50+ T-states)
- **Memory Layout**: Organized for 64KB address space with paging support
- **Register Allocation**: Z80-aware allocation considering 8/16-bit register relationships
- **TRUE SMC (истинный SMC)**: Parameters patched directly into instruction immediates
  - Enable with `--enable-true-smc` flag
  - See `docs/018_TRUE_SMC_Design_v2.md` for current design
  - Provides 3-5x performance improvement for function calls
- **@abi Annotations**: Seamless integration with existing assembly functions
  - Use existing ROM routines, drivers, libraries without modification
  - Precise register mapping: `@abi("register: A=param1, HL=param2")`
  - Zero overhead assembly integration

## Language Features

MinZ supports modern programming constructs while targeting Z80:
- **Type System**: Static typing with inference (u8, u16, i8, i16, bool, arrays, pointers)
- **Structs and Enums**: Organized data structures with memory-efficient layout
- **Module System**: Import/export with visibility control
- **@abi Attributes**: Revolutionary seamless assembly integration system
- **Lua Metaprogramming**: Full Lua 5.1 interpreter at compile time for code generation
- **Inline Assembly**: Direct Z80 assembly integration with register constraints

## Important Files and Directories

### Core Implementation
- `grammar.js`: Tree-sitter grammar defining MinZ syntax
- `minzc/cmd/minzc/main.go`: Compiler CLI entry point
- `minzc/pkg/`: Modular compiler packages (ast, parser, semantic, ir, optimizer, codegen)

### Documentation
- `README.md`: Complete language reference with examples
- `COMPILER_ARCHITECTURE.md`: Detailed compiler design documentation
- `DESIGN.md`: Language design philosophy and feature overview
- `docs/`: Technical guides including:
  - **`018_TRUE_SMC_Design_v2.md`**: CURRENT DESIGN for TRUE SMC implementation
  - **`029_MinZ_Strategic_Roadmap.md`**: Long-term vision and phases
  - **`030_TRUE_SMC_Lambdas_Design.md`**: Lambda implementation via SMC
  - **`031_Next_Steps_Prioritized.md`**: 70-day action plan
  - **`032_Architecture_Decision_Records.md`**: Key design decisions

### Examples and Testing
- `examples/`: Comprehensive MinZ programs showcasing all language features
- `examples/simple_abi_demo.minz`: Complete @abi demonstration with assembly integration
- `examples/asm_integration_tests.minz`: Comprehensive @abi test suite
- `test/`: Tree-sitter test corpus
- `stdlib/`: Standard library modules (std.mem, zx.screen, zx.input)

## Development Notes

### Working with Tree-sitter
- Grammar changes require running `tree-sitter generate`
- Test grammar with `tree-sitter test` and `tree-sitter parse <file>`
- Syntax highlighting queries in `queries/highlights.scm`

### Compiler Development
- The Go compiler uses tree-sitter C bindings via external process calls
- AST conversion happens in `pkg/parser/parser.go` with fallback to simple parser
- Register allocation and optimization passes are modular and can be configured by optimization level

### Output Format
- Generates sjasmplus `.a80` assembly files
- Uses ORG $8000 as default code origin
- Includes header comments with generation timestamp
- Compatible with ZX Spectrum assemblers and emulators

### Testing Strategy
- Tree-sitter corpus tests for grammar validation
- Go unit tests in optimizer packages (e.g., `optimizer_test.go`, `smc_optimization_test.go`)
- Integration tests with sample MinZ programs in examples/