# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Communication Style

When writing documentation, README files, or public-facing content:
- Use humble, professional language by default
- Avoid superlatives like "revolutionary", "world's first", "breakthrough" in general docs
- Focus on technical accuracy over marketing language
- Present features as "experimental", "in development", or "research" when uncertain
- Be honest about the project's maturity level
- Use "UNDER CONSTRUCTION" warnings where appropriate

BUT! Sometimes we can be откровенны (frank/candid) and say when something is truly круто (cool) and mindblowing! 🚀
- When we achieve zero-cost abstractions that actually work - that's amazing!
- When lambdas compile to identical assembly as hand-written functions - celebrate it!
- When we solve "impossible" problems on 8-bit hardware - be proud!

The key is balance: professional humility most of the time, but genuine excitement when we earn it.

## 🚀 AI-Driven Development Practices

@CLAUDE_BEST_PRACTICES.md

## 🎯 Custom Commands Available

- `/ai-testing-revolution` - Build complete testing infrastructure using parallel agents
- `/parallel-development` - Execute multiple tasks simultaneously with AI orchestration
- `/performance-verification` - Verify optimization claims with comprehensive benchmarking

### Quick Usage Examples:
```bash
# Build comprehensive testing
/ai-testing-revolution "Create E2E tests for new optimizer"

# Parallel task execution
/parallel-development "Fix bug X, implement feature Y, add tests for Z"

# Verify optimizations
/performance-verification "Prove new optimization delivers 25% improvement"
```

## Project Overview

MinZ is a systems programming language for Z80-based computers (ZX Spectrum). The repository contains:
- **Tree-sitter grammar** for parsing MinZ syntax
- **Go-based compiler (minzc)** that generates Z80 assembly (.a80 format)
- **Advanced optimization framework** with register allocation and self-modifying code support

## 🚀 Recent Achievement: TRUE ZERO-OVERHEAD LAMBDAS!

Revolutionary breakthrough in functional programming for 8-bit systems:
- ✅ **Compile-time lambda transformation** - Lambdas become named functions
- ✅ **Zero runtime overhead** - Lambda calls identical to traditional function calls  
- ✅ **TRUE SMC integration** - Same self-modifying code optimization as regular functions
- ✅ **Function reference copying** - `let f = someFunction` works perfectly
- ✅ **Type safety preserved** - Full compile-time type checking
- ✅ **ZX Spectrum standard library** - 32-character ROM font printing routine

Performance verified: **Lambda functions run at 100% the speed of traditional functions**

## 🏆 WORLD FIRST: Zero-Cost Abstractions on 8-bit Hardware!

MinZ has achieved the impossible - modern programming abstractions with ZERO runtime overhead on Z80:

### ✅ **Zero-Overhead Lambdas** - COMPLETE
```minz
let add = |x: u8, y: u8| => u8 { x + y };
add(5, 3)  // Compiles to direct CALL - 100% performance of traditional functions
```

### ✅ **Zero-Cost Interfaces** - COMPLETE  
```minz
interface Drawable {
    fun draw(self) -> u8;
}

impl Drawable for Circle {
    fun draw(self) -> u8 { self.radius * 2 }
}

let circle = Circle { radius: 5 };
circle.draw()  // Compiles to: CALL Circle_draw - NO vtables, NO overhead!
```

### ✅ **ZX Spectrum Standard Library** - COMPLETE
- 32-character ROM font printing using hardware font at $3D00
- Hardware-optimized graphics primitives  
- Memory layout and attribute handling

### 📋 **Coming Soon**
- Generic functions with monomorphization
- Interface casting and type erasure
- Advanced standard library modules

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
- **Lambda Expressions**: Compile-time transformed into efficient functions (see Lambda Design below)

## Design Philosophy

### TSMC Reference Philosophy (Revolutionary - Article 040)
MinZ is evolving beyond traditional pointers to **TSMC-native references** where:

1. **References ARE addresses of data inside opcodes** - The data lives in immediate fields of instructions
2. **Zero indirection** - `&T` parameters become direct immediate values in instructions
3. **Self-modifying by design** - Functions modify their own immediates for iteration
4. **Code IS the data structure** - Parameters live in instruction stream, not memory

Example of the vision:
```asm
; Traditional pointer approach:
LD HL, string_addr  ; Load pointer
LD A, (HL)         ; Dereference

; TSMC reference approach:
str$immOP:
    LD A, (0000)   ; The 0000 IS the reference - patched at call time!
str$imm0 EQU str$immOP+1
```

Currently, syntax uses `*T` but semantics are evolving to true TSMC references where every pointer parameter becomes a self-modifying immediate operand. This eliminates register pressure, memory usage, and indirection overhead.

See `docs/040_TSMC_Reference_Philosophy.md` for the complete revolutionary vision and `POINTER_PHILOSOPHY.md` for the migration path.

### Lambda Design Philosophy (Compile-Time Transformation)
MinZ lambdas are **not runtime values** but compile-time constructs:

1. **Lambda assignments become named functions** - `let f = |x| { x + 1 }` creates a module function
2. **Only fully-curried lambdas can be returned** - Must be completely specialized, returning just addresses
3. **No runtime overhead** - All lambda calls compile to direct function calls
4. **Perfect SMC integration** - Currying uses parameter patching for specialization

Example:
```minz
// This lambda assignment:
let add = |x: u8, y: u8| => u8 { x + y };
add(5, 3)

// Compiles to:
fun scope$add_0(x: u8, y: u8) -> u8 { x + y }
scope$add_0(5, 3)  // Direct call!
```

For returning lambdas (must be fully curried):
```minz
fun make_adder(n: u8) -> fn(u8) -> u8 {
    @curry(|x: u8| => u8 { x + n }, n)  // Returns address of SMC-specialized function
}
```

See `docs/094_Lambda_Design_Complete.md` for full design details.

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
  - **`040_TSMC_Reference_Philosophy.md`**: Revolutionary vision - references as immediate operands
  - **`094_Lambda_Design_Complete.md`**: Lambda compile-time transformation design

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
- **NO FALLBACK PARSERS**: We maintain only one parser - tree-sitter. If something doesn't parse, fix the grammar, don't create workarounds

### Compiler Development
- The Go compiler uses tree-sitter C bindings via external process calls
- AST conversion happens in `pkg/parser/parser.go`
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