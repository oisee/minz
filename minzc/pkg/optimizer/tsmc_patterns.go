package optimizer

import (
	"fmt"
	"github.com/minz/minzc/pkg/ir"
)

// TSMCPatternOptimizer optimizes common TSMC reference patterns
type TSMCPatternOptimizer struct {
	debug bool
}

// NewTSMCPatternOptimizer creates a new TSMC pattern optimizer
func NewTSMCPatternOptimizer(debug bool) *TSMCPatternOptimizer {
	return &TSMCPatternOptimizer{
		debug: debug,
	}
}

// OptimizeModule applies TSMC pattern optimizations to all functions
func (o *TSMCPatternOptimizer) OptimizeModule(module *ir.Module) error {
	for _, function := range module.Functions {
		if function.CallingConvention == "smc" {
			o.optimizeFunction(function)
		}
	}
	return nil
}

// optimizeFunction optimizes TSMC patterns in a function
func (o *TSMCPatternOptimizer) optimizeFunction(f *ir.Function) {
	// Find TSMC reference parameters
	tsmcParams := make(map[string]*ir.Parameter)
	for i := range f.Params {
		if f.Params[i].IsTSMCRef {
			tsmcParams[f.Params[i].Name] = &f.Params[i]
		}
	}
	
	if len(tsmcParams) == 0 {
		return
	}
	
	// Look for common patterns
	o.optimizeLoopIterators(f, tsmcParams)
	o.optimizeConditionalUpdates(f, tsmcParams)
}

// optimizeLoopIterators optimizes TSMC references used as loop iterators
func (o *TSMCPatternOptimizer) optimizeLoopIterators(f *ir.Function, tsmcParams map[string]*ir.Parameter) {
	// Pattern: TSMC reference incremented in loop body
	// Example:
	//   loop:
	//     *ptr = something
	//     ptr = ptr + 1
	//     jump_if condition, loop
	
	for i := 0; i < len(f.Instructions); i++ {
		inst := &f.Instructions[i]
		
		// Look for loop starts (jump targets)
		if inst.Op == ir.OpLabel {
			o.analyzeLoopPattern(f, i, tsmcParams)
		}
	}
}

// analyzeLoopPattern analyzes a potential loop for TSMC optimization
func (o *TSMCPatternOptimizer) analyzeLoopPattern(f *ir.Function, labelIdx int, tsmcParams map[string]*ir.Parameter) {
	labelName := f.Instructions[labelIdx].Symbol
	
	// Find the loop body (from label to jump back)
	var loopEnd int
	for i := labelIdx + 1; i < len(f.Instructions); i++ {
		inst := &f.Instructions[i]
		if inst.Op == ir.OpJump && inst.Symbol == labelName {
			loopEnd = i
			break
		}
		if inst.Op == ir.OpJumpIf && inst.Symbol == labelName {
			loopEnd = i
			break
		}
	}
	
	if loopEnd == 0 {
		return
	}
	
	// Analyze loop body for TSMC pattern
	var tsmcUpdates []tsmcUpdate
	for i := labelIdx + 1; i < loopEnd; i++ {
		inst := &f.Instructions[i]
		
		// Look for TSMC reference updates
		if inst.Op == ir.OpStoreTSMCRef {
			if _, isTSMC := tsmcParams[inst.Symbol]; isTSMC {
				tsmcUpdates = append(tsmcUpdates, tsmcUpdate{
					index:    i,
					paramName: inst.Symbol,
					valueReg: inst.Src1,
				})
			}
		}
	}
	
	// Optimize if we found TSMC updates
	if len(tsmcUpdates) > 0 && o.debug {
		fmt.Printf("Found loop with %d TSMC updates at label %s\n", len(tsmcUpdates), labelName)
	}
}

// optimizeConditionalUpdates optimizes conditional TSMC updates
func (o *TSMCPatternOptimizer) optimizeConditionalUpdates(f *ir.Function, tsmcParams map[string]*ir.Parameter) {
	// Pattern: TSMC reference updated conditionally
	// Example:
	//   if condition {
	//     ptr = ptr + offset
	//   }
	
	for i := 0; i < len(f.Instructions)-2; i++ {
		inst := &f.Instructions[i]
		
		// Look for conditional jumps
		if inst.Op == ir.OpJumpIf || inst.Op == ir.OpJumpIfNot {
			o.analyzeConditionalPattern(f, i, tsmcParams)
		}
	}
}

// analyzeConditionalPattern analyzes conditional TSMC updates
func (o *TSMCPatternOptimizer) analyzeConditionalPattern(f *ir.Function, jumpIdx int, tsmcParams map[string]*ir.Parameter) {
	targetLabel := f.Instructions[jumpIdx].Symbol
	
	// Find the conditional block
	blockStart := jumpIdx + 1
	var blockEnd int
	
	for i := blockStart; i < len(f.Instructions); i++ {
		inst := &f.Instructions[i]
		if inst.Op == ir.OpLabel && inst.Symbol == targetLabel {
			blockEnd = i
			break
		}
	}
	
	if blockEnd == 0 || blockEnd <= blockStart {
		return
	}
	
	// Check for TSMC updates in conditional block
	for i := blockStart; i < blockEnd; i++ {
		inst := &f.Instructions[i]
		if inst.Op == ir.OpStoreTSMCRef {
			if _, isTSMC := tsmcParams[inst.Symbol]; isTSMC && o.debug {
				fmt.Printf("Found conditional TSMC update for %s\n", inst.Symbol)
			}
		}
	}
}

type tsmcUpdate struct {
	index     int
	paramName string
	valueReg  ir.Register
}

// IsTSMCPatternPass returns true if this is the TSMC pattern optimization pass
func (o *TSMCPatternOptimizer) IsTSMCPatternPass() bool {
	return true
}

// GetName returns the name of this optimization pass
func (o *TSMCPatternOptimizer) GetName() string {
	return "TSMC Pattern Optimization"
}