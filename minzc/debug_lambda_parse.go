package main

import (
    "fmt"
    "minzc/pkg/parser"
)

func main() {
    code := `fun main() -> void {
    let add_one = |x: u8| => u8 { x + 1 };
}`
    
    p := parser.NewParser()
    program, err := p.Parse(code)
    if err != nil {
        fmt.Printf("Parse error: %v\n", err)
        return
    }
    
    // Print AST
    for _, decl := range program.Declarations {
        if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name == "main" {
            for _, stmt := range fn.Body.Statements {
                if varDecl, ok := stmt.(*ast.VarDecl); ok {
                    fmt.Printf("VarDecl: %s\n", varDecl.Name)
                    fmt.Printf("  Value type: %T\n", varDecl.Value)
                    fmt.Printf("  Value: %+v\n", varDecl.Value)
                }
            }
        }
    }
}