package optimizer

import (
	"fmt"

	"github.com/minz/minzc/pkg/ir"
)

// SelfModifyingCodePass optimizes code using self-modifying patterns
type SelfModifyingCodePass struct {
	enabled       bool
	labelCounter  int
	smcCandidates map[ir.Register]*SMCCandidate
}

// SMCCandidate represents a potential self-modifying code optimization
type SMCCandidate struct {
	Reg        ir.Register
	LoadInst   *ir.Instruction
	LoadIndex  int
	StoreInsts []StoreInfo
	IsConstant bool
	IsParameter bool
}

// StoreInfo tracks where a register is modified
type StoreInfo struct {
	Inst  *ir.Instruction
	Index int
}

// NewSelfModifyingCodePass creates a new SMC optimization pass
func NewSelfModifyingCodePass() Pass {
	return &SelfModifyingCodePass{
		enabled:       true,
		smcCandidates: make(map[ir.Register]*SMCCandidate),
	}
}

// Name returns the name of this pass
func (p *SelfModifyingCodePass) Name() string {
	return "Self-Modifying Code Optimization"
}

// Run performs SMC optimization on the module
func (p *SelfModifyingCodePass) Run(module *ir.Module) (bool, error) {
	if !p.enabled {
		return false, nil
	}

	changed := false
	
	for _, function := range module.Functions {
		// Skip functions that can't use SMC
		if function.IsRecursive || !function.IsSMCEnabled {
			continue
		}
		
		if p.optimizeFunction(function) {
			changed = true
		}
	}
	
	return changed, nil
}

// optimizeFunction applies SMC optimization to a function
func (p *SelfModifyingCodePass) optimizeFunction(fn *ir.Function) bool {
	// First, analyze the function to find SMC candidates
	p.analyzeCandidates(fn)
	
	// Apply SMC transformation to suitable candidates
	changed := false
	for _, candidate := range p.smcCandidates {
		if p.isSuitableForSMC(candidate, fn) {
			if p.transformToSMC(candidate, fn) {
				changed = true
			}
		}
	}
	
	return changed
}

// analyzeCandidates finds potential SMC optimization opportunities
func (p *SelfModifyingCodePass) analyzeCandidates(fn *ir.Function) {
	p.smcCandidates = make(map[ir.Register]*SMCCandidate)
	
	// First pass: find all constant loads and parameter loads
	for i := range fn.Instructions {
		inst := &fn.Instructions[i]
		switch inst.Op {
		case ir.OpLoadConst:
			p.smcCandidates[inst.Dest] = &SMCCandidate{
				Reg:        inst.Dest,
				LoadInst:   inst,
				LoadIndex:  i,
				IsConstant: true,
			}
			
		case ir.OpLoadParam:
			// Parameters can be good SMC candidates if not modified
			if inst.Src1 < ir.Register(fn.NumParams) {
				p.smcCandidates[inst.Dest] = &SMCCandidate{
					Reg:         inst.Dest,
					LoadInst:    inst,
					LoadIndex:   i,
					IsParameter: true,
				}
			}
		}
	}
	
	// Second pass: find all stores that modify these values
	for i, inst := range fn.Instructions {
		// Check if this instruction modifies a candidate
		if candidate, exists := p.smcCandidates[inst.Dest]; exists {
			if i != candidate.LoadIndex {
				// This register is redefined, not suitable for SMC
				delete(p.smcCandidates, inst.Dest)
				continue
			}
		}
		
		// Check for stores that could modify SMC values
		switch inst.Op {
		case ir.OpStoreVar, ir.OpStoreField:
			// These might be modifying our SMC values indirectly
			// For now, be conservative
		case ir.OpAdd, ir.OpSub, ir.OpMul, ir.OpDiv:
			// If the result overwrites a candidate, track it
			if candidate, exists := p.smcCandidates[inst.Dest]; exists && i > candidate.LoadIndex {
				candidate.StoreInsts = append(candidate.StoreInsts, StoreInfo{
					Inst:  &fn.Instructions[i],
					Index: i,
				})
			}
		}
	}
}

// isSuitableForSMC determines if a candidate is suitable for SMC optimization
func (p *SelfModifyingCodePass) isSuitableForSMC(candidate *SMCCandidate, fn *ir.Function) bool {
	// Don't optimize if there are no stores (constant never changes)
	if len(candidate.StoreInsts) == 0 && candidate.IsConstant {
		return false
	}
	
	// Parameters are good candidates if they're read-only
	if candidate.IsParameter && len(candidate.StoreInsts) == 0 {
		return true
	}
	
	// Constants that are modified a few times are good candidates
	if candidate.IsConstant && len(candidate.StoreInsts) <= 3 {
		return true
	}
	
	// Check if the value is used frequently enough to benefit
	useCount := p.countUses(candidate.Reg, fn)
	if useCount < 2 {
		return false // Not worth it for single use
	}
	
	return true
}

// countUses counts how many times a register is used
func (p *SelfModifyingCodePass) countUses(reg ir.Register, fn *ir.Function) int {
	count := 0
	for _, inst := range fn.Instructions {
		if inst.Src1 == reg || inst.Src2 == reg {
			count++
		}
	}
	return count
}

// transformToSMC transforms a candidate to use self-modifying code
func (p *SelfModifyingCodePass) transformToSMC(candidate *SMCCandidate, fn *ir.Function) bool {
	// Generate unique label for this SMC location
	label := p.generateSMCLabel()
	
	// Transform the load instruction
	candidate.LoadInst.Op = ir.OpSMCLoadConst
	candidate.LoadInst.SMCLabel = label
	
	// Initialize SMC locations map if needed
	if fn.SMCLocations == nil {
		fn.SMCLocations = make(map[string]int)
	}
	fn.SMCLocations[label] = candidate.LoadIndex
	
	// Transform all stores to use SMC store
	for _, storeInfo := range candidate.StoreInsts {
		// Create new SMC store instruction
		smcStore := ir.Instruction{
			Op:        ir.OpSMCStoreConst,
			Src1:      storeInfo.Inst.Dest, // The new value
			SMCTarget: label,
			Type:      candidate.LoadInst.Type,
			Comment:   fmt.Sprintf("SMC store to %s", label),
		}
		
		// Insert after the computation
		fn.Instructions = insertInstruction(fn.Instructions, storeInfo.Index+1, smcStore)
		
		// Update indices for remaining stores
		for j := range candidate.StoreInsts {
			if candidate.StoreInsts[j].Index > storeInfo.Index {
				candidate.StoreInsts[j].Index++
			}
		}
	}
	
	return true
}

// generateSMCLabel generates a unique label for SMC
func (p *SelfModifyingCodePass) generateSMCLabel() string {
	p.labelCounter++
	return fmt.Sprintf("smc_%d", p.labelCounter)
}

// insertInstruction inserts an instruction at the specified index
func insertInstruction(insts []ir.Instruction, index int, inst ir.Instruction) []ir.Instruction {
	if index >= len(insts) {
		return append(insts, inst)
	}
	
	// Make space for the new instruction
	insts = append(insts, ir.Instruction{})
	copy(insts[index+1:], insts[index:])
	insts[index] = inst
	
	return insts
}

// Special optimization for function parameters using SMC
func (p *SelfModifyingCodePass) optimizeFunctionParameters(fn *ir.Function) bool {
	if fn.IsRecursive || !fn.IsSMCEnabled {
		return false
	}
	
	changed := false
	
	// For each parameter, check if it's suitable for SMC
	for i, param := range fn.Params {
		// Check if parameter is never modified within the function
		modified := false
		for _, inst := range fn.Instructions {
			if inst.Op == ir.OpStoreVar && inst.Symbol == param.Name {
				modified = true
				break
			}
		}
		
		if !modified {
			// Transform parameter loads to SMC
			label := fmt.Sprintf("%s_param_%d", fn.Name, i)
			
			// Find all loads of this parameter
			for j, inst := range fn.Instructions {
				if inst.Op == ir.OpLoadParam && inst.Src1 == ir.Register(i) {
					fn.Instructions[j].Op = ir.OpSMCLoadConst
					fn.Instructions[j].SMCLabel = label
					fn.Instructions[j].Comment = fmt.Sprintf("SMC parameter %s", param.Name)
					changed = true
				}
			}
		}
	}
	
	return changed
}