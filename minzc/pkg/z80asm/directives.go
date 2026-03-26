package z80asm

import (
	"fmt"
	"os"
	"path/filepath"
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
	case "DW24", "DL":
		return a.handleDW24(line)
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
	case "INCBIN":
		return a.handleINCBIN(line)
	case "MACRO":
		return a.handleMACRO(line)
	case "ENDM":
		return a.handleENDM(line)
	case ".ASSUME", "ASSUME":
		return a.handleASSUME(line)
	case "TARGET":
		return a.handleTARGET(line)
	case "MODEL":
		return a.handleMODEL(line)
	default:
		if a.Strict {
			return fmt.Errorf("unknown directive: %s", directive)
		}
		// Ignore unknown directives in non-strict mode
		return nil
	}
}

// calculateLengthOfOperands calculates the total byte length of operands
func (a *Assembler) calculateLengthOfOperands(operands []string) int {
	length := 0
	for _, operand := range operands {
		if isString(operand) {
			// String contributes its character count
			str := parseString(operand)
			length += len(str)
		} else if strings.HasPrefix(operand, "@") {
			// Skip macros - they don't contribute to the data length
			continue
		} else {
			// Numeric values contribute 1 byte in DB context
			length++
		}
	}
	return length
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

	// Track first ORG for multi-section code (useful for entry point fallback)
	if a.pass == 1 && !a.hasFirstOrg {
		a.firstOrg = addr
		a.hasFirstOrg = true
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
	
	for i, operand := range line.Operands {
		// Check for special macros
		if operand == "@len" || operand == "@size" || operand == "@len_u8" {
			// Calculate length of remaining operands (8-bit with overflow check)
			length := a.calculateLengthOfOperands(line.Operands[i+1:])
			if length > 255 {
				return fmt.Errorf("%s overflow: length is %d bytes (max 255)", operand, length)
			}
			bytes = append(bytes, byte(length))
		} else if operand == "@len_u16" || operand == "@size16" {
			// Calculate length of remaining operands (16-bit, emit as 2 bytes)
			length := a.calculateLengthOfOperands(line.Operands[i+1:])
			if length > 65535 {
				return fmt.Errorf("%s overflow: length is %d bytes (max 65535)", operand, length)
			}
			// Little-endian: low byte first, then high byte
			bytes = append(bytes, byte(length), byte(length>>8))
		} else if operand == "@count" || operand == "@count_u8" {
			// Count number of remaining operands (8-bit with overflow check)
			count := len(line.Operands) - i - 1
			if count > 255 {
				return fmt.Errorf("%s overflow: count is %d (max 255)", operand, count)
			}
			bytes = append(bytes, byte(count))
		} else if operand == "@count_u16" {
			// Count number of remaining operands (16-bit, emit as 2 bytes)
			count := len(line.Operands) - i - 1
			if count > 65535 {
				return fmt.Errorf("%s overflow: count is %d (max 65535)", operand, count)
			}
			// Little-endian: low byte first, then high byte
			bytes = append(bytes, byte(count), byte(count>>8))
		} else if isString(operand) {
			// Handle string
			str := parseString(operand)
			bytes = append(bytes, []byte(str)...)
		} else {
			// Parse as numeric value
			val, err := a.resolveValue(operand)
			if err != nil {
				return fmt.Errorf("invalid DB operand '%s': %w", operand, err)
			}
			// Accept -128..255 range, truncate to byte via two's complement
			if val > 255 || val < -128 {
				return fmt.Errorf("DB value out of range: %d (must be -128..255)", val)
			}
			bytes = append(bytes, byte(val))
		}
	}
	
	if a.pass >= 2 {
		inst := &AssembledInstruction{
			Address: a.currentAddr,
			Line:    line,
			Bytes:   bytes,
		}
		a.instructions = append(a.instructions, inst)
		a.output = append(a.output, bytes...)
	}
	
	a.currentAddr += len(bytes)
	return nil
}

// handleDW handles word definitions
func (a *Assembler) handleDW(line *Line) error {
	if len(line.Operands) == 0 {
		return fmt.Errorf("DW requires at least one operand")
	}
	
	var bytes []byte
	
	for i, operand := range line.Operands {
		// Check for special macros
		if operand == "@len_u16" || operand == "@size16" {
			// Calculate length of remaining operands (16-bit)
			length := a.calculateLengthOfOperands(line.Operands[i+1:])
			if length > 65535 {
				return fmt.Errorf("%s overflow: length is %d bytes (max 65535)", operand, length)
			}
			// Little-endian encoding
			bytes = append(bytes, byte(length), byte(length>>8))
		} else if operand == "@len" || operand == "@len_u8" {
			// For DW context, promote 8-bit length to 16-bit
			length := a.calculateLengthOfOperands(line.Operands[i+1:])
			if length > 255 {
				return fmt.Errorf("%s overflow: length is %d bytes (max 255 for u8)", operand, length)
			}
			// Little-endian encoding (high byte will be 0)
			bytes = append(bytes, byte(length), 0)
		} else if operand == "@count_u16" {
			// Count number of remaining operands (16-bit)
			count := len(line.Operands) - i - 1
			if count > 65535 {
				return fmt.Errorf("%s overflow: count is %d (max 65535)", operand, count)
			}
			// Little-endian encoding
			bytes = append(bytes, byte(count), byte(count>>8))
		} else if operand == "@count" || operand == "@count_u8" {
			// For DW context, promote 8-bit count to 16-bit
			count := len(line.Operands) - i - 1
			if count > 255 {
				return fmt.Errorf("%s overflow: count is %d (max 255 for u8)", operand, count)
			}
			// Little-endian encoding (high byte will be 0)
			bytes = append(bytes, byte(count), 0)
		} else {
			val, err := a.resolveValue(operand)
			if err != nil {
				return fmt.Errorf("invalid DW operand '%s': %w", operand, err)
			}
			// Little-endian encoding (always 2 bytes — DW is Define Word)
			bytes = append(bytes, byte(val), byte(val>>8))
		}
	}
	
	if a.pass >= 2 {
		inst := &AssembledInstruction{
			Address: a.currentAddr,
			Line:    line,
			Bytes:   bytes,
		}
		a.instructions = append(a.instructions, inst)
		a.output = append(a.output, bytes...)
	}
	
	a.currentAddr += len(bytes)
	return nil
}

// handleASSUME handles .ASSUME directive (eZ80 mode control).
// Syntax: .ASSUME ADL=1 (enable ADL mode) or .ASSUME ADL=0 (Z80 mode)
// Compatible with agon-ez80asm and fasmg.
func (a *Assembler) handleASSUME(line *Line) error {
	if len(line.Operands) == 0 {
		return fmt.Errorf(".ASSUME requires operand (e.g. ADL=1)")
	}
	operand := strings.ToUpper(strings.Join(line.Operands, ""))
	operand = strings.ReplaceAll(operand, " ", "")
	switch operand {
	case "ADL=1":
		a.SetCPUMode(CPUModeEZ80ADL)
	case "ADL=0":
		a.SetCPUMode(CPUModeEZ80Z80)
	default:
		return fmt.Errorf("unknown .ASSUME operand: %s (expected ADL=0 or ADL=1)", operand)
	}
	return nil
}

// handleDW24 handles DW24/DL — 24-bit (3 byte) data definitions.
// Compatible with agon-ez80asm syntax.
func (a *Assembler) handleDW24(line *Line) error {
	if len(line.Operands) == 0 {
		return fmt.Errorf("DW24 requires at least one operand")
	}

	var bytes []byte
	for _, operand := range line.Operands {
		val, err := a.resolveValue(operand)
		if err != nil {
			return fmt.Errorf("invalid DW24 operand '%s': %w", operand, err)
		}
		// 24-bit little-endian encoding
		bytes = append(bytes, byte(val), byte(val>>8), byte(val>>16))
	}

	if a.pass >= 2 {
		inst := &AssembledInstruction{
			Address: a.currentAddr,
			Line:    line,
			Bytes:   bytes,
		}
		a.instructions = append(a.instructions, inst)
		a.output = append(a.output, bytes...)
	}

	a.currentAddr += len(bytes)
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
	
	if a.pass >= 2 {
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
	label := a.symbolKey(line.Label)
	
	if a.pass == 1 {
		if sym, exists := a.symbols[label]; exists && sym.Defined {
			return fmt.Errorf("symbol '%s' already defined", label)
		}

		a.symbols[label] = &Symbol{
			Name:    label,
			Value:   value,
			Defined: true,
		}
	} else {
		// In pass 2+, update the value (may change due to multi-pass convergence)
		if sym, exists := a.symbols[label]; exists {
			sym.Value = value // Update for multi-pass convergence
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
		
		if a.pass >= 2 {
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

// handleEND marks end of assembly and optionally sets entry point
func (a *Assembler) handleEND(line *Line) error {
	// END directive can have an optional entry point label
	// e.g., END main  - sets execution start address to 'main' label
	if len(line.Operands) > 0 {
		entryLabel := line.Operands[0]

		// Resolve the entry point in pass 2 (symbols are defined)
		if a.pass >= 2 {
			entryAddr, err := a.resolveSymbol(entryLabel)
			if err != nil {
				return fmt.Errorf("END: cannot resolve entry point '%s': %w", entryLabel, err)
			}
			a.entryPoint = entryAddr
			a.hasEntryPoint = true
		}
	}
	return nil
}

// handleINCBIN includes a binary file verbatim as data bytes.
// Syntax: INCBIN "filename" [,offset [,length]]
func (a *Assembler) handleINCBIN(line *Line) error {
	if len(line.Operands) < 1 {
		return fmt.Errorf("INCBIN requires a filename")
	}

	// Parse filename (strip quotes)
	filename := line.Operands[0]
	if isString(filename) {
		filename = parseString(filename)
	}

	// Resolve relative to source directory
	if !filepath.IsAbs(filename) && a.sourceDir != "" {
		filename = filepath.Join(a.sourceDir, filename)
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("INCBIN: %w", err)
	}

	// Optional offset
	offset := 0
	if len(line.Operands) >= 2 {
		v, err := a.resolveValue(line.Operands[1])
		if err != nil {
			return fmt.Errorf("INCBIN offset: %w", err)
		}
		offset = v
		if offset < 0 || offset > len(data) {
			return fmt.Errorf("INCBIN offset %d out of range (file is %d bytes)", offset, len(data))
		}
	}

	// Optional length
	length := len(data) - offset
	if len(line.Operands) >= 3 {
		v, err := a.resolveValue(line.Operands[2])
		if err != nil {
			return fmt.Errorf("INCBIN length: %w", err)
		}
		length = v
		if length < 0 || offset+length > len(data) {
			return fmt.Errorf("INCBIN length %d out of range (available: %d bytes from offset %d)", length, len(data)-offset, offset)
		}
	}

	// Emit bytes
	for _, b := range data[offset : offset+length] {
		a.EmitByte(b)
	}
	a.currentAddr += length
	return nil
}

// handleINCLUDE includes another file
// Note: INCLUDE is preprocessed before parsing (see preprocessIncludes in assembler.go).
// This handler should not be reached in normal flow.
func (a *Assembler) handleINCLUDE(line *Line) error {
	return fmt.Errorf("INCLUDE directive should be preprocessed before assembly; use AssembleFile() instead of AssembleString() for INCLUDE support")
}

// handleMACRO begins a macro definition
func (a *Assembler) handleMACRO(line *Line) error {
	if !a.EnableMacros {
		return fmt.Errorf("macros are disabled")
	}
	
	// Parse macro name and parameters
	if len(line.Operands) < 1 {
		return fmt.Errorf("MACRO requires a name")
	}
	
	macroName := line.Operands[0]
	var params []string
	if len(line.Operands) > 1 {
		params = line.Operands[1:]
	}
	
	// Parameters are already parsed into line.Operands
	
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
	if len(s) < 2 {
		return s
	}
	
	// Remove quotes
	s = s[1 : len(s)-1]
	
	// Process escape sequences
	var result []byte
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			// Handle escape sequences
			switch s[i+1] {
			case 'n':
				result = append(result, '\n')
				i++ // Skip the next character
			case 'r':
				result = append(result, '\r')
				i++
			case 't':
				result = append(result, '\t')
				i++
			case '\\':
				result = append(result, '\\')
				i++
			case '"':
				result = append(result, '"')
				i++
			case '\'':
				result = append(result, '\'')
				i++
			case '0':
				result = append(result, 0)
				i++
			default:
				// Unknown escape, keep the backslash
				result = append(result, s[i])
			}
		} else {
			result = append(result, s[i])
		}
	}
	
	return string(result)
}

// handleTARGET sets the target platform
func (a *Assembler) handleTARGET(line *Line) error {
	if len(line.Operands) != 1 {
		return fmt.Errorf("TARGET requires exactly one operand")
	}
	
	targetStr := strings.Trim(line.Operands[0], "\"'")
	target, err := ParseTarget(targetStr)
	if err != nil {
		// Provide helpful error with available targets
		return fmt.Errorf("unknown target '%s'. Available targets: generic, zxspectrum, cpm, msx, gameboy", targetStr)
	}
	
	// Set the target configuration
	err = a.SetTarget(target)
	if err != nil {
		return fmt.Errorf("failed to set target: %w", err)
	}
	
	// Log the target selection
	if a.pass == 1 {
		a.warnings = append(a.warnings, fmt.Sprintf("Target platform set to: %s", a.target.Name))
	}
	
	return nil
}

// handleMODEL sets the specific model of the target platform
func (a *Assembler) handleMODEL(line *Line) error {
	if len(line.Operands) != 1 {
		return fmt.Errorf("MODEL requires exactly one operand")
	}
	
	if a.target == nil {
		return fmt.Errorf("MODEL directive requires TARGET to be set first")
	}
	
	model := strings.Trim(line.Operands[0], "\"'")
	
	// Store model in target extensions
	if a.target.Extensions == nil {
		a.target.Extensions = make(map[string]interface{})
	}
	a.target.Extensions["model"] = model
	
	// Apply model-specific configurations
	switch a.target.Name {
	case "ZX Spectrum":
		switch strings.ToLower(model) {
		case "48k":
			// 48K specific settings
			a.target.MemoryLayout.RAMSize = 49152
		case "128k":
			// 128K specific settings (bank switching required)
			// Note: Only 64K addressable at once without banking
			a.target.MemoryLayout.RAMSize = 65535 // Max addressable
			a.target.Extensions["banks"] = 8 // 8 x 16K banks
			a.warnings = append(a.warnings, "128K model set - bank switching support limited")
		case "+2", "+3":
			// +2/+3 specific settings
			a.target.MemoryLayout.RAMSize = 65535 // Max addressable
			a.target.Extensions["banks"] = 8 // 8 x 16K banks
			a.warnings = append(a.warnings, fmt.Sprintf("%s model set - disk support not yet implemented", model))
		default:
			return fmt.Errorf("unknown ZX Spectrum model '%s'. Valid models: 48k, 128k, +2, +3", model)
		}
	case "MSX":
		switch strings.ToLower(model) {
		case "msx1":
			a.target.MemoryLayout.RAMSize = 32768
		case "msx2", "msx2+":
			a.target.MemoryLayout.RAMSize = 65535 // Max addressable
			a.target.Extensions["vram"] = 131072 // 128K VRAM
		default:
			return fmt.Errorf("unknown MSX model '%s'. Valid models: msx1, msx2, msx2+", model)
		}
	case "CP/M":
		// CP/M doesn't really have models, but we can accept version numbers
		switch model {
		case "2.2", "3.0":
			// Version-specific settings could go here
		default:
			a.warnings = append(a.warnings, fmt.Sprintf("CP/M version '%s' noted", model))
		}
	}
	
	if a.pass == 1 {
		a.warnings = append(a.warnings, fmt.Sprintf("Model set to: %s", model))
	}
	
	return nil
}