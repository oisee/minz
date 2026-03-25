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
		
		// JRS — "JR if possible and Short, else JP" pseudo-instruction.
		// Emitted by MIR2 codegen for all local label branches.
		// Rules:
		//   - Unconditional or condition in {NZ,Z,NC,C} → JR (2B)
		//     MZA's encodeJRRel auto-promotes JR→JP if offset > ±127.
		//   - Condition not supported by JR (PE,PO,P,M) → JP (3B, always).
		// Codegen never needs to know which conditions JR supports.
		if mnemonic == "JRS" {
			expanded := expandJRS(line)
			result = append(result, expanded)
			continue
		}

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

	// DD/FD prefix conflict: LD IXH,H / LD IXH,L / LD IXL,H / LD IXL,L
	// (and IYH/IYL variants). Under DD prefix, H→IXH and L→IXL, making
	// these either no-ops or wrong. Route through A.
	isIXY := func(r string) bool {
		return r == "IXH" || r == "IXL" || r == "IYH" || r == "IYL"
	}
	isHL := func(r string) bool { return r == "H" || r == "L" }
	if (isIXY(dst) && isHL(src)) || (isHL(dst) && isIXY(src)) {
		return []*Line{
			{Number: line.Number, Mnemonic: "LD", Operands: []string{"A", src}},
			{Number: line.Number, Mnemonic: "LD", Operands: []string{dst, "A"}},
		}
	}

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

	// Pseudo-instructions: LD rr, SP (not valid Z80; expand to LD rr,0 + ADD rr,SP)
	// LD HL, SP  → LD HL, 0 (3B/10T) + ADD HL, SP (1B/11T) = 4B/21T
	// LD IX, SP  → LD IX, 0 (4B/14T) + ADD IX, SP (2B/15T) = 6B/29T
	// LD IY, SP  → LD IY, 0 (4B/14T) + ADD IY, SP (2B/15T) = 6B/29T
	case dst == "HL" && src == "SP":
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "LD",
			Operands: []string{"HL", "0"},
			Comment:  line.Comment + " (LD HL,SP expanded: clear then add)",
		})
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "ADD",
			Operands: []string{"HL", "SP"},
		})

	case dst == "IX" && src == "SP":
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "LD",
			Operands: []string{"IX", "0"},
			Comment:  line.Comment + " (LD IX,SP expanded: clear then add)",
		})
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "ADD",
			Operands: []string{"IX", "SP"},
		})

	case dst == "IY" && src == "SP":
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "LD",
			Operands: []string{"IY", "0"},
			Comment:  line.Comment + " (LD IY,SP expanded: clear then add)",
		})
		result = append(result, &Line{
			Number:   line.Number,
			Mnemonic: "ADD",
			Operands: []string{"IY", "SP"},
		})

	default:
		// Not a fake instruction we recognize
		return nil
	}
	
	return result
}

// expandJRS expands a JRS (JR-if-Short) pseudo-instruction into JR or JP.
// Conditions supported by JR (NZ/Z/NC/C + unconditional) → JR.
// Conditions only supported by JP (PE/PO/P/M) → JP.
// MZA's encodeJRRel auto-promotes JR→JP when offset > ±127.
func expandJRS(line *Line) *Line {
	newMnemonic := "JR"
	if len(line.Operands) >= 1 {
		cond := strings.ToUpper(strings.TrimSpace(line.Operands[0]))
		switch cond {
		case "NZ", "Z", "NC", "C":
			// JR supports these — keep as JR (encodeJRRel auto-promotes to JP)
		case "PE", "PO", "P", "M":
			newMnemonic = "JP"
		}
	}
	return &Line{
		Number:   line.Number,
		Label:    line.Label,
		Mnemonic: newMnemonic,
		Operands: line.Operands,
		Comment:  line.Comment,
		IsBlank:  line.IsBlank,
		FromJRS:  true, // signal to encodeJRRel to reserve 3 bytes in pass 1
	}
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