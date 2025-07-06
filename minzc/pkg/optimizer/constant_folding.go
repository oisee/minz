package optimizer

import (
	"github.com/minz/minzc/pkg/ir"
)

// ConstantFoldingPass performs constant folding optimization
type ConstantFoldingPass struct {
	constants map[ir.Register]int64
}

// NewConstantFoldingPass creates a new constant folding pass
func NewConstantFoldingPass() Pass {
	return &ConstantFoldingPass{
		constants: make(map[ir.Register]int64),
	}
}

// Name returns the name of this pass
func (p *ConstantFoldingPass) Name() string {
	return "Constant Folding"
}

// Run performs constant folding on the module
func (p *ConstantFoldingPass) Run(module *ir.Module) (bool, error) {
	changed := false
	
	for _, function := range module.Functions {
		if p.optimizeFunction(function) {
			changed = true
		}
	}
	
	return changed, nil
}

// optimizeFunction performs constant folding on a single function
func (p *ConstantFoldingPass) optimizeFunction(fn *ir.Function) bool {
	changed := false
	p.constants = make(map[ir.Register]int64)
	
	newInstructions := []ir.Instruction{}
	
	for _, inst := range fn.Instructions {
		optimized := false
		
		switch inst.Op {
		case ir.OpLoadConst:
			// Track constant values
			p.constants[inst.Dest] = inst.Imm
			newInstructions = append(newInstructions, inst)
			
		case ir.OpAdd, ir.OpSub, ir.OpMul, ir.OpDiv, ir.OpMod,
			 ir.OpAnd, ir.OpOr, ir.OpXor, ir.OpShl, ir.OpShr:
			// Try to fold binary operations
			if val1, ok1 := p.constants[inst.Src1]; ok1 {
				if val2, ok2 := p.constants[inst.Src2]; ok2 {
					// Both operands are constants - fold them
					result := p.foldBinaryOp(inst.Op, val1, val2)
					newInst := ir.Instruction{
						Op:   ir.OpLoadConst,
						Dest: inst.Dest,
						Imm:  result,
						Comment: "Folded: " + inst.Comment,
					}
					newInstructions = append(newInstructions, newInst)
					p.constants[inst.Dest] = result
					optimized = true
					changed = true
				}
			}
			
			if !optimized {
				newInstructions = append(newInstructions, inst)
				// Result is not constant
				delete(p.constants, inst.Dest)
			}
			
		case ir.OpNeg, ir.OpNot:
			// Try to fold unary operations
			if val, ok := p.constants[inst.Src1]; ok {
				result := p.foldUnaryOp(inst.Op, val)
				newInst := ir.Instruction{
					Op:   ir.OpLoadConst,
					Dest: inst.Dest,
					Imm:  result,
					Comment: "Folded: " + inst.Comment,
				}
				newInstructions = append(newInstructions, newInst)
				p.constants[inst.Dest] = result
				optimized = true
				changed = true
			}
			
			if !optimized {
				newInstructions = append(newInstructions, inst)
				delete(p.constants, inst.Dest)
			}
			
		case ir.OpEq, ir.OpNe, ir.OpLt, ir.OpGt, ir.OpLe, ir.OpGe:
			// Try to fold comparison operations
			if val1, ok1 := p.constants[inst.Src1]; ok1 {
				if val2, ok2 := p.constants[inst.Src2]; ok2 {
					// Both operands are constants - fold them
					result := p.foldComparisonOp(inst.Op, val1, val2)
					newInst := ir.Instruction{
						Op:   ir.OpLoadConst,
						Dest: inst.Dest,
						Imm:  result,
						Comment: "Folded: " + inst.Comment,
					}
					newInstructions = append(newInstructions, newInst)
					p.constants[inst.Dest] = result
					optimized = true
					changed = true
				}
			}
			
			if !optimized {
				newInstructions = append(newInstructions, inst)
				delete(p.constants, inst.Dest)
			}
			
		case ir.OpJumpIfNot:
			// Try to fold conditional jumps with constant conditions
			if val, ok := p.constants[inst.Src1]; ok {
				if val == 0 {
					// Condition is always false - convert to unconditional jump
					newInst := ir.Instruction{
						Op:    ir.OpJump,
						Label: inst.Label,
						Comment: "Always false: " + inst.Comment,
					}
					newInstructions = append(newInstructions, newInst)
					optimized = true
					changed = true
				} else {
					// Condition is always true - remove jump
					optimized = true
					changed = true
				}
			}
			
			if !optimized {
				newInstructions = append(newInstructions, inst)
			}
			
		default:
			// For all other operations, just copy and invalidate destination
			newInstructions = append(newInstructions, inst)
			if inst.Dest != 0 {
				delete(p.constants, inst.Dest)
			}
		}
	}
	
	if changed {
		fn.Instructions = newInstructions
	}
	
	return changed
}

// foldBinaryOp performs constant folding for binary operations
func (p *ConstantFoldingPass) foldBinaryOp(op ir.Opcode, val1, val2 int64) int64 {
	switch op {
	case ir.OpAdd:
		return val1 + val2
	case ir.OpSub:
		return val1 - val2
	case ir.OpMul:
		return val1 * val2
	case ir.OpDiv:
		if val2 != 0 {
			return val1 / val2
		}
		return 0 // Division by zero - should report error
	case ir.OpMod:
		if val2 != 0 {
			return val1 % val2
		}
		return 0
	case ir.OpAnd:
		return val1 & val2
	case ir.OpOr:
		return val1 | val2
	case ir.OpXor:
		return val1 ^ val2
	case ir.OpShl:
		return val1 << uint(val2)
	case ir.OpShr:
		return val1 >> uint(val2)
	default:
		return 0
	}
}

// foldUnaryOp performs constant folding for unary operations
func (p *ConstantFoldingPass) foldUnaryOp(op ir.Opcode, val int64) int64 {
	switch op {
	case ir.OpNeg:
		return -val
	case ir.OpNot:
		if val == 0 {
			return 1
		}
		return 0
	default:
		return 0
	}
}

// foldComparisonOp performs constant folding for comparison operations
func (p *ConstantFoldingPass) foldComparisonOp(op ir.Opcode, val1, val2 int64) int64 {
	switch op {
	case ir.OpEq:
		if val1 == val2 {
			return 1
		}
		return 0
	case ir.OpNe:
		if val1 != val2 {
			return 1
		}
		return 0
	case ir.OpLt:
		if val1 < val2 {
			return 1
		}
		return 0
	case ir.OpGt:
		if val1 > val2 {
			return 1
		}
		return 0
	case ir.OpLe:
		if val1 <= val2 {
			return 1
		}
		return 0
	case ir.OpGe:
		if val1 >= val2 {
			return 1
		}
		return 0
	default:
		return 0
	}
}