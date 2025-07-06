# MinZ Compiler Architecture and Implementation

## Overview

MinZ is a systems programming language designed specifically for the Z80 processor, with a focus on modern language features while generating efficient machine code. This article describes the current implementation status, architecture, and key innovations in the MinZ compiler.

## Language Features

MinZ provides a modern syntax inspired by Rust and Zig, while being tailored for 8-bit systems:

- **Strong static typing** with inference
- **Memory safety** through ownership semantics  
- **Zero-cost abstractions** for systems programming
- **Compile-time metaprogramming** via embedded Lua
- **First-class Z80 support** with register hints and inline assembly

### Example MinZ Code

```minz
// Modern syntax for Z80 development
fn process_buffer(data: *mut u8, len: u16) -> u16 {
    let mut checksum: u16 = 0;
    let mut i: u16 = 0;
    
    while i < len {
        checksum = checksum + data[i] as u16;
        i = i + 1;
    }
    
    return checksum;
}

// Interrupt handler with automatic register saving
@interrupt
@port(0x38)
fn timer_interrupt() -> void {
    update_system_tick();
}
```

## Compiler Architecture

The MinZ compiler (`minzc`) is written in Go and follows a traditional multi-pass architecture:

```
Source (.minz) → Parser → AST → Semantic Analysis → IR → Optimization → Code Generation → Assembly (.a80)
```

### 1. Parser

The parser supports two modes:
- **Tree-sitter integration** for robust parsing with good error recovery
- **Fallback recursive descent parser** for simpler environments

Key files:
- `pkg/parser/parser.go` - Main parser interface
- `pkg/parser/simple_parser.go` - Fallback parser implementation
- `tree-sitter-minz/` - Tree-sitter grammar (external)

### 2. Abstract Syntax Tree (AST)

The AST closely mirrors the source structure with nodes for:
- Declarations (functions, structs, enums, variables)
- Statements (if, while, return, blocks)
- Expressions (binary ops, calls, literals)
- Types (primitives, pointers, arrays, structs)

Key file: `pkg/ast/ast.go`

### 3. Semantic Analysis

The semantic analyzer performs:
- **Type checking** and inference
- **Symbol resolution** with scoped symbol tables
- **Ownership analysis** for memory safety
- **Compile-time evaluation** of Lua metaprograms

Key files:
- `pkg/semantic/analyzer.go` - Main analyzer
- `pkg/semantic/symbols.go` - Symbol table implementation
- `pkg/meta/lua_evaluator.go` - Lua integration

### 4. Intermediate Representation (IR)

The IR is a low-level, register-based representation optimized for Z80 target:

```go
type Instruction struct {
    Op       Opcode
    Dest     Register      // Destination register
    Src1     Register      // Source register 1
    Src2     Register      // Source register 2  
    Imm      int64        // Immediate value
    Symbol   string       // Symbol reference
    Label    string       // Jump target
    Type     Type         // Type information
    
    // Self-modifying code support
    SMCLabel  string      // Label for SMC location
    SMCTarget string      // Target label for SMC stores
}
```

The IR includes Z80-specific optimizations:
- **Self-modifying code (SMC)** opcodes for runtime constants
- **Register usage tracking** for optimal allocation
- **Shadow register hints** for performance-critical code

Key files:
- `pkg/ir/ir.go` - IR definitions
- `pkg/ir/builder.go` - IR construction helpers

### 5. Optimization Passes

The optimizer implements several passes tailored for Z80:

#### Register Analysis Pass
Tracks which physical Z80 registers each function uses and modifies:
- Main registers: A, B, C, D, E, H, L
- Register pairs: BC, DE, HL, IX, IY, SP
- Shadow registers: AF', BC', DE', HL'

This enables **lean prologue/epilogue generation** where functions only save registers they actually modify.

#### Self-Modifying Code Pass
Identifies frequently accessed constants and parameters that can benefit from SMC:
- Constants loaded in hot loops
- Function parameters that don't change
- Jump table addresses

#### Other Passes
- **Constant folding** and propagation
- **Dead code elimination**
- **Peephole optimization** for Z80 patterns
- **Function inlining** for small functions

Key files:
- `pkg/optimizer/optimizer.go` - Pass manager
- `pkg/optimizer/register_analysis.go` - Register tracking
- `pkg/optimizer/smc_optimization.go` - SMC optimization

### 6. Code Generation

The code generator produces Z80 assembly with several innovations:

#### Lean Function Prologue/Epilogue

Traditional Z80 function prologue saves all registers:
```asm
func:
    PUSH AF
    PUSH BC
    PUSH DE  
    PUSH HL
    PUSH IX
    ; ... function body ...
    POP IX
    POP HL
    POP DE
    POP BC
    POP AF
    RET
```

MinZ generates minimal prologues based on actual usage:
```asm
simple_func:
    ; Function only uses A register - no saves needed!
    LD A, (HL)
    INC A
    RET

complex_func:
    PUSH HL      ; Only save registers we actually modify
    PUSH DE
    ; ... function body ...
    POP DE
    POP HL
    RET
```

#### Interrupt Handler Optimization

Interrupt handlers use shadow registers for ultra-fast save/restore:
```asm
interrupt_handler:
    EX AF, AF'    ; Save AF to shadow (4 T-states)
    EXX           ; Save BC,DE,HL to shadows (4 T-states)
    ; ... handler body ...
    EXX           ; Restore BC,DE,HL
    EX AF, AF'    ; Restore AF
    EI
    RETI
```

This is much faster than traditional PUSH/POP sequences (8 vs 50+ T-states).

#### Register Allocator

The allocator (`pkg/codegen/register_allocator.go`) implements:
- **Linear scan allocation** with live interval analysis
- **Shadow register allocation** for high-pressure functions
- **Intelligent spilling** with reuse of spill slots
- **Special handling** for Z80 register pairs

Key files:
- `pkg/codegen/z80.go` - Main code generator
- `pkg/codegen/register_allocator.go` - Register allocation

## Current Implementation Status

### Completed Features

✅ **Core Language**
- Functions with parameters and return values
- Local variables and basic types (u8, u16, i8, i16, bool)
- Control flow (if, while, return)
- Expressions and operators
- Pointers and arrays
- Structs and enums

✅ **Optimization Infrastructure**
- Multi-pass optimizer framework
- Register usage analysis
- Self-modifying code generation
- Function prologue/epilogue optimization

✅ **Code Generation**
- Basic Z80 instruction selection
- Stack frame management
- Function calls with proper conventions
- Interrupt handler support

### In Progress

🚧 **Advanced Features**
- Full metaprogramming system (Lua integration has memory issues)
- Module system and imports
- Inline assembly support
- Memory management operators

🚧 **Optimizations**
- Advanced register allocation with graph coloring
- Instruction scheduling
- Loop optimizations
- Whole program optimization

### Future Work

📅 **Planned Features**
- Generics and traits
- Pattern matching
- Async/await for interrupt-driven code
- Package manager
- Debugger integration

## Example: Compiled Output

Here's how MinZ compiles a simple function:

**MinZ Source:**
```minz
fn add(a: u16, b: u16) -> u16 {
    return a + b;
}
```

**Generated Assembly:**
```asm
add:
    ; Lean prologue - only save what we use
    PUSH HL
    PUSH DE
    
    ; Load parameters
    LD L, (IX+4)
    LD H, (IX+5)
    LD E, (IX+6)
    LD D, (IX+7)
    
    ; Add
    ADD HL, DE
    
    ; Lean epilogue
    POP DE
    POP HL        ; Note: result in HL overwrites this
    RET
```

## Building the Compiler

```bash
cd minzc
go build -o minzc cmd/minzc/main.go
./minzc examples/hello.minz -o hello.a80
```

## Conclusion

MinZ represents a significant step forward in Z80 development tooling. By combining modern language design with deep understanding of Z80 architecture, it enables developers to write safe, efficient code without sacrificing the low-level control needed for systems programming.

The compiler's focus on register optimization and Z80-specific features like shadow registers and self-modifying code makes it particularly well-suited for performance-critical applications like game engines, operating systems, and embedded databases.

While still under development, MinZ already demonstrates that modern compiler techniques can bring significant improvements to 8-bit development workflows.