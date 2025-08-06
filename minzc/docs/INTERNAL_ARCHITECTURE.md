# MinZ Compiler Internal Architecture

## Table of Contents
1. [Overview](#overview)
2. [Compilation Pipeline](#compilation-pipeline)
3. [Package Architecture](#package-architecture)
4. [Key Data Structures](#key-data-structures)
5. [Backend System](#backend-system)
6. [Optimization Framework](#optimization-framework)
7. [Build System](#build-system)
8. [Testing Infrastructure](#testing-infrastructure)
9. [Dead Code Analysis](#dead-code-analysis)

## Overview

The MinZ compiler is a multi-backend compiler for a modern systems programming language targeting 8-bit architectures. It's written in Go and uses tree-sitter for parsing.

### Key Design Principles
- **Multi-backend support**: 8 different backends (Z80, 6502, 68000, i8080, GB, C, LLVM, WebAssembly)
- **Optimization-focused**: Sophisticated optimization passes including SMC (self-modifying code)
- **Zero-cost abstractions**: Lambdas, iterators, and interfaces compile to optimal assembly
- **Metaprogramming**: Compile-time Lua execution and code generation

## Compilation Pipeline

```
Source (.minz) → Parser → AST → Semantic Analysis → IR → Optimizer → Code Generation → Output
```

### Detailed Flow

1. **Parsing** (`pkg/parser/`)
   - Uses tree-sitter C bindings via external process
   - Converts S-expressions to Go AST structures
   - Entry: `parser.go:ParseFile()`

2. **Semantic Analysis** (`pkg/semantic/`)
   - Type checking and inference
   - Symbol resolution and scope management
   - Template expansion (@define)
   - Error propagation analysis
   - Entry: `analyzer.go:Analyze()`

3. **IR Generation** (`pkg/ir/`)
   - Converts AST to Machine-Independent Representation (MIR)
   - SSA-like instruction format
   - Entry: Part of semantic analysis

4. **Optimization** (`pkg/optimizer/`)
   - Multiple optimization passes
   - Register allocation
   - Self-modifying code transformation
   - Entry: `optimizer.go:Optimize()`

5. **Code Generation** (`pkg/codegen/`)
   - Backend-specific code generation
   - Entry: `backend.go:Generate()`

## Package Architecture

### Core Packages

#### `pkg/ast` - Abstract Syntax Tree
- **Purpose**: Define AST node types
- **Key Types**: 
  - `Node`, `Expression`, `Statement` interfaces
  - `File`, `FunctionDecl`, `VarDecl` structs
- **No dependencies on other MinZ packages**

#### `pkg/parser` - Parser Integration
- **Purpose**: Tree-sitter integration
- **Key Functions**:
  - `ParseFile()` - Main parsing entry point
  - `convertNode()` - S-expression to AST conversion
- **Depends on**: `pkg/ast`

#### `pkg/semantic` - Semantic Analysis
- **Purpose**: Type checking, symbol resolution, IR generation
- **Key Components**:
  - `Analyzer` - Main analysis driver
  - `Scope` - Symbol table management
  - `ErrorPropagationContext` - Error handling analysis
- **Depends on**: `pkg/ast`, `pkg/ir`, `pkg/meta`

#### `pkg/ir` - Intermediate Representation
- **Purpose**: Define MIR instructions and types
- **Key Types**:
  - `Instruction` - MIR instruction
  - `Function` - Function representation
  - `Module` - Complete program
  - `Type` interface - Type system
- **No dependencies on other MinZ packages**

#### `pkg/optimizer` - Optimization Passes
- **Purpose**: Transform and optimize IR
- **Key Passes**:
  - `PeepholeOptimizationPass` - Pattern-based optimizations
  - `MIRReorderingPass` - Instruction scheduling
  - `RegisterAllocationPass` - Register assignment
  - `TrueSMCOptimizationPass` - Self-modifying code
- **Depends on**: `pkg/ir`

#### `pkg/codegen` - Code Generation
- **Purpose**: Generate target-specific code
- **Structure**:
  - `Backend` interface - Common backend interface
  - `backend_toolkit.go` - Shared utilities
  - Per-backend implementations (`z80_backend.go`, etc.)
- **Depends on**: `pkg/ir`

### Supporting Packages

#### `pkg/meta` - Metaprogramming
- Lua interpreter integration for compile-time execution

#### `pkg/interpreter` - MIR Interpreter
- Interprets MIR for @minz metaprogramming

#### `pkg/z80asm` - Built-in Assembler
- Z80 assembler for the full toolchain

#### `pkg/emulator` - Z80 Emulator
- For testing generated code

## Key Data Structures

### AST Nodes (`pkg/ast/ast.go`)
```go
type Node interface {
    Pos() Position
    End() Position
}

type Expression interface {
    Node
    exprNode()
}

type Statement interface {
    Node
    stmtNode()
}
```

### IR Instructions (`pkg/ir/ir.go`)
```go
type Instruction struct {
    Op           Opcode
    Dest         Register
    Src1         Register
    Src2         Register
    Imm          int64
    Symbol       string
    Type         Type
    // ... more fields
}
```

### Type System (`pkg/ir/ir.go`)
```go
type Type interface {
    Size() int
    String() string
}
```

## Backend System

### Backend Interface (`pkg/codegen/backend.go`)
```go
type Backend interface {
    Name() string
    Generate(module *ir.Module) (string, error)
}
```

### Available Backends
1. **Z80** (`z80_backend.go`) - Default, production-ready
2. **6502** (`m6502_backend.go`) - NES, C64, Apple II
3. **68000** (`m68k_backend.go`) - Amiga, Atari ST
4. **i8080** (`i8080_backend.go`) - Intel 8080
5. **Game Boy** (`gb_backend.go`) - Modified Z80
6. **C** (`c_backend.go`) - C code generation
7. **LLVM** (`llvm_backend.go`) - LLVM IR
8. **WebAssembly** (`wasm_backend.go`) - WAT format

### Backend Registration
Backends register themselves in `init()` functions using `RegisterBackend()`.

## Optimization Framework

### Pass Structure
Each optimization pass implements:
```go
type Pass interface {
    Name() string
    Run(module *ir.Module) error
}
```

### Key Optimizations
1. **Peephole Optimization** - Pattern matching on instruction sequences
2. **Register Allocation** - Hierarchical allocation (physical → shadow → memory)
3. **Instruction Reordering** - Minimize register pressure
4. **Self-Modifying Code** - Parameter patching for performance
5. **Tail Call Optimization** - Convert to jumps
6. **Dead Code Elimination** - Remove unreachable code

### Optimization Pipeline (`pkg/optimizer/optimizer.go`)
```go
passes := []Pass{
    NewRegisterAnalysisPass(),
    NewMIRReorderingPass(),
    NewPeepholeOptimizationPass(),
    NewConstantFoldingPass(),
    NewDeadCodeEliminationPass(),
    // ... more passes
}
```

## Build System

### Makefile Targets
- `make build` - Build compiler
- `make test` - Run tests
- `make run` - Compile and run example
- `make benchmark` - Run performance benchmarks

### Key Scripts
- `compile_all_examples.sh` - Test all examples
- `test_backend_e2e.sh` - End-to-end backend testing
- `build_release_v*.sh` - Release building
- `comprehensive_backend_test.sh` - Full backend validation

### Dependencies
- Go 1.19+
- tree-sitter CLI
- Optional: sjasmplus (for Z80 assembly)

## Testing Infrastructure

### Test Categories
1. **Unit Tests** - Go test files (`*_test.go`)
2. **Integration Tests** - `tests/integration/*.minz`
3. **E2E Tests** - `tests/backend_e2e/`
4. **Example Programs** - `examples/*.minz`

### Test Execution
- `go test ./...` - All unit tests
- `./test_all_examples.sh` - Compile all examples
- `./test_backend_e2e.sh` - Backend E2E testing

## Dead Code Analysis

### Potentially Unused Components
Based on static analysis, these components may be dead code:

1. **Test/Analysis Scripts** (not part of main flow):
   - Various one-off analysis scripts in root directory
   - Old benchmark/test runners

2. **Experimental Features**:
   - Some optimizer passes may be disabled
   - Prototype backends

### Active Core Components
All packages under `pkg/` are actively used in the compilation pipeline:
- ✅ `ast`, `parser`, `semantic`, `ir`, `optimizer`, `codegen` - Core pipeline
- ✅ `meta`, `interpreter` - Metaprogramming support
- ✅ `z80asm`, `emulator` - Toolchain support

## Making Changes

### Adding a New Optimization Pass
1. Create new file in `pkg/optimizer/`
2. Implement the `Pass` interface
3. Add to pipeline in `optimizer.go`

### Adding a New Backend
1. Create `pkg/codegen/XXX_backend.go`
2. Implement `Backend` interface
3. Register in `init()` function
4. Add tests in `tests/backend_e2e/`

### Modifying the AST
1. Update `pkg/ast/ast.go`
2. Update parser in `pkg/parser/parser.go`
3. Update semantic analysis in `pkg/semantic/`

### Updating IR
1. Modify `pkg/ir/ir.go`
2. Update all backends to handle new instructions
3. Update optimizer passes as needed

## Future Improvements

### Identified Issues
1. **LLVM Backend** - IR syntax errors need fixing
2. **WebAssembly** - Missing global declarations
3. **Array Operations** - LOAD_INDEX not implemented for some backends
4. **Z80 Assembly** - Duplicate label issues with sjasmplus

### Optimization Opportunities
1. Better instruction selection for complex expressions
2. Inter-procedural optimization
3. Profile-guided optimization
4. Better register allocation for specific architectures