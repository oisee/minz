package z80asm

import (
	"fmt"
	"strconv"
	"strings"
)

// processInstruction handles instruction encoding using the table-driven approach
func (a *Assembler) processInstruction(line *Line) error {
	// Try table-driven encoding first
	encoded, err := a.encodeInstructionTable(line)
	if err == nil {
		// Success! Add to output
		if a.pass == 2 {
			inst := &AssembledInstruction{
				Address: a.currentAddr,
				Line:    line,
				Bytes:   encoded,
			}
			a.instructions = append(a.instructions, inst)
			a.output = append(a.output, encoded...)
		}
		a.currentAddr += uint16(len(encoded))
		return nil
	}
	
	// Fall back to old instruction processing for now
	// This will be removed once table is complete
	return a.processInstructionOld(line)
}

// encodeInstructionTable uses the table-driven approach to encode instructions
func (a *Assembler) encodeInstructionTable(line *Line) ([]byte, error) {
	// Get all patterns for this mnemonic
	patterns := GetInstructionPatterns(line.Mnemonic)
	if len(patterns) == 0 {
		return nil, fmt.Errorf("unknown instruction: %s", line.Mnemonic)
	}
	
	// Try to match each pattern
	for _, pattern := range patterns {
		if match, values := a.matchPattern(pattern, line); match {
			// Generate encoding
			if pattern.EncodingFunc != nil {
				return pattern.EncodingFunc(a, &pattern, values)
			}
			// Simple encoding (no operands or fixed encoding)
			return pattern.Encoding, nil
		}
	}
	
	// No pattern matched
	return nil, fmt.Errorf("no matching pattern for %s with operands %v", 
		line.Mnemonic, line.Operands)
}

// matchPattern checks if a line matches an instruction pattern
func (a *Assembler) matchPattern(pattern InstructionPattern, line *Line) (bool, []interface{}) {
	// Check operand count
	if len(pattern.Operands) != len(line.Operands) {
		return false, nil
	}
	
	// No operands - simple match
	if len(pattern.Operands) == 0 {
		return true, nil
	}
	
	// Match each operand
	values := make([]interface{}, len(line.Operands))
	for i, opPattern := range pattern.Operands {
		operand := line.Operands[i]
		
		// Parse operand based on expected type
		value, ok := a.parseOperandAs(operand, opPattern)
		if !ok {
			return false, nil
		}
		values[i] = value
	}
	
	return true, values
}

// parseOperandAs tries to parse an operand as a specific type
func (a *Assembler) parseOperandAs(operand string, pattern OperandPattern) (interface{}, bool) {
	operand = strings.TrimSpace(operand)
	
	switch pattern.Type {
	case OpTypeReg8:
		reg := parseReg8(operand)
		if reg == "" {
			return nil, false
		}
		// Check constraint if specified
		if pattern.Constraint != "" && reg != pattern.Constraint {
			return nil, false
		}
		return reg, true
		
	case OpTypeReg16:
		reg := parseReg16(operand)
		if reg == "" {
			return nil, false
		}
		// Check constraint if specified
		if pattern.Constraint != "" && reg != pattern.Constraint {
			return nil, false
		}
		return reg, true
		
	case OpTypeImm8, OpTypeImm16:
		// Try to resolve value (number or symbol)
		value, err := a.resolveValue(operand)
		if err != nil {
			return nil, false
		}
		
		// Check size constraint for 8-bit
		if pattern.Type == OpTypeImm8 && value > 0xFF {
			return nil, false
		}
		
		return value, true
		
	case OpTypeIndReg:
		// Check for indirect register addressing
		if !strings.HasPrefix(operand, "(") || !strings.HasSuffix(operand, ")") {
			return nil, false
		}
		
		inner := operand[1:len(operand)-1]
		
		// Check constraint if specified
		if pattern.Constraint != "" {
			if operand != pattern.Constraint {
				return nil, false
			}
			return operand, true
		}
		
		// Check if it's a valid indirect register
		if inner == "HL" || inner == "BC" || inner == "DE" || inner == "SP" ||
		   inner == "IX" || inner == "IY" {
			return operand, true
		}
		
		return nil, false
		
	case OpTypeIndImm:
		// Check for indirect immediate (memory address)
		if !strings.HasPrefix(operand, "(") || !strings.HasSuffix(operand, ")") {
			return nil, false
		}
		
		inner := operand[1:len(operand)-1]
		
		// Resolve the address
		addr, err := a.resolveValue(inner)
		if err != nil {
			return nil, false
		}
		
		return addr, true
		
	case OpTypeRelative:
		// For relative jumps, resolve target and calculate offset
		target, err := a.resolveValue(operand)
		if err != nil {
			return nil, false
		}
		
		// In pass 2, calculate actual offset
		if a.pass == 2 {
			offset := int(target) - int(a.currentAddr) - 2
			if offset < -128 || offset > 127 {
				return nil, false // Out of range
			}
			return int8(offset), true
		}
		
		// In pass 1, just accept it
		return target, true
		
	case OpTypeCondition:
		// Check if it's a valid condition code
		cond := strings.ToUpper(operand)
		if pattern.Constraint != "" && cond != pattern.Constraint {
			return nil, false
		}
		
		// Valid conditions
		validConds := []string{"NZ", "Z", "NC", "C", "PO", "PE", "P", "M"}
		for _, valid := range validConds {
			if cond == valid {
				return cond, true
			}
		}
		
		return nil, false
		
	case OpTypeBit:
		// Parse bit number (0-7)
		bit, err := strconv.Atoi(operand)
		if err != nil || bit < 0 || bit > 7 {
			return nil, false
		}
		return uint8(bit), true
		
	default:
		return nil, false
	}
}

// parseReg8 parses an 8-bit register name
func parseReg8(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	switch s {
	case "A", "B", "C", "D", "E", "H", "L":
		return s
	default:
		return ""
	}
}

// parseReg16 parses a 16-bit register name
func parseReg16(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	switch s {
	case "BC", "DE", "HL", "SP", "IX", "IY", "AF":
		return s
	default:
		return ""
	}
}

