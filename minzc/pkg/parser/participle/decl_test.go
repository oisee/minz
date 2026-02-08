package participle

import (
	"testing"

	"github.com/alecthomas/participle/v2"
)

// Create parsers for testing
var declParser = participle.MustBuild[Decl](
	participle.Lexer(MinZLexer),
	participle.Elide("whitespace", "Newline", "LineComment", "BlockComment"),
	participle.UseLookahead(3),
)

var fileParser = participle.MustBuild[File](
	participle.Lexer(MinZLexer),
	participle.Elide("whitespace", "Newline", "LineComment", "BlockComment"),
	participle.UseLookahead(3),
)

func TestParseImportDecl(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple", "import foo;"},
		{"dotted", "import foo.bar;"},
		{"deep", "import std.io.file;"},
		{"alias", "import long.module.name as short;"},
		{"no_semi", "import foo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := declParser.ParseString("test", tt.input)
			if err != nil {
				t.Errorf("failed to parse %q: %v", tt.input, err)
				return
			}
			if result.Import == nil {
				t.Errorf("expected Import declaration, got %+v", result)
			}
		})
	}
}

func TestParseFunctionDecl(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple", "fun foo() { }"},
		{"fn_type_return", "fun higher() -> fn(u8) -> u8 { }"},  // fn(params) is a function type
		{"fn_type_param", "fun apply(f: fn(u8) -> u8, x: u8) -> u8 { }"},  // fn type as parameter
		{"with_params", "fun add(a: u8, b: u8) { }"},
		{"with_return", "fun get() -> u8 { }"},
		{"full", "fun calculate(x: u16, y: u16) -> u32 { }"},
		{"public", "pub fun api() { }"},
		{"generic", "fun swap<T>(a: T, b: T) { }"},
		{"constrained", "fun process<T: Drawable>(item: T) { }"},
		{"multi_generic", "fun pair<K, V>(key: K, val: V) { }"},
		{"with_body", "fun double(x: u8) -> u8 { return x * 2; }"},
		{"with_attribute", "@[inline] fun fast() { }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := declParser.ParseString("test", tt.input)
			if err != nil {
				t.Errorf("failed to parse %q: %v", tt.input, err)
				return
			}
			if result.Function == nil {
				t.Errorf("expected Function declaration, got %+v", result)
			}
		})
	}
}

func TestParseStructDecl(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", "struct Empty { }"},
		{"one_field", "struct Point { x: u8 }"},
		{"two_fields", "struct Point { x: u8, y: u8 }"},
		{"trailing_comma", "struct Point { x: u8, y: u8, }"},
		{"public", "pub struct API { }"},
		{"generic", "struct Box<T> { value: T }"},
		{"multi_generic", "struct Pair<K, V> { key: K, value: V }"},
		{"pub_field", "struct Data { pub id: u16 }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := declParser.ParseString("test", tt.input)
			if err != nil {
				t.Errorf("failed to parse %q: %v", tt.input, err)
				return
			}
			if result.Struct == nil {
				t.Errorf("expected Struct declaration, got %+v", result)
			}
		})
	}
}

func TestParseEnumDecl(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", "enum Empty { }"},
		{"simple", "enum Color { RED, GREEN, BLUE }"},
		{"trailing_comma", "enum State { ON, OFF, }"},
		{"with_values", "enum Status { OK = 0, ERR = 1 }"},
		{"with_types", "enum Option { Some(u8), None }"},
		{"public", "pub enum Direction { UP, DOWN }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := declParser.ParseString("test", tt.input)
			if err != nil {
				t.Errorf("failed to parse %q: %v", tt.input, err)
				return
			}
			if result.Enum == nil {
				t.Errorf("expected Enum declaration, got %+v", result)
			}
		})
	}
}

func TestParseImplDecl(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", "impl Point { }"},
		{"with_method", "impl Point { fun new() -> Point { } }"},
		{"trait_impl", "impl Drawable for Circle { }"},
		{"multiple_methods", "impl Vec { fun len() -> u16 { } fun push(x: u8) { } }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := declParser.ParseString("test", tt.input)
			if err != nil {
				t.Errorf("failed to parse %q: %v", tt.input, err)
				return
			}
			if result.Impl == nil {
				t.Errorf("expected Impl declaration, got %+v", result)
			}
		})
	}
}

func TestParseGlobalDecl(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple", "global counter: u8;"},
		{"with_init", "global counter: u8 = 0;"},
		{"public", "pub global state: u16;"},
		{"no_semi", "global x: u8 = 5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := declParser.ParseString("test", tt.input)
			if err != nil {
				t.Errorf("failed to parse %q: %v", tt.input, err)
				return
			}
			if result.Global == nil {
				t.Errorf("expected Global declaration, got %+v", result)
			}
		})
	}
}

func TestParseConstDecl(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple", "const MAX = 100;"},
		{"typed", "const MAX: u16 = 1000;"},
		{"public", "pub const API_VERSION = 1;"},
		{"no_semi", "const X = 5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := declParser.ParseString("test", tt.input)
			if err != nil {
				t.Errorf("failed to parse %q: %v", tt.input, err)
				return
			}
			if result.Const == nil {
				t.Errorf("expected Const declaration, got %+v", result)
			}
		})
	}
}

func TestParseTypeDecl(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple", "type Byte = u8;"},
		{"public", "pub type Word = u16;"},
		{"pointer", "type Ptr = *u8;"},
		{"no_semi", "type X = u8"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := declParser.ParseString("test", tt.input)
			if err != nil {
				t.Errorf("failed to parse %q: %v", tt.input, err)
				return
			}
			if result.Type == nil {
				t.Errorf("expected Type declaration, got %+v", result)
			}
		})
	}
}

func TestParseFile(t *testing.T) {
	input := `
import std.io;

const VERSION = 1;

struct Point {
    x: u8,
    y: u8,
}

impl Point {
    fun new(x: u8, y: u8) -> Point { }
}

fun main() {
    let p = Point::new(10, 20);
}
`
	result, err := fileParser.ParseString("test.minz", input)
	if err != nil {
		t.Fatalf("failed to parse file: %v", err)
	}

	if len(result.Declarations) != 5 {
		t.Errorf("expected 5 declarations, got %d", len(result.Declarations))
	}

	// Check declaration types
	if result.Declarations[0].Import == nil {
		t.Error("first declaration should be import")
	}
	if result.Declarations[1].Const == nil {
		t.Error("second declaration should be const")
	}
	if result.Declarations[2].Struct == nil {
		t.Error("third declaration should be struct")
	}
	if result.Declarations[3].Impl == nil {
		t.Error("fourth declaration should be impl")
	}
	if result.Declarations[4].Function == nil {
		t.Error("fifth declaration should be function")
	}
}
