package optimizer

import (
	"github.com/minz/minzc/pkg/ir"
)

// OptimizeTailRecursion converts tail recursive calls into jumps
func OptimizeTailRecursion(fn *ir.Function) bool {
	if !fn.IsRecursive {
		return false
	}

	optimized := false
	
	for i := 0; i < len(fn.Instructions)-1; i++ {
		inst := &fn.Instructions[i]
		nextInst := &fn.Instructions[i+1]
		
		// Look for pattern: CALL self followed by RETURN
		if inst.Op == ir.OpCall && inst.Symbol == fn.Name &&
		   nextInst.Op == ir.OpReturn && nextInst.Src1 == inst.Dest {
			// This is a tail recursive call!
			fn.HasTailRecursion = true
			
			// Convert to tail recursion:
			// 1. Update parameters with new values
			// 2. Jump to function start instead of CALL
			
			// Replace CALL with OpJump
			inst.Op = ir.OpJump
			inst.Label = fn.Name + "_start"
			inst.Comment = "Tail recursion optimized"
			
			// Remove the RETURN instruction
			fn.Instructions = append(fn.Instructions[:i+1], fn.Instructions[i+2:]...)
			
			optimized = true
		}
	}
	
	// If we found tail recursion, add a label at the start
	if fn.HasTailRecursion {
		// Insert label at the beginning of function body
		labelInst := ir.Instruction{
			Op:    ir.OpLabel,
			Label: fn.Name + "_start",
		}
		
		// Find where to insert (after parameter setup)
		insertPos := 0
		for i, inst := range fn.Instructions {
			if inst.Op != ir.OpLoadParam && inst.Op != ir.OpSMCParam {
				insertPos = i
				break
			}
		}
		
		// Insert the label
		fn.Instructions = append(fn.Instructions[:insertPos], 
			append([]ir.Instruction{labelInst}, fn.Instructions[insertPos:]...)...)
	}
	
	return optimized
}

// IsTailRecursive checks if a function has any tail recursive calls
func IsTailRecursive(fn *ir.Function) bool {
	if !fn.IsRecursive {
		return false
	}
	
	for i := 0; i < len(fn.Instructions)-1; i++ {
		inst := &fn.Instructions[i]
		nextInst := &fn.Instructions[i+1]
		
		// Look for pattern: CALL self followed by RETURN
		if inst.Op == ir.OpCall && inst.Symbol == fn.Name &&
		   nextInst.Op == ir.OpReturn && nextInst.Src1 == inst.Dest {
			return true
		}
	}
	
	return false
}