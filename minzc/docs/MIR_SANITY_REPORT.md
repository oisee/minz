# MIR Sanity Check Report

**Date:** 2026-01-09
**Status:** FIXED (Updated 2026-01-09)
**Author:** Claude Code

## Executive Summary

The MIR (MinZ Intermediate Representation) layer has been analyzed and **fixed**. The VM now correctly handles arrays, structs, pointers, and function parameters.

**Status:** MIR round-trip (Semantic → String() → Parse → Execute) now works for all supported operations. 32 tests pass.

## Architecture Overview

```
MinZ Source
    ↓
Semantic Analyzer (generates 79 opcodes)
    ↓
MIR Backend (calls inst.String() for text output)
    ↓
MIR Parser (pattern matching, ~50 opcodes)
    ↓
MIR VM (executes ~60 opcodes)
    ↓
Output (PNG via syscalls)
```

## Coverage Analysis

### By Numbers

| Stage | Opcodes Handled | Percentage |
|-------|-----------------|------------|
| Semantic Analyzer generates | 79 | 100% |
| String() can serialize | ~65 | 82% |
| MIR Parser can parse | ~50 | 63% |
| MIR VM can execute | ~60 | 76% |

### Effective Coverage

For **simple programs** (loops, arithmetic, variables, syscalls): **100% working**

For **arrays, structs, pointers, params**: **FIXED - working**

## Detailed Findings

### 1. Interface Opcodes - NOT NEEDED

| Opcode | Usage in Semantic | Verdict |
|--------|-------------------|---------|
| OpInterfaceCall | 0 | Dead code |
| OpMethodDispatch | 0 | Dead code |
| OpCastInterface | 0 | Dead code |
| OpCheckCast | 0 | Dead code |

**Reason:** MinZ uses monomorphization and zero-cost interfaces (direct calls, no vtables).

**Recommendation:** Remove from ir.go or mark as deprecated.

### 2. Array Opcodes - FIXED

| Opcode | String() Output | Parser | VM |
|--------|-----------------|--------|-----|
| OpLoadIndex | `r%d = r%d[r%d]` | ✅ | ✅ |
| OpStoreIndex | `r%d[r%d] = r%d` | ✅ | ✅ |
| OpLoadElement | `r%d = r%d[%d]` | ✅ | ✅ |
| OpStoreElement | `r%d[%d] = r%d` | ✅ | ✅ |

**Status:** Fixed in commit 6904779

### 3. Struct Opcodes - FIXED

| Opcode | String() Output | Parser | VM |
|--------|-----------------|--------|-----|
| OpLoadField | `r%d = r%d.field[%d]` | ✅ | ✅ |
| OpStoreField | `r%d.field[%d] = r%d` | ✅ | ✅ |

**Status:** Fixed in commit 6904779

### 4. Function Parameter Opcodes - FIXED

| Opcode | String() Output | Parser | VM |
|--------|-----------------|--------|-----|
| OpLoadParam | `r%d = param %s` | ✅ | ✅ |

**Status:** Fixed in commit 6904779

### 5. SMC/TSMC Opcodes - Z80 BACKEND ONLY

| Opcode | Usage |
|--------|-------|
| OpSMCLoadConst | Z80 codegen only |
| OpSMCStoreConst | Z80 codegen only |
| OpTrueSMCLoad | Z80 codegen only |
| OpTrueSMCPatch | Z80 codegen only |
| OpTSMCRef* (5 ops) | Z80 codegen only |

**Verdict:** Correctly not needed in MIR VM. These are target-specific optimizations.

### 6. Pointer Opcodes - FIXED

| Opcode | String() Output | Parser | VM |
|--------|-----------------|--------|-----|
| OpLoad | `r%d = *r%d` | ✅ | ✅ |
| OpStore | `*r%d = r%d` | ✅ | ✅ |
| OpAddr | `r%d = &r%d` | ✅ | ✅ |

**Status:** Fixed in commit 81767e4

### 7. Memory Allocation Opcodes - NOT NEEDED YET

| Opcode | String() Output | Status |
|--------|-----------------|--------|
| OpAlloc | `r%d = alloc(%d)` | No heap support yet |
| OpFree | `free(r%d)` | No heap support yet |
| OpMemcpy | `memcpy(...)` | Could add if needed |

**Verdict:** Not needed until MinZ has dynamic memory allocation.

### 8. Error Handling Opcodes - DISABLED

| Opcode | String() Output | Status |
|--------|-----------------|--------|
| OpSetError | `set_error r%d` | Code exists but disabled |
| OpCheckError | `r%d = check_error` | Code exists but disabled |

**Note:** Error propagation (`?` operator) code exists in `error_handling.go` but is commented out pending AST support.

## Working Features

All these work correctly end-to-end:

| Category | Opcodes |
|----------|---------|
| Arithmetic | Add, Sub, Mul, Div, Mod, Neg, Inc, Dec |
| Bitwise | And, Or, Xor, Not, Shl, Shr |
| Comparison | Lt, Gt, Eq, Ne, Le, Ge |
| Control Flow | Jump, JumpIf, JumpIfNot, Label, Return, DJNZ |
| Variables | LoadVar, StoreVar, LoadConst, LoadImm |
| Arrays | LoadIndex, StoreIndex, LoadElement, StoreElement |
| Structs | LoadField, StoreField |
| Pointers | Load (*r), Store (*r=), Addr (&r) |
| Params | LoadParam |
| I/O | Syscall, Print, PrintChar |
| Stack | Push, Pop |

## Test Results

| Test Case | Status |
|-----------|--------|
| While loop | ✅ PASS |
| For loop | ✅ PASS |
| Nested loops | ✅ PASS |
| Arithmetic | ✅ PASS |
| Syscalls | ✅ PASS |
| Arrays | ✅ PASS |
| Structs | ✅ PASS |
| Pointers | ✅ PASS |
| Function params | ✅ PASS |

**Total: 32 tests, all passing**

## Conclusion

The MIR layer is now **functionally complete** for:
- Basic programs (loops, arithmetic, variables)
- Graphics demos (syscalls, PNG output)
- Data structures (arrays, structs, pointers)
- Function parameters

**Remaining gaps:**
- Memory allocation (no heap yet)
- Error handling (code exists but disabled)
- Interface opcodes (deprecated - using monomorphization)

---

*Generated by Claude Code. Updated 2026-01-09 after fixes.*
