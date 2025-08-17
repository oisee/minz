# MZA Length-Prefix Macros - Compiler Integration Guide

## Overview
MZA now supports automatic length-prefix macros that simplify generating Pascal-style strings and counted arrays. This can significantly clean up MinZ compiler's assembly output.

## Available Macros

### Basic Length Macros (8-bit)
- `@len` / `@len_u8` - Calculates byte length of remaining operands (max 255)
- `@size` - Alias for @len
- `@count` / `@count_u8` - Counts number of remaining operands (max 255)

### Extended Length Macros (16-bit)
- `@len_u16` / `@size16` - Calculates byte length as 16-bit value (max 65535)
- `@count_u16` - Counts operands as 16-bit value

## Usage Examples

### Before (what compiler currently generates):
```asm
; String with manual length calculation
string_hello:
    DB 5            ; Manual length
    DB "Hello"
    
; Array with count
array_data:
    DB 3            ; Manual count
    DB 10, 20, 30
```

### After (using new macros):
```asm
; String with automatic length
string_hello:
    DB @len, "Hello"        ; Automatically inserts 5
    
; Array with count
array_data:
    DB @count, 10, 20, 30   ; Automatically inserts 3
```

## Compiler Benefits

### 1. String Literals
Instead of:
```asm
_str_0:
    DB 13
    DB "Hello, World!"
```

Generate:
```asm
_str_0:
    DB @len, "Hello, World!"
```

### 2. String Tables
Instead of calculating offsets:
```asm
error_table:
    DW 3                    ; Count
    DW error_1, error_2, error_3
error_1:
    DB 14
    DB "File not found"
error_2:
    DB 13
    DB "Out of memory"
error_3:
    DB 14
    DB "Invalid syntax"
```

Generate:
```asm
error_1:
    DB @len, "File not found"
error_2:
    DB @len, "Out of memory"
error_3:
    DB @len, "Invalid syntax"
error_table:
    DW @count_u16, error_1, error_2, error_3
```

### 3. Dynamic Arrays
For MinZ arrays that track their size:
```asm
; MinZ: let data = [10, 20, 30, 40, 50];
_array_data:
    DB @count, 10, 20, 30, 40, 50  ; Count included automatically
```

### 4. Large Data Blocks
For data that might exceed 255 bytes:
```asm
large_text:
    DB @len_u16         ; Emits 2 bytes (little-endian)
    DB "Very long text..."
    DB "...continues..."
```

## Integration with MinZ Features

### For MinZ Strings
When compiling MinZ string literals, use @len for automatic length prefixing:
```minz
let msg = "Hello";
```
Generates:
```asm
_str_msg:
    DB @len, "Hello"    ; 05 48 65 6C 6C 6F
```

### For MinZ Arrays
```minz
let nums: [u8; 5] = [1, 2, 3, 4, 5];
```
Could generate:
```asm
_array_nums:
    DB @count, 1, 2, 3, 4, 5    ; 05 01 02 03 04 05
```

### For Interfaces/Vtables
Method count in vtables:
```asm
_vtable_drawable:
    DW @count_u16       ; Number of methods
    DW _method_draw
    DW _method_clear
    DW _method_update
```

## Overflow Protection
All macros include overflow checking:
- `@len/@count` - Error if length/count > 255
- `@len_u16/@count_u16` - Error if length/count > 65535

Example error:
```
Assembly error: @len overflow: length is 300 bytes (max 255)
```

## Compatibility Notes

### With DB Directive
- `@len` / `@len_u8` → emits 1 byte
- `@len_u16` → emits 2 bytes (low, high)
- `@count` / `@count_u8` → emits 1 byte  
- `@count_u16` → emits 2 bytes (low, high)

### With DW Directive
- All macros work with DW
- 8-bit macros are zero-extended to 16-bit
- Useful for offset tables

## Implementation Priority

### High Priority (Immediate Wins)
1. **String literals** - Replace manual length calculation
2. **String constants** - Use @len for all string data
3. **Error messages** - Simplify error table generation

### Medium Priority
1. **Array literals** - Add @count for sized arrays
2. **Module strings** - Module name tables
3. **Debug info** - Source location strings

### Low Priority  
1. **Optimization** - Analyze whether @len_u8 or @len_u16 is needed
2. **Custom macros** - Extend for specific patterns

## Example: Complete String Module
```asm
; Before (manual calculation)
_mod_strings:
    DB 3            ; Count of strings
_str_0:
    DB 5
    DB "Hello"
_str_1:
    DB 5  
    DB "World"
_str_2:
    DB 4
    DB "Test"

; After (with macros)
_mod_strings:
    DB @count       ; Automatically counts following strings
_str_0:
    DB @len, "Hello"
_str_1:
    DB @len, "World"
_str_2:
    DB @len, "Test"
```

## Performance Impact
- **Compile time**: Slightly faster (no manual length calculation)
- **Runtime**: Identical (macros expand at assembly time)
- **Binary size**: Identical
- **Debugging**: Cleaner assembly output

## Migration Path
1. Start with new code generation
2. Update string literal generation first
3. Gradually migrate other patterns
4. Keep manual calculation as fallback

## Testing
```bash
# Verify macro expansion
echo 'DB @len, "Hello"' | mza -o test.bin -
hexdump -C test.bin  # Should show: 05 48 65 6C 6C 6F
```

---

This feature is available in MZA as of the latest build. The MinZ compiler can start using these macros immediately for cleaner, more maintainable assembly output.