package main

import (
	"fmt"
	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/parser"
)

func main() {
	p := parser.NewSimpleParser()
	file, err := p.ParseFile("test_parser.minz")
	if err != nil {
		fmt.Printf("Parse error: %v\n", err)
		return
	}
	
	for _, decl := range file.Declarations {
		fmt.Printf("Declaration: %T\n", decl)
		if c, ok := decl.(*ast.ConstDecl); ok {
			fmt.Printf("  Name: %s\n", c.Name)
			fmt.Printf("  Type: %v\n", c.Type)
			fmt.Printf("  Value: %v (%T)\n", c.Value, c.Value)
		}
	}
}