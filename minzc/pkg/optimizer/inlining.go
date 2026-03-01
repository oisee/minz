package optimizer

import (
	"fmt"
	"strings"

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
	// Don't inline main, interrupt handlers, or extern functions
	if fn.Name == "main" || fn.IsInterrupt || fn.IsExtern {
		return false
	}

	// Don't inline asm functions (IsAsm flag or body contains OpAsm instructions)
	// Asm functions use physical registers (A, HL) directly in raw assembly —
	// IR-level inlining can't correctly wire up argument loading because
	// OpAsm instructions don't reference IR virtual registers, causing the
	// argument OpLoadConst to become dead code and get eliminated.
	if fn.IsAsm {
		return false
	}
	for _, inst := range fn.Instructions {
		if inst.Op == ir.OpAsm {
			return false
		}
	}

	// Don't inline reducer lambdas (2-parameter lambdas for reduce operations)
	// The parameter remapping isn't fully implemented for multi-param calls
	if strings.HasPrefix(fn.Name, "reducer_lambda_") {
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
	
	// Don't inline functions with LOAD_PARAM or SMC_LOAD instructions
	// (proper parameter remapping not yet implemented - causes "parameter not found" errors)
	for _, inst := range fn.Instructions {
		if inst.Op == ir.OpLoadParam || inst.Op == ir.OpTrueSMCLoad {
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

// inlineCounter tracks unique suffixes for inlined labels
var inlineCounter int

// generateInlinedCode generates the inlined version of a function call
func (p *InliningPass) generateInlinedCode(fn *ir.Function, call ir.Instruction, nextReg *ir.Register) []ir.Instruction {
	var result []ir.Instruction

	// Increment inline counter for unique labels
	inlineCounter++
	labelSuffix := fmt.Sprintf("_inline%d", inlineCounter)

	// Create register mapping for inlining
	regMap := make(map[ir.Register]ir.Register)

	// Create label mapping for inlining (to avoid duplicates when same function inlined multiple times)
	labelMap := make(map[string]string)

	// Collect all labels from the function
	for _, inst := range fn.Instructions {
		if inst.Op == ir.OpLabel && inst.Label != "" {
			labelMap[inst.Label] = inst.Label + labelSuffix
		}
	}

	// Map parameters: connect formal parameter registers to actual argument registers.
	// Use Params[i].Reg to get the actual register assigned to each parameter,
	// falling back to registers 1, 2, ... for functions that don't set Param.Reg.
	for i := 0; i < fn.NumParams; i++ {
		var formalReg ir.Register
		if i < len(fn.Params) && fn.Params[i].Reg != 0 {
			formalReg = fn.Params[i].Reg
		} else {
			formalReg = ir.Register(i + 1)
		}
		if i < len(call.Args) && call.Args[i] != 0 {
			// Map the formal parameter to the actual argument register
			regMap[formalReg] = call.Args[i]
		} else {
			regMap[formalReg] = formalReg // Fallback identity mapping
		}
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
	
	// Build parameter name → argument register map for OpLoadVar/OpLoadParam substitution.
	// When inlining, "load x" (where x is a parameter) becomes a move from the actual argument.
	paramArgMap := make(map[string]ir.Register)
	for i, param := range fn.Params {
		if i < len(call.Args) && call.Args[i] != 0 {
			paramArgMap[param.Name] = call.Args[i]
		}
	}

	// Generate inlined instructions
	for _, inst := range fn.Instructions {
		newInst := inst

		// Replace parameter loads with moves from the argument register.
		// This handles the case where a function loads its parameter by name
		// (OpLoadVar/OpLoadParam with Symbol matching a parameter name).
		if (inst.Op == ir.OpLoadVar || inst.Op == ir.OpLoadParam) && inst.Symbol != "" {
			if argReg, ok := paramArgMap[inst.Symbol]; ok {
				destReg := regMap[inst.Dest]
				if destReg == 0 {
					destReg = inst.Dest
				}
				result = append(result, ir.Instruction{
					Op:      ir.OpMove,
					Dest:    destReg,
					Src1:    argReg,
					Comment: fmt.Sprintf("Inlined: load param %s from arg r%d", inst.Symbol, argReg),
				})
				continue
			}
		}

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

		// Remap Args registers (for OpCall instructions inside inlined body)
		if len(inst.Args) > 0 {
			newArgs := make([]ir.Register, len(inst.Args))
			for i, arg := range inst.Args {
				if mapped, ok := regMap[arg]; ok {
					newArgs[i] = mapped
				} else {
					newArgs[i] = arg
				}
			}
			newInst.Args = newArgs
		}

		// Remap labels to avoid duplicates when inlining same function multiple times
		if inst.Label != "" {
			if mapped, ok := labelMap[inst.Label]; ok {
				newInst.Label = mapped
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