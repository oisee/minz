package main

import (
	"fmt"
	"os"
	
	"github.com/minz/minzc/pkg/parser"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: tokentest <file>")
		return
	}
	
	p := parser.NewSimpleParser()
	
	// We need to access the tokenization, but it's internal
	// For now, let's just try parsing
	ast, err := p.ParseFile(os.Args[1])
	if err != nil {
		fmt.Printf("Parse error: %v\n", err)
		return
	}
	
	fmt.Printf("Parsed successfully. Module: %s\n", ast.Name)
	fmt.Printf("Declarations: %d\n", len(ast.Declarations))
	
	for _, decl := range ast.Declarations {
		switch d := decl.(type) {
		case *parser.VarDecl:
			fmt.Printf("  VarDecl: %s\n", d.Name)
			if d.Type != nil {
				fmt.Printf("    Type: %T\n", d.Type)
			}
		}
	}
}