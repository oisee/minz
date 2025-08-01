package z80asm

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

// Line represents a parsed line from the source
type Line struct {
	Number     int
	Label      string
	Directive  string
	Mnemonic   string
	Operands   []string
	Comment    string
	IsBlank    bool
}

// ParseLine parses a single line of assembly
func ParseLine(line string, lineNum int) (*Line, error) {
	result := &Line{Number: lineNum}
	
	// Remove comments
	if idx := strings.Index(line, ";"); idx >= 0 {
		result.Comment = strings.TrimSpace(line[idx+1:])
		line = line[:idx]
	}
	
	// Trim whitespace
	line = strings.TrimSpace(line)
	
	// Check if blank
	if line == "" {
		result.IsBlank = true
		return result, nil
	}
	
	// Check for label (ends with :)
	if strings.HasSuffix(line, ":") {
		result.Label = strings.TrimSuffix(line, ":")
		return result, nil
	}
	
	// Split into tokens
	tokens := strings.Fields(line)
	if len(tokens) == 0 {
		result.IsBlank = true
		return result, nil
	}
	
	// Check if first token is a directive (starts with uppercase)
	if isDirective(tokens[0]) {
		result.Directive = strings.ToUpper(tokens[0])
		if len(tokens) > 1 {
			// Handle operands - could be comma-separated
			operandStr := strings.Join(tokens[1:], " ")
			result.Operands = parseOperands(operandStr)
		}
		return result, nil
	}
	
	// Otherwise it's a mnemonic
	result.Mnemonic = strings.ToUpper(tokens[0])
	if len(tokens) > 1 {
		// Handle operands
		operandStr := strings.Join(tokens[1:], " ")
		result.Operands = parseOperands(operandStr)
	}
	
	return result, nil
}

// isDirective checks if a token is a directive
func isDirective(token string) bool {
	upper := strings.ToUpper(token)
	directives := []string{"ORG", "END", "DB", "DEFB", "DW", "DEFW", "DS", "DEFS", "EQU"}
	for _, d := range directives {
		if upper == d {
			return true
		}
	}
	return false
}

// parseOperands splits operands by comma, handling parentheses
func parseOperands(operandStr string) []string {
	var operands []string
	var current strings.Builder
	parenDepth := 0
	
	for _, ch := range operandStr {
		switch ch {
		case '(':
			parenDepth++
			current.WriteRune(ch)
		case ')':
			parenDepth--
			current.WriteRune(ch)
		case ',':
			if parenDepth == 0 {
				operands = append(operands, strings.TrimSpace(current.String()))
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}
	
	// Add last operand
	if current.Len() > 0 {
		operands = append(operands, strings.TrimSpace(current.String()))
	}
	
	return operands
}

// ParseSource parses a complete assembly source file
func ParseSource(source string) ([]*Line, error) {
	var lines []*Line
	scanner := bufio.NewScanner(strings.NewReader(source))
	lineNum := 1
	
	for scanner.Scan() {
		line, err := ParseLine(scanner.Text(), lineNum)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}
		lines = append(lines, line)
		lineNum++
	}
	
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	
	return lines, nil
}

// parseRegister parses a register name
func parseRegister(s string) (Register, bool) {
	s = strings.ToUpper(strings.TrimSpace(s))
	switch s {
	case "A":
		return RegA, true
	case "B":
		return RegB, true
	case "C":
		return RegC, true
	case "D":
		return RegD, true
	case "E":
		return RegE, true
	case "H":
		return RegH, true
	case "L":
		return RegL, true
	case "I":
		return RegI, true
	case "R":
		return RegR, true
	case "BC":
		return RegBC, true
	case "DE":
		return RegDE, true
	case "HL":
		return RegHL, true
	case "AF":
		return RegAF, true
	case "AF'":
		return RegAF, true // Handle shadow register notation
	case "SP":
		return RegSP, true
	case "IX":
		return RegIX, true
	case "IY":
		return RegIY, true
	// Undocumented IX/IY halves
	case "IXH":
		return RegIXH, true
	case "IXL":
		return RegIXL, true
	case "IYH":
		return RegIYH, true
	case "IYL":
		return RegIYL, true
	default:
		return RegNone, false
	}
}

// parseCondition parses a condition code
func parseCondition(s string) (Condition, bool) {
	s = strings.ToUpper(strings.TrimSpace(s))
	switch s {
	case "NZ":
		return CondNZ, true
	case "Z":
		return CondZ, true
	case "NC":
		return CondNC, true
	case "C":
		return CondC, true
	case "PO":
		return CondPO, true
	case "PE":
		return CondPE, true
	case "P":
		return CondP, true
	case "M":
		return CondM, true
	default:
		return CondNone, false
	}
}

// parseNumber parses a numeric value (decimal, hex with $/#/0x, or binary)
func parseNumber(s string) (uint16, error) {
	s = strings.TrimSpace(s)
	
	// Check for hex prefixes
	if strings.HasPrefix(s, "$") {
		v, err := strconv.ParseUint(s[1:], 16, 16)
		return uint16(v), err
	} else if strings.HasPrefix(s, "#") {
		v, err := strconv.ParseUint(s[1:], 16, 16)
		return uint16(v), err
	} else if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		v, err := strconv.ParseUint(s[2:], 16, 16)
		return uint16(v), err
	} else if strings.HasPrefix(s, "%") {
		// Binary
		v, err := strconv.ParseUint(s[1:], 2, 16)
		return uint16(v), err
	}
	
	// Try decimal
	val, err := strconv.ParseUint(s, 10, 16)
	return uint16(val), err
}

// isIndirect checks if operand is indirect addressing (HL), (nn), etc
func isIndirect(s string) bool {
	return strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")")
}

// stripIndirect removes parentheses from indirect operand
func stripIndirect(s string) string {
	if isIndirect(s) {
		return s[1 : len(s)-1]
	}
	return s
}