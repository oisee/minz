# Basic Syntax Examples

This section covers fundamental MinZ language constructs and syntax patterns.

## Examples in This Category

### Core Language Constructs

**`simple_add.minz`** ⭐ Perfect Introduction
- Basic function definition syntax
- Parameter passing and return values
- Simple arithmetic operations
- Function calls and variable declarations

**`working_demo.minz`** ⭐ Variable Fundamentals  
- Variable declarations with explicit types
- Let bindings and mutability
- Basic operations and expressions
- Main function structure

**`const_only.minz`** - Constant Declarations
- Constant definitions and usage
- Compile-time value resolution
- Scope and visibility rules

**`types_demo.minz`** - Type System Overview
- All MinZ primitive types (u8, u16, i8, i16, bool)
- Type inference and explicit typing
- Type casting and conversion
- Memory layout considerations

### Basic Operations

**`arithmetic_16bit.minz`** - 16-bit Arithmetic
- Large number calculations
- Overflow handling
- 16-bit operations on Z80
- Performance considerations for wide types

**`test_cast.minz`** - Type Casting
- Safe type conversions
- Narrowing and widening casts
- Runtime vs compile-time casting
- Z80 register optimization for casts

### Assignment and Variables

**`test_assignment.minz`** - Assignment Patterns
- Simple variable assignment
- Multiple assignment patterns
- Initialization vs assignment
- Memory layout optimization

**`test_simple_assign.minz`** - Assignment Fundamentals
- Basic assignment syntax
- Variable lifetime and scope
- Register allocation for assignments

## Learning Path

### 1. Start Here: `simple_add.minz`
Learn the fundamental structure of MinZ programs:
```minz
fun add(a: u16, b: u16) -> u16 {
    return a + b;
}

fun main() -> void {
    let x = add(10, 20);
    return;
}
```

### 2. Variables and Types: `working_demo.minz`
Understand variable declarations and basic operations:
```minz
fun main() -> void {
    let x: u16 = 1000;
    let y: u8 = 255;
    let result = x + (y as u16);
}
```

### 3. Type System: `types_demo.minz`
Explore MinZ's type system:
```minz
fun main() -> void {
    let small: u8 = 255;      // 8-bit unsigned
    let medium: u16 = 65535;  // 16-bit unsigned  
    let signed: i8 = -128;    // 8-bit signed
    let flag: bool = true;    // Boolean type
}
```

## Key Language Features Demonstrated

### Function Syntax
- Function declarations with `fun` keyword
- Parameter lists with explicit types
- Return type annotations
- Body blocks with explicit returns

### Variable Declarations
- `let` bindings for immutable variables
- `let mut` for mutable variables
- Type inference vs explicit typing
- Scope and lifetime rules

### Type System
- Primitive types: `u8`, `u16`, `i8`, `i16`, `bool`
- Type casting with `as` keyword
- Implicit type promotion rules
- Z80-optimized type layouts

### Basic Operations
- Arithmetic: `+`, `-`, `*`, `/`, `%`
- Comparison: `==`, `!=`, `<`, `>`, `<=`, `>=`
- Logical: `&&`, `||`, `!`
- Bitwise: `&`, `|`, `^`, `<<`, `>>`

## Compilation Results

All examples in this category compile successfully with:
- ✅ **Unoptimized compilation** - Full success rate
- ✅ **Standard optimization** (-O flag) - Consistent improvements  
- ✅ **SMC optimization** (--enable-smc) - Self-modifying code benefits

### Performance Insights

**Size Optimizations:**
- Simple arithmetic functions show consistent 2-5% size reductions
- Type casting operations benefit from register allocation optimization
- Constant folding provides significant improvements in const-heavy code

**SMC Benefits:**
- Function calls with constant parameters get TRUE SMC optimization
- Parameter passing overhead eliminated in simple cases
- Register allocation improvements in optimized builds

## Best Practices Demonstrated

### 1. Clear Function Signatures
Always specify parameter and return types explicitly for documentation and optimization.

### 2. Appropriate Type Choice
Use smallest types that fit your data to optimize Z80 register usage.

### 3. Explicit Returns
Always include explicit `return` statements for clarity and optimization.

### 4. Meaningful Names
Use descriptive variable and function names that explain intent.

## Next Steps

After mastering basic syntax:
1. Move to **02_functions** for advanced function patterns
2. Explore **03_control_flow** for conditional logic and loops
3. Study optimization output to understand compiler behavior

## Compilation Command Examples

```bash
# Basic compilation
./minzc simple_add.minz -o simple_add.a80

# With optimization
./minzc simple_add.minz -O -o simple_add_opt.a80

# With TRUE SMC optimization  
./minzc simple_add.minz -O --enable-smc -o simple_add_smc.a80

# With debug output (generates .mir file)
./minzc simple_add.minz -d -o simple_add_debug.a80
```

These examples provide the foundation for understanding MinZ syntax and serve as building blocks for more complex programs.