package z80asm

import (
	"fmt"
	"os"
	"strings"
)

// Assembler is the main Z80 assembler
type Assembler struct {
	// Configuration options
	AllowUndocumented bool // Default: true
	Strict            bool // Sjasmplus compatibility mode
	CaseSensitive     bool // Case sensitivity for labels
	EnableMacros      bool // Enable macro processing
	
	// Internal state
	pass          int
	currentAddr   uint16
	origin        uint16
	symbols       map[string]*Symbol
	lines         []*Line
	output        []byte
	instructions  []*AssembledInstruction
	errors        []AssemblerError
	warnings      []string
	macroProcessor *MacroProcessor
	macroDefinition *macroDefinitionState // Current macro being defined
	
	// Target platform support
	target        *TargetConfig
}

// macroDefinitionState tracks a macro being defined
type macroDefinitionState struct {
	name   string
	params []string
	body   []string
}

// AssemblerError represents an assembly error
type AssemblerError struct {
	Line        int
	Column      int
	Message     string
	// Enhanced error context (optional - maintains backward compatibility)
	Context     string      // Problematic text
	Suggestion  string      // How to fix
	Examples    []string    // Valid alternatives
}

func (e AssemblerError) Error() string {
	// Backward compatible simple format
	if e.Suggestion == "" && len(e.Examples) == 0 {
		return fmt.Sprintf("line %d: %s", e.Line, e.Message)
	}
	
	// Enhanced format with suggestions
	return e.FormatEnhanced()
}

// FormatEnhanced returns detailed error with suggestions
func (e AssemblerError) FormatEnhanced() string {
	var buf strings.Builder
	
	// Main error message
	fmt.Fprintf(&buf, "Line %d: %s", e.Line, e.Message)
	
	// Context highlighting
	if e.Context != "" {
		fmt.Fprintf(&buf, "\n  Problem: '%s'", e.Context)
	}
	
	// Suggestion with visual indicator
	if e.Suggestion != "" {
		fmt.Fprintf(&buf, "\n  💡 %s", e.Suggestion)
	}
	
	// Examples for guidance
	if len(e.Examples) > 0 {
		fmt.Fprintf(&buf, "\n  Examples:")
		for _, example := range e.Examples {
			fmt.Fprintf(&buf, "\n    • %s", example)
		}
	}
	
	return buf.String()
}

// Helper functions for creating enhanced errors

// NewUndefinedSymbolError creates a contextual undefined symbol error
func NewUndefinedSymbolError(line int, symbol string) AssemblerError {
	var suggestion string
	var examples []string
	
	// Pattern-based suggestions
	switch {
	case isRegisterIndirect(symbol): // (HL), (BC), etc.
		suggestion = "Register indirect addressing may not be supported for this instruction"
		examples = []string{
			"Check instruction syntax: LD A, (HL) vs LD A, $12",
			"Verify register indirect is supported for this operation",
			"Try immediate addressing if applicable",
		}
		
	case isMemoryIndirect(symbol): // ($8100), etc.
		suggestion = "Memory indirect addressing requires proper format and instruction support"
		examples = []string{
			"Ensure hex format: LD HL, ($8000)",
			"Check if instruction supports memory indirect",
			"Consider using defined symbols: LD A, (buffer_addr)",
		}
		
	case isLikelyHexAddress(symbol):
		suggestion = "Address may need parentheses for indirect access or label definition"
		examples = []string{
			"For memory access: LD A, ($8000)",
			"For immediate value: LD HL, $8000",
			"Define as label: " + symbol + "_addr:",
		}
		
	default:
		suggestion = "Symbol not defined in current scope"
		examples = []string{
			"Define symbol with: " + symbol + ":",
			"Check for typos in symbol name",
			"Verify symbol is defined before use",
		}
	}
	
	return AssemblerError{
		Line:        line,
		Message:     fmt.Sprintf("undefined symbol: %s", symbol),
		Context:     symbol,
		Suggestion:  suggestion,
		Examples:    examples,
	}
}

// NewUnsupportedInstructionError creates error for unsupported instruction patterns
func NewUnsupportedInstructionError(line int, mnemonic string, operands []string) AssemblerError {
	opStr := strings.Join(operands, ", ")
	instruction := fmt.Sprintf("%s %s", mnemonic, opStr)
	
	var suggestion string
	var examples []string
	
	switch strings.ToUpper(mnemonic) {
	case "LD":
		suggestion = "This LD instruction pattern is not yet supported"
		examples = []string{
			"Check Z80 instruction reference for valid LD patterns",
			"Try alternative addressing mode if available",
			"Consider breaking into multiple simpler instructions",
		}
	default:
		suggestion = fmt.Sprintf("%s instruction pattern not supported", mnemonic)
		examples = []string{
			"Check if instruction is standard Z80",
			"Verify operand types match instruction requirements",
		}
	}
	
	return AssemblerError{
		Line:        line,
		Message:     fmt.Sprintf("unsupported instruction: %s", instruction),
		Context:     instruction,
		Suggestion:  suggestion,
		Examples:    examples,
	}
}

// Pattern recognition helpers
func isRegisterIndirect(symbol string) bool {
	if !strings.HasPrefix(symbol, "(") || !strings.HasSuffix(symbol, ")") {
		return false
	}
	inner := strings.TrimSpace(symbol[1:len(symbol)-1])
	upper := strings.ToUpper(inner)
	return upper == "HL" || upper == "BC" || upper == "DE" || upper == "SP" ||
		   strings.Contains(upper, "IX") || strings.Contains(upper, "IY")
}

func isMemoryIndirect(symbol string) bool {
	if !strings.HasPrefix(symbol, "(") || !strings.HasSuffix(symbol, ")") {
		return false
	}
	if isRegisterIndirect(symbol) {
		return false
	}
	inner := strings.TrimSpace(symbol[1:len(symbol)-1])
	return strings.HasPrefix(inner, "$") || strings.HasPrefix(inner, "0x") || isAllDigits(inner)
}

func isLikelyHexAddress(symbol string) bool {
	return strings.HasPrefix(symbol, "$") || strings.HasPrefix(symbol, "0x")
}

func isAllDigits(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

// Result contains the assembled output
type Result struct {
	Binary      []byte
	Origin      uint16
	Size        uint16
	Symbols     map[string]uint16
	Listing     []ListingLine
	Errors      []AssemblerError
	Warnings    []string
}

// ListingLine represents a line in the assembly listing
type ListingLine struct {
	Address     uint16
	Bytes       []byte
	LineNumber  int
	SourceLine  string
	Label       string
}

// AssembledInstruction represents a fully assembled instruction
type AssembledInstruction struct {
	Address     uint16
	Line        *Line
	Bytes       []byte
	Fixups      []Fixup
}

// Fixup represents a forward reference that needs fixing
type Fixup struct {
	Offset      int    // Offset in instruction bytes
	Symbol      string // Symbol to resolve
	Type        FixupType
	Expression  string // For complex expressions
}

// FixupType indicates how to apply the fixup
type FixupType int

const (
	FixupByte FixupType = iota   // 8-bit value
	FixupWord                     // 16-bit value (little-endian)
	FixupRelative                 // Relative jump offset
)

// NewAssembler creates a new assembler instance
func NewAssembler() *Assembler {
	a := &Assembler{
		AllowUndocumented: true,
		Strict:            false,
		CaseSensitive:     false,
		EnableMacros:      true,
		symbols:           make(map[string]*Symbol),
		origin:            0x8000, // Default origin
		macroProcessor:    NewMacroProcessor(),
	}
	
	// Define standard macros
	if a.EnableMacros {
		a.macroProcessor.DefineStandardMacros()
	}
	
	return a
}

// AssembleFile assembles a source file
func (a *Assembler) AssembleFile(filename string) (*Result, error) {
	// Read file
	source, err := ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	
	return a.AssembleString(source)
}

// AssembleString assembles source code from a string
func (a *Assembler) AssembleString(source string) (*Result, error) {
	// Reset state
	a.reset()
	
	// Parse source into lines
	lines, err := ParseSource(source)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	
	// Preprocess local labels (expand .loop to main.loop)
	lines, err = preprocessLocalLabels(lines)
	if err != nil {
		return nil, fmt.Errorf("local label error: %w", err)
	}
	
	// Expand multi-argument instructions (PUSH AF, BC, DE -> multiple PUSHes)
	lines = expandMultiArgInstructions(lines)
	
	// Expand fake instructions (LD HL, DE -> LD H, D : LD L, E)
	lines = expandFakeInstructions(lines)
	
	a.lines = lines
	
	// Pass 1: Build symbol table and calculate addresses
	a.pass = 1
	if err := a.performPass(); err != nil {
		return nil, fmt.Errorf("pass 1 error: %w", err)
	}
	
	// Pass 2: Generate code
	a.pass = 2
	a.currentAddr = a.origin
	a.output = make([]byte, 0, 65536)
	a.instructions = make([]*AssembledInstruction, 0)
	
	if err := a.performPass(); err != nil {
		return nil, fmt.Errorf("pass 2 error: %w", err)
	}
	
	// Validate memory layout for target platform
	if err := a.ValidateMemoryLayout(); err != nil {
		return nil, fmt.Errorf("memory layout error: %w", err)
	}
	
	// Build result
	result := &Result{
		Binary:  a.output,
		Origin:  a.origin,
		Size:    uint16(len(a.output)),
		Symbols: make(map[string]uint16),
		Listing: make([]ListingLine, 0),
		Errors:  a.errors,
	}
	
	// Copy symbols
	for name, sym := range a.symbols {
		if sym.Defined {
			result.Symbols[name] = sym.Value
		}
	}
	
	// Generate listing
	for _, inst := range a.instructions {
		listing := ListingLine{
			Address:    inst.Address,
			Bytes:      inst.Bytes,
			LineNumber: inst.Line.Number,
			SourceLine: formatSourceLine(inst.Line),
			Label:      inst.Line.Label,
		}
		result.Listing = append(result.Listing, listing)
	}
	
	return result, nil
}

// reset clears assembler state
func (a *Assembler) reset() {
	a.pass = 0
	a.currentAddr = a.origin
	
	// Preserve target symbols if target is set
	targetSymbols := make(map[string]*Symbol)
	if a.target != nil {
		for symbol, addr := range a.target.Conventions.CommonSymbols {
			symbolName := symbol
			if !a.CaseSensitive {
				symbolName = strings.ToUpper(symbol)
			}
			targetSymbols[symbolName] = &Symbol{
				Name:    symbolName,
				Value:   addr,
				Defined: true,
			}
		}
	}
	
	a.symbols = targetSymbols
	a.output = nil
	a.instructions = nil
	a.errors = nil
	a.warnings = nil
}

// performPass executes one assembly pass
func (a *Assembler) performPass() error {
	a.currentAddr = a.origin
	
	for _, line := range a.lines {
		if err := a.processLine(line); err != nil {
			// Create enhanced error based on error type
			var assemblyError AssemblerError
			
			errMsg := err.Error()
			if strings.Contains(errMsg, "undefined symbol:") {
				// Extract symbol name from error message
				parts := strings.Split(errMsg, "undefined symbol: ")
				if len(parts) > 1 {
					symbol := strings.TrimSpace(parts[1])
					assemblyError = NewUndefinedSymbolError(line.Number, symbol)
				} else {
					// Fallback to basic error
					assemblyError = AssemblerError{
						Line:    line.Number,
						Message: errMsg,
					}
				}
			} else if strings.Contains(errMsg, "unsupported") && line.Mnemonic != "" {
				// Create enhanced unsupported instruction error
				assemblyError = NewUnsupportedInstructionError(line.Number, line.Mnemonic, line.Operands)
			} else {
				// Default error format
				assemblyError = AssemblerError{
					Line:    line.Number,
					Message: errMsg,
				}
			}
			
			a.errors = append(a.errors, assemblyError)
			if a.Strict {
				return err
			}
		}
	}
	
	return nil
}

// processLine processes a single line
func (a *Assembler) processLine(line *Line) error {
	// Skip blank lines
	if line.IsBlank {
		return nil
	}
	
	// Handle label
	if line.Label != "" {
		if err := a.defineLabel(line.Label); err != nil {
			return err
		}
	}
	
	// Handle directive
	if line.Directive != "" {
		return a.processDirective(line)
	}
	
	// Handle instruction
	if line.Mnemonic != "" {
		return a.processInstruction(line)
	}
	
	return nil
}

// defineLabel defines a label at the current address
func (a *Assembler) defineLabel(label string) error {
	if !a.CaseSensitive {
		label = strings.ToUpper(label)
	}
	
	if a.pass == 1 {
		// Check for redefinition
		if sym, exists := a.symbols[label]; exists && sym.Defined {
			return fmt.Errorf("label '%s' already defined", label)
		}
		
		a.symbols[label] = &Symbol{
			Name:    label,
			Value:   a.currentAddr,
			Defined: true,
		}
	}
	
	return nil
}

// resolveSymbol resolves a symbol to its value
func (a *Assembler) resolveSymbol(name string) (uint16, error) {
	if !a.CaseSensitive {
		name = strings.ToUpper(name)
	}
	
	if sym, exists := a.symbols[name]; exists && sym.Defined {
		return sym.Value, nil
	}
	
	// Try to parse as number
	if val, err := parseNumber(name); err == nil {
		return val, nil
	}
	
	if a.pass == 1 {
		// Create forward reference
		a.symbols[name] = &Symbol{
			Name:    name,
			Defined: false,
		}
		return 0, nil
	}
	
	return 0, fmt.Errorf("undefined symbol: %s", name)
}

// formatSourceLine formats a line for listing output
func formatSourceLine(line *Line) string {
	var parts []string
	
	if line.Label != "" {
		parts = append(parts, line.Label+":")
	}
	
	if line.Directive != "" {
		parts = append(parts, line.Directive)
		if len(line.Operands) > 0 {
			parts = append(parts, strings.Join(line.Operands, ", "))
		}
	} else if line.Mnemonic != "" {
		parts = append(parts, line.Mnemonic)
		if len(line.Operands) > 0 {
			parts = append(parts, strings.Join(line.Operands, ", "))
		}
	}
	
	result := strings.Join(parts, " ")
	if line.Comment != "" {
		result += " ; " + line.Comment
	}
	
	return result
}

// ReadFile reads a source file
func ReadFile(filename string) (string, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %v", filename, err)
	}
	return string(content), nil
}

// EmitByte emits a byte to the output in pass 2
func (a *Assembler) EmitByte(b byte) {
	if a.pass == 2 {
		a.output = append(a.output, b)
	}
}

// EmitWord emits a word (little-endian) to the output in pass 2
func (a *Assembler) EmitWord(w uint16) {
	if a.pass == 2 {
		a.output = append(a.output, byte(w), byte(w>>8))
	}
}