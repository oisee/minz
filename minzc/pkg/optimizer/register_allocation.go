package optimizer

import (
	"sort"

	"github.com/minz/minzc/pkg/ir"
)

// RegisterAllocationPass performs register allocation optimization
type RegisterAllocationPass struct {
	// Z80 registers available for allocation
	availableRegs []string
	// Maps virtual registers to physical registers
	allocation map[ir.Register]string
	// Live ranges for each virtual register
	liveRanges map[ir.Register]*LiveRange
}

// LiveRange represents when a register is live
type LiveRange struct {
	Start int // First instruction where register is defined
	End   int // Last instruction where register is used
	Uses  int // Number of uses
}

// NewRegisterAllocationPass creates a new register allocation pass
func NewRegisterAllocationPass() Pass {
	return &RegisterAllocationPass{
		// Z80 register set (excluding special purpose registers)
		availableRegs: []string{
			"A", "B", "C", "D", "E", "H", "L",
			"BC", "DE", "HL", // 16-bit register pairs
		},
		allocation: make(map[ir.Register]string),
		liveRanges: make(map[ir.Register]*LiveRange),
	}
}

// Name returns the name of this pass
func (p *RegisterAllocationPass) Name() string {
	return "Register Allocation"
}

// Run performs register allocation on the module
func (p *RegisterAllocationPass) Run(module *ir.Module) (bool, error) {
	changed := false
	
	for _, function := range module.Functions {
		if p.optimizeFunction(function) {
			changed = true
		}
	}
	
	return changed, nil
}

// optimizeFunction performs register allocation on a single function
func (p *RegisterAllocationPass) optimizeFunction(fn *ir.Function) bool {
	// Reset allocations for this function
	p.allocation = make(map[ir.Register]string)
	p.liveRanges = make(map[ir.Register]*LiveRange)
	
	// Compute live ranges
	p.computeLiveRanges(fn)
	
	// Perform register allocation using linear scan algorithm
	p.linearScanAllocation(fn)
	
	// Apply the allocation to the instructions
	changed := p.applyAllocation(fn)
	
	return changed
}

// computeLiveRanges computes the live range for each virtual register
func (p *RegisterAllocationPass) computeLiveRanges(fn *ir.Function) {
	for i, inst := range fn.Instructions {
		// Handle destination register (definition)
		if inst.Dest != 0 {
			if lr, exists := p.liveRanges[inst.Dest]; exists {
				lr.End = i
			} else {
				p.liveRanges[inst.Dest] = &LiveRange{
					Start: i,
					End:   i,
					Uses:  0,
				}
			}
		}
		
		// Handle source registers (uses)
		if inst.Src1 != 0 {
			if lr, exists := p.liveRanges[inst.Src1]; exists {
				lr.End = i
				lr.Uses++
			} else {
				// Register used before defined (parameter?)
				p.liveRanges[inst.Src1] = &LiveRange{
					Start: 0,
					End:   i,
					Uses:  1,
				}
			}
		}
		
		if inst.Src2 != 0 {
			if lr, exists := p.liveRanges[inst.Src2]; exists {
				lr.End = i
				lr.Uses++
			} else {
				p.liveRanges[inst.Src2] = &LiveRange{
					Start: 0,
					End:   i,
					Uses:  1,
				}
			}
		}
	}
}

// linearScanAllocation performs linear scan register allocation
func (p *RegisterAllocationPass) linearScanAllocation(fn *ir.Function) {
	// Sort virtual registers by start position
	var registers []ir.Register
	for reg := range p.liveRanges {
		registers = append(registers, reg)
	}
	
	sort.Slice(registers, func(i, j int) bool {
		return p.liveRanges[registers[i]].Start < p.liveRanges[registers[j]].Start
	})
	
	// Track which physical registers are free
	freeRegs := make(map[string]bool)
	for _, reg := range p.availableRegs {
		freeRegs[reg] = true
	}
	
	// Track active intervals
	type activeInterval struct {
		vreg ir.Register
		end  int
	}
	var active []activeInterval
	
	// Allocate registers
	for _, vreg := range registers {
		lr := p.liveRanges[vreg]
		
		// Free registers that are no longer live
		newActive := []activeInterval{}
		for _, interval := range active {
			if interval.end < lr.Start {
				// Free this register
				if physReg, ok := p.allocation[interval.vreg]; ok {
					freeRegs[physReg] = true
				}
			} else {
				newActive = append(newActive, interval)
			}
		}
		active = newActive
		
		// Try to allocate a register
		allocated := false
		
		// Prefer registers based on usage patterns
		preferredRegs := p.getPreferredRegisters(vreg, fn)
		
		// Try preferred registers first
		for _, reg := range preferredRegs {
			if freeRegs[reg] {
				p.allocation[vreg] = reg
				freeRegs[reg] = false
				allocated = true
				break
			}
		}
		
		// If no preferred register available, use any free register
		if !allocated {
			for reg, free := range freeRegs {
				if free {
					p.allocation[vreg] = reg
					freeRegs[reg] = false
					allocated = true
					break
				}
			}
		}
		
		// If no register available, we need to spill
		// For now, we'll keep using virtual registers
		if allocated {
			active = append(active, activeInterval{vreg: vreg, end: lr.End})
		}
	}
}

// getPreferredRegisters returns preferred physical registers for a virtual register
func (p *RegisterAllocationPass) getPreferredRegisters(vreg ir.Register, fn *ir.Function) []string {
	// Analyze usage to determine preferences
	uses8bit := 0
	uses16bit := 0
	usesAccumulator := false
	
	for _, inst := range fn.Instructions {
		if inst.Dest == vreg || inst.Src1 == vreg || inst.Src2 == vreg {
			switch inst.Op {
			case ir.OpAdd, ir.OpSub, ir.OpAnd, ir.OpOr, ir.OpXor:
				// These operations often use the accumulator
				usesAccumulator = true
				uses8bit++
			case ir.OpMul, ir.OpDiv:
				// These need specific registers
				uses16bit++
			case ir.OpLoadConst:
				if inst.Dest == vreg && inst.Imm <= 255 && inst.Imm >= -128 {
					uses8bit++
				} else {
					uses16bit++
				}
			default:
				uses8bit++
			}
		}
	}
	
	// Return preferences based on usage
	if usesAccumulator {
		return []string{"A", "B", "C", "D", "E"}
	} else if uses16bit > uses8bit {
		return []string{"HL", "DE", "BC"}
	} else {
		return []string{"B", "C", "D", "E", "H", "L", "A"}
	}
}

// applyAllocation applies the register allocation to instructions
func (p *RegisterAllocationPass) applyAllocation(fn *ir.Function) bool {
	changed := false
	
	for i := range fn.Instructions {
		inst := &fn.Instructions[i]
		
		// Apply allocation to destination
		if inst.Dest != 0 {
			if physReg, ok := p.allocation[inst.Dest]; ok {
				// Add allocation info to instruction
				if inst.PhysicalRegs == nil {
					inst.PhysicalRegs = make(map[string]string)
				}
				inst.PhysicalRegs["dest"] = physReg
				changed = true
			}
		}
		
		// Apply allocation to sources
		if inst.Src1 != 0 {
			if physReg, ok := p.allocation[inst.Src1]; ok {
				if inst.PhysicalRegs == nil {
					inst.PhysicalRegs = make(map[string]string)
				}
				inst.PhysicalRegs["src1"] = physReg
				changed = true
			}
		}
		
		if inst.Src2 != 0 {
			if physReg, ok := p.allocation[inst.Src2]; ok {
				if inst.PhysicalRegs == nil {
					inst.PhysicalRegs = make(map[string]string)
				}
				inst.PhysicalRegs["src2"] = physReg
				changed = true
			}
		}
	}
	
	return changed
}