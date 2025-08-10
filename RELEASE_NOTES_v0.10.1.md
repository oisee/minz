# MinZ v0.10.1: Professional Toolchain Evolution 🛠️

*August 10, 2025*

## The Professional Compiler Suite You Deserve

MinZ v0.10.1 transforms our compiler toolkit into a **professional-grade development environment** with standardized CLI interfaces, proper architecture documentation, and powerful new language features.

## 🎯 Highlights

### Standardized CLI Experience
Every MinZ tool now follows Unix/POSIX conventions perfectly:
```bash
mza -o output.bin input.a80      # Short options
mza --output=output.bin input.a80 # Long options
mza -vc input.a80                 # Combined short options
```

### Real Enums & Logical Operators
```minz
enum Status { Ready, Running, Done }

if (status == Status.Ready && count > 0) {
    // Now with proper && and || operators!
}
```

### Array Literals
```minz
let numbers: [u8; 5] = [1, 2, 3, 4, 5];  // Finally!
```

### Architecture Decision Records
Professional engineering practices with documented decisions in `/adr`:
- Why we chose Cobra for CLI
- How platform independence works
- Character literal implementation
- And more!

## 💔 Breaking Changes

### CLI Option Changes
If you have scripts using the old options, update them:

**mza (assembler)**:
- `-undoc` → `-u` or `--undocumented`
- `-case` → `-c` or `--case-sensitive`

**mze (emulator)**:
- Removed duplicate `-target` flag (use `-t` or `--target`)

### Type Renames
- `str` → `String` (MinZ native strings)
- `*str` → `*u8` (C-style strings)

## 🚀 Quick Migration Guide

### Update Your Scripts
```bash
# Old
mza -undoc program.a80

# New
mza -u program.a80
# or
mza --undocumented program.a80
```

### Update Your Code
```minz
// Old
let name: str = "MinZ";
let cstr: *str = "Hello";

// New  
let name: String = "MinZ";
let cstr: *u8 = "Hello";
```

## 📦 What's New

### Language Features
- ✅ **Enum support** with proper variant checking
- ✅ **Logical operators** `&&` and `||` with short-circuit evaluation
- ✅ **Array literals** `[1, 2, 3]` syntax
- ✅ **Fixed string types** for clarity

### Toolchain Improvements
- ✅ **Standardized CLI** across all tools
- ✅ **Better help text** with examples
- ✅ **Consistent option naming**
- ✅ **Professional documentation**

### Documentation
- ✅ **5 Architecture Decision Records**
- ✅ **CLI standards in CONTRIBUTING.md**
- ✅ **Enhanced README** with tool overview
- ✅ **Platform independence guide**

## 📊 By The Numbers

- **3 tools** standardized with Cobra
- **5 ADRs** documenting decisions
- **4 major** language features added
- **70%** test success rate maintained
- **20+ commits** of improvements

## 🔧 Installation

```bash
# Get the latest
git checkout v0.10.1

# Build and install
cd minzc
make install-user  # No sudo needed!

# Verify
mz --version  # Should show v0.10.1
```

## 🎮 Try It Out

```minz
// New enum support
enum Color { Red, Green, Blue }

// New logical operators
fun is_valid(x: u8, y: u8) -> bool {
    return x > 0 && y < 100 || x == 255;
}

// New array literals
let data: [u8; 3] = [0xFF, 0x00, 0xAA];

// It all just works!
fun main() -> void {
    let color = Color.Blue;
    if is_valid(42, 50) {
        @print("Valid!");
    }
}
```

## 🔮 What's Next

- **v0.11.0**: Module system implementation
- **v0.12.0**: Standard library expansion
- **v1.0.0**: Production readiness!

## 💬 Community

Found a bug? Have a suggestion? 
- Open an issue on GitHub
- Check our new ADRs for design decisions
- Read CONTRIBUTING.md for guidelines

## 🙏 Thank You!

To everyone who contributed code, reported issues, or provided feedback - this release is for you! MinZ is becoming the professional language that retro systems deserve.

---

**MinZ v0.10.1**: *Where professional meets retro* 🚀

*Download now and experience the evolution!*