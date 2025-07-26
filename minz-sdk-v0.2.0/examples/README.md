# MinZ Examples

This directory contains MinZ example programs demonstrating various language features.

## Directory Structure

- **Source files** (`.minz`) - MinZ source code in the root directory
- **output/** - Compiled output files
  - `.a80` - Z80 assembly code (with SMC optimizations)
  - `.mir` - MinZ Intermediate Representation

## Working Examples

These examples compile successfully with the current compiler:

1. **fibonacci.minz** - Iterative Fibonacci sequence calculator
2. **game_sprite.minz** - Sprite rendering for games
3. **main.minz** - Simple game loop example
4. **screen_color.minz** - ZX Spectrum screen attribute manipulation
5. **simple_add.minz** - Basic addition function demonstrating SMC
6. **smc_optimization_simple.minz** - SMC optimization examples
7. **tail_recursive.minz** - Tail recursion examples
8. **tail_sum.minz** - Tail recursive sum with optimization
9. **test_simple_vars.minz** - Variable declaration tests
10. **test_var_decls.minz** - Type casting and declarations

## Examples Requiring Unimplemented Features

These examples need compiler features not yet implemented:

- **enums.minz** - Requires enum support
- **structs.minz** - Requires struct support
- **register_test.minz**, **test_registers.minz** - Cause compiler crashes
- **shadow_registers.minz** - Requires register intrinsics
- **lua_*.minz**, **metaprogramming.minz** - Require Lua metaprogramming
- **zvdb_*.minz** - Require module system and advanced features
- **modules/** - Requires module import system

## Compilation

To compile an example:
```bash
../minzc/main example.minz -o output/example.a80
```

With optimizations:
```bash
../minzc/main example.minz -o output/example.a80 -O
```

## Key Features Demonstrated

- **Self-Modifying Code (SMC)** - All functions use SMC by default
- **Absolute addressing** - No IX register usage
- **Tail recursion optimization** - Tail calls become jumps
- **Peephole optimizations** - Efficient code generation

All compiled output uses the SMC-first approach with parameters embedded in the instruction stream.