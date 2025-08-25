package optimizer

import (
	"fmt"
	"os"
	"github.com/minz/minzc/pkg/ir"
)

// TailRecursionPass implements tail recursion optimization
type TailRecursionPass struct {
	diagnostics bool
	optimized   int
}

// NewTailRecursionPass creates a new tail recursion optimization pass
func NewTailRecursionPass() Pass {
	return &TailRecursionPass{
		diagnostics: os.Getenv("MINZ_QUIET") == "" && os.Getenv("DEBUG") != "",
		optimized:   0,
	}
}

// Name returns the name of this optimization pass
func (p *TailRecursionPass) Name() string {
	return "tail-recursion"
}

// Run performs tail recursion optimization on the entire module
func (p *TailRecursionPass) Run(module *ir.Module) (bool, error) {
	if p.diagnostics {
		fmt.Println("\n=== TAIL RECURSION OPTIMIZATION ===")
	}
	
	changed := false
	for _, fn := range module.Functions {
		if p.optimizeFunction(fn) {
			p.optimized++
			changed = true
			if p.diagnostics {
				fmt.Printf("  ✅ %s: Converted tail recursion to loop\n", getShortFunctionName(fn.Name))
			}
		}
	}
	
	if p.diagnostics {
		fmt.Printf("  Total functions optimized: %d\n", p.optimized)
		fmt.Println("=====================================")
	}
	
	return changed, nil
}

// optimizeFunction performs tail recursion optimization on a single function
func (p *TailRecursionPass) optimizeFunction(fn *ir.Function) bool {
	if !fn.IsRecursive {
		return false
	}
	
	// Find all tail recursive calls
	tailCalls := p.findTailRecursiveCalls(fn)
	if len(tailCalls) == 0 {
		return false
	}
	
	if p.diagnostics {
		fmt.Printf("  🔍 %s: Found %d tail recursive calls\n", getShortFunctionName(fn.Name), len(tailCalls))
	}
	
	// Transform the function for tail recursion optimization
	p.transformFunction(fn, tailCalls)
	
	fn.HasTailRecursion = true
	return true
}

// TailCallInfo contains information about a tail recursive call
type TailCallInfo struct {
	CallIndex    int    // Index of the CALL instruction
	ReturnIndex  int    // Index of the RETURN instruction
	ParamUpdates []int  // Indices of parameter update instructions
}

// findTailRecursiveCalls identifies all tail recursive calls in a function
func (p *TailRecursionPass) findTailRecursiveCalls(fn *ir.Function) []TailCallInfo {
	var tailCalls []TailCallInfo
	
	for i := 0; i < len(fn.Instructions); i++ {
		inst := &fn.Instructions[i]
		
		// Look for recursive calls
		if inst.Op == ir.OpCall && p.isRecursiveCall(inst, fn.Name) {
			// Check if this is a tail call
			if returnIndex := p.findImmediateReturn(fn.Instructions, i); returnIndex != -1 {
				// Find parameter updates before this call
				paramUpdates := p.findParameterUpdates(fn.Instructions, i)
				
				tailCalls = append(tailCalls, TailCallInfo{
					CallIndex:    i,
					ReturnIndex:  returnIndex,
					ParamUpdates: paramUpdates,
				})
			}
		}
	}
	
	return tailCalls
}

// isRecursiveCall checks if an instruction is a recursive call
func (p *TailRecursionPass) isRecursiveCall(inst *ir.Instruction, funcName string) bool {
	if inst.Op != ir.OpCall {
		return false
	}
	
	// Check exact match or short name match
	if inst.Symbol == funcName {
		return true
	}
	
	// Check if it's a short name call to the same function
	shortName := getShortFunctionName(funcName)
	return inst.Symbol == shortName
}

// findImmediateReturn finds a return statement that immediately follows the call
func (p *TailRecursionPass) findImmediateReturn(instructions []ir.Instruction, callIndex int) int {
	// Look for RETURN in the next few instructions (allowing for simple assignments)
	for i := callIndex + 1; i < len(instructions) && i < callIndex+5; i++ {
		inst := &instructions[i]
		
		if inst.Op == ir.OpReturn {
			// Check if the return value is the result of the call
			callInst := &instructions[callIndex]
			if inst.Src1 == callInst.Dest || inst.Src1 == 0 {
				return i
			}
		}
		
		// Stop if we hit a branch or another call
		if inst.Op == ir.OpJump || inst.Op == ir.OpCall || 
		   inst.Op == ir.OpJumpIf || inst.Op == ir.OpJumpIfNot {
			break
		}
	}
	
	return -1
}

// findParameterUpdates finds instructions that update parameters before the tail call
func (p *TailRecursionPass) findParameterUpdates(instructions []ir.Instruction, callIndex int) []int {
	var updates []int
	
	// Look backwards from the call to find parameter updates
	for i := callIndex - 1; i >= 0 && i >= callIndex-10; i-- {
		inst := &instructions[i]
		
		// Look for parameter-related operations
		if inst.Op == ir.OpLoadParam || inst.Op == ir.OpSMCParam || 
		   inst.Op == ir.OpTrueSMCPatch {
			updates = append([]int{i}, updates...) // Prepend to maintain order
		}
		
		// Stop at labels or branches
		if inst.Op == ir.OpLabel || inst.Op == ir.OpJump {
			break
		}
	}
	
	return updates
}

// transformFunction transforms a function to use tail recursion optimization
func (p *TailRecursionPass) transformFunction(fn *ir.Function, tailCalls []TailCallInfo) {
	// Add loop label at the start of the function
	loopLabel := fn.Name + "_tail_loop"
	
	// Find insertion point (after parameter setup)
	insertPos := p.findParameterSetupEnd(fn)
	
	// Insert loop label
	labelInst := ir.Instruction{
		Op:      ir.OpLabel,
		Label:   loopLabel,
		Comment: "Tail recursion loop start",
	}
	
	fn.Instructions = append(fn.Instructions[:insertPos], 
		append([]ir.Instruction{labelInst}, fn.Instructions[insertPos:]...)...)
	
	// Transform tail calls (process in reverse order to maintain indices)
	for i := len(tailCalls) - 1; i >= 0; i-- {
		call := tailCalls[i]
		// Adjust indices after label insertion
		if call.CallIndex >= insertPos {
			call.CallIndex++
		}
		if call.ReturnIndex >= insertPos {
			call.ReturnIndex++
		}
		
		p.transformTailCall(fn, call, loopLabel)
	}
}

// findParameterSetupEnd finds where parameter setup ends
func (p *TailRecursionPass) findParameterSetupEnd(fn *ir.Function) int {
	for i, inst := range fn.Instructions {
		if inst.Op != ir.OpLoadParam && inst.Op != ir.OpSMCParam && 
		   inst.Op != ir.OpTrueSMCLoad {
			return i
		}
	}
	return 0
}

// transformTailCall transforms a single tail call into a loop jump
func (p *TailRecursionPass) transformTailCall(fn *ir.Function, call TailCallInfo, loopLabel string) {
	// Replace CALL with JUMP to loop start
	fn.Instructions[call.CallIndex] = ir.Instruction{
		Op:      ir.OpJump,
		Label:   loopLabel,
		Comment: "Tail recursion optimized to loop",
	}
	
	// Remove the RETURN instruction
	if call.ReturnIndex < len(fn.Instructions) {
		fn.Instructions = append(fn.Instructions[:call.ReturnIndex], 
			fn.Instructions[call.ReturnIndex+1:]...)
	}
}

// getShortFunctionName extracts short name from full function name
func getShortFunctionName(fullName string) string {
	for i := len(fullName) - 1; i >= 0; i-- {
		if fullName[i] == '.' {
			return fullName[i+1:]
		}
	}
	return fullName
}

// OptimizeTailRecursion converts tail recursive calls into jumps (legacy function)
func OptimizeTailRecursion(fn *ir.Function) bool {
	pass := &TailRecursionPass{diagnostics: false}
	return pass.optimizeFunction(fn)
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