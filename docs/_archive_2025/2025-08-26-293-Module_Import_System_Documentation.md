# MinZ Module Import System Documentation

## Overview
MinZ supports a hierarchical module system for organizing code into reusable packages.

## Import Syntax

### Basic Import
```minz
import std.io;
import zx.screen;
```

### Import with Alias
```minz
import std.math as m;
import hardware.spectrum as hw;
```

## Module Path Resolution

Modules are resolved using dot notation:
- `std` - Standard library modules
- `std.io` - I/O functions
- `std.math` - Math functions
- `zx.screen` - ZX Spectrum screen functions
- `hardware.spectrum` - Hardware-specific functions

## Using Imported Functions

### Direct Path Access
```minz
import zx.screen;

fun main() -> void {
    zx.screen.set_border(2);  // Full path
    zx.screen.clear();
}
```

### Aliased Access
```minz
import std.math as m;

fun calculate() -> u16 {
    return m.abs(-10);  // Using alias
}
```

## Built-in Modules

### std - Standard Library
- `std.io` - Input/output functions
  - `print_u8(value: u8)`
  - `print_u16(value: u16)`
  - `print(str: str)`
  
- `std.mem` - Memory functions
  - `memset(dest: *u8, value: u8, size: u16)`
  - `memcpy(dest: *u8, src: *u8, size: u16)`
  
- `std.math` - Math functions
  - `abs(value: i16) -> u16`
  - `min(a: u16, b: u16) -> u16`
  - `max(a: u16, b: u16) -> u16`

### zx - ZX Spectrum Specific
- `zx.screen` - Screen functions
  - `set_border(color: u8)`
  - `clear()`
  - `set_attr(x: u8, y: u8, attr: u8)`
  
- `zx.sound` - Sound functions
  - `beep(freq: u16, duration: u16)`

## Module Declaration

To create your own module, use file organization:

```
project/
├── main.minz
└── modules/
    └── mylib.minz
```

In `mylib.minz`:
```minz
// Module is implicitly named based on path
pub fun helper() -> u8 {
    return 42;
}
```

In `main.minz`:
```minz
import modules.mylib;

fun main() -> void {
    let x = modules.mylib.helper();
}
```

## Visibility

- `pub` - Public functions/types can be imported
- Default (no modifier) - Private to module

## Current Limitations

1. **No Wildcard Imports**: Cannot use `import std.io.*`
2. **No Selective Imports**: Cannot use `import std.io.{print, read}`
3. **Path-based Resolution**: Module paths must match file system structure
4. **No Circular Dependencies**: Modules cannot import each other circularly

## Implementation Status

✅ **Working**:
- Basic import statements
- Module path resolution
- Full path function calls (e.g., `std.io.print()`)
- Built-in module definitions

🚧 **In Progress**:
- Module aliasing
- Custom module creation
- Package management

## Examples

### Complete Example
```minz
import std.io;
import zx.screen;

fun draw_box(x: u8, y: u8, w: u8, h: u8) -> void {
    // Draw using ZX Spectrum screen functions
    zx.screen.set_attr(x, y, 0x47);  // White on black
    
    // Print size info
    std.io.print_u8(w);
    std.io.print_u8(h);
}

fun main() -> void {
    zx.screen.clear();
    zx.screen.set_border(1);  // Blue border
    draw_box(10, 10, 20, 10);
}
```

## Best Practices

1. **Use explicit paths** for clarity
2. **Group related imports** together
3. **Avoid deep nesting** (max 3 levels recommended)
4. **Use meaningful aliases** for long paths

## Future Enhancements

- Package manager integration
- Module versioning
- Dynamic module loading
- Export lists
- Re-exports