package optimizer

import (
	"github.com/minz/minzc/pkg/ir"
)

// IncDecOptimizer provides sophisticated INC/DEC optimization based on target register
type IncDecOptimizer struct {
	regAlloc RegisterAllocator
}

// RegisterAllocator interface for querying register assignments
type RegisterAllocator interface {
	GetPhysicalRegister(virtual ir.Register) (PhysicalRegister, bool)
}

// PhysicalRegister represents actual Z80 registers
type PhysicalRegister int

const (
	RegNone PhysicalRegister = iota
	RegA
	RegB
	RegC
	RegD
	RegE
	RegH
	RegL
	RegHL      // For (HL) indirect
	RegIX      // For (IX+d) indirect
	RegIY      // For (IY+d) indirect
	RegMemory  // Spilled to memory
)

// ShouldUseIncDec determines if INC/DEC is more efficient than ADD for given register and delta
func (o *IncDecOptimizer) ShouldUseIncDec(virtualReg ir.Register, delta int64) bool {
	// Get the physical register assignment
	physReg, allocated := o.regAlloc.GetPhysicalRegister(virtualReg)
	if !allocated {
		// Not yet allocated, use conservative approach
		return o.conservativeDecision(delta)
	}
	
	// Make decision based on physical register
	return o.decisionForRegister(physReg, delta)
}

// conservativeDecision when we don't know the register yet
func (o *IncDecOptimizer) conservativeDecision(delta int64) bool {
	absDelta := delta
	if absDelta < 0 {
		absDelta = -absDelta
	}
	// Conservative: assume non-A register, so up to ±3 is good
	// But limit to ±2 to avoid code bloat
	return absDelta >= 1 && absDelta <= 2
}

// decisionForRegister makes optimal decision based on actual register
func (o *IncDecOptimizer) decisionForRegister(reg PhysicalRegister, delta int64) bool {
	absDelta := delta
	if absDelta < 0 {
		absDelta = -absDelta
	}
	
	switch reg {
	case RegA:
		// For A register, only ±1 is beneficial
		return absDelta == 1
		
	case RegB, RegC, RegD, RegE, RegH, RegL:
		// For other 8-bit registers, up to ±3 is beneficial
		return absDelta >= 1 && absDelta <= 3
		
	case RegHL:
		// For (HL) indirect, up to ±2 is beneficial
		// INC (HL) is 11 cycles vs LD A,(HL); ADD A,n; LD (HL),A = 21 cycles
		return absDelta >= 1 && absDelta <= 2
		
	case RegIX, RegIY:
		// For (IX+d) or (IY+d), only ±1 is beneficial
		// INC (IX+d) is 23 cycles, very slow
		return absDelta == 1
		
	case RegMemory:
		// For memory spills, don't use INC/DEC
		// Would require load/inc/store sequence
		return false
		
	default:
		return false
	}
}

// OptimizeIncDec generates optimal INC/DEC sequence or returns nil if ADD is better
func (o *IncDecOptimizer) OptimizeIncDec(dest ir.Register, delta int64) []ir.Instruction {
	if !o.ShouldUseIncDec(dest, delta) {
		return nil
	}
	
	// Generate INC/DEC sequence
	result := []ir.Instruction{}
	count := delta
	if count < 0 {
		count = -count
	}
	
	op := ir.OpInc
	if delta < 0 {
		op = ir.OpDec
	}
	
	for i := int64(0); i < count; i++ {
		result = append(result, ir.Instruction{
			Op:   op,
			Dest: dest,
			Src1: dest,
		})
	}
	
	return result
}

// AnalyzeIncDecOpportunities reports potential INC/DEC optimizations in a function
func (o *IncDecOptimizer) AnalyzeIncDecOpportunities(fn *ir.Function) []OptimizationOpportunity {
	opportunities := []OptimizationOpportunity{}
	
	for i := 0; i < len(fn.Instructions)-1; i++ {
		inst1 := &fn.Instructions[i]
		inst2 := &fn.Instructions[i+1]
		
		// Look for pattern: load const; add reg, reg, const
		if inst1.Op == ir.OpLoadConst && inst2.Op == ir.OpAdd &&
			inst2.Src1 == inst2.Dest && inst2.Src2 == inst1.Dest {
			
			physReg, _ := o.regAlloc.GetPhysicalRegister(inst2.Dest)
			shouldOpt := o.ShouldUseIncDec(inst2.Dest, inst1.Imm)
			
			opp := OptimizationOpportunity{
				Location:    i,
				Type:        "INC/DEC",
				Register:    inst2.Dest,
				PhysicalReg: physReg,
				Delta:       inst1.Imm,
				ShouldApply: shouldOpt,
				Cycles:      o.calculateCycleSavings(physReg, inst1.Imm),
			}
			
			opportunities = append(opportunities, opp)
		}
	}
	
	return opportunities
}

// OptimizationOpportunity describes a potential INC/DEC optimization
type OptimizationOpportunity struct {
	Location    int
	Type        string
	Register    ir.Register
	PhysicalReg PhysicalRegister
	Delta       int64
	ShouldApply bool
	Cycles      int // Cycles saved (negative means slower)
}

// calculateCycleSavings returns cycles saved by using INC/DEC
func (o *IncDecOptimizer) calculateCycleSavings(reg PhysicalRegister, delta int64) int {
	absDelta := delta
	if absDelta < 0 {
		absDelta = -absDelta
	}
	
	switch reg {
	case RegA:
		// INC A = 4 cycles vs ADD A,n = 7 cycles
		if absDelta == 1 {
			return 3 // Save 3 cycles
		}
		return -int(absDelta*4 - 7) // Multiple INCs are slower
		
	case RegB, RegC, RegD, RegE, RegH, RegL:
		// INC reg = 4 cycles vs LD A,reg; ADD A,n; LD reg,A = 15 cycles
		incCycles := int(absDelta * 4)
		addCycles := 15
		return addCycles - incCycles
		
	case RegHL:
		// INC (HL) = 11 cycles vs LD A,(HL); ADD A,n; LD (HL),A = 21 cycles
		incCycles := int(absDelta * 11)
		addCycles := 21
		return addCycles - incCycles
		
	case RegIX, RegIY:
		// INC (IX+d) = 23 cycles, very expensive
		incCycles := int(absDelta * 23)
		addCycles := 30 // Rough estimate for indexed addressing
		return addCycles - incCycles
		
	default:
		return 0
	}
}

// GetRegisterName returns human-readable register name
func GetRegisterName(reg PhysicalRegister) string {
	switch reg {
	case RegA:
		return "A"
	case RegB:
		return "B"
	case RegC:
		return "C"
	case RegD:
		return "D"
	case RegE:
		return "E"
	case RegH:
		return "H"
	case RegL:
		return "L"
	case RegHL:
		return "(HL)"
	case RegIX:
		return "(IX+d)"
	case RegIY:
		return "(IY+d)"
	case RegMemory:
		return "memory"
	default:
		return "unknown"
	}
}