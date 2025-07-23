# Root Cause Analysis - MinZ Compiler Issues

## Executive Summary

The MinZ compiler currently has 5 major issues preventing the MNIST editor examples from compiling. The root causes stem from incomplete implementation of core language features and architectural decisions that don't align with user expectations.

## Issue Analysis

### 1. Missing Return Statement (Issue #001)

**Status**: ✅ Can be fixed easily

**Root Cause**: 
- The semantic analyzer's control flow analysis requires all code paths to have explicit returns
- No special case for void functions reaching the end of their body
- Located in `analyzeReturnStmt` and function analysis logic

**Fix Approach**:
```go
// In analyzeBlock, after processing all statements:
if len(block.Statements) > 0 {
    lastStmt := block.Statements[len(block.Statements)-1]
    if _, isReturn := lastStmt.(*ast.ReturnStmt); !isReturn {
        if irFunc.ReturnType == &ir.BasicType{Kind: ir.TypeVoid} {
            // Add implicit return for void functions
            irFunc.Emit(ir.OpReturn, 0, 0, 0)
        }
    }
}
```

### 2. Inline Assembly Not Implemented (Issue #002)

**Status**: ✅ ALREADY FIXED - Just implemented inline assembly support

**Root Cause**: Was missing AST nodes, parser support, and code generation
**Resolution**: Complete implementation added with `asm { }` syntax

### 3. Missing Module Imports (Issue #003)

**Status**: 🔧 Requires significant work

**Root Cause**:
- Module resolver is created but never connected to the analyzer
- See `cmd/minzc/main.go` line 76: `// TODO: Set module resolver on analyzer`
- No implementation of module file loading from stdlib
- Symbol resolution doesn't check imported modules

**Architecture Issue**:
```go
// Created but not used:
moduleManager := module.NewModuleManager(projectRoot)

// Analyzer created without resolver:
analyzer := semantic.NewAnalyzer()
// analyzer.SetModuleResolver(resolver) // This method doesn't exist!
```

**Fix Approach**:
1. Add `SetModuleResolver` method to Analyzer
2. Implement actual module loading in ModuleManager
3. Update symbol resolution to check imported modules
4. Handle module path resolution (stdlib vs project-relative)

### 4. Array Access Syntax (Issue #004)

**Status**: 🔧 Medium complexity

**Root Cause**:
- No AST node for index expressions (`array[index]`)
- Parser doesn't recognize `[]` operator
- Expression analyzer can't handle compound expressions like `struct.field[index]`

**Missing Components**:
```go
// Need in AST:
type IndexExpr struct {
    Array Expression
    Index Expression
    StartPos Position
    EndPos Position
}

// Need in parser:
case "[":
    return p.parseIndexExpr(expr)
```

### 5. String Literals and Pointers (Issue #005)

**Status**: 🔧 Medium complexity

**Root Cause**:
- No string literal support in lexer/parser
- Pointer arithmetic not implemented
- No data section generation for string constants

**Missing Components**:
- TokenString in lexer
- StringLiteral AST node
- String constant handling in code generator
- Pointer indexing syntax (`ptr[i]`)

## Architectural Root Causes

### 1. **Two-Parser Problem**
- Tree-sitter parser for JSON AST (incomplete)
- Simple parser as fallback (limited features)
- Features added to one parser not reflected in the other
- Grammar.js not updated when simple parser is extended

### 2. **Module System Design Flaw**
- Module components exist but aren't connected
- No clear separation between stdlib and user modules
- Import resolution happens too late in the pipeline

### 3. **Expression Parsing Limitations**
- Simple recursive descent parser lacks operator precedence handling
- Complex expressions (array access, field access chains) not supported
- No unified expression parsing strategy

### 4. **Type System Gaps**
- String type not defined
- Array access type checking missing
- Pointer arithmetic rules not implemented

## Fix Priority and Effort Estimation

### Immediate Fixes (Low Effort):
1. ✅ **Inline Assembly** - DONE
2. **Void Function Returns** - 2 hours
   - Add implicit return in semantic analyzer
   - Update function epilogue generation

### High Priority (Medium Effort):
3. **Array Access** - 1 day
   - Add IndexExpr to AST
   - Update parser for `[]` operator
   - Implement in semantic analyzer and code gen

4. **String Literals** - 1 day
   - Add string tokenization
   - Implement StringLiteral AST node
   - Generate data section for strings

### Complex Fixes (High Effort):
5. **Module System** - 3-5 days
   - Design module resolution strategy
   - Implement module loading
   - Update symbol resolution
   - Handle circular dependencies

## Recommended Fix Order

1. **Fix void returns** (Quick win)
2. **Implement array access** (Enables MNIST editor)
3. **Add string literals** (Common use case)
4. **Complete module system** (Enables stdlib usage)

## Testing Strategy for Fixes

Each fix should include:

1. **Parser Test**:
   ```minz
   // Test array access parsing
   let arr: [10]u8;
   let x = arr[5];
   ```

2. **Semantic Test**:
   ```minz
   // Test type checking
   let arr: [10]u8;
   let ptr: *u8 = &arr[0];
   ```

3. **Codegen Test**:
   ```minz
   // Test actual code generation
   fn test() -> u8 {
       let arr: [10]u8;
       arr[0] = 42;
       return arr[0];
   }
   ```

4. **Integration Test**:
   - Compile and run actual MNIST editor examples

## Conclusion

The MinZ compiler has solid foundations but lacks several essential features. The issues are fixable but require careful implementation to maintain consistency across the compiler pipeline. Priority should be given to features that unblock the most use cases (array access, strings) before tackling the more complex module system.