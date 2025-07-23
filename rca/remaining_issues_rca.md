# Root Cause Analysis: Remaining MinZ Compiler Issues

Date: 2025-07-23

## Issue 1: Const Declaration Lookup Failure

### Status: PARTIALLY RESOLVED
The const declarations are being parsed and analyzed, but there's still an issue with symbol resolution in certain contexts.

### Root Cause:
- Constants ARE being defined in the symbol table
- The lookup logic with module prefixes may not be working correctly
- Possible timing issue between when constants are defined vs when they're looked up

### Current State:
- Simple const test case still fails
- Need to investigate why symbol lookup fails despite correct definition

## Issue 2: Undefined Type "Editor"

### Root Cause:
The struct `Editor` is defined in files with module declarations (e.g., `module mnist.editor`). When the struct is defined, it gets prefixed as "mnist.editor.Editor". However, when the type is referenced in function parameters or variables, the lookup for "Editor" fails.

### Analysis:
- The type lookup in `convertType` does try with module prefix
- But something is still failing in the resolution

## Issue 3: Nil Expression Types

### Root Cause:
The MinZ files use TWO different inline assembly syntaxes:

1. **Block syntax** (what we implemented):
```minz
asm {
    ld a, 1
}
```

2. **GCC-style syntax** (not implemented):
```minz
asm("code" : outputs : inputs : clobbers)
```

The parser returns nil for the GCC-style syntax because it's not recognized as a valid expression.

### Files Affected:
- mnist_editor_simple.minz uses GCC-style inline assembly

## Issue 4: Missing Module Functions

### Root Cause:
- Module functions like `screen.clear()` are referenced but not defined
- We have partial module support but no actual module file loading
- The screen module functions are manually registered but incomplete

## Summary of Fixes Needed:

1. **Const lookup**: Debug why the symbol table lookup fails for constants
2. **Type resolution**: Fix struct type lookup with module prefixes
3. **GCC-style inline asm**: Add parser support for `asm("..." : : : )` syntax
4. **Module loading**: Implement actual module file loading or complete the standard library