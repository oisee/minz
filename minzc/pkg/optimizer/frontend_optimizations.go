package optimizer

import (
	"github.com/minz/minzc/pkg/ir"
)

// FrontendOptimizer provides high-level optimization interface
type FrontendOptimizer struct {
	simpleOptimizer *SimpleOptimizer
}

// NewFrontendOptimizer creates a new frontend optimizer
func NewFrontendOptimizer() *FrontendOptimizer {
	return &FrontendOptimizer{
		simpleOptimizer: NewSimpleOptimizer(),
	}
}

// ApplyOptimizations applies all frontend optimizations to a module
func (fo *FrontendOptimizer) ApplyOptimizations(module *ir.Module) *SimpleOptimizationReport {
	return fo.simpleOptimizer.OptimizeModule(module)
}