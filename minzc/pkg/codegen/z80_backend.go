package codegen

import (
	"bytes"
	"github.com/minz/minzc/pkg/ir"
)

// Z80Backend implements the Backend interface for Z80 code generation
type Z80Backend struct {
	options *BackendOptions
}

// NewZ80Backend creates a new Z80 backend
func NewZ80Backend(options *BackendOptions) Backend {
	return &Z80Backend{
		options: options,
	}
}

// Name returns the name of this backend
func (b *Z80Backend) Name() string {
	return "z80"
}

// Generate generates Z80 assembly code for the given IR module
func (b *Z80Backend) Generate(module *ir.Module) (string, error) {
	// Create a buffer to collect the generated code
	var buf bytes.Buffer
	
	// Create the Z80 generator with the buffer
	gen := NewZ80Generator(&buf)
	
	// Configure based on options
	if b.options != nil {
		if b.options.EnableSMC {
			// Enable SMC for all functions
			for _, fn := range module.Functions {
				fn.IsSMCEnabled = true
			}
		}
		
		// Set target address if specified
		if b.options.TargetAddress != 0 {
			// TODO: Add support for custom origin address in Z80Generator
			// For now, we'll add it as a comment
		}
	}
	
	// Generate the code
	if err := gen.Generate(module); err != nil {
		return "", err
	}
	
	return buf.String(), nil
}

// GetFileExtension returns the file extension for Z80 assembly
func (b *Z80Backend) GetFileExtension() string {
	return ".a80"
}

// SupportsFeature checks if the Z80 backend supports a specific feature
func (b *Z80Backend) SupportsFeature(feature string) bool {
	switch feature {
	case FeatureSelfModifyingCode:
		return true
	case FeatureInterrupts:
		return true
	case FeatureShadowRegisters:
		return true
	case Feature16BitPointers:
		return true
	case Feature24BitPointers:
		return false // Z80 doesn't have native 24-bit support
	case FeatureFloatingPoint:
		return false // No hardware floating point
	case FeatureFixedPoint:
		return true // We support fixed-point arithmetic
	default:
		return false
	}
}

// Register the Z80 backend
func init() {
	RegisterBackend("z80", func(options *BackendOptions) Backend {
		return NewZ80Backend(options)
	})
}