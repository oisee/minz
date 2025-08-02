package main

import (
	"encoding/json"
	"fmt"
	"os"
	
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
	
	// Pretty print the AST as JSON
	data, _ := json.MarshalIndent(program, "", "  ")
	fmt.Println(string(data))
}