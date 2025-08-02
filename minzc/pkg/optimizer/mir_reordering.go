package optimizer

import (
	"github.com/minz/minzc/pkg/ir"
)

// MIRReorderingPass implements instruction reordering optimizations at the MIR level
// This pass reorders instructions to:
// 1. Expose more optimization opportunities
// 2. Reduce register pressure
// 3. Enable better peephole patterns
// 4. Minimize memory accesses
type MIRReorderingPass struct {
	name string
}

// NewMIRReorderingPass creates a new MIR reordering optimization pass
func NewMIRReorderingPass() *MIRReorderingPass {
	return &MIRReorderingPass{
		name: "MIR Instruction Reordering",
	}
}

// Name returns the name of the pass
func (p *MIRReorderingPass) Name() string {
	return p.name
}

// Run executes the MIR reordering optimization pass
func (p *MIRReorderingPass) Run(module *ir.Module) (bool, error) {
	changed := false
	
	for _, fn := range module.Functions {
		if p.optimizeFunction(fn) {
			changed = true
		}
	}
	
	return changed, nil
}

// optimizeFunction reorders instructions within a function
func (p *MIRReorderingPass) optimizeFunction(fn *ir.Function) bool {
	if len(fn.Instructions) < 2 {
		return false
	}
	
	changed := false
	
	// Build dependency graph
	deps := p.buildDependencyGraph(fn)
	
	// Apply reordering strategies
	if p.reorderIndependentLoads(fn, deps) {
		changed = true
	}
	
	if p.hoistInvariantCode(fn, deps) {
		changed = true
	}
	
	if p.sinkStores(fn, deps) {
		changed = true
	}
	
	if p.clusterRelatedOps(fn, deps) {
		changed = true
	}
	
	return changed
}

// Dependency tracking
type dependency struct {
	reads  map[ir.Register][]int // Register -> instruction indices that read it
	writes map[ir.Register][]int // Register -> instruction indices that write it
	memory []int                 // Instructions that access memory
}

// buildDependencyGraph analyzes data dependencies between instructions
func (p *MIRReorderingPass) buildDependencyGraph(fn *ir.Function) *dependency {
	deps := &dependency{
		reads:  make(map[ir.Register][]int),
		writes: make(map[ir.Register][]int),
		memory: []int{},
	}
	
	for i, inst := range fn.Instructions {
		// Track register reads
		if inst.Src1 != 0 {
			deps.reads[inst.Src1] = append(deps.reads[inst.Src1], i)
		}
		if inst.Src2 != 0 {
			deps.reads[inst.Src2] = append(deps.reads[inst.Src2], i)
		}
		
		// Track register writes
		if inst.Dest != 0 {
			deps.writes[inst.Dest] = append(deps.writes[inst.Dest], i)
		}
		
		// Track memory operations
		switch inst.Op {
		case ir.OpLoadVar, ir.OpStoreVar, ir.OpLoadField, ir.OpStoreField,
			 ir.OpLoadElement, ir.OpStoreElement, ir.OpCall:
			deps.memory = append(deps.memory, i)
		}
	}
	
	return deps
}

// canReorder checks if two instructions can be safely reordered
func (p *MIRReorderingPass) canReorder(inst1, inst2 *ir.Instruction, idx1, idx2 int, deps *dependency) bool {
	// Can't reorder if inst2 reads what inst1 writes
	if inst1.Dest != 0 {
		for _, readIdx := range deps.reads[inst1.Dest] {
			if readIdx == idx2 {
				return false
			}
		}
	}
	
	// Can't reorder if inst1 reads what inst2 writes
	if inst2.Dest != 0 {
		if inst1.Src1 == inst2.Dest || inst1.Src2 == inst2.Dest {
			return false
		}
	}
	
	// Can't reorder memory operations (conservative)
	isMemOp1 := isMemoryOp(inst1)
	isMemOp2 := isMemoryOp(inst2)
	if isMemOp1 && isMemOp2 {
		return false
	}
	
	// Can't reorder control flow
	if isControlFlow(inst1) || isControlFlow(inst2) {
		return false
	}
	
	return true
}

// reorderIndependentLoads moves independent loads together to enable better scheduling
func (p *MIRReorderingPass) reorderIndependentLoads(fn *ir.Function, deps *dependency) bool {
	changed := false
	
	// Group consecutive loads that don't depend on each other
	for i := 0; i < len(fn.Instructions)-1; {
		inst1 := &fn.Instructions[i]
		
		// Look for a load instruction
		if inst1.Op != ir.OpLoadVar && inst1.Op != ir.OpLoadField {
			i++
			continue
		}
		
		// Find next load that we can move adjacent
		for j := i + 2; j < len(fn.Instructions) && j < i+5; j++ {
			inst2 := &fn.Instructions[j]
			
			if inst2.Op != ir.OpLoadVar && inst2.Op != ir.OpLoadField {
				continue
			}
			
			// Check if we can move inst2 to be right after inst1
			canMove := true
			for k := i + 1; k < j; k++ {
				if !p.canReorder(&fn.Instructions[k], inst2, k, j, deps) {
					canMove = false
					break
				}
			}
			
			if canMove {
				// Move inst2 to position i+1
				temp := fn.Instructions[j]
				copy(fn.Instructions[i+2:j+1], fn.Instructions[i+1:j])
				fn.Instructions[i+1] = temp
				changed = true
				break
			}
		}
		
		i++
	}
	
	return changed
}

// hoistInvariantCode moves loop-invariant code out of loops
func (p *MIRReorderingPass) hoistInvariantCode(fn *ir.Function, deps *dependency) bool {
	// For now, just identify simple patterns where calculations can be moved up
	changed := false
	
	// Find basic blocks (simplified - just use labels and jumps)
	blocks := p.findBasicBlocks(fn)
	
	for _, block := range blocks {
		if block.isLoop {
			// Move invariant instructions to loop preheader
			for i := block.start; i < block.end; i++ {
				inst := &fn.Instructions[i]
				
				if p.isLoopInvariant(inst, block, deps) {
					// TODO: Actually move the instruction
					// For now, we just identify opportunities
				}
			}
		}
	}
	
	return changed
}

// sinkStores moves stores as late as possible to reduce register pressure
func (p *MIRReorderingPass) sinkStores(fn *ir.Function, deps *dependency) bool {
	changed := false
	
	for i := 0; i < len(fn.Instructions)-1; i++ {
		inst := &fn.Instructions[i]
		
		if inst.Op != ir.OpStoreVar && inst.Op != ir.OpStoreField {
			continue
		}
		
		// Find the latest position we can move this store
		latestPos := i
		for j := i + 1; j < len(fn.Instructions); j++ {
			// Stop if we hit a use of the stored location
			if p.usesMemoryLocation(&fn.Instructions[j], inst) {
				break
			}
			
			// Stop if we hit another store to the same location
			if p.storesSameLocation(&fn.Instructions[j], inst) {
				break
			}
			
			// Stop at control flow
			if isControlFlow(&fn.Instructions[j]) {
				break
			}
			
			latestPos = j
		}
		
		if latestPos > i {
			// Move store to latest position
			temp := fn.Instructions[i]
			copy(fn.Instructions[i:latestPos], fn.Instructions[i+1:latestPos+1])
			fn.Instructions[latestPos] = temp
			changed = true
		}
	}
	
	return changed
}

// clusterRelatedOps groups related operations together for better peephole optimization
func (p *MIRReorderingPass) clusterRelatedOps(fn *ir.Function, deps *dependency) bool {
	changed := false
	
	// Group arithmetic operations on the same registers
	for i := 0; i < len(fn.Instructions)-1; i++ {
		inst1 := &fn.Instructions[i]
		
		if !isArithmetic(inst1) {
			continue
		}
		
		// Look for related arithmetic ops we can cluster
		for j := i + 2; j < len(fn.Instructions) && j < i+8; j++ {
			inst2 := &fn.Instructions[j]
			
			if !isArithmetic(inst2) {
				continue
			}
			
			// Check if they operate on related registers
			if p.areRelatedOps(inst1, inst2) {
				// Check if we can move them together
				canMove := true
				for k := i + 1; k < j; k++ {
					if !p.canReorder(&fn.Instructions[k], inst2, k, j, deps) {
						canMove = false
						break
					}
				}
				
				if canMove {
					// Move inst2 closer to inst1
					temp := fn.Instructions[j]
					copy(fn.Instructions[i+2:j+1], fn.Instructions[i+1:j])
					fn.Instructions[i+1] = temp
					changed = true
					break
				}
			}
		}
	}
	
	return changed
}

// Helper functions

func isMemoryOp(inst *ir.Instruction) bool {
	switch inst.Op {
	case ir.OpLoadVar, ir.OpStoreVar, ir.OpLoadField, ir.OpStoreField,
		 ir.OpLoadElement, ir.OpStoreElement:
		return true
	}
	return false
}

func isControlFlow(inst *ir.Instruction) bool {
	switch inst.Op {
	case ir.OpJump, ir.OpJumpIf, ir.OpJumpIfNot, ir.OpCall, ir.OpReturn:
		return true
	}
	return false
}

func isArithmetic(inst *ir.Instruction) bool {
	switch inst.Op {
	case ir.OpAdd, ir.OpSub, ir.OpMul, ir.OpDiv, ir.OpMod,
		 ir.OpAnd, ir.OpOr, ir.OpXor, ir.OpShl, ir.OpShr:
		return true
	}
	return false
}

// basicBlock represents a sequence of instructions without branches
type basicBlock struct {
	start  int
	end    int
	isLoop bool
}

// findBasicBlocks identifies basic blocks in the function
func (p *MIRReorderingPass) findBasicBlocks(fn *ir.Function) []basicBlock {
	blocks := []basicBlock{}
	
	start := 0
	for i, inst := range fn.Instructions {
		if inst.Op == ir.OpLabel || isControlFlow(&inst) {
			if i > start {
				blocks = append(blocks, basicBlock{
					start:  start,
					end:    i,
					isLoop: false, // TODO: Detect loops properly
				})
			}
			start = i + 1
		}
	}
	
	if start < len(fn.Instructions) {
		blocks = append(blocks, basicBlock{
			start:  start,
			end:    len(fn.Instructions),
			isLoop: false,
		})
	}
	
	return blocks
}

// isLoopInvariant checks if an instruction is loop invariant
func (p *MIRReorderingPass) isLoopInvariant(inst *ir.Instruction, block basicBlock, deps *dependency) bool {
	// An instruction is loop invariant if:
	// 1. It doesn't depend on any values defined in the loop
	// 2. It doesn't have side effects
	
	if isMemoryOp(inst) || inst.Op == ir.OpCall {
		return false // Conservative: assume memory ops and calls have side effects
	}
	
	// Check if source registers are defined outside the loop
	// (Simplified check - would need proper SSA form for accuracy)
	return true
}

// usesMemoryLocation checks if an instruction uses the memory location stored by another
func (p *MIRReorderingPass) usesMemoryLocation(inst, store *ir.Instruction) bool {
	// Conservative: assume any load might alias with any store
	if inst.Op == ir.OpLoadVar || inst.Op == ir.OpLoadField || inst.Op == ir.OpLoadElement {
		return true
	}
	return false
}

// storesSameLocation checks if two stores write to the same location
func (p *MIRReorderingPass) storesSameLocation(inst1, inst2 *ir.Instruction) bool {
	if inst1.Op != inst2.Op {
		return false
	}
	
	switch inst1.Op {
	case ir.OpStoreVar:
		// Same variable?
		return inst1.Symbol == inst2.Symbol
	case ir.OpStoreField:
		// Same field of same object?
		return inst1.Src1 == inst2.Src1 && inst1.Symbol == inst2.Symbol
	}
	
	return false
}

// areRelatedOps checks if two operations are related and benefit from clustering
func (p *MIRReorderingPass) areRelatedOps(inst1, inst2 *ir.Instruction) bool {
	// Operations are related if they:
	// 1. Use the same source registers
	// 2. Feed into each other
	// 3. Are part of the same expression pattern
	
	// Same operation type is often beneficial to cluster
	if inst1.Op == inst2.Op {
		return true
	}
	
	// One feeds into the other
	if inst1.Dest == inst2.Src1 || inst1.Dest == inst2.Src2 {
		return true
	}
	
	// Use same sources (might enable CSE)
	if (inst1.Src1 == inst2.Src1 && inst1.Src1 != 0) ||
	   (inst1.Src2 == inst2.Src2 && inst1.Src2 != 0) {
		return true
	}
	
	return false
}