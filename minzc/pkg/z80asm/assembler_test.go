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
				JR $8004
				RET
			`,
			expected: []byte{
				0xC3, 0x34, 0x12,   // JP $1234
				0x18, 0x02,         // JR +2 (to $8004)
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
	asm := NewAssembler()
	
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
			_, err := asm.AssembleString(tt.source)
			if err == nil {
				t.Errorf("Expected error for %s, but got none", tt.name)
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
	expectedSymbols := map[string]uint16{
		"START": 0x8000,
		"VALUE": 42,
		"LOOP":  0x8006,
	}
	
	for name, expectedAddr := range expectedSymbols {
		if addr, ok := result.Symbols[name]; !ok {
			t.Errorf("Symbol %s not found", name)
		} else if addr != expectedAddr {
			t.Errorf("Symbol %s: got $%04X, want $%04X", name, addr, expectedAddr)
		}
	}
}