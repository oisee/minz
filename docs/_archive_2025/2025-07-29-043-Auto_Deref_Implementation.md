# Auto-Deref Implementation Complete

## Overview

Successfully implemented auto-dereferencing for pointer assignments in MinZ. This makes pointer usage more ergonomic by automatically inserting dereference operations when assigning values to pointers.

## Key Implementation Details

### 1. Assignment Target Auto-Deref
When assigning a value to a pointer variable, the compiler automatically dereferences:
```minz
var p: *u8 = &x;
p = 42;  // Automatically becomes: *p = 42
```

### 2. Semantic Analysis
In `analyzeAssignment`:
- Check if target is a pointer type
- Check if value type is compatible with pointed-to type
- If yes, generate OpLoadVar + OpStore instead of OpStoreVar
- Special handling to exclude TSMC references

### 3. Key Bug Fix: SMC Timing Issue
**Problem**: Functions start with SMC enabled by default, then calling convention is determined later. This caused non-SMC functions to incorrectly treat pointer parameters as TSMC references.

**Solution**: Check `irFunc.CallingConvention == "smc"` instead of `irFunc.IsSMCEnabled` when determining TSMC references.

### 4. Generated Code Example
For `p = 42` where p is a pointer parameter:
```asm
; Auto-deref assignment
PUSH HL
LD A, ($F000)     ; Load value to store
POP HL
LD (HL), A        ; Store through pointer
```

## Technical Challenges Resolved

### 1. TSMC vs Regular Pointers
- TSMC references are special self-modifying code patterns
- Regular pointers should use normal indirection
- Fixed by checking actual calling convention, not default flags

### 2. Type Compatibility
- Number literals get type inferred (u8 for 0-255)
- Type compatibility check ensures safe auto-deref
- Falls back to checking for number literals when type inference fails

### 3. Parameter Handling
- Parameters already have allocated registers
- Local variables need OpLoadVar first
- Handled with conditional logic based on IsParameter flag

## Impact

This implementation enables:
- Cleaner pointer code without explicit dereferencing
- Natural syntax for pointer arithmetic: `p = p + 1`
- Foundation for more complex auto-deref contexts
- Better ergonomics for TSMC patterns

## Next Steps

1. Auto-deref in expressions (not just assignments)
2. Auto-deref for function arguments
3. Multiple levels of indirection handling
4. Complex assignment targets (arrays, structs)

## Code Quality Note

The current implementation works but could be refined:
- Better code generation for pointer operations
- More efficient register usage
- Cleaner handling of parameter loading

However, the core functionality is solid and provides a good foundation for future enhancements.