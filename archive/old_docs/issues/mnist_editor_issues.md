# MNIST Editor Compilation Issues

Date: 2025-07-23

## Issue #006: Undefined type: Editor
**Error**: `invalid return type for function editor_init: undefined type: Editor`
**Description**: The Editor struct type is not recognized, likely because it's defined in a different module that isn't being imported correctly.

## Issue #007: Module imports not resolving types
**Error**: `function editor_init not found in symbol table`
**Description**: Functions from imported modules are not being added to the symbol table.

## Issue #008: Const keyword not supported
**Error**: `invalid parameter type for text: undefined type: const`
**Description**: The `const` keyword is being treated as a type name instead of a type modifier.

## Issue #009: Field expression type inference
**Error**: `cannot infer type for variable x: cannot infer type from expression of type *ast.FieldExpr`
**Description**: Type inference for field expressions (e.g., `editor.cursor_x`) is not implemented.

## Issue #010: Cross-module function calls
**Error**: `error in function main: cannot infer type for variable editor: undefined function: editor_init`
**Description**: Functions from imported modules cannot be called because they're not in the symbol table.

## Summary
The main issues are:
1. Module import system not working (types and functions from imported modules are not available)
2. Field expression type inference missing
3. Const keyword not supported as a type modifier