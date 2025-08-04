package semantic

import (
	"fmt"
	"strings"
	"strconv"
	
	"github.com/minz/minzc/pkg/ir"
)

// EnhancedStringPart represents a part of an interpolated string
type EnhancedStringPart struct {
	IsLiteral      bool
	IsConstant     bool   // True for { constant } syntax
	IsExpression   bool   // True for {} runtime syntax
	Content        string // The literal string or expression
	EvaluatedValue string // For constants, the evaluated value
}

// parseEnhancedInterpolation parses string with enhanced { constant } syntax
func (a *Analyzer) parseEnhancedInterpolation(format string) ([]EnhancedStringPart, error) {
	var parts []EnhancedStringPart
	var current strings.Builder
	
	i := 0
	for i < len(format) {
		if i+1 < len(format) && format[i] == '{' && format[i+1] == '{' {
			// Escaped brace {{
			current.WriteRune('{')
			i += 2
		} else if i+1 < len(format) && format[i] == '}' && format[i+1] == '}' {
			// Escaped brace }}
			current.WriteRune('}')
			i += 2
		} else if format[i] == '{' {
			// Start of interpolation
			if current.Len() > 0 {
				parts = append(parts, EnhancedStringPart{
					IsLiteral: true,
					Content:   current.String(),
				})
				current.Reset()
			}
			
			i++ // Skip opening brace
			exprStart := i
			
			// Find matching }
			braceDepth := 1
			inString := false
			var stringChar rune
			
			for i < len(format) && braceDepth > 0 {
				if inString {
					if format[i] == byte(stringChar) && (i == 0 || format[i-1] != '\\') {
						inString = false
					}
				} else {
					switch format[i] {
					case '"', '\'':
						inString = true
						stringChar = rune(format[i])
					case '{':
						braceDepth++
					case '}':
						braceDepth--
					}
				}
				if braceDepth > 0 {
					i++
				}
			}
			
			if braceDepth == 0 && i > exprStart {
				expr := strings.TrimSpace(format[exprStart:i])
				
				// Determine if this is a compile-time constant
				isConstant, evaluatedValue := a.tryEvaluateConstant(expr)
				
				if isConstant {
					parts = append(parts, EnhancedStringPart{
						IsConstant:     true,
						Content:        expr,
						EvaluatedValue: evaluatedValue,
					})
				} else {
					parts = append(parts, EnhancedStringPart{
						IsExpression: true,
						Content:      expr,
					})
				}
				
				i++ // Skip closing brace
			} else {
				return nil, fmt.Errorf("unclosed interpolation at position %d", exprStart)
			}
		} else {
			current.WriteByte(format[i])
			i++
		}
	}
	
	// Add any remaining literal content
	if current.Len() > 0 {
		parts = append(parts, EnhancedStringPart{
			IsLiteral: true,
			Content:   current.String(),
		})
	}
	
	return parts, nil
}

// tryEvaluateConstant attempts to evaluate an expression as a compile-time constant
func (a *Analyzer) tryEvaluateConstant(expr string) (bool, string) {
	// First, check for simple literals
	expr = strings.TrimSpace(expr)
	
	if debug {
		fmt.Printf("DEBUG: tryEvaluateConstant: expr=%q\n", expr)
	}
	
	// Integer literal
	if val, err := strconv.ParseInt(expr, 0, 64); err == nil {
		if debug {
			fmt.Printf("DEBUG: Evaluated as integer literal: %d\n", val)
		}
		return true, fmt.Sprintf("%d", val)
	}
	
	// Boolean literal
	if expr == "true" || expr == "false" {
		return true, expr
	}
	
	// String literal (quoted)
	if (strings.HasPrefix(expr, "\"") && strings.HasSuffix(expr, "\"")) ||
	   (strings.HasPrefix(expr, "'") && strings.HasSuffix(expr, "'")) {
		// Remove quotes
		return true, expr[1:len(expr)-1]
	}
	
	// Metafunction calls that return constants
	if strings.HasPrefix(expr, "@") {
		// Extract metafunction name and args
		parenIdx := strings.Index(expr, "(")
		if parenIdx > 0 {
			metaName := expr[1:parenIdx]
			
			// Handle compile-time metafunctions
			switch metaName {
			case "hex":
				// Parse argument
				argStr := expr[parenIdx+1 : len(expr)-1]
				if val, err := strconv.ParseInt(strings.TrimSpace(argStr), 0, 64); err == nil {
					return true, fmt.Sprintf("%X", val)
				}
			case "build_time":
				// Return a compile-time constant
				return true, "2025-08-02"
			}
		}
	}
	
	// Try to evaluate any expression with Lua (for arithmetic, etc.)
	// This will handle constant expressions like "2 + 3" or "5 * 4"
	if a.luaEvaluator != nil {
		// First check if it looks like it could be an expression
		if containsArithmeticOps(expr) || a.isConstantExpression(expr) {
			if result, err := a.luaEvaluator.EvaluateExpression(expr); err == nil {
				return true, result
			}
		}
	}
	
	// Check if it's a known constant in current scope
	if a.currentScope != nil {
		symbol := a.currentScope.Lookup(expr)
		if _, isConst := symbol.(*ConstSymbol); isConst {
			// For now, we can't evaluate the constant's value without more context
			// This would be enhanced in a full implementation
			return false, ""
		}
	}
	
	// Not a constant
	return false, ""
}

// containsArithmeticOps checks if a string contains arithmetic operators
func containsArithmeticOps(expr string) bool {
	ops := []string{"+", "-", "*", "/", "%", "(", ")"}
	for _, op := range ops {
		if strings.Contains(expr, op) {
			return true
		}
	}
	return false
}

// isConstantExpression checks if an expression contains only constants and operators
func (a *Analyzer) isConstantExpression(expr string) bool {
	// Simple check for arithmetic expressions
	// In a full implementation, this would parse the expression properly
	
	// Check if it contains only numbers, operators, and whitespace
	validChars := "0123456789+-*/%() \t"
	for _, ch := range expr {
		if !strings.ContainsRune(validChars, ch) {
			return false
		}
	}
	
	// Make sure it's not empty and contains at least one digit
	hasDigit := false
	for _, ch := range expr {
		if ch >= '0' && ch <= '9' {
			hasDigit = true
			break
		}
	}
	
	return hasDigit
}

// processEnhancedStringInterpolation handles the new { constant } syntax
func (a *Analyzer) processEnhancedStringInterpolation(format string, irFunc *ir.Function) error {
	if debug {
		fmt.Printf("DEBUG: processEnhancedStringInterpolation called with: %q\n", format)
	}
	
	parts, err := a.parseEnhancedInterpolation(format)
	if err != nil {
		return err
	}
	
	if debug {
		fmt.Printf("DEBUG: Parsed %d parts\n", len(parts))
		for i, part := range parts {
			fmt.Printf("  Part %d: IsLiteral=%v, IsConstant=%v, IsExpression=%v, Content=%q, EvaluatedValue=%q\n",
				i, part.IsLiteral, part.IsConstant, part.IsExpression, part.Content, part.EvaluatedValue)
		}
	}
	
	// Optimize by combining adjacent literals and constants
	var optimizedParts []EnhancedStringPart
	var combinedLiteral strings.Builder
	
	for _, part := range parts {
		if part.IsLiteral || part.IsConstant {
			// Can be combined into a single literal
			if part.IsLiteral {
				combinedLiteral.WriteString(part.Content)
			} else {
				combinedLiteral.WriteString(part.EvaluatedValue)
			}
		} else {
			// Runtime expression - flush any accumulated literal
			if combinedLiteral.Len() > 0 {
				optimizedParts = append(optimizedParts, EnhancedStringPart{
					IsLiteral: true,
					Content:   combinedLiteral.String(),
				})
				combinedLiteral.Reset()
			}
			optimizedParts = append(optimizedParts, part)
		}
	}
	
	// Flush any remaining literal
	if combinedLiteral.Len() > 0 {
		optimizedParts = append(optimizedParts, EnhancedStringPart{
			IsLiteral: true,
			Content:   combinedLiteral.String(),
		})
	}
	
	// Generate code for each part
	for _, part := range optimizedParts {
		if part.IsLiteral {
			a.generatePrintString(part.Content, irFunc)
		} else if part.IsExpression {
			// Parse and evaluate runtime expression
			if debug {
				fmt.Printf("DEBUG: Processing runtime expression: %q\n", part.Content)
			}
			expr := a.parseSimpleExpression(part.Content, irFunc)
			if debug {
				fmt.Printf("DEBUG: parseSimpleExpression returned: %v (type: %T)\n", expr, expr)
			}
			if expr != nil {
				reg, err := a.analyzeExpression(expr, irFunc)
				if err != nil {
					return fmt.Errorf("failed to evaluate expression %s: %w", part.Content, err)
				}
				exprType := a.exprTypes[expr]
				a.generatePrintValue(reg, exprType, irFunc)
			} else {
				// If parseSimpleExpression returns nil, we can't handle this expression yet
				return fmt.Errorf("unsupported expression type: %v", expr)
			}
		}
	}
	
	return nil
}