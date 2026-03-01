package z80asm

import (
	"bytes"
	"testing"
)

func TestAssembler(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		expected []byte
		wantErr  bool
	}{
		{
			name: "simple NOP",
			source: `
				ORG $8000
				NOP
			`,
			expected: []byte{0x00},
		},
		{
			name: "LD instructions",
			source: `
				ORG $8000
				LD A, B
				LD B, C
				LD A, 42
				LD HL, $1234
			`,
			expected: []byte{
				0x78,                   // LD A, B
				0x41,                   // LD B, C
				0x3E, 0x2A,             // LD A, 42
				0x21, 0x34, 0x12,       // LD HL, $1234
			},
		},
		{
			name: "arithmetic",
			source: `
				ORG $8000
				ADD A, B
				SUB C
				INC A
				DEC HL
			`,
			expected: []byte{
				0x80,       // ADD A, B
				0x91,       // SUB C
				0x3C,       // INC A
				0x2B,       // DEC HL
			},
		},
		{
			name: "jumps",
			source: `
				ORG $8000
				JP $1234
				JR $8006
				RET
			`,
			expected: []byte{
				0xC3, 0x34, 0x12,   // JP $1234
				0x18, 0x01,         // JR +1 (skip RET, to $8006)
				0xC9,               // RET
			},
		},
		{
			name: "undocumented SLL",
			source: `
				ORG $8000
				SLL B
				SLL (HL)
			`,
			expected: []byte{
				0xCB, 0x30,         // SLL B
				0xCB, 0x36,         // SLL (HL)
			},
		},
		{
			name: "IX/IY operations",
			source: `
				ORG $8000
				LD IX, $1234
				LD (IX+5), A
				INC (IX+0)
			`,
			expected: []byte{
				0xDD, 0x21, 0x34, 0x12,   // LD IX, $1234
				0xDD, 0x77, 0x05,         // LD (IX+5), A
				0xDD, 0x34, 0x00,         // INC (IX+0)
			},
		},
		{
			name: "undocumented IX half registers",
			source: `
				ORG $8000
				LD IXH, 10
				LD IXL, 20
				INC IXH
				DEC IXL
			`,
			expected: []byte{
				0xDD, 0x26, 0x0A,   // LD IXH, 10
				0xDD, 0x2E, 0x14,   // LD IXL, 20
				0xDD, 0x24,         // INC IXH
				0xDD, 0x2D,         // DEC IXL
			},
		},
		{
			name: "bit operations",
			source: `
				ORG $8000
				BIT 7, A
				SET 0, B
				RES 3, (HL)
			`,
			expected: []byte{
				0xCB, 0x7F,         // BIT 7, A
				0xCB, 0xC0,         // SET 0, B
				0xCB, 0x9E,         // RES 3, (HL)
			},
		},
		{
			name: "ED prefix instructions",
			source: `
				ORG $8000
				NEG
				LDIR
				IN A, (C)
				OUT (C), B
			`,
			expected: []byte{
				0xED, 0x44,         // NEG
				0xED, 0xB0,         // LDIR
				0xED, 0x78,         // IN A, (C)
				0xED, 0x41,         // OUT (C), B
			},
		},
		{
			name: "data directives",
			source: `
				ORG $8000
				DB 1, 2, 3
				DW $1234, $5678
				DS 4, $FF
			`,
			expected: []byte{
				0x01, 0x02, 0x03,           // DB 1, 2, 3
				0x34, 0x12, 0x78, 0x56,     // DW $1234, $5678
				0xFF, 0xFF, 0xFF, 0xFF,     // DS 4, $FF
			},
		},
		{
			name: "labels and jumps",
			source: `
				ORG $8000
			start:
				LD A, 0
				JP start
			loop:
				INC A
				JR loop
			`,
			expected: []byte{
				0x3E, 0x00,             // LD A, 0
				0xC3, 0x00, 0x80,       // JP $8000 (start)
				0x3C,                   // INC A
				0x18, 0xFD,             // JR -3 (loop)
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asm := NewAssembler()
			result, err := asm.AssembleString(tt.source)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("AssembleString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !tt.wantErr {
				if !bytes.Equal(result.Binary, tt.expected) {
					t.Errorf("Binary mismatch:\ngot:  %X\nwant: %X", result.Binary, tt.expected)
				}
			}
		})
	}
}

func TestUndocumentedInstructions(t *testing.T) {
	asm := NewAssembler()
	asm.AllowUndocumented = true
	
	tests := []struct {
		instruction string
		expected    []byte
	}{
		// SLL (Shift Left Logical)
		{"SLL A", []byte{0xCB, 0x37}},
		{"SLL B", []byte{0xCB, 0x30}},
		{"SLL (IX+5)", []byte{0xDD, 0xCB, 0x05, 0x36}},
		
		// IX/IY half registers
		{"LD IXH, 10", []byte{0xDD, 0x26, 0x0A}},
		{"LD IYL, B", []byte{0xFD, 0x68}},
		{"ADD A, IXH", []byte{0xDD, 0x84}},
		{"SUB IYL", []byte{0xFD, 0x95}},
		
		// Undocumented OUT (C), 0
		{"OUT (C), 0", []byte{0xED, 0x71}},
	}
	
	for _, tt := range tests {
		t.Run(tt.instruction, func(t *testing.T) {
			source := "ORG $8000\n" + tt.instruction
			result, err := asm.AssembleString(source)
			
			if err != nil {
				t.Fatalf("Failed to assemble %s: %v", tt.instruction, err)
			}
			
			if !bytes.Equal(result.Binary, tt.expected) {
				t.Errorf("%s: got %X, want %X", tt.instruction, result.Binary, tt.expected)
			}
		})
	}
}

func TestErrorHandling(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name:   "undefined symbol",
			source: "JP undefined_label",
		},
		{
			name:   "invalid register",
			source: "LD Q, A",
		},
		{
			name:   "out of range immediate",
			source: "LD A, 256",
		},
		{
			name:   "invalid bit number",
			source: "BIT 8, A",
		},
		{
			name:   "relative jump out of range",
			source: "ORG $8000\nJR $8100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asm := NewAssembler()
			asm.Strict = true // Enable strict mode to get errors returned
			result, err := asm.AssembleString(tt.source)
			if err == nil {
				// Also check collected errors (non-strict path)
				if result != nil && len(result.Errors) > 0 {
					return // Error was collected, test passes
				}
				t.Errorf("Expected error for %s, but got none", tt.name)
			}
		})
	}
}

func TestBracketSyntax(t *testing.T) {
	// Bracket indirection [x] should produce identical bytes to parenthesized (x)
	tests := []struct {
		name       string
		bracketSrc string
		parenSrc   string
	}{
		{
			name:       "indirect register LD A,[HL]",
			bracketSrc: "ORG $8000\nLD A, [HL]",
			parenSrc:   "ORG $8000\nLD A, (HL)",
		},
		{
			name:       "indirect immediate LD HL,[$8000]",
			bracketSrc: "ORG $8000\nLD HL, [$8000]",
			parenSrc:   "ORG $8000\nLD HL, ($8000)",
		},
		{
			name:       "indexed IX LD A,[IX+5]",
			bracketSrc: "ORG $8000\nLD A, [IX+5]",
			parenSrc:   "ORG $8000\nLD A, (IX+5)",
		},
		{
			name:       "indexed IY LD [IY+3],B",
			bracketSrc: "ORG $8000\nLD [IY+3], B",
			parenSrc:   "ORG $8000\nLD (IY+3), B",
		},
		{
			name:       "indirect BC LD A,[BC]",
			bracketSrc: "ORG $8000\nLD A, [BC]",
			parenSrc:   "ORG $8000\nLD A, (BC)",
		},
		{
			name:       "indirect DE LD A,[DE]",
			bracketSrc: "ORG $8000\nLD A, [DE]",
			parenSrc:   "ORG $8000\nLD A, (DE)",
		},
		{
			name:       "store to indirect LD [HL],A",
			bracketSrc: "ORG $8000\nLD [HL], A",
			parenSrc:   "ORG $8000\nLD (HL), A",
		},
		{
			name:       "store to memory LD [$9000],A",
			bracketSrc: "ORG $8000\nLD [$9000], A",
			parenSrc:   "ORG $8000\nLD ($9000), A",
		},
		{
			name:       "store to memory 16bit LD [$9000],HL",
			bracketSrc: "ORG $8000\nLD [$9000], HL",
			parenSrc:   "ORG $8000\nLD ($9000), HL",
		},
		{
			name:       "INC [HL]",
			bracketSrc: "ORG $8000\nINC [HL]",
			parenSrc:   "ORG $8000\nINC (HL)",
		},
		{
			name:       "BIT 7,[HL]",
			bracketSrc: "ORG $8000\nBIT 7, [HL]",
			parenSrc:   "ORG $8000\nBIT 7, (HL)",
		},
		{
			name:       "IN A,[C]",
			bracketSrc: "ORG $8000\nIN A, [C]",
			parenSrc:   "ORG $8000\nIN A, (C)",
		},
		{
			name:       "OUT [C],B",
			bracketSrc: "ORG $8000\nOUT [C], B",
			parenSrc:   "ORG $8000\nOUT (C), B",
		},
		{
			name:       "JP [HL]",
			bracketSrc: "ORG $8000\nJP [HL]",
			parenSrc:   "ORG $8000\nJP (HL)",
		},
		{
			name:       "EX [SP],HL",
			bracketSrc: "ORG $8000\nEX [SP], HL",
			parenSrc:   "ORG $8000\nEX (SP), HL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asmBracket := NewAssembler()
			asmParen := NewAssembler()

			bracketResult, err := asmBracket.AssembleString(tt.bracketSrc)
			if err != nil {
				t.Fatalf("Bracket syntax failed to assemble: %v", err)
			}
			if len(bracketResult.Errors) > 0 {
				t.Fatalf("Bracket syntax had errors: %v", bracketResult.Errors)
			}

			parenResult, err := asmParen.AssembleString(tt.parenSrc)
			if err != nil {
				t.Fatalf("Paren syntax failed to assemble: %v", err)
			}

			if !bytes.Equal(bracketResult.Binary, parenResult.Binary) {
				t.Errorf("Bracket vs paren mismatch:\n  bracket: %X\n  paren:   %X",
					bracketResult.Binary, parenResult.Binary)
			}
		})
	}
}

func TestBracketMismatchRejected(t *testing.T) {
	// Mismatched brackets like [) or (] must NOT be treated as valid indirection
	tests := []struct {
		name   string
		source string
	}{
		{
			name:   "open bracket close paren [HL)",
			source: "ORG $8000\nLD A, [HL)",
		},
		{
			name:   "open paren close bracket (HL]",
			source: "ORG $8000\nLD A, (HL]",
		},
		{
			name:   "mismatched memory indirect [$8000)",
			source: "ORG $8000\nLD HL, [$8000)",
		},
		{
			name:   "mismatched memory indirect ($8000]",
			source: "ORG $8000\nLD HL, ($8000]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asm := NewAssembler()
			asm.Strict = true
			_, err := asm.AssembleString(tt.source)
			if err == nil {
				t.Errorf("Expected error for mismatched brackets in %q, but got none", tt.name)
			}
		})
	}
}

func TestNormalizeBrackets(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"[HL]", "(HL)"},
		{"[IX+5]", "(IX+5)"},
		{"[$8000]", "($8000)"},
		{"(HL)", "(HL)"},      // unchanged
		{"A", "A"},            // unchanged
		{"[HL)", "[HL)"},      // mismatched - unchanged
		{"(HL]", "(HL]"},      // mismatched - unchanged
		{"[]", "()"},          // edge case
		{"[C]", "(C)"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeBrackets(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeBrackets(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSymbols(t *testing.T) {
	source := `
		ORG $8000
		
	START:
		LD A, VALUE
		JP LOOP
		
	VALUE:  EQU 42
		
	LOOP:
		INC A
		JP START
	`
	
	asm := NewAssembler()
	result, err := asm.AssembleString(source)
	
	if err != nil {
		t.Fatalf("Assembly failed: %v", err)
	}
	
	// Check symbols
	expectedSymbols := map[string]int{
		"START": 0x8000,
		"VALUE": 42,
		"LOOP":  0x8005,
	}
	
	for name, expectedAddr := range expectedSymbols {
		if addr, ok := result.Symbols[name]; !ok {
			t.Errorf("Symbol %s not found", name)
		} else if addr != expectedAddr {
			t.Errorf("Symbol %s: got $%04X, want $%04X", name, addr, expectedAddr)
		}
	}
}