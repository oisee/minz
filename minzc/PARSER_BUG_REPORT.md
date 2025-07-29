# MinZ Parser Critical Bug Report

## Executive Summary

The MinZ compiler has a **critical parser bug** that prevents compilation of any function with more than one variable declaration. This affects 70%+ of all examples.

## Bug Details

### Description
The parser correctly parses only the **first** `let` statement in a function block. All subsequent `let` statements are incorrectly parsed as `ExpressionStmt` nodes containing just the variable name as an `Identifier`.

### Example
```minz
fun test() -> void {
    let x: u16 = 10    // ✅ Parsed as VarDecl
    let y: u16 = 20    // ❌ Parsed as ExpressionStmt(Identifier("y"))
    let z: u16 = 30    // ❌ Parsed as ExpressionStmt(Identifier("z"))
}
```

### Impact
- **70%+ of examples fail** to compile
- Error messages are misleading (report "undefined identifier")
- Variable scope resolution appears broken but is actually fine
- Type system appears broken but is actually fine

## Root Cause

The bug is in the **parser/AST generation layer**, not in the semantic analyzer. Possible locations:

1. **Tree-sitter grammar** (`grammar.js`)
2. **AST conversion** (`pkg/parser/parser.go`)
3. **Simple parser fallback** (`pkg/parser/simple_parser.go`)

## Evidence

Debug output clearly shows the issue:
```
DEBUG: Processing statement 0 in block
DEBUG: analyzeStatement processing *ast.VarDecl        // First let
DEBUG: analyzeVarDeclInFunc called for variable 'x'
DEBUG: Processing statement 1 in block
DEBUG: analyzeStatement processing *ast.ExpressionStmt  // Second let (WRONG!)
DEBUG: ExpressionStmt contains *ast.Identifier
  Identifier: y
```

## Recommended Fix

### Option 1: Fix the Parser (Correct Solution)
1. Debug the tree-sitter grammar rules for variable declarations
2. Check if there's a state management issue in the parser
3. Ensure the parser correctly recognizes multiple `let` statements

### Option 2: Semantic Analyzer Workaround (Temporary)
Detect misparsed variable declarations in the semantic analyzer and handle them specially. However, this won't work with the current AST structure since the type and value information is lost.

## Next Steps

1. **Immediate**: Document this as a known issue
2. **Short-term**: Investigate the parser code to find the bug
3. **Long-term**: Fix the parser and add comprehensive parser tests

## Workaround for Users

Until fixed, users must declare only one variable per function, which severely limits the language's usability.