# MinZ v0.4.2 Release Notes - "Foundation Complete" 🎉

**Release Date**: July 29, 2025

## 🌟 Highlights

MinZ v0.4.2 represents a major milestone in the compiler's evolution. With 8 new language features and critical bug fixes, MinZ can now compile 58.3% of example programs - up from 46.7% in the previous release.

## 📊 Compilation Statistics

- **Success Rate**: 70/120 files (58.3%)
- **Improvement**: +14 files (+25% relative improvement)
- **New Features**: 8 major additions
- **Bug Fixes**: 4 critical issues resolved

## ✨ New Features

### 1. Built-in Functions
MinZ now provides essential functions as compiler built-ins for optimal performance:
- `print(ch: u8)` - Output character (RST 16 on ZX Spectrum)
- `len(arr: array) -> u16` - Get array length
- `memcpy(dst: *u8, src: *u8, size: u16)` - Copy memory
- `memset(dst: *u8, value: u8, size: u16)` - Set memory

### 2. Type Casting with `as` Operator
```minz
let byte: u8 = 42;
let word: u16 = byte as u16;  // Explicit widening
let narrow: u8 = word as u8;   // Explicit narrowing
```

### 3. Function Pointers
```minz
fun callback() -> void { }
let fn_ptr: *void = &callback;  // Take function address
```

### 4. Mutable Variable Declarations
```minz
let mut counter: u8 = 0;  // Explicitly mutable
counter = counter + 1;    // Allowed
```

### 5. Pointer Dereference Assignment
```minz
let ptr: *u8 = &value;
*ptr = 42;  // Assign through pointer
```

### 6. Unary Minus Operator
```minz
let positive: i8 = 10;
let negative: i8 = -positive;  // Negation works
```

### 7. Enhanced Address Calculation
- Local arrays now have correct memory addresses
- `&array[0]` returns the actual array address
- No more placeholder addresses in generated code

### 8. Inline Assembly Expressions
```minz
asm("ld hl, {0}" : : "r"(value));
```

## 🐛 Bug Fixes

1. **Cast Expression Parser** - Fixed S-expression parser creating incorrect binary expressions
2. **Parser Directory Detection** - Resolved "No language found" errors
3. **Function Symbol Handling** - Fixed nil pointer dereference when taking function addresses
4. **IR Debug Output** - Added missing String() methods for opcodes 45, 47, 50

## 🔧 Technical Improvements

### Parser Enhancements
- Tree-sitter grammar updated with `mut` keyword support
- S-expression parser properly handles cast expressions
- Improved error recovery and directory detection

### Type System
- Enhanced type compatibility between arrays and pointers
- Proper type inference for cast expressions
- Function types now fully supported

### Code Generation
- Optimized built-in functions to single instructions where possible
- Correct address calculation for all variable types
- Improved register allocation for complex expressions

## 📝 Example Programs

### Using Built-in Functions
```minz
fun print_string(str: *u8, len: u8) {
    let i: u8 = 0;
    while i < len {
        print(str[i]);
        i = i + 1;
    }
}
```

### Function Pointers for Callbacks
```minz
fun process_data(data: *u8, size: u16, callback: *void) {
    // Process data...
    // Call the callback when done
}

fun on_complete() {
    print('D');
    print('o');
    print('n');
    print('e');
}

fun main() {
    let data: [100]u8;
    process_data(&data[0], 100, &on_complete);
}
```

## 🚀 Getting Started

```bash
# Compile a MinZ program
./minzc program.minz -o program.a80

# With optimizations
./minzc program.minz -O -o program.a80

# Enable all features
./minzc program.minz -O --enable-smc --enable-true-smc -o program.a80
```

## 📚 Documentation

See the included documentation:
- `docs/082_MinZ_v0.4.2_Milestone_Reflection.md` - Detailed development journey
- `docs/081_MinZ_v0.4.1_Session_Progress.md` - Technical implementation details
- `README.md` - Language reference and examples

## 🙏 Acknowledgments

This release represents significant progress in making MinZ a practical language for Z80 development. Thank you to everyone following the project and providing feedback.

## 🔮 Next Steps

- Bit struct implementation for hardware register modeling
- Lua metaprogramming for compile-time code generation
- Match/case expressions for pattern matching
- Goal: 75% compilation success rate

---

**MinZ - Modern Systems Programming for Classic Hardware**
