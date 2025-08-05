package codegen

import (
	"bytes"
	"github.com/minz/minzc/pkg/ir"
)

// GBBackend implements the Backend interface for Game Boy (LR35902) code generation
type GBBackend struct {
	options *BackendOptions
}

// NewGBBackend creates a new Game Boy backend
func NewGBBackend(options *BackendOptions) Backend {
	return &GBBackend{
		options: options,
	}
}

// Name returns the name of this backend
func (b *GBBackend) Name() string {
	return "gb"
}

// Generate generates Game Boy assembly code for the given IR module
func (b *GBBackend) Generate(module *ir.Module) (string, error) {
	// Create a buffer to collect the generated code
	var buf bytes.Buffer
	
	// Create the GB generator with the buffer
	gen := NewGBGenerator(&buf)
	
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
			// Game Boy typically starts at $0150 (after header)
			// TODO: Add support for custom origin address in GBGenerator
		}
	}
	
	// Generate the code
	if err := gen.Generate(module); err != nil {
		return "", err
	}
	
	return buf.String(), nil
}

// GetFileExtension returns the file extension for Game Boy assembly
func (b *GBBackend) GetFileExtension() string {
	return ".gb.s"  // Use .gb.s to distinguish from regular assembly
}

// SupportsFeature checks if the GB backend supports a specific feature
func (b *GBBackend) SupportsFeature(feature string) bool {
	switch feature {
	case FeatureSelfModifyingCode:
		return true  // GB supports SMC
	case FeatureInterrupts:
		return true  // Different interrupt system than Z80
	case FeatureShadowRegisters:
		return false // No shadow registers on GB!
	case Feature16BitPointers:
		return true
	case Feature24BitPointers:
		return false
	case FeatureFloatingPoint:
		return false
	case FeatureFixedPoint:
		return true
	case "indexed_addressing":
		return false // No IX/IY registers
	case "gb_specific":
		return true  // For GB-specific features
	default:
		return false
	}
}

// Register the GB backend
func init() {
	RegisterBackend("gb", func(options *BackendOptions) Backend {
		return NewGBBackend(options)
	})
}