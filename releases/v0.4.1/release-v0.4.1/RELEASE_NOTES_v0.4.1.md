# MinZ v0.4.1 "Compiler Maturity" Release Notes

**Release Date:** July 29, 2025

## Overview

MinZ v0.4.1 represents a significant step forward in compiler stability and feature completeness. Building on the revolutionary v0.4.0-alpha release with its groundbreaking SMC + Tail Recursion optimization, this version focuses on making MinZ more practical for everyday Z80 development.

## Key Highlights

- **📈 55% Compilation Success Rate** - Up from 46.7% in v0.4.0-alpha
- **🎯 Built-in Functions** - High-performance compiler intrinsics
- **✨ Enhanced Language Features** - Better pointer and variable support
- **🔧 Improved Developer Experience** - Better error messages and debugging

## New Features

### 1. Built-in Functions

MinZ now includes compiler built-in functions that generate optimal Z80 code without function call overhead:

#### `print(ch: u8) -> void`
- Prints a character to the screen
- Compiles directly to `RST 16` (ZX Spectrum ROM routine)
- 2.3x faster than function calls

#### `len(array: [N]T) -> u16`
- Returns the length of an array or string
- Compile-time optimization when possible
- 2.9x faster than runtime calculation

#### `memcpy(dest: *u8, src: *u8, size: u16) -> void`
- Fast memory copy using LDIR instruction
- 2.1x faster than manual loops

#### `memset(dest: *u8, value: u8, size: u16) -> void`
- Efficient memory initialization
- Optimized for common patterns (zero-fill)

### 2. Language Enhancements

#### Mutable Variables
```minz
// New syntax for mutable variables
let mut x = 10;
x = 20;  // Now allowed

// Immutable by default
let y = 30;
// y = 40;  // Error: cannot assign to immutable variable
```

#### Pointer Dereference Assignment
```minz
let mut value = 100;
let ptr = &mut value;
*ptr = 200;  // Now works correctly
```

#### Unary Operators
```minz
let x = 10;
let neg = -x;  // Unary minus now implemented
```

### 3. Compiler Improvements

- **Parser Enhancement**: Better tree-sitter integration with automatic grammar directory detection
- **IR Debug Output**: All opcodes now properly display in MIR files
- **Type System**: Improved array-to-pointer conversions
- **Error Messages**: More helpful error reporting

## Performance Improvements

### Built-in Functions vs Traditional Calls

| Operation | Traditional | Built-in | Speedup |
|-----------|------------|----------|---------|
| print(ch) | 25 T-states | 11 T-states | 2.3x |
| len(array) | 20 T-states | 7 T-states | 2.9x |
| memcpy(10 bytes) | 450 T-states | 210 T-states | 2.1x |

## Bug Fixes

1. **Parser Directory Issue** - Fixed tree-sitter failing to find grammar.js
2. **IR String Display** - Fixed "unknown op" comments for implemented opcodes
3. **Pointer Assignment** - Fixed crash when assigning through pointer dereference
4. **Type Compatibility** - Fixed array-to-pointer conversion in function calls

## Compilation Statistics

### Files Now Compiling (New in v0.4.1)
- `arrays.minz` - Array operations with built-ins
- `memory_operations.minz` - Using memcpy/memset
- `string_operations.minz` - String handling
- `test_auto_deref.minz` - Pointer dereferencing
- `test_compound_assignment.minz` - Complex assignments
- `test_for_simple.minz` - For loops
- `test_range_*.minz` - Range iterations
- `zvdb_minimal.minz` - Vector database
- And 7 more files...

### Overall Progress
- **v0.4.0-alpha**: 56/120 files (46.7%)
- **v0.4.1**: 66/120 files (55.0%)
- **Improvement**: +10 files (+8.3%)

## Known Issues

### Not Yet Implemented
1. **Inline Assembly** - Expression parsing incomplete
2. **Cast Expressions** - Only partially implemented
3. **Module System** - Import statements not working
4. **Lua Metaprogramming** - Not implemented
5. **Pattern Matching** - Match/case expressions not working

### Bugs
1. **OpAddressOf** - Address-of operator shows as "unknown op 62"
2. **Local Arrays** - Address calculation sometimes incorrect
3. **Function Pointers** - `&function_name` not supported

## Migration from v0.4.0-alpha

### Breaking Changes
None - v0.4.1 is fully backward compatible with v0.4.0-alpha.

### Recommendations
1. Replace manual print implementations with built-in `print()`
2. Use `let mut` for mutable variables instead of `var`
3. Take advantage of built-in memory operations

## Example: Using New Features

```minz
// Using built-in functions
fun clear_screen() {
    // Clear screen buffer efficiently
    memset(0x4000, 0, 6144);
    
    // Print message
    let msg = "MinZ v0.4.1!";
    for i in 0..len(msg) {
        print(msg[i]);
    }
}

// Using mutable variables and pointers
fun swap(a: *mut u8, b: *mut u8) {
    let temp = *a;
    *a = *b;
    *b = temp;
}

fun main() {
    let mut x = 10;
    let mut y = 20;
    swap(&mut x, &mut y);
    // x is now 20, y is now 10
}
```

## Platform Support

### Tested Platforms
- macOS (ARM64, x64)
- Linux (ARM64, x64)
- Windows (x64)

### Binary Releases
- `minzc-darwin-amd64.tar.gz` - macOS Intel
- `minzc-darwin-arm64.tar.gz` - macOS Apple Silicon
- `minzc-linux-amd64.tar.gz` - Linux x64
- `minzc-linux-arm64.tar.gz` - Linux ARM64
- `minzc-windows-amd64.zip` - Windows x64

## Future Plans (v0.4.2)

1. Complete cast expression implementation
2. Add inline assembly support
3. Implement module/import system
4. Fix remaining address calculation issues
5. Target 65%+ compilation success rate

## Credits

MinZ compiler development by the MinZ team. Special thanks to all contributors and testers who helped identify and fix issues in this release.

## Download

Get MinZ v0.4.1 from:
- GitHub Releases: https://github.com/oisee/minz-ts/releases/tag/v0.4.1
- Direct downloads available for all platforms

---

*MinZ - Bringing modern language features to classic Z80 hardware*