# Issue #004: Array Access and Pointer Dereferencing Issues

## Summary
The MNIST editor examples use array access syntax (`canvas[idx]`) and pointer dereferencing that may not be properly implemented in the compiler.

## Severity
Medium - Essential for data structure manipulation

## Affected Files
- `/examples/mnist/mnist_editor.minz` (line 23: `editor.canvas[i] = false`)
- `/examples/mnist/mnist_editor.minz` (line 86: `if editor.canvas[idx]`)
- All files using array or pointer access

## Reproduction Steps
1. Create a MinZ file with array access:
```minz
type Data = struct {
    values: [256]bool
};

fn test() -> void {
    let mut data: Data;
    data.values[0] = true;  // Array access through struct field
    return;
}
```
2. Compile the file
3. Observe potential compilation errors

## Expected Behavior
The compiler should support:
1. Array indexing with `[]` operator
2. Struct field access combined with array indexing
3. Pointer dereferencing with array access
4. Bounds checking (at least in debug mode)

## Actual Behavior
Array access through struct fields or complex expressions may not be properly handled by the semantic analyzer or code generator.

## Root Cause Analysis
1. The expression analyzer may not handle compound expressions like `struct.field[index]`
2. Array indexing might not be implemented as a specific AST node
3. Code generation for array access might be incomplete

## Suggested Fix
1. Ensure `IndexExpr` AST node exists and is properly handled
2. In semantic analyzer, handle array indexing:
   - Verify the base expression is an array type
   - Check index expression is integral
   - Calculate element address
3. In code generator, emit proper Z80 code for array access:
   - Calculate base + (index * element_size)
   - Load/store at calculated address

## Workaround
Use explicit address calculation and pointer arithmetic instead of array syntax (not recommended for readability).