// Package participle provides a Participle-based parser for MinZ source files.
// This replaces the tree-sitter parser to eliminate OOM issues on large files.
package participle

import (
	"github.com/alecthomas/participle/v2/lexer"
)

// MinZLexer is the lexer definition for MinZ source files.
// It handles all tokens including keywords, operators, literals, and comments.
var MinZLexer = lexer.MustStateful(lexer.Rules{
	"Root": {
		// Whitespace (skip)
		{Name: "whitespace", Pattern: `[ \t\r]+`, Action: nil},
		// Newlines (tracked for significant newline handling if needed)
		{Name: "Newline", Pattern: `\n`, Action: nil},

		// Comments (skip)
		{Name: "LineComment", Pattern: `//[^\n]*`, Action: nil},
		{Name: "BlockComment", Pattern: `/\*[^*]*\*+(?:[^/*][^*]*\*+)*/`, Action: nil},

		// Multi-character operators (must come before single-char)
		{Name: "FatArrow", Pattern: `=>`},
		{Name: "Arrow", Pattern: `->`},
		{Name: "DotDot", Pattern: `\.\.`},
		{Name: "ColonColon", Pattern: `::`},
		{Name: "EqEq", Pattern: `==`},
		{Name: "NotEq", Pattern: `!=`},
		{Name: "LtEq", Pattern: `<=`},
		{Name: "GtEq", Pattern: `>=`},
		{Name: "AndAnd", Pattern: `&&`},
		{Name: "OrOr", Pattern: `\|\|`},
		{Name: "LtLtEq", Pattern: `<<=`},
		{Name: "GtGtEq", Pattern: `>>=`},
		{Name: "LtLt", Pattern: `<<`},
		{Name: "GtGt", Pattern: `>>`},
		{Name: "QQ", Pattern: `\?\?`},
		{Name: "PlusEq", Pattern: `\+=`},
		{Name: "MinusEq", Pattern: `-=`},
		{Name: "StarEq", Pattern: `\*=`},
		{Name: "SlashEq", Pattern: `/=`},
		{Name: "PercentEq", Pattern: `%=`},
		{Name: "AmpEq", Pattern: `&=`},
		{Name: "PipeEq", Pattern: `\|=`},
		{Name: "CaretEq", Pattern: `\^=`},

		// Brackets and delimiters
		{Name: "LParen", Pattern: `\(`},
		{Name: "RParen", Pattern: `\)`},
		{Name: "LBrace", Pattern: `\{`},
		{Name: "RBrace", Pattern: `\}`},
		{Name: "LBracket", Pattern: `\[`},
		{Name: "RBracket", Pattern: `\]`},

		// Single-character operators
		{Name: "Plus", Pattern: `\+`},
		{Name: "Minus", Pattern: `-`},
		{Name: "Star", Pattern: `\*`},
		{Name: "Slash", Pattern: `/`},
		{Name: "Percent", Pattern: `%`},
		{Name: "Amp", Pattern: `&`},
		{Name: "Pipe", Pattern: `\|`},
		{Name: "Caret", Pattern: `\^`},
		{Name: "Tilde", Pattern: `~`},
		{Name: "Bang", Pattern: `!`},
		{Name: "Lt", Pattern: `<`},
		{Name: "Gt", Pattern: `>`},
		{Name: "Eq", Pattern: `=`},
		{Name: "Dot", Pattern: `\.`},
		{Name: "Comma", Pattern: `,`},
		{Name: "Colon", Pattern: `:`},
		{Name: "Semi", Pattern: `;`},
		{Name: "Question", Pattern: `\?`},
		{Name: "At", Pattern: `@`},
		{Name: "Hash", Pattern: `#`},

		// String literals (with escape sequences)
		{Name: "String", Pattern: `"(?:[^"\\]|\\.)*"`},
		{Name: "Char", Pattern: `'(?:[^'\\]|\\.)'`},
		{Name: "RawString", Pattern: "`[^`]*`"},

		// Number literals (order matters: hex/bin before decimal)
		{Name: "HexNumber", Pattern: `0[xX][0-9a-fA-F]+`},
		{Name: "BinNumber", Pattern: `0[bB][01]+`},
		{Name: "Number", Pattern: `[0-9]+(?:\.[0-9]+)?`},

		// Identifiers and keywords
		// Keywords are handled by participle's keyword detection
		{Name: "Ident", Pattern: `[a-zA-Z_][a-zA-Z0-9_]*`},
	},
})

// keywords maps keyword strings for use by IsKeyword.
// Unexported to prevent external mutation of global state.
var keywords = map[string]struct{}{
	// Function keywords
	"fun":      {},
	"fn":       {},
	"return":   {},

	// Variable keywords
	"let":      {},
	"var":      {},
	"const":    {},
	"global":   {},
	"mut":      {},

	// Control flow
	"if":       {},
	"else":     {},
	"elif":     {},
	"while":    {},
	"for":      {},
	"in":       {},
	"loop":     {},
	"break":    {},
	"continue": {},
	"match":    {},
	"defer":    {},
	"case":     {},
	"when":     {},

	// Type definitions
	"struct":   {},
	"enum":     {},
	"impl":     {},
	"trait":    {},
	"type":     {},
	"interface": {},

	// Modules
	"import":   {},
	"export":   {},
	"as":       {},
	"pub":      {},
	"mod":      {},

	// Boolean literals
	"true":     {},
	"false":    {},

	// Special
	"self":     {},
	"Self":     {},
	"nil":      {},
	"null":     {},

	// Inline code
	"asm":      {},
	"mir":      {},

	// Built-in operators (keyword form)
	"or":       {},
	"and":      {},

	// Built-in functions
	"sizeof":   {},
	"alignof":  {},

	// Primitive types (not strictly keywords, but reserved)
	"u8":       {},
	"u16":      {},
	"u24":      {},
	"u32":      {},
	"i8":       {},
	"i16":      {},
	"i24":      {},
	"i32":      {},
	"bool":     {},
	"void":     {},
	"str":      {},
	"ptr":      {},
}

// IsKeyword returns true if the given identifier is a keyword.
func IsKeyword(s string) bool {
	_, ok := keywords[s]
	return ok
}

// primitiveTypes is the set of primitive type names.
// Unexported to prevent external mutation of global state.
var primitiveTypes = map[string]struct{}{
	"u8":   {},
	"u16":  {},
	"u24":  {},
	"u32":  {},
	"i8":   {},
	"i16":  {},
	"i24":  {},
	"i32":  {},
	"bool": {},
	"void": {},
	"str":  {},
	"ptr":  {},
}

// IsPrimitiveType returns true if the given identifier is a primitive type.
func IsPrimitiveType(s string) bool {
	_, ok := primitiveTypes[s]
	return ok
}
