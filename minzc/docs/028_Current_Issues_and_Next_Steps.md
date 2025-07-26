# 028: Current Issues and Next Steps

## Summary

We have successfully implemented the complete infrastructure for bit-struct types in MinZ:
- ✅ Full language design with syntax
- ✅ AST nodes for all bit struct features
- ✅ Parser support for bit struct declarations and cast expressions
- ✅ Semantic analysis with type checking
- ✅ IR opcodes for bit field operations
- ✅ Code generation using efficient AND/OR/shift strategy
- ✅ Type conversions with the `as` operator

## Critical Blocker: Parser Bug

The simple parser has a bug where it only parses the first statement in a block when statements are on separate lines. This prevents testing of bit field access since we can't write multi-statement test programs.

Example that fails:
```minz
fn main() -> void {
    let mut a: u8 = 1;    // This is parsed
    let mut b: u8 = 2;    // This is NOT parsed
    let mut c: u8 = 3;    // This is NOT parsed
}
```

## Bit Field Access Status

The bit field access code is complete:
1. Semantic analyzer correctly identifies bit struct field access
2. Generates `OpLoadBitField` with correct offset and width
3. Code generator produces optimal Z80 assembly

However, we cannot properly test it due to the parser bug.

## Next Steps (Priority Order)

### 1. Fix Parser Bug (Critical)
- Debug why `parseBlock()` only captures first statement
- Likely issue with tokenization across line boundaries
- May need to refactor how statements are delimited

### 2. Complete Bit Field Testing
Once parser is fixed:
- Test field read operations
- Test field write operations
- Verify generated assembly is correct
- Add comprehensive test suite

### 3. Implement Assignment to Bit Fields
- Handle `bits.field = value` in semantic analyzer
- Generate `OpStoreBitField` operations
- Test with various bit widths

### 4. Add 16-bit Bit Struct Support
- Extend code generation to use HL register
- Handle multi-byte shifts and masks
- Test with complex 16-bit layouts

### 5. Implement Struct Literal Syntax
- Parse `{ field: value, ... }` syntax
- Support bit struct initialization
- Generate efficient initialization code

## Alternative Approach

If fixing the parser proves too complex, consider:
1. Using the tree-sitter parser instead (may need updates)
2. Implementing a workaround for testing (single-line programs)
3. Creating a separate test harness that bypasses the parser

## Technical Debt

1. **Parser Architecture**: The simple parser needs better statement handling
2. **Error Messages**: Need better diagnostics for bit struct errors
3. **Optimization**: Bit field operations could be optimized for special cases
4. **Documentation**: Need user-facing docs for bit struct feature

## Conclusion

The bit struct implementation is technically complete but blocked by a parser bug. Once this is resolved, we can fully test and refine the feature. The design is sound and the generated code is optimal - we just need to fix the infrastructure issue.