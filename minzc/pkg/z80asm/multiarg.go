package z80asm

import (
	"strings"
)

// multiArgInstructions defines which instructions support multi-arg expansion
var multiArgInstructions = map[string]bool{
	// Stack operations
	"PUSH": true,
	"POP":  true,
	
	// Shifts and rotates
	"SRL": true, "SRA": true, "SLA": true, "SLL": true,
	"RLC": true, "RRC": true, "RL": true, "RR": true,
	"RLA": true, "RRA": true, "RLCA": true, "RRCA": true,
	
	// Arithmetic (useful for quick increments/decrements)
	"INC": true, "DEC": true,
	
	// Logical operations (multiple shifts for multiplication/division)
	"ADD": true,
}

// expandMultiArgInstructions expands multi-argument instructions into multiple single-arg instructions
// For example: PUSH AF, BC, DE becomes three separate PUSH instructions
func expandMultiArgInstructions(lines []*Line) []*Line {
	var result []*Line
	
	for _, line := range lines {
		// Skip non-instruction lines
		if line.Mnemonic == "" || line.IsBlank {
			result = append(result, line)
			continue
		}
		
		mnemonic := strings.ToUpper(line.Mnemonic)
		
		// Check if this instruction supports multi-arg
		if multiArgInstructions[mnemonic] && len(line.Operands) > 1 {
			expanded := expandMultiArg(line)
			result = append(result, expanded...)
		} else {
			result = append(result, line)
		}
	}
	
	return result
}

// expandMultiArg expands a single multi-arg instruction into multiple instructions
func expandMultiArg(line *Line) []*Line {
	mnemonic := strings.ToUpper(line.Mnemonic)
	var result []*Line
	
	switch mnemonic {
	case "POP":
		// POP must be in reverse order!
		// POP HL, DE, BC becomes: POP HL; POP DE; POP BC
		for i := 0; i < len(line.Operands); i++ {
			newLine := &Line{
				Number:   line.Number,
				Mnemonic: line.Mnemonic,
				Operands: []string{line.Operands[i]},
				Comment:  "",
			}
			if i == 0 && line.Comment != "" {
				newLine.Comment = line.Comment
			}
			result = append(result, newLine)
		}
		
	case "PUSH":
		// PUSH in forward order
		// PUSH AF, BC, DE becomes: PUSH AF; PUSH BC; PUSH DE
		for i := 0; i < len(line.Operands); i++ {
			newLine := &Line{
				Number:   line.Number,
				Mnemonic: line.Mnemonic,
				Operands: []string{line.Operands[i]},
				Comment:  "",
			}
			if i == 0 && line.Comment != "" {
				newLine.Comment = line.Comment
			}
			result = append(result, newLine)
		}
		
	case "SRL", "SRA", "SLA", "SLL", "RLC", "RRC", "RL", "RR":
		// Shift/rotate instructions - repeat for each operand
		// SRL A, A, A becomes: SRL A; SRL A; SRL A
		// This allows "shift right 3 times" syntax
		for i := 0; i < len(line.Operands); i++ {
			newLine := &Line{
				Number:   line.Number,
				Mnemonic: line.Mnemonic,
				Operands: []string{line.Operands[i]},
				Comment:  "",
			}
			if i == 0 && line.Comment != "" {
				newLine.Comment = line.Comment + " (expanded)"
			}
			result = append(result, newLine)
		}
		
	case "RLA", "RRA", "RLCA", "RRCA":
		// These take no operands, but we can repeat them
		// RRA, RRA, RRA means rotate 3 times
		count := len(line.Operands)
		if count == 0 {
			count = 1 // Single instruction as normal
		}
		for i := 0; i < count; i++ {
			newLine := &Line{
				Number:   line.Number,
				Mnemonic: line.Mnemonic,
				Operands: []string{}, // These instructions take no operands
				Comment:  "",
			}
			if i == 0 && line.Comment != "" {
				newLine.Comment = line.Comment
			}
			result = append(result, newLine)
		}
		
	case "INC", "DEC":
		// INC A, A, A becomes: INC A; INC A; INC A
		// Useful for quick +3 or -3
		for i := 0; i < len(line.Operands); i++ {
			newLine := &Line{
				Number:   line.Number,
				Mnemonic: line.Mnemonic,
				Operands: []string{line.Operands[i]},
				Comment:  "",
			}
			if i == 0 && line.Comment != "" {
				newLine.Comment = line.Comment
			}
			result = append(result, newLine)
		}
		
	case "ADD":
		// Special case for ADD HL, HL, HL (multiply by 4)
		// ADD HL, BC, DE is also valid (add multiple values)
		for i := 0; i < len(line.Operands); i++ {
			// For ADD, we need the first operand as destination
			destOp := line.Operands[0]
			srcOp := line.Operands[i]
			
			// Skip first iteration if it's self-reference
			if i == 0 && len(line.Operands) > 1 {
				continue
			}
			
			newLine := &Line{
				Number:   line.Number,
				Mnemonic: line.Mnemonic,
				Operands: []string{destOp, srcOp},
				Comment:  "",
			}
			if i == 0 && line.Comment != "" {
				newLine.Comment = line.Comment
			}
			result = append(result, newLine)
		}
		
	default:
		// Should not happen, but return original line
		result = append(result, line)
	}
	
	return result
}