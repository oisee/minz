package main

import (
	"fmt"
	"os"
	"github.com/minz/minzc/pkg/parser"
)

func main() {
	// Create a temp file with the content
	file, _ := os.CreateTemp("", "test*.minz")
	file.WriteString("const Y: u8 = @lua(20);")
	file.Close()
	
	p := parser.NewSimpleParser()
	// Access internal tokenization
	fp, _ := os.Open(file.Name())
	p.Tokenize(fp)
	fp.Close()
	
	// Print tokens
	for i, tok := range p.Tokens {
		fmt.Printf("Token %d: Type=%v Value='%s'\n", i, tok.Type, tok.Value)
	}
	
	os.Remove(file.Name())
}