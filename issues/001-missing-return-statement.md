# Issue #001: Missing Return Statement in Void Functions

## Summary
The MinZ compiler requires explicit `return` statements at the end of all functions, including void functions. This is inconsistent with most modern languages where void functions can implicitly return.

## Severity
Medium - This is a usability issue that affects code ergonomics

## Affected Files
- `/examples/simple_add.minz`
- All MNIST editor examples that have void functions without explicit returns

## Reproduction Steps
1. Create a MinZ file with a void function lacking an explicit return:
```minz
fn main() -> void {
    let x = 5;
    // No return statement
}
```
2. Compile with `minzc file.minz -o output.a80`
3. Observe: "semantic analysis failed with 1 errors"

## Expected Behavior
Void functions should be able to implicitly return at the end of their body without requiring an explicit `return;` statement.

## Actual Behavior
Compilation fails with a semantic error when void functions lack explicit return statements.

## Root Cause Analysis
The semantic analyzer likely checks that all code paths in a function lead to a return statement, without special-casing void functions that reach the end of their body.

## Suggested Fix
In `semantic/analyzer.go`, modify the function analysis to:
1. Check if the function return type is void
2. If so, allow the function to end without an explicit return
3. Optionally, automatically insert a return instruction in the IR for void functions

## Workaround
Always add explicit `return;` statements at the end of void functions:
```minz
fn main() -> void {
    let x = 5;
    return;  // Explicit return required
}
```