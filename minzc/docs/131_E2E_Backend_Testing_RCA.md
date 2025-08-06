# E2E Backend Testing - Root Cause Analysis

## Executive Summary

E2E backend testing revealed that 7/8 backends successfully generate output, but deeper analysis shows a critical issue affecting multiple backends: **empty variable names in STORE_VAR MIR instructions**.

## Test Results Overview

### ✅ Successful Code Generation (7/8)
- **Z80** - Generates .a80 assembly
- **6502** - Generates .s assembly  
- **68000** - Generates .s assembly
- **i8080** - Generates .asm assembly
- **Game Boy** - Generates .gb.s assembly
- **C** - Generates .c and compiles to binary
- **LLVM** - Generates .ll (but with errors)

### ❌ Failed Code Generation (1/8)
- **WebAssembly** - Fails with undefined global variables

## Root Cause Analysis

### Primary Issue: Empty Variable Names in MIR

The core problem is in the MIR generation phase, not the backends themselves. When analyzing the generated MIR:

```mir
; store , r2      # Variable name is missing!
; store , r4      # Should be: store x, r2
; store , r8      # Should be: store sum, r8
```

This manifests differently in each backend:

#### 1. Assembly Backends (Z80, 6502, 68000, i8080, GB)
**Status**: ✅ Generate valid assembly  
**Why they work**: Assembly backends use absolute memory addresses instead of variable names
```asm
LD ($F002), A    ; Works - uses address, not name
```

#### 2. C Backend
**Status**: ⚠️ Generates compilable code but incorrect behavior
**Issue**: Skips stores with empty names
```c
r2 = 42;
// Skipping store to empty variable name
r6 = x;  // Loads uninitialized value (0)
```
**Result**: Program outputs 0 instead of 52

#### 3. LLVM Backend  
**Status**: ❌ Generates invalid LLVM IR
**Issue**: Invalid store instructions
```llvm
store i8 %r2, i8* %.addr    ; Missing variable name
```
**Result**: LLVM compilation fails

#### 4. WebAssembly Backend
**Status**: ❌ Generates invalid WAT
**Issue**: Undefined global variables
```wat
global.set $     ; Empty variable name
global.get $x    ; Variable never defined
```
**Result**: WAT to WASM conversion fails

## Why This Happens

Looking at the test program:
```minz
fun main() -> void {
    let x: u8 = 42;
    let y: u8 = 10;
    let sum: u8 = x + y;
}
```

The semantic analyzer should generate:
1. `LOAD_CONST 42` → r2
2. `STORE_VAR x, r2` ← **Variable name missing here**
3. `LOAD_CONST 10` → r4  
4. `STORE_VAR y, r4` ← **Variable name missing here**
5. `LOAD_VAR x` → r6
6. `LOAD_VAR y` → r7
7. `ADD r6, r7` → r8
8. `STORE_VAR sum, r8` ← **Variable name missing here**

The issue is in `pkg/semantic/analyzer.go` where local variable assignments generate STORE_VAR instructions without properly setting the Symbol field.

## Impact Analysis

### Severity by Backend

1. **Low Impact** (Assembly backends): Work correctly due to address-based approach
2. **Medium Impact** (C backend): Compiles but produces wrong results
3. **High Impact** (LLVM, WebAssembly): Cannot generate valid code

### Code Quality Impact

- **Data Loss**: Variable assignments are silently dropped
- **Debugging Difficulty**: No clear error messages about missing names
- **Cross-Platform Issues**: Code that works on Z80 fails on modern platforms

## Verification

To confirm this is the root cause:

```bash
# Check MIR for empty symbols
./mz test.minz -o test.mir -d
grep "store ," test.mir  # Shows empty variable names

# Check C backend behavior
./mz test.minz -b c -o test.c
grep "Skipping store" test.c  # Shows skipped stores

# Check LLVM IR
./mz test.minz -b llvm -o test.ll  
grep "%.addr" test.ll  # Shows malformed addresses
```

## Solution Path

### Immediate Fix
In `pkg/semantic/analyzer.go`, ensure STORE_VAR instructions have Symbol set:
```go
inst := ir.Instruction{
    Op:     ir.OpStoreVar,
    Src1:   valueReg,
    Symbol: varName,  // This is missing!
    Type:   varType,
}
```

### Long-term Improvements

1. **MIR Validation**: Add checks for empty symbols in STORE_VAR/LOAD_VAR
2. **Backend Assertions**: Backends should error on missing variable names
3. **Integration Tests**: Test variable assignment across all backends
4. **Debug Output**: Better MIR debugging to catch these issues early

## Conclusion

The E2E testing successfully revealed a critical bug in MIR generation that affects high-level backends. While assembly backends mask the issue through their low-level nature, modern backends (LLVM, WebAssembly, C) expose the problem clearly.

**Priority**: HIGH - This blocks proper cross-platform compilation and affects the reliability of the entire compiler.

**Next Steps**:
1. Fix STORE_VAR symbol generation in semantic analyzer
2. Add MIR validation pass
3. Re-run E2E tests to verify fix
4. Add regression tests for variable assignments