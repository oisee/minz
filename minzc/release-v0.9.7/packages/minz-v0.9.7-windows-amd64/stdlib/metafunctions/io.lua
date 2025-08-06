-- MinZ I/O Metafunctions
-- Zero-cost I/O operations via compile-time Lua scripting

-- @write_byte - Direct byte output with optimal instruction selection
minz.register_metafunction("write_byte", function(value, target)
    target = target or "stdout"
    
    if type(value) == "number" then
        -- Compile-time constant
        if value >= 32 and value <= 126 then
            -- Printable ASCII
            return string.format([[
LD A, %d ; '%s'
RST 16]], value, string.char(value))
        else
            -- Non-printable
            return string.format([[
LD A, %d ; 0x%02X
RST 16]], value, value)
        end
    else
        -- Runtime variable
        return string.format([[
LD A, (%s)
RST 16]], value.name)
    end
end)

-- @write_string - Optimal string output
minz.register_metafunction("write_string", function(str, target)
    target = target or "stdout"
    
    if type(str) == "string" then
        -- Compile-time string literal
        local result = {}
        for i = 1, #str do
            local char = str:sub(i, i)
            local byte = string.byte(char)
            table.insert(result, string.format("LD A, %d ; '%s'", byte, char))
            table.insert(result, "RST 16")
        end
        return table.concat(result, "\n")
    else
        -- Runtime string variable
        return string.format([[
LD HL, %s
CALL print_string]], str.name)
    end
end)

-- @read_byte - Optimal byte input
minz.register_metafunction("read_byte", function(source)
    source = source or "stdin"
    
    return [[
RST 8   ; Read byte from keyboard
; Result in A register]]
end)

-- @hex - Format number as hexadecimal
minz.register_metafunction("hex", function(value, width)
    width = width or "auto"
    
    if type(value) == "number" then
        -- Compile-time constant
        if width == "auto" then
            if value <= 255 then
                return minz.call_metafunction("write_string", string.format("%02X", value))
            else
                return minz.call_metafunction("write_string", string.format("%04X", value))
            end
        else
            local format_str = string.format("%%0%dX", width)
            return minz.call_metafunction("write_string", string.format(format_str, value))
        end
    else
        -- Runtime variable - generate hex conversion code
        if value.type == "u8" then
            return string.format([[
LD A, (%s)
CALL print_hex_u8]], value.name)
        elseif value.type == "u16" then
            return string.format([[
LD HL, (%s)
CALL print_hex_u16]], value.name)
        end
    end
end)

-- @bin - Format number as binary
minz.register_metafunction("bin", function(value, width)
    if type(value) == "number" then
        -- Compile-time constant
        local binary = ""
        local temp = value
        local bits = width or (value <= 255 and 8 or 16)
        
        for i = bits-1, 0, -1 do
            if temp >= (2^i) then
                binary = binary .. "1"
                temp = temp - (2^i)
            else
                binary = binary .. "0"
            end
        end
        
        return minz.call_metafunction("write_string", binary)
    else
        -- Runtime variable
        return string.format([[
LD A, (%s)
CALL print_bin_u8]], value.name)
    end
end)

-- @assert - Zero-cost assertions
minz.register_metafunction("assert", function(condition, message)
    if minz.is_debug_build() then
        -- Generate assertion code only in debug builds
        return string.format([[
; Assert: %s
%s
JR NZ, assert_ok_%d
%s
CALL panic
assert_ok_%d:]], 
            condition.source_text,
            generate_condition_check(condition),
            minz.next_label_id(),
            minz.call_metafunction("write_string", "ASSERTION FAILED: " .. message),
            minz.current_label_id())
    else
        -- Zero cost in release builds
        return "; Assert optimized out in release build"
    end
end)

-- @static_assert - Compile-time assertions
minz.register_metafunction("static_assert", function(condition, message)
    if not condition then
        error("Static assertion failed: " .. message)
    end
    return "; Static assertion passed"
end)

-- @benchmark - Performance measurement
minz.register_metafunction("benchmark", function(name, code_block)
    if minz.is_benchmark_build() then
        return string.format([[
; Benchmark start: %s
LD HL, benchmark_start_%d
CALL get_cycles
PUSH HL
%s
; Benchmark end: %s
LD HL, benchmark_end_%d
CALL get_cycles
POP DE
SBC HL, DE
CALL print_benchmark_result]], 
            name, minz.next_label_id(), code_block, name, minz.current_label_id())
    else
        return code_block
    end
end)

-- @profile - Function profiling
minz.register_metafunction("profile", function(func_name)
    if minz.is_profile_build() then
        return string.format([[
; Profile entry: %s
CALL profile_enter
DB "%s", 0]], func_name, func_name)
    else
        return ""
    end
end)

-- @optimize - Force optimization level for code block
minz.register_metafunction("optimize", function(level, code_block)
    local old_level = minz.get_optimization_level()
    minz.set_optimization_level(level)
    local result = minz.compile_block(code_block)
    minz.set_optimization_level(old_level)
    return result
end)

-- @inline_asm - Inline assembly with register constraints
minz.register_metafunction("inline_asm", function(asm_code, inputs, outputs)
    inputs = inputs or {}
    outputs = outputs or {}
    
    local result = {}
    
    -- Set up input registers
    for reg, var in pairs(inputs) do
        table.insert(result, string.format("LD %s, (%s)", reg, var))
    end
    
    -- Emit assembly code
    table.insert(result, asm_code)
    
    -- Store output registers
    for reg, var in pairs(outputs) do
        table.insert(result, string.format("LD (%s), %s", var, reg))
    end
    
    return table.concat(result, "\n")
end)

-- @atomic - Atomic operations
minz.register_metafunction("atomic", function(code_block)
    return string.format([[
DI          ; Disable interrupts
%s
EI          ; Re-enable interrupts]], code_block)
end)

-- Platform-specific metafunctions

-- @zx_cls - ZX Spectrum clear screen
minz.register_metafunction("zx_cls", function()
    return "CALL 0x0DAF ; ROM CLS"
end)

-- @zx_beep - ZX Spectrum beep
minz.register_metafunction("zx_beep", function(duration, pitch)
    if type(duration) == "number" and type(pitch) == "number" then
        return string.format([[
LD HL, %d   ; Duration
LD DE, %d   ; Pitch
CALL 0x03B5 ; ROM BEEP]], duration, pitch)
    else
        return [[
LD HL, (duration)
LD DE, (pitch)
CALL 0x03B5 ; ROM BEEP]]
    end
end)

-- @msx_vpoke - MSX video poke
minz.register_metafunction("msx_vpoke", function(addr, value)
    return string.format([[
LD HL, %d
LD A, %d
CALL 0x004D ; MSX VPOKE]], addr, value)
end)

-- Helper functions
function generate_condition_check(condition)
    -- Generate Z80 code to check condition
    -- This would be more complex in practice
    return string.format("LD A, (%s)\nOR A", condition.name)
end