package participle

import (
	"testing"

	"github.com/alecthomas/participle/v2"
)

// Create a parser for expressions for testing
var exprParser = participle.MustBuild[Expression](
	participle.Lexer(MinZLexer),
	participle.Elide("whitespace", "Newline", "LineComment", "BlockComment"),
	participle.UseLookahead(2),
)

func TestParseNumber(t *testing.T) {
	tests := []struct {
		input string
	}{
		{"42"},
		{"0"},
		{"255"},
		{"0xFF"},
		{"0b1010"},
		{"3.14"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := exprParser.ParseString("test", tt.input)
			if err != nil {
				t.Errorf("failed to parse %q: %v", tt.input, err)
			}
		})
	}
}

func TestParseString(t *testing.T) {
	tests := []struct {
		input string
	}{
		{`"hello"`},
		{`"hello world"`},
		{`"line\nbreak"`},
		{`'c'`},
		{"`raw string`"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := exprParser.ParseString("test", tt.input)
			if err != nil {
				t.Errorf("failed to parse %q: %v", tt.input, err)
			}
		})
	}
}

func TestParseBooleans(t *testing.T) {
	tests := []string{"true", "false"}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := exprParser.ParseString("test", input)
			if err != nil {
				t.Errorf("failed to parse %q: %v", input, err)
			}
		})
	}
}

func TestParseIdentifier(t *testing.T) {
	tests := []string{"foo", "bar", "myVar", "_private", "count123"}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := exprParser.ParseString("test", input)
			if err != nil {
				t.Errorf("failed to parse %q: %v", input, err)
			}
		})
	}
}

func TestParseBinaryExpr(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"add", "1 + 2"},
		{"sub", "3 - 1"},
		{"mul", "2 * 3"},
		{"div", "6 / 2"},
		{"mod", "7 % 3"},
		{"and", "a && b"},
		{"or", "a || b"},
		{"bitand", "a & b"},
		{"bitor", "a | b"},
		{"bitxor", "a ^ b"},
		{"eq", "a == b"},
		{"neq", "a != b"},
		{"lt", "a < b"},
		{"gt", "a > b"},
		{"lte", "a <= b"},
		{"gte", "a >= b"},
		{"shl", "a << b"},
		{"shr", "a >> b"},
		{"complex", "a + b * c"},
		{"parens", "(a + b) * c"},
		{"chained", "a + b + c + d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := exprParser.ParseString("test", tt.input)
			if err != nil {
				t.Errorf("failed to parse %q: %v", tt.input, err)
			}
		})
	}
}

func TestParseUnaryExpr(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"neg", "-x"},
		{"not", "!flag"},
		{"bitnot", "~mask"},
		{"addr", "&ptr"},
		{"deref", "*ptr"},
		{"double_neg", "--x"},
		{"complex", "-a + b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := exprParser.ParseString("test", tt.input)
			if err != nil {
				t.Errorf("failed to parse %q: %v", tt.input, err)
			}
		})
	}
}

func TestParseFunctionCall(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"no_args", "foo()"},
		{"one_arg", "print(42)"},
		{"two_args", "add(1, 2)"},
		{"nested", "outer(inner())"},
		{"chained", "a.b.c()"},
		{"complex_args", "foo(1 + 2, bar())"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := exprParser.ParseString("test", tt.input)
			if err != nil {
				t.Errorf("failed to parse %q: %v", tt.input, err)
			}
		})
	}
}

func TestParseArrayIndex(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple", "arr[0]"},
		{"expr_index", "arr[i + 1]"},
		{"chained", "matrix[i][j]"},
		{"mixed", "get_array()[0]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := exprParser.ParseString("test", tt.input)
			if err != nil {
				t.Errorf("failed to parse %q: %v", tt.input, err)
			}
		})
	}
}

func TestParseFieldAccess(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple", "obj.field"},
		{"chained", "a.b.c"},
		{"method", "obj.method()"},
		{"mixed", "arr[0].field"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := exprParser.ParseString("test", tt.input)
			if err != nil {
				t.Errorf("failed to parse %q: %v", tt.input, err)
			}
		})
	}
}

func TestParseArrayLiteral(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", "[]"},
		{"single", "[1]"},
		{"multiple", "[1, 2, 3]"},
		{"nested", "[[1, 2], [3, 4]]"},
		{"exprs", "[a + b, c * d]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := exprParser.ParseString("test", tt.input)
			if err != nil {
				t.Errorf("failed to parse %q: %v", tt.input, err)
			}
		})
	}
}

func TestParseTernary(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple", "a ? b : c"},
		{"nested_cond", "a > b ? x : y"},
		{"nested_branch", "a ? b ? c : d : e"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := exprParser.ParseString("test", tt.input)
			if err != nil {
				t.Errorf("failed to parse %q: %v", tt.input, err)
			}
		})
	}
}

func TestParseComplexExpressions(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"method_chain", "obj.method1().method2().field"},
		{"mixed_ops", "arr[i].field + other[j].value"},
		{"cast", "x as u16"},
		{"func_call_chain", "get().transform().result"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := exprParser.ParseString("test", tt.input)
			if err != nil {
				t.Errorf("failed to parse %q: %v", tt.input, err)
			}
		})
	}
}
