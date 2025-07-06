package optimizer

import (
	"fmt"

	"github.com/minz/minzc/pkg/ir"
)

// OptimizationLevel represents different optimization levels
type OptimizationLevel int

const (
	OptLevelNone OptimizationLevel = 0
	OptLevelBasic OptimizationLevel = 1
	OptLevelFull OptimizationLevel = 2
)

// Pass represents an optimization pass
type Pass interface {
	Name() string
	Run(module *ir.Module) (bool, error)
}

// Optimizer manages and runs optimization passes
type Optimizer struct {
	level  OptimizationLevel
	passes []Pass
}

// NewOptimizer creates a new optimizer with the specified level
func NewOptimizer(level OptimizationLevel) *Optimizer {
	opt := &Optimizer{
		level: level,
	}
	
	// Configure passes based on optimization level
	if level >= OptLevelBasic {
		// Basic optimizations
		opt.passes = append(opt.passes,
			NewConstantFoldingPass(),
			NewDeadCodeEliminationPass(),
		)
	}
	
	if level >= OptLevelFull {
		// Advanced optimizations
		opt.passes = append(opt.passes,
			NewPeepholeOptimizationPass(),
			NewRegisterAllocationPass(),
			NewInliningPass(),
		)
	}
	
	return opt
}

// Optimize runs all configured optimization passes on the module
func (o *Optimizer) Optimize(module *ir.Module) error {
	if o.level == OptLevelNone {
		return nil
	}
	
	// Keep running passes until no more changes
	maxIterations := 10
	for iteration := 0; iteration < maxIterations; iteration++ {
		changed := false
		
		for _, pass := range o.passes {
			passChanged, err := pass.Run(module)
			if err != nil {
				return fmt.Errorf("optimization pass %s failed: %w", pass.Name(), err)
			}
			
			if passChanged {
				changed = true
			}
		}
		
		// If no pass made changes, we're done
		if !changed {
			break
		}
	}
	
	return nil
}

// Helper functions for common optimization tasks

// IsConstant checks if an instruction produces a constant value
func IsConstant(inst *ir.Instruction) bool {
	return inst.Op == ir.OpLoadConst
}

// GetConstantValue returns the constant value from a load constant instruction
func GetConstantValue(inst *ir.Instruction) (int64, bool) {
	if inst.Op == ir.OpLoadConst {
		return inst.Imm, true
	}
	return 0, false
}

// IsDeadStore checks if a store instruction is dead (value never used)
func IsDeadStore(inst *ir.Instruction, uses map[ir.Register]int) bool {
	if inst.Op == ir.OpStoreVar || inst.Op == ir.OpStoreField {
		return uses[inst.Dest] == 0
	}
	return false
}

// ReplaceRegister replaces all uses of oldReg with newReg in the instruction
func ReplaceRegister(inst *ir.Instruction, oldReg, newReg ir.Register) {
	if inst.Src1 == oldReg {
		inst.Src1 = newReg
	}
	if inst.Src2 == oldReg {
		inst.Src2 = newReg
	}
	if inst.Dest == oldReg {
		inst.Dest = newReg
	}
}