package codegen

import (
	"github.com/minz/minzc/pkg/ir"
)

// Z80RegisterAllocator manages allocation of virtual registers to Z80 registers
type Z80RegisterAllocator struct {
	// Virtual to physical register mapping
	allocation map[ir.Register]PhysicalReg
	
	// Track which physical registers are free
	freeRegs RegisterPool
	
	// Track register contents for optimization
	regContents map[PhysicalReg]ir.Register
	
	// Spill slots on stack
	spillSlots map[ir.Register]int
	nextSpillSlot int
	
	// Whether we can use shadow registers
	useShadowRegs bool
	
	// Current function being processed
	currentFunc *ir.Function
}

// PhysicalReg represents a physical Z80 register
type PhysicalReg int

const (
	RegNone PhysicalReg = iota
	RegA
	RegB
	RegC
	RegD
	RegE
	RegH
	RegL
	RegBC
	RegDE
	RegHL
	RegIX
	RegIY
	RegSP
	// Shadow registers
	RegA_Shadow
	RegB_Shadow
	RegC_Shadow
	RegD_Shadow
	RegE_Shadow
	RegH_Shadow
	RegL_Shadow
	RegBC_Shadow
	RegDE_Shadow
	RegHL_Shadow
)

// RegisterPool tracks available registers
type RegisterPool struct {
	available map[PhysicalReg]bool
}

// NewZ80RegisterAllocator creates a new register allocator
func NewZ80RegisterAllocator() *Z80RegisterAllocator {
	ra := &Z80RegisterAllocator{
		allocation:   make(map[ir.Register]PhysicalReg),
		regContents:  make(map[PhysicalReg]ir.Register),
		spillSlots:   make(map[ir.Register]int),
		nextSpillSlot: 0,
	}
	
	ra.freeRegs = RegisterPool{
		available: make(map[PhysicalReg]bool),
	}
	
	// Initialize available registers
	ra.initializeRegisterPool()
	
	return ra
}

// initializeRegisterPool sets up the initial register pool
func (ra *Z80RegisterAllocator) initializeRegisterPool() {
	// Main registers
	ra.freeRegs.available[RegA] = true
	ra.freeRegs.available[RegB] = true
	ra.freeRegs.available[RegC] = true
	ra.freeRegs.available[RegD] = true
	ra.freeRegs.available[RegE] = true
	ra.freeRegs.available[RegH] = true
	ra.freeRegs.available[RegL] = true
	// Register pairs
	ra.freeRegs.available[RegBC] = true
	ra.freeRegs.available[RegDE] = true
	ra.freeRegs.available[RegHL] = true
	// IX/IY usually reserved for frame pointer and special uses
	// SP is never allocated
}

// EnableShadowRegisters enables use of shadow registers
func (ra *Z80RegisterAllocator) EnableShadowRegisters() {
	ra.useShadowRegs = true
	// Add shadow registers to pool
	ra.freeRegs.available[RegB_Shadow] = true
	ra.freeRegs.available[RegC_Shadow] = true
	ra.freeRegs.available[RegD_Shadow] = true
	ra.freeRegs.available[RegE_Shadow] = true
	ra.freeRegs.available[RegH_Shadow] = true
	ra.freeRegs.available[RegL_Shadow] = true
	ra.freeRegs.available[RegBC_Shadow] = true
	ra.freeRegs.available[RegDE_Shadow] = true
	ra.freeRegs.available[RegHL_Shadow] = true
}

// AllocateFunction performs register allocation for a function
func (ra *Z80RegisterAllocator) AllocateFunction(fn *ir.Function) {
	ra.currentFunc = fn
	ra.nextSpillSlot = 0
	
	// Clear previous allocations
	ra.allocation = make(map[ir.Register]PhysicalReg)
	ra.spillSlots = make(map[ir.Register]int)
	
	// Check if we should use shadow registers
	if fn.UsedRegisters.Contains(ir.Z80_BC_SHADOW | ir.Z80_DE_SHADOW | ir.Z80_HL_SHADOW) {
		ra.EnableShadowRegisters()
	}
	
	// Perform linear scan allocation
	ra.linearScanAllocation(fn)
}

// linearScanAllocation performs simple linear scan register allocation
func (ra *Z80RegisterAllocator) linearScanAllocation(fn *ir.Function) {
	// Build live intervals
	liveIntervals := ra.computeLiveIntervals(fn)
	
	// Sort by start position
	// For now, simple allocation in order
	
	for _, inst := range fn.Instructions {
		// Handle destination register
		if inst.Dest != 0 && inst.Dest != ir.RegZero {
			if _, allocated := ra.allocation[inst.Dest]; !allocated {
				ra.allocateRegister(inst.Dest, &inst)
			}
		}
		
		// Handle source registers
		if inst.Src1 != 0 && inst.Src1 != ir.RegZero {
			if _, allocated := ra.allocation[inst.Src1]; !allocated {
				ra.allocateRegister(inst.Src1, &inst)
			}
		}
		
		if inst.Src2 != 0 && inst.Src2 != ir.RegZero {
			if _, allocated := ra.allocation[inst.Src2]; !allocated {
				ra.allocateRegister(inst.Src2, &inst)
			}
		}
		
		// Free dead registers after this instruction
		ra.freeDeadRegisters(&inst, liveIntervals)
	}
}

// allocateRegister allocates a physical register for a virtual register
func (ra *Z80RegisterAllocator) allocateRegister(virtReg ir.Register, inst *ir.Instruction) PhysicalReg {
	// Try to get a free register
	physReg := ra.getFreeRegister(inst)
	
	if physReg != RegNone {
		ra.allocation[virtReg] = physReg
		ra.regContents[physReg] = virtReg
		return physReg
	}
	
	// No free register - need to spill
	spillReg := ra.selectSpillRegister()
	ra.spillRegister(spillReg)
	
	// Now allocate the freed register
	ra.allocation[virtReg] = spillReg
	ra.regContents[spillReg] = virtReg
	
	return spillReg
}

// getFreeRegister finds a free physical register suitable for the instruction
func (ra *Z80RegisterAllocator) getFreeRegister(inst *ir.Instruction) PhysicalReg {
	// For 16-bit operations, prefer register pairs
	if inst.Type != nil && inst.Type.Size() > 1 {
		if ra.freeRegs.available[RegHL] {
			ra.freeRegs.available[RegHL] = false
			ra.freeRegs.available[RegH] = false
			ra.freeRegs.available[RegL] = false
			return RegHL
		}
		if ra.freeRegs.available[RegDE] {
			ra.freeRegs.available[RegDE] = false
			ra.freeRegs.available[RegD] = false
			ra.freeRegs.available[RegE] = false
			return RegDE
		}
		if ra.freeRegs.available[RegBC] {
			ra.freeRegs.available[RegBC] = false
			ra.freeRegs.available[RegB] = false
			ra.freeRegs.available[RegC] = false
			return RegBC
		}
		
		// Try shadow registers if enabled
		if ra.useShadowRegs {
			if ra.freeRegs.available[RegHL_Shadow] {
				ra.freeRegs.available[RegHL_Shadow] = false
				return RegHL_Shadow
			}
		}
	}
	
	// For 8-bit operations
	for _, reg := range []PhysicalReg{RegA, RegB, RegC, RegD, RegE, RegH, RegL} {
		if ra.freeRegs.available[reg] {
			ra.freeRegs.available[reg] = false
			return reg
		}
	}
	
	// Try shadow registers for 8-bit
	if ra.useShadowRegs {
		for _, reg := range []PhysicalReg{RegB_Shadow, RegC_Shadow, RegD_Shadow, RegE_Shadow} {
			if ra.freeRegs.available[reg] {
				ra.freeRegs.available[reg] = false
				return reg
			}
		}
	}
	
	return RegNone
}

// spillRegister spills a register to memory
func (ra *Z80RegisterAllocator) spillRegister(physReg PhysicalReg) {
	virtReg := ra.regContents[physReg]
	if virtReg == 0 {
		return
	}
	
	// Allocate spill slot if needed
	if _, hasSlot := ra.spillSlots[virtReg]; !hasSlot {
		ra.spillSlots[virtReg] = ra.nextSpillSlot
		ra.nextSpillSlot += 2 // 2 bytes per spill slot
	}
	
	// Mark register as spilled
	delete(ra.allocation, virtReg)
	delete(ra.regContents, physReg)
	
	// Free the physical register
	ra.freePhysicalRegister(physReg)
}

// selectSpillRegister selects which register to spill (simple LRU for now)
func (ra *Z80RegisterAllocator) selectSpillRegister() PhysicalReg {
	// For now, just spill HL as it's often used temporarily
	if _, inUse := ra.regContents[RegHL]; inUse {
		return RegHL
	}
	
	// Otherwise spill first allocated register
	for physReg := range ra.regContents {
		return physReg
	}
	
	return RegNone
}

// freePhysicalRegister marks a physical register as free
func (ra *Z80RegisterAllocator) freePhysicalRegister(physReg PhysicalReg) {
	ra.freeRegs.available[physReg] = true
	
	// If it's a register pair, also free components
	switch physReg {
	case RegBC:
		ra.freeRegs.available[RegB] = true
		ra.freeRegs.available[RegC] = true
	case RegDE:
		ra.freeRegs.available[RegD] = true
		ra.freeRegs.available[RegE] = true
	case RegHL:
		ra.freeRegs.available[RegH] = true
		ra.freeRegs.available[RegL] = true
	}
}

// computeLiveIntervals computes live intervals for all virtual registers
func (ra *Z80RegisterAllocator) computeLiveIntervals(fn *ir.Function) map[ir.Register]LiveInterval {
	intervals := make(map[ir.Register]LiveInterval)
	
	// Simple backward analysis
	live := make(map[ir.Register]bool)
	
	for i := len(fn.Instructions) - 1; i >= 0; i-- {
		inst := &fn.Instructions[i]
		
		// Kill destination
		if inst.Dest != 0 {
			delete(live, inst.Dest)
			// Record interval start
			if interval, exists := intervals[inst.Dest]; exists {
				interval.Start = i
				intervals[inst.Dest] = interval
			} else {
				intervals[inst.Dest] = LiveInterval{Start: i, End: i}
			}
		}
		
		// Generate sources
		if inst.Src1 != 0 {
			live[inst.Src1] = true
			// Extend interval
			if interval, exists := intervals[inst.Src1]; exists {
				interval.Start = i
				intervals[inst.Src1] = interval
			} else {
				intervals[inst.Src1] = LiveInterval{Start: i, End: len(fn.Instructions)}
			}
		}
		
		if inst.Src2 != 0 {
			live[inst.Src2] = true
			// Extend interval
			if interval, exists := intervals[inst.Src2]; exists {
				interval.Start = i
				intervals[inst.Src2] = interval
			} else {
				intervals[inst.Src2] = LiveInterval{Start: i, End: len(fn.Instructions)}
			}
		}
	}
	
	return intervals
}

// freeDeadRegisters frees registers that are no longer live
func (ra *Z80RegisterAllocator) freeDeadRegisters(inst *ir.Instruction, intervals map[ir.Register]LiveInterval) {
	// Check each allocated register
	for virtReg := range ra.allocation {
		if _, exists := intervals[virtReg]; exists {
			// If this is the last use, free the register
			// (simplified - should check actual position)
			if inst.Src1 == virtReg || inst.Src2 == virtReg {
				// Mark for potential freeing after this instruction
				// In real implementation, would check if this is last use
			}
		}
	}
}

// LiveInterval represents when a virtual register is live
type LiveInterval struct {
	Start int
	End   int
}

// GetAllocation returns the physical register allocated to a virtual register
func (ra *Z80RegisterAllocator) GetAllocation(virtReg ir.Register) (PhysicalReg, bool) {
	phys, ok := ra.allocation[virtReg]
	return phys, ok
}

// IsSpilled checks if a virtual register is spilled
func (ra *Z80RegisterAllocator) IsSpilled(virtReg ir.Register) bool {
	_, spilled := ra.spillSlots[virtReg]
	return spilled
}

// GetSpillSlot returns the spill slot offset for a virtual register
func (ra *Z80RegisterAllocator) GetSpillSlot(virtReg ir.Register) (int, bool) {
	slot, ok := ra.spillSlots[virtReg]
	return slot, ok
}