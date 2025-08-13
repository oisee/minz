package parser

import (
	"os"
	"testing"
)

func TestParserFactory(t *testing.T) {
	// Save original environment
	originalAntlr := os.Getenv("MINZ_USE_ANTLR_PARSER")
	defer func() {
		if originalAntlr == "" {
			os.Unsetenv("MINZ_USE_ANTLR_PARSER")
		} else {
			os.Setenv("MINZ_USE_ANTLR_PARSER", originalAntlr)
		}
	}()
	
	// Test default (native) parser
	os.Unsetenv("MINZ_USE_ANTLR_PARSER")
	parser := NewParser()
	if _, ok := parser.(*NativeParser); !ok {
		t.Errorf("Expected NativeParser by default, got %T", parser)
	}
	
	// Test ANTLR parser selection
	os.Setenv("MINZ_USE_ANTLR_PARSER", "1")
	parser = NewParser()
	if _, ok := parser.(*AntlrParser); !ok {
		t.Errorf("Expected AntlrParser when MINZ_USE_ANTLR_PARSER=1, got %T", parser)
	}
	
	// Test explicit backend selection
	nativeParser := NewParserWithBackend("native")
	if _, ok := nativeParser.(*NativeParser); !ok {
		t.Errorf("Expected NativeParser for 'native' backend, got %T", nativeParser)
	}
	
	antlrParser := NewParserWithBackend("antlr")
	if _, ok := antlrParser.(*AntlrParser); !ok {
		t.Errorf("Expected AntlrParser for 'antlr' backend, got %T", antlrParser)
	}
	
	// Test unknown backend defaults to native
	defaultParser := NewParserWithBackend("unknown")
	if _, ok := defaultParser.(*NativeParser); !ok {
		t.Errorf("Expected NativeParser for unknown backend, got %T", defaultParser)
	}
}

func TestGetParserBackend(t *testing.T) {
	// Save original environment
	originalAntlr := os.Getenv("MINZ_USE_ANTLR_PARSER")
	defer func() {
		if originalAntlr == "" {
			os.Unsetenv("MINZ_USE_ANTLR_PARSER")
		} else {
			os.Setenv("MINZ_USE_ANTLR_PARSER", originalAntlr)
		}
	}()
	
	// Test default backend
	os.Unsetenv("MINZ_USE_ANTLR_PARSER")
	if backend := GetParserBackend(); backend != "native" {
		t.Errorf("Expected 'native' backend by default, got %s", backend)
	}
	
	// Test ANTLR backend
	os.Setenv("MINZ_USE_ANTLR_PARSER", "1")
	if backend := GetParserBackend(); backend != "antlr" {
		t.Errorf("Expected 'antlr' backend when MINZ_USE_ANTLR_PARSER=1, got %s", backend)
	}
}

func TestSwitchToAntlr(t *testing.T) {
	// Save original environment
	originalAntlr := os.Getenv("MINZ_USE_ANTLR_PARSER")
	defer func() {
		if originalAntlr == "" {
			os.Unsetenv("MINZ_USE_ANTLR_PARSER")
		} else {
			os.Setenv("MINZ_USE_ANTLR_PARSER", originalAntlr)
		}
	}()
	
	// Start with native parser
	os.Unsetenv("MINZ_USE_ANTLR_PARSER")
	if backend := GetParserBackend(); backend != "native" {
		t.Errorf("Expected 'native' backend initially, got %s", backend)
	}
	
	// Switch to ANTLR temporarily
	restore := SwitchToAntlr()
	if backend := GetParserBackend(); backend != "antlr" {
		t.Errorf("Expected 'antlr' backend after switch, got %s", backend)
	}
	
	// Restore original
	restore()
	if backend := GetParserBackend(); backend != "native" {
		t.Errorf("Expected 'native' backend after restore, got %s", backend)
	}
}

func TestConvenienceFunctions(t *testing.T) {
	source := `fun test() -> u8 { return 42; }`
	
	// Test ParseString convenience function
	decls, err := ParseString(source, "test_convenience.minz")
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}
	
	if len(decls) != 1 {
		t.Errorf("Expected 1 declaration, got %d", len(decls))
	}
}

func TestParserCompatibility(t *testing.T) {
	// Test that both parsers produce similar results for basic cases
	source := `fun add(a: u8, b: u8) -> u8 { return a + b; }`
	
	// Parse with native parser
	nativeParser := NewParserWithBackend("native")
	nativeDecls, err := nativeParser.ParseString(source, "test_native.minz")
	if err != nil {
		t.Fatalf("Native parser failed: %v", err)
	}
	
	// Parse with ANTLR parser (if available)
	antlrParser := NewParserWithBackend("antlr")
	antlrDecls, err := antlrParser.ParseString(source, "test_antlr.minz")
	if err != nil {
		t.Fatalf("ANTLR parser failed: %v", err)
	}
	
	// Basic compatibility checks
	if len(nativeDecls) != len(antlrDecls) {
		t.Errorf("Declaration count mismatch: native=%d, antlr=%d", 
			len(nativeDecls), len(antlrDecls))
	}
	
	if len(nativeDecls) > 0 && len(antlrDecls) > 0 {
		// Both should parse as function declarations
		nativeFn, nativeOk := nativeDecls[0].(*ast.FunctionDecl)
		antlrFn, antlrOk := antlrDecls[0].(*ast.FunctionDecl)
		
		if !nativeOk || !antlrOk {
			t.Errorf("Function declaration type mismatch: native=%T, antlr=%T", 
				nativeDecls[0], antlrDecls[0])
		}
		
		if nativeOk && antlrOk {
			if nativeFn.Name != antlrFn.Name {
				t.Errorf("Function name mismatch: native=%s, antlr=%s", 
					nativeFn.Name, antlrFn.Name)
			}
			
			if len(nativeFn.Params) != len(antlrFn.Params) {
				t.Errorf("Parameter count mismatch: native=%d, antlr=%d", 
					len(nativeFn.Params), len(antlrFn.Params))
			}
		}
	}
}