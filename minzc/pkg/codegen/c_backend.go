package codegen

import (
	"bytes"
	"github.com/minz/minzc/pkg/ir"
	"time"
)

// CBackend implements the Backend interface for C code generation
type CBackend struct {
	options *BackendOptions
}

// NewCBackend creates a new C backend
func NewCBackend(options *BackendOptions) Backend {
	return &CBackend{
		options: options,
	}
}

// Name returns the name of this backend
func (b *CBackend) Name() string {
	return "c"
}

// Generate generates C code for the given IR module
func (b *CBackend) Generate(module *ir.Module) (string, error) {
	// C doesn't support SMC - use standard calling conventions
	for _, fn := range module.Functions {
		fn.IsSMCEnabled = false
	}
	
	var buf bytes.Buffer
	gen := &CGenerator{
		backend: b,
		module:  module,
		output:  &buf,
		indent:  0,
		varTypes: make(map[string]string),
		timestamp: time.Now().Format("2006-01-02 15:04:05"),
	}
	
	if err := gen.Generate(); err != nil {
		return "", err
	}
	
	return buf.String(), nil
}

// GetFileExtension returns the file extension for C code
func (b *CBackend) GetFileExtension() string {
	return ".c"
}

// SupportsFeature checks if this backend supports a specific feature
func (b *CBackend) SupportsFeature(feature string) bool {
	switch feature {
	case FeatureSelfModifyingCode:
		return false // C doesn't support SMC
	case FeatureShadowRegisters:
		return false // No hardware registers
	case Feature16BitPointers:
		return true
	case Feature24BitPointers:
		return true
	case Feature32BitPointers:
		return true
	case FeatureFloatingPoint:
		return true
	case FeatureFixedPoint:
		return true // We can emulate fixed-point
	default:
		return false
	}
}

// Register the C backend
func init() {
	RegisterBackend("c", func(options *BackendOptions) Backend {
		return NewCBackend(options)
	})
}