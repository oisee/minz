# MinZ Design Notes

## Alternative Register Set Support

The Z80 has a shadow register set that can be swapped with the main registers:
- `EXX` - Swaps BC, DE, HL with BC', DE', HL'
- `EX AF,AF'` - Swaps AF with AF'

### Use Cases
1. **Fast interrupt handlers** - Save context without stack operations
2. **Coroutines** - Quick context switching between execution contexts
3. **Performance optimization** - Use shadow registers as extra storage

### Proposed MinZ Syntax

```minz
// Inline assembly for direct control
fn interrupt_handler() -> void {
    asm("ex af, af'");  // Save AF
    asm("exx");         // Save BC, DE, HL
    
    // Handler code here
    
    asm("exx");         // Restore BC, DE, HL
    asm("ex af, af'");  // Restore AF
}

// High-level support via attributes
@shadow_registers
fn fast_memcpy(dst: *mut u8, src: *u8, len: u16) -> void {
    // Compiler uses shadow registers for loop optimization
}

// Explicit shadow register access
fn compute() -> u16 {
    let result: u16;
    shadow {
        // Code here runs with shadow registers active
        // Compiler automatically generates EXX/EX AF,AF'
        let temp = calculate_something();
        result = temp * 2;
    }
    return result;
}
```

### Implementation Plan
1. Extend IR with shadow register operations
2. Add register allocator support for shadow registers
3. Implement syntax extensions
4. Add optimization pass to use shadow registers

## Metaprogramming and Compile-Time Evaluation

### Current Grammar Support
The grammar already defines:
- `@if(condition, true_expr, false_expr)` - Compile-time conditional
- `@print("message")` - Compile-time output
- `@assert(condition, "message")` - Compile-time assertion
- `@attribute` - General attributes

### Proposed Extensions

#### 1. Code Generation
```minz
// Generate MinZ code at compile time
@generate
fn make_accessors(type_name: str, fields: [(str, type)]) -> str {
    let code = "";
    for (name, type) in fields {
        code += "fn get_" + name + "(&self) -> " + type + " { self." + name + " }\n";
        code += "fn set_" + name + "(&mut self, val: " + type + ") { self." + name + " = val; }\n";
    }
    return code;
}

// Usage
struct Point {
    x: i16,
    y: i16,
}

@eval make_accessors("Point", [("x", "i16"), ("y", "i16")])
```

#### 2. Compile-Time Computation
```minz
// Compute lookup tables at compile time
@const_eval
fn compute_sine_table() -> [i16; 256] {
    let table: [i16; 256];
    for i in 0..256 {
        table[i] = (sin(i * 2 * PI / 256) * 32767) as i16;
    }
    return table;
}

const SINE_TABLE: [i16; 256] = @eval compute_sine_table();
```

#### 3. Conditional Compilation
```minz
@if(TARGET_SPECTRUM)
const SCREEN_WIDTH: u16 = 256;
@else
const SCREEN_WIDTH: u16 = 320;
@endif

// Or inline
const MAX_SPRITES: u8 = @if(DEBUG, 16, 64);
```

### Implementation Plan
1. Add compile-time interpreter for MinZ subset
2. Implement AST transformation pipeline
3. Add source code generation from AST
4. Integrate with semantic analyzer

## Roadmap Implementation Priority

Based on compiler maturity and user needs:

### Phase 1: Core Language Features
1. **Struct support** (High priority)
   - Essential for organized code
   - Needed for many examples
2. **Enum types** (High priority)
   - State machines
   - Error handling

### Phase 2: Metaprogramming
3. **Compile-time evaluation**
   - Lookup table generation
   - Conditional compilation
4. **Code generation**
   - Macro-like features
   - Reduces boilerplate

### Phase 3: Tooling
5. **Module system**
   - Code organization
   - Separate compilation
6. **Standard library**
   - Common functions
   - Platform abstractions

### Phase 4: Developer Experience
7. **Optimization passes**
   - Peephole optimizations
   - Register allocation improvements
8. **VS Code extension**
   - Syntax highlighting
   - IntelliSense
9. **Debugger support**
   - Source-level debugging
   - Symbol information

### Phase 5: Ecosystem
10. **Package manager**
    - Dependency management
    - Library distribution