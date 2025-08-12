# MinZ v0.13.0: Module Revolution! 📦

**Release Date:** January 12, 2025  
**Theme:** Complete Module System with Zero-Cost Imports

## 🎉 Major Features

### 1. **Complete Module System** ✨
MinZ now has a full module system with both built-in and file-based modules!

```minz
// Import built-in modules
import std;
import zx.screen;
import zx.input;
import zx.sound;

// Import file-based modules
import math;  // Loads from stdlib/math.minz
```

### 2. **Module Aliasing** 🏷️
Import modules with custom names for cleaner code:

```minz
import std as io;           // Alias standard library
import math as m;           // Short alias
import zx.screen as display; // Descriptive alias
import zx.input as kbd;     // Keyboard alias

fun main() -> void {
    io.cls();                    // Instead of std.cls()
    let sq = m.square(5);        // Instead of math.square()
    display.set_border(2);       // Instead of zx.screen.set_border()
    let key = kbd.wait_key();    // Instead of zx.input.wait_key()
}
```

### 3. **File-Based Module Loading** 📁
Create your own modules as `.minz` files:

```minz
// stdlib/math.minz
pub fun square(x: u8) -> u16 {
    return x * x;
}

pub fun gcd(a: u8, b: u8) -> u8 {
    while b != 0 {
        let temp = b;
        b = a % b;
        a = temp;
    }
    return a;
}

pub const PI_TIMES_100: u8 = 141;
```

### 4. **Platform-Specific Modules** 🖥️
ZX Spectrum modules with complete functionality:

```minz
// Screen control
import zx.screen as scr;
scr.set_border(3);
scr.set_ink(7);
scr.set_paper(0);

// Input handling
import zx.input as input;
let key = input.wait_key();
if input.is_key_pressed(32) { /* space pressed */ }

// Sound generation
import zx.sound as audio;
audio.beep(100, 440);  // Duration, pitch
audio.click();         // Quick click sound
```

## 📊 Compilation Success Metrics

**Before v0.13.0:** 69% success rate (103/148 examples)  
**After v0.13.0:** ~85% success rate (126/148 examples)

Import-related failures reduced from ~40% to 0%!

## 🚀 Zero-Cost Abstractions

The module system maintains MinZ's philosophy:
- **Compile-time resolution** - No runtime module loading
- **Direct function calls** - No indirection or vtables
- **Zero overhead** - Modules compile to same code as inline functions
- **Tree-shaking** - Only used functions included in output

## 💻 Example: Complete Module Usage

```minz
// Comprehensive module demonstration
import std;                    // Standard library
import math as m;              // Math with alias
import zx.screen as display;   // Platform graphics
import zx.input as kbd;        // Input handling
import zx.sound;              // Sound (no alias)

fun main() -> void {
    std.cls();
    std.println("Module System Demo!");
    
    // Math module
    let x: u8 = 12;
    std.print("Square of 12 = ");
    std.println(m.square(x));
    
    // Platform modules
    display.set_border(2);
    display.set_ink(7);
    
    // Input handling
    std.print("Press any key...");
    let key = kbd.wait_key();
    
    // Sound
    zx.sound.beep(50, 880);  // A5 note
    
    std.println("Done!");
}
```

## 🛠️ Technical Details

### Module Resolution Order
1. Check built-in modules (`std`, `zx.*`)
2. Search `stdlib/` directory
3. Search current directory
4. Search paths in `MINZ_PATH` environment variable

### Built-in Modules
- `std` - Standard library (print, cls, memory functions)
- `zx.screen` - ZX Spectrum screen control
- `zx.input` - Keyboard and input handling
- `zx.sound` - Sound generation
- `math` - Mathematical functions (file-based)

### Module Declaration Syntax
```minz
// Public functions (exported)
pub fun my_function() -> void { }

// Public constants
pub const MAX_VALUE: u8 = 255;

// Public variables
pub global counter: u16 = 0;

// Private (module-only) items
fun internal_helper() -> void { }
```

## 🐛 Bug Fixes
- Fixed module-prefixed builtin function calls
- Added nil type checking for polymorphic functions
- Fixed mangling of module function names
- Improved module symbol registration

## 🔧 Infrastructure
- Complete module loader system (`pkg/semantic/module_loader.go`)
- Module registry with alias support
- Automatic standard library path detection
- Module caching to prevent duplicate loading

## 📈 Next Steps
- Expand standard library modules
- Add string manipulation module
- Implement file I/O module
- Create collections module (lists, maps)
- Design package management system

## 🎊 Summary

MinZ v0.13.0 delivers a **complete, pragmatic module system** that maintains our core philosophy of zero-cost abstractions. Import modules, use aliases, create your own - all with **zero runtime overhead**!

This release improves compilation success from 69% to ~85%, making MinZ significantly more usable for real projects. The module system provides the foundation for a rich ecosystem of reusable code.

**MinZ: Modern modularity meets vintage performance!** 🚀

---

*Try it:* `mz --version` → MinZ v0.13.0  
*Import anything:* `import math as m;` → Zero-cost modules!