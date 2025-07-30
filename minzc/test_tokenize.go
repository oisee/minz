package main

import (
	"fmt"
	"github.com/minz/minzc/pkg/parser"
)

func main() {
	p := parser.NewSimpleParser()
	// Use the tokenization method directly
	tokens := p.TokenizeString("const X: u8 = @lua(8);")
	for _, tok := range tokens {
		fmt.Printf("Token: Type=%v Value='%s'\n", tok.Type, tok.Value)
	}
}