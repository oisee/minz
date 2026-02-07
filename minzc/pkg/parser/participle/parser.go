// Package participle provides a Participle-based parser for MinZ source files.
// This replaces the tree-sitter parser to eliminate OOM issues on large files.
package participle

import (
	"fmt"
	"io"
	"os"

	"github.com/alecthomas/participle/v2"
)

// Parser is the MinZ parser using Participle
type Parser struct {
	parser *participle.Parser[File]
}

// New creates a new MinZ parser
func New() *Parser {
	p := participle.MustBuild[File](
		participle.Lexer(MinZLexer),
		participle.Elide("whitespace", "Newline", "LineComment", "BlockComment"),
		participle.UseLookahead(3),
	)
	return &Parser{parser: p}
}

// ParseString parses MinZ source from a string
func (p *Parser) ParseString(filename, source string) (*File, error) {
	file, err := p.parser.ParseString(filename, source)
	if err != nil {
		return nil, formatError(filename, source, err)
	}
	return file, nil
}

// ParseReader parses MinZ source from a reader
func (p *Parser) ParseReader(filename string, r io.Reader) (*File, error) {
	file, err := p.parser.Parse(filename, r)
	if err != nil {
		return nil, err
	}
	return file, nil
}

// ParseFile parses a MinZ source file
func (p *Parser) ParseFile(path string) (*File, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return p.ParseString(path, string(content))
}

// formatError formats a parse error with context
func formatError(filename, source string, err error) error {
	// For now, just return the error as-is
	// In the future, we can add source context
	return fmt.Errorf("%s: %w", filename, err)
}

// Global parser instance for convenience
var defaultParser = New()

// Parse parses MinZ source from a string using the default parser
func Parse(filename, source string) (*File, error) {
	return defaultParser.ParseString(filename, source)
}

// ParseFile parses a MinZ source file using the default parser
func ParseFile(path string) (*File, error) {
	return defaultParser.ParseFile(path)
}
