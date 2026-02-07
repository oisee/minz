package participle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParserBasic(t *testing.T) {
	source := `
// Simple MinZ program
import std.io;

const MAX_SIZE = 100;

struct Point {
    x: u8,
    y: u8,
}

impl Point {
    fun new(x: u8, y: u8) -> Point {
        return Point { x: x, y: y };
    }

    fun distance(self, other: Point) -> u16 {
        let dx = self.x - other.x;
        let dy = self.y - other.y;
        return dx * dx + dy * dy;
    }
}

global counter: u8 = 0;

fun main() {
    let p1 = Point::new(10, 20);
    let p2 = Point::new(30, 40);

    if p1.x > p2.x {
        counter = 1;
    } else {
        counter = 2;
    }

    for i in 0..10 {
        counter = counter + 1;
    }

    while counter > 0 {
        counter = counter - 1;
    }
}
`

	parser := New()
	file, err := parser.ParseString("test.minz", source)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	// Expected: import, const, struct, impl, global, fun main
	if len(file.Declarations) != 6 {
		t.Errorf("expected 6 declarations, got %d", len(file.Declarations))
		for i, d := range file.Declarations {
			t.Logf("  [%d] %+v", i, d)
		}
	}
}

func TestParseExampleFiles(t *testing.T) {
	// Find example files
	exampleDir := "../../../examples"
	if _, err := os.Stat(exampleDir); os.IsNotExist(err) {
		exampleDir = "../../../../examples"
	}

	entries, err := os.ReadDir(exampleDir)
	if err != nil {
		t.Skipf("examples directory not found: %v", err)
	}

	parser := New()
	passed := 0
	failed := 0
	var failures []string

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".minz") {
			path := filepath.Join(exampleDir, entry.Name())
			t.Run(entry.Name(), func(t *testing.T) {
				_, err := parser.ParseFile(path)
				if err != nil {
					failed++
					failures = append(failures, entry.Name()+": "+err.Error())
					t.Errorf("failed to parse: %v", err)
				} else {
					passed++
				}
			})
		}
	}

	t.Logf("Parsed %d/%d files successfully (%.1f%%)",
		passed, passed+failed,
		float64(passed)/float64(passed+failed)*100)

	if len(failures) > 0 && len(failures) <= 10 {
		t.Log("Failures:")
		for _, f := range failures {
			t.Log("  " + f)
		}
	}
}

func TestParseComplexPatterns(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "lambda_iterator",
			input: `
fun main() {
    let result = items.iter()
        .map(|x| x * 2)
        .filter(|x| x > 5)
        .sum();
}`,
		},
		{
			name: "generic_function",
			input: `
fun swap<T>(a: *T, b: *T) -> void {
    let temp = *a;
    *a = *b;
    *b = temp;
}`,
		},
		{
			name: "nested_structs",
			input: `
struct Inner { value: u8 }
struct Outer { inner: Inner, count: u16 }

fun access() -> u8 {
    let o = Outer { inner: Inner { value: 42 }, count: 100 };
    return o.inner.value;
}`,
		},
		{
			name: "complex_expressions",
			input: `
fun calc(a: u8, b: u8, c: u8) -> u16 {
    return (a + b) * c >> 2 & 0xFF | (a ^ b);
}`,
		},
		{
			name: "metafunction",
			input: `
@define("SCREEN_WIDTH", 256)
@define("SCREEN_HEIGHT", 192)

fun clear_screen() {
    @print("Clearing screen");
}`,
		},
		{
			name: "enum_with_values",
			input: `
enum Status {
    OK = 0,
    ERROR = 1,
    PENDING = 2,
}

fun check(s: Status) -> bool {
    return s == Status::OK;
}`,
		},
	}

	parser := New()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.ParseString("test.minz", tt.input)
			if err != nil {
				t.Errorf("failed to parse: %v", err)
			}
		})
	}
}
