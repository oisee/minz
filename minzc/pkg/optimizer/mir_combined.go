package optimizer

import (
	"github.com/minz/minzc/pkg/ir"
)

// MIRCombinedPass runs multiple MIR optimization passes in a loop
// until no more improvements can be made (fixed-point iteration)
type MIRCombinedPass struct {
	name              string
	reorderingPass    *MIRReorderingPass
	peepholePass      *MIRPeepholePass
	maxIterations     int
	optimizationsCount int
}

// NewMIRCombinedPass creates a new combined MIR optimization pass
func NewMIRCombinedPass() *MIRCombinedPass {
	return &MIRCombinedPass{
		name:           "MIR Combined Optimization",
		reorderingPass: NewMIRReorderingPass(),
		peepholePass:   NewMIRPeepholePass(),
		maxIterations:  10, // Prevent infinite loops
	}
}

// Name returns the name of the pass
func (p *MIRCombinedPass) Name() string {
	return p.name
}

// OptimizationsCount returns the total number of optimizations applied
func (p *MIRCombinedPass) OptimizationsCount() int {
	return p.optimizationsCount
}

// Run executes the combined MIR optimization passes
// Order: Reordering -> Peephole -> Repeat until fixpoint
func (p *MIRCombinedPass) Run(module *ir.Module) (bool, error) {
	totalChanged := false
	iterations := 0

	for iterations < p.maxIterations {
		iterationChanged := false
		iterations++

		// Phase 1: Reorder instructions to expose optimization opportunities
		if changed, err := p.reorderingPass.Run(module); err != nil {
			return totalChanged, err
		} else if changed {
			iterationChanged = true
		}

		// Phase 2: Apply peephole optimizations
		if changed, err := p.peepholePass.Run(module); err != nil {
			return totalChanged, err
		} else if changed {
			iterationChanged = true
		}

		if iterationChanged {
			totalChanged = true
		} else {
			// Fixed point reached
			break
		}
	}

	p.optimizationsCount = p.peepholePass.OptimizationsCount()
	return totalChanged, nil
}

// RunOnFunction runs the combined optimization on a single function
func (p *MIRCombinedPass) RunOnFunction(fn *ir.Function) bool {
	changed := false
	iterations := 0

	// Create a temporary module containing just this function
	module := &ir.Module{
		Functions: []*ir.Function{fn},
	}

	for iterations < p.maxIterations {
		iterationChanged := false
		iterations++

		// Apply passes
		if c, _ := p.reorderingPass.Run(module); c {
			iterationChanged = true
		}
		if c, _ := p.peepholePass.Run(module); c {
			iterationChanged = true
		}

		if iterationChanged {
			changed = true
		} else {
			break
		}
	}

	return changed
}

// OptimizationReport returns a summary of optimizations applied
type OptimizationReport struct {
	ConstantFolding       int
	AlgebraicSimplification int
	StrengthReduction     int
	CopyPropagation       int
	DeadCodeElimination   int
	RedundantLoadElimination int
	InstructionReordering int
	TotalIterations       int
}
