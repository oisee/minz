# MinZ Backend Status Report

Generated: 2025-08-06

## Overview

The MinZ compiler currently supports 8 backend targets with varying levels of completeness.

## Backend Status Summary

| Backend | Code Gen | Binary Gen | Key Issues | Priority |
|---------|----------|------------|------------|----------|
| **Z80** | ✅ Works | ❌ sjasmplus | Duplicate labels | HIGH |
| **6502** | ✅ Works | ⚠️ No assembler | Zero-page SMC ready | HIGH |
| **68000** | ✅ Works | ⚠️ No assembler | Complete | MEDIUM |
| **i8080** | ⚠️ Partial | ⚠️ No assembler | Missing LOAD_INDEX | LOW |
| **GB** | ✅ Works | ⚠️ No assembler | Complete | LOW |
| **C** | ⚠️ Partial | ✅ gcc works | Missing LOAD_INDEX | HIGH |
| **LLVM** | ✅ Works | ❌ Syntax errors | Invalid IR syntax | MEDIUM |
| **WASM** | ✅ Works | ❌ wat2wasm | Global var syntax | MEDIUM |

## Detailed Issues

### Z80 Backend
- **Status**: Code generation works perfectly
- **Issue**: sjasmplus reports duplicate label errors
- **Root Cause**: Label generation for loops creates duplicates
- **Fix**: Add function-scoped label prefixes

### 6502 Backend
- **Status**: Code generation works, zero-page SMC optimization complete
- **Issue**: No assembler integration
- **Solution**: Add ca65 or custom assembler support

### C Backend
- **Status**: Basic programs work
- **Issue**: Missing LOAD_INDEX operation for array access
- **Fix**: Implement array indexing in C backend

### LLVM Backend
- **Status**: Generates IR but with syntax errors
- **Issue**: Invalid LLVM IR syntax (missing types, wrong format)
- **Example**: `%1 = load %x` should be `%1 = load i32, i32* %x`

### WebAssembly Backend
- **Status**: Generates WAT format
- **Issue**: Global variable declarations have wrong syntax
- **Fix**: Update global syntax to match WAT spec

## Test Results

Based on comprehensive testing with 4 test programs:

1. **arrays.minz** - Tests array declaration and indexing
2. **basic_math.minz** - Tests arithmetic operations
3. **control_flow.minz** - Tests if/else and loops
4. **function_calls.minz** - Tests function declarations and calls

### Success Matrix

| Test | Z80 | 6502 | 68000 | i8080 | GB | C | LLVM | WASM |
|------|-----|------|-------|-------|----|---|------|------|
| arrays | ✅ | ✅ | ✅ | ❌ | ✅ | ❌ | ✅ | ✅ |
| basic_math | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| control_flow | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| function_calls | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

## Backend Development Toolkit

We've created a comprehensive backend development toolkit:

### Tools Available

1. **backend-devkit** CLI tool:
   ```bash
   backend-devkit -action=create -backend=arm -desc="ARM processor"
   backend-devkit -action=test -backend=z80
   backend-devkit -action=validate -backend=6502
   backend-devkit -action=doc -backend=68000
   ```

2. **BackendToolkit** framework:
   - Pattern-based code generation
   - Common optimization patterns
   - Register allocation helpers
   - Debug instrumentation

3. **Test Suite Generator**:
   - Automatically creates test programs
   - Validates backend functionality
   - Generates expected output patterns

## Recommendations

### Immediate Fixes (This Week)
1. **Fix C backend LOAD_INDEX** - Critical for array support
2. **Fix Z80 duplicate labels** - Blocks production use
3. **Fix WASM global syntax** - Simple syntax fix

### Medium Priority (Next Sprint)
1. **Fix LLVM IR syntax** - Important for LLVM ecosystem
2. **Add assembler support** - For 6502, 68000, i8080, GB
3. **Complete 68000 backend** - Already mostly working

### Future Enhancements
1. **ARM backend** - Use toolkit to scaffold
2. **RISC-V backend** - Modern architecture
3. **x86 backend** - For native execution

## Using the Backend Toolkit

To add a new backend:

```go
toolkit := NewBackendBuilder().
    WithInstruction(ir.OpLoadConst, "li %reg%, %imm%").
    WithPattern("load", "lw %reg%, %addr%").
    WithPattern("store", "sw %reg%, %addr%").
    WithCallConvention("registers", "a0").
    Build()
```

The toolkit handles common patterns, letting developers focus on architecture-specific details.