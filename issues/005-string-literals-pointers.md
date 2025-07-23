# Issue #005: String Literals and Pointer Operations

## Summary
The MNIST editor uses string literals and pointer operations (e.g., `text[i] != 0`) that may not be properly supported by the compiler.

## Severity
Medium - Required for text handling and C-style string operations

## Affected Files
- `/examples/mnist/mnist_editor.minz` (line 60: `while text[i] != 0`)
- All files using string literals and pointer arithmetic

## Reproduction Steps
1. Create a MinZ file with string operations:
```minz
fn print_string(text: *const u8) -> void {
    let mut i: u8 = 0;
    while text[i] != 0 {
        // Process character
        i = i + 1;
    }
    return;
}

fn main() -> void {
    print_string("Hello");
    return;
}
```
2. Compile the file
3. Observe compilation errors

## Expected Behavior
1. String literals should be:
   - Stored in the data section
   - Null-terminated
   - Accessible via pointer to const u8
2. Pointer indexing (`ptr[i]`) should work like array access
3. String literals should be implicitly convertible to `*const u8`

## Actual Behavior
- String literal syntax may not be recognized
- Pointer indexing might not be implemented
- Type conversion from string literal to pointer might fail

## Root Cause Analysis
1. String literals might not be defined in the AST
2. The semantic analyzer might not handle string literal to pointer conversion
3. Pointer arithmetic and indexing might not be implemented
4. Data section generation for strings might be missing

## Suggested Fix
1. Add `StringLiteral` to AST
2. In semantic analyzer:
   - Treat string literals as `*const u8`
   - Generate unique labels for each string
   - Add strings to data section
3. In code generator:
   - Emit string data with null termination
   - Generate proper labels and references

## Workaround
Define character arrays explicitly:
```minz
const HELLO: [6]u8 = [72, 101, 108, 108, 111, 0]; // "Hello\0"
```