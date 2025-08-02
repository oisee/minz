package main

import (
	"fmt"
	"minzc/pkg/ast"
	"minzc/pkg/parser"
)

func main() {
	code := `fun main() -> void {
    let add_one = |x: u8| => u8 { x + 1 };
}`
	
	p := parser.NewParser()
	program, err := p.ParseBytes([]byte(code))
	if err != nil {
		fmt.Printf("Parse error: %v\n", err)
		return
	}
	
	// Find main function
	for _, decl := range program.Declarations {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name == "main" {
			fmt.Printf("Found main function with %d statements\n", len(fn.Body.Statements))
			
			// Check first statement
			if len(fn.Body.Statements) > 0 {
				if varDecl, ok := fn.Body.Statements[0].(*ast.VarDecl); ok {
					fmt.Printf("VarDecl: name=%s, isMutable=%v\n", varDecl.Name, varDecl.IsMutable)
					fmt.Printf("  Type: %v\n", varDecl.Type)
					fmt.Printf("  Value type: %T\n", varDecl.Value)
					
					if lambda, ok := varDecl.Value.(*ast.LambdaExpr); ok {
						fmt.Printf("  Lambda found!\n")
						fmt.Printf("    Params: %d\n", len(lambda.Params))
						fmt.Printf("    ReturnType: %v\n", lambda.ReturnType)
						fmt.Printf("    Body type: %T\n", lambda.Body)
					}
				}
			}
		}
	}
}