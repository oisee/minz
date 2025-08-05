# 063: MinZ Compiler Issues Analysis and Resolution Plan

## 1. Executive Summary

This document provides a comprehensive analysis of MinZ compiler issues found through systematic testing of 127 example files. The analysis reveals four critical bug categories affecting compilation success rates.

## 2. Test Results Overview

### 2.1 Statistics
- **Total Examples Tested**: 127
- **Passing**: ~15 files (12%)
- **Failing**: ~100 files (79%)
- **Hanging**: ~12 files (9%)

### 2.2 Passing Examples
1. `arithmetic_demo.minz` - Basic arithmetic operations
2. `const_only.minz` - Constant declarations only
3. `debug_bit_field.minz` - Bit field operations
4. `debug_const.minz` - Constant debugging
5. `debug_tokenize.minz` - Tokenization test
6. `fibonacci.minz` - Recursive function with single variables
7. `simple_add.minz` - Simple addition function
8. `mnist/test_minimal.minz` - Minimal test case
9. `mnist/test_basic.minz` - Basic function calls
10. `mnist/test_constant_only.minz` - Constants only
11. `mnist/test_explicit_return.minz` - Explicit returns
12. `mnist/test_field_assignment.minz` - Field assignments
13. `mnist/test_imports.minz` - Simple imports
14. `mnist/test_simple_import.minz` - Basic import
15. `mnist/test_void_main.minz` - Void main function

## 3. Critical Bug Categories

### 3.1 Bug Category A: Variable Scope Resolution Failure
**Severity**: Critical
**Impact**: 70% of failures

#### 3.1.1 Symptoms
- Second variable in any function scope cannot be resolved
- Error: `undefined identifier: <variable_name> (analyzeIdentifier)`

#### 3.1.2 Affected Examples
- `debug_scope.minz`: `undefined identifier: b`
- `editor_minimal_smc.minz`: `undefined identifier: y_wide`
- `game_sprite.minz`: Multiple variable resolution failures
- Most examples with 2+ variables per function

#### 3.1.3 Root Cause
The semantic analyzer's scope management incorrectly handles multiple variable declarations within the same function scope. Variables are declared but not properly registered in the scope lookup table.

### 3.2 Bug Category B: Type System Failures
**Severity**: High
**Impact**: 15% of failures

#### 3.2.1 Symptoms
- Custom types (enums, structs) not found during analysis
- Error: `undefined type: <TypeName>`

#### 3.2.2 Affected Examples
- `enums.minz`: `undefined type: Direction`, `undefined type: GameState`
- `structs.minz`: Struct type resolution failures
- Examples using custom type definitions

#### 3.2.3 Root Cause
Type registration happens after type usage in the semantic analysis phase, causing forward reference failures.

### 3.3 Bug Category C: Advanced Feature Parsing
**Severity**: High
**Impact**: 10% of failures (causes hangs)

#### 3.3.1 Symptoms
- Compiler enters infinite loop
- No error message, process must be killed

#### 3.3.2 Affected Examples
- `test_abi_comparison.minz`: `@abi` attributes
- `lua_assets.minz`: Lua metaprogramming
- `lua_metaprogramming.minz`: Compile-time code generation

#### 3.3.3 Root Cause
Unimplemented features cause the parser or analyzer to enter infinite loops without proper error handling.

### 3.4 Bug Category D: Module System Failures
**Severity**: Medium
**Impact**: 5% of failures

#### 3.4.1 Symptoms
- Import statements fail to resolve
- Module functions not found

#### 3.4.2 Affected Examples
- `modules/main.minz`: Import resolution
- `modules/game/*.minz`: Module exports

#### 3.4.3 Root Cause
Incomplete module system implementation.

## 4. Resolution Plan

### 4.1 Phase 1: Fix Variable Scope Resolution (Priority: CRITICAL)
**Target**: Fix 70% of failures

#### 4.1.1 Current Problem Code Location
File: `pkg/semantic/analyzer.go`
Function: `analyzeVarDeclInFunc` (lines 834-930)

#### 4.1.2 Proposed Fix
```go
// Before analyzing value expression, ensure variable is in scope
func (a *Analyzer) analyzeVarDeclInFunc(v *ast.VarDecl, irFunc *ir.Function) error {
    // 1. Determine type first
    var varType ir.Type
    if v.Type != nil {
        varType, _ = a.convertType(v.Type)
    }
    
    // 2. Allocate register
    reg := irFunc.AddLocal(v.Name, varType)
    
    // 3. CRITICAL: Register in scope BEFORE analyzing value
    a.currentScope.Define(v.Name, &VarSymbol{
        Name:      v.Name,
        Type:      varType,
        Reg:       reg,
        IsMutable: v.IsMutable,
    })
    
    // 4. NOW analyze the value expression (which might reference other vars)
    if v.Value != nil {
        valueReg, err := a.analyzeExpression(v.Value, irFunc)
        if err != nil {
            return err
        }
        irFunc.Emit(ir.OpStoreVar, reg, valueReg, 0)
    }
    
    return nil
}
```

### 4.2 Phase 2: Fix Type Registration Order (Priority: HIGH)
**Target**: Fix enum/struct type resolution

#### 4.2.1 Current Problem Code Location
File: `pkg/semantic/analyzer.go`
Function: `Analyze` (lines 51-115)

#### 4.2.2 Proposed Fix
```go
func (a *Analyzer) Analyze(file *ast.File) (*ir.Module, error) {
    // Phase 0: Register built-ins
    a.addBuiltins()
    
    // Phase 1: Register ALL types first (before any function analysis)
    for _, decl := range file.Declarations {
        switch d := decl.(type) {
        case *ast.StructDecl:
            if err := a.registerStructType(d); err != nil {
                return nil, err
            }
        case *ast.EnumDecl:
            if err := a.registerEnumType(d); err != nil {
                return nil, err
            }
        case *ast.TypeDecl:
            if err := a.registerTypeAlias(d); err != nil {
                return nil, err
            }
        }
    }
    
    // Phase 2: Register function signatures
    for _, decl := range file.Declarations {
        if fn, ok := decl.(*ast.FunctionDecl); ok {
            if err := a.registerFunctionSignature(fn); err != nil {
                return nil, err
            }
        }
    }
    
    // Phase 3: Analyze declarations (with all types available)
    for _, decl := range file.Declarations {
        if err := a.analyzeDeclaration(decl); err != nil {
            return nil, err
        }
    }
    
    return a.module, nil
}
```

### 4.3 Phase 3: Add Feature Guards (Priority: HIGH)
**Target**: Prevent hangs on unimplemented features

#### 4.3.1 Implementation
```go
// Add to analyzer.go
func (a *Analyzer) checkUnsupportedFeatures(node ast.Node) error {
    switch n := node.(type) {
    case *ast.AttributeExpr:
        switch n.Name {
        case "abi":
            return fmt.Errorf("@abi attribute not yet implemented")
        case "interrupt":
            return fmt.Errorf("@interrupt attribute not yet implemented")
        }
    case *ast.LuaExpression:
        return fmt.Errorf("Lua metaprogramming not yet implemented")
    }
    return nil
}
```

### 4.4 Phase 4: Improve Error Messages (Priority: MEDIUM)
**Target**: Better debugging information

#### 4.4.1 Implementation
- Add source location to all error messages
- Include variable declaration context in scope errors
- Show type resolution path for type errors

## 5. Implementation Steps

### 5.1 Step 1: Backup Current State
```bash
git add -A
git commit -m "Pre-fix state: Document compiler issues"
```

### 5.2 Step 2: Implement Variable Scope Fix
1. Modify `analyzeVarDeclInFunc` as specified
2. Test with `debug_scope.minz`
3. Verify fix with multi-variable examples

### 5.3 Step 3: Implement Type Registration Fix
1. Refactor `Analyze` function for proper phases
2. Add `registerStructType`, `registerEnumType` functions
3. Test with `enums.minz` and `structs.minz`

### 5.4 Step 4: Add Feature Guards
1. Implement `checkUnsupportedFeatures`
2. Call it early in analysis
3. Test with hanging examples

### 5.5 Step 5: Regression Testing
1. Re-run all 127 examples
2. Document new success rate
3. Identify any new issues

## 6. Success Criteria

### 6.1 Target Metrics
- **Phase 1 Success**: 80%+ examples compile (up from 12%)
- **Phase 2 Success**: 90%+ examples compile
- **Phase 3 Success**: 0 hanging compilations
- **Overall Goal**: 95%+ compilation success rate

### 6.2 Specific Test Cases
Must pass after fixes:
1. `debug_scope.minz` - Multiple variables
2. `enums.minz` - Enum types
3. `game_sprite.minz` - Complex example
4. `test_abi_comparison.minz` - Should error cleanly, not hang

## 7. Risk Assessment

### 7.1 Risks
1. **Scope fix might break working examples** - Mitigate with regression tests
2. **Type registration changes could affect performance** - Profile if needed
3. **Feature guards might be too restrictive** - Add flags to enable experimental features

### 7.2 Rollback Plan
- Git commit before each phase
- Test suite to verify no regressions
- Ability to revert individual fixes

## 8. Timeline

- **Phase 1**: 2-3 hours (Variable scope fix)
- **Phase 2**: 3-4 hours (Type registration)
- **Phase 3**: 1-2 hours (Feature guards)
- **Testing**: 2-3 hours
- **Total**: 8-12 hours of implementation

## 9. Next Steps

Begin with Phase 1 implementation as it will fix the majority of compilation failures.