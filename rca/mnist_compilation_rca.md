# Root Cause Analysis: MNIST Editor Compilation Failures

Date: 2025-07-23

## Issue #1: Undefined Type: Editor

### Affected Files:
- editor.minz
- mnist_attr_editor.minz
- mnist_editor_simple.minz

### Root Cause:
The type definition syntax varies between files:
- Some use direct struct syntax
- The last mnist_editor.minz uses `type Editor = struct` syntax which IS supported
- The issue is that in some files, the Editor type is used before it's defined or in parameters where type resolution happens in a different order

### Evidence:
- mnist_editor_minimal.minz compiles successfully with the same Editor struct
- The issue appears when Editor is used as a parameter type

## Issue #2: Undefined Constants (SCREEN_START, SCREEN_ADDR)

### Affected Files:
- editor.minz (SCREEN_START)
- mnist_attr_editor.minz (SCREEN_START)
- mnist_editor_simple.minz (SCREEN_ADDR)

### Root Cause:
These files expect hardware-specific constants to be predefined. The constants are:
- SCREEN_START: ZX Spectrum screen memory start (16384)
- ATTR_START: Attribute memory start (22528)
- SCREEN_ADDR: Same as SCREEN_START

These constants are not defined in the files and there's no const declaration support or standard library providing them.

### Evidence:
```minz
const SCREEN_START: u16 = 16384;
const ATTR_START: u16 = 22528;
```
This syntax is used but `const` is not fully implemented as a declaration type.

## Issue #3: Module Import System

### Affected Files:
- mnist_editor.minz (the screen module version)

### Root Cause:
While we implemented basic module support:
1. The `screen` module functions are registered manually
2. No actual file loading/parsing of imported modules
3. The type inference issue with module function calls assigned to variables
4. Missing standard library modules

### Evidence:
- `screen.attr_addr()` works as a statement but fails when assigned to a variable
- No actual `zx.screen` module file exists

## Issue #4: Forward Function References

### Affected Files:
- mnist_editor.minz

### Root Cause:
Functions are only registered in their declaration order. The semantic analyzer does a two-pass analysis but only for types and function signatures, not for all functions.

### Evidence:
- `handle_input` is defined after `main` but called from `main`
- The first pass only registers function signatures, not all function references

## Issue #5: Nil Expression Type

### Affected Files:
- mnist_editor_simple.minz

### Root Cause:
The parser returns nil for some expression types that aren't fully implemented. This could be:
- Complex expressions
- Inline assembly expressions
- Unsupported syntax

### Evidence:
- `unsupported expression type: <nil>` in set_pixel function
- Likely related to inline assembly or pointer operations

## Issue #6: Parameter Scope Issues

### Affected Files:
- mnist_editor.minz (undefined identifier: editor)

### Root Cause:
Function parameters are not properly added to the scope in some contexts, particularly in for loops or nested blocks.

### Evidence:
- `editor` is a parameter but reported as undefined
- The error occurs in loop contexts