package z80asm

import (
	"strings"
)

// fakeInstructions defines pseudo-instructions that expand to real Z80 code
// These make assembly more readable and match SjASMPlus behavior
var fakeInstructions = map[string]bool{
	"LD": true, // When used with 16-bit register pairs
}

// expandFakeInstructions expands fake/pseudo instructions into real Z80 instructions
// This handles things like LD HL, DE which isn't a real Z80 instruction
func expandFakeInstructions(lines []*Line) []*Line {
	var result []*Line
	
	for _, line := range lines {
		// Skip non-instruction lines
		if line.Mnemonic == "" || line.IsBlank {
			result = append(result, line)
			continue
		}
		
		mnemonic := strings.ToUpper(line.Mnemonic)
		
		// Check for fake LD instructions
		if mnemonic == "LD" && len(line.Operands) == 2 {
			expanded := tryExpandFakeLD(line)
			if expanded != nil {
				result = append(result, expanded...)
				continue
			}
		}
		
		// Not a fake instruction, keep as-is
		result = append(result, line)
	}
	
	return result
}

// tryExpandFakeLD attempts to expand fake LD instructions
// Returns nil if this is a real LD instruction
func tryExpandFakeLD(line *Line) []*Line {
	if len(line.Operands) != 2 {
		return nil
	}
	
	dst := strings.ToUpper(strings.TrimSpace(line.Operands[0]))
	src := strings.ToUpper(strings.TrimSpace(line.Operands[1]))
	
	// Check if this is a 16-bit to 16-bit transfer (fake instruction)
	if !is16BitReg(dst) || !is16BitReg(src) {
		return nil // This is a normal LD instruction
	}
	
	// Special cases that ARE real instructions
	if dst == "SP" && src == "HL" {
		return nil // LD SP, HL is a real instruction (F9)
	}
	if dst == "SP" && src == "IX" {
		return nil // LD SP, IX is real (DD F9)
	}
	if dst == "SP" && src == "IY" {
		return nil // LD SP, IY is real (FD F9)
	}
	
	// Skip the immediate check - we already know these are register names
	// The issue was parseOperandValue treats "DE" as a symbol and returns nil error
	// But we KNOW if both dst and src are 16-bit registers, this is a fake instruction
	
	// Now handle the fake register-to-register transfers
	var result []*Line
	
	switch {
	case dst == "HL" && src == "DE":
		// LD HL, DE -> LD H, D : LD L, E
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "LD",
			Operands: []string{"H", "D"},
			Comment:  line.Comment + " (expanded from LD HL, DE)",
		})
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "LD",
			Operands: []string{"L", "E"},
		})
		
	case dst == "HL" && src == "BC":
		// LD HL, BC -> LD H, B : LD L, C
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "LD",
			Operands: []string{"H", "B"},
			Comment:  line.Comment + " (expanded from LD HL, BC)",
		})
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "LD",
			Operands: []string{"L", "C"},
		})
		
	case dst == "DE" && src == "HL":
		// LD DE, HL -> LD D, H : LD E, L
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "LD",
			Operands: []string{"D", "H"},
			Comment:  line.Comment + " (expanded from LD DE, HL)",
		})
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "LD",
			Operands: []string{"E", "L"},
		})
		
	case dst == "DE" && src == "BC":
		// LD DE, BC -> LD D, B : LD E, C
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "LD",
			Operands: []string{"D", "B"},
			Comment:  line.Comment + " (expanded from LD DE, BC)",
		})
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "LD",
			Operands: []string{"E", "C"},
		})
		
	case dst == "BC" && src == "HL":
		// LD BC, HL -> LD B, H : LD C, L
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "LD",
			Operands: []string{"B", "H"},
			Comment:  line.Comment + " (expanded from LD BC, HL)",
		})
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "LD",
			Operands: []string{"C", "L"},
		})
		
	case dst == "BC" && src == "DE":
		// LD BC, DE -> LD B, D : LD C, E
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "LD",
			Operands: []string{"B", "D"},
			Comment:  line.Comment + " (expanded from LD BC, DE)",
		})
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "LD",
			Operands: []string{"C", "E"},
		})
		
	case dst == "IX" && (src == "BC" || src == "DE"):
		// LD IX, BC -> LD IXH, B : LD IXL, C
		highReg := "IXH"
		lowReg := "IXL"
		srcHigh := string(src[0]) // B or D
		srcLow := string(src[1])  // C or E
		
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "LD",
			Operands: []string{highReg, srcHigh},
			Comment:  line.Comment + " (expanded from LD IX, " + src + ")",
		})
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "LD",
			Operands: []string{lowReg, srcLow},
		})
		
	case dst == "IY" && (src == "BC" || src == "DE"):
		// LD IY, BC -> LD IYH, B : LD IYL, C
		highReg := "IYH"
		lowReg := "IYL"
		srcHigh := string(src[0]) // B or D
		srcLow := string(src[1])  // C or E
		
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "LD",
			Operands: []string{highReg, srcHigh},
			Comment:  line.Comment + " (expanded from LD IY, " + src + ")",
		})
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "LD",
			Operands: []string{lowReg, srcLow},
		})
		
	case dst == "IX" && src == "HL":
		// LD IX, HL -> PUSH HL : POP IX (special case)
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "PUSH",
			Operands: []string{"HL"},
			Comment:  line.Comment + " (expanded from LD IX, HL)",
		})
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "POP",
			Operands: []string{"IX"},
		})
		
	case dst == "IY" && src == "HL":
		// LD IY, HL -> PUSH HL : POP IY (special case)
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "PUSH",
			Operands: []string{"HL"},
			Comment:  line.Comment + " (expanded from LD IY, HL)",
		})
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "POP",
			Operands: []string{"IY"},
		})
		
	case dst == "HL" && src == "IX":
		// LD HL, IX -> PUSH IX : POP HL (special case)
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "PUSH",
			Operands: []string{"IX"},
			Comment:  line.Comment + " (expanded from LD HL, IX)",
		})
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "POP",
			Operands: []string{"HL"},
		})
		
	case dst == "HL" && src == "IY":
		// LD HL, IY -> PUSH IY : POP HL (special case)
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "PUSH",
			Operands: []string{"IY"},
			Comment:  line.Comment + " (expanded from LD HL, IY)",
		})
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "POP",
			Operands: []string{"HL"},
		})
		
	default:
		// Not a fake instruction we recognize
		return nil
	}
	
	return result
}

// is16BitReg checks if a string is a 16-bit register name
func is16BitReg(s string) bool {
	switch s {
	case "BC", "DE", "HL", "SP", "IX", "IY", "AF":
		return true
	default:
		return false
	}
}