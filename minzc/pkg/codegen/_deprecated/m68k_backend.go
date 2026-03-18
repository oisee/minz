package codegen

import (
	"bytes"
	"github.com/minz/minzc/pkg/ir"
)

// M68kBackend implements the Backend interface for Motorola 68000 code generation
type M68kBackend struct {
	options *BackendOptions
}

// NewM68kBackend creates a new 68000 backend
func NewM68kBackend(options *BackendOptions) Backend {
	return &M68kBackend{
		options: options,
	}
}

// Name returns the name of this backend
func (b *M68kBackend) Name() string {
	return "m68k"
}

// Generate generates 68000 assembly code for the given IR module
func (b *M68kBackend) Generate(module *ir.Module) (string, error) {
	// Create a buffer to collect the generated code
	var buf bytes.Buffer
	
	// Create the 68k generator with the buffer
	gen := NewM68kGenerator(&buf)
	
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

// GetFileExtension returns the file extension for 68000 assembly
func (b *M68kBackend) GetFileExtension() string {
	return ".s"  // Standard assembly extension
}

// SupportsFeature checks if the 68000 backend supports a specific feature
func (b *M68kBackend) SupportsFeature(feature string) bool {
	switch feature {
	case FeatureSelfModifyingCode:
		return true  // 68000 supports SMC perfectly!
	case FeatureInterrupts:
		return true
	case FeatureShadowRegisters:
		return false // No shadow registers like Z80
	case Feature16BitPointers:
		return true  // Can use 16-bit addressing modes
	case Feature24BitPointers:
		return true  // 68000 has native 24-bit addressing!
	case Feature32BitPointers:
		return true  // Full 32-bit registers
	case FeatureFloatingPoint:
		return false // Base 68000 has no FPU (68881/68882 needed)
	case FeatureFixedPoint:
		return true  // We can implement in software
	case "32bit_registers":
		return true  // D0-D7, A0-A7 are all 32-bit!
	case "orthogonal_instruction_set":
		return true  // Very clean, orthogonal design
	default:
		return false
	}
}

// Register the 68000 backend
func init() {
	RegisterBackend("m68k", func(options *BackendOptions) Backend {
		return NewM68kBackend(options)
	})
	
	// Also register common aliases
	RegisterBackend("68000", func(options *BackendOptions) Backend {
		return NewM68kBackend(options)
	})
	RegisterBackend("68k", func(options *BackendOptions) Backend {
		return NewM68kBackend(options)
	})
}