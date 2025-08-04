# MinZ Programming Language
![](/media/minz-logo-shamrock-mint.png)

## 🚧 **UNDER CONSTRUCTION** 🚧

**A modern systems programming language for Z80-based computers** (ZX Spectrum, CP/M, MSX)

### Current Version: v0.9.4 "Early Development" (August 2025)

🔬 **EXPERIMENTAL RELEASE**: MinZ v0.9.4 is exploring advanced compiler techniques including metaprogramming and optimization for Z80 systems. This is early-stage research software with promising initial results.

🚧 **DEVELOPMENT STATUS**: Core language features work (functions, structs, basic types), with experimental work ongoing for advanced features like metaprogramming and iterator chains. Error propagation system recently implemented.

⚠️ **Important**: This is experimental research software. The language is actively evolving and many features are still under development. Not yet suitable for production use.

📋 **Stability Roadmap**: See [STABILITY_ROADMAP.md](STABILITY_ROADMAP.md) for our detailed plan to reach v1.0 production readiness by November 2025.

## 📖 **Quick Syntax Reference**

New to MinZ? Here's the essential syntax at a glance:

```minz
// Variables and constants
let age: u8 = 25;           // Immutable variable
var score: u16 = 1000;      // Mutable variable  
const MAX_HP: u8 = 100;     // Compile-time constant
global lives: u8 = 3;       // Global variable

// Functions
fun greet(name: string) -> void {
    @print("Hello {}!", name);
}

// Error-throwing functions (NEW!)
fun divide?(a: u8, b: u8) -> u8 ? MathError {
    if b == 0 { @error(MathError.DivideByZero); }
    return a / b;
}

// Types
enum Status { Ready, Busy, Error }
struct Player { name: string, hp: u8, mp: u8 }

// Control flow
if condition { 
    do_something(); 
} else { 
    do_other(); 
}

for i in 0..10 { process(i); }
while alive { keep_going(); }

// Error handling with ?? operator
let result = risky_operation?() ?? default_value;

// Zero-cost lambdas (compile-time transformation)
let double = |x: u8| => u8 { x * 2 };
let mapped = numbers.map(double);  // Becomes direct function call

// Zero-cost iterators (experimental)
numbers
    .filter(|x| x > 5)      // Keep values > 5
    .map(|x| x * 2)         // Double each
    .forEach(print_u8);     // Print results
// ^ Compiles to single optimized loop!

// Zero-cost interfaces (experimental)
interface Drawable {
    fun draw(self) -> u8;
}

impl Drawable for Circle {
    fun draw(self) -> u8 { self.radius * 2 }
}

let circle = Circle { radius: 5 };
circle.draw()  // Direct call - no vtables!
```

**📚 Complete syntax guide**: See our [AI Colleagues MinZ Crash Course](AI_COLLEAGUES_MINZ_CRASH_COURSE.md) for comprehensive examples and patterns.

**Status**: ✅ Error propagation working | 🚧 Lambdas, iterators, interfaces experimental

## 🔬 **Research Goals: Advanced Language Features for Z80**

```minz
// Current working features:
fun fibonacci(n: u8) -> u8 {
    if n <= 1 {
        return n;
    }
    return fibonacci(n - 1) + fibonacci(n - 2);
}

// Error propagation system (recently implemented):
enum MathError { DivideByZero, Overflow }

fun safe_divide?(a: u8, b: u8) -> u8 ? MathError {
    if b == 0 {
        @error(MathError.DivideByZero);
    }
    return a / b;
}

fun example() -> void {
    let result = safe_divide?(10, 2) ?? 0;  // Returns 5
    let failed = safe_divide?(10, 0) ?? 0;  // Returns 0 (default)
    @print("Results: {} {}", result, failed);
}
```

**Current achievements:**
- **Core language**: Functions, structs, enums, basic types working
- **Error propagation**: Zero-overhead error handling with @error and ?? operators
- **Z80 code generation**: Generates working assembly for ZX Spectrum
- **Type system**: Static type checking and inference
- **Basic optimizations**: Register allocation and peephole optimization

**Experimental features under development:**
- Template metaprogramming with @minz functions
- Iterator chains with functional programming syntax
- Advanced optimization passes

## 🎯 Recent Progress (v0.9.4)

- 🔧 **Error Propagation**: Zero-overhead error handling system implemented
- 🚧 **@minz Metafunctions**: Experimental compile-time code generation (work in progress)
- 🔧 **Template Substitution**: {0}, {1}, {2} parameter system (experimental)
- 🔧 **MIR Interpreter**: Basic compile-time execution support (in development)
- 🚧 **Iterator Chains**: Functional programming syntax research (experimental)
- 🔧 **Z80 Optimizations**: Basic register allocation and peephole optimization
- 🔧 **Type Safety**: Compile-time type checking (basic implementation)
- 📊 **~60% success rate** on test examples (improving steadily)
- 🚧 **Core Features**: Functions, types, control flow working; advanced features experimental

[See full release notes](RELEASE_NOTES_v0.9.4.md) | [MIR Interpreter design](docs/126_MIR_Interpreter_Design.md)

## Key Features

### 🚧 @minz Metafunctions (Experimental)
```minz
// Research: Compile-time code generation for Z80
@minz("fun greet_{0}() -> void { @print(\"Hello {0}!\"); }", "alice")
@minz("fun greet_{0}() -> void { @print(\"Hello {0}!\"); }", "bob")

// Goal: Generate optimized functions
// fun greet_alice() -> void { @print("Hello alice!"); }
// fun greet_bob() -> void { @print("Hello bob!"); }

// Template parameter system under development:
@minz("var {0}_hp: u8 = {1}; var {0}_mp: u8 = {2};", "player", "100", "50")

// Note: This feature is experimental and not fully implemented yet
```

### 🚧 Iterator Chains (Research)
```minz
// Goal: Functional programming syntax for Z80
let scores: [u8; 10] = [45, 67, 89, 92, 78, 85, 91, 88, 76, 95];

// Research: Compile iterator chains to optimized loops
scores
    .filter(|x| x >= 80)    // Keep high scores  
    .map(|x| x / 10)        // Convert to grade
    .filter(|x| x == 9)     // Keep A grades
    .forEach(celebrate);     // Process results

// Note: Iterator chains are experimental; basic for loops work well
```

### ✅ Error Propagation (Recently Implemented)
```minz
// Zero-overhead error handling on Z80
enum MathError { DivideByZero, Overflow }
enum AppError { Math, IO, Validation }

// Functions that can throw errors
fun safe_divide?(a: u8, b: u8) -> u8 ? MathError {
    if b == 0 { @error(MathError.DivideByZero); }
    return a / b;
}

// Error propagation with type conversion
fun process_data?(input: u8) -> u8 ? AppError {
    let result = safe_divide?(input, 2) ?? @error;  // MathError -> AppError
    return result * 2;
}

// Usage with nil coalescing  
fun main() -> void {
    let result = process_data?(10) ?? 99;  // Default value on error
    @print("Result: {}", result);
}

// Same-type propagation generates single RET instruction
```

### 🚧 Loop Optimization (Research)
```minz
// Goal: Optimize loop structures for Z80
for i in 0..10 {
    if data[i] > threshold {
        process(data[i]);
    }
}

// Basic loop optimization works; advanced chain fusion under research
```

### 🔧 Self-Modifying Code (Experimental)
```minz
#[smc_enabled]
fun add(a: u8, b: u8) -> u8 {
    return a + b;  // Parameters patched directly into code!
}
// Research goal: Faster function calls through SMC (experimental)
```

### 🔧 @abi Integration (Basic Support)
```minz
// Call existing assembly/ROM routines with precise register mapping
@abi("register: A=char")
extern fun rom_print_char(char: u8) -> void;

@abi("register: HL=addr, DE=len")
extern fun custom_memcpy(addr: u16, len: u16) -> void;
```

### 🚧 Advanced Debugging (Research)

**TAS-Inspired Debugging System (Experimental)**
```bash
mz game.minz --debug --tas
> record                  # Goal: Record execution
> play                    # Basic debugging support
> rewind 1000            # Research: Time-travel debugging
> savestate checkpoint   # Experimental feature
> continue               # Standard debugging
```
- **Basic debugging** support implemented
- **Advanced features** under research and development
- **Cycle recording** system in early development

### 🚧 Features In Development

**📋 See [STABILITY_ROADMAP.md](STABILITY_ROADMAP.md) for our 3-phase plan to v1.0 (14 weeks)**

- **Interfaces**: Design complete, implementation in progress
- **Module System**: Import mechanism being built
- **Standard Library**: Core functions being added
- **Advanced Metafunctions**: @hex, @bin, @debug planned
- **Pattern Matching**: Grammar ready, semantics next

## 📊 Current Status

```
Development Status (v0.9.4):
┌─────────────────────┬──────────────┬────────────────────────┐
│ Feature             │ Status       │ Notes                  │
├─────────────────────┼──────────────┼────────────────────────┤
│ Basic functions     │ ✅ Working   │ Good reliability       │
│ Types & structs     │ ✅ Working   │ Basic implementation   │
│ Error propagation   │ ✅ Working   │ Recently implemented   │
│ Metaprogramming     │ 🚧 Research  │ Experimental           │
│ Iterator chains     │ 🚧 Research  │ Basic loops work       │
│ Module system       │ 🚧 Planned   │ Design in progress     │
│ Standard library    │ 🚧 Basic     │ Core functions needed  │
└─────────────────────┴──────────────┴────────────────────────┘

Success Rate: ~60% of test examples compile and run correctly
```

## Quick Start

### Installation
```bash
# Clone and install
git clone https://github.com/oisee/minz.git
cd minz
./install.sh  # Installs to ~/.local/bin

# Or build from source
cd minzc
make all
```

### Commands
```bash
mz  file.minz   # Compile MinZ to Z80 assembly (like 'go build')
mzr             # Start interactive REPL (MinZ REPL)

# Alternative names for compatibility:
minzc           # Same as mz
minz            # Same as mzr
```

### Hello World Example
```minz
// hello.minz - Basic example
fun main() -> void {
    @print("Hello, World!\n");
    @print("MinZ { 1 + 1 } on Z80!");  // Prints: "MinZ 2 on Z80!"
}

// iterator_hello.minz - Showcase zero-cost iterators!
fun main() -> void {
    let numbers: [u8; 5] = [1, 2, 3, 4, 5];
    
    @print("Iterator Revolution Demo:\n");
    
    // Zero-cost functional programming on Z80!
    numbers
        .map(|x| x * 2)      // Double each
        .filter(|x| x > 5)   // Keep > 5
        .forEach(print_u8);  // Print results
    
    @print("\nCompiled to ONE optimized loop!\n");
}

fun print_u8(x: u8) -> void {
    // Print implementation
}
```

```bash
# Compile with optimizations
mz hello.minz -o hello.a80 -O --enable-smc

# Compile iterator example (NEW!)
mz iterator_hello.minz -o iter.a80 -O  # Automatic DJNZ optimization

# Run in REPL for interactive testing
mzr
minz> let nums: [u8; 3] = [1,2,3];
minz> nums.map(|x| x*2).forEach(print_u8);  // Zero-cost functional programming!
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

### Implementation Status (v0.9.4)

✅ **Working Features (60% of examples compile)**
- **@minz metafunctions** - Revolutionary compile-time code generation
- **Template substitution** - {0}, {1}, {2} parameter expansion
- **MIR interpreter** - Complete compile-time execution environment
- **Zero-cost iterator chains** (.map, .filter, .forEach) with fusion
- **DJNZ optimization** for arrays ≤255 elements (3x faster)
- **Perfect type safety** through iterator chains and generated code
- Core type system (u8, u16, i8, i16, bool)
- Functions, variables, control flow
- Arrays, structs, pointers
- String operations with smart optimization
- @print with compile-time evaluation
- @abi for assembly integration
- Basic lambda expressions
- Self-modifying code optimization

🚧 **In Progress (40% need these)**
- **Advanced @minz features** (conditional generation, loops)
- **Lambda support in iterator chains** (syntax ready)
- Interface implementation (self parameter issue)
- Module import system
- Standard library functions (print_u8, etc.)
- More metafunctions (@hex, @bin, @debug)
- `reduce` and `collect` operations
- String iteration
- More collection types (lists, sets)

See [Iterator Transformation Mechanics](docs/125_Iterator_Transformation_Mechanics.md) for technical details.

## 📚 **Language Features**

### **Type System**
```minz
// Static typing with inference
let x: u8 = 42;           // Explicit type
let y = 128;              // Inferred as u8
let ptr: *u16 = &value;   // Pointer types
let arr: [u8; 10];        // Fixed arrays
```

### 🏆 Zero-Cost Abstractions on 8-bit Hardware

MinZ achieves the **impossible**: modern programming abstractions with **ZERO runtime overhead** on Z80! 

#### Zero-Cost Interfaces (Monomorphization)
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

#### Zero-Overhead Lambdas
```minz
let add_five = |x: u8| => u8 { x + 5 };
add_five(10)  // Compiles to direct CALL - 100% performance of functions!

// Higher-order functions with zero cost
enemies.forEach(|enemy| enemy.update(player_pos));
```

**Performance verified**: Lambda functions run at **100% the speed** of traditional functions!

#### ✅ Zero-Cost Iterator Chains - COMPLETE (v0.9.3)
```minz
// THE IMPOSSIBLE ACHIEVED: Functional programming with ZERO overhead on Z80!
scores.map(|x| x + 5)           // Add bonus
      .filter(|x| x >= 90)      // High scores only
      .forEach(|x| print_u8(x)); // Print results

// Compiles to SINGLE optimized loop - NO function calls, NO allocations!
// Uses DJNZ instruction for arrays ≤255 elements (67% faster!)

// ANY combination works:
numbers.forEach(print_u8);                    // ✅ Simple iteration
numbers.map(double).forEach(print_u8);        // ✅ Transform + print
numbers.filter(is_even).forEach(print_u8);    // ✅ Filter + print
numbers.map(double).filter(gt_5).forEach(print_u8); // ✅ Complex chains
```

**How it works**: Iterator chains are transformed at compile-time into imperative loops. Multiple operations fuse into single pass with DJNZ optimization. See the [Iterator Transformation Mechanics](docs/125_Iterator_Transformation_Mechanics.md) for complete mathematical analysis.

[Read the complete guide](docs/Zero_Cost_Abstractions_Explained.md) | [🚀 ITERATOR REVOLUTION](docs/Zero_Cost_Iterators_Revolution.md) | [Performance analysis](docs/094_Lambda_Design_Complete.md)

### Language Features (In Development)
```minz
// Error handling with ? operator (working!)
fun open_file(name: *u8) -> File? {
    let handle = fopen(name)?;  // Returns on error (CY flag)
    return File { handle };
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

### Core Documentation
- [Compiler Snapshot](COMPILER_SNAPSHOT.md) - Current state, features, and known issues
- [REPL Implementation](docs/124_MinZ_REPL_Implementation.md) - Interactive development environment
- [TAS Debugging Revolution](docs/127_TAS_Debugging_Revolution.md) - Time-travel debugging for Z80
- [Cycle-Perfect Recording](docs/128_TAS_Cycle_Perfect_Recording.md) - 50-600x compression with perfect replay
- [Technical Reports](docs/) - Research notes and experiments
- [AI Colleagues Crash Course](AI_COLLEAGUES_MINZ_CRASH_COURSE.md) - Complete training for AI-driven development

### Design Documents
- [Local Functions Design](docs/125_Local_Functions_Design.md) - Lexical scope and closures
- [TRUE SMC Design](docs/018_TRUE_SMC_Design_v2.md) - Self-modifying code optimization
- [Lambda Design](docs/094_Lambda_Design_Complete.md) - Lambda expressions

## Examples

### NEW: Iterator Revolution Examples ⚡
- [Iterator Comprehensive Test](test_iterator_comprehensive.minz) - All combinations working
- [Iterator Fusion Demo](test_iterator_fusion.minz) - Chain fusion in action
- [DJNZ Optimization Demo](test_iterator_visualization.minz) - Performance visualization

### Basic Examples
- [Fibonacci](examples/fibonacci.minz) - Classic recursive example  
- [Hello World](examples/hello_world.minz) - Simple output
- [Arrays](examples/arrays.minz) - Array manipulation

### Revolutionary Features
- [Zero-Cost Iterators](docs/125_Iterator_Transformation_Mechanics.md) - Complete technical guide
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
./minzc file.minz -O --enable-smc      # Full optimization with SMC
./tests/e2e/run_e2e_tests.sh          # Complete testing

# Test iterator chains (NEW in v0.9.3!)
echo 'numbers.map(double).filter(gt_5).forEach(print_u8);' | mzr
./mz iterator_example.minz -O          # Compile with DJNZ optimization
```

## 🤝 **Contributing**

MinZ welcomes contributions! Key areas:

- **Language Features**: New syntax, optimizations, standard library
- **Compiler**: Parser improvements, optimization passes, code generation
- **Testing**: Test cases, benchmarks, verification tools
- **Documentation**: Guides, examples, API documentation

### 🤖 **AI-Driven Development**
- **[AI Colleagues Crash Course](AI_COLLEAGUES_MINZ_CRASH_COURSE.md)** - Complete training for autonomous AI development
- **[CLAUDE.md](CLAUDE.md)** - AI development guidelines and best practices
- **Parallel development** supported - multiple AI agents can work independently

See **[CONTRIBUTING.md](CONTRIBUTING.md)** for development setup and guidelines.


## 🚀 Project Goals

MinZ aims to bring modern programming concepts to Z80 systems while maintaining hardware-level performance. Our research explores:

- **Language Design**: How far can we push high-level features on 8-bit systems?
- **Compiler Optimization**: Novel techniques like TRUE SMC for vintage hardware
- **Zero-Cost Abstractions**: Can we truly eliminate abstraction overhead?
- **Developer Experience**: Modern tooling for retro development

This is an ongoing research project. We're discovering what's possible when combining modern compiler techniques with deep hardware knowledge.

## 📥 **Installation**

### **Latest Release (v0.9.4 "Metaprogramming Revolution")**
Download from [GitHub Releases](https://github.com/oisee/minz/releases/tag/v0.9.4)

**Available for:**
- Linux (AMD64, ARM64)
- macOS (Intel, Apple Silicon) 
- Windows (AMD64)

**What's included:**
- `mz` - MinZ compiler with @minz metafunctions and iterator chains
- `mzr` - Interactive REPL with Z80 emulator
- Complete examples showcasing metaprogramming
- Documentation and installation scripts

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

**MinZ v0.9.4 "Metaprogramming Revolution": Compile-time code generation on Z80 hardware**

*@minz metafunctions achieved! Modern metaprogramming with zero-cost abstractions on vintage hardware!*