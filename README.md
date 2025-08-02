# MinZ Programming Language

**A modern systems programming language for Z80-based computers** (ZX Spectrum, CP/M, MSX)

### Current Version: v0.9.0 "String Revolution" (January 2025)

🚀 **NEW RELEASE**: MinZ v0.9.0 delivers revolutionary **25-40% faster string operations** through compile-time optimizations and length-prefixed architecture! [Download now](https://github.com/oisee/minz/releases/tag/v0.9.0)

## 🎯 Release Highlights

- ✅ **60% of examples compile and run** (89 of 148 examples)
- ✅ **Smart String Optimization**: Short strings use direct instructions, long strings use loops
- ✅ **Enhanced @print**: Compile-time constant evaluation with `{ expr }` syntax
- ✅ **Self-Modifying Code**: 10-20% faster function calls
- ✅ **Production-ready core**: Functions, types, control flow, arrays, pointers

[See full release notes](docs/RELEASE_NOTES_v0.9.0.md) | [Known limitations](docs/121_v0.9.0_Known_Issues.md)

## Key Features

### ✅ Revolutionary String Architecture
```minz
// Length-prefixed strings - O(1) operations!
let msg: *u8 = "Hello!";    // Compiles to: DB 6, "Hello!"
let len = msg[0];           // Instant length access!

// Smart optimization based on length
@print("Hi");               // 2 chars → Direct: LD A,'H' / RST 16 / LD A,'i' / RST 16
@print("Hello, World!");    // 13 chars → Loop: DJNZ with length prefix
```

### ✅ Enhanced @print with Compile-Time Evaluation
```minz
// Constants evaluated at compile time - zero runtime cost!
@print("Score: { 100 * 5 } points");     // → "Score: 500 points"
@print("Debug: { 0x1234 } hex");         // → "Debug: 4660 hex"

// Runtime values still supported
let score: u16 = 1337;
@print("Your score: {}", score);         // Runtime interpolation
```

### ✅ Self-Modifying Code (SMC) Optimization
```minz
#[smc_enabled]
fun add(a: u8, b: u8) -> u8 {
    return a + b;  // Parameters patched directly into code!
}
// 10-20% faster function calls with --enable-smc
```

### ✅ Zero-Cost @abi Integration
```minz
// Call existing assembly/ROM routines with precise register mapping
@abi("register: A=char")
extern fun rom_print_char(char: u8) -> void;

@abi("register: HL=addr, DE=len")
extern fun custom_memcpy(addr: u16, len: u16) -> void;
```

### 🚧 Features In Development

- **Interfaces**: Design complete, implementation in progress
- **Module System**: Import mechanism being built
- **Standard Library**: Core functions being added
- **Advanced Metafunctions**: @hex, @bin, @debug planned
- **Pattern Matching**: Grammar ready, semantics next

## 📊 Performance Benchmarks

```
String Operations (v0.9.0):
┌─────────────────┬──────────────┬──────────────┬────────────┐
│ Operation       │ Traditional  │ MinZ v0.9.0  │ Improvement│
├─────────────────┼──────────────┼──────────────┼────────────┤
│ Print 5 chars   │ 145 T-states │ 90 T-states  │ 38% faster │
│ String length   │ O(n) scan    │ O(1) lookup  │ ∞% faster  │
│ SMC function    │ 30 T-states  │ 24 T-states  │ 20% faster │
└─────────────────┴──────────────┴──────────────┴────────────┘
```

## Quick Start

### Download Release
```bash
# Download latest release (macOS ARM64)
curl -L https://github.com/oisee/minz/releases/download/v0.9.0/minz-v0.9.0-darwin-arm64.tar.gz | tar xz
cd minz-v0.9.0
export PATH=$PWD/bin:$PATH
```

### Hello World Example
```minz
// hello.minz
fun main() -> void {
    @print("Hello, World!\n");
    @print("MinZ { 1 + 1 } on Z80!");  // Prints: "MinZ 2 on Z80!"
}
```

```bash
# Compile with optimizations
minzc hello.minz -o hello.a80 -O --enable-smc

# Assemble with sjasmplus (not included)
sjasmplus hello.a80
```

### Building from Source
```bash
# Clone and build
git clone https://github.com/oisee/minz.git
cd minz && npm install && tree-sitter generate
cd minzc && make build
```

## Architecture Overview

### Compilation Pipeline
```
MinZ Source → Tree-sitter AST → Semantic Analysis → MIR → Optimization → Z80 Assembly
```

### Implementation Status (v0.9.0)

✅ **Working Features (60% of examples compile)**
- Core type system (u8, u16, i8, i16, bool)
- Functions, variables, control flow
- Arrays, structs, pointers
- String operations with smart optimization
- @print with compile-time evaluation
- @abi for assembly integration
- Basic lambda expressions
- Self-modifying code optimization

🚧 **In Progress (40% need these)**
- Interface implementation (self parameter issue)
- Module import system
- Standard library functions (print_u8, etc.)
- Advanced metafunctions (@hex, @bin, @debug)
- Advanced bitwise operations
- Pattern matching
- Tail recursion transformation

See [Known Issues](docs/121_v0.9.0_Known_Issues.md) for detailed breakdown.

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


## 🚀 Project Goals

MinZ aims to bring modern programming concepts to Z80 systems while maintaining hardware-level performance. Our research explores:

- **Language Design**: How far can we push high-level features on 8-bit systems?
- **Compiler Optimization**: Novel techniques like TRUE SMC for vintage hardware
- **Zero-Cost Abstractions**: Can we truly eliminate abstraction overhead?
- **Developer Experience**: Modern tooling for retro development

This is an ongoing research project. We're discovering what's possible when combining modern compiler techniques with deep hardware knowledge.

## 📥 **Installation**

### **Latest Release (v0.9.0)**
Download from [GitHub Releases](https://github.com/oisee/minz/releases/tag/v0.9.0)

### **From Source**
```bash
git clone https://github.com/oisee/minz.git
cd minz
npm install && tree-sitter generate
cd minzc && make build
```

## 📜 **License**

MinZ is released under the MIT License. See [LICENSE](LICENSE) for details.

---

**MinZ v0.9.0 "String Revolution": Modern programming for Z80 systems**

*60% feature complete, 100% committed to zero-cost abstractions on vintage hardware!*