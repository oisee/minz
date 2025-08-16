# MinZ Metafunction Design Decisions

## Core Philosophy
MinZ metafunctions provide compile-time code generation and transformation capabilities. Each metafunction has a specific purpose and execution model.

## Metafunction Types

### 1. @define - Preprocessor Template Substitution
**Execution:** First phase, before any parsing
**Syntax:** `@define("template_with_{0}_placeholders", arg1, arg2, ...)`
**Purpose:** Simple text-based template substitution
**Example:**
```minz
@define("fun add_{0}_{1}(a: {0}, b: {1}) -> {0} { return a + b; }", "u8", "u16")
// Generates: fun add_u8_u16(a: u8, b: u16) -> u8 { return a + b; }
```
**Key Points:**
- Pure text substitution with {0}, {1}, etc. placeholders
- Happens BEFORE parsing - it's a preprocessor
- No language awareness, just string replacement
- Processed before @minz or any other metafunctions

### 2. @minz[[[ ]]] - Immediate Compile-Time Execution
**Execution:** During semantic analysis, after parsing
**Syntax:** `@minz[[[...code...]]]` (NO ARGUMENTS)
**Purpose:** Execute MinZ code at compile-time to generate declarations
**Example:**
```minz
@minz[[[
    @emit("fun hello_world() -> void {")
    @emit("    @print(\"Hello, World!\");")
    @emit("}")
]]]
```
**Key Points:**
- Executes MinZ code at compile-time
- Uses @emit to generate code
- Can use loops, conditionals, variables
- Generated code is parsed and analyzed
- NO ARGUMENTS - it's immediate execution, not a template

### 3. @lua[[[ ]]] - Lua Compile-Time Execution
**Execution:** During semantic analysis
**Syntax:** `@lua[[[...lua code...]]]`
**Purpose:** Execute Lua code at compile-time for complex metaprogramming
**Example:**
```minz
@lua[[[
    for i = 1, 10 do
        emit(string.format("const VALUE_%d: u8 = %d;", i, i * 2))
    end
]]]
```
**Key Points:**
- Full Lua scripting at compile-time
- More powerful than @minz for complex generation
- Has access to emit() function

### 4. @print - Compile-Time String Output
**Execution:** During semantic analysis
**Syntax:** `@print("message")`
**Purpose:** Optimized print function calls
**Key Points:**
- Generates optimized assembly for string printing
- NOT for compile-time debug output (that's println!() in the compiler)

### 5. @if / @elif / @else - Compile-Time Conditionals
**Execution:** During semantic analysis
**Syntax:** Standard if/elif/else with @ prefix
**Purpose:** Conditional compilation based on compile-time values
**Example:**
```minz
@if(TARGET_SPECTRUM) {
    const SCREEN_WIDTH: u16 = 256;
} @else {
    const SCREEN_WIDTH: u16 = 320;
}
```

### 6. @error - Compile-Time Error
**Execution:** During semantic analysis
**Syntax:** `@error("message")`
**Purpose:** Emit compile-time errors
**Example:**
```minz
@if(!FEATURE_ENABLED) {
    @error("This feature requires FEATURE_ENABLED flag");
}
```

## Execution Order

1. **Preprocessing Phase**
   - @define macros are expanded (simple text substitution)

2. **Parsing Phase**
   - Source is parsed into AST (with expanded @define results)

3. **Semantic Analysis Phase 1**
   - @minz[[[ ]]] blocks execute and generate code
   - @lua[[[ ]]] blocks execute and generate code
   
4. **Semantic Analysis Phase 2**
   - Generated code is analyzed
   - @if/@elif/@else conditionals are evaluated
   - @print calls are optimized
   - @error directives are processed

## Important Design Decisions

### Why @minz Has No Arguments
- @minz is for immediate execution, not template instantiation
- If you need templates with arguments, use @define (preprocessor)
- If you need complex generation with arguments, use @lua with variables
- This keeps each metafunction focused on one job

### Why @define is a Preprocessor
- Simple, predictable text substitution
- No need for AST manipulation
- Works with any syntax (even invalid MinZ temporarily)
- Fast and efficient
- Language-agnostic (could generate assembly, comments, etc.)

### The @emit Pattern
- @emit is only available inside @minz[[[ ]]] blocks
- It accumulates generated code line by line
- The accumulated code is parsed after the block completes
- This allows for clean, readable code generation

## Common Patterns

### Generating Multiple Functions
```minz
@minz[[[
    for i in 0..4 {
        @emit("fun handler_" + i + "() -> void {")
        @emit("    @print(\"Handler " + i + " called\");")
        @emit("}")
    }
]]]
```

### Template-Based Generation
```minz
// Use @define for simple templates
@define("MAKE_GETTER(fun get_{0}() -> {1} { return self.{0}; })", "x", "u8")
@define("MAKE_GETTER(fun get_{0}() -> {1} { return self.{0}; })", "y", "u8")
```

### Complex Metaprogramming
```minz
@lua[[[
    -- Generate a jump table
    emit("const JUMP_TABLE: [u16; 256] = [")
    for i = 0, 255 do
        if i % 16 == 0 then emit("\n    ") end
        emit(string.format("0x%04X, ", i * 3))
    end
    emit("\n];")
]]]
```

## Future Considerations

- @define could support nested templates
- @minz could support importing compile-time modules
- @lua could expose more compiler internals
- New metafunctions could be added for specific tasks

## Summary

Each metafunction has a clear, focused purpose:
- **@define** - Preprocessor templates (text substitution)
- **@minz** - MinZ compile-time execution (no args)
- **@lua** - Lua compile-time execution
- **@print** - Optimized printing
- **@if/@elif/@else** - Conditional compilation
- **@error** - Compile-time errors

This design ensures clarity, predictability, and power in MinZ's metaprogramming system.