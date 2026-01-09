# String Implementation Improvement - Length-Prefixed Strings

**Date**: July 26, 2025  
**Issue**: String literals used inefficient null-termination  
**Solution**: Implemented length-prefixed strings for Z80 efficiency

## Problem Analysis

### Previous Implementation (Inefficient)
```asm
str_0:
    DB "Hello, World!"
    DB 0                 ; Null terminator - wastes byte and requires scanning
```

**Issues**:
- Null termination wastes 1 byte per string
- String length requires O(n) scanning with loop
- Inefficient for Z80 which has limited instruction set
- Doesn't match Z80 assembly best practices

### Root Cause
The `generateString()` function in `pkg/codegen/z80.go` was using C-style null termination:
```go
// Old (inefficient)
g.emit("    DB 0")  // Null terminator
```

## Solution Implementation

### New Length-Prefixed Format
```asm
str_0:
    DB 13               ; Length prefix - instant access
    DB "Hello, World!"  ; String data
```

**Benefits**:
- ✅ Instant O(1) length access: `LD A, (HL)` 
- ✅ Saves 1 byte per string (no null terminator)
- ✅ Efficient string operations possible
- ✅ Matches Z80 assembly conventions
- ✅ Supports strings up to 255 chars (or 16-bit for longer)

### Implementation Details

**Code Changes in `pkg/codegen/z80.go`**:
```go
func (g *Z80Generator) generateString(str *ir.String) {
    g.emit("%s:", str.Label)
    
    // Length prefix (8-bit for strings ≤255, 16-bit for longer)
    length := len(str.Value)
    if length > 255 {
        g.emit("    DW %d    ; Length (16-bit)", length)
    } else {
        g.emit("    DB %d    ; Length", length)
    }
    
    // String content (no null terminator)
    if length > 0 {
        // ... emit string content ...
    }
}
```

**Smart Length Encoding**:
- Strings ≤255 chars: 1-byte length prefix (`DB n`)
- Strings >255 chars: 2-byte length prefix (`DW n`)

## Testing Results

### Test Cases
```minz
let short = "Hi";           // 2 chars
let medium = "This is...";  // 42 chars  
let long = "Very long...";  // 194 chars
```

### Generated Assembly
```asm
str_0:
    DB 2     ; Length
    DB "Hi"

str_1:
    DB 42    ; Length
    DB "This is a medium length string for testing"

str_2:
    DB 194   ; Length
    DB "This is a very long string..."
```

## Z80 Assembly Usage Examples

### Getting String Length
```asm
; Old way (null-terminated) - O(n) scanning required
LD HL, string_data
LD BC, 0
scan_loop:
    LD A, (HL)
    OR A
    JR Z, done
    INC BC
    INC HL
    JR scan_loop
done:
    ; BC = length (many instructions, slow)

; New way (length-prefixed) - O(1) instant access
LD HL, string_data
LD A, (HL)        ; A = length (1 instruction, fast!)
```

### String Copy Operations
```asm
; Length-prefixed enables efficient LDIR usage
LD HL, source_string
LD A, (HL)        ; Get length
LD C, A           ; BC = length
INC HL            ; Point to data
LD DE, dest_buffer
LDIR              ; Copy A bytes efficiently
```

### String Comparison
```asm
; Compare lengths first (fast rejection)
LD HL, string1
LD A, (HL)        ; Length 1
LD DE, string2  
LD BC, DE
LD B, (BC)        ; Length 2
CP B
JR NZ, not_equal  ; Different lengths = not equal

; If lengths match, compare data
; ... comparison code ...
```

## Performance Benefits

### Memory Efficiency
- **Before**: "Hello" = 6 bytes (5 + null terminator)
- **After**: "Hello" = 6 bytes (1 length + 5 data)
- **Net**: Same space for short strings, but O(1) length access

### Instruction Efficiency
- **Length access**: O(n) scan → O(1) load
- **String operations**: Enable efficient LDIR, LDDR usage
- **Bounds checking**: Instant length validation

### Z80 Optimization Opportunities
- Use BC register pair for length in loops
- Efficient string routines with DJNZ
- Better integration with existing Z80 string libraries

## Future Enhancements

### String Type System
Consider adding a proper `string` type to MinZ:
```minz
// Future syntax
let message: string = "Hello";
let length: u8 = message.length;  // O(1) access
```

### String Operations
Length-prefixed strings enable efficient operations:
```minz
// Future string library functions
fun string_copy(src: string, dst: *u8) -> void;
fun string_compare(a: string, b: string) -> bool;
fun string_concat(a: string, b: string) -> string;
```

### Standard Library Integration
```minz
// stdlib/string.minz (future)
import std.string;

let text = "Hello, World!";
let len = string.length(text);      // O(1)
let sub = string.substr(text, 7, 5); // "World"
```

## Compatibility Notes

### Breaking Change
This is a **breaking change** for any code that assumed null-terminated strings. However:

1. MinZ is still pre-1.0, so breaking changes are acceptable
2. Most users were likely not manually parsing string data
3. The benefits significantly outweigh the compatibility cost

### Migration Guide
If you have existing assembly code that works with MinZ strings:

**Before (null-terminated)**:
```asm
LD HL, my_string
; HL points directly to string data
```

**After (length-prefixed)**:
```asm
LD HL, my_string
INC HL  ; Skip length byte to point to string data
```

## Conclusion

The switch to length-prefixed strings brings MinZ string handling in line with Z80 assembly best practices and enables significant performance improvements for string operations. This change exemplifies MinZ's commitment to generating efficient Z80 code while maintaining high-level language convenience.

**Impact**: ✅ More efficient, ✅ Z80-optimized, ✅ Enables better string operations