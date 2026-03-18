package codegen

import (
	"bytes"
	"github.com/minz/minzc/pkg/ir"
)

// I8080Backend implements the Backend interface for Intel 8080 code generation
// The 8080 is essentially Z80 without:
// - IX/IY index registers
// - Shadow registers (no EXX, EX AF,AF')
// - DJNZ instruction
// - JR (relative jump) instructions
// - Bit manipulation instructions (SET, RES, BIT)
// - Block instructions (LDIR, CPIR, etc.)
type I8080Backend struct {
	options *BackendOptions
}

// NewI8080Backend creates a new Intel 8080 backend
func NewI8080Backend(options *BackendOptions) Backend {
	return &I8080Backend{
		options: options,
	}
}

// Name returns the name of this backend
func (b *I8080Backend) Name() string {
	return "i8080"
}

// Generate generates 8080 assembly code for the given IR module
func (b *I8080Backend) Generate(module *ir.Module) (string, error) {
	// Create a buffer to collect the generated code
	var buf bytes.Buffer
	
	// Create the 8080 generator with the buffer
	gen := NewI8080Generator(&buf)
	
	// Configure based on options
	if b.options != nil {
		if b.options.EnableSMC {
			// Enable SMC for all functions
			for _, fn := range module.Functions {
				fn.IsSMCEnabled = true
			}
		}
	}
	
	// Generate the code
	if err := gen.Generate(module); err != nil {
		return "", err
	}
	
	return buf.String(), nil
}

// GetFileExtension returns the file extension for 8080 assembly
func (b *I8080Backend) GetFileExtension() string {
	return ".a80"  // Same as Z80 for compatibility
}

// SupportsFeature checks if the 8080 backend supports a specific feature
func (b *I8080Backend) SupportsFeature(feature string) bool {
	switch feature {
	case FeatureSelfModifyingCode:
		return true  // 8080 supports SMC!
	case FeatureInterrupts:
		return true
	case FeatureShadowRegisters:
		return false // No shadow registers on 8080
	case Feature16BitPointers:
		return true
	case Feature24BitPointers:
		return false // 8080 is strictly 16-bit addressing
	case Feature32BitPointers:
		return false
	case FeatureFloatingPoint:
		return false // No hardware FP
	case FeatureFixedPoint:
		return true  // Can implement in software
	case "indexed_addressing":
		return false // No IX/IY registers
	case "relative_jumps":
		return false // No JR instructions
	case "bit_operations":
		return false // No BIT/SET/RES instructions
	case "block_operations":
		return false // No LDIR/CPIR etc
	case "djnz":
		return false // No DJNZ instruction
	default:
		return false
	}
}

// Register the 8080 backend
func init() {
	RegisterBackend("i8080", func(options *BackendOptions) Backend {
		return NewI8080Backend(options)
	})
	
	// Also register common aliases
	RegisterBackend("8080", func(options *BackendOptions) Backend {
		return NewI8080Backend(options)
	})
	RegisterBackend("intel8080", func(options *BackendOptions) Backend {
		return NewI8080Backend(options)
	})
}