# 041: Iterator Implementation Plan

## Overview

This document outlines the step-by-step implementation of Z80-native iterators in MinZ.

## Phase 1: `do N times` (Week 1)

### 1.1 AST Changes (Day 1)

Add to `ast.go`:
```go
// DoTimesStmt represents a counting loop
type DoTimesStmt struct {
    Count    Expression  // How many times
    Body     *BlockStmt
    StartPos Position
    EndPos   Position
}
```

### 1.2 Parser Changes (Day 2)

Update `simple_parser.go`:
```go
case "do":
    return p.parseDoStatement()

func (p *SimpleParser) parseDoStatement() ast.Statement {
    p.expect(TokenKeyword, "do")
    count := p.parseExpression()
    p.expect(TokenKeyword, "times")
    body := p.parseBlock()
    
    return &ast.DoTimesStmt{
        Count: count,
        Body:  body,
        // positions...
    }
}
```

### 1.3 Semantic Analysis (Day 3)

```go
func (a *Analyzer) analyzeDoTimesStmt(stmt *ast.DoTimesStmt, irFunc *ir.Function) error {
    // Analyze count expression
    countReg, err := a.analyzeExpression(stmt.Count, irFunc)
    
    // Generate IR
    loopLabel := a.genLabel("do_times")
    endLabel := a.genLabel("do_end")
    
    // Initialize counter
    counterReg := irFunc.AllocReg()
    irFunc.Emit(ir.OpMove, counterReg, countReg, 0)
    
    // Loop start
    irFunc.EmitLabel(loopLabel)
    
    // Check if done
    irFunc.Emit(ir.OpJumpIfZero, counterReg, 0, 0, endLabel)
    
    // Body
    a.analyzeStatement(stmt.Body, irFunc)
    
    // Decrement and loop
    irFunc.Emit(ir.OpDec, counterReg, counterReg, 0)
    irFunc.Emit(ir.OpJump, 0, 0, 0, loopLabel)
    
    irFunc.EmitLabel(endLabel)
}
```

### 1.4 Code Generation (Day 4)

```go
// Optimize for DJNZ pattern
if inst.Op == ir.OpDec && 
   nextInst.Op == ir.OpJumpIfNotZero &&
   inst.Dest == nextInst.Src1 {
    // Generate DJNZ
    g.emit("    DJNZ %s", nextInst.Symbol)
    skip = true
}
```

### 1.5 Testing (Day 5)

```minz
// Test basic counting
fun test_do_times() -> void {
    let mut count: u8 = 0;
    
    do 5 times {
        count = count + 1;
    }
    
    // count should be 5
}

// Test with variable
fun test_do_variable() -> void {
    let n: u8 = get_count();
    
    do n times {
        process();
    }
}
```

## Phase 2: Basic `loop at` (Week 2)

### 2.1 AST Changes (Day 1-2)

```go
// LoopAtStmt represents table iteration
type LoopAtStmt struct {
    Table      Expression   // Array/table to iterate
    Iterator   string       // Iterator variable name
    IsModifying bool        // Has ! prefix
    Body       *BlockStmt
    StartPos   Position
    EndPos     Position
}
```

### 2.2 Parser Support (Day 3-4)

```go
case "loop":
    return p.parseLoopStatement()

func (p *SimpleParser) parseLoopStatement() ast.Statement {
    p.expect(TokenKeyword, "loop")
    p.expect(TokenKeyword, "at")
    
    table := p.parseExpression()
    p.expect(TokenOperator, "->")
    
    // Check for ! prefix
    isModifying := false
    if p.peek().Value == "!" {
        isModifying = true
        p.advance()
    }
    
    iterator := p.expectIdent()
    body := p.parseBlock()
    
    return &ast.LoopAtStmt{
        Table:       table,
        Iterator:    iterator,
        IsModifying: isModifying,
        Body:        body,
    }
}
```

### 2.3 Work Area Management (Day 5)

```go
// In semantic analyzer
type WorkArea struct {
    Name     string
    Type     ir.Type
    BaseAddr int      // Offset in work area
    Size     int
}

func (a *Analyzer) allocateWorkArea(elemType ir.Type) *WorkArea {
    wa := &WorkArea{
        Name:     a.genWorkAreaName(),
        Type:     elemType,
        BaseAddr: a.currentWorkAreaOffset,
        Size:     elemType.Size(),
    }
    a.currentWorkAreaOffset += wa.Size
    return wa
}
```

## Phase 3: Write-Back Logic (Week 3)

### 3.1 IR Extensions

```go
// New opcodes
OpLoadToWork     // Copy from array to work area
OpStoreFromWork  // Copy from work area to array
OpIncPtr         // Increment pointer by size
```

### 3.2 Code Generation Pattern

For `loop at enemies -> !enemy`:

```go
// Setup
g.emit("    LD IX, %s", table)
g.emit("    LD B, %d", count)
g.emit(".loop_%d:", loopNum)

// Load to work area
g.emit("    ; Load to work area")
for i, field := range structType.Fields {
    g.emit("    LD A, (IX+%d)", field.Offset)
    g.emit("    LD (work_%s+%d), A", iterator, field.Offset)
}

// Body (uses work area)
// ...

// Write back (only if isModifying)
if isModifying {
    g.emit("    ; Write back")
    for i, field := range structType.Fields {
        g.emit("    LD A, (work_%s+%d)", iterator, field.Offset)
        g.emit("    LD (IX+%d), A", field.Offset)
    }
}

// Next element
g.emit("    LD DE, %d", elemSize)
g.emit("    ADD IX, DE")
g.emit("    DJNZ .loop_%d", loopNum)
```

## Phase 4: Advanced Features (Week 4)

### 4.1 Skip/Break Support

```minz
loop at data -> !item {
    if item.flag == 0 {
        skip;  // Continue without write-back
    }
    if item.done {
        break; // Exit loop
    }
    item.value = process(item.value);
}
```

### 4.2 Explicit Modify

```minz
loop at items -> item {
    item.temp = calculate();
    if item.temp > threshold {
        modify item;  // Selective write-back
    }
}
```

### 4.3 Optimization Passes

- Detect constant counts → unroll small loops
- Merge adjacent loads/stores
- Use LDIR for simple copies
- Eliminate redundant work area ops

## Testing Strategy

### Unit Tests
1. Parser tests for new syntax
2. Semantic analyzer tests
3. IR generation tests
4. Code generation tests

### Integration Tests
1. Simple counting loops
2. Array iteration (read-only)
3. Modifying iteration
4. Nested loops
5. Early exit (break/skip)
6. Performance benchmarks

### Example Test Program

```minz
type Sprite = struct {
    x: u8,
    y: u8,
    frame: u8
};

fun animate_sprites() -> void {
    let sprites: [Sprite; 10];
    
    // Initialize
    do 10 times {
        sprites[i].x = i * 16;
        sprites[i].y = 50;
        sprites[i].frame = 0;
    }
    
    // Animate
    loop at sprites -> !sprite {
        sprite.x = sprite.x + 1;
        sprite.frame = (sprite.frame + 1) & 7;
    }
}
```

## Success Criteria

1. `do N times` generates single DJNZ instruction
2. Work areas correctly allocated and managed
3. Read-only loops have no write-back overhead
4. Modification tracking works correctly
5. Performance better than indexed access
6. Clear error messages for misuse

## Risk Mitigation

1. **Complex work areas**: Start with simple types, add structs later
2. **Memory usage**: Implement work area pooling if needed
3. **Parser complexity**: Consider separate keyword for modification
4. **Breaking changes**: Keep old syntax working during transition

## Timeline

- Week 1: `do N times` complete
- Week 2: Basic `loop at` working
- Week 3: Write-back and modification
- Week 4: Polish and optimization

Total: 1 month to production-ready