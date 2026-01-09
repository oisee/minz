# Semantic Analyzer Fix: If Expressions with Block Statements

## Problem
The semantic analyzer fails with "unsupported expression type: *ast.BlockStmt" when if expressions use block statements for their branches.

## Current Code Issue
In `analyzer.go:8309-8311`:
```go
// Then branch
thenReg, err := a.analyzeExpression(expr.ThenBranch, irFunc)
if err != nil {
    return 0, fmt.Errorf("if expression then branch: %w", err)
}
```

The problem: `expr.ThenBranch` can be a `*ast.BlockStmt`, but `analyzeExpression` doesn't handle BlockStmt.

## Solution

### Option 1: Add BlockStmt handling to analyzeExpression (RECOMMENDED)
Add a case in `analyzeExpression` to handle BlockStmt as an expression:

```go
case *ast.BlockStmt:
    return a.analyzeBlockExpression(e, irFunc)
```

Then implement `analyzeBlockExpression`:
```go
func (a *Analyzer) analyzeBlockExpression(block *ast.BlockStmt, irFunc *ir.Function) (ir.Register, error) {
    // Push new scope for block
    a.pushScope()
    defer a.popScope()
    
    // Analyze all statements except the last
    for i := 0; i < len(block.Statements)-1; i++ {
        if err := a.analyzeStatement(block.Statements[i], irFunc); err != nil {
            return 0, err
        }
    }
    
    // The last statement should be an expression that provides the block's value
    if len(block.Statements) > 0 {
        lastStmt := block.Statements[len(block.Statements)-1]
        
        // Check if it's an expression statement
        if exprStmt, ok := lastStmt.(*ast.ExpressionStmt); ok {
            return a.analyzeExpression(exprStmt.Expression, irFunc)
        }
        
        // If it's a return statement in a block, that's also valid
        if retStmt, ok := lastStmt.(*ast.ReturnStmt); ok && retStmt.Value != nil {
            return a.analyzeExpression(retStmt.Value, irFunc)
        }
        
        // Otherwise, block has no value (void/unit type)
        return 0, nil
    }
    
    // Empty block returns void/unit
    return 0, nil
}
```

### Option 2: Wrap analyzeIfExpr to handle blocks specially
Modify `analyzeIfExpr` to check if branches are blocks:

```go
func (a *Analyzer) analyzeIfExpr(expr *ast.IfExpr, irFunc *ir.Function) (ir.Register, error) {
    // ... existing condition analysis ...
    
    // Helper to analyze branch (expression or block)
    analyzeBranch := func(branch ast.Expression) (ir.Register, error) {
        if block, ok := branch.(*ast.BlockStmt); ok {
            return a.analyzeBlockExpression(block, irFunc)
        }
        return a.analyzeExpression(branch, irFunc)
    }
    
    // Then branch
    thenReg, err := analyzeBranch(expr.ThenBranch)
    if err != nil {
        return 0, fmt.Errorf("if expression then branch: %w", err)
    }
    
    // ... similar for else branch ...
}
```

## Implementation Steps

1. **Add BlockStmt case to analyzeExpression** (~5 lines)
2. **Implement analyzeBlockExpression** (~30 lines)
3. **Ensure proper scope management** (push/pop scope)
4. **Handle block value semantics** (last expression is the value)
5. **Add type checking** to ensure then/else branches have compatible types

## Test Cases

```minz
// Should compile after fix
fun test_if_block() -> u8 {
    let x = 10;
    let result = if x > 5 {
        let temp = x * 2;
        temp + 1  // Block value is 21
    } else {
        let temp = x / 2;
        temp - 1  // Block value is 4
    };
    return result;
}
```

## Expected Impact
- Will unblock 5+ files that use if expressions with blocks
- Foundation for other block-as-expression features
- Enables more idiomatic MinZ code patterns