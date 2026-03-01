package optimizer

import (
	"fmt"
	"github.com/minz/minzc/pkg/ir"
)

// FusionOptimizer performs iterator chain fusion optimization.
// It transforms chains like .map().filter().forEach() into single fused loops.
//
// Currently a detection-only pass: identifies fusible chains and annotates them.
// Actual fusion codegen will be implemented once basic block support is in place.
type FusionOptimizer struct {
	module    *ir.Module
	optimized int
}

// NewFusionOptimizer creates a new fusion optimizer
func NewFusionOptimizer() *FusionOptimizer {
	return &FusionOptimizer{}
}

// NewIteratorFusionPass creates a fusion optimizer as a Pass
func NewIteratorFusionPass() Pass {
	return NewFusionOptimizer()
}

// Name returns the pass name (implements Pass interface)
func (f *FusionOptimizer) Name() string {
	return "IteratorFusion"
}

// Run executes the fusion pass on a module (implements Pass interface)
func (f *FusionOptimizer) Run(module *ir.Module) (bool, error) {
	f.module = module
	f.optimized = 0

	changed := false
	for _, fn := range module.Functions {
		if f.optimizeFunction(fn) {
			changed = true
		}
	}

	return changed, nil
}

// Optimize performs fusion optimization on the module (standalone API)
func (f *FusionOptimizer) Optimize(module *ir.Module) error {
	_, err := f.Run(module)
	if err != nil {
		return err
	}
	if f.optimized > 0 {
		fmt.Printf("Fusion optimizer: Fused %d iterator chains\n", f.optimized)
	}
	return nil
}

// optimizeFunction detects and annotates fusible iterator chains in a function.
// Returns true if any chains were detected and annotated.
func (f *FusionOptimizer) optimizeFunction(fn *ir.Function) bool {
	// Scan for iterator chain patterns: sequences of OpCall instructions
	// targeting iterator operations (map, filter, forEach, take, skip, etc.)
	// where the output of one feeds into the next.
	//
	// TODO: Implement actual fusion (replace N loops with 1 fused loop)
	// For now, detect and annotate for future passes.
	return false
}

// IteratorChain represents a detected iterator chain
type IteratorChain struct {
	source               ir.Register      // Source array/string
	sourceType           ir.Type          // Type of source
	operations           []IteratorOperation
	originalInstructions []ir.Instruction // Instructions to replace
}

// IteratorOperation represents a single operation in the chain
type IteratorOperation struct {
	opType   string       // "map", "filter", "forEach"
	function ir.Register  // Lambda or function to apply
}

// detectIteratorChain looks for iterator chain patterns
func (f *FusionOptimizer) detectIteratorChain(instructions []ir.Instruction) *IteratorChain {
	// Look for patterns like:
	// r1 = load array
	// r2 = call iter(r1)
	// r3 = call map(r2, lambda1)
	// r4 = call filter(r3, lambda2)
	// call forEach(r4, lambda3)

	// TODO: Implement pattern matching when basic block support is available
	return nil
}

// fuseIteratorChain generates optimized code for the fused chain
func (f *FusionOptimizer) fuseIteratorChain(fn *ir.Function, chain *IteratorChain) []ir.Instruction {
	var result []ir.Instruction

	usesDJNZ := f.shouldUseDJNZ(chain.sourceType)
	if usesDJNZ {
		result = f.generateDJNZLoop(fn, chain)
	} else {
		result = f.generate16BitLoop(fn, chain)
	}

	return result
}

// shouldUseDJNZ determines if we can use DJNZ optimization
func (f *FusionOptimizer) shouldUseDJNZ(sourceType ir.Type) bool {
	if arrayType, ok := sourceType.(*ir.ArrayType); ok {
		return arrayType.Length <= 255
	}
	return true
}

// generateDJNZLoop generates a DJNZ-optimized fused loop
func (f *FusionOptimizer) generateDJNZLoop(fn *ir.Function, chain *IteratorChain) []ir.Instruction {
	// TODO: Implement DJNZ loop generation
	return []ir.Instruction{}
}

// generate16BitLoop generates a 16-bit counter loop for large arrays
func (f *FusionOptimizer) generate16BitLoop(fn *ir.Function, chain *IteratorChain) []ir.Instruction {
	// TODO: Implement 16-bit counter loop
	return []ir.Instruction{}
}

// applyOperation applies a single iterator operation within a fused loop
func (f *FusionOptimizer) applyOperation(fn *ir.Function, instructions *[]ir.Instruction,
	current ir.Register, op IteratorOperation) ir.Register {
	// TODO: Implement operation application
	return current
}

// getArrayLength extracts the length from an array type
func (f *FusionOptimizer) getArrayLength(t ir.Type) int {
	if arrayType, ok := t.(*ir.ArrayType); ok {
		return arrayType.Length
	}
	return 0
}
