# MinZ Compiler: Deep Root Cause Analysis

## Executive Summary

This comprehensive analysis provides an in-depth examination of all issues in the MinZ compiler, including those mentioned in the initial root cause analysis and additional problems discovered through systematic code review. The analysis includes exact code locations, technical details, and implementation recommendations.

## Critical Issues

### 1. Module Import Resolution - ALREADY FIXED ✅

**Status**: The initial root cause analysis appears outdated. The code already handles module constants correctly.

**Current Implementation** (semantic/analyzer.go:1651-1685):
```go
if id, ok := field.Object.(*ast.Identifier); ok {
    fullName := id.Name + "." + field.Field
    sym := a.currentScope.Lookup(fullName)
    if sym != nil {
        switch s := sym.(type) {
        case *ConstSymbol:
            // Correctly handles constants
            return a.loadConstant(s.Value, s.Type, irFunc)
        case *FuncSymbol:
            // Handles functions
            return a.currentReg, s.ReturnType, nil
        }
    }
}
```

**Why It Works**: The analyzeFieldExpr function checks for module.field patterns BEFORE trying to evaluate the module identifier, preventing the "module cannot be used as value" error.

### 2. Array Element Assignment - NOT IMPLEMENTED ❌

**Location**: semantic/analyzer.go:941-944

**Current State**:
```go
case *ast.IndexExpr:
    // Array element assignment
    // TODO: Implement array assignment
    return fmt.Errorf("array assignment not yet implemented")
```

**Technical Requirements**:

1. **IR Instruction Format Limitation**:
   - Current: `Instruction` has only `Src1` and `Src2` fields
   - Needed: Array store requires THREE operands (array, index, value)
   - Solution: Either add `Src3` field or use a different approach

2. **Alternative Implementation Without Src3**:
   ```go
   // Use two instructions instead:
   // 1. Calculate address: array + index -> temp register
   irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
       Op:   ir.OpAdd,
       Dest: tempReg,
       Src1: arrayReg,
       Src2: indexReg,
   })
   // 2. Store value at calculated address
   irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
       Op:   ir.OpStoreDirect,
       Src1: valueReg,
       Src2: tempReg, // address in Src2
   })
   ```

3. **Code Generation** (z80.go needs OpStoreIndex case):
   ```asm
   ; arr[i] = value (byte array)
   LD HL, array_base   ; Load array pointer
   LD DE, index        ; Load index
   ADD HL, DE          ; Calculate address
   LD A, value         ; Load value
   LD (HL), A          ; Store at array[index]
   ```

### 3. String Literals - PARTIALLY WORKING ⚠️

**Status**: Works with simple parser, broken with tree-sitter parser

**Issue Location**: pkg/parser/parser.go:578 (parseExpression switch)
- Missing case for "string_literal" node type
- Tree-sitter grammar defines it, but parser ignores it

**Fix Required**:
```go
case "string_literal":
    value := n.Content()
    // Remove quotes and handle escapes
    unquoted := value[1:len(value)-1]
    return &ast.StringLiteral{
        Value:    unquoted,
        StartPos: int(n.StartPosition().Column),
        EndPos:   int(n.EndPosition().Column),
    }
```

### 4. Function Forward Declarations - MISCONCEPTION ✅

**Status**: Actually supported through two-pass compilation

**Evidence** (semantic/analyzer.go:71-95):
1. First pass: Registers all function signatures
2. Second pass: Analyzes function bodies

**Why It Works**: Functions can call each other regardless of definition order because signatures are registered before any bodies are analyzed.

## Additional Critical Issues

### 5. Module Function Parameter Types Missing ❌

**Location**: semantic/analyzer.go:227 (and similar)

**Problem**:
```go
a.currentScope.Define("screen.set_pixel", &FuncSymbol{
    Name:       "screen.set_pixel",
    ReturnType: &ir.BasicType{Kind: ir.TypeVoid},
    Params:     nil, // TODO: Add proper parameter types
})
```

**Impact**: No type checking for module function calls

**Fix**: Define proper parameter types:
```go
Params: []ir.Type{
    &ir.BasicType{Kind: ir.TypeU8}, // x
    &ir.BasicType{Kind: ir.TypeU8}, // y
    &ir.BasicType{Kind: ir.TypeU8}, // color
}
```

### 6. Array Type Validation Missing ❌

**Location**: semantic/analyzer.go:1784

**Problem**: No validation that indexed expression is actually an array
```go
// TODO: Type check that array is actually an array type
// For now, we'll just generate the load instruction
```

**Required Fix**:
```go
arrayType, ok := exprType.(*ir.ArrayType)
if !ok {
    if _, isPtr := exprType.(*ir.PointerType); !isPtr {
        return 0, nil, fmt.Errorf("cannot index non-array type %s", exprType)
    }
}
```

### 7. Type Promotion Rules Missing ❌

**Location**: semantic/analyzer.go:2037

**Problem**: Binary operations don't handle mixed types
```go
// TODO: Implement proper type promotion rules
return leftType, nil
```

**Required Implementation**:
- u8 + u16 → u16
- i8 + i16 → i16
- u8 + i8 → i16 (to avoid overflow)

### 8. Hardcoded Loop Buffer Address ⚠️

**Location**: semantic/analyzer.go:1132, 1199

**Problem**: Fixed buffer at 0xF000 could conflict
```go
bufferAddr := uint16(0xF000) // TODO: Make this configurable
```

**Solution**: Add compiler option or use stack allocation

### 9. Incomplete Struct Allocation ❌

**Location**: semantic/analyzer.go:1558

**Problem**: Struct literals don't generate proper allocation
```go
// TODO: Generate IR for struct allocation
// For now, just allocate on stack
```

### 10. Lua Metaprogramming Disabled ❌

**Location**: semantic/analyzer.go:46

**Problem**: Feature completely disabled
```go
// luaEvaluator: meta.NewLuaEvaluator(), // Temporarily disabled
```

## Architecture Issues

### 11. Module System Hardcoding ❌

**Location**: semantic/analyzer.go:143-162

**Problem**: Only handles specific hardcoded modules
- No generic module loading
- No file-based module resolution
- Module paths hardcoded in processImport

### 12. Missing Break/Continue Statements ❌

**Location**: semantic/analyzer.go:694-716

**Problem**: Switch statement in analyzeStatement doesn't handle:
- break statements (except in switch)
- continue statements
- No loop context tracking for validation

### 13. No Error Recovery ❌

**Problem**: Semantic analyzer stops at first error in each function
- Makes debugging difficult
- Users can't see multiple issues at once

## Implementation Priority

### Immediate Fixes (< 2 hours each)
1. **String literal tree-sitter parsing** - Add missing case
2. **Module function parameter types** - Add type definitions
3. **Array type validation** - Add type check

### Short-term Fixes (2-6 hours each)
1. **Array element assignment** - Critical for data manipulation
2. **Type promotion rules** - Needed for mixed-type operations
3. **Struct allocation** - Complete the implementation

### Medium-term Fixes (1-2 days each)
1. **Generic module system** - File-based module loading
2. **Break/continue statements** - Loop control flow
3. **Error recovery** - Better developer experience

### Long-term Improvements
1. **Lua metaprogramming** - Re-enable and test
2. **Configurable memory layout** - Remove hardcoded addresses
3. **Full type system** - Function types, generics, etc.

## Technical Debt Summary

The compiler has solid foundations but numerous TODOs and incomplete features:
- 15+ TODO comments in semantic analyzer alone
- Missing implementations for documented features
- Hardcoded values that should be configurable
- Disabled features (Lua metaprogramming)

## Recommendations

1. **Fix array assignment first** - Most critical for real programs
2. **Complete type checking** - Add missing validations
3. **Enable all parsers** - Fix tree-sitter string parsing
4. **Document limitations** - Update README with current state
5. **Add integration tests** - Catch regressions early

## Conclusion

The MinZ compiler is well-architected but incomplete. The most critical issue (array assignment) has a clear implementation path. Many "issues" in the original analysis are actually already fixed or were misconceptions. The real problems are the numerous unimplemented features marked with TODO comments throughout the codebase.

The compiler would benefit most from completing the existing partial implementations rather than adding new features. With focused effort on the identified issues, MinZ could become a fully functional systems programming language for Z80 platforms.