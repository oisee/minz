# MinZ v0.6.0 "Zero-Cost Interfaces" Release Package

🎉 **Welcome to the Zero-Cost Interfaces release!**

## Quick Start

1. **Run the compiler:**
   ```bash
   ./minzc test_interface_simple.minz -o output.a80
   ```

2. **Try the examples:**
   - `test_interface_simple.minz` - Basic interface usage
   - `test_interface_complete.minz` - Comprehensive example with multiple interfaces

3. **View generated assembly:**
   - `test_interface_simple.a80` - See zero-cost interface calls
   - `test_interface_complete.a80` - Advanced interface usage

## Interface System Overview

```minz
// Define an interface
interface Printable {
    fun print(self) -> void;
}

// Implement for any type  
impl Printable for Point {
    fun print(self) -> void {
        @print("Point(");
        u8.print(self.x);
        @print(",");
        u8.print(self.y); 
        @print(")");
    }
}

// Call with beautiful syntax
Point.print(myPoint);  // Zero-cost!
```

## Key Benefits

- ✅ **Zero runtime overhead** - Direct function calls
- ✅ **Compile-time type safety** - All checks at build time
- ✅ **Beautiful syntax** - Clear and explicit
- ✅ **Perfect for embedded** - No memory or performance cost

## Documentation

- **`042_Zero_Cost_Interfaces.md`** - Complete technical specification
- **`RELEASE_NOTES.md`** - Full release notes and examples
- **Generated `.a80` files** - See the optimal assembly output

## What Makes This Special

Unlike traditional interfaces in Java/C#/Go that use vtables and dynamic dispatch, MinZ interfaces are **completely resolved at compile time** with **zero runtime cost**. Each `Type.method()` call compiles to a direct function call with optimal Z80 assembly generation.

**This is the future of systems programming interfaces!** 🚀

---

**MinZ v0.6.0 - Zero-cost abstractions that actually work on retro hardware.**