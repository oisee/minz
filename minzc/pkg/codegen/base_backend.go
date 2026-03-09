package codegen

import (
	"fmt"
	"github.com/minz/minzc/pkg/ir"
)

// BaseBackend provides common functionality for all backends
type BaseBackend struct {
	options  *BackendOptions
	features map[string]bool
}

// NewBaseBackend creates a new base backend with default features
func NewBaseBackend(options *BackendOptions) BaseBackend {
	return BaseBackend{
		options: options,
		features: map[string]bool{
			FeatureSelfModifyingCode: false,
			FeatureInterrupts:        false,
			FeatureShadowRegisters:   false,
			Feature16BitPointers:     true,
			Feature24BitPointers:     false,
			Feature32BitPointers:     false,
			FeatureFloatingPoint:     false,
			FeatureFixedPoint:        false,
			FeatureIndirectCalls:     true,
			FeatureBitManipulation:   true,
		},
	}
}

// ValidateOptions checks if the requested options are supported
func (b *BaseBackend) ValidateOptions() error {
	if b.options == nil {
		return nil
	}
	
	// Check SMC support
	if b.options.EnableSMC && !b.features[FeatureSelfModifyingCode] {
		return fmt.Errorf("this backend does not support self-modifying code")
	}
	
	if b.options.EnableTrueSMC && !b.features[FeatureSelfModifyingCode] {
		return fmt.Errorf("this backend does not support TRUE self-modifying code")
	}
	
	return nil
}

// PreprocessModule applies backend-specific preprocessing to the module
func (b *BaseBackend) PreprocessModule(module *ir.Module) error {
	// Disable SMC if not supported
	if !b.features[FeatureSelfModifyingCode] {
		for _, fn := range module.Functions {
			fn.IsSMCEnabled = false
			fn.IsSMCDefault = false
			fn.UsesTrueSMC = false
		}
	}
	
	// Apply SMC options if supported.
	// Only enable SMC for functions that actually need it:
	//   UsesTrueSMC=true  → anchor-based patching (TRUE SMC, explicit in source)
	//   IsSMCDefault=true → function was marked for SMC convention by the analyzer
	// Normal functions (register/stack convention) must NOT be forced into the SMC
	// codegen path — they use generateFunction() + physical register allocation.
	if b.options != nil && b.options.EnableSMC && b.features[FeatureSelfModifyingCode] {
		for _, fn := range module.Functions {
			if fn.UsesTrueSMC || fn.IsSMCDefault {
				fn.IsSMCEnabled = true
			}
		}
	}
	
	return nil
}

// GetOptions returns the backend options
func (b *BaseBackend) GetOptions() *BackendOptions {
	return b.options
}

// SetFeature sets a feature support flag
func (b *BaseBackend) SetFeature(feature string, supported bool) {
	b.features[feature] = supported
}

// CheckFeature checks if a feature is supported
func (b *BaseBackend) CheckFeature(feature string) bool {
	return b.features[feature]
}