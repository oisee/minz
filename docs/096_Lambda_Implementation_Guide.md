# Lambda Implementation Guide

## Overview

This guide describes how to implement compile-time lambda transformation in the MinZ compiler.

## Implementation Steps

### Step 1: Modify Semantic Analyzer

#### 1.1 Detect Lambda Assignments

In `analyzeVarDecl`:
```go
func (a *Analyzer) analyzeVarDecl(v *ast.VarDecl, irFunc *ir.Function) error {
    // Check if value is a lambda
    if lambdaExpr, ok := v.Value.(*ast.LambdaExpr); ok {
        return a.transformLambdaAssignment(v, lambdaExpr, irFunc)
    }
    // ... existing var handling
}
```

#### 1.2 Transform Lambda to Function

```go
func (a *Analyzer) transformLambdaAssignment(v *ast.VarDecl, lambda *ast.LambdaExpr, parentFunc *ir.Function) error {
    // Generate unique function name
    funcName := fmt.Sprintf("%s$%s_%d", parentFunc.Name, v.Name, a.lambdaCounter)
    a.lambdaCounter++
    
    // Create IR function
    lambdaFunc := &ir.Function{
        Name:              funcName,
        CallingConvention: "register", // Fast calls
        IsSMCDefault:      false,      // No SMC for local lambdas
    }
    
    // Add parameters
    for _, param := range lambda.Params {
        paramType, _ := a.convertType(param.Type)
        lambdaFunc.Params = append(lambdaFunc.Params, ir.Parameter{
            Name: param.Name,
            Type: paramType,
        })
    }
    
    // Set return type
    if lambda.ReturnType != nil {
        lambdaFunc.ReturnType, _ = a.convertType(lambda.ReturnType)
    } else {
        // Infer from body
        lambdaFunc.ReturnType = a.inferLambdaReturnType(lambda)
    }
    
    // Analyze lambda body in new scope
    lambdaScope := NewScope(a.currentScope)
    prevFunc := a.currentFunc
    a.currentFunc = lambdaFunc
    
    // Add parameters to scope
    for _, param := range lambdaFunc.Params {
        reg := lambdaFunc.AddParam(param.Name, param.Type)
        lambdaScope.Define(param.Name, &ParamSymbol{
            Name: param.Name,
            Type: param.Type,
            Reg:  reg,
        })
    }
    
    // Analyze body
    prevScope := a.currentScope
    a.currentScope = lambdaScope
    a.analyzeBlock(lambda.Body, lambdaFunc)
    a.currentScope = prevScope
    a.currentFunc = prevFunc
    
    // Add to module
    a.module.Functions = append(a.module.Functions, lambdaFunc)
    
    // Register in parent scope as function reference
    a.currentScope.Define(v.Name, &FuncSymbol{
        Name:       funcName,
        IsLambda:   true,
        ReturnType: lambdaFunc.ReturnType,
        Params:     lambdaFunc.Params,
    })
    
    return nil
}
```

#### 1.3 Transform Lambda Calls

In `analyzeCallExpr`:
```go
func (a *Analyzer) analyzeCallExpr(call *ast.CallExpr, irFunc *ir.Function) (ir.Register, error) {
    // Check if calling a lambda variable
    if id, ok := call.Function.(*ast.Identifier); ok {
        sym := a.currentScope.Lookup(id.Name)
        if funcSym, ok := sym.(*FuncSymbol); ok && funcSym.IsLambda {
            // Direct call to lambda function
            return a.generateDirectCall(funcSym.Name, call.Arguments, irFunc)
        }
    }
    // ... existing call handling
}
```

### Step 2: Add Capture Detection

```go
func (a *Analyzer) checkLambdaCaptures(lambda *ast.LambdaExpr) error {
    captured := make(map[string]bool)
    
    var checkExpr func(expr ast.Expr)
    checkExpr = func(expr ast.Expr) {
        switch e := expr.(type) {
        case *ast.Identifier:
            // Is this a parameter?
            isParam := false
            for _, p := range lambda.Params {
                if p.Name == e.Name {
                    isParam = true
                    break
                }
            }
            
            if !isParam {
                // Check if exists in outer scope
                if a.currentScope.Lookup(e.Name) != nil {
                    captured[e.Name] = true
                }
            }
        case *ast.BinaryExpr:
            checkExpr(e.Left)
            checkExpr(e.Right)
        // ... other expression types
        }
    }
    
    // Check lambda body
    for _, stmt := range lambda.Body.Statements {
        if exprStmt, ok := stmt.(*ast.ExpressionStmt); ok {
            checkExpr(exprStmt.Expression)
        }
    }
    
    if len(captured) > 0 {
        return fmt.Errorf("lambda captures variables %v (not yet supported)", captured)
    }
    return nil
}
```

### Step 3: Implement @curry Intrinsic

```go
func (a *Analyzer) analyzeCurryCall(call *ast.CallExpr, irFunc *ir.Function) (ir.Register, error) {
    if len(call.Arguments) != 2 {
        return 0, fmt.Errorf("@curry expects 2 arguments: lambda and value")
    }
    
    // Get lambda expression
    lambda, ok := call.Arguments[0].(*ast.LambdaExpr)
    if !ok {
        return 0, fmt.Errorf("@curry first argument must be a lambda")
    }
    
    // Get curry value
    valueReg, err := a.analyzeExpression(call.Arguments[1], irFunc)
    if err != nil {
        return 0, err
    }
    
    // Generate specialized function
    specializedName := fmt.Sprintf("curry_%s_%d", irFunc.Name, a.curryCounter)
    a.curryCounter++
    
    // Create SMC template function
    curryFunc := &ir.Function{
        Name:         specializedName,
        IsSMCDefault: true,
        UsesTrueSMC:  true,
    }
    
    // First parameter is curried, rest are normal
    if len(lambda.Params) > 1 {
        for i := 1; i < len(lambda.Params); i++ {
            param := lambda.Params[i]
            paramType, _ := a.convertType(param.Type)
            curryFunc.Params = append(curryFunc.Params, ir.Parameter{
                Name: param.Name,
                Type: paramType,
            })
        }
    }
    
    // Generate body with curried value
    // This would generate SMC code that patches the first parameter
    a.generateCurriedBody(curryFunc, lambda, valueReg)
    
    // Add to module
    a.module.Functions = append(a.module.Functions, curryFunc)
    
    // Return function address
    resultReg := irFunc.AllocReg()
    irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
        Op:    ir.OpLoadAddr,
        Dest:  resultReg,
        Label: specializedName,
    })
    
    return resultReg, nil
}
```

### Step 4: Update Code Generator

No changes needed! The code generator already handles:
- Regular function calls
- Function addresses (OpLoadAddr)
- SMC functions

### Step 5: Add Tests

```minz
// test_lambda_transform.minz

// Test 1: Basic lambda
fun test_basic() -> u8 {
    let add = |x: u8, y: u8| => u8 { x + y };
    add(5, 3)  // Should compile to direct call
}

// Test 2: Lambda reference
fun test_reference() -> u8 {
    let double = |x: u8| => u8 { x * 2 };
    let f = double;  // Just copying reference
    f(10)
}

// Test 3: Curry
fun test_curry() -> fn(u8) -> u8 {
    @curry(|x: u8, n: u8| => u8 { x + n }, 5)
}

// Test 4: Error on capture
fun test_capture_error() {
    let n = 10;
    // This should error:
    let bad = |x| { x + n };  // ERROR: captures 'n'
}
```

## Debugging Tips

### 1. Check Generated Function Names
```bash
# Look for generated lambda functions in assembly
./minzc examples/test_lambda.minz -o test.a80
grep "\\$.*_[0-9]:" test.a80  # Find generated functions
```

### 2. Verify Direct Calls
```bash
# Ensure lambda calls are direct, not indirect
grep "CALL.*\\$" test.a80  # Should see direct calls to generated functions
```

### 3. Check MIR Output
```bash
# Verify lambda transformation in MIR
./minzc examples/test_lambda.minz -o test.a80
cat test.mir | grep "Function.*\\$"  # See generated functions
```

## Common Issues and Solutions

### Issue 1: Lambda Not Recognized
**Symptom**: Lambda compiles as runtime value
**Solution**: Ensure lambda is assigned to variable, not passed directly

### Issue 2: Capture Detection Too Strict
**Symptom**: False positives for captures
**Solution**: Check that parameters are properly excluded from capture analysis

### Issue 3: Generated Names Clash
**Symptom**: Duplicate function names
**Solution**: Include more context in name generation (scope path)

## Future Enhancements

### 1. Capture Support
Transform captures into additional parameters:
```minz
fun outer(n: u8) {
    let add = |x| { x + n };  // Captures n
    add(5)  // Becomes: outer$add_0(5, n)
}
```

### 2. Lambda Deduplication
Detect identical lambdas and reuse functions:
```minz
let f1 = |x| { x + 1 };
let f2 = |x| { x + 1 };  // Same as f1, reuse function
```

### 3. Inline Optimization
For tiny lambdas, inline at call site:
```minz
map(arr, |x| { x * 2 });  // Inline multiplication in map's loop
```

## Conclusion

This implementation transforms MinZ lambdas from runtime constructs to compile-time functions, achieving zero-cost abstractions while maintaining expressive syntax. The key is recognizing that in a static, embedded environment like Z80, compile-time transformation is not a limitation but an optimization.