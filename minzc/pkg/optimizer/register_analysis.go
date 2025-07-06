package optimizer

import (
	"github.com/minz/minzc/pkg/ir"
)

// RegisterAnalysisPass analyzes register usage in functions
type RegisterAnalysisPass struct {
	// Maps virtual registers to Z80 registers
	allocation map[ir.Register]ir.Z80Register
}

// NewRegisterAnalysisPass creates a new register analysis pass
func NewRegisterAnalysisPass() Pass {
	return &RegisterAnalysisPass{
		allocation: make(map[ir.Register]ir.Z80Register),
	}
}

// Name returns the name of this pass
func (p *RegisterAnalysisPass) Name() string {
	return "Register Usage Analysis"
}

// Run analyzes register usage for all functions
func (p *RegisterAnalysisPass) Run(module *ir.Module) (bool, error) {
	for _, fn := range module.Functions {
		p.analyzeFunction(fn)
	}
	return true, nil
}

// analyzeFunction determines which Z80 registers are used/modified
func (p *RegisterAnalysisPass) analyzeFunction(fn *ir.Function) {
	// Clear register sets
	fn.UsedRegisters.Clear()
	fn.ModifiedRegisters.Clear()
	fn.CalleeSavedRegs.Clear()
	
	// Track virtual to physical register mapping
	// virtToPhys := make(map[ir.Register]ir.Z80Register) // TODO: Use when implementing full allocation
	
	// Analyze each instruction
	for _, inst := range fn.Instructions {
		switch inst.Op {
		case ir.OpLoadConst, ir.OpSMCLoadConst:
			// Loading constant typically uses A or HL
			if inst.Imm < 256 {
				fn.UsedRegisters.Add(ir.Z80_A)
				fn.ModifiedRegisters.Add(ir.Z80_A)
			} else {
				fn.UsedRegisters.Add(ir.Z80_HL)
				fn.ModifiedRegisters.Add(ir.Z80_HL)
			}
			
		case ir.OpAdd, ir.OpSub:
			// Arithmetic uses HL, DE
			fn.UsedRegisters.Add(ir.Z80_HL | ir.Z80_DE)
			fn.ModifiedRegisters.Add(ir.Z80_HL)
			
		case ir.OpMul, ir.OpDiv:
			// Complex operations use more registers
			fn.UsedRegisters.Add(ir.Z80_HL | ir.Z80_DE | ir.Z80_BC | ir.Z80_A)
			fn.ModifiedRegisters.Add(ir.Z80_HL | ir.Z80_DE | ir.Z80_BC | ir.Z80_A)
			
		case ir.OpCall:
			// Function calls affect all volatile registers
			fn.UsedRegisters.Add(ir.Z80_AF | ir.Z80_BC | ir.Z80_DE | ir.Z80_HL)
			fn.ModifiedRegisters.Add(ir.Z80_AF | ir.Z80_BC | ir.Z80_DE | ir.Z80_HL)
			// Also uses SP
			fn.UsedRegisters.Add(ir.Z80_SP)
			
		case ir.OpLoadField, ir.OpStoreField:
			// Field access uses HL for pointer, DE for offset
			fn.UsedRegisters.Add(ir.Z80_HL | ir.Z80_DE)
			fn.ModifiedRegisters.Add(ir.Z80_HL | ir.Z80_DE)
			
		case ir.OpReturn:
			// Return value in HL
			if inst.Src1 != 0 {
				fn.UsedRegisters.Add(ir.Z80_HL)
			}
		}
	}
	
	// Determine callee-saved registers
	// In Z80, typically IX, IY are callee-saved
	// We always save IX as frame pointer
	fn.CalleeSavedRegs.Add(ir.Z80_IX)
	
	// If function modifies IY, it should save it
	if fn.ModifiedRegisters.Contains(ir.Z80_IY) {
		fn.CalleeSavedRegs.Add(ir.Z80_IY)
	}
	
	// For interrupt handlers, all registers are callee-saved
	if fn.IsInterrupt {
		fn.CalleeSavedRegs = fn.ModifiedRegisters
	}
	
	// Check if we can use shadow registers for optimization
	p.analyzeShadowRegisterUsage(fn)
}

// analyzeShadowRegisterUsage determines if shadow registers can optimize the function
func (p *RegisterAnalysisPass) analyzeShadowRegisterUsage(fn *ir.Function) {
	// Count register pressure
	mainRegsUsed := 0
	if fn.UsedRegisters.Contains(ir.Z80_BC) { mainRegsUsed++ }
	if fn.UsedRegisters.Contains(ir.Z80_DE) { mainRegsUsed++ }
	if fn.UsedRegisters.Contains(ir.Z80_HL) { mainRegsUsed++ }
	
	// If we have high register pressure and no function calls,
	// we could use shadow registers
	hasCall := false
	for _, inst := range fn.Instructions {
		if inst.Op == ir.OpCall {
			hasCall = true
			break
		}
	}
	
	// Shadow registers are useful for:
	// 1. Interrupt handlers (already handled)
	// 2. Leaf functions with high register pressure
	// 3. Hot loops that need extra registers
	if !hasCall && mainRegsUsed >= 2 && len(fn.Instructions) > 20 {
		// Mark that shadow registers could be beneficial
		// This is just analysis - actual usage decided during code generation
		fn.UsedRegisters.Add(ir.Z80_BC_SHADOW | ir.Z80_DE_SHADOW | ir.Z80_HL_SHADOW)
	}
}

// EstimateStackDepth estimates maximum stack usage
func (p *RegisterAnalysisPass) EstimateStackDepth(fn *ir.Function) int {
	depth := 0
	
	// Space for locals
	depth += len(fn.Locals) * 2
	
	// Saved registers (each PUSH is 2 bytes)
	savedRegs := fn.CalleeSavedRegs.Count()
	depth += savedRegs * 2
	
	// Find maximum call depth
	maxCallDepth := 0
	for _, inst := range fn.Instructions {
		if inst.Op == ir.OpCall {
			// Each call needs return address (2 bytes) + arguments
			callDepth := 2 + len(fn.Params)*2 // Simplified
			if callDepth > maxCallDepth {
				maxCallDepth = callDepth
			}
		}
	}
	
	depth += maxCallDepth
	fn.MaxStackDepth = depth
	
	return depth
}