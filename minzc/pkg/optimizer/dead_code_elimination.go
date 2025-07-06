package optimizer

import (
	"github.com/minz/minzc/pkg/ir"
)

// DeadCodeEliminationPass removes dead code
type DeadCodeEliminationPass struct {
	used      map[ir.Register]bool
	labelRefs map[string]bool
}

// NewDeadCodeEliminationPass creates a new dead code elimination pass
func NewDeadCodeEliminationPass() Pass {
	return &DeadCodeEliminationPass{
		used:      make(map[ir.Register]bool),
		labelRefs: make(map[string]bool),
	}
}

// Name returns the name of this pass
func (p *DeadCodeEliminationPass) Name() string {
	return "Dead Code Elimination"
}

// Run performs dead code elimination on the module
func (p *DeadCodeEliminationPass) Run(module *ir.Module) (bool, error) {
	changed := false
	
	for _, function := range module.Functions {
		if p.optimizeFunction(function) {
			changed = true
		}
	}
	
	return changed, nil
}

// optimizeFunction performs dead code elimination on a single function
func (p *DeadCodeEliminationPass) optimizeFunction(fn *ir.Function) bool {
	changed := false
	
	// Mark all used registers and referenced labels
	p.markUsedRegisters(fn)
	p.markReferencedLabels(fn)
	
	// Remove dead instructions
	newInstructions := []ir.Instruction{}
	afterUnreachable := false
	
	for _, inst := range fn.Instructions {
		keep := true
		
		// Skip instructions after unconditional jump/return until next label
		if afterUnreachable && inst.Op != ir.OpLabel {
			keep = false
			changed = true
		}
		
		switch inst.Op {
		case ir.OpReturn:
			afterUnreachable = true
			
		case ir.OpJump:
			afterUnreachable = true
			
		case ir.OpLabel:
			afterUnreachable = false
			// Remove unreferenced labels
			if !p.labelRefs[inst.Label] {
				keep = false
				changed = true
			}
			
		case ir.OpLoadConst, ir.OpLoadVar, ir.OpLoadField,
			 ir.OpAdd, ir.OpSub, ir.OpMul, ir.OpDiv, ir.OpMod,
			 ir.OpAnd, ir.OpOr, ir.OpXor, ir.OpShl, ir.OpShr,
			 ir.OpNeg, ir.OpNot,
			 ir.OpEq, ir.OpNe, ir.OpLt, ir.OpGt, ir.OpLe, ir.OpGe,
			 ir.OpAlloc:
			// Remove instructions whose results are never used
			if inst.Dest != 0 && !p.used[inst.Dest] {
				keep = false
				changed = true
			}
			
		case ir.OpStoreVar, ir.OpStoreField:
			// Keep stores for now (may have side effects)
			// TODO: More sophisticated analysis for dead stores
			
		case ir.OpCall:
			// Keep calls for now (may have side effects)
			// TODO: Pure function analysis
			
		case ir.OpJumpIfNot:
			// Keep conditional jumps
			afterUnreachable = false
		}
		
		if keep {
			newInstructions = append(newInstructions, inst)
		}
	}
	
	// Remove redundant jumps
	finalInstructions := []ir.Instruction{}
	for i, inst := range newInstructions {
		keep := true
		
		if inst.Op == ir.OpJump {
			// Check if jumping to next instruction
			if i+1 < len(newInstructions) && newInstructions[i+1].Op == ir.OpLabel {
				if inst.Label == newInstructions[i+1].Label {
					keep = false
					changed = true
				}
			}
		}
		
		if keep {
			finalInstructions = append(finalInstructions, inst)
		}
	}
	
	if changed {
		fn.Instructions = finalInstructions
	}
	
	return changed
}

// markUsedRegisters marks all registers that are used
func (p *DeadCodeEliminationPass) markUsedRegisters(fn *ir.Function) {
	p.used = make(map[ir.Register]bool)
	
	// Mark function parameters as used
	for i := 0; i < fn.NumParams; i++ {
		p.used[ir.Register(i+1)] = true
	}
	
	// Mark registers used in instructions
	for _, inst := range fn.Instructions {
		switch inst.Op {
		case ir.OpReturn:
			if inst.Src1 != 0 {
				p.used[inst.Src1] = true
			}
			
		case ir.OpStoreVar, ir.OpStoreField:
			if inst.Src1 != 0 {
				p.used[inst.Src1] = true
			}
			if inst.Src2 != 0 {
				p.used[inst.Src2] = true
			}
			
		case ir.OpAdd, ir.OpSub, ir.OpMul, ir.OpDiv, ir.OpMod,
			 ir.OpAnd, ir.OpOr, ir.OpXor, ir.OpShl, ir.OpShr,
			 ir.OpEq, ir.OpNe, ir.OpLt, ir.OpGt, ir.OpLe, ir.OpGe:
			if inst.Src1 != 0 {
				p.used[inst.Src1] = true
			}
			if inst.Src2 != 0 {
				p.used[inst.Src2] = true
			}
			
		case ir.OpNeg, ir.OpNot, ir.OpLoadVar, ir.OpLoadField:
			if inst.Src1 != 0 {
				p.used[inst.Src1] = true
			}
			
		case ir.OpJumpIfNot:
			if inst.Src1 != 0 {
				p.used[inst.Src1] = true
			}
			
		case ir.OpCall:
			// Mark all argument registers as used
			// TODO: Track actual arguments
			if inst.Src1 != 0 {
				p.used[inst.Src1] = true
			}
			if inst.Src2 != 0 {
				p.used[inst.Src2] = true
			}
		}
		
		// If this instruction's result is used, mark its operands as used too
		if inst.Dest != 0 && p.used[inst.Dest] {
			if inst.Src1 != 0 {
				p.used[inst.Src1] = true
			}
			if inst.Src2 != 0 {
				p.used[inst.Src2] = true
			}
		}
	}
}

// markReferencedLabels marks all labels that are referenced by jumps
func (p *DeadCodeEliminationPass) markReferencedLabels(fn *ir.Function) {
	p.labelRefs = make(map[string]bool)
	
	for _, inst := range fn.Instructions {
		switch inst.Op {
		case ir.OpJump, ir.OpJumpIfNot:
			if inst.Label != "" {
				p.labelRefs[inst.Label] = true
			}
		}
	}
}