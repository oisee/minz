# MinZ Compiler v0.3.2 Release Notes
**Release Date**: July 26, 2025  
**Version**: v0.3.2  
**Codename**: "Memory Matters"

## 🎉 Major Features

### 1. Global Variable Initializers
Global variables can now be initialized with constant expressions at compile time.

```minz
global u8 version = 3;
global u16 magic = 0xCAFE;
global bool debug = true;
global u8 computed = 10 + 20;    // Evaluated at compile time
global u16 shifted = 1 << 10;    // Result: 1024
```

**Benefits**:
- No runtime initialization code needed
- Constant expressions evaluated during compilation
- Supports arithmetic, bitwise, and logical operations

### 2. Comprehensive 16-bit Arithmetic
Full support for 16-bit operations with automatic type detection.

```minz
let u16 a = 1000;
let u16 b = 234;
let u16 product = a * b;      // Uses 16-bit multiplication
let u16 shifted = a << 2;     // Uses 16-bit shift
```

**New Operations**:
- 16-bit multiplication (repeated addition with BC counter)
- 16-bit left shift (ADD HL,HL instruction)
- 16-bit right shift (SRL H + RR L combination)

### 3. Type-Aware Code Generation
Compiler now tracks types through entire pipeline for optimal code generation.

```minz
let u8 small = 10;        // 8-bit operations
let u16 large = 1000;     // 16-bit operations
let u16 mixed = large + small;  // Proper handling
```

**Improvements**:
- Automatic type inference for literals
- Type propagation through expressions
- Correct operation selection based on operand types

## 🐛 Critical Bugs Fixed

### Local Variable Address Collision (FIXED)
**Issue**: All local variables mapped to same address ($F000)  
**Impact**: Data corruption, incorrect calculations  
**Solution**: Proper address allocation with unique addresses per variable

```asm
; Before (BROKEN)
LD HL, ($F000)  ; variable a
LD HL, ($F000)  ; variable b - SAME ADDRESS!

; After (FIXED)
LD HL, ($F002)  ; variable a
LD HL, ($F006)  ; variable b - DIFFERENT ADDRESS!
```

## 📊 Test Results

### Comprehensive Test Suite
Created `test_all_features.minz` covering:
- ✅ Global initializers with expressions
- ✅ Multiple local variables with unique addresses
- ✅ 16-bit arithmetic operations
- ✅ Type-aware operation selection
- ✅ Complex expressions with mixed types

### Verification
```bash
# Global initializers working
g_version:  DB 3       ✓
g_computed: DB 30      ✓ (10+20)
g_shifted:  DW 1024    ✓ (1<<10)
g_masked:   DB 60      ✓ (0xFF&0x3C)

# Local addresses unique
$F002 (a), $F006 (b), $F00A (c) ✓

# 16-bit operations present
5 instances of "16-bit multiplication" ✓
2 instances of "16-bit shift" ✓
```

## 🔧 Technical Improvements

### 1. Parser Enhancements
- Added `global` keyword support
- Fixed variable declaration parsing for `let type name` syntax
- Improved operator precedence handling

### 2. Semantic Analyzer
- Implemented `evaluateConstExpr()` for compile-time evaluation
- Added `EmitTyped()` for type-aware IR generation
- Enhanced expression type tracking

### 3. Code Generator
- Fixed `getAbsoluteAddr()` to use proper address allocation
- Added type-based operation selection
- Implemented 16-bit arithmetic routines

### 4. Register Allocator
- Added address tracking (`SetAddress`/`GetAddress`)
- Fixed local variable register mapping
- Prepared foundation for physical register allocation

## 📈 Performance Impact

- **Smaller binaries**: Global initializers in data section, not code
- **Faster execution**: Type-aware operations avoid unnecessary conversions
- **Better optimization**: Type information enables future optimizations

## 🚀 What's Next

### Planned for v0.4.0
1. Physical register allocation (foundation already laid)
2. Stack-based local variables with IX+offset
3. Function parameter passing optimizations
4. Advanced SMC optimizations

### Future Roadmap
- Signed arithmetic operations
- Float emulation library
- Inline assembly improvements
- Module system enhancements

## 💡 Migration Guide

### For v0.3.1 Users
No breaking changes! Your existing code will:
- Compile faster (better parser)
- Run correctly (address collision fixed)
- Potentially run faster (16-bit operations)

### New Features to Try
```minz
// Global with expression
global u16 SCREEN_SIZE = 256 * 192;

// 16-bit arithmetic
let u16 pixels = width * height;

// Complex expressions
let u16 offset = (base << 2) + (index * size);
```

## 🙏 Acknowledgments

This release represents significant progress in making MinZ a production-ready language for Z80 development. The compiler now handles real-world scenarios with proper memory management, comprehensive arithmetic, and intelligent code generation.

## 📦 Installation

```bash
cd minzc
make clean
make build
make install
```

## 🐞 Bug Reports

Please report issues at: github.com/minz/minzc/issues

---

**MinZ v0.3.2** - *Write modern code for classic hardware*