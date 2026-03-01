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

// markUsedRegisters marks all registers that are used.
// Uses two-phase analysis: first marks registers consumed by side-effecting
// instructions, then iteratively propagates through data-flow dependencies.
func (p *DeadCodeEliminationPass) markUsedRegisters(fn *ir.Function) {
	p.used = make(map[ir.Register]bool)

	// Mark function parameters as used
	for i := 0; i < fn.NumParams; i++ {
		p.used[ir.Register(i+1)] = true
	}

	// Phase 1: Mark registers directly consumed by side-effecting instructions
	// (returns, stores, calls, prints, jumps, syscalls, port I/O, etc.)
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

		case ir.OpJumpIf, ir.OpJumpIfNot, ir.OpJumpIfZero, ir.OpJumpIfNotZero:
			if inst.Src1 != 0 {
				p.used[inst.Src1] = true
			}

		case ir.OpJumpIfFlag:
			// Inline filter: Src1 = element register, Src2 = constant register
			if inst.Src1 != 0 {
				p.used[inst.Src1] = true
			}
			if inst.Src2 != 0 {
				p.used[inst.Src2] = true
			}

		case ir.OpCall:
			// Mark all argument registers as used
			if inst.Src1 != 0 {
				p.used[inst.Src1] = true
			}
			if inst.Src2 != 0 {
				p.used[inst.Src2] = true
			}
			for _, arg := range inst.Args {
				if arg != 0 {
					p.used[arg] = true
				}
			}
			// Call results are used if dest is consumed (handled in phase 2)

		case ir.OpPrintU8, ir.OpPrintU16, ir.OpPrintI8, ir.OpPrintI16,
			 ir.OpPrintBool, ir.OpPrintString, ir.OpPrintChar:
			if inst.Src1 != 0 {
				p.used[inst.Src1] = true
			}

		case ir.OpSyscall:
			if inst.Src1 != 0 {
				p.used[inst.Src1] = true
			}
			if inst.Src2 != 0 {
				p.used[inst.Src2] = true
			}
			p.used[ir.Register(0)] = true

		case ir.OpMove:
			if inst.Src1 != 0 {
				p.used[inst.Src1] = true
			}

		case ir.OpPortIn, ir.OpPortOut:
			if inst.Src1 != 0 {
				p.used[inst.Src1] = true
			}
			if inst.Src2 != 0 {
				p.used[inst.Src2] = true
			}

		case ir.OpDJNZ:
			if inst.Src1 != 0 {
				p.used[inst.Src1] = true
			}

		case ir.OpInc:
			if inst.Src1 != 0 {
				p.used[inst.Src1] = true
			}

		case ir.OpLoad:
			if inst.Src1 != 0 {
				p.used[inst.Src1] = true
			}

		case ir.OpPush:
			if inst.Src1 != 0 {
				p.used[inst.Src1] = true
			}

		// Pure computation ops (OpAdd, OpSub, OpMul, etc.) are NOT marked here.
		// Their sources are only used if their dest is consumed (phase 2).
		}
	}

	// Phase 2: Iterative data-flow propagation.
	// If an instruction's dest is used, its sources become used too.
	// Iterate until stable (removing OpAdd makes its LoadConst sources dead).
	changed := true
	for changed {
		changed = false
		for _, inst := range fn.Instructions {
			if inst.Dest != 0 && p.used[inst.Dest] {
				if inst.Src1 != 0 && !p.used[inst.Src1] {
					p.used[inst.Src1] = true
					changed = true
				}
				if inst.Src2 != 0 && !p.used[inst.Src2] {
					p.used[inst.Src2] = true
					changed = true
				}
			}
		}
	}
}

// markReferencedLabels marks all labels that are referenced by jumps
func (p *DeadCodeEliminationPass) markReferencedLabels(fn *ir.Function) {
	p.labelRefs = make(map[string]bool)

	for _, inst := range fn.Instructions {
		switch inst.Op {
		case ir.OpJump, ir.OpJumpIf, ir.OpJumpIfNot,
			ir.OpJumpIfZero, ir.OpJumpIfNotZero, ir.OpJumpIfFlag,
			ir.OpDJNZ:
			if inst.Label != "" {
				p.labelRefs[inst.Label] = true
			}
		}
	}
}