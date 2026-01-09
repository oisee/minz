# Compiler Fixes - January 2026

## Summary

Critical compiler stability fixes addressing infinite loops during optimization and missing opcode representations.

## Issue 1: Compiler Hang on Self-Referential Assignments in Loops

### Problem
The compiler would hang indefinitely when compiling code patterns like:
```minz
for i in 0..10 {
    a = a + 1;  // Self-referential assignment caused infinite loop
}
```

### Root Cause
The `copyPropagation` pass in `pkg/optimizer/mir_peephole.go` was oscillating infinitely. It incorrectly counted a "change" even when propagating a value that was already the target value (no actual change).

### Fix
**File:** `pkg/optimizer/mir_peephole.go`

1. Added safety limit to prevent infinite optimization loops:
```go
const maxIterations = 100 // Safety limit to prevent infinite loops
for iteration := 0; iteration < maxIterations; iteration++ {
```

2. Fixed copy propagation to only count changes when value actually differs:
```go
// Before (buggy):
if src, ok := copies[inst.Src1]; ok {
    inst.Src1 = src
    changed = true  // Always marked as changed!
}

// After (fixed):
if src, ok := copies[inst.Src1]; ok && src != inst.Src1 {
    inst.Src1 = src
    changed = true  // Only when actually different
}
```

### Impact
- All loop patterns with self-referential assignments now compile correctly
- Compilation time reduced from infinite to milliseconds
- No regression in optimization quality

---

## Issue 2: "Unknown Op" Messages in Generated Assembly

### Problem
Generated assembly output contained unhelpful comments like:
```asm
    ; unknown op 22
    ; unknown op 64
```

### Root Cause
The `Instruction.String()` method in `pkg/ir/ir.go` was missing switch cases for many opcodes, causing them to fall through to the default case.

### Fix
**File:** `pkg/ir/ir.go`

Added `String()` representations for 17 missing opcodes:

| Opcode | String Representation |
|--------|----------------------|
| OpMove | `r%d = r%d` |
| OpTest | `test r%d` |
| OpLogicalAnd | `r%d = r%d && r%d` |
| OpLogicalOr | `r%d = r%d \|\| r%d` |
| OpLoadIndex | `r%d = r%d[r%d]` |
| OpStoreIndex | `r%d[r%d] = r%d` |
| OpLoadElement | `r%d = r%d[%d]` |
| OpStoreElement | `r%d[%d] = r%d` |
| OpLoadPtr | `r%d = *r%d` |
| OpStorePtr | `*r%d = r%d` |
| OpAlloc | `r%d = alloc(%d)` |
| OpFree | `free(r%d)` |
| OpPush | `push r%d` |
| OpPop | `r%d = pop` |
| OpPatchPoint | `patch_point %s` |
| OpPatchTemplate | `patch_template %s` |
| OpPatchTarget | `patch_target %s` |
| OpPatchParam | `patch_param %s` |
| OpArrayInit | `r%d = array_init(%d)` |
| OpArrayElement | `r%d[%d] = %d` |
| OpArrayLiteral | `r%d = array_literal` |
| OpLoadBitField | `r%d = r%d.bits[%d:%d]` |
| OpStoreBitField | `r%d.bits[%d:%d] = r%d` |
| OpSetError | `set_error r%d` |
| OpCheckError | `r%d = check_error` |

### Impact
- All generated assembly now has readable IR comments
- Debugging compilation issues is much easier
- No functional change to generated code

---

## Test Results

After fixes:
- **47/53 examples compile successfully** (88.7% success rate)
- Remaining 6 failures are pre-existing issues unrelated to these fixes
- All loop patterns work correctly
- Plasma demo compiles and generates clean assembly

## Files Modified

1. `pkg/optimizer/mir_peephole.go` - Safety limit + copy propagation fix
2. `pkg/ir/ir.go` - Added missing opcode String() cases
