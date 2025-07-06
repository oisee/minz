package optimizer

import (
	"github.com/minz/minzc/pkg/ir"
)

// InliningPass performs function inlining optimization
type InliningPass struct {
	inlineCandidates map[string]*ir.Function
	maxInlineSize    int
}

// NewInliningPass creates a new inlining pass
func NewInliningPass() Pass {
	return &InliningPass{
		inlineCandidates: make(map[string]*ir.Function),
		maxInlineSize:    10, // Max instructions for a function to be inlined
	}
}

// Name returns the name of this pass
func (p *InliningPass) Name() string {
	return "Function Inlining"
}

// Run performs function inlining on the module
func (p *InliningPass) Run(module *ir.Module) (bool, error) {
	// First, identify inline candidates
	p.identifyCandidates(module)
	
	// Then inline calls to these functions
	changed := false
	for _, function := range module.Functions {
		if p.inlineCalls(function) {
			changed = true
		}
	}
	
	return changed, nil
}

// identifyCandidates identifies functions suitable for inlining
func (p *InliningPass) identifyCandidates(module *ir.Module) {
	p.inlineCandidates = make(map[string]*ir.Function)
	
	for _, fn := range module.Functions {
		if p.isInlineCandidate(fn) {
			p.inlineCandidates[fn.Name] = fn
		}
	}
}

// isInlineCandidate checks if a function is suitable for inlining
func (p *InliningPass) isInlineCandidate(fn *ir.Function) bool {
	// Don't inline main or interrupt handlers
	if fn.Name == "main" || fn.IsInterrupt {
		return false
	}
	
	// Check size
	if len(fn.Instructions) > p.maxInlineSize {
		return false
	}
	
	// Don't inline recursive functions
	for _, inst := range fn.Instructions {
		if inst.Op == ir.OpCall && inst.Symbol == fn.Name {
			return false
		}
	}
	
	// Don't inline functions with loops (for now)
	for _, inst := range fn.Instructions {
		if inst.Op == ir.OpJump || inst.Op == ir.OpJumpIfNot {
			// Check if jumping backwards (simple loop detection)
			for i, checkInst := range fn.Instructions {
				if checkInst.Op == ir.OpLabel && checkInst.Label == inst.Label {
					// Found the target label
					for j := i + 1; j < len(fn.Instructions); j++ {
						if &fn.Instructions[j] == &inst {
							// Jump is backwards - likely a loop
							return false
						}
					}
					break
				}
			}
		}
	}
	
	return true
}

// inlineCalls inlines eligible function calls in the given function
func (p *InliningPass) inlineCalls(fn *ir.Function) bool {
	changed := false
	newInstructions := []ir.Instruction{}
	nextReg := fn.NextRegister
	
	for _, inst := range fn.Instructions {
		if inst.Op == ir.OpCall {
			if inlineFunc, ok := p.inlineCandidates[inst.Symbol]; ok {
				// Inline this call
				inlined := p.generateInlinedCode(inlineFunc, inst, &nextReg)
				newInstructions = append(newInstructions, inlined...)
				changed = true
			} else {
				// Keep the call
				newInstructions = append(newInstructions, inst)
			}
		} else {
			newInstructions = append(newInstructions, inst)
		}
	}
	
	if changed {
		fn.Instructions = newInstructions
		fn.NextRegister = nextReg
	}
	
	return changed
}

// generateInlinedCode generates the inlined version of a function call
func (p *InliningPass) generateInlinedCode(fn *ir.Function, call ir.Instruction, nextReg *ir.Register) []ir.Instruction {
	var result []ir.Instruction
	
	// Create register mapping for inlining
	regMap := make(map[ir.Register]ir.Register)
	
	// Map parameters
	// TODO: Proper parameter passing
	// For now, assume parameters are passed in order starting from register 1
	for i := 0; i < fn.NumParams; i++ {
		regMap[ir.Register(i+1)] = ir.Register(i+1) // Identity mapping for params
	}
	
	// Map other registers to new ones to avoid conflicts
	for _, inst := range fn.Instructions {
		if inst.Dest != 0 {
			if _, exists := regMap[inst.Dest]; !exists {
				regMap[inst.Dest] = *nextReg
				(*nextReg)++
			}
		}
	}
	
	// Generate inlined instructions
	for _, inst := range fn.Instructions {
		newInst := inst
		
		// Skip return instructions
		if inst.Op == ir.OpReturn {
			// Map return value to call destination
			if inst.Src1 != 0 && call.Dest != 0 {
				result = append(result, ir.Instruction{
					Op:   ir.OpMove,
					Dest: call.Dest,
					Src1: regMap[inst.Src1],
					Comment: "Inlined return value",
				})
			}
			continue
		}
		
		// Remap registers
		if inst.Dest != 0 {
			newInst.Dest = regMap[inst.Dest]
		}
		if inst.Src1 != 0 {
			if mapped, ok := regMap[inst.Src1]; ok {
				newInst.Src1 = mapped
			}
		}
		if inst.Src2 != 0 {
			if mapped, ok := regMap[inst.Src2]; ok {
				newInst.Src2 = mapped
			}
		}
		
		// Add comment to indicate inlining
		if newInst.Comment != "" {
			newInst.Comment = "Inlined: " + newInst.Comment
		} else {
			newInst.Comment = "Inlined from " + fn.Name
		}
		
		result = append(result, newInst)
	}
	
	return result
}