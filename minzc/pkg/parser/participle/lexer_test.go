package participle

import (
	"testing"

	"github.com/alecthomas/participle/v2/lexer"
)

// tokenName returns the name of a token type by inverting the Symbols() map.
func tokenName(symbols map[string]lexer.TokenType, t lexer.TokenType) string {
	for name, typ := range symbols {
		if typ == t {
			return name
		}
	}
	return "Unknown"
}

func TestLexerTokenizes(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		tokens []string
	}{
		{
			name:  "simple identifiers",
			input: "foo bar baz",
			tokens: []string{"Ident", "Ident", "Ident", "EOF"},
		},
		{
			name:  "keywords",
			input: "fun let if else while for return",
			tokens: []string{"Ident", "Ident", "Ident", "Ident", "Ident", "Ident", "Ident", "EOF"},
		},
		{
			name:  "numbers",
			input: "42 0xFF 0b1010 3.14",
			tokens: []string{"Number", "HexNumber", "BinNumber", "Number", "EOF"},
		},
		{
			name:  "strings",
			input: `"hello" 'c' ` + "`raw`",
			tokens: []string{"String", "Char", "RawString", "EOF"},
		},
		{
			name:  "operators",
			input: "+ - * / % & | ^ ~ ! < > =",
			tokens: []string{"Plus", "Minus", "Star", "Slash", "Percent", "Amp", "Pipe", "Caret", "Tilde", "Bang", "Lt", "Gt", "Eq", "EOF"},
		},
		{
			name:  "multi-char operators",
			input: "== != <= >= && || << >> -> => .. :: ??",
			tokens: []string{"EqEq", "NotEq", "LtEq", "GtEq", "AndAnd", "OrOr", "LtLt", "GtGt", "Arrow", "FatArrow", "DotDot", "ColonColon", "QQ", "EOF"},
		},
		{
			name:  "brackets",
			input: "( ) { } [ ]",
			tokens: []string{"LParen", "RParen", "LBrace", "RBrace", "LBracket", "RBracket", "EOF"},
		},
		{
			name:  "punctuation",
			input: ". , : ; ? @",
			tokens: []string{"Dot", "Comma", "Colon", "Semi", "Question", "At", "EOF"},
		},
		{
			name:  "function declaration",
			input: "fun add(a: u8, b: u8) -> u8 { return a + b; }",
			tokens: []string{
				"Ident", "Ident", "LParen", "Ident", "Colon", "Ident", "Comma",
				"Ident", "Colon", "Ident", "RParen", "Arrow", "Ident",
				"LBrace", "Ident", "Ident", "Plus", "Ident", "Semi", "RBrace", "EOF",
			},
		},
		{
			name:  "line comment",
			input: "foo // this is a comment\nbar",
			tokens: []string{"Ident", "Newline", "Ident", "EOF"},
		},
		{
			name:  "block comment",
			input: "foo /* block */ bar",
			tokens: []string{"Ident", "Ident", "EOF"},
		},
		{
			name:  "metafunction",
			input: "@print @if @define",
			tokens: []string{"At", "Ident", "At", "Ident", "At", "Ident", "EOF"},
		},
		{
			name:  "generic type",
			input: "Vec<T>",
			tokens: []string{"Ident", "Lt", "Ident", "Gt", "EOF"},
		},
		{
			name:  "scope resolution",
			input: "State::IDLE",
			tokens: []string{"Ident", "ColonColon", "Ident", "EOF"},
		},
		{
			name:  "string with escape",
			input: `"hello\nworld"`,
			tokens: []string{"String", "EOF"},
		},
		{
			name:  "compound assignment",
			input: "+= -= *= /= %= &= |= ^=",
			tokens: []string{"PlusEq", "MinusEq", "StarEq", "SlashEq", "PercentEq", "AmpEq", "PipeEq", "CaretEq", "EOF"},
		},
		{
			name:  "shift assign operators",
			input: "<<= >>=",
			tokens: []string{"LtLtEq", "GtGtEq", "EOF"},
		},
		{
			name:  "empty input",
			input: "",
			tokens: []string{"EOF"},
		},
		{
			name:  "hash token",
			input: "# #{}",
			tokens: []string{"Hash", "Hash", "LBrace", "RBrace", "EOF"},
		},
		{
			name:  "lambda",
			input: "|x, y| => x + y",
			tokens: []string{"Pipe", "Ident", "Comma", "Ident", "Pipe", "FatArrow", "Ident", "Plus", "Ident", "EOF"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lex, err := MinZLexer.LexString("test", tt.input)
			if err != nil {
				t.Fatalf("failed to create lexer: %v", err)
			}

			symbols := MinZLexer.Symbols()
			var got []string
			for {
				tok, err := lex.Next()
				if err != nil {
					t.Fatalf("lexer error: %v", err)
				}

				typeName := tokenName(symbols, tok.Type)
				// Skip whitespace and comments for comparison (Newline tokens are kept)
				if typeName == "whitespace" || typeName == "LineComment" || typeName == "BlockComment" {
					continue
				}

				got = append(got, typeName)

				if tok.EOF() {
					break
				}
			}

			if len(got) != len(tt.tokens) {
				t.Errorf("token count mismatch: got %d, want %d\ngot:  %v\nwant: %v", len(got), len(tt.tokens), got, tt.tokens)
				return
			}

			for i := range got {
				if got[i] != tt.tokens[i] {
					t.Errorf("token[%d] = %q, want %q", i, got[i], tt.tokens[i])
				}
			}
		})
	}
}

func TestIsKeyword(t *testing.T) {
	keywords := []string{
		"fun", "fn", "return",
		"let", "var", "const", "global", "mut",
		"if", "else", "elif", "while", "for", "in", "loop", "break", "continue", "match", "defer", "case", "when",
		"struct", "enum", "impl", "trait", "type", "interface",
		"import", "export", "as", "pub", "mod",
		"true", "false",
		"self", "Self", "nil", "null",
		"asm", "mir",
		"or", "and",
		"sizeof", "alignof",
		"u8", "u16", "u24", "u32", "i8", "i16", "i24", "i32", "bool", "void", "str", "ptr",
	}
	for _, kw := range keywords {
		if !IsKeyword(kw) {
			t.Errorf("%q should be a keyword", kw)
		}
	}

	notKeywords := []string{"foo", "bar", "myFunc", "counter", "x", "main", "println"}
	for _, id := range notKeywords {
		if IsKeyword(id) {
			t.Errorf("%q should not be a keyword", id)
		}
	}
}

func TestIsPrimitiveType(t *testing.T) {
	primitives := []string{"u8", "u16", "u24", "u32", "i8", "i16", "i24", "i32", "bool", "void", "str", "ptr"}
	for _, prim := range primitives {
		if !IsPrimitiveType(prim) {
			t.Errorf("%q should be a primitive type", prim)
		}
	}

	notPrimitives := []string{"MyType", "Vec", "List", "fun"}
	for _, id := range notPrimitives {
		if IsPrimitiveType(id) {
			t.Errorf("%q should not be a primitive type", id)
		}
	}
}

func TestLexerPositions(t *testing.T) {
	input := "fun foo() {\n    return 42;\n}"

	lex, err := MinZLexer.LexString("test.minz", input)
	if err != nil {
		t.Fatalf("failed to create lexer: %v", err)
	}

	// Check first token position
	tok, err := lex.Next()
	if err != nil {
		t.Fatalf("lexer error: %v", err)
	}

	if tok.Pos.Line != 1 {
		t.Errorf("first token line = %d, want 1", tok.Pos.Line)
	}
	if tok.Pos.Column != 1 {
		t.Errorf("first token column = %d, want 1", tok.Pos.Column)
	}
}
