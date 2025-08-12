package parser

import (
	"fmt"
	"strings"
	"github.com/minz/minzc/pkg/ast"
)

// FallbackParser provides error reporting when tree-sitter is unavailable
type FallbackParser struct {
	source string
}

// NewFallback creates a new fallback parser
func NewFallback(source string) *FallbackParser {
	return &FallbackParser{source: source}
}

// ParseToAST returns error requiring tree-sitter installation
func (f *FallbackParser) ParseToAST(filename string) (*ast.File, error) {
	// Always fail - tree-sitter is a required dependency
	return nil, fmt.Errorf(`tree-sitter is required to parse MinZ source files.

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