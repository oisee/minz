# MinZ SMC Examples

This directory contains all MinZ examples compiled with Self-Modifying Code (SMC) enabled by default.

## Summary
- **Total Examples**: 21
- **Successfully Compiled**: 10 (4 more fixed)
- **Failed**: 11

## Successfully Compiled Examples

### fibonacci
- **Features**: SMC functions, iterative algorithms
- **SMC Parameters**: 
  - `fib_param_n` at `fib + 1`

### game_sprite
- **Features**: SMC functions, game graphics, bit manipulation
- **SMC Parameters**:
  - `draw_sprite_param_x` at `draw_sprite + 1`
  - `draw_sprite_param_y` at `draw_sprite + 3`
  - `draw_sprite_param_sprite` at `draw_sprite + 5`

### simple_add
- **Features**: SMC functions, basic arithmetic
- **SMC Parameters**:
  - `add_param_a` at `add + 1`
  - `add_param_b` at `add + 4`

### tail_recursive
- **Features**: SMC functions, tail recursion examples
- **SMC Parameters**: Multiple functions with SMC parameters

### tail_sum
- **Features**: SMC functions, tail recursion optimization
- **SMC Parameters**:
  - `sum_tail_param_n` at `sum_tail + 1`
  - `sum_tail_param_acc` at `sum_tail + 4`
- **Optimization**: Tail calls converted to jumps with -O flag

### test_simple_vars
- **Features**: SMC functions, variable declarations
- **SMC Parameters**: None (no parameterized functions)

### main (fixed)
- **Features**: SMC functions, simple game loop
- **SMC Parameters**: None (no parameterized functions)

### screen_color (fixed)
- **Features**: SMC functions, screen attribute manipulation
- **SMC Parameters**: None (no parameterized functions)

### test_var_decls (fixed)
- **Features**: SMC functions, variable type casting
- **SMC Parameters**: None (no parameterized functions)

### smc_optimization_simple (fixed)
- **Features**: SMC functions, inline assembly examples
- **SMC Parameters**: Various demonstration parameters

## Failed to Compile

The following examples require features not yet implemented:

### enums
- Missing: Enum type support


### register_test
- Missing: Hardware register constants


### shadow_registers
- Missing: Register intrinsics

### smc_optimization
- Missing: `@smc_hint` pragma support

### structs
- Missing: Struct type support

### test_registers
- Missing: Register intrinsics


### zvdb_* (6 examples)
- Missing: Module system, structs, constants, metaprogramming

## Features Demonstrated

- **SMC Functions**: All functions use Self-Modifying Code by default
- **Tail Recursion**: Optimized tail recursive calls become jumps
- **Peephole Optimization**: LD A,0 becomes XOR A for efficiency
- **Parameter Embedding**: Function parameters embedded in instruction stream
- **Recursive Context Management**: SMC functions save/restore parameter values for recursive calls
- **Absolute Addressing Optimization**: Non-recursive functions use fast absolute addressing for locals (see OPTIMIZATION_NOTES.md)

## File Structure

Each successfully compiled example contains:
- `.minz` - Original MinZ source code
- `.mir` - MinZ Intermediate Representation
- `.a80` - Generated Z80 assembly with SMC
- `README.md` - Example-specific documentation