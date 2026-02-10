# 🎉 MZA Major Improvements: Arithmetic Expressions & Length-Prefix Macros

## Summary
MZA (MinZ Assembler) has received two major improvements that will significantly simplify the MinZ compiler's code generation:

1. **Full arithmetic expression support** in all contexts
2. **Automatic length-prefix macros** for strings and arrays

## 1. Arithmetic Expressions ✅

MZA now evaluates arithmetic expressions everywhere:

```asm
; Constants with expressions
BUFFER_SIZE EQU 256
DOUBLE_SIZE EQU BUFFER_SIZE*2    ; 512

; Direct arithmetic in instructions
LD A, 10+5                        ; Assembles to: LD A, 15
LD HL, start+10                   ; Address arithmetic

; Expressions in data
DB 5+3                            ; Emits: 08
DW BUFFER_SIZE+1                  ; Emits: 01 01 (257 in little-endian)
```

### Supported Operations
- Addition: `value+10`
- Subtraction: `end-start`  
- Multiplication: `SIZE*2`
- Division: `TOTAL/4`
- Symbol references: `label+offset`

## 2. Length-Prefix Macros 🎯

Revolutionary feature for Pascal-style strings and counted arrays:

### Basic Macros (8-bit)
```asm
; Automatic string length
msg:
    DB @len, "Hello, World!"      ; Emits: 0D 48 65 6C 6C 6F...

; Count array elements
data:
    DB @count, 10, 20, 30, 40     ; Emits: 04 0A 14 1E 28
```

### Extended Macros (16-bit)
```asm
; For larger data (>255 bytes)
big_text:
    DB @len_u16, "Very long string..."   ; Emits 2-byte length (little-endian)

; 16-bit counts
vtable:
    DW @count_u16, method1, method2, method3
```

### Complete Macro Set
- `@len` / `@len_u8` - 8-bit byte length (max 255)
- `@len_u16` - 16-bit byte length (max 65535)
- `@count` / `@count_u8` - 8-bit item count
- `@count_u16` - 16-bit item count
- `@size` - Alias for @len

All include **overflow protection** with clear error messages!

## Compiler Integration Benefits

### Before (Current MinZ Compiler Output)
```asm
_str_hello:
    DB 5            ; Manually calculated
    DB "Hello"
    
_array:
    DB 3            ; Manual count
    DB 10, 20, 30
    
some_func:
    LD HL, 32778    ; Manual calculation of $8000+10
```

### After (Using New Features)
```asm
_str_hello:
    DB @len, "Hello"              ; Automatic!
    
_array:
    DB @count, 10, 20, 30         ; Automatic!
    
some_func:
    LD HL, BASE_ADDR+10           ; Readable!
```

## Real Example: String Table
```asm
; Old way (error-prone, hard to maintain)
error_table:
    DW 3                          ; Manual count
    DW error_1, error_2, error_3
error_1:
    DB 14                         ; Manual length
    DB "File not found"
error_2:
    DB 13                         ; Manual length
    DB "Out of memory"
    
; New way (automatic, maintainable)
error_1:
    DB @len, "File not found"     ; Length calculated automatically
error_2:
    DB @len, "Out of memory"      ; No manual counting!
error_table:
    DW @count_u16, error_1, error_2, error_3  ; Count calculated!
```

## Testing the Features

```bash
# Test arithmetic
echo 'SIZE EQU 10
LD A, SIZE*2+5' | mza -o test.bin -
hexdump -C test.bin  # Shows: 3E 19 (LD A, 25)

# Test @len macro
echo 'DB @len, "Hello"' | mza -o test.bin -
hexdump -C test.bin  # Shows: 05 48 65 6C 6C 6F

# Test overflow protection
echo 'DB @len_u8, "x"*300' | mza -o test.bin -
# Error: @len_u8 overflow: length is 300 bytes (max 255)
```

## Impact on MinZ Compiler

### Immediate Benefits
1. **Cleaner code generation** - No manual length calculations
2. **Fewer bugs** - Automatic counting prevents off-by-one errors
3. **Better readability** - `start+10` instead of hardcoded addresses
4. **Easier maintenance** - Change strings without updating lengths

### Suggested Adoption Path
1. **Phase 1**: Use @len for all new string literals
2. **Phase 2**: Add arithmetic for address calculations  
3. **Phase 3**: Migrate existing patterns gradually

### Code Generation Examples

For MinZ string literals:
```minz
let msg = "Hello, World!";
```
Generate:
```asm
_str_msg:
    DB @len, "Hello, World!"    ; No manual calculation needed!
```

For MinZ arrays:
```minz
let data = [10, 20, 30, 40];
```
Generate:
```asm
_array_data:
    DB @count, 10, 20, 30, 40   ; Count included automatically!
```

## Backwards Compatibility
- Existing assembly code works unchanged
- Manual length calculation still supported
- Can mix manual and automatic approaches

## Performance
- **Assembly time**: Slightly faster (less processing in compiler)
- **Runtime**: Identical (macros expand during assembly)
- **Binary size**: Identical

## Available Now!
These features are implemented and tested in the latest MZA build. The MinZ compiler can start using them immediately!

## Documentation
See `minzc/docs/MZA_LENGTH_PREFIX_MACROS.md` for complete integration guide.

---

*These improvements make MZA more powerful while keeping assembly output cleaner and more maintainable. Perfect for compiler-generated code!*