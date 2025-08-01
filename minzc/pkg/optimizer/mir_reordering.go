package optimizer

import (
	"fmt"
	"github.com/minz/minzc/pkg/ir"
)

// MIRReorderingPass reorders MIR instructions to expose more optimization opportunities
type MIRReorderingPass struct {
	reorderedCount int
	debug         bool
}

// NewMIRReorderingPass creates a new MIR reordering pass
func NewMIRReorderingPass() Pass {
	return &MIRReorderingPass{
		reorderedCount: 0,
		debug:         false,
	}
}

// Name returns the name of this pass  
func (p *MIRReorderingPass) Name() string {
	return "MIR Instruction Reordering"
}

// Run performs instruction reordering on the module
func (p *MIRReorderingPass) Run(module *ir.Module) (bool, error) {
	changed := false
	
	for _, function := range module.Functions {
		if p.reorderInstructions(function) {
			changed = true
		}
	}
	
	if p.debug && changed {
		fmt.Printf("=== MIR REORDERING PASS ===\n")
		fmt.Printf("  Instructions reordered: %d\n", p.reorderedCount)
		fmt.Printf("===============================\n")
	}
	
	return changed, nil
}

// reorderInstructions reorders instructions within a function for better optimization
func (p *MIRReorderingPass) reorderInstructions(fn *ir.Function) bool {
	if len(fn.Instructions) < 2 {
		return false
	}
	
	changed := false
	newInstructions := make([]ir.Instruction, 0, len(fn.Instructions))
	dependencies := p.buildDependencyGraph(fn.Instructions)
	
	// Strategy 1: Group constant loads together
	constantInsts, otherInsts := p.separateConstantLoads(fn.Instructions, dependencies)
	
	// Strategy 2: Move independent operations closer to their consumers
	reorderedOther := p.moveOperationsCloserToConsumers(otherInsts, dependencies)
	
	// Combine results: constants first, then reordered operations
	newInstructions = append(newInstructions, constantInsts...)
	newInstructions = append(newInstructions, reorderedOther...)
	
	// Check if anything changed
	if len(newInstructions) != len(fn.Instructions) {
		changed = true
	} else {
		for i, inst := range newInstructions {
			if !p.instructionsEqual(inst, fn.Instructions[i]) {
				changed = true
				break
			}
		}
	}
	
	if changed {
		fn.Instructions = newInstructions
		p.reorderedCount += len(fn.Instructions)
	}
	
	return changed
}

// buildDependencyGraph analyzes data dependencies between instructions
func (p *MIRReorderingPass) buildDependencyGraph(instructions []ir.Instruction) map[int][]int {
	dependencies := make(map[int][]int) // instruction index -> list of dependent instruction indices
	
	for i, inst := range instructions {
		dependencies[i] = []int{}
		
		// Find instructions that depend on this one (write-after-read)
		for j := i + 1; j < len(instructions); j++ {
			laterInst := instructions[j]
			
			// Check if later instruction reads what this instruction writes
			if p.hasDataDependency(inst, laterInst) {
				dependencies[i] = append(dependencies[i], j)
			}
		}
	}
	
	return dependencies
}

// hasDataDependency checks if inst2 depends on inst1 (inst2 reads what inst1 writes)
func (p *MIRReorderingPass) hasDataDependency(inst1, inst2 ir.Instruction) bool {
	// inst2 depends on inst1 if:
	// - inst1 writes to a register that inst2 reads
	// - inst1 writes to memory that inst2 reads
	// - Both access the same memory location
	
	// Register dependencies
	if inst1.Dest != 0 && (inst2.Src1 == inst1.Dest || inst2.Src2 == inst1.Dest) {
		return true
	}
	
	// Memory dependencies (conservative)
	if p.isMemoryOperation(inst1) && p.isMemoryOperation(inst2) {
		return true // Conservative: assume all memory ops are dependent
	}
	
	// Function call dependencies
	if inst1.Op == ir.OpCall || inst2.Op == ir.OpCall {
		return true // Conservative: calls can have side effects
	}
	
	return false
}

// separateConstantLoads groups constant loads together for better peephole opportunities
func (p *MIRReorderingPass) separateConstantLoads(instructions []ir.Instruction, dependencies map[int][]int) ([]ir.Instruction, []ir.Instruction) {
	constants := []ir.Instruction{}
	others := []ir.Instruction{}
	
	for i, inst := range instructions {
		if inst.Op == ir.OpLoadConst && len(dependencies[i]) > 0 {
			// This is a constant load that other instructions depend on
			constants = append(constants, inst)
		} else {
			others = append(others, inst)
		}
	}
	
	return constants, others
}

// moveOperationsCloserToConsumers reorders operations to be closer to where they're used
func (p *MIRReorderingPass) moveOperationsCloserToConsumers(instructions []ir.Instruction, dependencies map[int][]int) []ir.Instruction {
	// For now, return as-is. This is where more sophisticated reordering would go.
	// We could implement:
	// - Moving loads closer to their uses
	// - Grouping related arithmetic operations
	// - Minimizing register pressure
	
	return instructions
}

// isMemoryOperation checks if an instruction accesses memory
func (p *MIRReorderingPass) isMemoryOperation(inst ir.Instruction) bool {
	switch inst.Op {
	case ir.OpLoad, ir.OpStore, ir.OpLoadVar, ir.OpStoreVar:
		return true
	case ir.OpStoreTSMCRef, ir.OpTSMCRefLoad, ir.OpTSMCRefPatch:
		return true
	default:
		return false
	}
}

// instructionsEqual compares two instructions for equality
func (p *MIRReorderingPass) instructionsEqual(inst1, inst2 ir.Instruction) bool {
	return inst1.Op == inst2.Op &&
		   inst1.Dest == inst2.Dest &&
		   inst1.Src1 == inst2.Src1 &&
		   inst1.Src2 == inst2.Src2 &&
		   inst1.Imm == inst2.Imm &&
		   inst1.Symbol == inst2.Symbol &&
		   inst1.Label == inst2.Label
}

// Advanced reordering strategies that could be implemented:

// Strategy 1: Constant Folding Preparation
// Groups arithmetic operations with constants together so peephole can fold them
func (p *MIRReorderingPass) prepareForConstantFolding(instructions []ir.Instruction) []ir.Instruction {
	// Find patterns like:
	// r1 = 10
	// r2 = some_operation()  
	// r3 = 20
	// r4 = r1 + r3
	//
	// Reorder to:
	// r1 = 10
	// r3 = 20  
	// r4 = r1 + r3  ; Now peephole can see constant folding opportunity
	// r2 = some_operation()
	
	return instructions // Placeholder
}

// Strategy 2: Register Pressure Reduction  
// Reorders to minimize the number of live registers at any point
func (p *MIRReorderingPass) minimizeRegisterPressure(instructions []ir.Instruction) []ir.Instruction {
	// Analyze register lifetimes and reorder to minimize overlaps
	return instructions // Placeholder
}

// Strategy 3: Loop Optimization Preparation
// Moves loop-invariant operations out of loops and groups loop operations
func (p *MIRReorderingPass) prepareLoopOptimizations(instructions []ir.Instruction) []ir.Instruction {
	// Identify loop boundaries and move invariant operations
	return instructions // Placeholder
}

// Strategy 4: SMC Pattern Preparation
// Groups SMC parameter operations to enable better SMC→register conversion
func (p *MIRReorderingPass) prepareSMCOptimizations(instructions []ir.Instruction) []ir.Instruction {
	// Group SMC parameter setups together:
	// r1 = 10
	// SMC_PARAM p1, r1
	// r2 = 20  
	// SMC_PARAM p2, r2
	// CALL func
	//
	// This enables the peephole SMC→register pattern to trigger
	
	return instructions // Placeholder
}