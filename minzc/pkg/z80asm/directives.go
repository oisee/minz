package z80asm

import (
	"fmt"
	"strings"
)

// processDirective handles assembly directives
func (a *Assembler) processDirective(line *Line) error {
	directive := strings.ToUpper(line.Directive)
	
	switch directive {
	case "ORG":
		return a.handleORG(line)
	case "DB", "DEFB":
		return a.handleDB(line)
	case "DW", "DEFW":
		return a.handleDW(line)
	case "DS", "DEFS":
		return a.handleDS(line)
	case "EQU":
		return a.handleEQU(line)
	case "ALIGN":
		return a.handleALIGN(line)
	case "END":
		return a.handleEND(line)
	case "INCLUDE":
		return a.handleINCLUDE(line)
	case "MACRO":
		return a.handleMACRO(line)
	case "ENDM":
		return a.handleENDM(line)
	default:
		if a.Strict {
			return fmt.Errorf("unknown directive: %s", directive)
		}
		// Ignore unknown directives in non-strict mode
		return nil
	}
}

// handleORG sets the origin address
func (a *Assembler) handleORG(line *Line) error {
	if len(line.Operands) != 1 {
		return fmt.Errorf("ORG requires exactly one operand")
	}
	
	addr, err := a.resolveValue(line.Operands[0])
	if err != nil {
		return fmt.Errorf("invalid ORG address: %w", err)
	}
	
	a.currentAddr = addr
	if a.pass == 1 && a.origin == 0x8000 { // Default origin
		a.origin = addr
	}
	
	return nil
}

// handleDB handles byte definitions
func (a *Assembler) handleDB(line *Line) error {
	if len(line.Operands) == 0 {
		return fmt.Errorf("DB requires at least one operand")
	}
	
	var bytes []byte
	
	for _, operand := range line.Operands {
		// Check if it's a string
		if isString(operand) {
			str := parseString(operand)
			bytes = append(bytes, []byte(str)...)
		} else {
			// Parse as numeric value
			val, err := a.resolveValue(operand)
			if err != nil {
				return fmt.Errorf("invalid DB operand '%s': %w", operand, err)
			}
			if val > 255 {
				return fmt.Errorf("DB value out of range: %d", val)
			}
			bytes = append(bytes, byte(val))
		}
	}
	
	if a.pass == 2 {
		inst := &AssembledInstruction{
			Address: a.currentAddr,
			Line:    line,
			Bytes:   bytes,
		}
		a.instructions = append(a.instructions, inst)
		a.output = append(a.output, bytes...)
	}
	
	a.currentAddr += uint16(len(bytes))
	return nil
}

// handleDW handles word definitions
func (a *Assembler) handleDW(line *Line) error {
	if len(line.Operands) == 0 {
		return fmt.Errorf("DW requires at least one operand")
	}
	
	var bytes []byte
	
	for _, operand := range line.Operands {
		val, err := a.resolveValue(operand)
		if err != nil {
			return fmt.Errorf("invalid DW operand '%s': %w", operand, err)
		}
		// Little-endian encoding
		bytes = append(bytes, byte(val), byte(val>>8))
	}
	
	if a.pass == 2 {
		inst := &AssembledInstruction{
			Address: a.currentAddr,
			Line:    line,
			Bytes:   bytes,
		}
		a.instructions = append(a.instructions, inst)
		a.output = append(a.output, bytes...)
	}
	
	a.currentAddr += uint16(len(bytes))
	return nil
}

// handleDS handles space definitions
func (a *Assembler) handleDS(line *Line) error {
	if len(line.Operands) == 0 {
		return fmt.Errorf("DS requires at least one operand")
	}
	
	// Get size
	size, err := a.resolveValue(line.Operands[0])
	if err != nil {
		return fmt.Errorf("invalid DS size: %w", err)
	}
	
	// Get fill value (default 0)
	fillValue := byte(0)
	if len(line.Operands) > 1 {
		val, err := a.resolveValue(line.Operands[1])
		if err != nil {
			return fmt.Errorf("invalid DS fill value: %w", err)
		}
		if val > 255 {
			return fmt.Errorf("DS fill value out of range: %d", val)
		}
		fillValue = byte(val)
	}
	
	if a.pass == 2 {
		bytes := make([]byte, size)
		for i := range bytes {
			bytes[i] = fillValue
		}
		
		inst := &AssembledInstruction{
			Address: a.currentAddr,
			Line:    line,
			Bytes:   bytes,
		}
		a.instructions = append(a.instructions, inst)
		a.output = append(a.output, bytes...)
	}
	
	a.currentAddr += size
	return nil
}

// handleEQU handles constant definitions
func (a *Assembler) handleEQU(line *Line) error {
	if line.Label == "" {
		return fmt.Errorf("EQU requires a label")
	}
	if len(line.Operands) != 1 {
		return fmt.Errorf("EQU requires exactly one operand")
	}
	
	value, err := a.resolveValue(line.Operands[0])
	if err != nil {
		return fmt.Errorf("invalid EQU value: %w", err)
	}
	
	// Define the symbol
	label := line.Label
	if !a.CaseSensitive {
		label = strings.ToUpper(label)
	}
	
	if a.pass == 1 {
		if sym, exists := a.symbols[label]; exists && sym.Defined {
			return fmt.Errorf("symbol '%s' already defined", label)
		}
		
		a.symbols[label] = &Symbol{
			Name:    label,
			Value:   value,
			Defined: true,
		}
	}
	
	return nil
}

// handleALIGN aligns to a boundary
func (a *Assembler) handleALIGN(line *Line) error {
	if len(line.Operands) != 1 {
		return fmt.Errorf("ALIGN requires exactly one operand")
	}
	
	alignment, err := a.resolveValue(line.Operands[0])
	if err != nil {
		return fmt.Errorf("invalid ALIGN value: %w", err)
	}
	
	// Check if alignment is power of 2
	if alignment == 0 || (alignment&(alignment-1)) != 0 {
		return fmt.Errorf("ALIGN value must be a power of 2")
	}
	
	// Calculate padding needed
	remainder := a.currentAddr % alignment
	if remainder != 0 {
		padding := alignment - remainder
		
		if a.pass == 2 {
			bytes := make([]byte, padding)
			inst := &AssembledInstruction{
				Address: a.currentAddr,
				Line:    line,
				Bytes:   bytes,
			}
			a.instructions = append(a.instructions, inst)
			a.output = append(a.output, bytes...)
		}
		
		a.currentAddr += padding
	}
	
	return nil
}

// handleEND marks end of assembly
func (a *Assembler) handleEND(line *Line) error {
	// Could implement early termination, but for now just ignore
	return nil
}

// handleINCLUDE includes another file
func (a *Assembler) handleINCLUDE(line *Line) error {
	if len(line.Operands) != 1 {
		return fmt.Errorf("INCLUDE requires exactly one operand")
	}
	
	// For now, we don't support includes
	return fmt.Errorf("INCLUDE directive not yet implemented")
}

// handleMACRO begins a macro definition
func (a *Assembler) handleMACRO(line *Line) error {
	if !a.EnableMacros {
		return fmt.Errorf("macros are disabled")
	}
	
	// Parse macro name and parameters
	parts := strings.Fields(line.Operands)
	if len(parts) < 1 {
		return fmt.Errorf("MACRO requires a name")
	}
	
	macroName := parts[0]
	var params []string
	
	// Parse parameters (if any)
	if len(parts) > 1 {
		// Join remaining parts and split by comma
		paramStr := strings.Join(parts[1:], " ")
		paramStr = strings.ReplaceAll(paramStr, " ", "")
		if paramStr != "" {
			params = strings.Split(paramStr, ",")
		}
	}
	
	// Start collecting macro body
	a.macroDefinition = &macroDefinitionState{
		name:   macroName,
		params: params,
		body:   []string{},
	}
	
	return nil
}

// handleENDM ends a macro definition
func (a *Assembler) handleENDM(line *Line) error {
	if !a.EnableMacros {
		return fmt.Errorf("macros are disabled")
	}
	
	if a.macroDefinition == nil {
		return fmt.Errorf("ENDM without matching MACRO")
	}
	
	// Register the macro
	err := a.macroProcessor.DefineMacro(
		a.macroDefinition.name,
		a.macroDefinition.params,
		a.macroDefinition.body,
	)
	
	// Clear definition state
	a.macroDefinition = nil
	
	return err
}

// Helper functions

func isString(s string) bool {
	s = strings.TrimSpace(s)
	return (strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) ||
	       (strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'"))
}

func parseString(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		return s[1 : len(s)-1]
	}
	return s
}