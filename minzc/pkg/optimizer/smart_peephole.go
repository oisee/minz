package optimizer

import (
	"github.com/minz/minzc/pkg/ir"
)

// SmartPeepholeOptimizationPass combines instruction reordering with peephole optimization
// This pass:
// 1. Reorders instructions to expose patterns
// 2. Applies Z80-specific peephole optimizations
// 3. Handles complex multi-instruction patterns
type SmartPeepholeOptimizationPass struct {
	name      string
	reorderer *MIRReorderingPass
}

// NewSmartPeepholeOptimizationPass creates a new smart peephole optimization pass
func NewSmartPeepholeOptimizationPass() *SmartPeepholeOptimizationPass {
	return &SmartPeepholeOptimizationPass{
		name:      "Smart Peephole Optimization",
		reorderer: NewMIRReorderingPass(),
	}
}

// Name returns the name of the pass
func (p *SmartPeepholeOptimizationPass) Name() string {
	return p.name
}

// Run executes the smart peephole optimization pass
func (p *SmartPeepholeOptimizationPass) Run(module *ir.Module) (bool, error) {
	changed := false
	
	// First pass: reorder instructions
	reordered, err := p.reorderer.Run(module)
	if err != nil {
		return false, err
	}
	changed = changed || reordered
	
	// Second pass: apply peephole patterns
	for _, fn := range module.Functions {
		if p.optimizeFunction(fn) {
			changed = true
		}
	}
	
	// Third pass: cleanup redundant instructions after optimization
	for _, fn := range module.Functions {
		if p.cleanupFunction(fn) {
			changed = true
		}
	}
	
	return changed, nil
}

// Pattern represents a peephole optimization pattern
type Pattern struct {
	name      string
	matcher   func([]ir.Instruction, int) (bool, int) // Returns match and length
	optimizer func([]ir.Instruction, int) []ir.Instruction
}

// optimizeFunction applies peephole optimizations to a function
func (p *SmartPeepholeOptimizationPass) optimizeFunction(fn *ir.Function) bool {
	if len(fn.Instructions) == 0 {
		return false
	}
	
	changed := false
	patterns := p.getPatterns()
	
	// Apply patterns repeatedly until no more changes
	for {
		localChanged := false
		newInstructions := []ir.Instruction{}
		
		i := 0
		for i < len(fn.Instructions) {
			matched := false
			
			// Try each pattern
			for _, pattern := range patterns {
				if matches, length := pattern.matcher(fn.Instructions, i); matches {
					// Apply optimization
					optimized := pattern.optimizer(fn.Instructions, i)
					newInstructions = append(newInstructions, optimized...)
					i += length
					matched = true
					localChanged = true
					break
				}
			}
			
			if !matched {
				// No pattern matched, keep instruction
				newInstructions = append(newInstructions, fn.Instructions[i])
				i++
			}
		}
		
		fn.Instructions = newInstructions
		changed = changed || localChanged
		
		if !localChanged {
			break
		}
	}
	
	return changed
}

// getPatterns returns all peephole optimization patterns
func (p *SmartPeepholeOptimizationPass) getPatterns() []Pattern {
	return []Pattern{
		// Load-Store elimination
		{
			name: "redundant-load-store",
			matcher: func(insts []ir.Instruction, i int) (bool, int) {
				if i+1 >= len(insts) {
					return false, 0
				}
				// Pattern: load r1, var; store var, r1
				inst1, inst2 := &insts[i], &insts[i+1]
				return inst1.Op == ir.OpLoadVar &&
					inst2.Op == ir.OpStoreVar &&
					inst1.Symbol == inst2.Symbol &&
					inst1.Dest == inst2.Src1, 2
			},
			optimizer: func(insts []ir.Instruction, i int) []ir.Instruction {
				// Eliminate both instructions
				return []ir.Instruction{}
			},
		},
		
		// Constant folding for arithmetic
		{
			name: "constant-arithmetic",
			matcher: func(insts []ir.Instruction, i int) (bool, int) {
				if i+2 >= len(insts) {
					return false, 0
				}
				// Pattern: load r1, const1; load r2, const2; add r3, r1, r2
				inst1, inst2, inst3 := &insts[i], &insts[i+1], &insts[i+2]
				if inst1.Op != ir.OpLoadConst || inst2.Op != ir.OpLoadConst {
					return false, 0
				}
				if inst3.Op != ir.OpAdd && inst3.Op != ir.OpSub &&
					inst3.Op != ir.OpMul && inst3.Op != ir.OpAnd &&
					inst3.Op != ir.OpOr && inst3.Op != ir.OpXor {
					return false, 0
				}
				return inst3.Src1 == inst1.Dest && inst3.Src2 == inst2.Dest, 3
			},
			optimizer: func(insts []ir.Instruction, i int) []ir.Instruction {
				inst1, inst2, inst3 := &insts[i], &insts[i+1], &insts[i+2]
				
				// Compute result at compile time
				var result int64
				switch inst3.Op {
				case ir.OpAdd:
					result = inst1.Imm + inst2.Imm
				case ir.OpSub:
					result = inst1.Imm - inst2.Imm
				case ir.OpMul:
					result = inst1.Imm * inst2.Imm
				case ir.OpAnd:
					result = inst1.Imm & inst2.Imm
				case ir.OpOr:
					result = inst1.Imm | inst2.Imm
				case ir.OpXor:
					result = inst1.Imm ^ inst2.Imm
				}
				
				// Replace with single load constant
				return []ir.Instruction{
					{
						Op:   ir.OpLoadConst,
						Dest: inst3.Dest,
						Imm:  result,
					},
				}
			},
		},
		
		// Increment/Decrement optimization
		{
			name: "increment-optimization",
			matcher: func(insts []ir.Instruction, i int) (bool, int) {
				if i+1 >= len(insts) {
					return false, 0
				}
				// Pattern: load r2, n; add r1, r1, r2
				inst1, inst2 := &insts[i], &insts[i+1]
				if inst1.Op != ir.OpLoadConst || inst2.Op != ir.OpAdd {
					return false, 0
				}
				if inst2.Src1 != inst2.Dest || inst2.Src2 != inst1.Dest {
					return false, 0
				}
				
				// Check if INC/DEC is beneficial based on register and value
				absVal := inst1.Imm
				if absVal < 0 {
					absVal = -absVal
				}
				
				// For now, we don't know the actual register mapping
				// So we use conservative approach: up to ±3 for most cases
				// TODO: Get actual register info from register allocator
				return absVal >= 1 && absVal <= 3, 2
			},
			optimizer: func(insts []ir.Instruction, i int) []ir.Instruction {
				inst1, inst2 := &insts[i], &insts[i+1]
				
				// Generate multiple inc/dec instructions
				result := []ir.Instruction{}
				count := inst1.Imm
				if count < 0 {
					count = -count
				}
				
				op := ir.OpInc
				if inst1.Imm < 0 {
					op = ir.OpDec
				}
				
				for j := int64(0); j < count; j++ {
					result = append(result, ir.Instruction{
						Op:   op,
						Dest: inst2.Dest,
						Src1: inst2.Dest,
					})
				}
				
				return result
			},
		},
		
		// Double negation elimination
		{
			name: "double-negation",
			matcher: func(insts []ir.Instruction, i int) (bool, int) {
				if i+1 >= len(insts) {
					return false, 0
				}
				inst1, inst2 := &insts[i], &insts[i+1]
				return inst1.Op == ir.OpNot &&
					inst2.Op == ir.OpNot &&
					inst2.Src1 == inst1.Dest, 2
			},
			optimizer: func(insts []ir.Instruction, i int) []ir.Instruction {
				// Eliminate both negations
				return []ir.Instruction{}
			},
		},
		
		// Zero comparison optimization
		{
			name: "zero-comparison",
			matcher: func(insts []ir.Instruction, i int) (bool, int) {
				if i+2 >= len(insts) {
					return false, 0
				}
				// Pattern: load r2, 0; cmp r3, r1, r2
				inst1, inst2 := &insts[i], &insts[i+1]
				return inst1.Op == ir.OpLoadConst &&
					inst1.Imm == 0 &&
					(inst2.Op == ir.OpEq || inst2.Op == ir.OpNe ||
						inst2.Op == ir.OpLt || inst2.Op == ir.OpGt) &&
					inst2.Src2 == inst1.Dest, 2
			},
			optimizer: func(insts []ir.Instruction, i int) []ir.Instruction {
				inst2 := &insts[i+1]
				
				// Use test instruction (sets flags without compare)
				return []ir.Instruction{
					{
						Op:   ir.OpTest,
						Src1: inst2.Src1,
						Dest: inst2.Dest,
					},
				}
			},
		},
		
		// Shift-by-constant optimization
		{
			name: "shift-constant",
			matcher: func(insts []ir.Instruction, i int) (bool, int) {
				if i+1 >= len(insts) {
					return false, 0
				}
				inst1, inst2 := &insts[i], &insts[i+1]
				return inst1.Op == ir.OpLoadConst &&
					inst1.Imm >= 0 && inst1.Imm <= 7 &&
					(inst2.Op == ir.OpShl || inst2.Op == ir.OpShr) &&
					inst2.Src2 == inst1.Dest, 2
			},
			optimizer: func(insts []ir.Instruction, i int) []ir.Instruction {
				inst1, inst2 := &insts[i], &insts[i+1]
				
				// For small shifts, unroll into multiple single shifts
				// (often faster on Z80)
				result := []ir.Instruction{}
				for j := int64(0); j < inst1.Imm; j++ {
					result = append(result, ir.Instruction{
						Op:   inst2.Op,
						Dest: inst2.Dest,
						Src1: inst2.Src1,
						Imm:  1,
					})
				}
				return result
			},
		},
		
		// Multiply by power of 2
		{
			name: "multiply-power-of-2",
			matcher: func(insts []ir.Instruction, i int) (bool, int) {
				if i+2 >= len(insts) {
					return false, 0
				}
				inst1, inst2 := &insts[i], &insts[i+1]
				if inst1.Op != ir.OpLoadConst || inst2.Op != ir.OpMul {
					return false, 0
				}
				// Check if constant is power of 2
				val := inst1.Imm
				return val > 0 && (val&(val-1)) == 0 && inst2.Src2 == inst1.Dest, 2
			},
			optimizer: func(insts []ir.Instruction, i int) []ir.Instruction {
				inst1, inst2 := &insts[i], &insts[i+1]
				
				// Count trailing zeros to get shift amount
				shifts := 0
				val := inst1.Imm
				for val > 1 {
					val >>= 1
					shifts++
				}
				
				// Replace with shift
				return []ir.Instruction{
					{
						Op:   ir.OpLoadConst,
						Dest: inst1.Dest,
						Imm:  int64(shifts),
					},
					{
						Op:   ir.OpShl,
						Dest: inst2.Dest,
						Src1: inst2.Src1,
						Src2: inst1.Dest,
					},
				}
			},
		},
	}
}

// cleanupFunction removes redundant instructions after optimization
func (p *SmartPeepholeOptimizationPass) cleanupFunction(fn *ir.Function) bool {
	if len(fn.Instructions) == 0 {
		return false
	}
	
	changed := false
	
	// Remove dead stores
	uses := make(map[ir.Register]int)
	for _, inst := range fn.Instructions {
		if inst.Src1 != 0 {
			uses[inst.Src1]++
		}
		if inst.Src2 != 0 {
			uses[inst.Src2]++
		}
	}
	
	newInstructions := []ir.Instruction{}
	for _, inst := range fn.Instructions {
		// Skip dead stores
		if (inst.Op == ir.OpStoreVar || inst.Op == ir.OpStoreField) && uses[inst.Dest] == 0 {
			changed = true
			continue
		}
		
		// Skip loads to unused registers
		if inst.Op == ir.OpLoadConst || inst.Op == ir.OpLoadVar {
			if uses[inst.Dest] == 0 {
				changed = true
				continue
			}
		}
		
		newInstructions = append(newInstructions, inst)
	}
	
	fn.Instructions = newInstructions
	return changed
}