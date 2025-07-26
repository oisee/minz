# MinZ Compiler Progress Update: Global Initializers & 16-bit Arithmetic
**Date**: July 26, 2025  
**Version**: v0.3.1 → v0.3.2  
**Author**: MinZ Development Team

## Executive Summary

This update documents the implementation of three major features:
1. Global variable initializers with constant expression evaluation
2. Comprehensive 16-bit arithmetic operations  
3. Foundation for physical Z80 register allocation

## Features Implemented

### 1. Global Variable Initializers ✓

**Capability**: Global variables can now be initialized with constant expressions
```minz
global u8 g_byte = 42;
global u16 g_word = 0x1234;
global bool g_flag = true;
global u8 g_calc = 10 + 5;  // Constant expression
global u16 g_shift = 1 << 8; // Bit operations
```

**Implementation Details**:
- Added `global` keyword to parser (`simple_parser.go`)
- Implemented `evaluateConstExpr()` for compile-time evaluation
- Enhanced `generateGlobal()` to emit proper DB/DW directives
- Supports: literals, arithmetic, bitwise, and unary operations

**Generated Assembly**:
```asm
g_byte:
    DB 42
g_word:
    DW 4660    ; 0x1234
g_calc:
    DB 15      ; 10 + 5 evaluated at compile time
```

### 2. 16-bit Arithmetic Operations ✓

**New Operations**:
- **16-bit Multiplication**: Using repeated addition with BC counter
- **16-bit Shift Left**: Using ADD HL,HL instruction
- **16-bit Shift Right**: Using SRL H + RR L combination

**Example Code**:
```minz
let u16 a = 100;
let u16 b = 200;
let u16 product = a * b;  // 16-bit multiplication
let u16 shifted = a << 4;  // 16-bit shift
```

**Generated Assembly (16-bit multiply)**:
```asm
; 16-bit multiplication
LD (mul_src1_0), HL  ; Save multiplicand
LD (mul_src2_0), HL  ; Save multiplier
LD HL, 0             ; Result = 0
LD DE, (mul_src1_0)  ; DE = multiplicand
LD BC, (mul_src2_0)  ; BC = multiplier
.mul16_loop_0:
    ADD HL, DE       ; Result += multiplicand
    DEC BC
    LD A, B
    OR C
    JR NZ, .mul16_loop_0
```

### 3. Register Allocator V2 (Foundation) 🚧

**Design**: Physical Z80 register mapping
- Maps virtual registers → physical registers (A, B, C, D, E, H, L)
- Supports register pairs (BC, DE, HL)
- Spill mechanism when out of registers
- Created `register_allocator_v2.go` (not yet integrated)

## Critical Issues Discovered

### 1. Local Variable Address Collision 🔴
**Problem**: All local variables use the same address ($F000)
```asm
; BUG: All loads from same address
r5 = load a     ; LD HL, ($F000)
r6 = load b     ; LD HL, ($F000)  <-- WRONG!
r15 = load c    ; LD HL, ($F000)  <-- WRONG!
```

**Root Cause Analysis**:
1. `analyzeVarDeclInFunc()` allocates registers but doesn't assign memory addresses
2. `getLocalAddr()` returns register 0 for all untracked locals
3. Register 0 maps to $F000, causing collision

**Impact**: All local variables share same memory location, corrupting data

### 2. Missing Type Information in IR 🟡
**Problem**: IR instructions lack type information for proper 8/16-bit selection
```go
// Current: No type info
irFunc.Emit(op, resultReg, leftReg, rightReg)

// Needed: Type-aware emit
irFunc.EmitTyped(op, resultReg, leftReg, rightReg, resultType)
```

**Impact**: 
- 16-bit variables use 8-bit operations
- Incorrect shift implementations
- Wrong multiplication routines

### 3. Parser Variable Declaration Bug 🟡
**Problem**: Local variables parsed as globals in some cases
- `let u16 a = 100` inside function → treated as global
- Missing scope tracking in parser

## Test Results

### Global Initializers Test ✅
```
g_byte:    DB 42     ✓
g_word:    DW 4660   ✓ (0x1234)
g_flag:    DB 1      ✓ (true)
g_calc:    DB 15     ✓ (10 + 5)
g_shift:   DW 256    ✓ (1 << 8)
g_masked:  DB 15     ✓ (0xFF & 0x0F)
```

### 16-bit Arithmetic Test ⚠️
- Addition/Subtraction: ✓ Working
- Multiplication: ⚠️ Using 8-bit routine (type info missing)
- Shifts: ✓ Implemented but need type propagation
- Bitwise: ✓ Working for 16-bit

## Fixes Required

### Priority 1: Fix Local Variable Addressing
1. Implement proper local variable memory allocation
2. Track local addresses in semantic analyzer
3. Update `getLocalAddr()` to return correct addresses

### Priority 2: Add Type Information to IR
1. Add Type field usage in `Emit()` functions
2. Propagate types through expression analysis
3. Use type info in code generator for operation selection

### Priority 3: Fix Parser Scope Issues
1. Ensure local variables stay local
2. Fix variable declaration parsing inside functions
3. Add proper scope tracking

## Code Changes Summary

### Files Modified:
1. `simple_parser.go`: Added global keyword and parsing
2. `semantic/analyzer.go`: Added `evaluateConstExpr()` for initializers
3. `codegen/z80.go`: 
   - Enhanced `generateGlobal()` for initializers
   - Added 16-bit multiplication
   - Added 16-bit shift operations
4. `ir/ir.go`: Global struct already had Init field

### New Files:
1. `register_allocator_v2.go`: Physical register allocator (not integrated)

## Next Steps

1. **Immediate**: Fix local variable addressing bug
2. **High Priority**: Implement type propagation in IR
3. **Medium Priority**: Integrate physical register allocator
4. **Future**: Optimize 16-bit operations (strength reduction, special cases)

## Version Notes

This work positions MinZ for v0.3.2 release after critical fixes. The compiler now supports modern features expected in a systems language while maintaining Z80 optimization focus.