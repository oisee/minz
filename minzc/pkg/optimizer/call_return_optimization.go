package optimizer

import (
	"strings"
	
	"github.com/minz/minzc/pkg/ir"
)

// CallReturnOptimizationPass optimizes function calls that directly assign to variables
// When a function is only called once and its return value is immediately stored,
// we can have the function directly write to the target location
type CallReturnOptimizationPass struct {
	callSites map[string][]*CallSite // function name -> call sites
}

// CallSite represents a function call and its usage
type CallSite struct {
	Function     *ir.Function
	CallInst     *ir.Instruction
	CallIndex    int
	StoreInst    *ir.Instruction
	StoreIndex   int
	TargetSymbol string
}

// NewCallReturnOptimizationPass creates a new call return optimization pass
func NewCallReturnOptimizationPass() Pass {
	return &CallReturnOptimizationPass{
		callSites: make(map[string][]*CallSite),
	}
}

// Name returns the name of this pass
func (p *CallReturnOptimizationPass) Name() string {
	return "Call Return Optimization"
}

// Run performs the optimization
func (p *CallReturnOptimizationPass) Run(module *ir.Module) (bool, error) {
	// First pass: collect all call sites
	for _, fn := range module.Functions {
		p.analyzeFunction(fn)
	}

	modified := false

	// Second pass: optimize single-call functions
	for funcName, sites := range p.callSites {
		if len(sites) == 1 && sites[0].StoreInst != nil {
			// This function is called only once and its result is stored
			if p.optimizeCallSite(module, funcName, sites[0]) {
				modified = true
			}
		}
	}

	return modified, nil
}

// analyzeFunction collects call sites in a function
func (p *CallReturnOptimizationPass) analyzeFunction(fn *ir.Function) {
	for i, inst := range fn.Instructions {
		if inst.Op == ir.OpCall {
			site := &CallSite{
				Function:  fn,
				CallInst:  &fn.Instructions[i],
				CallIndex: i,
			}

			// Check if the next instruction stores the result
			if i+1 < len(fn.Instructions) {
				nextInst := fn.Instructions[i+1]
				if nextInst.Op == ir.OpStoreVar && nextInst.Src1 == inst.Dest {
					site.StoreInst = &fn.Instructions[i+1]
					site.StoreIndex = i + 1
					if nextInst.Symbol != "" {
						site.TargetSymbol = nextInst.Symbol
					}
				}
			}

			funcName := inst.Symbol
			p.callSites[funcName] = append(p.callSites[funcName], site)
		}
	}
}

// optimizeCallSite optimizes a single call site
func (p *CallReturnOptimizationPass) optimizeCallSite(module *ir.Module, funcName string, site *CallSite) bool {
	// Find the called function
	var calledFunc *ir.Function
	for _, fn := range module.Functions {
		// Check both exact match and suffix match (for module-prefixed names)
		if fn.Name == funcName || strings.HasSuffix(fn.Name, "."+funcName) {
			calledFunc = fn
			break
		}
	}

	if calledFunc == nil {
		return false
	}

	// Mark the function for direct return optimization
	calledFunc.SetMetadata("direct_return_target", site.TargetSymbol)
	calledFunc.SetMetadata("direct_return_optimization", "true")

	// Remove the store instruction from the caller
	site.Function.Instructions = append(
		site.Function.Instructions[:site.StoreIndex],
		site.Function.Instructions[site.StoreIndex+1:]...,
	)

	// Add a comment to indicate optimization
	site.CallInst.Comment = "Optimized: return value directly stored to " + site.TargetSymbol

	return true
}