-- MinZ @print Metafunction Implementation
-- Pure Lua script that runs at compile time to generate zero-cost printing

-- Core metafunction registration
minz.register_metafunction("print", function(format_str, ...)
    local args = {...}
    local result = {}
    
    -- Parse format string at compile time
    local i = 1
    local arg_index = 1
    
    while i <= #format_str do
        local char = format_str:sub(i, i)
        
        if char == '{' and i < #format_str and format_str:sub(i+1, i+1) == '}' then
            -- Found {} placeholder
            if arg_index <= #args then
                local arg = args[arg_index]
                
                -- Generate optimal code based on argument type
                if type(arg) == "number" then
                    -- Compile-time constant
                    if arg == math.floor(arg) and arg >= 0 and arg <= 255 then
                        -- u8 constant - emit direct digit sequence
                        table.insert(result, emit_u8_digits(arg))
                    elseif arg == math.floor(arg) and arg >= 0 and arg <= 65535 then
                        -- u16 constant - emit direct digit sequence
                        table.insert(result, emit_u16_digits(arg))
                    else
                        error("Unsupported number constant: " .. arg)
                    end
                elseif type(arg) == "string" then
                    -- String literal - emit direct bytes
                    table.insert(result, emit_string_literal(arg))
                elseif type(arg) == "boolean" then
                    -- Boolean constant
                    if arg then
                        table.insert(result, emit_string_literal("true"))
                    else
                        table.insert(result, emit_string_literal("false"))
                    end
                else
                    -- Runtime variable - emit call to print function
                    table.insert(result, emit_variable_print(arg))
                end
                
                arg_index = arg_index + 1
            else
                error("Not enough arguments for format string")
            end
            i = i + 2  -- Skip {}
        else
            -- Regular character - accumulate into string literal
            local literal = ""
            while i <= #format_str and not (format_str:sub(i, i) == '{' and i < #format_str and format_str:sub(i+1, i+1) == '}') do
                literal = literal .. format_str:sub(i, i)
                i = i + 1
            end
            if #literal > 0 then
                table.insert(result, emit_string_literal(literal))
            end
        end
    end
    
    return table.concat(result, "\n")
end)

-- Helper function to emit u8 digits directly
function emit_u8_digits(value)
    if value == 0 then
        return "RST 16 ; '0'"
    end
    
    local digits = {}
    local temp = value
    while temp > 0 do
        table.insert(digits, 1, temp % 10)
        temp = math.floor(temp / 10)
    end
    
    local result = {}
    for _, digit in ipairs(digits) do
        table.insert(result, string.format("LD A, %d ; '%s'", 48 + digit, digit))
        table.insert(result, "RST 16")
    end
    
    return table.concat(result, "\n")
end

-- Helper function to emit u16 digits directly
function emit_u16_digits(value)
    if value == 0 then
        return "RST 16 ; '0'"
    end
    
    local digits = {}
    local temp = value
    while temp > 0 do
        table.insert(digits, 1, temp % 10)
        temp = math.floor(temp / 10)
    end
    
    local result = {}
    for _, digit in ipairs(digits) do
        table.insert(result, string.format("LD A, %d ; '%s'", 48 + digit, digit))
        table.insert(result, "RST 16")
    end
    
    return table.concat(result, "\n")
end

-- Helper function to emit string literal
function emit_string_literal(str)
    local result = {}
    for i = 1, #str do
        local byte = string.byte(str:sub(i, i))
        table.insert(result, string.format("LD A, %d ; '%s'", byte, str:sub(i, i)))
        table.insert(result, "RST 16")
    end
    return table.concat(result, "\n")
end

-- Helper function to emit variable printing
function emit_variable_print(var)
    -- This would need to know the variable type
    -- For now, assume it's a register variable
    return string.format([[
; Print variable %s
CALL print_%s
]], var.name, var.type)
end

-- Register other print metafunctions
minz.register_metafunction("println", function(format_str, ...)
    local print_code = minz.call_metafunction("print", format_str, ...)
    return print_code .. "\nLD A, 10 ; '\\n'\nRST 16"
end)

minz.register_metafunction("debug", function(expr)
    local debug_prefix = string.format("[DEBUG] %s = ", expr.source_text)
    return minz.call_metafunction("print", debug_prefix .. "{}", expr)
end)

minz.register_metafunction("format", function(format_str, ...)
    -- Build format string at compile time if all args are constants
    local args = {...}
    local all_constants = true
    
    for _, arg in ipairs(args) do
        if not (type(arg) == "number" or type(arg) == "string" or type(arg) == "boolean") then
            all_constants = false
            break
        end
    end
    
    if all_constants then
        -- Fully resolve at compile time
        local result = format_str
        for i, arg in ipairs(args) do
            local replacement
            if type(arg) == "number" then
                replacement = tostring(arg)
            elseif type(arg) == "string" then
                replacement = arg
            elseif type(arg) == "boolean" then
                replacement = arg and "true" or "false"
            end
            result = result:gsub("{}", replacement, 1)
        end
        return minz.emit_string_constant(result)
    else
        -- Generate runtime format code
        return generate_runtime_format(format_str, args)
    end
end)

function generate_runtime_format(format_str, args)
    -- This would generate efficient runtime formatting code
    -- Similar to @print but storing to a buffer instead of printing
    return "CALL runtime_format ; TODO: implement"
end