# MinZ v0.9.6 "Swift & Ruby Dreams" Release Notes 🎉

*Released: August 5, 2025*

## 🌟 Major Features

### Function Overloading ✨
MinZ now supports function overloading, bringing modern language elegance to 8-bit systems!

```minz
// Before: Ugly type suffixes
print_u8(42);
print_u16(1000);
print_bool(true);

// Now: Beautiful and natural!
print(42);        // Automatically resolves to print$u8
print(1000);      // Automatically resolves to print$u16  
print(true);      // Automatically resolves to print$bool
```

- Full name mangling system (`function$type1$type2$...`)
- Compile-time overload resolution
- Zero runtime overhead
- Works with all types including user-defined structs

### Interface Self Parameter Resolution 🎯
Interface methods now work perfectly with natural syntax!

```minz
interface Drawable {
    fun draw(self) -> void;
    fun get_area(self) -> u16;
}

impl Drawable for Circle {
    fun draw(self) -> void {
        // self is properly typed as Circle
        draw_circle_at(self.center, self.radius);
    }
}

// Natural method calls - just like Swift!
let circle = Circle { center: point, radius: 5 };
circle.draw();       // Compiles to: CALL Circle.draw$Circle
circle.get_area();   // No vtables, no overhead!
```

- Self parameters automatically get correct type
- Methods are properly mangled with self type
- Zero-cost dispatch - all resolved at compile time
- Multiple interface methods supported

### Developer Happiness Features 💝
Inspired by Ruby and Swift!

- Both `fn` and `fun` keywords work - choose your style!
- `global` keyword for Ruby-style global variables
- Natural method call syntax (`object.method()`)
- Clean error messages for overload resolution

## 🔧 Technical Improvements

### Compiler Enhancements
- Name mangling system for overloaded functions
- Improved type resolution in method calls
- Better AST type handling (TypeIdentifier support)
- Enhanced semantic analysis for impl blocks

### Bug Fixes
- Fixed interface method registration in impl blocks
- Resolved self parameter type issues
- Fixed overload set symbol lookup
- Corrected method call resolution for overloaded functions

## 📊 Impact

This release brings MinZ's feature completion to **70%**! The addition of function overloading and proper interface methods unlocks:

- **Modern Standard Library** - Clean APIs without type suffixes
- **Protocol-Oriented Programming** - Swift-style design patterns
- **Zero-Cost Abstractions** - All dispatch at compile time
- **Developer Joy** - Write beautiful code that compiles to perfect assembly

## 🚀 What's Now Possible

```minz
// Beautiful standard library
print(42);
print("Hello");
print(player);  // If Player implements Printable

// Natural collections (coming soon)
numbers.map(|x| x * 2)
       .filter(|x| x > 10)
       .forEach(print);

// Type-safe graphics
sprites.forEach(|s| s.draw(screen));
```

## 💔 Breaking Changes

None! This release is fully backward compatible.

## 🙏 Acknowledgments

This release was heavily inspired by:
- **Swift** - For showing us that protocols can be elegant
- **Ruby/Crystal** - For proving that developer happiness matters
- **Our Community** - For believing in modern languages on retro hardware

## 📚 Documentation

- [Interface & Overloading Revolution](docs/128_Interface_Overloading_Revolution.md)
- [Updated AI Crash Course](AI_COLLEAGUES_MINZ_CRASH_COURSE.md)
- [Updated Examples](examples/)

## 🎮 Get Started

```bash
# Install MinZ
curl -L https://github.com/your-repo/minz/releases/download/v0.9.6/minz-v0.9.6-$(uname -s)-$(uname -m).tar.gz | tar xz
sudo mv minz /usr/local/bin/

# Try the new features
minz examples/interface_simple.minz -o demo.a80
minz examples/overloading_demo.minz -o demo.a80
```

## 🌈 Next Steps

With these foundations, v0.10.0 will bring:
- Complete iterator system with DJNZ optimization
- Generic functions with monomorphization
- Extension methods for built-in types
- Pattern matching with exhaustiveness checking

---

*"The best of modern language design, running on your favorite 8-bit computer!"* 🚀