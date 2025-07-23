# Issue #002: Inline Assembly Not Implemented

## Summary
The MinZ compiler does not support inline assembly blocks (`asm` statements), which are essential for low-level Z80 programming and hardware interaction.

## Severity
High - This blocks implementation of hardware-specific code and optimized routines

## Affected Files
- `/examples/mnist/mnist_editor_minimal.minz` (lines 13-18)
- `/examples/mnist/mnist_editor.minz` (multiple locations)
- All MNIST editor variants that use inline assembly

## Reproduction Steps
1. Create a MinZ file with inline assembly:
```minz
fn set_pixel(x: u8, y: u8) -> void {
    asm("
        ld hl, {0}
        ld a, (hl)
        or {1}
        ld (hl), a
    " : : "r"(addr), "r"(bit));
}
```
2. Compile with `minzc file.minz -o output.a80`
3. Observe compilation failure

## Expected Behavior
The compiler should:
1. Parse inline assembly blocks
2. Validate assembly syntax
3. Handle parameter substitution ({0}, {1}, etc.)
4. Integrate the assembly into the generated code

## Actual Behavior
The compiler fails during parsing or semantic analysis because it doesn't recognize the `asm` keyword or syntax.

## Root Cause Analysis
1. The AST (pkg/ast) doesn't define an `InlineAsm` or `AsmStmt` node type
2. The parser doesn't recognize `asm` as a keyword
3. The semantic analyzer has no handler for inline assembly statements
4. The code generator lacks inline assembly support

## Suggested Fix
1. Add `AsmStmt` to the AST:
```go
type AsmStmt struct {
    Code string
    Outputs []AsmOperand
    Inputs []AsmOperand
    Clobbers []string
}
```
2. Update the parser to recognize `asm` syntax
3. Add semantic analysis for inline assembly
4. Implement code generation that directly emits the assembly code

## Workaround
No direct workaround. Alternatives:
1. Write separate .asm files and link them
2. Use external functions implemented in assembly
3. Generate Z80 assembly directly without using MinZ for hardware-specific code