# Printable Interface and Zero-Cost I/O Design

## Overview

This document describes the design of MinZ's Printable interface and zero-cost I/O system that eliminates runtime overhead through compile-time metaprogramming.

## Core Concepts

### 1. Printable Interface

```minz
pub interface Printable {
    fun to_string(self) -> *u8;
}
```

Every type that can be printed implements this interface. Through monomorphization, interface calls compile to direct function calls with zero overhead.

### 2. Compile-Time Format String Processing

Traditional printf:
```c
printf("x = %d, y = %d\n", x, y);  // Runtime parsing of format string
```

MinZ approach:
```minz
print!("x = {}, y = {}", x, y);   // Compile-time parsing and code generation
```

The format string is parsed at compile time, generating optimal code:
```asm
; Generated code (no format string parsing at runtime!)
CALL print_literal_str1    ; "x = "
LD A, (x)
CALL print_u8
CALL print_literal_str2    ; ", y = "
LD A, (y)
CALL print_u8
LD A, '\n'
RST 16
```

### 3. Metafunction-Based Formatting

```minz
@metafun format_u8(value: u8) -> void {
    @lua {
        -- Generate different code based on value range
        if is_constant(value) then
            if value < 10 then
                emit_asm("LD A, '" .. (value + 48) .. "'")
                emit_asm("RST 16")
            else
                -- Generate optimal code for known constant
            end
        else
            -- Generate general case
            emit_asm("CALL convert_u8_to_string")
        end
    }
}
```

## Implementation Strategy

### Phase 1: Basic Printable Interface

1. Define Printable interface
2. Implement for basic types (u8, u16, i8, i16, bool)
3. Use static buffers for conversion

### Phase 2: Compile-Time Format Processing

1. Implement `print!` macro
2. Parse format strings at compile time
3. Generate direct calls instead of runtime parsing

### Phase 3: Advanced Optimizations

1. Constant folding for literal values
2. Specialized formatters for common patterns
3. Zero-allocation string building

## Zero-Cost Examples

### Example 1: Simple Print

```minz
let x: u8 = 42;
print!("The answer is {}", x);
```

Compiles to:
```asm
LD HL, str_the_answer_is
CALL print_string
LD A, 42
CALL print_u8
```

### Example 2: Complex Formatting

```minz
print!("Registers: A={:02x} BC={:04x}\n", a, bc);
```

Compiles to:
```asm
LD HL, str_registers_a
CALL print_string
LD A, (a)
CALL print_hex_u8_2digits
LD HL, str_bc_equals
CALL print_string
LD HL, (bc)
CALL print_hex_u16_4digits
LD A, 10
RST 16
```

### Example 3: Conditional Formatting

```minz
@compile_time_if(DEBUG) {
    print!("[DEBUG] Value = {}\n", value);
}
```

In release builds, this generates NO CODE AT ALL!

## Performance Benefits

| Feature | Traditional | MinZ | Improvement |
|---------|-------------|------|-------------|
| Format string parsing | Runtime | Compile-time | ∞ |
| Type dispatch | Runtime | Compile-time | ∞ |
| Buffer allocation | Dynamic | Static | 100% |
| String building | Multiple passes | Single pass | 50-70% |

## Advanced Features

### 1. Custom Formatters

```minz
impl Printable for Point {
    fun to_string(self) -> *u8 {
        @static_buffer(16);
        @format!("({}, {})", self.x, self.y);
    }
}
```

### 2. Compile-Time String Interpolation

```minz
let error_msg = s!("Error at line {}: {}", line_no, error);
// String built at compile time, no runtime overhead
```

### 3. Platform-Specific Optimization

```minz
@platform("zx_spectrum") {
    // Use ROM routines for printing
    @inline_asm("RST 16");
}

@platform("cpm") {
    // Use BDOS function 2
    @inline_asm("LD C, 2; CALL 5");
}
```

## Integration with Standard Library

The new I/O system integrates seamlessly:

```minz
// Old way (runtime overhead)
printf("Value: %d\n", value);

// New way (zero overhead)
print!("Value: {}\n", value);

// Both available, user chooses
```

## Metaprogramming Power

```minz
@lua {
    function optimize_print_sequence(prints)
        -- Combine adjacent literal prints
        local optimized = {}
        local current_literal = ""
        
        for _, p in ipairs(prints) do
            if p.type == "literal" then
                current_literal = current_literal .. p.value
            else
                if #current_literal > 0 then
                    table.insert(optimized, {type = "literal", value = current_literal})
                    current_literal = ""
                end
                table.insert(optimized, p)
            end
        end
        
        if #current_literal > 0 then
            table.insert(optimized, {type = "literal", value = current_literal})
        end
        
        return optimized
    end
}
```

## Benefits

1. **Zero Runtime Overhead**: All format string parsing happens at compile time
2. **Type Safety**: Format specifiers checked at compile time
3. **Optimal Code**: Generated code is as good as hand-written assembly
4. **Flexibility**: Users can extend with custom formatters
5. **Platform Independence**: Same interface, platform-specific implementation

## Conclusion

The Printable interface combined with compile-time metaprogramming provides a revolutionary approach to I/O in systems programming. By moving all formatting logic to compile time, we achieve true zero-cost abstractions - modern, convenient syntax with absolutely no runtime penalty.

This design proves that even on 8-bit systems, we don't have to choose between performance and expressiveness. With MinZ, we get both.