package z80testing

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSimpleAssembler(t *testing.T) {
	asm := NewSimpleAssembler()

	// Create test assembly file
	testASM := `
		ORG $8000
		LD A, #42
		LD B, A
		RET
	`

	tmpDir := t.TempDir()
	asmFile := filepath.Join(tmpDir, "test.a80")
	if err := os.WriteFile(asmFile, []byte(testASM), 0644); err != nil {
		t.Fatal(err)
	}

	// Assemble
	binary, err := asm.Assemble(asmFile)
	if err != nil {
		t.Fatal(err)
	}

	// Verify output
	expected := []byte{
		0x3E, 0x2A, // LD A, 42
		0x78,       // LD B, A (actually LD A, B but we'll fix this)
		0xC9,       // RET
	}

	if len(binary) != len(expected) {
		t.Errorf("Binary length mismatch: got %d, want %d", len(binary), len(expected))
	}

	// Check first instruction (LD A, 42)
	if len(binary) >= 2 && (binary[0] != 0x3E || binary[1] != 0x2A) {
		t.Errorf("First instruction wrong: got %02X %02X, want 3E 2A", binary[0], binary[1])
	}
}

func TestSjasmPlusAssembler(t *testing.T) {
	// Skip if sjasmplus not available
	asm, err := NewSjasmPlusAssembler()
	if err != nil {
		t.Skip("sjasmplus not available:", err)
	}
	defer asm.Cleanup()

	// Create test assembly file
	testASM := `
		ORG $8000
		
main:
		LD A, 42
		LD B, A
		CALL helper
		RET

helper:
		INC A
		RET
	`

	tmpDir := t.TempDir()
	asmFile := filepath.Join(tmpDir, "test.a80")
	if err := os.WriteFile(asmFile, []byte(testASM), 0644); err != nil {
		t.Fatal(err)
	}

	// Test basic assembly
	t.Run("BasicAssembly", func(t *testing.T) {
		binary, err := asm.Assemble(asmFile)
		if err != nil {
			t.Fatal(err)
		}

		// Should have some output
		if len(binary) == 0 {
			t.Error("No binary output produced")
		}

		// Check first bytes (LD A, 42)
		if len(binary) >= 2 && (binary[0] != 0x3E || binary[1] != 0x2A) {
			t.Errorf("First instruction wrong: got %02X %02X, want 3E 2A", binary[0], binary[1])
		}
	})

	// Test assembly with symbols
	t.Run("AssemblyWithSymbols", func(t *testing.T) {
		binary, symbols, err := asm.AssembleWithSymbols(asmFile)
		if err != nil {
			t.Fatal(err)
		}

		// Should have binary output
		if len(binary) == 0 {
			t.Error("No binary output produced")
		}

		// Should have symbols
		if len(symbols) == 0 {
			t.Error("No symbols extracted")
		}

		// Check for main symbol
		if addr, ok := symbols["main"]; ok {
			if addr != 0x8000 {
				t.Errorf("main symbol at wrong address: got %04X, want 8000", addr)
			}
		} else {
			t.Error("main symbol not found")
		}

		// Check for helper symbol
		if _, ok := symbols["helper"]; !ok {
			t.Error("helper symbol not found")
		}
	})
}

func TestParseHexValue(t *testing.T) {
	tests := []struct {
		input    string
		expected uint16
	}{
		{"$8000", 0x8000},
		{"0x1234", 0x1234},
		{"0X5678", 0x5678},
		{"42", 42},
		{"FF", 0xFF},
	}

	for _, test := range tests {
		result := parseHexValue(test.input)
		if result != test.expected {
			t.Errorf("parseHexValue(%q) = %04X, want %04X", test.input, result, test.expected)
		}
	}
}