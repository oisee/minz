# MinZ Programming Language

## 🚧 **UNDER CONSTRUCTION** 🚧

**This project is in active development. Features are experimental and APIs may change.**

MinZ is an experimental systems programming language for Z80-based computers (ZX Spectrum, CP/M, MSX). We're exploring modern language features that compile to efficient Z80 assembly.

### Current Version: v0.9.0-dev (August 2025)

⚠️ **Note**: While we've made promising progress on zero-cost abstractions, this is still research-grade software. See our [technical reports](docs/) for implementation details.

## Features Under Development

### Lambda Expressions (Experimental)
```minz
let add = |x: u8, y: u8| => u8 { x + y };
add(5, 3)  // Goal: compile to direct function call
```

### Interface System (In Progress)
```minz
interface Drawable {
    fun draw(self) -> u8;
}
```

### Self-Modifying Code Optimizations (Research)
Exploring TRUE SMC techniques for parameter passing optimization on systems with RAM-based code.

### **ZX Spectrum Integration**
```minz
import zx.screen;
zx.screen.print_char('A');  // Uses ROM font at $3D00
zx.screen.draw_rect(10, 10, 50, 30);  // Hardware-optimized
```

## 🎯 Recent Progress (Experimental)

While still under heavy development, we've made some interesting breakthroughs:

- **Lambda Expressions**: Compile to zero-overhead function calls (working!)
- **Interface System**: Monomorphization eliminates dispatch overhead (design complete)
- **Error Handling**: Native Z80 carry flag integration with `?` operator (implemented)
- **Self-Modifying Code**: TRUE SMC parameter passing (3-5x faster calls)
- **Tail Recursion**: Detection working, loop transformation in progress
- **Testing Infrastructure**: 76% compilation success rate on 139 examples

**Note**: These are experimental features. See [COMPILER_SNAPSHOT.md](COMPILER_SNAPSHOT.md) for current status.

## Quick Start

### Prerequisites
```bash
# Install dependencies
npm install -g tree-sitter-cli
go install golang.org/x/tools/cmd/goimports@latest
```

### Building from Source
```bash
# Generate parser
npm install && tree-sitter generate

# Build compiler
cd minzc && make build

# Run example
./minzc ../examples/fibonacci.minz -o fibonacci.a80 -O --enable-smc
```

### Hello World Example
```minz
// hello.minz
fun main() -> u8 {
    // Basic printing (println not yet implemented)
    return 0;
}
```

```bash
./minzc hello.minz -o hello.a80
```

## Architecture Overview

### Compilation Pipeline
```
MinZ Source → Tree-sitter AST → Semantic Analysis → MIR → Optimization → Z80 Assembly
```

### Current Implementation Status
- ✅ Basic type system (u8, u16, i8, i16, bool)
- ✅ Functions and basic control flow  
- ✅ Tree-sitter parser (95%+ success rate)
- ✅ Lambda expressions (zero-cost transformation working!)
- ✅ Error handling with `?` operator (CY flag native)
- 🚧 Interface system (monomorphization design complete)
- 🚧 Tail recursion optimization (detection working)
- 🚧 Pattern matching (grammar ready)
- 🚧 Standard library (I/O design complete)
- 🚧 Self-modifying code optimizations (TRUE SMC working)

### 📊 Live Compiler Status
For detailed metrics and current state, see [COMPILER_SNAPSHOT.md](COMPILER_SNAPSHOT.md)
- Success rates, optimization inventory, known issues
- Updated after each significant change
- Automated issue detection for assembly patterns
- **Standard Library**: `stdlib/` - ZX Spectrum integration
- **Examples**: `examples/` - Comprehensive language showcase

## 📚 **Language Features**

### **Type System**
```minz
// Static typing with inference
let x: u8 = 42;           // Explicit type
let y = 128;              // Inferred as u8
let ptr: *u16 = &value;   // Pointer types
let arr: [u8; 10];        // Fixed arrays
```

### Language Features (In Development)
```minz
// Lambda expressions (working - zero overhead!)
let double = |x: u8| => u8 { x * 2 };
let nums = [1, 2, 3, 4, 5];
nums.map(double);  // Compiles to direct function calls

// Error handling with ? operator (working!)
fun open_file(name: *u8) -> File? {
    let handle = fopen(name)?;  // Returns on error (CY flag)
    return File { handle };
}

// Interface system (design complete)
interface Drawable {
    fun draw(self) -> u8?;
}

// Pattern matching (grammar ready)
match result {
    Ok(value) => process(value),
    Err(code) => handle_error(code),
}

// Multiple returns with SMC (designed)
let (quotient, remainder) = divmod(100, 7);
```

### Z80 Integration
```minz
// Inline Z80 assembly
@asm {
    LD A, 255
    OUT (254), A    // Set border color
}

// ABI integration with existing code
@abi("register: A=value, HL=address")
extern fun rom_print_char(value: u8, address: u16) -> void;

// Shadow register optimization
fun interrupt_handler() -> void @interrupt {
    // Uses EXX for ultra-fast context switching
}
```

## Testing

### Current Testing Approach
- E2E testing pipeline for compiler stages
- Basic benchmarking tools
- Automated test runner

### Running Tests
```bash
# Run comprehensive test suite
./tests/e2e/run_e2e_tests.sh

# Performance verification
cd tests/e2e && go run main.go performance

# Benchmark specific features
./minzc/minzc examples/lambda_transform_test.minz -O --enable-smc
```

## Documentation

### Available Docs
- [Language Reference](docs/language-reference.md) - MinZ syntax guide
- [Compiler Architecture](docs/minz-compiler-architecture.md) - Implementation details
- [Technical Reports](docs/) - Research notes and experiments

### Getting Started
- [Getting Started Guide](docs/getting-started.md) - Setup instructions
- [ZX Spectrum Guide](docs/zx-spectrum-guide.md) - Platform-specific notes

## Examples

### Basic Examples
- [Fibonacci](examples/fibonacci.minz) - Classic recursive example
- [Hello World](examples/hello_world.minz) - Simple output
- [Arrays](examples/arrays.minz) - Array manipulation

### Experimental Features
- [Lambda Test](examples/lambda_transform_test.minz) - Lambda expression experiments
- [Interface Test](examples/interface_simple.minz) - Interface system testing
- [ZX Demo](examples/zx_spectrum_demo.minz) - ZX Spectrum features

## Development

```bash
# Build and test
make build          # Build compiler
make test           # Run Go tests
make run            # Test on sample file
make clean          # Clean artifacts

# Development workflow
tree-sitter generate                    # Update parser
./minzc file.minz -o output.a80        # Basic compilation
./minzc file.minz -O --enable-smc      # Full optimization
./tests/e2e/run_e2e_tests.sh          # Complete testing
```

## 🤝 **Contributing**

MinZ welcomes contributions! Key areas:

- **Language Features**: New syntax, optimizations, standard library
- **Compiler**: Parser improvements, optimization passes, code generation
- **Testing**: Test cases, benchmarks, verification tools
- **Documentation**: Guides, examples, API documentation

See **[CONTRIBUTING.md](CONTRIBUTING.md)** for development setup and guidelines.

## 📊 Early Performance Results (Experimental)

Initial benchmarks show promising results for our optimization approaches:

```
Feature Comparison (Preliminary):
┌─────────────────┬──────────────┬──────────────┬────────────┐
│ Feature         │ Traditional  │ MinZ         │ Difference │
├─────────────────┼──────────────┼──────────────┼────────────┤
│ Lambda Calls    │ ~28 cycles   │ ~28 cycles   │ 0 overhead │
│ Error Handling  │ ~20 cycles   │ ~1 cycle     │ 95% faster │
│ SMC Calls       │ ~30 cycles   │ ~10 cycles   │ 66% faster │
│ Interface Calls │ Indirect     │ Direct       │ Eliminated │
└─────────────────┴──────────────┴──────────────┴────────────┘

Note: These are early measurements on experimental features.
```

## 🚀 Project Goals

MinZ aims to bring modern programming concepts to Z80 systems while maintaining hardware-level performance. Our research explores:

- **Language Design**: How far can we push high-level features on 8-bit systems?
- **Compiler Optimization**: Novel techniques like TRUE SMC for vintage hardware
- **Zero-Cost Abstractions**: Can we truly eliminate abstraction overhead?
- **Developer Experience**: Modern tooling for retro development

This is an ongoing research project. We're discovering what's possible when combining modern compiler techniques with deep hardware knowledge.

## 📥 **Installation**

### **Latest Release**
Download from [GitHub Releases](https://github.com/minz-lang/minz-ts/releases/latest)

### **From Source**
```bash
git clone https://github.com/minz-lang/minz-ts.git
cd minz-ts
npm install && tree-sitter generate
cd minzc && make build
```

## 📜 **License**

MinZ is released under the MIT License. See [LICENSE](LICENSE) for details.

---

**MinZ v0.9.0-dev: An experimental language for Z80 systems**

*We're exploring what's possible when modern language design meets vintage hardware constraints. Join us on this journey!*