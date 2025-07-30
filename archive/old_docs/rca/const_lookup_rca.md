# Root Cause Analysis: Const Declaration Lookup Failure

Date: 2025-07-23

## Problem Statement
Constants declared with `const X: u8 = 42` are not found when referenced in functions, resulting in "undefined identifier" errors.

## Investigation Steps

1. **Const Parsing**: Works correctly - files with only const declarations compile
2. **Const Analysis**: analyzeConstDecl is called in first pass
3. **Symbol Storage**: Constants are stored as VarSymbol with IsMutable=false
4. **Symbol Names**: 
   - Stored with prefix: "module.X"
   - Also stored without prefix: "X"
   - Both point to the same VarSymbol with Name="module.X"

## Root Cause Hypothesis

The issue appears to be timing-related:
1. Constants are processed in the first pass
2. But the first pass processes ALL declarations including functions
3. When a function is processed in analyzeFunctionDecl, it might be clearing or replacing the scope

## Code Flow Analysis

1. Analyze() is called
2. First pass iterates declarations:
   - Registers function signatures
   - Processes structs
   - **Processes constants** (should add to global scope)
3. Second pass iterates declarations:
   - Processes function bodies (creates new scope)
   - Constants already processed (skipped)

## Potential Issue

When analyzeFunctionDecl is called in the FIRST pass (registerFunctionSignature), it might be modifying the scope. Let me check if registerFunctionSignature changes scope.