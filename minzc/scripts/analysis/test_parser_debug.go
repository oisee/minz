package main

import (
	"fmt"
	"github.com/minz/minzc/pkg/parser"
)

func main() {
	p := parser.New()
	
	// Test string with escapes
	testCode := `fun main() -> void { let x = "Hello\nWorld"; }`
	
	ast, err := p.Parse("test.minz", testCode)
	if err != nil {
		fmt.Printf("Parse error: %v\n", err)
		return
	}
	
	// Find the string literal
	if file, ok := ast.(*parser.File); ok {
		fmt.Printf("Found file with %d declarations\n", len(file.Declarations))
		// Add more debugging as needed
	}
}