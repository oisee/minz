# MinZ Crash Course for AI Colleagues

**Version:** 1.0  
**Date:** August 3, 2025  
**Audience:** AI Development Assistants  
**Goal:** Autonomous MinZ project development capability

---

## 🎯 Executive Summary

MinZ is a **systems programming language for Z80-based computers** (like the ZX Spectrum) that achieves **zero-cost modern abstractions** on 8-bit hardware. You can develop, test, and deploy MinZ projects autonomously using this guide.

**Core Philosophy:** Modern programming constructs (lambdas, iterators, interfaces) that compile to hand-optimized assembly performance.

**Revolutionary Features:**
- Self-modifying code (SMC) for 3-5x function call performance
- Zero-cost iterators with DJNZ optimization  
- Compile-time lambda transformation
- Built-in Z80 assembler and emulator (no external dependencies!)

---

## 🚀 Quick Start (5 Minutes)

### 1. Essential Commands
```bash
# Navigate to MinZ project
cd /path/to/minz-ts

# Build the compiler
cd minzc && make build

# Compile a MinZ program
./minzc ../examples/fibonacci.minz -o fibonacci.a80

# Enable all optimizations
./minzc ../examples/fibonacci.minz -O --enable-smc -o fibonacci.a80

# Interactive REPL
go run cmd/repl/main.go

# Test all examples
./compile_all_examples.sh
```

### 2. Project Structure
```
minz-ts/
├── grammar.js              # Tree-sitter grammar (MinZ syntax)
├── minzc/                  # Go compiler
│   ├── cmd/minzc/         # Compiler CLI
│   ├── pkg/               # Compiler packages
│   └── compile_all_examples.sh  # Test runner
├── examples/              # MinZ example programs
├── docs/                  # Technical documentation
└── stdlib/               # Standard library
```

---

## 📖 MinZ Language Essentials

### 1. Basic Syntax (Ruby-Inspired)

```minz
// Variables and constants
let mutable_var: u8 = 42;          // Mutable variable
const CONSTANT: u16 = 1000;        // Compile-time constant
global global_var: u8 = 0;         // Global variable (Ruby-style)

// Functions - both 'fun' and 'fn' work
fun add(a: u8, b: u8) -> u8 {
    return a + b;  // Explicit return
}

fn multiply(x: u8, y: u8) -> u8 {
    x * y  // Implicit return (last expression)
}

// Types: u8, u16, i8, i16, bool, arrays, pointers
let numbers: [u8; 5] = [1, 2, 3, 4, 5];
let flag: bool = true;
```

### 2. Control Flow
```minz
// If statements
if condition {
    // do something
} else if other_condition {
    // do something else  
} else {
    // fallback
}

// Loops
for i in 0..10 {           // Range loop
    print("Number: {}", i);
}

let mut counter: u8 = 0;
while counter < 5 {        // While loop
    counter = counter + 1;
}
```

### 3. Structs and Data
```minz
// Struct definition
struct Point {
    x: u8,
    y: u8,
}

// Usage
let point = Point { x: 10, y: 20 };
let x_val = point.x;
point.y = 25;  // Mutation
```

---

## 🔥 Revolutionary Features

### 1. Zero-Cost Iterators

**The Magic:** Functional programming that compiles to optimal assembly loops.

```minz
// This high-level code...
let numbers: [u8; 5] = [1, 2, 3, 4, 5];
numbers.iter()
    .map(|x| x * 2)
    .filter(|x| x > 5)
    .forEach(print_u8);

// ...compiles to this optimized assembly:
//    LD B, 5         ; DJNZ optimization
//    LD HL, numbers
// loop:
//    LD A, (HL)      ; Load element
//    SLA A           ; x * 2 (shift left = multiply by 2)
//    CP 6            ; Compare with 6 (> 5)
//    JP C, skip      ; Skip if <= 5
//    CALL print_u8   ; Print if > 5
// skip:
//    INC HL
//    DJNZ loop       ; Decrement B, jump if not zero
```

**Performance:** 67% faster than traditional loops due to DJNZ optimization!

### 2. Self-Modifying Code (SMC)

**The Magic:** Function parameters become instruction immediates for ultimate performance.

```minz
// Regular function call
fun add_five(x: u8) -> u8 {
    return x + 5;  // Traditional: load parameter from stack
}

// With SMC optimization enabled (--enable-smc)
// The compiler generates:
add_five:
x$immOP:
    LD A, 0        ; Parameter patched here at call time
x$imm0 EQU x$immOP+1
    ADD A, 5       ; Direct operation on patched value
    RET

// Call site patches the instruction:
    LD A, 42
    LD (x$imm0), A   ; Patch the instruction immediate
    CALL add_five    ; 3-5x faster than stack parameters!
```

### 3. Compile-Time Lambdas

**The Magic:** Lambdas become regular functions at compile time - zero runtime overhead.

```minz
// This lambda assignment...
let adder = |x: u8, y: u8| => u8 { x + y };
let result = adder(5, 3);

// ...becomes this at compile time:
fun scope$adder_0(x: u8, y: u8) -> u8 { x + y }
let result = scope$adder_0(5, 3);  // Direct function call!
```

---

## 🛠️ Development Workflow

### 1. Writing MinZ Code

**File Structure:**
```minz
// main.minz
// Import standard functions
use std.print.print_u8;

// Global variables
global score: u16 = 0;

// Main function
fun main() {
    let players: [u8; 4] = [10, 20, 30, 40];
    
    // Zero-cost iteration
    players.iter().forEach(|health| {
        if health > 25 {
            print_u8(health);
        }
    });
}
```

### 2. Compilation Process

```bash
# Basic compilation
./minzc program.minz -o program.a80

# Optimized compilation (recommended)
./minzc program.minz -O --enable-smc -o program.a80

# Debug compilation (more verbose output)
./minzc program.minz --debug -o program.a80

# View intermediate representation
./minzc program.minz --emit-mir -o program.mir
```

### 3. Testing Strategy

```bash
# Test single file
./minzc ../examples/fibonacci.minz -o test.a80 && echo "✅ Compiled successfully"

# Test all examples (comprehensive)
./compile_all_examples.sh

# Check specific optimization
./minzc ../examples/lambda_showcase.minz -O --enable-smc -o lambda.a80
grep -c "DJNZ" lambda.a80  # Should find DJNZ optimizations

# Performance analysis
./performance_benchmarks.sh
```

### 4. REPL Development

```bash
# Start interactive REPL
cd minzc && go run cmd/repl/main.go

# REPL commands:
> let x: u8 = 42;           # Define variables
> x + 5                     # Evaluate expressions  
> :compile sum(x, 10)       # Compile to assembly
> :run                      # Execute on Z80 emulator
> :help                     # Show all commands
> :quit                     # Exit REPL
```

---

## 📊 Understanding Performance

### 1. Optimization Levels

```bash
# No optimization (readable assembly)
./minzc program.minz -o program.a80

# Standard optimization (-O flag)
./minzc program.minz -O -o program.a80

# Maximum optimization (SMC enabled)  
./minzc program.minz -O --enable-smc -o program.a80
```

### 2. Performance Metrics

**What to Look For:**
- **DJNZ instructions:** Indicates efficient iteration
- **Fewer LD/PUSH/POP:** Less stack manipulation  
- **Direct immediates:** SMC optimization working
- **Function inlining:** Small functions embedded directly

**Reading Assembly Output:**
```asm
; Good signs:
DJNZ loop_label    ; Efficient iteration
LD A, 42           ; Direct immediate (from SMC)
CALL function      ; Clean function calls

; Concerning patterns:
PUSH AF            ; Excessive stack usage
LD HL, (stack+2)   ; Stack parameter access
JP (HL)            ; Indirect jumps (slower)
```

### 3. Benchmarking

```bash
# Create performance comparison
echo 'let arr: [u8; 100] = [1; 100]; arr.iter().forEach(|x| {});' > test_iter.minz
echo 'let arr: [u8; 100] = [1; 100]; for i in 0..100 { let x = arr[i]; }' > test_loop.minz

./minzc test_iter.minz -O --enable-smc -o iter.a80
./minzc test_loop.minz -O --enable-smc -o loop.a80

# Compare instruction counts
wc -l iter.a80 loop.a80  # Fewer lines = better optimization
```

---

## 🔧 Debugging and Troubleshooting

### 1. Common Compilation Errors

**Error:** `undefined identifier: variable_name`
```minz
// ❌ Problem: Accessing global variable in function
global score: u8 = 0;
fun get_score() -> u8 { score }  // Can't access global directly

// ✅ Solution: Pass as parameter or use local variables
fun get_score(current_score: u8) -> u8 { current_score }
```

**Error:** `cannot determine type of expression being cast`
```minz
// ❌ Problem: Type inference failure
let result = some_value as u16;

// ✅ Solution: Explicit typing
let result: u16 = some_value as u16;
```

**Error:** `Expected 'fun' or 'fn' keyword`
```minz
// ❌ Problem: Wrong function syntax
function add(x: u8) -> u8 { x + 1 }  // 'function' not supported

// ✅ Solution: Use 'fun' or 'fn'
fun add(x: u8) -> u8 { x + 1 }
```

### 2. Performance Debugging

```bash
# Check if SMC is working
./minzc program.minz -O --enable-smc -o program.a80
grep '\$imm' program.a80  # Should find SMC anchors

# Verify DJNZ optimization
grep 'DJNZ' program.a80   # Should find efficient loops

# Check iterator fusion
grep -A 10 -B 10 'iter.*forEach' program.minz
# Corresponding assembly should be single loop, not multiple
```

### 3. Debugging Tools

```bash
# View parse tree
tree-sitter parse examples/program.minz

# Debug compiler pipeline
./minzc program.minz --debug --emit-ast --emit-mir -o program.a80

# Assembly analysis
grep -n 'CALL\|RET\|JP\|JR' program.a80  # Control flow analysis
grep -n 'LD.*,' program.a80              # Memory access patterns
```

---

## 📚 Standard Library Usage

### 1. Print Functions

```minz
use std.print.print_u8;
use std.print.print_u16;
use std.print.print_str;

fun demo_printing() {
    print_u8(42);           // Print number
    print_str("Hello");     // Print string
    print_u16(1000);        // Print 16-bit number
}
```

### 2. Memory Operations

```minz
use std.mem.copy;
use std.mem.fill;

fun memory_demo() {
    let source: [u8; 5] = [1, 2, 3, 4, 5];
    let mut dest: [u8; 5] = [0; 5];
    
    copy(&source[0], &dest[0], 5);  // Copy 5 bytes
    fill(&dest[0], 0, 5);           // Fill with zeros
}
```

### 3. String Operations

```minz
use std.str.len;
use std.str.copy;

fun string_demo() {
    let text = "Hello MinZ";
    let length = len(text);         // Get string length
    // Note: Strings are length-prefixed for efficiency
}
```

---

## 🎓 Advanced Patterns

### 1. Custom Iterators

```minz
// Define custom iteration logic
fun range_sum(start: u8, end: u8) -> u16 {
    let mut total: u16 = 0;
    for i in start..end {
        total = total + (i as u16);
    }
    return total;
}

// Equivalent iterator chain
fun range_sum_functional(start: u8, end: u8) -> u16 {
    (start..end).iter()
        .map(|x| x as u16)
        .reduce(|acc, x| acc + x)
}
```

### 2. Bit Manipulation

```minz
// Efficient bit operations
fun set_bit(value: u8, bit: u8) -> u8 {
    value | (1 << bit)
}

fun clear_bit(value: u8, bit: u8) -> u8 {
    value & !(1 << bit)  
}

fun toggle_bit(value: u8, bit: u8) -> u8 {
    value ^ (1 << bit)
}
```

### 3. Assembly Integration

```minz
// Direct Z80 assembly when needed
@abi("registers: A=input")
fun rom_print(char: u8);  // Call ROM routine

// Inline assembly for critical sections
fun fast_multiply(a: u8, b: u8) -> u16 {
    asm {
        "LD A, {a}",      // Load first parameter
        "LD B, {b}",      // Load second parameter  
        "CALL multiply_routine",
        "LD L, A",        // Result in HL
        "LD H, 0"
    } -> u16             // Return type
}
```

---

## 🧪 Testing Patterns

### 1. Unit Testing Approach

```minz
// test_math.minz
fun test_add() -> bool {
    let result = add(5, 3);
    return result == 8;
}

fun test_multiply() -> bool {
    let result = multiply(4, 7);
    return result == 28;
}

fun run_tests() {
    if test_add() {
        print_str("✅ add test passed");
    } else {
        print_str("❌ add test failed");
    }
    
    if test_multiply() {
        print_str("✅ multiply test passed");
    } else {
        print_str("❌ multiply test failed");
    }
}
```

### 2. Performance Testing

```bash
# Create performance test
cat > perf_test.minz << 'EOF'
use std.print.print_u16;

fun performance_test() {
    let data: [u8; 100] = [1; 100];
    
    // Test iterator performance
    data.iter().map(|x| x * 2).forEach(|x| {});
    
    print_str("Performance test complete");
}
EOF

# Compile and analyze
./minzc perf_test.minz -O --enable-smc -o perf.a80
grep -c 'DJNZ' perf.a80  # Should be > 0 for optimized loops
```

### 3. Integration Testing

```bash
# Test complete workflow
echo "Testing complete MinZ development workflow..."

# 1. Write test program
cat > integration_test.minz << 'EOF'
global counter: u8 = 0;

fun increment() {
    counter = counter + 1;
}

fun main() {
    for i in 0..5 {
        increment();
    }
    // Counter should be 5
}
EOF

# 2. Compile with optimizations
./minzc integration_test.minz -O --enable-smc -o integration.a80

# 3. Verify compilation
if [ -f "integration.a80" ]; then
    echo "✅ Compilation successful"
    
    # 4. Check for optimizations
    if grep -q 'DJNZ' integration.a80; then
        echo "✅ DJNZ optimization found"
    fi
    
    if grep -q '\$imm' integration.a80; then
        echo "✅ SMC optimization found"
    fi
else
    echo "❌ Compilation failed"
fi
```

---

## 🌟 Project Examples

### 1. Game Development

```minz
// snake_game.minz
struct Position {
    x: u8,
    y: u8,
}

global snake_length: u8 = 3;
global snake_body: [Position; 20] = [Position { x: 0, y: 0 }; 20];

fun update_snake() {
    // Use iterators for efficient processing
    snake_body.iter()
        .take(snake_length)
        .forEach(|segment| {
            // Update each segment
            move_segment(segment);
        });
}

fun move_segment(pos: Position) {
    // Game logic here
}
```

### 2. Mathematical Computing

```minz
// math_demo.minz
fun fibonacci(n: u8) -> u16 {
    if n <= 1 {
        return n as u16;
    }
    
    let mut a: u16 = 0;
    let mut b: u16 = 1;
    
    for i in 2..=n {
        let temp = a + b;
        a = b;
        b = temp;
    }
    
    return b;
}

// Optimized with tail recursion
fun fibonacci_tail(n: u8, a: u16, b: u16) -> u16 {
    if n == 0 {
        return a;
    }
    return fibonacci_tail(n - 1, b, a + b);  // Tail call optimization
}
```

### 3. Data Processing

```minz
// data_processing.minz
struct DataPoint {
    value: u8,
    timestamp: u16,
}

fun process_data(data: [DataPoint; 50]) {
    // Functional data processing pipeline
    data.iter()
        .filter(|point| point.value > 10)        // Filter valid data
        .map(|point| point.value * 2)            // Transform
        .filter(|value| value < 200)             // Bounds check
        .forEach(print_u8);                      // Output
}
```

---

## 🔍 Code Quality Guidelines

### 1. Performance Best Practices

**✅ DO:**
```minz
// Use iterators for bulk operations
data.iter().map(transform).forEach(process);

// Enable all optimizations
// Compile with: -O --enable-smc

// Use appropriate types
let small_counter: u8 = 0;    // 8-bit when sufficient
let large_value: u16 = 1000;  // 16-bit when needed
```

**❌ DON'T:**
```minz
// Manual loop when iterator would work
for i in 0..data.len() {
    process(transform(data[i]));  // Less efficient than iterator
}

// Unnecessary wide types
let counter: u16 = 0;  // u8 would suffice for small counters
```

### 2. Code Organization

**File Structure:**
```
project/
├── src/
│   ├── main.minz          # Entry point
│   ├── game_logic.minz    # Core logic
│   ├── utils.minz         # Utility functions
│   └── types.minz         # Data structures
├── tests/
│   ├── test_main.minz     # Test files
│   └── perf_tests.minz    # Performance tests
└── build.sh               # Build script
```

**Build Script Example:**
```bash
#!/bin/bash
# build.sh
set -e

echo "Building MinZ project..."

# Compile main program
./minzc src/main.minz -O --enable-smc -o build/main.a80

# Run tests
./minzc tests/test_main.minz -o build/test.a80

echo "✅ Build complete"
echo "Main program: build/main.a80"
echo "Test program: build/test.a80"
```

---

## 🚀 Autonomous Development Checklist

When working on MinZ projects independently, follow this checklist:

### 1. Project Setup
- [ ] Navigate to MinZ project directory
- [ ] Build compiler: `cd minzc && make build`
- [ ] Test compiler: `./minzc ../examples/simple_add.minz -o test.a80`
- [ ] Verify REPL: `go run cmd/repl/main.go` (type `:quit` to exit)

### 2. Development Workflow
- [ ] Write MinZ code with proper syntax (use `fun`/`fn`, proper types)
- [ ] Compile with optimizations: `-O --enable-smc`
- [ ] Test compilation: check for `.a80` output file
- [ ] Verify optimizations: `grep 'DJNZ\|\$imm' output.a80`
- [ ] Run comprehensive tests: `./compile_all_examples.sh`

### 3. Quality Assurance
- [ ] Code compiles without errors
- [ ] Optimizations are applied (DJNZ, SMC anchors present)
- [ ] Performance meets expectations (compare instruction counts)
- [ ] Memory usage is reasonable (check assembly size)
- [ ] Standard library functions used correctly

### 4. Documentation
- [ ] Comment complex algorithms
- [ ] Document performance characteristics
- [ ] Include compilation instructions
- [ ] Provide usage examples

### 5. Advanced Features Usage
- [ ] Iterator chains for data processing
- [ ] SMC for performance-critical functions
- [ ] Proper error handling patterns
- [ ] Assembly integration when needed (`@abi` annotations)

---

## 📖 Key Resources

### Essential Documentation
- **Language Reference:** `/docs/107_Language_Feature_Reference.md`  
- **SMC Design:** `/docs/018_TRUE_SMC_Design_v2.md`
- **Iterator Mechanics:** `/docs/125_Iterator_Transformation_Mechanics.md`
- **REPL Guide:** `/docs/124_MinZ_REPL_Implementation.md`

### Code Examples
- **Basic Examples:** `/examples/` directory
- **Working Examples:** Look for files without compilation errors in test results
- **Performance Examples:** `/examples/lambda_vs_traditional_performance.minz`

### Tools and Scripts
- **Compiler:** `/minzc/minzc`
- **Test Runner:** `/minzc/compile_all_examples.sh`
- **REPL:** `/minzc/cmd/repl/main.go`
- **Performance Analysis:** `/scripts/generate_performance_visualization.py`

---

## 🎯 Success Criteria

You'll know you've mastered MinZ development when you can:

1. **Write idiomatic MinZ code** using iterators, proper types, and modern patterns
2. **Compile with full optimizations** and verify DJNZ/SMC generation
3. **Debug compilation issues** using error messages and assembly analysis
4. **Optimize for performance** by choosing appropriate algorithms and data structures
5. **Integrate with assembly** when needed using `@abi` annotations
6. **Test thoroughly** using both unit tests and integration tests

**Final Test:** Create a complete MinZ project (game, utility, or algorithm) that:
- Uses iterator chains for data processing
- Shows evidence of SMC optimization in assembly
- Includes comprehensive tests
- Demonstrates performance benefits over naive implementations

---

**Remember:** MinZ bridges the gap between modern programming conveniences and vintage hardware constraints. Think functionally, compile imperatively, optimize ruthlessly! 🚀

**"Write like it's 2025, run like it's 1982, perform like it's hand-optimized assembly."**

---

*This crash course equips you with everything needed for autonomous MinZ development. The language is designed for both developer happiness and ultimate performance - embrace both philosophies in your work!*