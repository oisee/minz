package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/minz-lang/minz/pkg/parser"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run debug_tokens.go <file>")
		os.Exit(1)
	}

	content, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}

	// Create simple parser
	p, err := parser.NewSimpleParser(string(content), os.Args[1])
	if err != nil {
		panic(err)
	}

	// Extract tokens
	fmt.Printf("Tokens for: %s\n", strings.TrimSpace(string(content)))
	fmt.Println("=" + strings.Repeat("=", 40))

	// We need to access the tokens field, but it's not exported
	// Let's just try to parse and see what happens
	ast, err := p.Parse()
	if err != nil {
		fmt.Printf("Parse error: %v\n", err)
	} else {
		fmt.Printf("AST: %+v\n", ast)
	}
}