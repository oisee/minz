package optimizer

import (
	"fmt"
	"os"

	"github.com/minz/minzc/pkg/ir"
)

// MIRPeepholePass implements peephole optimizations at the MIR level
// These optimizations work on the intermediate representation before code generation
// Benefits: Architecture-independent, more optimization opportunities, better analysis
type MIRPeepholePass struct {
	name              string
	optimizationsCount int
	constants         map[ir.Register]int  // Track known constant values
}

// NewMIRPeepholePass creates a new MIR peephole optimization pass
func NewMIRPeepholePass() *MIRPeepholePass {
	return &MIRPeepholePass{
		name:      "MIR Peephole Optimization",
		constants: make(map[ir.Register]int),
	}
}

// Name returns the name of the pass
func (p *MIRPeepholePass) Name() string {
	return p.name
}

// OptimizationsCount returns the number of optimizations applied
func (p *MIRPeepholePass) OptimizationsCount() int {
	return p.optimizationsCount
}

// Run executes the MIR peephole optimization pass
func (p *MIRPeepholePass) Run(module *ir.Module) (bool, error) {
	changed := false

	for _, fn := range module.Functions {
		p.constants = make(map[ir.Register]int) // Reset per function
		if p.optimizeFunction(fn) {
			changed = true
		}
	}

	return changed, nil
}

// optimizeFunction applies peephole optimizations to a function
func (p *MIRPeepholePass) optimizeFunction(fn *ir.Function) bool {
	changed := false
	const maxIterations = 100 // Safety limit to prevent infinite loops

	// Multiple passes until no more changes
	for iteration := 0; iteration < maxIterations; iteration++ {
		passChanged := false
		var changedBy string

		// Track constants
		p.trackConstants(fn)

		// Apply each optimization
		if p.constantFolding(fn) {
			passChanged = true
			changedBy = "constantFolding"
		}

		if p.algebraicSimplification(fn) {
			passChanged = true
			if changedBy != "" {
				changedBy += ", algebraicSimplification"
			} else {
				changedBy = "algebraicSimplification"
			}
		}

		if p.strengthReduction(fn) {
			passChanged = true
			if changedBy != "" {
				changedBy += ", strengthReduction"
			} else {
				changedBy = "strengthReduction"
			}
		}

		if p.copyPropagation(fn) {
			passChanged = true
			if changedBy != "" {
				changedBy += ", copyPropagation"
			} else {
				changedBy = "copyPropagation"
			}
		}

		if p.deadCodeElimination(fn) {
			passChanged = true
			if changedBy != "" {
				changedBy += ", deadCodeElimination"
			} else {
				changedBy = "deadCodeElimination"
			}
		}

		if p.redundantLoadElimination(fn) {
			passChanged = true
			if changedBy != "" {
				changedBy += ", redundantLoadElimination"
			} else {
				changedBy = "redundantLoadElimination"
			}
		}

		if !passChanged {
			break
		}
		changed = true

		// Debug: log if we're getting close to the limit
		if iteration >= maxIterations-5 {
			fmt.Fprintf(os.Stderr, "Warning: optimizer iteration %d for %s, changed by: %s\n",
				iteration, fn.Name, changedBy)
		}
	}

	return changed
}

// trackConstants identifies registers with known constant values
func (p *MIRPeepholePass) trackConstants(fn *ir.Function) {
	p.constants = make(map[ir.Register]int)

	for _, inst := range fn.Instructions {
		if inst.Op == ir.OpLoadConst {
			p.constants[inst.Dest] = inst.Value
		}
	}
}

// constantFolding evaluates constant expressions at compile time
func (p *MIRPeepholePass) constantFolding(fn *ir.Function) bool {
	changed := false

	for i := range fn.Instructions {
		inst := &fn.Instructions[i]

		// Check if both operands are constants
		val1, const1 := p.constants[inst.Src1]
		val2, const2 := p.constants[inst.Src2]

		if !const1 || !const2 {
			continue
		}

		var result int
		canFold := true

		switch inst.Op {
		case ir.OpAdd:
			result = val1 + val2
		case ir.OpSub:
			result = val1 - val2
		case ir.OpMul:
			result = val1 * val2
		case ir.OpDiv:
			if val2 != 0 {
				result = val1 / val2
			} else {
				canFold = false
			}
		case ir.OpMod:
			if val2 != 0 {
				result = val1 % val2
			} else {
				canFold = false
			}
		case ir.OpAnd:
			result = val1 & val2
		case ir.OpOr:
			result = val1 | val2
		case ir.OpXor:
			result = val1 ^ val2
		case ir.OpShl:
			result = val1 << uint(val2)
		case ir.OpShr:
			result = val1 >> uint(val2)
		case ir.OpEq:
			if val1 == val2 { result = 1 } else { result = 0 }
		case ir.OpNe:
			if val1 != val2 { result = 1 } else { result = 0 }
		case ir.OpLt:
			if val1 < val2 { result = 1 } else { result = 0 }
		case ir.OpLe:
			if val1 <= val2 { result = 1 } else { result = 0 }
		case ir.OpGt:
			if val1 > val2 { result = 1 } else { result = 0 }
		case ir.OpGe:
			if val1 >= val2 { result = 1 } else { result = 0 }
		default:
			canFold = false
		}

		if canFold {
			// Replace with LoadConst
			inst.Op = ir.OpLoadConst
			inst.Value = result
			inst.Src1 = 0
			inst.Src2 = 0
			p.constants[inst.Dest] = result
			p.optimizationsCount++
			changed = true
		}
	}

	return changed
}

// algebraicSimplification simplifies expressions using algebraic identities
func (p *MIRPeepholePass) algebraicSimplification(fn *ir.Function) bool {
	changed := false

	for i := range fn.Instructions {
		inst := &fn.Instructions[i]

		val1, const1 := p.constants[inst.Src1]
		val2, const2 := p.constants[inst.Src2]

		switch inst.Op {
		case ir.OpAdd:
			// x + 0 = x
			if const2 && val2 == 0 {
				inst.Op = ir.OpMove
				inst.Src2 = 0
				p.optimizationsCount++
				changed = true
			}
			// 0 + x = x
			if const1 && val1 == 0 {
				inst.Op = ir.OpMove
				inst.Src1 = inst.Src2
				inst.Src2 = 0
				p.optimizationsCount++
				changed = true
			}

		case ir.OpSub:
			// x - 0 = x
			if const2 && val2 == 0 {
				inst.Op = ir.OpMove
				inst.Src2 = 0
				p.optimizationsCount++
				changed = true
			}
			// x - x = 0
			if inst.Src1 == inst.Src2 {
				inst.Op = ir.OpLoadConst
				inst.Value = 0
				inst.Src1 = 0
				inst.Src2 = 0
				p.constants[inst.Dest] = 0
				p.optimizationsCount++
				changed = true
			}

		case ir.OpMul:
			// x * 0 = 0
			if (const1 && val1 == 0) || (const2 && val2 == 0) {
				inst.Op = ir.OpLoadConst
				inst.Value = 0
				inst.Src1 = 0
				inst.Src2 = 0
				p.constants[inst.Dest] = 0
				p.optimizationsCount++
				changed = true
			}
			// x * 1 = x
			if const2 && val2 == 1 {
				inst.Op = ir.OpMove
				inst.Src2 = 0
				p.optimizationsCount++
				changed = true
			}
			// 1 * x = x
			if const1 && val1 == 1 {
				inst.Op = ir.OpMove
				inst.Src1 = inst.Src2
				inst.Src2 = 0
				p.optimizationsCount++
				changed = true
			}

		case ir.OpDiv:
			// x / 1 = x
			if const2 && val2 == 1 {
				inst.Op = ir.OpMove
				inst.Src2 = 0
				p.optimizationsCount++
				changed = true
			}
			// 0 / x = 0 (when x != 0)
			if const1 && val1 == 0 && (!const2 || val2 != 0) {
				inst.Op = ir.OpLoadConst
				inst.Value = 0
				inst.Src1 = 0
				inst.Src2 = 0
				p.constants[inst.Dest] = 0
				p.optimizationsCount++
				changed = true
			}

		case ir.OpAnd:
			// x & 0 = 0
			if (const1 && val1 == 0) || (const2 && val2 == 0) {
				inst.Op = ir.OpLoadConst
				inst.Value = 0
				inst.Src1 = 0
				inst.Src2 = 0
				p.constants[inst.Dest] = 0
				p.optimizationsCount++
				changed = true
			}
			// x & 0xFF = x (for 8-bit)
			if const2 && val2 == 0xFF {
				inst.Op = ir.OpMove
				inst.Src2 = 0
				p.optimizationsCount++
				changed = true
			}
			// x & x = x
			if inst.Src1 == inst.Src2 {
				inst.Op = ir.OpMove
				inst.Src2 = 0
				p.optimizationsCount++
				changed = true
			}

		case ir.OpOr:
			// x | 0 = x
			if const2 && val2 == 0 {
				inst.Op = ir.OpMove
				inst.Src2 = 0
				p.optimizationsCount++
				changed = true
			}
			// 0 | x = x
			if const1 && val1 == 0 {
				inst.Op = ir.OpMove
				inst.Src1 = inst.Src2
				inst.Src2 = 0
				p.optimizationsCount++
				changed = true
			}
			// x | x = x
			if inst.Src1 == inst.Src2 {
				inst.Op = ir.OpMove
				inst.Src2 = 0
				p.optimizationsCount++
				changed = true
			}

		case ir.OpXor:
			// x ^ 0 = x
			if const2 && val2 == 0 {
				inst.Op = ir.OpMove
				inst.Src2 = 0
				p.optimizationsCount++
				changed = true
			}
			// x ^ x = 0
			if inst.Src1 == inst.Src2 {
				inst.Op = ir.OpLoadConst
				inst.Value = 0
				inst.Src1 = 0
				inst.Src2 = 0
				p.constants[inst.Dest] = 0
				p.optimizationsCount++
				changed = true
			}

		case ir.OpShl, ir.OpShr:
			// x << 0 = x, x >> 0 = x
			if const2 && val2 == 0 {
				inst.Op = ir.OpMove
				inst.Src2 = 0
				p.optimizationsCount++
				changed = true
			}
		}
	}

	return changed
}

// strengthReduction replaces expensive operations with cheaper equivalents
func (p *MIRPeepholePass) strengthReduction(fn *ir.Function) bool {
	changed := false

	for i := range fn.Instructions {
		inst := &fn.Instructions[i]
		val2, const2 := p.constants[inst.Src2]

		switch inst.Op {
		case ir.OpMul:
			// x * 2 = x << 1
			if const2 && val2 == 2 {
				inst.Op = ir.OpShl
				inst.Value = 1
				// Create a temp const register for shift amount
				p.optimizationsCount++
				changed = true
			}
			// x * 4 = x << 2
			if const2 && val2 == 4 {
				inst.Op = ir.OpShl
				inst.Value = 2
				p.optimizationsCount++
				changed = true
			}
			// x * 8 = x << 3
			if const2 && val2 == 8 {
				inst.Op = ir.OpShl
				inst.Value = 3
				p.optimizationsCount++
				changed = true
			}
			// x * 16 = x << 4
			if const2 && val2 == 16 {
				inst.Op = ir.OpShl
				inst.Value = 4
				p.optimizationsCount++
				changed = true
			}

		case ir.OpDiv:
			// x / 2 = x >> 1 (for unsigned)
			if const2 && val2 == 2 {
				inst.Op = ir.OpShr
				inst.Value = 1
				p.optimizationsCount++
				changed = true
			}
			// x / 4 = x >> 2
			if const2 && val2 == 4 {
				inst.Op = ir.OpShr
				inst.Value = 2
				p.optimizationsCount++
				changed = true
			}
			// x / 8 = x >> 3
			if const2 && val2 == 8 {
				inst.Op = ir.OpShr
				inst.Value = 3
				p.optimizationsCount++
				changed = true
			}

		case ir.OpMod:
			// x % 2 = x & 1
			if const2 && val2 == 2 {
				inst.Op = ir.OpAnd
				inst.Value = 1
				p.optimizationsCount++
				changed = true
			}
			// x % 4 = x & 3
			if const2 && val2 == 4 {
				inst.Op = ir.OpAnd
				inst.Value = 3
				p.optimizationsCount++
				changed = true
			}
			// x % 8 = x & 7
			if const2 && val2 == 8 {
				inst.Op = ir.OpAnd
				inst.Value = 7
				p.optimizationsCount++
				changed = true
			}
			// x % 16 = x & 15
			if const2 && val2 == 16 {
				inst.Op = ir.OpAnd
				inst.Value = 15
				p.optimizationsCount++
				changed = true
			}
		}
	}

	return changed
}

// copyPropagation replaces uses of copied values with the original
func (p *MIRPeepholePass) copyPropagation(fn *ir.Function) bool {
	changed := false

	// Track copy relationships: dest -> src
	copies := make(map[ir.Register]ir.Register)

	for i := range fn.Instructions {
		inst := &fn.Instructions[i]

		// Track Move instructions
		if inst.Op == ir.OpMove && inst.Src1 != 0 {
			copies[inst.Dest] = inst.Src1
		}

		// Any write to a register invalidates copies from it
		if inst.Dest != 0 && inst.Op != ir.OpMove {
			delete(copies, inst.Dest)
			// Also invalidate anything that copies from this
			for dest, src := range copies {
				if src == inst.Dest {
					delete(copies, dest)
				}
			}
		}

		// Apply propagation - only count as change if value actually differs
		if inst.Src1 != 0 {
			if src, ok := copies[inst.Src1]; ok && src != inst.Src1 {
				inst.Src1 = src
				p.optimizationsCount++
				changed = true
			}
		}
		if inst.Src2 != 0 {
			if src, ok := copies[inst.Src2]; ok && src != inst.Src2 {
				inst.Src2 = src
				p.optimizationsCount++
				changed = true
			}
		}
	}

	return changed
}

// deadCodeElimination removes instructions whose results are never used
func (p *MIRPeepholePass) deadCodeElimination(fn *ir.Function) bool {
	changed := false

	// Find all used registers
	used := make(map[ir.Register]bool)

	for _, inst := range fn.Instructions {
		if inst.Src1 != 0 {
			used[inst.Src1] = true
		}
		if inst.Src2 != 0 {
			used[inst.Src2] = true
		}
	}

	// Mark instructions for removal
	newInsts := make([]ir.Instruction, 0, len(fn.Instructions))
	for _, inst := range fn.Instructions {
		// Keep if: no dest, dest is used, or has side effects
		if inst.Dest == 0 || used[inst.Dest] || p.hasSideEffects(&inst) {
			newInsts = append(newInsts, inst)
		} else {
			p.optimizationsCount++
			changed = true
		}
	}

	if changed {
		fn.Instructions = newInsts
	}

	return changed
}

// redundantLoadElimination removes redundant loads of the same value
func (p *MIRPeepholePass) redundantLoadElimination(fn *ir.Function) bool {
	changed := false

	// Track last load from each variable
	lastLoad := make(map[string]ir.Register) // Symbol -> register containing value

	for i := range fn.Instructions {
		inst := &fn.Instructions[i]

		// Reset tracking at control flow
		if inst.Op == ir.OpLabel || inst.Op == ir.OpJump || inst.Op == ir.OpJumpIf ||
		   inst.Op == ir.OpJumpIfNot || inst.Op == ir.OpCall || inst.Op == ir.OpReturn {
			lastLoad = make(map[string]ir.Register)
			continue
		}

		// Stores invalidate loads from that location
		if inst.Op == ir.OpStoreVar || inst.Op == ir.OpStoreField {
			delete(lastLoad, inst.Symbol)
			continue
		}

		// Check for redundant load
		if inst.Op == ir.OpLoadVar || inst.Op == ir.OpLoadField {
			if existingReg, ok := lastLoad[inst.Symbol]; ok {
				// Replace with move from existing register
				inst.Op = ir.OpMove
				inst.Src1 = existingReg
				inst.Symbol = ""
				p.optimizationsCount++
				changed = true
			} else {
				lastLoad[inst.Symbol] = inst.Dest
			}
		}
	}

	return changed
}

// hasSideEffects checks if an instruction has side effects
func (p *MIRPeepholePass) hasSideEffects(inst *ir.Instruction) bool {
	switch inst.Op {
	case ir.OpStoreVar, ir.OpStoreField, ir.OpStoreElement, ir.OpStorePtr,
		 ir.OpStoreDirect, ir.OpStoreBitField,
		 ir.OpCall, ir.OpReturn, ir.OpJump, ir.OpJumpIf, ir.OpJumpIfNot,
		 ir.OpLabel, ir.OpAsm, ir.OpPush, ir.OpPop,
		 ir.OpSMCStoreConst, ir.OpTrueSMCPatch, ir.OpTSMCRefPatch,
		 ir.OpPatchPoint, ir.OpPatchTemplate, ir.OpPatchTarget, ir.OpPatchParam,
		 ir.OpSetError:
		return true
	}
	return false
}
