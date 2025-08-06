package codegen

import (
	"github.com/minz/minzc/pkg/ir"
)

// Backend defines the interface for code generation backends
type Backend interface {
	// Name returns the name of this backend (e.g., "z80", "6502", "wasm")
	Name() string
	
	// Generate generates code for the given IR module
	Generate(module *ir.Module) (string, error)
	
	// GetFileExtension returns the file extension for generated code
	GetFileExtension() string
	
	// SupportsFeature checks if this backend supports a specific feature
	SupportsFeature(feature string) bool
}

// BackendOptions contains options that can be passed to backends
type BackendOptions struct {
	// OptimizationLevel controls optimization (0 = none, 1 = basic, 2 = aggressive)
	OptimizationLevel int
	
	// EnableSMC enables self-modifying code optimizations (Z80 specific)
	EnableSMC bool
	
	// EnableTrueSMC enables TRUE SMC optimizations (Z80 specific)
	EnableTrueSMC bool
	
	// TargetAddress is the origin address for code
	TargetAddress uint16
	
	// Debug enables debug output
	Debug bool
	
	// Custom backend-specific options
	CustomOptions map[string]interface{}
}

// Common backend features
const (
	FeatureSelfModifyingCode = "smc"
	FeatureInterrupts        = "interrupts"
	FeatureShadowRegisters   = "shadow_registers"
	Feature16BitPointers     = "16bit_pointers"
	Feature24BitPointers     = "24bit_pointers"
	Feature32BitPointers     = "32bit_pointers"
	FeatureFloatingPoint     = "floating_point"
	FeatureFixedPoint        = "fixed_point"
	FeatureInlineAssembly    = "inline_assembly"
	FeatureIndirectCalls     = "indirect_calls"
	FeatureBitManipulation   = "bit_manipulation"
	FeatureZeroPage          = "zero_page"
	FeatureBlockInstructions = "block_instructions"
	FeatureHardwareMultiply  = "hardware_multiply"
	FeatureHardwareDivide    = "hardware_divide"
)

// BackendFactory creates a backend instance
type BackendFactory func(options *BackendOptions) Backend

// Registry of available backends
var backends = make(map[string]BackendFactory)

// RegisterBackend registers a new backend
func RegisterBackend(name string, factory BackendFactory) {
	backends[name] = factory
}

// GetBackend returns a backend by name
func GetBackend(name string, options *BackendOptions) Backend {
	if factory, ok := backends[name]; ok {
		return factory(options)
	}
	return nil
}

// ListBackends returns the names of all registered backends
func ListBackends() []string {
	names := make([]string, 0, len(backends))
	for name := range backends {
		names = append(names, name)
	}
	return names
}