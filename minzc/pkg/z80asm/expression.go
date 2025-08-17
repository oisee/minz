package z80asm

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// EvaluateExpression evaluates arithmetic expressions in assembly operands
func (a *Assembler) EvaluateExpression(expr string) (uint16, error) {
	expr = strings.TrimSpace(expr)
	
	// Check for combined operators first (longest patterns)
	// ^^H and ^^L - alignment followed by byte extraction
	if strings.HasSuffix(expr, "^^H") || strings.HasSuffix(expr, "^^h") {
		baseExpr := expr[:len(expr)-3]
		val, err := a.EvaluateExpression(baseExpr)
		if err != nil {
			return 0, err
		}
		// Align then extract high byte
		aligned := (val + 0xFF) & 0xFF00
		return (aligned >> 8) & 0xFF, nil
	}
	
	if strings.HasSuffix(expr, "^^L") || strings.HasSuffix(expr, "^^l") {
		baseExpr := expr[:len(expr)-3]
		val, err := a.EvaluateExpression(baseExpr)
		if err != nil {
			return 0, err
		}
		// Align then extract low byte (should always be 0x00)
		aligned := (val + 0xFF) & 0xFF00
		return aligned & 0xFF, nil
	}
	
	// Check for address alignment (^^)
	if strings.HasSuffix(expr, "^^") {
		baseExpr := expr[:len(expr)-2]
		val, err := a.EvaluateExpression(baseExpr)
		if err != nil {
			return 0, err
		}
		// Align to next 256-byte boundary (unless already aligned)
		// Examples: $1234 -> $1300, $1200 -> $1200, $12FF -> $1300
		aligned := (val + 0xFF) & 0xFF00
		return aligned, nil
	}
	
	// Check for high/low byte extraction
	if strings.HasSuffix(expr, "^H") || strings.HasSuffix(expr, "^h") {
		baseExpr := expr[:len(expr)-2]
		val, err := a.EvaluateExpression(baseExpr)
		if err != nil {
			return 0, err
		}
		return (val >> 8) & 0xFF, nil // High byte
	}
	
	if strings.HasSuffix(expr, "^L") || strings.HasSuffix(expr, "^l") {
		baseExpr := expr[:len(expr)-2]
		val, err := a.EvaluateExpression(baseExpr)
		if err != nil {
			return 0, err
		}
		return val & 0xFF, nil // Low byte
	}
	
	// Check for current address symbol first
	if expr == "$" {
		return a.currentAddr, nil
	}
	
	// Quick path for simple numbers
	if val, err := a.parseImmediate(expr); err == nil {
		return uint16(val), nil
	}
	
	// Check for single symbol
	if isValidSymbol(expr) {
		if sym, ok := a.symbols[strings.ToUpper(expr)]; ok {
			return sym.Value, nil
		}
		// Don't error here - let the caller handle undefined symbols
		return 0, fmt.Errorf("undefined symbol: %s", expr)
	}
	
	// Parse arithmetic expression
	return a.evaluateArithmeticExpression(expr)
}

// evaluateArithmeticExpression handles +, -, *, / operations
func (a *Assembler) evaluateArithmeticExpression(expr string) (uint16, error) {
	// Simple tokenizer for arithmetic expressions
	// Supports: symbol+number, symbol-number, number+number, etc.
	
	// Check for operators that shouldn't be split (^^H, ^^L, ^^, ^H, ^L)
	// Handle combined operators first (longest patterns)
	if strings.HasSuffix(expr, "^^H") || strings.HasSuffix(expr, "^^h") {
		baseExpr := expr[:len(expr)-3]
		val, err := a.EvaluateExpression(baseExpr)
		if err != nil {
			return 0, err
		}
		// Align then extract high byte
		aligned := (val + 0xFF) & 0xFF00
		return (aligned >> 8) & 0xFF, nil
	}
	
	if strings.HasSuffix(expr, "^^L") || strings.HasSuffix(expr, "^^l") {
		baseExpr := expr[:len(expr)-3]
		val, err := a.EvaluateExpression(baseExpr)
		if err != nil {
			return 0, err
		}
		// Align then extract low byte (should always be 0x00)
		aligned := (val + 0xFF) & 0xFF00
		return aligned & 0xFF, nil
	}
	
	// Handle address alignment (^^)
	if strings.HasSuffix(expr, "^^") {
		baseExpr := expr[:len(expr)-2]
		val, err := a.EvaluateExpression(baseExpr)
		if err != nil {
			return 0, err
		}
		// Align to next 256-byte boundary (unless already aligned)
		aligned := (val + 0xFF) & 0xFF00
		return aligned, nil
	}
	
	// Check for high/low byte extraction
	if strings.HasSuffix(expr, "^H") || strings.HasSuffix(expr, "^h") {
		baseExpr := expr[:len(expr)-2]
		val, err := a.EvaluateExpression(baseExpr)
		if err != nil {
			return 0, err
		}
		return (val >> 8) & 0xFF, nil // High byte
	}
	
	if strings.HasSuffix(expr, "^L") || strings.HasSuffix(expr, "^l") {
		baseExpr := expr[:len(expr)-2]
		val, err := a.EvaluateExpression(baseExpr)
		if err != nil {
			return 0, err
		}
		return val & 0xFF, nil // Low byte
	}
	
	// Handle addition
	if idx := strings.LastIndex(expr, "+"); idx > 0 {
		left := expr[:idx]
		right := expr[idx+1:]
		
		leftVal, err := a.EvaluateExpression(left)
		if err != nil {
			return 0, err
		}
		
		rightVal, err := a.EvaluateExpression(right)
		if err != nil {
			return 0, err
		}
		
		return leftVal + rightVal, nil
	}
	
	// Handle subtraction (but not negative numbers)
	if idx := strings.LastIndex(expr, "-"); idx > 0 {
		left := expr[:idx]
		right := expr[idx+1:]
		
		leftVal, err := a.EvaluateExpression(left)
		if err != nil {
			return 0, err
		}
		
		rightVal, err := a.EvaluateExpression(right)
		if err != nil {
			return 0, err
		}
		
		return leftVal - rightVal, nil
	}
	
	// Handle multiplication
	if idx := strings.Index(expr, "*"); idx > 0 {
		left := expr[:idx]
		right := expr[idx+1:]
		
		leftVal, err := a.EvaluateExpression(left)
		if err != nil {
			return 0, err
		}
		
		rightVal, err := a.EvaluateExpression(right)
		if err != nil {
			return 0, err
		}
		
		return leftVal * rightVal, nil
	}
	
	// Handle division
	if idx := strings.Index(expr, "/"); idx > 0 {
		left := expr[:idx]
		right := expr[idx+1:]
		
		leftVal, err := a.EvaluateExpression(left)
		if err != nil {
			return 0, err
		}
		
		rightVal, err := a.EvaluateExpression(right)
		if err != nil {
			return 0, err
		}
		
		if rightVal == 0 {
			return 0, fmt.Errorf("division by zero")
		}
		
		return leftVal / rightVal, nil
	}
	
	// Check for current address symbol
	if expr == "$" {
		return a.currentAddr, nil
	}
	
	// No operators found, try to parse as immediate
	if val, err := a.parseImmediate(expr); err == nil {
		return uint16(val), nil
	}
	
	return 0, fmt.Errorf("invalid expression: %s", expr)
}

// isValidSymbol checks if a string is a valid symbol name
func isValidSymbol(s string) bool {
	if len(s) == 0 {
		return false
	}
	
	// Must start with letter or underscore
	if !unicode.IsLetter(rune(s[0])) && s[0] != '_' {
		return false
	}
	
	// Rest can be letters, digits, or underscore
	for _, r := range s[1:] {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	
	return true
}

// parseImmediate parses a number in various formats (hex, binary, decimal)
func (a *Assembler) parseImmediate(s string) (uint16, error) {
	s = strings.TrimSpace(s)
	
	// Handle different number formats
	if strings.HasPrefix(s, "$") {
		// Special case: bare "$" is current address symbol, not hex
		if s == "$" {
			return 0, fmt.Errorf("current address symbol not handled here")
		}
		// Hex: $FF
		val, err := strconv.ParseUint(s[1:], 16, 16)
		return uint16(val), err
	} else if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		// Hex: 0xFF
		val, err := strconv.ParseUint(s[2:], 16, 16)
		return uint16(val), err
	} else if strings.HasPrefix(s, "%") {
		// Binary: %11111111
		val, err := strconv.ParseUint(s[1:], 2, 16)
		return uint16(val), err
	} else if strings.HasPrefix(s, "0b") || strings.HasPrefix(s, "0B") {
		// Binary: 0b11111111
		val, err := strconv.ParseUint(s[2:], 2, 16)
		return uint16(val), err
	} else if strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'") && len(s) == 3 {
		// Character literal: 'A'
		return uint16(s[1]), nil
	} else {
		// Try decimal
		val, err := strconv.ParseUint(s, 10, 16)
		return uint16(val), err
	}
}