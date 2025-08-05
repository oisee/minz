# 135. Function Overloading Implementation Guide

## Quick Implementation Plan

### Step 1: Modify Symbol Table (semantic/symbol_table.go)

```go
// Current: Functions stored by name only
type SymbolTable struct {
    functions map[string]*FunctionSymbol
}

// New: Functions stored in overload sets
type SymbolTable struct {
    functions map[string]*FunctionOverloadSet
}

type FunctionOverloadSet struct {
    Name       string
    Overloads  []*FunctionSymbol
}

// Add function to overload set
func (st *SymbolTable) AddFunction(fn *FunctionSymbol) error {
    set, exists := st.functions[fn.Name]
    if !exists {
        set = &FunctionOverloadSet{Name: fn.Name}
        st.functions[fn.Name] = set
    }
    
    // Check for duplicate signature
    mangled := mangleFunctionName(fn)
    for _, existing := range set.Overloads {
        if mangleFunctionName(existing) == mangled {
            return fmt.Errorf("function %s with same signature already exists", fn.Name)
        }
    }
    
    set.Overloads = append(set.Overloads, fn)
    return nil
}
```

### Step 2: Implement Name Mangling (semantic/mangle.go)

```go
package semantic

import (
    "fmt"
    "strings"
)

// Mangle function name for assembly
func MangleFunctionName(fn *FunctionSymbol) string {
    parts := []string{fn.Name}
    
    for _, param := range fn.Params {
        parts = append(parts, mangleType(param.Type))
    }
    
    return strings.Join(parts, "$")
}

func mangleType(t Type) string {
    switch t := t.(type) {
    case *BasicType:
        switch t.Kind {
        case U8:  return "u8"
        case U16: return "u16"
        case U24: return "u24" 
        case I8:  return "i8"
        case I16: return "i16"
        case Bool: return "b"
        }
    case *PointerType:
        return "p_" + mangleType(t.Base)
    case *ArrayType:
        return fmt.Sprintf("a_%s_%d", mangleType(t.Elem), t.Size)
    case *StructType:
        return t.Name
    case *StringType:
        return "str"
    }
    return "unknown"
}
```

### Step 3: Modify Function Call Resolution (semantic/analyzer.go)

```go
// Current: Simple lookup
func (a *Analyzer) resolveFunctionCall(call *ast.CallExpr) (*FunctionSymbol, error) {
    fn := a.symbolTable.LookupFunction(call.Name)
    if fn == nil {
        return nil, fmt.Errorf("undefined function: %s", call.Name)
    }
    return fn, nil
}

// New: Overload resolution
func (a *Analyzer) resolveFunctionCall(call *ast.CallExpr) (*FunctionSymbol, error) {
    // Get argument types
    argTypes := make([]Type, len(call.Args))
    for i, arg := range call.Args {
        t, err := a.analyzeExpr(arg)
        if err != nil {
            return nil, err
        }
        argTypes[i] = t
    }
    
    // Find matching overload
    overloadSet := a.symbolTable.GetOverloadSet(call.Name)
    if overloadSet == nil {
        return nil, fmt.Errorf("undefined function: %s", call.Name)
    }
    
    // Find exact match
    for _, fn := range overloadSet.Overloads {
        if matchesSignature(fn, argTypes) {
            return fn, nil
        }
    }
    
    // No match found - generate helpful error
    return nil, fmt.Errorf("no matching overload for %s(%s)", 
        call.Name, formatTypes(argTypes))
}

func matchesSignature(fn *FunctionSymbol, argTypes []Type) bool {
    if len(fn.Params) != len(argTypes) {
        return false
    }
    
    for i, param := range fn.Params {
        if !typesEqual(param.Type, argTypes[i]) {
            return false
        }
    }
    
    return true
}
```

### Step 4: Update Code Generation (codegen/codegen.go)

```go
// Generate call with mangled name
func (g *Generator) generateCall(call *ir.CallInst) {
    mangledName := semantic.MangleFunctionName(call.Function)
    g.emit("call %s", mangledName)
}

// Generate function with mangled name
func (g *Generator) generateFunction(fn *ir.Function) {
    mangledName := semantic.MangleFunctionName(fn.Symbol)
    g.emit("%s:", mangledName)
    // ... rest of function generation
}
```

### Step 5: Update IR Builder (ir/builder.go)

```go
// Store the resolved function symbol in call instruction
type CallInst struct {
    Function *semantic.FunctionSymbol  // Resolved overload
    Args     []Value
}
```

## Testing the Implementation

### Test Case 1: Basic Overloading
```minz
fun add(a: u8, b: u8) -> u8 { return a + b; }
fun add(a: u16, b: u16) -> u16 { return a + b; }

fun test() -> void {
    let x = add(1, 2);        // Calls add$u8$u8
    let y = add(1000, 2000);  // Calls add$u16$u16
}
```

### Test Case 2: Error Detection
```minz
fun process(x: u8) -> void { }
fun process(x: u8) -> void { }  // ERROR: Duplicate

fun test() -> void {
    process(1.0);  // ERROR: No overload for f32
}
```

### Test Case 3: Complex Types
```minz
fun fill(arr: *u8, len: u16, val: u8) -> void { }
fun fill(arr: *u16, len: u16, val: u16) -> void { }

fun test() -> void {
    let bytes: [u8; 10];
    let words: [u16; 10];
    
    fill(&bytes[0], 10, 0);   // Calls fill$p_u8$u16$u8
    fill(&words[0], 10, 0);   // Calls fill$p_u16$u16$u16
}
```

## Error Messages

Improve error messages for overloading:

```go
func formatOverloadError(name string, argTypes []Type, available []*FunctionSymbol) string {
    var msg strings.Builder
    fmt.Fprintf(&msg, "no matching function for call to '%s(", name)
    
    for i, t := range argTypes {
        if i > 0 { msg.WriteString(", ") }
        msg.WriteString(t.String())
    }
    msg.WriteString(")'")
    
    if len(available) > 0 {
        msg.WriteString("\nAvailable overloads:")
        for _, fn := range available {
            msg.WriteString("\n  " + fn.Signature())
        }
    }
    
    return msg.String()
}
```

Example error:
```
Error: no matching function for call to 'max(u8, u16)'
Available overloads:
  fun max(a: u8, b: u8) -> u8
  fun max(a: u16, b: u16) -> u16
  fun max(a: i8, b: i8) -> i8
```

## Integration Points

1. **Parser**: No changes needed - already supports multiple functions with same name
2. **Semantic Analysis**: Main changes here - overload resolution
3. **IR Generation**: Use mangled names
4. **Code Generation**: Use mangled names
5. **Error Reporting**: Improve to show available overloads

## Estimated Work

- Symbol table changes: 2 hours
- Name mangling: 1 hour  
- Overload resolution: 3 hours
- Code generation updates: 1 hour
- Testing and debugging: 3 hours
- **Total: ~10 hours for basic implementation**

This is indeed a "quick win" that will dramatically improve MinZ usability!