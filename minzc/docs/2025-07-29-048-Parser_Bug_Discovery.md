# 064: Critical Parser Bug Discovery

## Summary
During the investigation of variable scope resolution issues, we discovered that the root cause is not in the semantic analyzer but in the parser itself.

## Bug Description

### Symptom
- Only the first `let` statement in a function block is parsed as a `VarDecl` (variable declaration)
- All subsequent `let` statements are incorrectly parsed as `ExpressionStmt` (expression statements)

### Evidence
```
DEBUG: Processing statement 0 in block
DEBUG: analyzeStatement processing *ast.VarDecl       # ✅ First let statement
DEBUG: analyzeVarDeclInFunc called for variable 'x'
DEBUG: Processing statement 1 in block
DEBUG: analyzeStatement processing *ast.ExpressionStmt # ❌ Second let statement
```

### Impact
- 70%+ of MinZ examples fail compilation
- Any function with 2+ variables cannot compile
- Error messages are misleading (report "undefined identifier" instead of parse error)

## Root Cause Analysis

### Location
The bug is in one of these components:
1. **Tree-sitter grammar** (`grammar.js`) - Grammar rule for variable declarations may be incorrect
2. **AST conversion** (`pkg/parser/parser.go`) - Tree-sitter parse tree to AST conversion may be faulty
3. **Simple parser fallback** (`pkg/parser/simple_parser.go`) - If tree-sitter fails, the simple parser may have bugs

### Likely Cause
The parser likely has a state management issue where after parsing one variable declaration, it doesn't properly reset state to parse subsequent declarations.

## Temporary Workaround

Until the parser is fixed, we could implement a workaround in the semantic analyzer:

```go
case *ast.ExpressionStmt:
    // WORKAROUND: Check if this is actually a misparsed variable declaration
    if letExpr, ok := s.Expression.(*ast.LetExpression); ok {
        // Convert to VarDecl and process
        varDecl := &ast.VarDecl{
            Name:      letExpr.Name,
            Type:      letExpr.Type,
            Value:     letExpr.Value,
            IsMutable: letExpr.IsMutable,
        }
        return a.analyzeVarDeclInFunc(varDecl, irFunc)
    }
    // Otherwise, analyze as normal expression
    _, err := a.analyzeExpression(s.Expression, irFunc)
    return err
```

## Proper Fix

The proper fix requires:
1. Examining the tree-sitter grammar for variable declaration rules
2. Checking the AST conversion logic
3. Adding parser tests to ensure multiple variable declarations work
4. Potentially fixing the grammar or conversion logic

## Next Steps

1. Implement temporary workaround to unblock testing
2. Investigate tree-sitter grammar
3. Fix the parser properly
4. Add comprehensive parser tests