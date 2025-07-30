# MinZ Lua Metaprogramming Guide

## Overview

MinZ uses embedded Lua for powerful compile-time metaprogramming. This allows you to:
- Generate code at compile time
- Load and process external files
- Perform complex calculations for lookup tables
- Conditional compilation with full programming logic
- Transform data files into optimized code

## Basic Syntax

### Lua Code Blocks

```minz
@lua[[
    -- Lua code executed at compile time
    function helper()
        return 42
    end
    
    MY_CONSTANT = helper() * 2
]]
```

### Lua Expressions

```minz
// Evaluate Lua expression and use result
const VALUE: u8 = @lua(MY_CONSTANT);
const TABLE: [u8; 4] = @lua({1, 2, 3, 4});
```

### Code Generation

```minz
// Generate MinZ code from Lua
@lua_eval(generate_some_code())
```

## Practical Examples

### 1. Lookup Table Generation

```minz
@lua[[
function generate_sqrt_table()
    local table = {}
    for i = 0, 255 do
        table[i + 1] = math.floor(math.sqrt(i) + 0.5)
    end
    return table
end
]]

const SQRT_TABLE: [u8; 256] = @lua(generate_sqrt_table());
```

### 2. Loading External Data

```minz
@lua[[
function load_level_data(filename)
    local file = io.open(filename, "rb")
    local data = {}
    -- Read tile data
    for i = 1, 32 * 24 do  -- 32x24 tiles
        local byte = file:read(1)
        data[i] = string.byte(byte)
    end
    file:close()
    return data
end
]]

const LEVEL_1: [u8; 768] = @lua(load_level_data("assets/level1.bin"));
```

### 3. Code Generation from Data

```minz
@lua[[
function generate_state_machine(states)
    local code = "enum State {\n"
    for _, state in ipairs(states) do
        code = code .. "    " .. state .. ",\n"
    end
    code = code .. "}\n"
    return code
end
]]

@lua_eval(generate_state_machine({"Idle", "Walking", "Jumping", "Falling"}))
```

### 4. Platform-Specific Compilation

```minz
@lua[[
platform = os.getenv("TARGET") or "ZX_SPECTRUM"
features = {
    has_ay_chip = platform == "ZX_SPECTRUM_128",
    has_extra_ram = platform ~= "ZX_SPECTRUM_48",
}
]]

@lua_if(features.has_ay_chip)
import zx.ay_music;
fn play_music(track: u8) -> void {
    ay_music.play(track);
}
@lua_else
fn play_music(track: u8) -> void { }
@lua_endif
```

### 5. Optimized Code Generation

```minz
@lua[[
function unroll_loop(count, body)
    local code = ""
    for i = 0, count - 1 do
        -- Replace $i with actual value
        local expanded = body:gsub("$i", tostring(i))
        code = code .. expanded .. "\n"
    end
    return code
end

function generate_fast_clear()
    return unroll_loop(24, "    ld (hl), 0\n    inc hl  ; Line $i")
end
]]

fn fast_clear_line() -> void {
    asm(@lua(generate_fast_clear()));
}
```

## Lua API Reference

### Built-in Functions

MinZ provides these Lua functions:

```lua
-- MinZ code generation
minz.enum(name, variants)        -- Generate enum
minz.struct(name, fields)        -- Generate struct
minz.const_array(name, type, values) -- Generate const array

-- File operations
load_binary(filename)            -- Load binary file as byte array
load_text(filename)              -- Load text file as string
file_exists(filename)            -- Check if file exists

-- Platform info
PLATFORM                         -- Target platform string
ARCH                            -- Architecture (always "Z80")
```

### Environment Variables

Access build-time environment variables:

```lua
debug_mode = os.getenv("DEBUG") == "1"
optimize_level = tonumber(os.getenv("OPT_LEVEL") or "0")
```

## Best Practices

### 1. Use Lua for Complex Logic

Good use case:
```minz
@lua[[
-- Complex calculation that would be tedious in macros
function generate_sine_table(bits)
    local scale = (1 << bits) - 1
    local table = {}
    for i = 0, 255 do
        local angle = (i * 2 * math.pi) / 256
        table[i + 1] = math.floor(math.sin(angle) * scale / 2 + 0.5)
    end
    return table
end
]]
```

### 2. Cache Expensive Operations

```minz
@lua[[
-- Cache loaded data
local sprite_cache = {}

function load_sprite(name)
    if not sprite_cache[name] then
        sprite_cache[name] = load_binary("sprites/" .. name .. ".spr")
    end
    return sprite_cache[name]
end
]]
```

### 3. Validate at Compile Time

```minz
@lua[[
function validate_config(config)
    assert(config.screen_width == 256, "Invalid screen width")
    assert(config.colors <= 8, "Too many colors")
    return config
end

config = validate_config(load_config("game.conf"))
]]
```

### 4. Generate Optimized Assembly

```minz
@lua[[
function generate_multiply_by_constant(n)
    -- Generate optimal shift/add sequence
    local code = ""
    local shifts = {}
    
    -- Find set bits
    local temp = n
    local bit = 0
    while temp > 0 do
        if temp & 1 == 1 then
            table.insert(shifts, bit)
        end
        temp = temp >> 1
        bit = bit + 1
    end
    
    -- Generate code
    if #shifts == 1 then
        -- Just shifting
        for i = 1, shifts[1] do
            code = code .. "add hl, hl\n"
        end
    else
        -- Shift and add
        code = code .. "push de\n"
        code = code .. "ld d, h\n"
        code = code .. "ld e, l\n"
        
        for i = 1, shifts[1] do
            code = code .. "add hl, hl\n"
        end
        
        for i = 2, #shifts do
            local extra_shifts = shifts[i] - shifts[i-1]
            for j = 1, extra_shifts do
                code = code .. "add hl, hl\n"
            end
            code = code .. "add hl, de\n"
        end
        
        code = code .. "pop de\n"
    end
    
    return code
end
]]

fn multiply_by_10(x: u16) -> u16 {
    // Generates: HL = HL * 2 + HL * 8
    asm(@lua(generate_multiply_by_constant(10)));
}
```

## Debugging Lua Code

### Print Debugging

```minz
@lua[[
print("Debug: Loading sprites...")
local data = load_sprite_file("player.spr")
print("Loaded " .. #data .. " bytes")
]]
```

### Error Handling

```minz
@lua[[
function safe_load(filename)
    local ok, result = pcall(load_binary, filename)
    if not ok then
        print("Warning: Could not load " .. filename)
        return {}  -- Return empty table
    end
    return result
end
]]
```

## Integration with Build System

Create a `build.lua` file:

```lua
-- build.lua
local config = {
    platform = arg[1] or "ZX_SPECTRUM",
    debug = arg[2] == "debug",
    features = {}
}

-- Platform-specific features
if config.platform == "ZX_SPECTRUM_128" then
    config.features.sound = true
    config.features.extra_ram = true
end

-- Export to MinZ
return config
```

Use in MinZ:
```minz
@lua[[
build_config = dofile("build.lua")
]]

const DEBUG: bool = @lua(build_config.debug);
```

## Limitations

1. Lua code runs at compile time only
2. Cannot access MinZ runtime values
3. File paths are relative to compiler working directory
4. Standard Lua libraries are available except those requiring runtime (coroutines, etc.)

## Future Enhancements

- Lua-based build system integration
- Custom Lua modules for common tasks
- AST manipulation APIs
- Source map generation for debugging