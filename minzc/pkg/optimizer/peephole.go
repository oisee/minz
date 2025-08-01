package optimizer

import (
	"fmt"
	"github.com/minz/minzc/pkg/ir"
)

// PeepholeOptimizationPass performs Z80-specific peephole optimizations
type PeepholeOptimizationPass struct {
	patterns []PeepholePattern
	diagnostic *DiagnosticCollector // Revolutionary diagnostic analysis
}

// PeepholePattern represents a pattern to match and replace
type PeepholePattern struct {
	Name        string
	Match       func([]ir.Instruction, int) (bool, int)
	Replace     func([]ir.Instruction, int) []ir.Instruction
}

// NewPeepholeOptimizationPass creates a new peephole optimization pass
func NewPeepholeOptimizationPass() Pass {
	p := &PeepholeOptimizationPass{
		diagnostic: NewDiagnosticCollector("https://github.com/user/minz-ts"),
	}
	p.initializePatterns()
	return p
}

// Name returns the name of this pass
func (p *PeepholeOptimizationPass) Name() string {
	return "Peephole Optimization"
}

// Run performs peephole optimization on the module
func (p *PeepholeOptimizationPass) Run(module *ir.Module) (bool, error) {
	changed := false
	
	for _, function := range module.Functions {
		if p.optimizeFunction(function) {
			changed = true
		}
	}
	
	// Generate diagnostic report after optimization
	report := p.diagnostic.GenerateReport()
	if len(report) > 100 { // Only print if we have diagnostics
		fmt.Printf("\n📊 Peephole Optimization Report:\n%s\n", report)
	}
	
	return changed, nil
}

// initializePatterns sets up Z80-specific optimization patterns
func (p *PeepholeOptimizationPass) initializePatterns() {
	p.patterns = []PeepholePattern{
		// Pattern: Load 0 -> XOR A,A (faster and smaller)
		{
			Name: "load_zero_to_xor",
			Match: func(insts []ir.Instruction, i int) (bool, int) {
				if i >= len(insts) {
					return false, 0
				}
				inst := &insts[i]
				// Match: LoadConst reg, 0 where reg is 8-bit
				if inst.Op == ir.OpLoadConst && inst.Imm == 0 {
					// TODO: Check if destination is 8-bit register
					return true, 1
				}
				return false, 0
			},
			Replace: func(insts []ir.Instruction, i int) []ir.Instruction {
				inst := &insts[i]
				return []ir.Instruction{
					{
						Op:   ir.OpXor,
						Dest: inst.Dest,
						Src1: inst.Dest,
						Src2: inst.Dest,
						Comment: "XOR A,A (optimized from LD A,0)",
					},
				}
			},
		},
		
		// Pattern: Add 1 -> INC (faster for 8-bit values)
		{
			Name: "add_one_to_inc",
			Match: func(insts []ir.Instruction, i int) (bool, int) {
				if i+1 >= len(insts) {
					return false, 0
				}
				// Match: LoadConst r1, 1; Add r2, r3, r1
				if insts[i].Op == ir.OpLoadConst && insts[i].Imm == 1 {
					if insts[i+1].Op == ir.OpAdd && insts[i+1].Src2 == insts[i].Dest {
						return true, 2
					}
				}
				return false, 0
			},
			Replace: func(insts []ir.Instruction, i int) []ir.Instruction {
				addInst := &insts[i+1]
				return []ir.Instruction{
					{
						Op:   ir.OpInc,
						Dest: addInst.Dest,
						Src1: addInst.Src1,
						Comment: "INC (optimized from ADD 1)",
					},
				}
			},
		},
		
		// Pattern: Sub 1 -> DEC (faster for 8-bit values)
		{
			Name: "sub_one_to_dec",
			Match: func(insts []ir.Instruction, i int) (bool, int) {
				if i+1 >= len(insts) {
					return false, 0
				}
				// Match: LoadConst r1, 1; Sub r2, r3, r1
				if insts[i].Op == ir.OpLoadConst && insts[i].Imm == 1 {
					if insts[i+1].Op == ir.OpSub && insts[i+1].Src2 == insts[i].Dest {
						return true, 2
					}
				}
				return false, 0
			},
			Replace: func(insts []ir.Instruction, i int) []ir.Instruction {
				subInst := &insts[i+1]
				return []ir.Instruction{
					{
						Op:   ir.OpDec,
						Dest: subInst.Dest,
						Src1: subInst.Src1,
						Comment: "DEC (optimized from SUB 1)",
					},
				}
			},
		},
		
		// Pattern: Multiply by power of 2 -> Shift left
		{
			Name: "mul_power2_to_shift",
			Match: func(insts []ir.Instruction, i int) (bool, int) {
				if i+1 >= len(insts) {
					return false, 0
				}
				// Match: LoadConst r1, power_of_2; Mul r2, r3, r1
				if insts[i].Op == ir.OpLoadConst && isPowerOfTwo(insts[i].Imm) {
					if insts[i+1].Op == ir.OpMul && insts[i+1].Src2 == insts[i].Dest {
						return true, 2
					}
				}
				return false, 0
			},
			Replace: func(insts []ir.Instruction, i int) []ir.Instruction {
				constInst := &insts[i]
				mulInst := &insts[i+1]
				shift := countTrailingZeros(constInst.Imm)
				
				return []ir.Instruction{
					{
						Op:   ir.OpLoadConst,
						Dest: constInst.Dest,
						Imm:  int64(shift),
					},
					{
						Op:   ir.OpShl,
						Dest: mulInst.Dest,
						Src1: mulInst.Src1,
						Src2: constInst.Dest,
						Comment: "SHL (optimized from MUL by power of 2)",
					},
				}
			},
		},
		
		// Pattern: Double jump elimination
		{
			Name: "double_jump",
			Match: func(insts []ir.Instruction, i int) (bool, int) {
				if i+2 >= len(insts) {
					return false, 0
				}
				// Match: Jump L1; Label L1; Jump L2
				if insts[i].Op == ir.OpJump &&
				   insts[i+1].Op == ir.OpLabel &&
				   insts[i+1].Label == insts[i].Label &&
				   insts[i+2].Op == ir.OpJump {
					return true, 3
				}
				return false, 0
			},
			Replace: func(insts []ir.Instruction, i int) []ir.Instruction {
				// Replace first jump with jump to final destination
				return []ir.Instruction{
					{
						Op:    ir.OpJump,
						Label: insts[i+2].Label,
						Comment: "Direct jump (optimized double jump)",
					},
				}
			},
		},
		
		// Pattern: Redundant load elimination
		{
			Name: "redundant_load",
			Match: func(insts []ir.Instruction, i int) (bool, int) {
				if i+1 >= len(insts) {
					return false, 0
				}
				// Match: LoadVar r1, x; LoadVar r2, x (same variable)
				if insts[i].Op == ir.OpLoadVar &&
				   insts[i+1].Op == ir.OpLoadVar &&
				   insts[i].Symbol == insts[i+1].Symbol {
					return true, 2
				}
				return false, 0
			},
			Replace: func(insts []ir.Instruction, i int) []ir.Instruction {
				// Replace second load with register copy
				return []ir.Instruction{
					insts[i], // Keep first load
					{
						Op:   ir.OpMove,
						Dest: insts[i+1].Dest,
						Src1: insts[i].Dest,
						Comment: "Register copy (optimized redundant load)",
					},
				}
			},
		},
		
		// Pattern: Load parameter then store to local -> Just use parameter directly
		{
			Name: "param_load_store_elimination",
			Match: func(insts []ir.Instruction, i int) (bool, int) {
				if i+1 >= len(insts) {
					return false, 0
				}
				// Match: LoadParam r1, param; StoreVar local, r1
				if insts[i].Op == ir.OpLoadParam && insts[i+1].Op == ir.OpStoreVar && 
				   insts[i+1].Src1 == insts[i].Dest {
					return true, 2
				}
				return false, 0
			},
			Replace: func(insts []ir.Instruction, i int) []ir.Instruction {
				// Skip the store, just keep the load
				return []ir.Instruction{insts[i]}
			},
		},
		
		// Pattern: Store then immediately load same location -> Keep value in register
		{
			Name: "store_load_elimination", 
			Match: func(insts []ir.Instruction, i int) (bool, int) {
				if i+1 >= len(insts) {
					return false, 0
				}
				// Match: StoreVar addr, r1; LoadVar r2, addr
				if insts[i].Op == ir.OpStoreVar && insts[i+1].Op == ir.OpLoadVar &&
				   insts[i].Dest == insts[i+1].Src1 {
					// Replace load with move
					return true, 2
				}
				return false, 0
			},
			Replace: func(insts []ir.Instruction, i int) []ir.Instruction {
				storeInst := &insts[i]
				loadInst := &insts[i+1]
				return []ir.Instruction{
					*storeInst, // Keep the store
					{
						Op:   ir.OpMove,
						Dest: loadInst.Dest,
						Src1: storeInst.Src1,
						Comment: "Move (optimized from store/load)",
					},
				}
			},
		},
		
		// Pattern: Small offset addition -> INC sequence (your brilliant optimization!)
		{
			Name: "small_offset_to_inc",
			Match: func(insts []ir.Instruction, i int) (bool, int) {
				if i+1 >= len(insts) {
					return false, 0
				}
				// Match: LoadConst reg_offset, small_const; Add reg_ptr, reg_ptr, reg_offset
				if insts[i].Op == ir.OpLoadConst && 
				   insts[i].Imm >= 1 && insts[i].Imm <= 3 { // Only optimize 1-3 (performance sweet spot)
					if insts[i+1].Op == ir.OpAdd && 
					   insts[i+1].Src2 == insts[i].Dest &&  // Using the loaded constant
					   insts[i+1].Src1 == insts[i+1].Dest { // Adding to same register (ptr += offset)
						return true, 2
					}
				}
				return false, 0
			},
			Replace: func(insts []ir.Instruction, i int) []ir.Instruction {
				constInst := &insts[i]
				addInst := &insts[i+1]
				offset := int(constInst.Imm)
				
				// Generate INC sequence
				result := make([]ir.Instruction, offset)
				for j := 0; j < offset; j++ {
					result[j] = ir.Instruction{
						Op:      ir.OpInc,
						Dest:    addInst.Dest, // Same destination register
						Src1:    addInst.Dest, // Increment itself
						Comment: fmt.Sprintf("INC (optimized small offset %d/%d)", j+1, offset),
					}
				}
				return result
			},
		},
		
		// SMC to register optimization - convert simple parameter patching to register passing
		{
			Name: "smc_parameter_to_register",
			Match: func(insts []ir.Instruction, i int) (bool, int) {
				// Look for pattern:
				// LOAD_CONST r1, value
				// SMC_PARAM paramName, r1  
				// LOAD_CONST r2, value2
				// SMC_PARAM paramName2, r2
				// CALL function
				
				if i+4 >= len(insts) {
					return false, 0
				}
				
				// Check for simple SMC parameter pattern with constants
				if insts[i].Op == ir.OpLoadConst &&
				   insts[i+1].Op == ir.OpSMCParam &&
				   insts[i+2].Op == ir.OpLoadConst &&
				   insts[i+3].Op == ir.OpSMCParam &&
				   insts[i+4].Op == ir.OpCall {
					
					// Check if parameters are simple constants
					if insts[i].Imm != 0 && insts[i+2].Imm != 0 {
						return true, 5 // Match 5 instructions (including CALL)
					}
				}
				
				return false, 0
			},
			Replace: func(insts []ir.Instruction, i int) []ir.Instruction {
				// Transform SMC parameter setup to direct register loads + register call:
				// This eliminates SMC overhead for simple constant parameters
				
				result := []ir.Instruction{
					{
						Op:       ir.OpLoadConst,
						Dest:     1, // Use virtual register 1 for first param
						Imm:      insts[i].Imm,
						Comment:  "Param 1 (SMC→register)",
					},
					{
						Op:       ir.OpLoadConst, 
						Dest:     2, // Use virtual register 2 for second param
						Imm:      insts[i+2].Imm,
						Comment:  "Param 2 (SMC→register)",
					},
					{
						Op:       ir.OpCall,
						Symbol:   insts[i+4].Symbol, // Keep original call target
						Src1:     1, // First parameter
						Src2:     2, // Second parameter  
						Comment:  "Register call (SMC optimized)",
					},
				}
				
				return result
			},
		},
	}
}

// optimizeFunction performs peephole optimization on a single function
func (p *PeepholeOptimizationPass) optimizeFunction(fn *ir.Function) bool {
	changed := false
	maxIterations := 10
	
	for iteration := 0; iteration < maxIterations; iteration++ {
		iterChanged := false
		i := 0
		
		newInstructions := []ir.Instruction{}
		
		for i < len(fn.Instructions) {
			matched := false
			
			// Try each pattern
			for _, pattern := range p.patterns {
				if match, length := pattern.Match(fn.Instructions, i); match {
					// Collect diagnostic before applying replacement
					optimizedInstructions := fn.Instructions[i:i+length]
					p.diagnostic.CollectDiagnostic(pattern.Name, fn, optimizedInstructions, i)
					
					// Apply the replacement
					replacement := pattern.Replace(fn.Instructions, i)
					newInstructions = append(newInstructions, replacement...)
					i += length
					matched = true
					iterChanged = true
					break
				}
			}
			
			if !matched {
				// No pattern matched, keep original instruction
				newInstructions = append(newInstructions, fn.Instructions[i])
				i++
			}
		}
		
		fn.Instructions = newInstructions
		
		if iterChanged {
			changed = true
		} else {
			// No changes in this iteration, we're done
			break
		}
	}
	
	return changed
}

// Helper functions

func isPowerOfTwo(n int64) bool {
	return n > 0 && (n & (n - 1)) == 0
}

func countTrailingZeros(n int64) int {
	if n == 0 {
		return 64
	}
	count := 0
	for n&1 == 0 {
		count++
		n >>= 1
	}
	return count
}