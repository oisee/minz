package parser

import (
	"os"
	"testing"
)

func TestNativeParser(t *testing.T) {
	// Skip if not explicitly enabled
	if os.Getenv("TEST_NATIVE_PARSER") != "1" {
		t.Skip("Native parser test skipped (set TEST_NATIVE_PARSER=1 to run)")
	}
	
	parser := NewNativeParser()
	
	// Test parsing a simple file
	testFile := "../../examples/fibonacci.minz"
	
	file, err := parser.ParseFile(testFile)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}
	
	if file == nil {
		t.Fatal("Expected non-nil AST")
	}
	
	if len(file.Declarations) == 0 {
		t.Error("Expected at least one declaration")
	}
	
	t.Logf("Successfully parsed %s with native parser", testFile)
	t.Logf("Found %d declarations", len(file.Declarations))
}

func BenchmarkParsing(b *testing.B) {
	if os.Getenv("BENCHMARK_PARSER") != "1" {
		b.Skip("Parser benchmark skipped (set BENCHMARK_PARSER=1 to run)")
	}
	
	testFile := "../../examples/fibonacci.minz"
	
	b.Run("CLI", func(b *testing.B) {
		p := New() // CLI parser
		for i := 0; i < b.N; i++ {
			_, _ = p.ParseFile(testFile)
		}
	})
	
	b.Run("Native", func(b *testing.B) {
		p := NewNativeParser()
		for i := 0; i < b.N; i++ {
			_, _ = p.ParseFile(testFile)
		}
	})
}