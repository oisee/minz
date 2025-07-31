package main

import (
	"fmt"
	"github.com/minz/minzc/pkg/parser"
)

func main() {
	p := parser.New()
	file, err := p.ParseFile("test_debug_pattern.minz")
	if err != nil {
		fmt.Printf("Parse error: %v\n", err)
		return
	}
	
	fmt.Printf("File parsed successfully: %s\n", file.Name)
	fmt.Printf("Declarations: %d\n", len(file.Declarations))
}