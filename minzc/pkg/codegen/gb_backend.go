package codegen

import (
	"bytes"
	"github.com/minz/minzc/pkg/ir"
)

// GBBackend implements the Backend interface for Game Boy (LR35902) code generation
type GBBackend struct {
	BaseBackend
}

// NewGBBackend creates a new Game Boy backend
func NewGBBackend(options *BackendOptions) Backend {
	backend := &GBBackend{
		BaseBackend: NewBaseBackend(options),
	}
	
	// Configure GB-specific features
	backend.SetFeature(FeatureSelfModifyingCode, true)
	backend.SetFeature(FeatureInterrupts, true)
	backend.SetFeature(FeatureShadowRegisters, false)  // No shadow registers on GB!
	backend.SetFeature(Feature16BitPointers, true)
	backend.SetFeature(Feature24BitPointers, false)
	backend.SetFeature(FeatureFloatingPoint, false)
	backend.SetFeature(FeatureFixedPoint, true)
	backend.SetFeature(FeatureInlineAssembly, true)
	backend.SetFeature(FeatureBitManipulation, true)
	backend.SetFeature(FeatureZeroPage, false)
	backend.SetFeature(FeatureBlockInstructions, false)  // No LDIR/CPIR on GB
	
	return backend
}

// Name returns the name of this backend
func (b *GBBackend) Name() string {
	return "gb"
}

// Generate generates Game Boy assembly code for the given IR module
func (b *GBBackend) Generate(module *ir.Module) (string, error) {
	// Validate options
	if err := b.ValidateOptions(); err != nil {
		return "", err
	}
	
	// Preprocess module based on backend capabilities
	if err := b.PreprocessModule(module); err != nil {
		return "", err
	}
	
	// Create a buffer to collect the generated code
	var buf bytes.Buffer
	
	// Create the GB generator with the buffer
	gen := NewGBGenerator(&buf)
	
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
	// Use the BaseBackend feature check
	return b.CheckFeature(feature)
}

// Register the GB backend
func init() {
	RegisterBackend("gb", func(options *BackendOptions) Backend {
		return NewGBBackend(options)
	})
}