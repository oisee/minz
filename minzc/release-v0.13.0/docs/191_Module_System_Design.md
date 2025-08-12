# MinZ Module System Design

## 🎯 Design Goals

1. **Simple & Practical** - Get imports working quickly
2. **Zero-Cost** - No runtime overhead for module resolution
3. **Platform-Aware** - Support platform-specific modules
4. **Extensible** - Easy to add new modules

## 📦 Module Types

### 1. Built-in Modules (Compile-Time)
```minz
import std;           // Standard library
import zx.screen;     // ZX Spectrum screen
import zx.input;      // ZX Spectrum input
import zx.sound;      // AY-3-8912 sound chip
```

### 2. File-Based Modules (Future)
```minz
import "utils.minz";  // Local file
import "./lib/math";  // Relative path
```

## 🏗️ Implementation Strategy

### Phase 1: Built-in Module Registry (TODAY)
```go
// pkg/semantic/modules.go
type BuiltinModule struct {
    Name      string
    Functions map[string]*FuncSymbol
    Constants map[string]*ConstSymbol
    Types     map[string]*TypeSymbol
}

var builtinModules = map[string]*BuiltinModule{
    "std": &stdModule,
    "zx.screen": &zxScreenModule,
    "zx.input": &zxInputModule,
}
```

### Phase 2: Module Resolution
1. Parse `import` statement
2. Look up in built-in registry
3. Add symbols to scope with module prefix
4. Allow both qualified (`std.print`) and unqualified (`print`) access

## 📝 Standard Library Module

### `std` Module Functions
```minz
// Output functions
fun print(value: any) -> void;           // Polymorphic print
fun print_string(str: *u8) -> void;      // Print null-terminated string
fun println(value: any) -> void;         // Print with newline
fun cls() -> void;                       // Clear screen
fun hex(value: u8) -> void;             // Print as hex

// Memory functions  
fun memcpy(dest: *mut u8, src: *u8, size: u16) -> void;
fun memset(dest: *mut u8, value: u8, size: u16) -> void;
fun len(array: *any) -> u16;            // Array/string length

// Math functions
fun abs(value: i8) -> u8;
fun min(a: u8, b: u8) -> u8;
fun max(a: u8, b: u8) -> u8;
```

## 🖥️ Platform Modules

### `zx.screen` Module
```minz
const SCREEN_WIDTH = 256;
const SCREEN_HEIGHT = 192;
const ATTR_START = 0x5800;
const SCREEN_START = 0x4000;

fun set_pixel(x: u8, y: u8) -> void;
fun clear() -> void;
fun set_border(color: u8) -> void;
fun set_ink(color: u8) -> void;
fun set_paper(color: u8) -> void;
fun plot(x: u8, y: u8) -> void;
fun draw_line(x1: u8, y1: u8, x2: u8, y2: u8) -> void;
```

### `zx.input` Module
```minz
fun read_keyboard() -> u8;
fun wait_key() -> u8;
fun is_key_pressed(key: u8) -> bool;
```

## 🔧 Implementation Steps

1. **Update Parser** - Handle import statements properly
2. **Create Module Registry** - Built-in modules with their symbols
3. **Update Semantic Analyzer** - Resolve imports and add symbols
4. **Generate Runtime Code** - Emit actual implementations

## 💡 Key Insights

- Start with built-in modules only (no file loading yet)
- Each module is just a collection of pre-defined symbols
- Module functions can be implemented in:
  - Pure MIR/assembly (for low-level functions)
  - MinZ itself (for high-level functions)
  - Runtime library (linked at assembly time)

## 🚀 Benefits

1. **Immediate Impact** - Fixes ~40% of failing examples
2. **Clean Namespace** - No more `print_u8`, just `print`
3. **Platform Abstraction** - Same code works on different platforms
4. **Future-Proof** - Easy to extend with file-based modules later