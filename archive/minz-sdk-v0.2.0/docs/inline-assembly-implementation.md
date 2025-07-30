# Inline Assembly Implementation Guide

## Quick Implementation Steps

### 1. Add AST Node (ast/ast.go)
```go
// AsmStmt represents an inline assembly block
type AsmStmt struct {
    Name string  // Optional name for named blocks
    Code string  // Raw assembly text
    Pos  token.Pos
}

func (s *AsmStmt) stmtNode() {}
func (s *AsmStmt) Start() token.Pos { return s.Pos }
func (s *AsmStmt) End() token.Pos   { return s.Pos }
```

### 2. Update Parser (parser/parser.go)

Add to keywords:
```go
"asm": token.ASM,
```

Add parsing function:
```go
func (p *Parser) parseAsmStmt() ast.Statement {
    p.expect(token.ASM)
    
    // Optional name
    var name string
    if p.tok == token.IDENT {
        name = p.lit
        p.next()
    }
    
    p.expect(token.LBRACE)
    
    // Collect raw text until closing brace
    var code strings.Builder
    braceCount := 1
    
    for braceCount > 0 {
        switch p.tok {
        case token.LBRACE:
            braceCount++
        case token.RBRACE:
            braceCount--
            if braceCount == 0 {
                break
            }
        case token.EOF:
            p.error("unexpected EOF in asm block")
            return nil
        }
        
        if braceCount > 0 {
            code.WriteString(p.lit)
            code.WriteString(" ")
        }
        p.next()
    }
    
    return &ast.AsmStmt{
        Name: name,
        Code: strings.TrimSpace(code.String()),
        Pos:  p.pos,
    }
}
```

### 3. Update Semantic Analyzer (semantic/analyzer.go)

```go
func (a *Analyzer) analyzeStatement(stmt ast.Statement, irFunc *ir.Function) error {
    switch s := stmt.(type) {
    // ... existing cases ...
    
    case *ast.AsmStmt:
        return a.analyzeAsmStmt(s, irFunc)
    }
}

func (a *Analyzer) analyzeAsmStmt(asm *ast.AsmStmt, irFunc *ir.Function) error {
    // Create IR node
    asmBlock := &ir.AsmBlock{
        Name: asm.Name,
        Code: asm.Code,
    }
    
    // If named, register in symbol table
    if asm.Name != "" {
        a.currentScope.Define(asm.Name, &AsmSymbol{
            Name:  asm.Name,
            Block: asmBlock,
        })
    }
    
    // Add to function
    irFunc.AddInstruction(asmBlock)
    
    return nil
}
```

### 4. Update IR (ir/ir.go)

```go
// AsmBlock represents inline assembly
type AsmBlock struct {
    Name string
    Code string
}

func (a *AsmBlock) String() string {
    if a.Name != "" {
        return fmt.Sprintf("asm %s { %s }", a.Name, a.Code)
    }
    return fmt.Sprintf("asm { %s }", a.Code)
}
```

### 5. Update Code Generator (codegen/z80.go)

```go
func (g *Generator) generateInstruction(inst ir.Instruction) {
    switch i := inst.(type) {
    // ... existing cases ...
    
    case *ir.AsmBlock:
        g.generateAsmBlock(i)
    }
}

func (g *Generator) generateAsmBlock(asm *ir.AsmBlock) {
    // Add label if named
    if asm.Name != "" {
        g.emit(asm.Name + ":")
    }
    
    // Process each line
    scanner := bufio.NewScanner(strings.NewReader(asm.Code))
    for scanner.Scan() {
        line := scanner.Text()
        processed := g.resolveAsmSymbols(line)
        g.emit(processed)
    }
}

func (g *Generator) resolveAsmSymbols(line string) string {
    // Simple regex to find !symbol patterns
    re := regexp.MustCompile(`!\w+(\.\w+)?`)
    
    return re.ReplaceAllStringFunc(line, func(match string) string {
        symbol := strings.TrimPrefix(match, "!")
        
        // Handle dotted notation
        if strings.Contains(symbol, ".") {
            parts := strings.Split(symbol, ".")
            // Look up asm block and its label
            if blockSym, ok := g.symbols[parts[0]]; ok {
                if asmSym, ok := blockSym.(*AsmSymbol); ok {
                    // For now, just use the block name with label
                    return parts[0] + "." + parts[1]
                }
            }
        }
        
        // Look up regular symbols
        if sym, ok := g.symbols[symbol]; ok {
            switch s := sym.(type) {
            case *VarSymbol:
                return s.Label  // Use generated label
            case *FuncSymbol:
                return s.Label
            case *ConstSymbol:
                return fmt.Sprintf("%d", s.Value)
            }
        }
        
        // Return unchanged if not found (let sjasmplus handle it)
        return match
    })
}
```

### 6. Symbol Table Addition

```go
type AsmSymbol struct {
    Name  string
    Block *ir.AsmBlock
}

func (s *AsmSymbol) SymbolName() string { return s.Name }
```

## Minimal Test Case

```minz
// test_asm.minz
const BORDER: u8 = $FE;
let color: u8 = 2;

fn set_border() -> void {
    asm {
        ld a, !color
        out (!BORDER), a
    }
    return;
}

fn main() -> void {
    set_border();
    return;
}
```

Expected output:
```asm
    ORG $8000

BORDER  EQU $FE
color:  DB 2

set_border:
    ; inline asm
    ld a, (color)
    out ($FE), a
    ret

main:
    call set_border
    ret
```

## Implementation Priority

1. **Phase 1**: Basic asm blocks without symbol resolution
2. **Phase 2**: Add !symbol resolution for variables and constants  
3. **Phase 3**: Add named asm blocks
4. **Phase 4**: Add cross-referencing between blocks

This minimal approach:
- Requires minimal parser changes
- No assembler implementation needed
- Preserves sjasmplus compatibility
- Allows gradual enhancement