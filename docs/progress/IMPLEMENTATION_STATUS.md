# MinZ Compiler Implementation Status

Last Updated: 2025-07-07

## Overview

The MinZ compiler is a Z80 assembly language compiler written in Go that compiles MinZ source code to Z80 assembly. This document outlines the current implementation status, what's working, and what remains to be implemented.

## Working Features

### Core Language Features

#### ✅ Functions
- Basic function declarations with parameters and return types
- Function calls with arguments
- Return statements
- Forward function references (functions can call other functions defined later)

```minz
fn add(a: u16, b: u16) -> u16 {
    return a + b;
}

fn main() -> void {
    let x = add(10, 20);
    return;
}
```

#### ✅ Variables
- Variable declarations with explicit type annotations
- Mutable variables with `let mut` syntax
- Basic type inference for initialized variables
- Local variable scoping

```minz
let x: u16 = 42;
let mut counter: u8 = 0;
let result = add(10, 20);  // Type inferred from function return
```

#### ✅ Basic Types
- `u8` - 8-bit unsigned integer
- `u16` - 16-bit unsigned integer
- `void` - No return value
- Number literals with automatic type inference

#### ✅ Expressions
- Binary operations: `+`, `-`, `*`, `/`, `%`, `&`, `|`, `^`
- Comparison operations: `==`, `!=`, `<`, `>`, `<=`, `>=`
- Unary operations: `-`, `!`
- Function calls as expressions
- Variable references
- Number literals

#### ✅ Control Flow
- If statements with else branches
- While loops
- Return statements

```minz
if n <= 1 {
    return n;
} else {
    return fibonacci(n - 1) + fibonacci(n - 2);
}

while i < 10 {
    sum = sum + i;
    i = i + 1;
}
```

### Compiler Infrastructure

#### ✅ Parser
- Tree-sitter based parser for accurate syntax parsing
- Fallback simple parser for basic constructs
- AST generation from source code

#### ✅ Semantic Analysis
- Symbol table management with scoping
- Type checking and inference
- Function signature validation
- Expression type validation
- Error reporting with detailed messages

#### ✅ Code Generation
- Z80 assembly output in sjasmplus format
- Register allocation for local variables
- Function prologue/epilogue generation
- Expression evaluation with register management
- Basic optimizations

#### ✅ Build System
- Go-based build using Make
- Command-line interface with options
- Debug output support

## Successfully Compiling Examples

1. **simple_add.minz** - Basic arithmetic functions
2. **fibonacci.minz** - Recursive Fibonacci implementation with loops
3. **test_simple_vars.minz** - Variable declarations and assignments

## Not Yet Implemented

### Language Features

#### ❌ Custom Types
- Struct definitions
- Enum definitions
- Type aliases
- Arrays

#### ❌ Advanced Features
- Module system and imports
- Metaprogramming with Lua integration
- Inline assembly
- Const declarations
- Global variables

#### ❌ Memory Management
- Pointer types
- Memory allocation
- Direct memory access

#### ❌ Z80-Specific Features
- Shadow register access
- Self-modifying code optimization
- Interrupt handlers
- I/O port operations

### Built-in Definitions

#### ❌ Constants
- `SCREEN_START`, `ATTR_START` for ZX Spectrum
- Other platform-specific constants

#### ❌ Built-in Functions
- Memory manipulation functions
- I/O functions
- System-specific functions

### Compiler Features

#### ❌ Optimizations
- Dead code elimination
- Constant folding
- Register optimization
- Peephole optimization

#### ❌ Advanced Code Generation
- Complex addressing modes
- Efficient struct/array access
- Optimized calling conventions

## Known Issues

1. **Type Casting**: Limited support for implicit type conversions between u8 and u16
2. **Error Recovery**: Parser may fail catastrophically on certain syntax errors
3. **Binary Operations**: Type inference for mixed-type operations needs improvement
4. **Code Generation**: Some opcodes not yet implemented (e.g., opcode 26)

## Compilation Requirements

To compile MinZ programs successfully, they must:
1. Use only implemented features (functions, basic types, variables, control flow)
2. Provide explicit type annotations for function parameters and return types
3. Avoid using undefined types (structs, enums) or built-in constants
4. Not use module/import system or metaprogramming features

## Example of Working Code

```minz
// Fibonacci sequence calculator that compiles successfully
fn fibonacci(n: u8) -> u16 {
    if n <= 1 {
        return n;
    }
    
    let mut a: u16 = 0;
    let mut b: u16 = 1;
    let mut i: u8 = 2;
    
    while i <= n {
        let temp: u16 = a + b;
        a = b;
        b = temp;
        i = i + 1;
    }
    
    return b;
}

fn main() -> void {
    let result = fibonacci(10);
}
```

## Build and Run

```bash
# Build the compiler
cd /home/alice/dev/minz/minzc
make build

# Compile a MinZ program
./minzc examples/fibonacci.minz -o fibonacci.a80

# With optimizations
./minzc examples/fibonacci.minz -O -o fibonacci.a80
```