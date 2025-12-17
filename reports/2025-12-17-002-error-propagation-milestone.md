# MinZ Error Propagation Milestone Report

**Date:** 2025-12-17
**Report:** 002
**Version:** v0.15.1-dev
**Status:** MILESTONE ACHIEVED

---

## Executive Summary

The `@error()` metafunction for error propagation is now **fully working**. This enables proper error handling in MinZ using the Z80's carry flag (CY) and A register ABI.

### Key Achievement

```minz
fun safe_divide?(a: u8, b: u8) -> u8 ? MathError {
    if b == 0 {
        @error(1);  // Sets CY flag, A = error code, returns
    }
    return a;
}
```

Generates correct Z80 assembly:
```asm
LD A, D          ; Load error code
SCF              ; Set carry flag (error indicator)
RET              ; Return with error
```

---

## What Was Implemented

### 1. Fixed `analyzeExplicitError` Function

**Before:**
```go
// Placeholder that didn't work
irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
    Op:   ir.OpLoadConst,  // Wrong!
    Dest: ir.Register(-10),
    Imm:  1,
})
```

**After:**
```go
// Proper error handling
irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
    Op:      ir.OpSetError,
    Src1:    errorReg,
    Comment: "@error() - set carry flag with error code in A",
})
```

### 2. Relaxed Type Checking for Enums

- `@error(1)` now works when function declares `? EnumType`
- Pragmatic: enums compile to u8 at runtime
- Integer literals within valid range are accepted

### 3. Z80 Code Generation

`OpSetError` generates:
```asm
LD A, <error_code>    ; Load error code to A register
SCF                   ; Set carry flag (error indicator)
```

---

## Error Handling ABI (Z80)

| Component | Usage |
|-----------|-------|
| **CY flag** | Error indicator (SCF sets it) |
| **A register** | Error code (flattened enum value) |
| **Caller check** | `JR NC, success_path` |

### Enum Error Ranges (Convention)
| Range | Purpose |
|-------|---------|
| 0-15 | Core errors |
| 16-31 | File errors |
| 32-47 | Network errors |
| 48+ | User-defined |

---

## Test Results

### E2E Compilation Test
```
=== COMPILATION SUCCESS ===
SCF              ; Set carry flag (error)
; Return with error (CY=1, A=error code)
```

### Compilation Success Rate
- **Before:** 81% (47/58)
- **After:** 81% (48/59) - +1 new example compiles
- **No regressions**

### Unit Tests
- `pkg/codegen` tests: **PASS**
- `pkg/ir` tests: **PASS**

---

## Files Changed

| File | Changes |
|------|---------|
| `minzc/pkg/semantic/analyzer.go` | Fixed `analyzeExplicitError`, relaxed type checking |
| `examples/error_propagation_demo.minz` | New E2E demo |
| `README.md` | Updated error propagation status to "Working" |

---

## Example: Complete Error Handling

```minz
enum MathError { None, DivByZero, Overflow }

// Function that can fail
fun safe_divide?(a: u8, b: u8) -> u8 ? MathError {
    if b == 0 {
        @error(1);  // MathError.DivByZero
    }
    return a;  // Success path
}

// Alternative: Manual assembly
fun checked_add?(a: u8, b: u8) -> u8 ? MathError {
    let result = a + b;
    if result < a {
        asm {
            LD A, 2       ; MathError.Overflow
            SCF           ; Set carry flag
        }
        return 0;
    }
    return result;
}
```

---

## What's Next

### Remaining Error Handling Work
1. **Enum value access** - `MathError.DivByZero` syntax (P1)
2. **`?? @error`** - Cleaner propagation syntax (P2)
3. **Type inference** - Remove explicit type annotations (P2)

### Priority Fixes from TODO.md
1. Array literal DB generation
2. Pattern matching codegen
3. Function pointer passing

---

## Commits

1. `9482e7e` - feat: Implement @error(value) metafunction for error propagation

---

## Conclusion

This milestone marks a significant step toward complete error handling in MinZ. The `@error()` metafunction now properly generates Z80 code that sets the carry flag and error code, enabling robust error propagation patterns.

The implementation follows the established CY flag + A register ABI, ensuring compatibility with existing error checking patterns in the codebase.

---

*Report generated: 2025-12-17*
*MinZ: Where Modern Dreams Meet Vintage Reality*
