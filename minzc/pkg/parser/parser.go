package parser

import (
	"fmt"
	"os"
	"strings"

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

	// Preprocess: wrap asm fun bodies in asm { } so the lexer captures them
	preprocessed := preprocessAsmFunctions(p.sourceCode)

	// Use Participle parser (native Go, no external dependencies)
	participleParser := participle.New()
	participleFile, err := participleParser.ParseString(filename, preprocessed)
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

// preprocessAsmFunctions wraps asm fun body contents in inner asm { } blocks.
// Before: pub asm fun foo() -> u8 { ; comment\n LD A,1\n RET\n }
// After:  pub asm fun foo() -> u8 { asm { ; comment\n LD A,1\n RET\n } }
// This allows the AsmBlock lexer token to capture the body as raw text,
// since assembly instructions and ; comments can't parse as MinZ statements.
func preprocessAsmFunctions(source string) string {
	// Quick check: if no "asm" keyword followed by "fun"/"fn", nothing to do
	if !strings.Contains(source, "asm") {
		return source
	}

	var result strings.Builder
	result.Grow(len(source) + 256)
	i := 0

	for i < len(source) {
		// Look for "asm" keyword that is followed by "fun" or "fn"
		if i+3 <= len(source) && source[i:i+3] == "asm" {
			// Check the character before "asm" is a word boundary
			if i > 0 && isIdentChar(source[i-1]) {
				result.WriteByte(source[i])
				i++
				continue
			}
			// Check character after "asm" is not an ident char (not "assembly" etc.)
			if i+3 < len(source) && isIdentChar(source[i+3]) {
				result.WriteByte(source[i])
				i++
				continue
			}

			// Skip whitespace after "asm" to find "fun"/"fn"
			j := i + 3
			for j < len(source) && (source[j] == ' ' || source[j] == '\t' || source[j] == '\n' || source[j] == '\r') {
				j++
			}

			// Check if followed by "fun" or "fn"
			isFun := j+3 <= len(source) && source[j:j+3] == "fun" && (j+3 >= len(source) || !isIdentChar(source[j+3]))
			isFn := j+2 <= len(source) && source[j:j+2] == "fn" && (j+2 >= len(source) || !isIdentChar(source[j+2]))

			if isFun || isFn {
				// This is "asm fun" or "asm fn" - find the opening brace of the body
				// First skip past the function signature to find the body '{'
				openBrace := findAsmFunBodyBrace(source, j)
				if openBrace >= 0 {
					// Find matching closing brace
					closeBrace := findMatchingBrace(source, openBrace)
					if closeBrace >= 0 {
						// Write everything up to and including the opening brace
						result.WriteString(source[i : openBrace+1])
						// Insert "asm {"
						result.WriteString(" asm {")
						// Write the body content (between braces)
						result.WriteString(source[openBrace+1 : closeBrace])
						// Insert closing "} " before the function's closing brace
						result.WriteString("} ")
						// Write the closing brace
						result.WriteByte('}')
						i = closeBrace + 1
						continue
					}
				}
			}
		}

		// Check for string literals to avoid matching inside strings
		if source[i] == '"' {
			start := i
			i++
			for i < len(source) && source[i] != '"' {
				if source[i] == '\\' {
					i++ // skip escaped char
				}
				i++
			}
			if i < len(source) {
				i++ // skip closing quote
			}
			result.WriteString(source[start:i])
			continue
		}

		// Check for line comments
		if i+1 < len(source) && source[i] == '/' && source[i+1] == '/' {
			start := i
			for i < len(source) && source[i] != '\n' {
				i++
			}
			result.WriteString(source[start:i])
			continue
		}

		// Check for block comments
		if i+1 < len(source) && source[i] == '/' && source[i+1] == '*' {
			start := i
			i += 2
			for i+1 < len(source) && !(source[i] == '*' && source[i+1] == '/') {
				i++
			}
			if i+1 < len(source) {
				i += 2
			}
			result.WriteString(source[start:i])
			continue
		}

		result.WriteByte(source[i])
		i++
	}

	return result.String()
}

func isIdentChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

// findAsmFunBodyBrace finds the opening brace '{' of an asm fun body.
// It skips the function signature: name, params, return type, etc.
// Returns -1 if not found.
func findAsmFunBodyBrace(source string, funKeywordPos int) int {
	i := funKeywordPos

	// Skip "fun" or "fn" keyword
	for i < len(source) && isIdentChar(source[i]) {
		i++
	}
	// Skip whitespace
	for i < len(source) && (source[i] == ' ' || source[i] == '\t' || source[i] == '\n' || source[i] == '\r') {
		i++
	}
	// Skip function name
	for i < len(source) && isIdentChar(source[i]) {
		i++
	}

	// Now scan forward looking for '{', handling parentheses for params
	// and other signature elements
	parenDepth := 0
	for i < len(source) {
		switch source[i] {
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		case '{':
			if parenDepth == 0 {
				return i
			}
		case ';':
			// Semicolon at paren depth 0 means extern declaration (no body)
			if parenDepth == 0 {
				return -1
			}
		case '"':
			// Skip string literal
			i++
			for i < len(source) && source[i] != '"' {
				if source[i] == '\\' {
					i++
				}
				i++
			}
		}
		i++
	}
	return -1
}

// findMatchingBrace finds the closing '}' that matches the opening '{' at pos.
// Handles nested braces within string literals.
func findMatchingBrace(source string, openPos int) int {
	depth := 1
	i := openPos + 1

	for i < len(source) && depth > 0 {
		switch source[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		case '"':
			// Skip string literals
			i++
			for i < len(source) && source[i] != '"' {
				if source[i] == '\\' {
					i++
				}
				i++
			}
		case '\'':
			// Skip char literals
			i++
			for i < len(source) && source[i] != '\'' {
				if source[i] == '\\' {
					i++
				}
				i++
			}
		}
		i++
	}
	return -1
}

// ParseString parses MinZ code from a string and returns declarations
func (p *Parser) ParseString(code string, context string) ([]ast.Declaration, error) {
	p.sourceCode = code

	// Preprocess: wrap asm fun bodies in asm { }
	preprocessed := preprocessAsmFunctions(code)

	// Parse directly from string
	participleParser := participle.New()
	participleFile, err := participleParser.ParseString(context, preprocessed)
	if err != nil {
		return nil, fmt.Errorf("failed to parse generated code: %w", err)
	}

	// Convert Participle AST to standard AST
	converter := participle.NewConverter(context)
	file, err := converter.Convert(participleFile)
	if err != nil {
		return nil, fmt.Errorf("AST conversion error: %w", err)
	}

	return file.Declarations, nil
}
