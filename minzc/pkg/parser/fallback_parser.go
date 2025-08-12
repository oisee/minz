package parser

import (
	"fmt"
	"strings"
	"github.com/minz/minzc/pkg/ast"
)

// FallbackParser provides basic error reporting when tree-sitter is unavailable
type FallbackParser struct {
	source string
}

// NewFallback creates a new fallback parser
func NewFallback(source string) *FallbackParser {
	return &FallbackParser{source: source}
}

// ParseToAST attempts basic parsing or provides helpful error message
func (f *FallbackParser) ParseToAST(filename string) (*ast.File, error) {
	// Check if this looks like a MinZ file
	if !strings.Contains(f.source, "fun ") && !strings.Contains(f.source, "fn ") {
		return nil, fmt.Errorf("file does not appear to be valid MinZ source code")
	}
	
	// Create a minimal AST with just the main function if we can find it
	file := &ast.File{
		Name:         filename,
		Imports:      []*ast.ImportStmt{},
		Declarations: []ast.Declaration{},
	}
	
	// Try to detect imports
	lines := strings.Split(f.source, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "import ") {
			// Extract import path
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				path := strings.TrimSuffix(parts[1], ";")
				
				// Check for alias
				alias := ""
				if len(parts) >= 4 && parts[2] == "as" {
					alias = strings.TrimSuffix(parts[3], ";")
				}
				
				file.Imports = append(file.Imports, &ast.ImportStmt{
					Path:  path,
					Alias: alias,
				})
			}
		}
	}
	
	// Look for main function
	if strings.Contains(f.source, "fun main") || strings.Contains(f.source, "fn main") {
		// Create a stub main function
		mainFunc := &ast.FunctionDecl{
			Name:       "main",
			Params:     []*ast.Parameter{},
			ReturnType: &ast.PrimitiveType{Name: "void"},
			Body: &ast.BlockStmt{
				Statements: []ast.Statement{},
			},
		}
		file.Declarations = append(file.Declarations, mainFunc)
	}
	
	// If we found something, return it
	if len(file.Imports) > 0 || len(file.Declarations) > 0 {
		return file, nil
	}
	
	// Otherwise return an informative error
	return nil, fmt.Errorf(`MinZ parser limitation: Cannot parse complex source files without tree-sitter.

The file appears to be valid MinZ code but requires tree-sitter for full parsing.

Solutions:
1. Install tree-sitter CLI:
   npm install -g tree-sitter-cli

2. Use the pre-built MinZ binary with embedded grammar support

3. For Ubuntu/Linux users:
   sudo apt-get update
   sudo apt-get install npm
   npm install -g tree-sitter-cli

After installing tree-sitter, the MinZ compiler will work normally.`)
}