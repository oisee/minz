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
		// Track instruction size for multi-pass convergence
		if a.instructionSizes != nil {
			a.instructionSizes[line.Number] = len(encoded)
		}

		// In passes 2+, add to output
		if a.pass >= 2 {
			inst := &AssembledInstruction{
				Address: a.currentAddr,
				Line:    line,
				Bytes:   encoded,
			}
			a.instructions = append(a.instructions, inst)
			a.output = append(a.output, encoded...)
		}
		a.currentAddr += len(encoded)
		return nil
	}

	// Table-driven encoding handles all Z80 instructions
	return fmt.Errorf("unknown instruction or invalid operands: %s", line.Mnemonic)
}

// encodeInstructionTable uses the table-driven approach to encode instructions
func (a *Assembler) encodeInstructionTable(line *Line) ([]byte, error) {
	// Thread JRS flag so encodeJRRel can reserve 3 bytes in pass 1
	a.currentFromJRS = line.FromJRS

	// Parse ADL suffix (e.g., "RST.LIL" -> "RST", ADLSuffixLIL)
	baseMnemonic, adlSuffix := a.ParseADLSuffix(line.Mnemonic)

	// Get all patterns for the base mnemonic (without suffix)
	patterns := GetInstructionPatterns(baseMnemonic)
	if len(patterns) == 0 {
		return nil, fmt.Errorf("unknown instruction: %s", line.Mnemonic)
	}

	// Try to match each pattern
	for _, pattern := range patterns {
		if match, values := a.matchPattern(pattern, line); match {
			var encoded []byte
			var err error

			// Generate encoding
			if pattern.EncodingFunc != nil {
				encoded, err = pattern.EncodingFunc(a, &pattern, values)
			} else {
				// Simple encoding (no operands or fixed encoding)
				encoded = make([]byte, len(pattern.Encoding))
				copy(encoded, pattern.Encoding)
			}

			if err != nil {
				return nil, err
			}

			// Prepend ADL suffix byte if specified (eZ80 mode)
			if adlSuffix != ADLSuffixNone {
				result := make([]byte, 1+len(encoded))
				result[0] = adlSuffix
				copy(result[1:], encoded)
				return result, nil
			}

			return encoded, nil
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
	operand = normalizeBrackets(operand)

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
		// Reject register names — they must match OpTypeReg8/OpTypeReg16, not immediate
		upper := strings.ToUpper(operand)
		switch upper {
		case "A", "B", "C", "D", "E", "F", "H", "L",
			"BC", "DE", "HL", "SP", "AF", "IX", "IY",
			"I", "R", "IXH", "IXL", "IYH", "IYL", "AF'":
			return nil, false
		}

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

		inner := strings.ToUpper(operand[1:len(operand)-1])
		upperOperand := strings.ToUpper(operand)

		// Check constraint if specified (case-insensitive)
		if pattern.Constraint != "" {
			if !strings.EqualFold(operand, pattern.Constraint) {
				return nil, false
			}
			return upperOperand, true
		}

		// Check if it's a valid indirect register (case-insensitive)
		if inner == "HL" || inner == "BC" || inner == "DE" || inner == "SP" ||
		   inner == "IX" || inner == "IY" || inner == "C" {
			return upperOperand, true
		}

		return nil, false
		
	case OpTypeIndImm:
		// Check for indirect immediate (memory address)
		if !strings.HasPrefix(operand, "(") || !strings.HasSuffix(operand, ")") {
			return nil, false
		}

		inner := operand[1 : len(operand)-1]

		// Reject IX/IY indexed addressing — those use OpTypeIndIdx
		upperInner := strings.ToUpper(strings.TrimSpace(inner))
		if strings.HasPrefix(upperInner, "IX") || strings.HasPrefix(upperInner, "IY") {
			return nil, false
		}

		// Reject register names — those use OpTypeIndReg
		switch upperInner {
		case "HL", "BC", "DE", "SP", "C",
			"A", "B", "D", "E", "H", "L":
			return nil, false
		}

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
		if a.pass >= 2 {
			offset := target - a.currentAddr - 2
			if offset >= -128 && offset <= 127 {
				return int8(offset), true
			}
			// Out of JR range — return absolute target as int so
			// encodeJRRel can auto-promote JR → JP (3 bytes).
			return target, true
		}

		// In pass 1, just accept it (return absolute address)
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
		
	case OpTypeIndIdx:
		// Check for (IX+d) or (IY+d) indexed addressing
		if !strings.HasPrefix(operand, "(") || !strings.HasSuffix(operand, ")") {
			return nil, false
		}
		inner := strings.TrimSpace(operand[1 : len(operand)-1])
		upper := strings.ToUpper(inner)

		// Check constraint (IX or IY)
		prefix := pattern.Constraint
		if prefix == "" {
			if strings.HasPrefix(upper, "IX") {
				prefix = "IX"
			} else if strings.HasPrefix(upper, "IY") {
				prefix = "IY"
			} else {
				return nil, false
			}
		}
		if !strings.HasPrefix(upper, prefix) {
			return nil, false
		}

		rest := inner[len(prefix):]
		if rest == "" {
			// (IX) = (IX+0)
			return int8(0), true
		}
		if rest[0] != '+' && rest[0] != '-' {
			return nil, false
		}

		var offsetStr string
		if rest[0] == '+' {
			offsetStr = strings.TrimSpace(rest[1:])
		} else {
			offsetStr = strings.TrimSpace(rest) // Keep the minus sign
		}

		val, err := parseNumber(offsetStr)
		if err != nil {
			return nil, false
		}
		if val > 127 && val < 0xFF80 {
			return nil, false
		}
		return int8(val), true

	case OpTypeBit:
		// Parse bit number (0-7) — try literal first, then resolve EQU constants
		bit, err := strconv.Atoi(operand)
		if err != nil {
			// Try resolving as symbol (e.g., EQU constant)
			val, resolveErr := a.resolveValue(operand)
			if resolveErr != nil {
				return nil, false
			}
			bit = val
		}
		if bit < 0 || bit > 7 {
			return nil, false
		}
		return uint8(bit), true

	default:
		return nil, false
	}
}

// parseReg8 parses an 8-bit register name (including undocumented IX/IY halves)
func parseReg8(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	switch s {
	case "A", "B", "C", "D", "E", "H", "L", "I", "R", "F",
		"IXH", "IXL", "IYH", "IYL":
		return s
	default:
		return ""
	}
}

// parseReg16 parses a 16-bit register name
func parseReg16(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	switch s {
	case "BC", "DE", "HL", "SP", "IX", "IY", "AF", "AF'":
		return s
	default:
		return ""
	}
}

// resolveValue resolves an operand to a numeric value
func (a *Assembler) resolveValue(operand string) (int, error) {
	// Use the expression evaluator which handles numbers, symbols, and arithmetic
	return a.EvaluateExpression(operand)
}

