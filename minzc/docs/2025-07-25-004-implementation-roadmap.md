# MinZ Compiler Implementation Roadmap

## Quick Wins (Can be done immediately)

### 1. Fix Tree-sitter String Parsing (15 minutes)
**File**: `pkg/parser/parser.go:578`
**Change**: Add case for "string_literal" in parseExpression
```go
case "string_literal":
    value := n.Content()
    unquoted := value[1:len(value)-1] // Remove quotes
    return &ast.StringLiteral{
        Value:    unquoted,
        StartPos: int(n.StartPosition().Column),
        EndPos:   int(n.EndPosition().Column),
    }
```

### 2. Add Module Function Parameter Types (30 minutes)
**File**: `pkg/semantic/analyzer.go:227-260`
**Change**: Add parameter types to all module functions
```go
// Example for screen.set_pixel
Params: []ir.Type{
    &ir.BasicType{Kind: ir.TypeU8}, // x
    &ir.BasicType{Kind: ir.TypeU8}, // y  
    &ir.BasicType{Kind: ir.TypeU8}, // color
}
```

### 3. Array Type Validation (30 minutes)
**File**: `pkg/semantic/analyzer.go:1784`
**Change**: Replace TODO with actual type checking
```go
switch t := exprType.(type) {
case *ir.ArrayType:
    elementType = t.Element
case *ir.PointerType:
    elementType = &ir.BasicType{Kind: ir.TypeU8} // Assume byte pointer
default:
    return 0, nil, fmt.Errorf("cannot index non-array type %s", exprType)
}
```

## Critical Features (Priority 1)

### Array Element Assignment Implementation

**Estimated Time**: 4-6 hours

#### Step 1: Update Semantic Analyzer
**File**: `pkg/semantic/analyzer.go:941`
```go
case *ast.IndexExpr:
    // Analyze array, index, and value
    arrayReg, arrayType, err := a.analyzeExpression(target.Array, irFunc)
    if err != nil {
        return err
    }
    
    indexReg, _, err := a.analyzeExpression(target.Index, irFunc)
    if err != nil {
        return err
    }
    
    valueReg, valueType, err := a.analyzeExpression(stmt.Value, irFunc)
    if err != nil {
        return err
    }
    
    // Type checking
    var elementType ir.Type
    switch t := arrayType.(type) {
    case *ir.ArrayType:
        elementType = t.Element
    case *ir.PointerType:
        elementType = &ir.BasicType{Kind: ir.TypeU8}
    default:
        return fmt.Errorf("cannot index non-array type %s", arrayType)
    }
    
    if !typesMatch(elementType, valueType) {
        return fmt.Errorf("type mismatch: array element is %s, value is %s",
            elementType, valueType)
    }
    
    // Generate IR - using two instructions approach
    tempReg := a.allocateRegister()
    
    // Calculate address
    irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
        Op:   ir.OpAdd,
        Dest: tempReg,
        Src1: arrayReg,
        Src2: indexReg,
        Type: &ir.PointerType{Base: elementType},
    })
    
    // Store value
    irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
        Op:   ir.OpStoreDirect,
        Src1: valueReg,
        Src2: tempReg,
        Type: elementType,
    })
    
    a.freeRegister(tempReg)
    return nil
```

#### Step 2: Update Code Generator
**File**: `pkg/codegen/z80.go` (add after OpLoadIndex)
```go
case ir.OpStoreDirect:
    // Store Src1 to address in Src2
    // Load address to HL
    g.loadToHL(inst.Src2)
    
    // Load value to A (for byte) or DE (for word)
    if inst.Type != nil && inst.Type.Size() == 1 {
        g.loadToA(inst.Src1)
        g.emit("    LD (HL), A")
    } else {
        g.loadToDE(inst.Src1)
        g.emit("    LD (HL), E")
        g.emit("    INC HL")
        g.emit("    LD (HL), D")
    }
```

## Type System Enhancements (Priority 2)

### Type Promotion Rules (2-3 hours)

**File**: `pkg/semantic/analyzer.go:2037`
```go
func promoteTypes(left, right ir.Type) ir.Type {
    // Handle same types
    if typesMatch(left, right) {
        return left
    }
    
    leftBasic, leftOk := left.(*ir.BasicType)
    rightBasic, rightOk := right.(*ir.BasicType)
    
    if !leftOk || !rightOk {
        return left // Default to left type for non-basic types
    }
    
    // Promotion matrix
    // u8 + u16 -> u16
    // i8 + i16 -> i16  
    // u8 + i8 -> i16 (to handle sign)
    // u16 + i16 -> i16 (prefer signed for safety)
    
    if leftBasic.Kind == ir.TypeU16 || rightBasic.Kind == ir.TypeU16 {
        if leftBasic.Kind == ir.TypeI16 || rightBasic.Kind == ir.TypeI16 {
            return &ir.BasicType{Kind: ir.TypeI16}
        }
        return &ir.BasicType{Kind: ir.TypeU16}
    }
    
    if leftBasic.Kind == ir.TypeI16 || rightBasic.Kind == ir.TypeI16 {
        return &ir.BasicType{Kind: ir.TypeI16}
    }
    
    // Mix of u8 and i8 -> i16
    if (leftBasic.Kind == ir.TypeU8 && rightBasic.Kind == ir.TypeI8) ||
       (leftBasic.Kind == ir.TypeI8 && rightBasic.Kind == ir.TypeU8) {
        return &ir.BasicType{Kind: ir.TypeI16}
    }
    
    return left
}
```

## Module System Improvements (Priority 3)

### Generic Module Loading (1-2 days)

1. **Create Module Resolver Interface**
```go
type ModuleResolver interface {
    ResolveModule(name string) (*ModuleInfo, error)
    LoadModuleSymbols(module *ModuleInfo) (map[string]Symbol, error)
}

type FileModuleResolver struct {
    searchPaths []string
    cache       map[string]*ModuleInfo
}
```

2. **Update Import Processing**
- Remove hardcoded module checks
- Use resolver to find module files
- Parse module files to extract exports

## Testing Strategy

### Unit Tests for Each Fix
1. Array assignment: Test basic assignment, bounds checking
2. Type promotion: Test all type combinations
3. Module loading: Test file resolution, symbol extraction

### Integration Tests
1. Complete MNIST editor example
2. Screen manipulation programs
3. Data structure manipulation

## Success Metrics

- [ ] MNIST editor compiles without errors
- [ ] Array manipulation works correctly
- [ ] All example programs compile
- [ ] No hardcoded module names in compiler
- [ ] Type checking catches all invalid operations

## Timeline

**Week 1**: Quick wins + Array assignment
**Week 2**: Type system improvements
**Week 3**: Module system refactor
**Week 4**: Testing and documentation

This roadmap focuses on making MinZ fully functional rather than adding new features. Each item builds on the previous ones, creating a solid foundation for future development.