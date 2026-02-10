# MinZ Metafunction Syntax Redesign
## Unifying Compile-Time Code Execution

*Date: August 4, 2025*

## Problem Statement

Current MinZ has inconsistent syntax for compile-time operations:
- `@lua[[[...]]]` - Lua code blocks executed at compile time
- `@minz("template", args)` - String template substitution
- `@print`, `@if`, etc. - Individual metafunctions

This creates confusion about what runs at compile-time vs runtime.

## Proposed Solution

### Core Principle: `@` Prefix = Compile-Time

Any function or block starting with `@` executes at compile time:

```minz
// All these run at COMPILE TIME:
@print("Compiling...")           // Compile-time print
@if(DEBUG, 10, 20)               // Compile-time conditional
@lua[[[print("Lua at compile")]]] // Lua code at compile time
@minz[[[                         // MinZ code at compile time
    let x = 10;
    @print("MinZ at compile: {}", x);
]]]
```

### Three Forms of @minz

#### 1. Simple Block: `@minz[[[...]]]`
Execute MinZ code at compile time, just like `@lua[[[...]]]`:

```minz
// Generate constants at compile time
@minz[[[
    let table_size = 256;
    for i in 0..table_size {
        @emit("const ENTRY_{} = {};", i, i * 2);
    }
]]]
```

#### 2. Template with Parameters: `@minz[[[...]]](params)`
Template-based code generation with substitution:

```minz
// Define a template
@minz[[[
    fun {0}_handler(event: {1}) -> void {
        @print("Handling {0} event");
        process_{0}(event);
    }
]]]("mouse", "MouseEvent")

// Generates:
// fun mouse_handler(event: MouseEvent) -> void {
//     @print("Handling mouse event");
//     process_mouse(event);
// }
```

#### 3. Named Metafunctions: `@function_name`
Any function starting with `@` is a metafunction:

```minz
// Define compile-time function
@fun generate_table(size: u8) -> void {
    for i in 0..size {
        @emit("const ENTRY_{} = {};", i, i * i);
    }
}

// Use it
@generate_table(16);  // Runs at compile time
```

## Implementation Plan

### Phase 1: Grammar Updates

```javascript
// grammar.js additions
minz_block: $ => seq(
    '@minz',
    '[[[',
    $.minz_code_block,
    ']]]',
    optional(seq(
        '(',
        commaSep($.expression),
        ')'
    ))
),

// Metafunction declaration
metafunction_declaration: $ => seq(
    '@fun',
    $.identifier,
    $.parameter_list,
    optional(seq('->', $.type)),
    $.block
),
```

### Phase 2: AST Nodes

```go
// ast.go
type MinzBlock struct {
    Code       string
    Parameters []Expression  // Optional for templates
    StartPos   Position
    EndPos     Position
}

type MetafunctionDecl struct {
    Name       string
    Parameters []Parameter
    ReturnType Type
    Body       Block
    StartPos   Position
    EndPos     Position
}
```

### Phase 3: Semantic Analysis

```go
func (a *Analyzer) analyzeMinzBlock(block *ast.MinzBlock) error {
    if len(block.Parameters) > 0 {
        // Template mode: substitute parameters
        code := a.substituteTemplate(block.Code, block.Parameters)
        return a.executeMinzCode(code)
    } else {
        // Direct execution mode
        return a.executeMinzCode(block.Code)
    }
}
```

## Migration Path

### Current Code (v0.9.4)
```minz
// Old style - string templates
@minz("fun add_{0}() -> u8 { return {1}; }", "numbers", "42")

// Lua blocks
@lua[[[
    for i = 1, 10 do
        emit("const VAL_" .. i .. " = " .. (i * 2))
    end
]]]
```

### New Style (v1.0)
```minz
// MinZ template with parameters
@minz[[[
    fun add_{0}() -> u8 { return {1}; }
]]]("numbers", "42")

// Pure MinZ compile-time code
@minz[[[
    for i in 1..10 {
        @emit("const VAL_{} = {}", i, i * 2);
    }
]]]

// Named metafunction
@fun generate_constants(count: u8) -> void {
    for i in 0..count {
        @emit("const VAL_{} = {}", i, i * 2);
    }
}

@generate_constants(10);  // Call at compile time
```

## Benefits

### 1. **Consistency**
- `@` always means compile-time
- `[[[...]]]` always means code block
- `(params)` always means parameters

### 2. **Clarity**
```minz
// Crystal clear what runs when:
let x = 10;              // Runtime variable
@let y = 10;             // Compile-time variable
fun foo() { }            // Runtime function
@fun bar() { }           // Compile-time function
```

### 3. **Power**
- Write metafunctions in MinZ itself
- No need to learn Lua for metaprogramming
- Gradual transition from Lua to MinZ

### 4. **Compatibility**
- Keep `@lua[[[...]]]` for complex metaprogramming
- Old `@minz("template", args)` can coexist during transition
- Clear deprecation path

## Examples

### Generate Lookup Table
```minz
@minz[[[
    // Generate sine table at compile time
    const TABLE_SIZE = 256;
    for i in 0..TABLE_SIZE {
        let angle = (i * 360) / TABLE_SIZE;
        let value = @sin(angle) * 127;  // Compile-time sin
        @emit("const SINE_{} = {};", i, value);
    }
]]]
```

### Conditional Compilation
```minz
@minz[[[
    @if(PLATFORM == "ZX_SPECTRUM") {
        @emit("const SCREEN_WIDTH = 256;");
        @emit("const SCREEN_HEIGHT = 192;");
    } else {
        @emit("const SCREEN_WIDTH = 320;");
        @emit("const SCREEN_HEIGHT = 200;");
    }
]]]
```

### Generic-like Templates
```minz
// Define template
@minz[[[
    struct Array_{0} {
        data: [{0}; {1}],
        size: u8
    }
    
    impl Array_{0} {
        fun new() -> Array_{0} {
            return Array_{0} { data: [0; {1}], size: 0 };
        }
        
        fun push(self, value: {0}) -> void {
            self.data[self.size] = value;
            self.size = self.size + 1;
        }
    }
]]]

// Instantiate for different types
@minz_template("u8", "32");   // Array_u8 with 32 elements
@minz_template("u16", "16");  // Array_u16 with 16 elements
```

## Implementation Priority

1. **Week 1**: Grammar and parser support for `@minz[[[...]]]`
2. **Week 2**: Basic execution without parameters
3. **Week 3**: Template parameter substitution
4. **Week 4**: Named metafunctions (`@fun`)
5. **Week 5**: Full integration and testing

## Success Metrics

- ✅ All `@lua[[[...]]]` code can be rewritten in `@minz[[[...]]]`
- ✅ Templates are more readable than string concatenation
- ✅ Compile-time vs runtime is immediately obvious
- ✅ No performance regression
- ✅ Clear migration path for existing code

## Conclusion

This redesign creates a consistent, powerful metaprogramming system where:
- **`@` = compile-time** (always)
- **`[[[...]]]` = code block** (Lua or MinZ)
- **`(...)` = parameters** (for templates or functions)

This gives MinZ a clean, orthogonal design that's easy to understand and powerful to use.