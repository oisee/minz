MinZ v0.4.1 brings significant improvements in compiler stability and feature completeness. With a 55% compilation success rate and new built-in functions, MinZ is becoming increasingly practical for Z80 development.

## Highlights
- 🎯 **Built-in Functions**: `print()`, `len()`, `memcpy()`, `memset()` - compile to optimal Z80 code
- ✨ **Language Features**: Mutable variables (`let mut`), pointer dereference assignment
- 📈 **55% Compilation Success**: Up from 46.7% in v0.4.0-alpha
- 🔧 **Parser Improvements**: Better error messages and tree-sitter integration

## Performance Improvements
- `print()`: 2.3x faster than function calls
- `len()`: 2.9x faster with compile-time optimization
- `memcpy()`: 2.1x faster than manual loops

## New Language Features

### Built-in Functions
```minz
// Direct ROM call for printing
print('A');  

// Compile-time array length
let arr: [10]u8;
let size = len(arr);  // Returns 10

// Optimized memory operations
memcpy(dest, src, 100);
memset(buffer, 0, 256);
```

### Mutable Variables
```minz
let mut x = 10;
x = 20;  // Now allowed

let ptr = &mut x;
*ptr = 30;  // Pointer dereference assignment works
```

## What's Fixed
- Parser directory detection issues
- All IR opcodes now properly displayed
- Pointer dereference assignment
- Array-to-pointer type conversions
- Unary minus operator

## Platform Support
- macOS (Intel & Apple Silicon)
- Linux (x64 & ARM64)
- Windows (x64)

## Installation
```bash
# Download for your platform (example: macOS ARM64)
curl -L https://github.com/oisee/minz-ts/releases/download/v0.4.1/minzc-darwin-arm64.tar.gz | tar xz
chmod +x minzc-darwin-arm64
sudo mv minzc-darwin-arm64 /usr/local/bin/minzc
```

See the [full release notes](https://github.com/oisee/minz-ts/blob/main/minzc/release-v0.4.1/RELEASE_NOTES_v0.4.1.md) for detailed information.

This release maintains full compatibility with v0.4.0-alpha while adding practical improvements for everyday use.
