package participle

import (
	"testing"

	"github.com/alecthomas/participle/v2"
)

// Create a parser for statements for testing
var stmtParser = participle.MustBuild[Stmt](
	participle.Lexer(MinZLexer),
	participle.Elide("whitespace", "Newline", "LineComment", "BlockComment"),
	participle.UseLookahead(3),
)

var blockParser = participle.MustBuild[StmtBlock](
	participle.Lexer(MinZLexer),
	participle.Elide("whitespace", "Newline", "LineComment", "BlockComment"),
	participle.UseLookahead(3),
)

func TestParseLetStmt(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple", "let x = 5;"},
		{"typed", "let x: u8 = 5;"},
		{"mutable", "let mut x = 5;"},
		{"typed_mutable", "let mut x: u16 = 100;"},
		{"no_init", "let x: u8;"},
		{"expr_init", "let x = a + b;"},
		{"no_semi", "let x = 5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := stmtParser.ParseString("test", tt.input)
			if err != nil {
				t.Errorf("failed to parse %q: %v", tt.input, err)
				return
			}
			if result.Let == nil {
				t.Errorf("expected Let statement, got %+v", result)
			}
		})
	}
}

func TestParseConstStmt(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple", "const MAX = 100;"},
		{"typed", "const MAX: u16 = 1000;"},
		{"expr", "const DOUBLE = MAX * 2;"},
		{"no_semi", "const X = 5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := stmtParser.ParseString("test", tt.input)
			if err != nil {
				t.Errorf("failed to parse %q: %v", tt.input, err)
				return
			}
			if result.Const == nil {
				t.Errorf("expected Const statement, got %+v", result)
			}
		})
	}
}

func TestParseReturnStmt(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"void", "return;"},
		{"value", "return 42;"},
		{"expr", "return a + b;"},
		{"no_semi", "return x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := stmtParser.ParseString("test", tt.input)
			if err != nil {
				t.Errorf("failed to parse %q: %v", tt.input, err)
				return
			}
			if result.Return == nil {
				t.Errorf("expected Return statement, got %+v", result)
			}
		})
	}
}

func TestParseIfStmt(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple", "if x { }"},
		{"condition", "if x > 0 { }"},
		{"with_else", "if x { } else { }"},
		{"else_if", "if x { } else if y { }"},
		{"full_chain", "if x { } else if y { } else { }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := stmtParser.ParseString("test", tt.input)
			if err != nil {
				t.Errorf("failed to parse %q: %v", tt.input, err)
				return
			}
			if result.If == nil {
				t.Errorf("expected If statement, got %+v", result)
			}
		})
	}
}

func TestParseWhileStmt(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple", "while x { }"},
		{"condition", "while x > 0 { }"},
		{"true", "while true { }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := stmtParser.ParseString("test", tt.input)
			if err != nil {
				t.Errorf("failed to parse %q: %v", tt.input, err)
				return
			}
			if result.While == nil {
				t.Errorf("expected While statement, got %+v", result)
			}
		})
	}
}

func TestParseForStmt(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"range", "for i in 0..10 { }"},
		{"expr_range", "for i in start..end { }"},
		{"collection", "for item in items { }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := stmtParser.ParseString("test", tt.input)
			if err != nil {
				t.Errorf("failed to parse %q: %v", tt.input, err)
				return
			}
			if result.For == nil {
				t.Errorf("expected For statement, got %+v", result)
			}
		})
	}
}

func TestParseLoopStmt(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"infinite", "loop { }"},
		{"into", "loop arr into item { }"},
		{"ref", "loop arr ref item { }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := stmtParser.ParseString("test", tt.input)
			if err != nil {
				t.Errorf("failed to parse %q: %v", tt.input, err)
				return
			}
			if result.Loop == nil {
				t.Errorf("expected Loop statement, got %+v", result)
			}
		})
	}
}

func TestParseBreakContinue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check func(*Stmt) bool
	}{
		{"break", "break;", func(s *Stmt) bool { return s.Break != nil }},
		{"break_no_semi", "break", func(s *Stmt) bool { return s.Break != nil }},
		{"continue", "continue;", func(s *Stmt) bool { return s.Continue != nil }},
		{"continue_no_semi", "continue", func(s *Stmt) bool { return s.Continue != nil }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := stmtParser.ParseString("test", tt.input)
			if err != nil {
				t.Errorf("failed to parse %q: %v", tt.input, err)
				return
			}
			if !tt.check(result) {
				t.Errorf("expected %s statement, got %+v", tt.name, result)
			}
		})
	}
}

func TestParseBlock(t *testing.T) {
	tests := []struct {
		name  string
		input string
		count int
	}{
		{"empty", "{}", 0},
		{"single", "{ let x = 5; }", 1},
		{"multiple", "{ let x = 5; let y = 10; return x + y; }", 3},
		{"nested", "{ if x { let y = 1; } }", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := blockParser.ParseString("test", tt.input)
			if err != nil {
				t.Errorf("failed to parse %q: %v", tt.input, err)
				return
			}
			if len(result.Statements) != tt.count {
				t.Errorf("expected %d statements, got %d", tt.count, len(result.Statements))
			}
		})
	}
}
