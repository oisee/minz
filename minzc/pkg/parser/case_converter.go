package parser

import (
	"strconv"
	"strings"
	
	"github.com/minz/minzc/pkg/ast"
)

// parseNumberValue converts a string to an int64 value
func parseNumberValue(s string) int64 {
	// Remove any underscores (for readability like 1_000)
	s = strings.ReplaceAll(s, "_", "")
	
	// Try to parse as hex
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		val, _ := strconv.ParseInt(s[2:], 16, 64)
		return val
	}
	
	// Try to parse as binary
	if strings.HasPrefix(s, "0b") || strings.HasPrefix(s, "0B") {
		val, _ := strconv.ParseInt(s[2:], 2, 64)
		return val
	}
	
	// Parse as decimal
	val, _ := strconv.ParseInt(s, 10, 64)
	return val
}

// parseStringValue extracts the string value from a quoted string literal
func parseStringValue(s string) string {
	// Remove quotes
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		s = s[1 : len(s)-1]
	}
	
	// Handle escape sequences
	s = strings.ReplaceAll(s, "\\n", "\n")
	s = strings.ReplaceAll(s, "\\t", "\t")
	s = strings.ReplaceAll(s, "\\r", "\r")
	s = strings.ReplaceAll(s, "\\\\", "\\")
	s = strings.ReplaceAll(s, "\\\"", "\"")
	s = strings.ReplaceAll(s, "\\'", "'")
	
	return s
}

// convertCaseExpr converts a case expression from the tree-sitter parse tree
func (p *Parser) convertCaseExpr(node *SExpNode) *ast.CaseExpr {
	caseExpr := &ast.CaseExpr{
		Arms:     []ast.CaseArm{},
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}

	// Parse case expression components
	for i := 0; i < len(node.Children); i++ {
		child := node.Children[i]
		
		// Check for field labels (value:, etc.)
		if child.Type == "atom" && child.Text == "value:" {
			if i+1 < len(node.Children) {
				valueNode := node.Children[i+1]
				if valueNode.Type == "expression" {
					caseExpr.Value = p.convertExpression(valueNode)
				}
				i++ // Skip the value node
			}
		} else if child.Type == "case_arm" {
			arm := p.convertCaseArm(child)
			if arm != nil {
				caseExpr.Arms = append(caseExpr.Arms, *arm)
			}
		}
	}

	return caseExpr
}

// convertCaseStmt converts a case statement from the tree-sitter parse tree
func (p *Parser) convertCaseStmt(node *SExpNode) *ast.CaseStmt {
	caseStmt := &ast.CaseStmt{
		Arms:     []ast.CaseArm{},
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}

	// Parse case statement components (similar to expression)
	for _, child := range node.Children {
		switch child.Type {
		case "expression":
			// First expression is the value being matched
			if caseStmt.Value == nil {
				caseStmt.Value = p.convertExpression(child)
			}
		case "case_arm":
			arm := p.convertCaseArm(child)
			if arm != nil {
				caseStmt.Arms = append(caseStmt.Arms, *arm)
			}
		}
	}

	return caseStmt
}

// convertCaseArm converts a single case arm
func (p *Parser) convertCaseArm(node *SExpNode) *ast.CaseArm {
	arm := &ast.CaseArm{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}

	// Parse arm components
	for _, child := range node.Children {
		switch child.Type {
		case "pattern":
			arm.Pattern = p.convertPattern(child)
		case "expression":
			// This is the body expression
			arm.Body = p.convertExpression(child)
		case "block":
			// Block statement as body
			arm.Body = p.convertBlock(child)
		}
		
		// Check for guard condition (if expression)
		if child.Type == "atom" && child.Text == "if" {
			// Next child should be the guard expression
			for i, c := range node.Children {
				if c == child && i+1 < len(node.Children) {
					guardNode := node.Children[i+1]
					if guardNode.Type == "expression" {
						arm.Guard = p.convertExpression(guardNode)
					}
					break
				}
			}
		}
	}

	return arm
}

// convertPattern converts a pattern node
func (p *Parser) convertPattern(node *SExpNode) ast.Pattern {
	// Check for empty pattern (wildcard _)
	if len(node.Children) == 0 {
		text := p.getNodeText(node)
		if text == "_" {
			return &ast.WildcardPattern{
				StartPos: node.StartPos,
				EndPos:   node.EndPos,
			}
		}
		// Empty children but not "_" - could be corrupted parse
		return nil
	}

	// Process child nodes
	for _, child := range node.Children {
		switch child.Type {
		case "literal_pattern":
			return p.convertLiteralPattern(child)
		case "range_pattern":
			return p.convertRangePattern(child)
		case "enum_pattern":
			return p.convertEnumPattern(child)
		case "identifier":
			// Identifier pattern (binds a variable)
			return &ast.IdentifierPattern{
				Name:     p.getNodeText(child),
				StartPos: child.StartPos,
				EndPos:   child.EndPos,
			}
		case "field_expression":
			// This might be an enum pattern like State.IDLE
			return p.convertEnumPatternFromField(child)
		}
		
		// If the child is the wildcard character
		if p.getNodeText(child) == "_" {
			return &ast.WildcardPattern{
				StartPos: child.StartPos,
				EndPos:   child.EndPos,
			}
		}
	}

	// If we get here, try to handle as a simple pattern
	text := p.getNodeText(node)
	if text == "_" {
		return &ast.WildcardPattern{
			StartPos: node.StartPos,
			EndPos:   node.EndPos,
		}
	}

	return nil
}

// convertLiteralPattern converts a literal pattern
func (p *Parser) convertLiteralPattern(node *SExpNode) ast.Pattern {
	// The literal pattern contains a literal value
	for _, child := range node.Children {
		switch child.Type {
		case "number_literal":
			return &ast.LiteralPattern{
				Value: &ast.NumberLiteral{
					Value:    parseNumberValue(p.getNodeText(child)),
					StartPos: child.StartPos,
					EndPos:   child.EndPos,
				},
				StartPos: node.StartPos,
				EndPos:   node.EndPos,
			}
		case "string_literal":
			return &ast.LiteralPattern{
				Value: &ast.StringLiteral{
					Value:    parseStringValue(p.getNodeText(child)),
					StartPos: child.StartPos,
					EndPos:   child.EndPos,
				},
				StartPos: node.StartPos,
				EndPos:   node.EndPos,
			}
		case "boolean_literal":
			return &ast.LiteralPattern{
				Value: &ast.BooleanLiteral{
					Value:    p.getNodeText(child) == "true",
					StartPos: child.StartPos,
					EndPos:   child.EndPos,
				},
				StartPos: node.StartPos,
				EndPos:   node.EndPos,
			}
		}
	}
	return nil
}

// convertRangePattern converts a range pattern (e.g., 1..10)
func (p *Parser) convertRangePattern(node *SExpNode) ast.Pattern {
	rangePattern := &ast.RangePattern{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}

	// Look for start and end fields
	for i := 0; i < len(node.Children); i++ {
		child := node.Children[i]
		if child.Type == "atom" {
			if child.Text == "start:" && i+1 < len(node.Children) {
				startNode := node.Children[i+1]
				if startNode.Type == "number_literal" {
					rangePattern.Start = &ast.NumberLiteral{
						Value:    parseNumberValue(p.getNodeText(startNode)),
						StartPos: startNode.StartPos,
						EndPos:   startNode.EndPos,
					}
				} else if startNode.Type == "identifier" {
					rangePattern.Start = &ast.Identifier{
						Name:     p.getNodeText(startNode),
						StartPos: startNode.StartPos,
						EndPos:   startNode.EndPos,
					}
				}
				i++
			} else if child.Text == "end:" && i+1 < len(node.Children) {
				endNode := node.Children[i+1]
				if endNode.Type == "number_literal" {
					rangePattern.RangeEnd = &ast.NumberLiteral{
						Value:    parseNumberValue(p.getNodeText(endNode)),
						StartPos: endNode.StartPos,
						EndPos:   endNode.EndPos,
					}
				} else if endNode.Type == "identifier" {
					rangePattern.RangeEnd = &ast.Identifier{
						Name:     p.getNodeText(endNode),
						StartPos: endNode.StartPos,
						EndPos:   endNode.EndPos,
					}
				}
				i++
			}
		}
	}

	return rangePattern
}

// convertEnumPattern converts an enum pattern (e.g., State.IDLE)
func (p *Parser) convertEnumPattern(node *SExpNode) ast.Pattern {
	enumPattern := &ast.EnumPattern{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}

	// Look for type and variant fields
	for i := 0; i < len(node.Children); i++ {
		child := node.Children[i]
		if child.Type == "atom" {
			if child.Text == "type:" && i+1 < len(node.Children) {
				typeNode := node.Children[i+1]
				if typeNode.Type == "identifier" {
					enumPattern.EnumType = p.getNodeText(typeNode)
				}
				i++
			} else if child.Text == "variant:" && i+1 < len(node.Children) {
				variantNode := node.Children[i+1]
				if variantNode.Type == "identifier" {
					enumPattern.Variant = p.getNodeText(variantNode)
				}
				i++
			}
		}
	}

	return enumPattern
}

// convertEnumPatternFromField converts a field expression to an enum pattern
func (p *Parser) convertEnumPatternFromField(node *SExpNode) ast.Pattern {
	// Convert field expression (State.IDLE) to enum pattern
	fieldExpr := p.convertFieldExpr(node)
	if fieldExpr != nil {
		if id, ok := fieldExpr.Object.(*ast.Identifier); ok {
			return &ast.EnumPattern{
				EnumType: id.Name,
				Variant:  fieldExpr.Field,
				StartPos: node.StartPos,
				EndPos:   node.EndPos,
			}
		}
	}
	return nil
}