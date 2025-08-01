package optimizer

import (
	"fmt"
	"github.com/minz/minzc/pkg/ir"
)

// SmartPeepholePattern includes both pattern matching and intelligent reordering
type SmartPeepholePattern struct {
	Name        string
	Description string
	
	// Advanced pattern matching with look-ahead
	Match func([]ir.Instruction, int) (bool, int, []ReorderOperation)
	
	// Replace with integrated reordering
	Replace func([]ir.Instruction, int, []ReorderOperation) []ir.Instruction
}

// ReorderOperation represents a reordering operation
type ReorderOperation struct {
	Type        ReorderType
	FromIndex   int
	ToIndex     int
	Description string
}

type ReorderType int

const (
	ReorderMove ReorderType = iota // Move instruction from one position to another
	ReorderSwap                    // Swap two instructions
	ReorderGroup                   // Group related instructions together
)

// SmartPeepholeOptimizationPass performs peephole optimization with intelligent reordering
type SmartPeepholeOptimizationPass struct {
	patterns []SmartPeepholePattern
	diagnostic *DiagnosticCollector
}

// NewSmartPeepholeOptimizationPass creates a new smart peephole pass
func NewSmartPeepholeOptimizationPass() Pass {
	return &SmartPeepholeOptimizationPass{
		patterns: createSmartPeepholePatterns(),
		diagnostic: NewDiagnosticCollector("minz-compiler"),
	}
}

// Name returns the name of this pass
func (p *SmartPeepholeOptimizationPass) Name() string {
	return "Smart Peephole Optimization (with Reordering)"
}

// Run performs smart peephole optimization with reordering
func (p *SmartPeepholeOptimizationPass) Run(module *ir.Module) (bool, error) {
	changed := false
	
	for _, function := range module.Functions {
		if p.optimizeFunction(function) {
			changed = true
		}
	}
	
	return changed, nil
}

// createSmartPeepholePatterns creates patterns that can reorder instructions
func createSmartPeepholePatterns() []SmartPeepholePattern {
	return []SmartPeepholePattern{
		
		// Pattern 1: Constant Folding with Reordering
		{
			Name: "constant_folding_with_reorder",
			Description: "Group constants together and fold arithmetic operations",
			Match: func(insts []ir.Instruction, i int) (bool, int, []ReorderOperation) {
				// Look for pattern:
				// r1 = 10        (constant)
				// r2 = some_op() (intervening operation) 
				// r3 = 20        (constant - separated!)
				// r4 = r1 + r3   (arithmetic using both constants)
				
				if i+3 >= len(insts) {
					return false, 0, nil
				}
				
				// Check for separated constants pattern
				if insts[i].Op == ir.OpLoadConst &&
				   insts[i+1].Op != ir.OpLoadConst && // Intervening operation
				   insts[i+2].Op == ir.OpLoadConst &&
				   insts[i+3].Op == ir.OpAdd &&
				   (insts[i+3].Src1 == insts[i].Dest || insts[i+3].Src2 == insts[i].Dest) &&
				   (insts[i+3].Src1 == insts[i+2].Dest || insts[i+3].Src2 == insts[i+2].Dest) &&
				   isSafeToReorderInst(insts[i+1]) { // Only reorder if intervening op is safe
					
					// We can reorder to group constants together
					reorderings := []ReorderOperation{
						{
							Type: ReorderMove,
							FromIndex: i+2, // Move second constant
							ToIndex: i+1,   // Right after first constant
							Description: "Group constants for folding",
						},
					}
					
					return true, 4, reorderings
				}
				
				return false, 0, nil
			},
			Replace: func(insts []ir.Instruction, i int, reorders []ReorderOperation) []ir.Instruction {
				// After reordering, we have:
				// r1 = 10     (constant 1)
				// r3 = 20     (constant 2 - now adjacent!)
				// r2 = some_op() (moved down)
				// r4 = r1 + r3   (arithmetic)
				
				// We can now fold the constants if they're both used in arithmetic
				const1 := insts[i].Imm
				const2 := insts[i+2].Imm // After reordering
				foldedValue := const1 + const2 // Simple addition folding
				
				return []ir.Instruction{
					// Keep the intervening operation
					insts[i+1], // some_op()
					// Replace constant arithmetic with folded constant
					{
						Op: ir.OpLoadConst,
						Dest: insts[i+3].Dest, // Result register
						Imm: foldedValue,
						Comment: fmt.Sprintf("Constant folded: %d + %d = %d", const1, const2, foldedValue),
					},
				}
			},
		},
		
		// Pattern 2: SMC Parameter Grouping with Register Conversion
		{
			Name: "smc_parameter_grouping",
			Description: "Group SMC parameters and convert to register passing when beneficial",
			Match: func(insts []ir.Instruction, i int) (bool, int, []ReorderOperation) {
				// Look for scattered SMC parameter setup:
				// r1 = 10
				// some_operation()     (intervening)
				// SMC_PARAM p1, r1
				// r2 = 20
				// another_operation()  (intervening)
				// SMC_PARAM p2, r2
				// CALL func
				
				if i+6 >= len(insts) {
					return false, 0, nil
				}
				
				// Check for scattered SMC pattern
				if insts[i].Op == ir.OpLoadConst &&
				   insts[i+2].Op == ir.OpSMCParam &&
				   insts[i+3].Op == ir.OpLoadConst &&
				   insts[i+5].Op == ir.OpSMCParam &&
				   insts[i+6].Op == ir.OpCall {
					
					// Reorder to group SMC operations
					reorderings := []ReorderOperation{
						{
							Type: ReorderGroup,
							FromIndex: i,
							ToIndex: i+4, // Group all SMC-related operations
							Description: "Group SMC parameters for optimization",
						},
					}
					
					return true, 7, reorderings
				}
				
				return false, 0, nil
			},
			Replace: func(insts []ir.Instruction, i int, reorders []ReorderOperation) []ir.Instruction {
				// After grouping:
				// r1 = 10
				// SMC_PARAM p1, r1
				// r2 = 20  
				// SMC_PARAM p2, r2
				// CALL func
				// ... (other operations moved)
				
				// Now we can apply SMC→register conversion
				return []ir.Instruction{
					{
						Op: ir.OpLoadConst,
						Dest: 1, // Register 1 for first param
						Imm: insts[i].Imm,
						Comment: "Param 1 (SMC→register via reordering)",
					},
					{
						Op: ir.OpLoadConst,
						Dest: 2, // Register 2 for second param
						Imm: insts[i+3].Imm,
						Comment: "Param 2 (SMC→register via reordering)",
					},
					{
						Op: ir.OpCall,
						Symbol: insts[i+6].Symbol,
						Src1: 1,
						Src2: 2,
						Comment: "Register call (optimized via reordering)",
					},
					// Include moved operations
					insts[i+1], // first intervening operation
					insts[i+4], // second intervening operation
				}
			},
		},
		
		// Pattern 3: Loop Invariant Motion
		{
			Name: "loop_invariant_reordering", 
			Description: "Move loop-invariant operations out of loops via reordering",
			Match: func(insts []ir.Instruction, i int) (bool, int, []ReorderOperation) {
				// Look for constant loads inside loops that could be moved out
				// This is a simplified example - real implementation would need loop analysis
				
				// Find loop pattern: label, operations, jump back
				if i > 0 && insts[i-1].Op == ir.OpLabel && // Loop start
				   insts[i].Op == ir.OpLoadConst &&        // Constant inside loop
				   i+2 < len(insts) && insts[i+2].Op == ir.OpJump { // Jump back
					
					reorderings := []ReorderOperation{
						{
							Type: ReorderMove,
							FromIndex: i,   // Move constant load
							ToIndex: i-1,   // Before loop start
							Description: "Move loop-invariant constant out of loop",
						},
					}
					
					return true, 1, reorderings
				}
				
				return false, 0, nil
			},
			Replace: func(insts []ir.Instruction, i int, reorders []ReorderOperation) []ir.Instruction {
				// Constant has been moved out of loop - just return the original instruction
				// (the reordering was already applied during matching)
				return []ir.Instruction{insts[i]}
			},
		},
	}
}

// optimizeFunction performs smart peephole optimization with reordering on a single function
func (p *SmartPeepholeOptimizationPass) optimizeFunction(fn *ir.Function) bool {
	changed := false
	maxIterations := 10
	
	for iteration := 0; iteration < maxIterations; iteration++ {
		iterChanged := false
		
		newInstructions := []ir.Instruction{}
		i := 0
		
		for i < len(fn.Instructions) {
			matched := false
			
			// Try each smart pattern
			for _, pattern := range p.patterns {
				if canMatch, length, reorderings := pattern.Match(fn.Instructions, i); canMatch {
					// Apply reordering if needed
					if len(reorderings) > 0 {
						fn.Instructions = p.applyReordering(fn.Instructions, reorderings)
					}
					
					// Apply the replacement
					replacement := pattern.Replace(fn.Instructions, i, reorderings)
					newInstructions = append(newInstructions, replacement...)
					
					// Collect diagnostic
					p.diagnostic.CollectDiagnostic(pattern.Name, fn, fn.Instructions[i:i+length], i)
					
					i += length
					matched = true
					iterChanged = true
					break
				}
			}
			
			if !matched {
				newInstructions = append(newInstructions, fn.Instructions[i])
				i++
			}
		}
		
		fn.Instructions = newInstructions
		changed = changed || iterChanged
		
		if !iterChanged {
			break // No more changes
		}
	}
	
	return changed
}

// applyReordering applies reordering operations to instruction list
func (p *SmartPeepholeOptimizationPass) applyReordering(instructions []ir.Instruction, reorderings []ReorderOperation) []ir.Instruction {
	result := make([]ir.Instruction, len(instructions))
	copy(result, instructions)
	
	for _, reorder := range reorderings {
		switch reorder.Type {
		case ReorderMove:
			// Move instruction from FromIndex to ToIndex
			if reorder.FromIndex < len(result) && reorder.ToIndex < len(result) {
				moved := result[reorder.FromIndex]
				// Remove from old position
				result = append(result[:reorder.FromIndex], result[reorder.FromIndex+1:]...)
				// Insert at new position
				if reorder.ToIndex >= len(result) {
					result = append(result, moved)
				} else {
					result = append(result[:reorder.ToIndex], append([]ir.Instruction{moved}, result[reorder.ToIndex:]...)...)
				}
			}
		case ReorderSwap:
			// Swap two instructions
			if reorder.FromIndex < len(result) && reorder.ToIndex < len(result) {
				result[reorder.FromIndex], result[reorder.ToIndex] = result[reorder.ToIndex], result[reorder.FromIndex]
			}
		case ReorderGroup:
			// Group related instructions together (simplified implementation)
			// Real implementation would be more sophisticated
		}
	}
	
	return result
}

// isSafeToReorderInst checks if an instruction can be safely reordered (standalone function)
func isSafeToReorderInst(inst ir.Instruction) bool {
	// Instructions with side effects that prevent reordering
	switch inst.Op {
	case ir.OpCall, ir.OpReturn:
		return false // Function calls have unknown side effects
	case ir.OpStore, ir.OpStoreVar, ir.OpStoreTSMCRef:
		return false // Memory writes can alias
	case ir.OpLoad, ir.OpLoadVar: 
		return false // Memory reads might be volatile
	case ir.OpJump, ir.OpJumpIfNot, ir.OpLabel:
		return false // Control flow must be preserved
	case ir.OpSMCParam, ir.OpSMCUpdate, ir.OpTSMCRefPatch:
		return false // SMC operations have ordering requirements
	}
	
	// Safe operations that can be reordered
	switch inst.Op {
	case ir.OpLoadConst, ir.OpMove:
		return true // Pure register operations
	case ir.OpAdd, ir.OpSub, ir.OpMul, ir.OpDiv:
		return true // Pure arithmetic (assuming no flag dependencies)
	case ir.OpAnd, ir.OpOr, ir.OpXor:
		return true // Pure bitwise operations
	case ir.OpNop:
		return true // No operation
	default:
		return false // Conservative: unknown operations are unsafe
	}
}