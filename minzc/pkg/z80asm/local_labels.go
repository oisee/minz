package z80asm

import (
	"fmt"
	"strings"
)

// LocalLabelContext manages local label scoping
type LocalLabelContext struct {
	currentGlobal string              // Current global label scope
	localLabels   map[string]*Symbol  // Map of expanded local labels
}

// NewLocalLabelContext creates a new local label context
func NewLocalLabelContext() *LocalLabelContext {
	return &LocalLabelContext{
		localLabels: make(map[string]*Symbol),
	}
}

// processLabelForContext handles both global and local labels
func (ctx *LocalLabelContext) processLabelForContext(label string, addr uint16, pass int) (string, error) {
	// Check if it's a local label (starts with exactly one dot, not multiple dots)
	if isLocalLabel(label) {
		// Local label
		if ctx.currentGlobal == "" {
			return "", fmt.Errorf("local label '%s' defined before any global label", label)
		}
		
		// Expand local label to include global scope
		expandedLabel := ctx.currentGlobal + label
		return expandedLabel, nil
	} else {
		// Global label - update current scope
		ctx.currentGlobal = label
		return label, nil
	}
}

// expandLocalLabelReferences expands local label references in operands
func (ctx *LocalLabelContext) expandLocalLabelReferences(operand string) string {
	// If operand is a local label reference (starts with exactly one dot)
	if isLocalLabel(operand) {
		if ctx.currentGlobal != "" {
			// Expand to full label name
			return ctx.currentGlobal + operand
		}
	}
	return operand
}

// preprocessLocalLabels processes all lines to expand local labels before assembly
func preprocessLocalLabels(lines []*Line) ([]*Line, error) {
	ctx := NewLocalLabelContext()
	result := make([]*Line, 0, len(lines))
	
	for _, line := range lines {
		newLine := &Line{
			Number:   line.Number,
			Label:    line.Label,
			Directive: line.Directive,
			Mnemonic: line.Mnemonic,
			Operands: make([]string, len(line.Operands)),
			Comment:  line.Comment,
			IsBlank:  line.IsBlank,
		}
		
		// Process label if present
		if line.Label != "" {
			expandedLabel, err := ctx.processLabelForContext(line.Label, 0, 1)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", line.Number, err)
			}
			
			// Update the label
			newLine.Label = expandedLabel
		}
		
		// Process operands for local label references
		for i, operand := range line.Operands {
			newLine.Operands[i] = ctx.expandLocalLabelReferences(operand)
		}
		
		result = append(result, newLine)
	}
	
	return result, nil
}

// isLocalLabel checks if a label is a local label (starts with exactly one dot)
func isLocalLabel(label string) bool {
	return strings.HasPrefix(label, ".") && !strings.HasPrefix(label, "..")
}

// getGlobalScope extracts the global scope from an expanded local label
func getGlobalScope(expandedLabel string) string {
	// Find the last dot
	lastDot := strings.LastIndex(expandedLabel, ".")
	if lastDot > 0 {
		return expandedLabel[:lastDot]
	}
	return expandedLabel
}