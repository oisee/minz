package z80asm

import (
	"fmt"
	"strings"
)

// Macro represents an assembler macro definition
type Macro struct {
	Name       string
	Parameters []string
	Body       []string
	LocalCount int // Counter for unique local labels
}

// MacroExpansion represents an expanded macro instance
type MacroExpansion struct {
	Macro      *Macro
	Arguments  map[string]string
	LocalBase  int // Base number for local labels in this expansion
}

// MacroProcessor handles macro definitions and expansions
type MacroProcessor struct {
	macros        map[string]*Macro
	localCounter  int // Global counter for unique local labels
	expansionDepth int
	maxDepth      int
}

// NewMacroProcessor creates a new macro processor
func NewMacroProcessor() *MacroProcessor {
	return &MacroProcessor{
		macros:   make(map[string]*Macro),
		maxDepth: 10, // Maximum recursion depth
	}
}

// DefineMacro adds a new macro definition
func (mp *MacroProcessor) DefineMacro(name string, params []string, body []string) error {
	if _, exists := mp.macros[name]; exists {
		return fmt.Errorf("macro '%s' already defined", name)
	}
	
	// Validate parameters
	paramSet := make(map[string]bool)
	for _, param := range params {
		if paramSet[param] {
			return fmt.Errorf("duplicate parameter '%s' in macro '%s'", param, name)
		}
		paramSet[param] = true
	}
	
	mp.macros[name] = &Macro{
		Name:       name,
		Parameters: params,
		Body:       body,
	}
	
	return nil
}

// ExpandMacro expands a macro invocation
func (mp *MacroProcessor) ExpandMacro(name string, args []string) ([]string, error) {
	macro, exists := mp.macros[name]
	if !exists {
		return nil, fmt.Errorf("undefined macro '%s'", name)
	}
	
	// Check recursion depth
	if mp.expansionDepth >= mp.maxDepth {
		return nil, fmt.Errorf("macro expansion depth exceeded (max %d)", mp.maxDepth)
	}
	
	// Check argument count
	if len(args) != len(macro.Parameters) {
		return nil, fmt.Errorf("macro '%s' expects %d arguments, got %d", 
			name, len(macro.Parameters), len(args))
	}
	
	// Create argument map
	argMap := make(map[string]string)
	for i, param := range macro.Parameters {
		argMap[param] = args[i]
	}
	
	// Get unique local label base
	localBase := mp.localCounter
	mp.localCounter += 100 // Reserve space for up to 100 local labels per expansion
	
	// Expand macro body
	mp.expansionDepth++
	defer func() { mp.expansionDepth-- }()
	
	var expanded []string
	for _, line := range macro.Body {
		expandedLine := mp.substituteLine(line, argMap, localBase)
		
		// Handle nested macro calls
		if mp.isMacroCall(expandedLine) {
			nestedLines, err := mp.expandNestedMacro(expandedLine)
			if err != nil {
				return nil, err
			}
			expanded = append(expanded, nestedLines...)
		} else {
			expanded = append(expanded, expandedLine)
		}
	}
	
	return expanded, nil
}

// substituteLine replaces parameters and local labels in a line
func (mp *MacroProcessor) substituteLine(line string, args map[string]string, localBase int) string {
	result := line
	
	// Replace parameters
	for param, value := range args {
		// Replace with word boundaries to avoid partial replacements
		result = mp.replaceParameter(result, param, value)
	}
	
	// Replace local labels (.label becomes .L<localBase>_label)
	result = mp.replaceLocalLabels(result, localBase)
	
	// Handle special macro directives
	result = mp.handleSpecialDirectives(result, args)
	
	return result
}

// replaceParameter replaces a parameter with its value
func (mp *MacroProcessor) replaceParameter(line, param, value string) string {
	// Use special markers to ensure word boundaries
	patterns := []string{
		"{" + param + "}",           // {param} style
		"%" + param,                  // %param style
		"&" + param,                  // &param style (string substitution)
	}
	
	result := line
	for _, pattern := range patterns {
		result = strings.ReplaceAll(result, pattern, value)
	}
	
	// Also replace bare parameter names if they're standalone
	words := strings.Fields(result)
	for i, word := range words {
		if word == param || strings.TrimSuffix(word, ",") == param {
			suffix := ""
			if strings.HasSuffix(word, ",") {
				suffix = ","
			}
			words[i] = value + suffix
		}
	}
	
	return strings.Join(words, " ")
}

// replaceLocalLabels converts local labels to unique global labels
func (mp *MacroProcessor) replaceLocalLabels(line string, localBase int) string {
	// Replace .label with .L<localBase>_label
	result := line
	
	// Find all local labels (starting with .)
	parts := strings.Fields(result)
	for i, part := range parts {
		if strings.HasPrefix(part, ".") && len(part) > 1 {
			// Check if it's a label definition or reference
			labelName := strings.TrimSuffix(part, ":")
			if strings.HasPrefix(labelName, ".") {
				newLabel := fmt.Sprintf(".L%d_%s", localBase, labelName[1:])
				if strings.HasSuffix(part, ":") {
					newLabel += ":"
				}
				parts[i] = newLabel
			}
		}
	}
	
	return strings.Join(parts, " ")
}

// handleSpecialDirectives processes special macro directives
func (mp *MacroProcessor) handleSpecialDirectives(line string, args map[string]string) string {
	trimmed := strings.TrimSpace(line)
	
	// Handle conditional assembly
	if strings.HasPrefix(trimmed, "#IF") {
		// Simple conditional - could be expanded
		condition := strings.TrimPrefix(trimmed, "#IF")
		condition = strings.TrimSpace(condition)
		// Evaluate condition (simplified)
		if condition == "0" || condition == "" {
			return "; " + line // Comment out
		}
	}
	
	// Handle repetition
	if strings.HasPrefix(trimmed, "#REPT") {
		// Repetition directive - would need more complex handling
		return "; REPT not yet implemented: " + line
	}
	
	// Handle string length
	if strings.Contains(line, "#STRLEN") {
		// Replace #STRLEN(string) with length
		// Simplified implementation
		line = strings.ReplaceAll(line, "#STRLEN", "; STRLEN")
	}
	
	return line
}

// isMacroCall checks if a line is a macro invocation
func (mp *MacroProcessor) isMacroCall(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, ";") {
		return false
	}
	
	// Get first word (potential macro name)
	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return false
	}
	
	// Check if it's a label
	if strings.HasSuffix(parts[0], ":") {
		if len(parts) > 1 {
			_, exists := mp.macros[parts[1]]
			return exists
		}
		return false
	}
	
	// Check if it's a known macro
	_, exists := mp.macros[parts[0]]
	return exists
}

// expandNestedMacro expands a nested macro call
func (mp *MacroProcessor) expandNestedMacro(line string) ([]string, error) {
	trimmed := strings.TrimSpace(line)
	parts := strings.Fields(trimmed)
	
	if len(parts) == 0 {
		return nil, nil
	}
	
	// Handle label prefix
	startIdx := 0
	var labelPrefix string
	if strings.HasSuffix(parts[0], ":") {
		labelPrefix = parts[0] + " "
		startIdx = 1
	}
	
	if startIdx >= len(parts) {
		return nil, nil
	}
	
	macroName := parts[startIdx]
	
	// Parse arguments (simplified - doesn't handle complex expressions)
	var args []string
	if startIdx+1 < len(parts) {
		// Join remaining parts and split by comma
		argString := strings.Join(parts[startIdx+1:], " ")
		args = mp.parseArguments(argString)
	}
	
	// Expand the macro
	expanded, err := mp.ExpandMacro(macroName, args)
	if err != nil {
		return nil, err
	}
	
	// Add label prefix to first line if present
	if labelPrefix != "" && len(expanded) > 0 {
		expanded[0] = labelPrefix + expanded[0]
	}
	
	return expanded, nil
}

// parseArguments parses macro arguments from a string
func (mp *MacroProcessor) parseArguments(argString string) []string {
	var args []string
	var current strings.Builder
	parenDepth := 0
	inString := false
	var stringChar rune
	
	for _, ch := range argString {
		switch ch {
		case '"', '\'':
			if !inString {
				inString = true
				stringChar = ch
			} else if ch == stringChar {
				inString = false
			}
			current.WriteRune(ch)
			
		case '(':
			if !inString {
				parenDepth++
			}
			current.WriteRune(ch)
			
		case ')':
			if !inString {
				parenDepth--
			}
			current.WriteRune(ch)
			
		case ',':
			if !inString && parenDepth == 0 {
				// End of argument
				arg := strings.TrimSpace(current.String())
				if arg != "" {
					args = append(args, arg)
				}
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
			
		default:
			current.WriteRune(ch)
		}
	}
	
	// Add last argument
	arg := strings.TrimSpace(current.String())
	if arg != "" {
		args = append(args, arg)
	}
	
	return args
}

// GetMacro returns a macro definition if it exists
func (mp *MacroProcessor) GetMacro(name string) (*Macro, bool) {
	macro, exists := mp.macros[name]
	return macro, exists
}

// Clear removes all macro definitions
func (mp *MacroProcessor) Clear() {
	mp.macros = make(map[string]*Macro)
	mp.localCounter = 0
	mp.expansionDepth = 0
}

// Standard Macros - commonly used macro definitions

// DefineStandardMacros adds commonly used macros
func (mp *MacroProcessor) DefineStandardMacros() {
	// PUSH_ALL - Save all registers
	mp.DefineMacro("PUSH_ALL", []string{}, []string{
		"PUSH AF",
		"PUSH BC",
		"PUSH DE",
		"PUSH HL",
		"PUSH IX",
		"PUSH IY",
	})
	
	// POP_ALL - Restore all registers
	mp.DefineMacro("POP_ALL", []string{}, []string{
		"POP IY",
		"POP IX",
		"POP HL",
		"POP DE",
		"POP BC",
		"POP AF",
	})
	
	// MEMCPY - Copy memory block
	mp.DefineMacro("MEMCPY", []string{"dst", "src", "size"}, []string{
		"LD HL, {src}",
		"LD DE, {dst}",
		"LD BC, {size}",
		"LDIR",
	})
	
	// MEMSET - Fill memory block
	mp.DefineMacro("MEMSET", []string{"dst", "value", "size"}, []string{
		"LD HL, {dst}",
		"LD A, {value}",
		"LD B, {size}",
		".loop:",
		"LD (HL), A",
		"INC HL",
		"DJNZ .loop",
	})
	
	// CALL_HL - Call address in HL
	mp.DefineMacro("CALL_HL", []string{}, []string{
		"LD DE, .return",
		"PUSH DE",
		"JP (HL)",
		".return:",
	})
	
	// DELAY - Simple delay loop
	mp.DefineMacro("DELAY", []string{"count"}, []string{
		"LD B, {count}",
		".loop:",
		"DJNZ .loop",
	})
}