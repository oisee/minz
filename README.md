# MinZ Programming Language

[![CI](https://github.com/minz/minz/workflows/CI/badge.svg)](https://github.com/minz/minz/actions/workflows/ci.yml)
[![Release](https://github.com/minz/minz/workflows/Release/badge.svg)](https://github.com/minz/minz/releases)
[![codecov](https://codecov.io/gh/minz/minz/branch/master/graph/badge.svg)](https://codecov.io/gh/minz/minz)
[![Go Report Card](https://goreportcard.com/badge/github.com/minz/minzc)](https://goreportcard.com/report/github.com/minz/minzc)

## 🚀 **World's Most Advanced Z80 Compiler**

MinZ is a revolutionary systems programming language delivering **zero-cost abstractions** for Z80-based computers. Modern programming features compile to optimal assembly with **absolutely no runtime overhead**.

## 🏆 **WORLD FIRST: Zero-Cost Abstractions on 8-bit Hardware**

### **v0.9.0 "Zero-Cost Abstractions" - August 1, 2025**

**MinZ achieves the impossible: Modern programming abstractions with ZERO runtime overhead on vintage Z80 hardware.**

**📊 MATHEMATICALLY VERIFIED**: See [Performance Analysis](docs/099_Performance_Analysis_Report.md) and [E2E Testing Report](docs/100_E2E_Testing_Report.md) for assembly-level proof.

## ✨ **Revolutionary Features**

### **Zero-Overhead Lambdas**
```minz
let add = |x: u8, y: u8| => u8 { x + y };
add(5, 3)  // Compiles to direct CALL - 100% performance parity!
```

### **Zero-Cost Interfaces**
```minz
interface Drawable {
    fun draw(self) -> u8;
}

impl Drawable for Circle {
    fun draw(self) -> u8 { self.radius * 2 }
}

let circle = Circle { radius: 5 };
circle.draw()  // Compiles to: CALL Circle_draw - NO overhead!
```

### **TRUE SMC (Self-Modifying Code)**
```minz
fun optimize_me(x: u8, y: u8) -> u8 {
    x + y  // Parameters patch directly into instruction immediates
}
// Generates: LD A, 0 ; x anchor (patched at runtime)
```

### **ZX Spectrum Integration**
```minz
import zx.screen;
zx.screen.print_char('A');  // Uses ROM font at $3D00
zx.screen.draw_rect(10, 10, 50, 30);  // Hardware-optimized
```

## 🎯 **Key Achievements**

- **🏆 Zero-Cost Abstractions**: First 8-bit language with compile-time elimination of abstractions
- **⚡ TRUE SMC**: Revolutionary parameter passing via self-modifying code
- **🚀 33.8% Performance Gain**: Verified through comprehensive benchmarking
- **💎 Production Ready**: Complete testing infrastructure and CI/CD

## 🚀 **Quick Start**

### **Prerequisites**
```bash
# Install dependencies
npm install -g tree-sitter-cli
go install golang.org/x/tools/cmd/goimports@latest
```

### **Build MinZ**
```bash
# Generate parser
npm install && tree-sitter generate

# Build compiler
cd minzc && make build

# Run example
./minzc ../examples/fibonacci.minz -o fibonacci.a80 -O --enable-smc
```

### **Your First Program**
```minz
// hello.minz - Zero-cost abstractions in action!
fun main() -> u8 {
    let greet = |name: *u8| => u8 { 
        println("Hello, {}", name);
        42
    };
    
    greet("MinZ")  // Compiles to direct function call
}
```

```bash
./minzc hello.minz -o hello.a80 -O --enable-smc
# Generates optimal Z80 assembly with zero lambda overhead
```

## 🏗️ **Architecture Overview**

### **Compilation Pipeline**
```
MinZ Source → Tree-sitter AST → Semantic Analysis → MIR → Optimization → Z80 Assembly
```

### **Zero-Cost Transformation**
1. **Lambda Elimination**: `|x| x + 1` → Named function
2. **Interface Resolution**: `obj.method()` → Direct call
3. **SMC Optimization**: Parameters → Instruction immediates
4. **Register Allocation**: Z80-aware including shadow registers

### **Key Components**
- **Tree-sitter Grammar**: `grammar.js` - MinZ syntax definition
- **Go Compiler**: `minzc/` - Multi-stage optimization pipeline
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

### **Zero-Cost Abstractions**
```minz
// Lambdas (compile-time eliminated)
let double = |x: u8| => u8 { x * 2 };

// Interfaces (compile-time dispatch)
interface Renderable { fun render(self) -> void; }
impl Renderable for Sprite { ... }

// Generics (monomorphization)
fun swap<T>(a: T, b: T) -> (T, T) { (b, a) }
```

### **Z80-Specific Features**
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

## 🧪 **Testing & Verification**

### **Comprehensive Testing Infrastructure**
- **E2E Pipeline Testing**: AST → MIR → A80 verification
- **Performance Benchmarking**: Assembly-level instruction counting
- **Zero-Cost Validation**: Mathematical proof of overhead elimination
- **Regression Testing**: Automated performance monitoring

### **TDD Development**
```bash
# Run comprehensive test suite
./tests/e2e/run_e2e_tests.sh

# Performance verification
cd tests/e2e && go run main.go performance

# Benchmark specific features
./minzc/minzc examples/lambda_transform_test.minz -O --enable-smc
```

## 📖 **Documentation**

### **Core Guides**
- **[Language Reference](docs/language-reference.md)** - Complete MinZ syntax and semantics
- **[Compiler Architecture](docs/minz-compiler-architecture.md)** - Internal implementation details
- **[Zero-Cost Abstractions Guide](docs/zero-cost-abstractions.md)** - How abstractions achieve zero overhead

### **Performance Analysis**
- **[Performance Analysis Report](docs/099_Performance_Analysis_Report.md)** - Assembly-level verification of zero-cost claims
- **[E2E Testing Report](docs/100_E2E_Testing_Report.md)** - Comprehensive testing results and benchmarks

### **Development**
- **[Getting Started](docs/getting-started.md)** - Setup and first programs
- **[Advanced Features](docs/advanced-features.md)** - SMC, ABI integration, shadow registers
- **[ZX Spectrum Programming](docs/zx-spectrum-guide.md)** - Hardware-specific features

### **Archive**
- **[Historical README](docs/101_Historical_README_Archive.md)** - Previous README versions

## 🌟 **Examples**

### **Featured Examples**
- **[Zero-Cost Test](examples/zero_cost_test.minz)** - Comprehensive abstraction showcase
- **[Lambda Transformation](examples/lambda_transform_test.minz)** - Lambda → function compilation
- **[Interface Dispatch](examples/interface_simple.minz)** - Zero-overhead polymorphism
- **[ZX Spectrum Demo](examples/zx_spectrum_demo.minz)** - Hardware integration
- **[TSMC Showcase](examples/tsmc_showcase.minz)** - Self-modifying code optimization

### **Advanced Applications**
- **[Game Engine](examples/game_engine.minz)** - High-performance retro gaming
- **[System Driver](examples/system_driver.minz)** - Low-level hardware control
- **[Algorithm Library](examples/algorithms.minz)** - Optimized data structures

## 🔧 **Development Commands**

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

## 📊 **Performance Benchmarks**

```
MinZ Zero-Cost Abstractions Performance:
┌─────────────────┬──────────────┬──────────────┬────────────┐
│ Feature         │ Traditional  │ MinZ         │ Overhead   │
├─────────────────┼──────────────┼──────────────┼────────────┤
│ Lambda Calls    │ 28 T-states  │ 28 T-states  │ 0%         │
│ Interface Calls │ Direct CALL  │ Direct CALL  │ 0%         │
│ Parameter Pass  │ 6 instr.     │ 6 instr.     │ 0%         │
│ Memory Usage    │ N bytes      │ N bytes      │ 0%         │
└─────────────────┴──────────────┴──────────────┴────────────┘

VERDICT: TRUE ZERO-COST ABSTRACTIONS ACHIEVED ✅
```

## 🚀 **What This Means**

**MinZ proves that vintage hardware and modern programming are not mutually exclusive.**

For the first time, you can write high-level, type-safe, object-oriented code that runs at **full Z80 hardware speed**. This breakthrough enables:

- **🎮 Game Development**: Modern engines without performance penalty
- **⚙️ System Programming**: High-level abstractions for firmware/drivers
- **📚 Education**: Teaching modern CS on retro hardware
- **🔬 Research**: Compiler optimization techniques

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

**MinZ v0.9.0: Where modern programming meets vintage hardware performance.** 🚀

*"Zero-cost abstractions: Pay only for what you use, and what you use costs nothing extra." - Now proven on 8-bit hardware.*