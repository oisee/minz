package parser

import (
	"fmt"
	"os"

	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/parser/participle"
)

var debug = os.Getenv("DEBUG") != ""

// Parser handles parsing MinZ source files using Participle
type Parser struct {
	sourceCode string
}

// New creates a new parser
func New() *Parser {
	return &Parser{}
}

// ParseFile parses a MinZ source file and returns an AST
func (p *Parser) ParseFile(filename string) (*ast.File, error) {
	// Read the source file first
	sourceCode, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	p.sourceCode = string(sourceCode)

	// Use Participle parser (native Go, no external dependencies)
	participleParser := participle.New()
	participleFile, err := participleParser.ParseFile(filename)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	// Convert Participle AST to standard AST
	converter := participle.NewConverter(filename)
	file, err := converter.Convert(participleFile)
	if err != nil {
		return nil, fmt.Errorf("AST conversion error: %w", err)
	}

	if debug {
		fmt.Printf("DEBUG: Parsed %d declarations using Participle parser\n", len(file.Declarations))
		for i, decl := range file.Declarations {
			fmt.Printf("  Decl %d: %T\n", i, decl)
		}
	}

	return file, nil
}

// ParseString parses MinZ code from a string and returns declarations
func (p *Parser) ParseString(code string, context string) ([]ast.Declaration, error) {
	// Create a temporary file with the code
	tmpFile, err := os.CreateTemp("", "minz_gen_*.minz")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write the code to the temp file
	if _, err := tmpFile.WriteString(code); err != nil {
		return nil, fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	// Parse the temp file
	astFile, err := p.ParseFile(tmpFile.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to parse generated code: %w", err)
	}

	// Return the declarations
	return astFile.Declarations, nil
}
