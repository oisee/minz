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
	
	// Internal state
	pass          int
	currentAddr   uint16
	origin        uint16
	symbols       map[string]*Symbol
	lines         []*Line
	output        []byte
	instructions  []*AssembledInstruction
	errors        []AssemblerError
}

// AssemblerError represents an assembly error
type AssemblerError struct {
	Line    int
	Column  int
	Message string
}

func (e AssemblerError) Error() string {
	return fmt.Sprintf("line %d: %s", e.Line, e.Message)
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
	return &Assembler{
		AllowUndocumented: true,
		Strict:            false,
		CaseSensitive:     false,
		symbols:           make(map[string]*Symbol),
		origin:            0x8000, // Default origin
	}
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
	a.symbols = make(map[string]*Symbol)
	a.output = nil
	a.instructions = nil
	a.errors = nil
}

// performPass executes one assembly pass
func (a *Assembler) performPass() error {
	a.currentAddr = a.origin
	
	for _, line := range a.lines {
		if err := a.processLine(line); err != nil {
			a.errors = append(a.errors, AssemblerError{
				Line:    line.Number,
				Message: err.Error(),
			})
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