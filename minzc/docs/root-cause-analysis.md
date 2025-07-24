# MinZ Compiler: Root Cause Analysis

## Executive Summary

This document provides a deep technical analysis of the remaining issues in the MinZ compiler after the milestone-struct-support release. The analysis identifies the exact code locations, design flaws, and implementation gaps that prevent full functionality.

## Issue 1: Module Import Resolution

### Problem
After `import zx.screen;`, using `screen.BLACK` results in "undefined identifier: screen" error.

### Root Cause
The issue lies in `analyzeFieldExpr()` function (semantic/analyzer.go:1651-1669):

1. **Current Flow**:
   ```
   import zx.screen → registers "screen" as ModuleSymbol
                   → registers "screen.BLACK" as ConstSymbol
   
   screen.BLACK → parser creates FieldExpr{Object: "screen", Field: "BLACK"}
                → analyzeFieldExpr() special case only checks for functions
                → falls through to normal field access
                → tries to evaluate "screen" as expression
                → fails because ModuleSymbol cannot be used as value
   ```

2. **Code Analysis**:
   ```go
   // Line 1651: Special handling for module field access
   if id, ok := field.Object.(*ast.Identifier); ok {
       // Check if this is a module function call
       fullName := id.Name + "." + field.Field
       sym := a.currentScope.Lookup(fullName)
       if sym != nil {
           // This only handles function symbols!
           // Constants are ignored and fall through
       }
   }
   ```

3. **The Fix**:
   The special case handling needs to check for ALL module symbols (constants, functions, types), not just functions. The fix is simple - extend the check to handle ConstSymbol.

### Impact
- Cannot use any module constants
- Module system appears broken to users
- Workaround requires avoiding imports entirely

## Issue 2: Array Element Assignment

### Problem
`arr[i] = value` returns "array assignment not yet implemented" error.

### Root Cause
Multiple implementation gaps:

1. **Semantic Analysis** (analyzer.go:941-944):
   ```go
   case *ast.IndexExpr:
       // Array element assignment
       // TODO: Implement array assignment
       return fmt.Errorf("array assignment not yet implemented")
   ```

2. **IR Design Limitation**:
   - Current `Instruction` struct has only `Src1` and `Src2`
   - Array store needs THREE operands: array pointer, index, value
   - No `Src3` field exists in the instruction format

3. **Missing Code Generation**:
   - `OpStoreIndex` is defined but not implemented in z80.go
   - No Z80 assembly generation for array element stores

### Technical Details

The implementation requires:

1. **IR Extension**:
   ```go
   type Instruction struct {
       Op      OpCode
       Dest    Register
       Src1    Register
       Src2    Register
       Src3    Register  // NEW: needed for 3-operand instructions
       // ... other fields
   }
   ```

2. **Semantic Analysis**:
   ```go
   case *ast.IndexExpr:
       arrayReg, _ := a.analyzeExpression(target.Array, irFunc)
       indexReg, _ := a.analyzeExpression(target.Index, irFunc)
       // ... type checking ...
       irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
           Op:   ir.OpStoreIndex,
           Src1: arrayReg,  // array base
           Src2: indexReg,  // index
           Src3: valueReg,  // value to store
           Type: elementType,
       })
   ```

3. **Code Generation**:
   ```asm
   ; For arr[i] = value (byte array)
   LD HL, (array_ptr)  ; Array base
   LD DE, (index)      ; Index
   ADD HL, DE          ; HL = base + index
   LD A, (value)       ; Value to store
   LD (HL), A          ; Store byte
   ```

### Impact
- Cannot modify array elements
- Makes data structures like canvas impossible
- Forces flat variable usage instead of arrays

## Issue 3: String Literals

### Problem
String literals like `"Hello"` are not recognized by the parser.

### Root Cause
1. **Tokenizer**: No string literal token type defined
2. **Parser**: No string literal parsing in `parseExpression()`
3. **AST**: `StringLiteral` node exists but parser doesn't create it
4. **Semantic**: `analyzeStringLiteral()` exists but is never called

### Implementation Gap
The entire chain from tokenization to parsing is missing:
- Need to add string tokenization with quote handling
- Need to add string parsing in expression parser
- Need to handle escape sequences

### Impact
- Cannot display any text
- Cannot use string constants
- Limits UI development significantly

## Issue 4: Function Forward Declaration

### Problem
Functions must be defined before use.

### Root Cause
The compiler uses a single-pass approach:
1. Functions are analyzed in order of appearance
2. No forward declaration mechanism exists
3. Symbol table doesn't distinguish between declared and defined

### Design Consideration
This is a common limitation in simple compilers. Solutions include:
- Two-pass compilation
- Forward declaration syntax
- Lazy function resolution

### Impact
- Forces specific code organization
- Prevents mutual recursion
- Limits natural code structure

## Recommendations

### Priority 1: Fix Module Constants (Quick Fix)
- **Effort**: 1 hour
- **Impact**: Enables imports to work properly
- **Change**: Modify `analyzeFieldExpr()` to handle `ConstSymbol`

### Priority 2: Implement Array Assignment (Medium)
- **Effort**: 4-6 hours
- **Impact**: Critical for any array manipulation
- **Change**: 
  1. Add `Src3` to Instruction struct
  2. Implement semantic analysis
  3. Add code generation

### Priority 3: String Literals (Medium)
- **Effort**: 4-6 hours
- **Impact**: Enables text display
- **Change**: Full implementation from tokenizer to code gen

### Priority 4: Forward Declarations (Large)
- **Effort**: 8-16 hours
- **Impact**: Better code organization
- **Change**: Requires architectural changes

## Conclusion

The MinZ compiler has made significant progress with struct support and basic features. The remaining issues are well-understood with clear implementation paths. The module constant issue is trivial to fix, while array assignment requires moderate effort but follows established patterns. String literals need end-to-end implementation, and forward declarations would benefit from architectural redesign.