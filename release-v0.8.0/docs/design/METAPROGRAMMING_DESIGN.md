# MinZ Metaprogramming Design Considerations

## Current Limitations

The current metaprogramming implementation only supports:
- Basic arithmetic expressions
- Boolean operations
- Simple conditionals

It does NOT support:
- Loops
- Function calls
- Complex data structures
- String manipulation
- File I/O

## Options for Full Compile-Time Evaluation

### Option 1: Full MinZ Interpreter (Current Approach)
**Pros:**
- Consistent language - write metaprograms in MinZ
- No additional dependencies
- Type safety at compile time

**Cons:**
- Massive implementation effort
- Need to implement interpreter for entire language
- Slow compile times
- Difficult to maintain two implementations

### Option 2: Embed Lua
**Pros:**
- Mature, well-tested interpreter
- Designed for embedding (small ~200KB)
- Fast execution
- Good C interop for tree-sitter integration
- Used successfully in many build tools (e.g., Neovim, Redis)

**Cons:**
- Different syntax from MinZ
- Dynamic typing vs MinZ's static typing
- Need to marshal data between Lua and MinZ

### Option 3: Embed JavaScript (V8/QuickJS)
**Pros:**
- Familiar syntax for many developers
- Rich ecosystem
- QuickJS is embeddable (~600KB)

**Cons:**
- Larger footprint than Lua
- More complex to embed properly
- Overkill for our needs

### Option 4: Embed Starlark (Python-like)
**Pros:**
- Designed for configuration/build tools
- Deterministic execution
- No I/O by design (safe)
- Used by Bazel

**Cons:**
- Less familiar syntax
- Smaller ecosystem

### Option 5: Embed Wren/Gravity
**Pros:**
- Tiny scripting languages designed for embedding
- Wren ~80KB, Gravity ~200KB
- Clean APIs

**Cons:**
- Less mature ecosystems
- Smaller communities

### Option 6: Custom DSL for Metaprogramming
**Pros:**
- Tailored exactly to our needs
- Can be much simpler than full MinZ
- Fast to implement

**Cons:**
- Yet another language to learn
- Limited capabilities

## Recommended Approach: Hybrid with Lua

```minz
// MinZ compile-time evaluation using embedded Lua
@lua[[
function generate_sine_table()
    local table = {}
    for i = 0, 255 do
        local angle = (i * 2 * math.pi) / 256
        table[i] = math.floor(math.sin(angle) * 127)
    end
    return table
end

-- Generate MinZ code
function generate_enum(name, count)
    local code = "enum " .. name .. " {\n"
    for i = 0, count-1 do
        code = code .. "    Item" .. i .. ",\n"
    end
    code = code .. "}\n"
    return code
end
]]

// Use Lua-generated data in MinZ
const SINE_TABLE: [i8; 256] = @lua(generate_sine_table());

// Generate MinZ code from Lua
@lua_eval(generate_enum("PowerUpType", 8))

// Conditional compilation with Lua
@lua_if(os.getenv("DEBUG") == "1")
const MAX_SPRITES: u8 = 16;
@lua_else
const MAX_SPRITES: u8 = 64;
@lua_endif
```

## Implementation Plan

### Phase 1: Basic Lua Integration
1. Embed Lua interpreter in the compiler
2. Add @lua[[...]] blocks for Lua code
3. Add @lua() for calling Lua functions
4. Marshal basic types (numbers, strings, arrays)

### Phase 2: Code Generation
1. Add MinZ AST builders in Lua
2. Allow Lua to generate MinZ code
3. Template system for common patterns

### Phase 3: Build System Integration
1. Lua-based build configuration
2. Conditional compilation
3. Platform-specific code generation

## Example: Advanced Metaprogramming with Lua

```minz
@lua[[
-- Load sprite data from external file
function load_sprite_data(filename)
    local file = io.open(filename, "rb")
    local data = {}
    while true do
        local byte = file:read(1)
        if not byte then break end
        table.insert(data, string.byte(byte))
    end
    file:close()
    return data
end

-- Generate optimized unrolled loops
function unroll_loop(var, start, stop, step, body)
    local code = ""
    for i = start, stop-1, step do
        local expanded = body:gsub("${" .. var .. "}", tostring(i))
        code = code .. expanded .. "\n"
    end
    return code
end

-- Z80-specific optimizations
function generate_fast_multiply(n)
    if n == 2 then return "add hl, hl"
    elseif n == 4 then return "add hl, hl\nadd hl, hl"
    elseif n == 8 then return "add hl, hl\nadd hl, hl\nadd hl, hl"
    else
        -- Generate optimal multiplication sequence
        local code = ""
        local shifts = {}
        local temp = n
        local bit = 0
        
        while temp > 0 do
            if temp & 1 == 1 then
                table.insert(shifts, bit)
            end
            temp = temp >> 1
            bit = bit + 1
        end
        
        -- Generate addition chain
        -- ... implementation ...
        
        return code
    end
end
]]

// Use Lua for complex compile-time tasks
const PLAYER_SPRITE: [u8; 64] = @lua(load_sprite_data("assets/player.bin"));

// Generate optimized code
fn draw_scanlines() -> void {
    @lua_code(unroll_loop("y", 0, 192, 8, [[
        draw_scanline(${y});
        draw_scanline(${y} + 1);
        draw_scanline(${y} + 2);
        draw_scanline(${y} + 3);
        draw_scanline(${y} + 4);
        draw_scanline(${y} + 5);
        draw_scanline(${y} + 6);
        draw_scanline(${y} + 7);
    ]]))
}

// Platform-specific optimizations
fn multiply_by_constant(x: u16, @const n: u16) -> u16 {
    asm(@lua(generate_fast_multiply(n)));
}
```

## Benefits of Lua Approach

1. **Powerful**: Full programming language at compile time
2. **Proven**: Used in many successful projects
3. **Fast**: Compile-time code runs at near-C speed
4. **Flexible**: Can generate any code pattern
5. **Extensible**: Easy to add new compile-time functions
6. **File I/O**: Can read assets, configuration files
7. **Debugging**: Lua has good debugging support

## Transition Plan

1. Keep current simple evaluator for basic @if, @assert
2. Add Lua support as @lua for advanced use cases
3. Gradually migrate complex metaprogramming to Lua
4. Eventually can implement MinZ interpreter in Lua itself!