# MinZ Metaprogramming Complete Design

## Overview

MinZ provides a powerful two-stage metaprogramming system:
1. **@define** - Template expansion (text substitution)
2. **@lang[[[...]]]** - Compile-time code execution

**Happy Syntax Rule**: All metafunctions take a SINGLE string parameter with embedded expressions in `{ }`:
```minz
@print("Value: { 2 + 3 }");      // ✅ The ONLY way
@print("Value: {}", 2 + 3);      // ❌ NOT supported
```

## Processing Pipeline

```
Source Code
    ↓
@define Expansion (recursive until no more @defines)
    ↓
@lang[[[]]] Execution (capture output as MinZ code)
    ↓
Normal Compilation (AST → MIR → Assembly)
```

## @define - Template System

### Syntax
```minz
@define(param1, param2, ...)[[[
    template body with {0}, {1}, {2}... substitutions
]]]
```

### Features
- Simple parameter substitution using {0}, {1}, {2}...
- No nested @define blocks allowed (keeps parsing simple)
- Can generate any MinZ code including functions, structs, etc.
- Processed first in the pipeline

### Examples

```minz
// Generate entity types
@define(entity_type, health, damage)[[[
    struct {0} {
        health: u8 = {1}
        damage: u8 = {2}
    }
    
    fun spawn_{0}() -> {0} {
        return {0} { health: {1}, damage: {2} };
    }
    
    fun damage_{0}(entity: *{0}, amount: u8) -> void {
        if entity.health > amount {
            entity.health -= amount;
        } else {
            entity.health = 0;
        }
    }
]]]

// Usage
@define("Enemy", 100, 25)
@define("Boss", 500, 50)
@define("Player", 200, 30)
```

## Compile-Time Code Execution

### @lua[[[ ]]] - Lua Execution

Execute Lua code at compile time. The output (via print) becomes MinZ code.

```minz
@lua[[[
    -- Generate constants
    for i = 1, 10 do
        print(string.format("const LEVEL_%d_SCORE: u16 = %d;", i, i * 1000))
    end
    
    -- Generate lookup table
    print("const SINE_TABLE: [u8; 256] = [")
    for i = 0, 255 do
        local val = math.floor(128 + 127 * math.sin(i * math.pi * 2 / 256))
        print(string.format("    %d,", val))
    end
    print("];")
]]]
```

### @minz[[[ ]]] - MinZ Compile-Time Execution

Execute MinZ code at compile time using the MIR interpreter.

```minz
@minz[[[
    // Generate helper functions
    for i in 0..8 {
        @emit("fun get_bit_{i}(value: u8) -> bool {")
        @emit("    return (value & {1 << i}) != 0;")
        @emit("}")
    }
    
    // Generate optimized switch table
    let powers: [u8; 8] = [1, 2, 4, 8, 16, 32, 64, 128];
    for i in 0..8 {
        @emit("const POWER_OF_2_{i}: u8 = {powers[i]};")
    }
]]]
```

### @mir[[[ ]]] - Direct MIR Generation

Generate MIR (Machine Independent Representation) directly for low-level optimization.

```minz
@mir[[[
    // Optimized memory copy routine
    r1 = load_param 0     // source
    r2 = load_param 1     // dest  
    r3 = load_param 2     // count
    
    loop_start:
    r4 = load_mem r1
    store_mem r2, r4
    inc r1
    inc r2
    dec r3
    branch_nz r3, loop_start
    
    ret
]]]
```

## Key Design Decisions

### Why No Nested @define?

Nested @define blocks would complicate parsing due to delimiter conflicts with `]]]`. Instead:
- Keep @define simple and single-level
- Use sequential @defines for multi-stage transformations
- Generate functions instead of nested templates

### Why [[[ ]]] Delimiters?

The triple-bracket syntax:
- Is visually distinctive and easy to spot
- Rarely conflicts with normal code
- Works consistently across all languages
- Makes block boundaries clear

### Parameter Syntax

Parameters go **before** the block:
- `@define(p1, p2)[[[...]]]` - Template with substitution
- `@minz[[[...]]]` - Direct execution (no parameters)

This clearly distinguishes templates from direct execution.

## Processing Order

1. **@define expansion**
   - All @define blocks are expanded
   - Simple text substitution with {0}, {1}, etc.
   - Results can be any valid MinZ code

2. **@lang execution**
   - @lua, @minz, @mir blocks are executed
   - Output is captured and inserted as MinZ code
   - Can generate more MinZ code dynamically

3. **Normal compilation**
   - The resulting MinZ code is compiled normally
   - All metaprogramming is resolved by this point

## Advanced Examples

### Compile-Time Computation

```minz
// Generate optimized jump table
@minz[[[
    // Compute jump offsets at compile time
    let handlers: [string; 4] = ["handle_up", "handle_down", "handle_left", "handle_right"];
    
    @emit("const JUMP_TABLE: [u16; 4] = [")
    for i in 0..4 {
        // In real implementation, would compute actual addresses
        @emit("    &{handlers[i]},")
    }
    @emit("];")
]]]
```

### Hardware-Specific Optimization

```minz
// Generate Z80-optimized bit manipulation
@define(bit_op, name)[[[
    fun {1}_bit(value: u8, bit: u8) -> u8 {
        @mir[[[
            r1 = load_param 0  // value
            r2 = load_param 1  // bit
            
            // Generate optimized Z80 bit instruction
            {0}_bit r1, r2     // SET/RES/BIT
            
            ret r1
        ]]]
    }
]]]

@define("set", "set")
@define("res", "clear")
@define("bit", "test")
```

### Metaprogramming with Types

```minz
// Generate type-safe vector operations
@define(vec_type, size)[[[
    struct Vec{1} {
        data: [f16.8; {1}]
    }
    
    fun vec{1}_add(a: Vec{1}, b: Vec{1}) -> Vec{1} {
        let result: Vec{1};
        for i in 0..{1} {
            result.data[i] = a.data[i] + b.data[i];
        }
        return result;
    }
    
    fun vec{1}_dot(a: Vec{1}, b: Vec{1}) -> f16.8 {
        let sum: f16.8 = 0.0;
        for i in 0..{1} {
            sum += a.data[i] * b.data[i];
        }
        return sum;
    }
]]]

// Generate 2D, 3D, 4D vector types
@define("Vec", 2)
@define("Vec", 3)  
@define("Vec", 4)
```

## Future Extensions

The `@lang[[[...]]]` pattern is extensible to any language:

```minz
// Future: JavaScript for web assembly target
@js[[[
    console.log("Generating WASM bindings...");
    // Generate MinZ code for WASM
]]]

// Future: Python for complex compile-time analysis
@python[[[
    import numpy as np
    # Generate optimized matrix operations
]]]

// Future: Even BASIC for retro meta-humor!
@basic[[[
    10 PRINT "GENERATING CODE..."
    20 FOR I = 1 TO 10
    30 PRINT "const VALUE_" + STR$(I) + ": u8 = " + STR$(I) + ";"
    40 NEXT I
]]]
```

## Summary

MinZ metaprogramming provides:
- **@define** for simple, powerful templates
- **@lua** for complex compile-time logic
- **@minz** for self-hosted metaprogramming
- **@mir** for low-level optimization
- Clear, extensible syntax with `[[[...]]]` blocks
- Two-stage processing: templates then execution

This design balances power with simplicity, enabling advanced compile-time programming while keeping the syntax clear and the implementation tractable.