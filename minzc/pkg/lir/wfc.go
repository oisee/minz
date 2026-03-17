package lir

import "fmt"

// WFC implements Wave Function Collapse for instruction selection and
// register allocation. Each instruction position has a LocSet per operand
// (superposition of possible physical locations). Constraints propagate
// forward and backward until a fixed point, then cells collapse to
// minimum-cost options.
//
// Dimensions:
//   1D: linear instruction sequence (basic block)
//   Operands: dst, src0, src1 per instruction
//
// Each propagation step is AND on LocSet bitfields — O(1) per operand.

// WFCState holds the superposition state for one basic block.
type WFCState struct {
	Desc  *MachineDesc
	Cells []WFCCell // one per LIR instruction
}

// WFCCell is the superposition for one instruction position.
type WFCCell struct {
	Pat     *Pattern // selected pattern (nil = not yet selected)
	DstLocs LocSet   // possible locations for destination
	SrcLocs [2]LocSet // possible locations for each source
	VRegDst int      // virtual register for dst (-1 = none)
	VRegSrc [2]int   // virtual registers for sources
	Imm     int64    // immediate value (preserved from isel)
}

// Entropy returns the number of possible states for this cell.
// Lower entropy = more constrained = should collapse first.
func (c *WFCCell) Entropy() int {
	return c.DstLocs.Count() + c.SrcLocs[0].Count() + c.SrcLocs[1].Count()
}

// IsCollapsed returns true if every operand has exactly 1 possible location.
func (c *WFCCell) IsCollapsed() bool {
	if c.DstLocs.Count() > 1 {
		return false
	}
	if c.SrcLocs[0].Count() > 1 {
		return false
	}
	if c.SrcLocs[1].Count() > 1 {
		return false
	}
	return true
}

// NewWFCState creates initial superposition from isel results.
func NewWFCState(desc *MachineDesc, insts []Inst) *WFCState {
	cells := make([]WFCCell, len(insts))
	for i, inst := range insts {
		cells[i] = WFCCell{
			Pat:     inst.Pat,
			DstLocs: inst.Dst.Allowed,
			SrcLocs: [2]LocSet{inst.Srcs[0].Allowed, inst.Srcs[1].Allowed},
			VRegDst: inst.Dst.VReg,
			VRegSrc: [2]int{inst.Srcs[0].VReg, inst.Srcs[1].VReg},
			Imm:     inst.Imm,
		}
	}
	return &WFCState{Desc: desc, Cells: cells}
}

// Propagate runs forward+backward constraint propagation until fixed point.
// Returns number of iterations.
func (s *WFCState) Propagate() int {
	iters := 0
	for iters < 20 { // safety limit
		changed := false
		if s.forwardPass() {
			changed = true
		}
		if s.backwardPass() {
			changed = true
		}
		if s.vregConsistency() {
			changed = true
		}
		iters++
		if !changed {
			break
		}
	}
	return iters
}

// forwardPass: if instruction N defines vreg V in LocSet S, then
// any later instruction using V as source can only have V in S.
func (s *WFCState) forwardPass() bool {
	changed := false
	// Track latest def LocSet for each vreg.
	vregDef := make(map[int]LocSet)

	for i := range s.Cells {
		c := &s.Cells[i]

		// Constrain sources: if this src's vreg was defined, narrow to def's LocSet.
		for src := 0; src < 2; src++ {
			if c.VRegSrc[src] < 0 {
				continue
			}
			if defSet, ok := vregDef[c.VRegSrc[src]]; ok && !defSet.IsEmpty() {
				narrowed := c.SrcLocs[src].And(defSet)
				if narrowed != c.SrcLocs[src] {
					c.SrcLocs[src] = narrowed
					changed = true
				}
			}
		}

		// Record def.
		if c.VRegDst >= 0 && !c.DstLocs.IsEmpty() {
			vregDef[c.VRegDst] = c.DstLocs
		}
	}
	return changed
}

// backwardPass: if instruction N needs src in LocSet S, and that src comes
// from vreg V defined at instruction M, then M's dst must intersect S.
func (s *WFCState) backwardPass() bool {
	changed := false
	// Track what consumers require for each vreg.
	vregRequired := make(map[int]LocSet)

	for i := len(s.Cells) - 1; i >= 0; i-- {
		c := &s.Cells[i]

		// If this instruction's dst vreg has a downstream requirement, narrow dst.
		if c.VRegDst >= 0 {
			if req, ok := vregRequired[c.VRegDst]; ok && !req.IsEmpty() {
				narrowed := c.DstLocs.And(req)
				if narrowed != c.DstLocs {
					c.DstLocs = narrowed
					changed = true
				}
			}
		}

		// Record source requirements backward.
		for src := 0; src < 2; src++ {
			if c.VRegSrc[src] < 0 || c.SrcLocs[src].IsEmpty() {
				continue
			}
			vreg := c.VRegSrc[src]
			if prev, ok := vregRequired[vreg]; ok {
				vregRequired[vreg] = prev.And(c.SrcLocs[src])
			} else {
				vregRequired[vreg] = c.SrcLocs[src]
			}
		}
	}
	return changed
}

// vregConsistency ensures all uses of the same vreg agree on location.
func (s *WFCState) vregConsistency() bool {
	changed := false
	// Collect all LocSets for each vreg (both def and use).
	vregSets := make(map[int]LocSet)

	for i := range s.Cells {
		c := &s.Cells[i]
		if c.VRegDst >= 0 && !c.DstLocs.IsEmpty() {
			if prev, ok := vregSets[c.VRegDst]; ok {
				vregSets[c.VRegDst] = prev.And(c.DstLocs)
			} else {
				vregSets[c.VRegDst] = c.DstLocs
			}
		}
		for src := 0; src < 2; src++ {
			v := c.VRegSrc[src]
			if v < 0 || c.SrcLocs[src].IsEmpty() {
				continue
			}
			if prev, ok := vregSets[v]; ok {
				vregSets[v] = prev.And(c.SrcLocs[src])
			} else {
				vregSets[v] = c.SrcLocs[src]
			}
		}
	}

	// Apply intersected sets back.
	for i := range s.Cells {
		c := &s.Cells[i]
		if c.VRegDst >= 0 {
			if consensus, ok := vregSets[c.VRegDst]; ok {
				if consensus != c.DstLocs {
					c.DstLocs = consensus
					changed = true
				}
			}
		}
		for src := 0; src < 2; src++ {
			v := c.VRegSrc[src]
			if v < 0 {
				continue
			}
			if consensus, ok := vregSets[v]; ok {
				if consensus != c.SrcLocs[src] {
					c.SrcLocs[src] = consensus
					changed = true
				}
			}
		}
	}
	return changed
}

// Collapse assigns physical locations to all operands.
// Sequential order: process instructions in program order, tracking which
// physical locations are occupied by live vregs to prevent conflicts.
func (s *WFCState) Collapse() error {
	// Track vreg → physical location assignment.
	vregPhys := make(map[int]int) // vreg → phys index
	// Track which vregs are still live (consumed by later instructions).
	liveVregs := make(map[int]bool)
	for i := range s.Cells {
		for src := 0; src < 2; src++ {
			if s.Cells[i].VRegSrc[src] >= 0 {
				liveVregs[s.Cells[i].VRegSrc[src]] = true
			}
		}
	}

	for i := range s.Cells {
		c := &s.Cells[i]

		// Build set of currently occupied locs (from live vregs).
		occupied := LocSet(0)
		for vreg, phys := range vregPhys {
			if liveVregs[vreg] {
				occupied = occupied.Set(phys)
			}
		}

		// Collapse dst: pick first allowed that isn't occupied by a different live vreg.
		if c.VRegDst >= 0 && c.DstLocs.Count() > 1 {
			available := c.DstLocs.And(occupied.Or(c.DstLocs)) // mask doesn't help, need NOT occupied
			// Remove occupied locations of OTHER vregs.
			free := c.DstLocs
			for vreg, phys := range vregPhys {
				if vreg != c.VRegDst && liveVregs[vreg] {
					free = free.Clear(phys)
				}
			}
			if !free.IsEmpty() {
				c.DstLocs = pickFirst(free)
			} else {
				c.DstLocs = pickFirst(c.DstLocs) // fallback
			}
			_ = available
		}

		// Record dst assignment.
		if c.VRegDst >= 0 {
			phys := PhysOf(c.DstLocs)
			if phys >= 0 {
				vregPhys[c.VRegDst] = phys
			}
		}

		// Collapse sources: use already-assigned phys for their vreg.
		for src := 0; src < 2; src++ {
			vreg := c.VRegSrc[src]
			if vreg < 0 {
				continue
			}
			if phys, ok := vregPhys[vreg]; ok {
				c.SrcLocs[src] = Singleton(phys)
			} else if c.SrcLocs[src].Count() > 1 {
				c.SrcLocs[src] = pickFirst(c.SrcLocs[src])
			}
		}

		// Mark consumed vregs as dead after this instruction.
		// (Simple: vreg dies at its last use.)
		for src := 0; src < 2; src++ {
			vreg := c.VRegSrc[src]
			if vreg < 0 {
				continue
			}
			// Check if any later instruction also uses this vreg.
			usedLater := false
			for j := i + 1; j < len(s.Cells); j++ {
				if s.Cells[j].VRegSrc[0] == vreg || s.Cells[j].VRegSrc[1] == vreg {
					usedLater = true
					break
				}
			}
			if !usedLater {
				delete(liveVregs, vreg)
			}
		}
	}

	// Verify no empty sets — attempt spill recovery for contradictions.
	for i := range s.Cells {
		c := &s.Cells[i]
		if c.VRegDst >= 0 && c.DstLocs.IsEmpty() {
			// Contradiction: no physical register available.
			// Recovery: widen DstLocs to include spill slots (LocMem).
			spillSet := s.spillLocs()
			if !spillSet.IsEmpty() {
				c.DstLocs = spillSet
				// Re-run propagation to push spill constraints through.
				s.Propagate()
			} else {
				return fmt.Errorf("wfc: cell %d dst has empty LocSet (contradiction, no spill slots)", i)
			}
		}
	}

	return nil
}

// spillLocs returns a LocSet of all memory-class locations (spill slots).
func (s *WFCState) spillLocs() LocSet {
	var spill LocSet
	for i, loc := range s.Desc.Locs {
		if loc.Kind == LocMem {
			spill = spill.Set(i)
		}
	}
	return spill
}

func pickFirst(s LocSet) LocSet {
	for bit := 0; bit < MaxLocs; bit++ {
		if s.Has(bit) {
			return Singleton(bit)
		}
	}
	return s
}

// PhysLoc returns the collapsed physical location index for a LocSet.
// Panics if not collapsed to exactly 1 bit.
func PhysOf(s LocSet) int {
	if s.Count() != 1 {
		return -1
	}
	for i := 0; i < MaxLocs; i++ {
		if s.Has(i) {
			return i
		}
	}
	return -1
}

// ToInsts converts WFC state back to LIR instructions with physical assignments.
func (s *WFCState) ToInsts() []Inst {
	insts := make([]Inst, len(s.Cells))
	for i, c := range s.Cells {
		insts[i] = Inst{
			Pat: c.Pat,
			Imm: c.Imm,
			Dst: Operand{VReg: c.VRegDst, Allowed: c.DstLocs, Phys: PhysOf(c.DstLocs)},
			Srcs: [2]Operand{
				{VReg: c.VRegSrc[0], Allowed: c.SrcLocs[0], Phys: PhysOf(c.SrcLocs[0])},
				{VReg: c.VRegSrc[1], Allowed: c.SrcLocs[1], Phys: PhysOf(c.SrcLocs[1])},
			},
		}
	}
	return insts
}
