package main

import (
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/parser"
	"os"
)

func main() {
	code := `fun main() -> void {
    let arr = [1, 2, 3];
}`
	
	p := parser.NewParser()
	ast, err := p.Parse(code)
	if err != nil {
		fmt.Printf("Parse error: %v\n", err)
		os.Exit(1)
	}
	
	// Output JSON representation
	data, _ := json.MarshalIndent(ast, "", "  ")
	fmt.Println(string(data))
}