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
	Sym     string   // symbol name (preserved for call templates)
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
			Sym:     inst.Sym,
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
		if s.clobberPass() {
			changed = true
		}
		// Note: destructive writes are handled at MIROp level by
		// insertSaveBeforeOverwrite in bridge.go, not in WFC.
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

// clobberPass: for each call instruction with Clobbers, find vregs that are
// defined before the call and used after it. Remove clobbered locations from
// those vregs' allowed sets, forcing WFC to place them in non-clobbered locs.
// If no non-clobbered location exists, the vreg must be spilled.
func (s *WFCState) clobberPass() bool {
	changed := false

	for ci := range s.Cells {
		c := &s.Cells[ci]
		if c.Pat == nil || c.Pat.Clobbers.IsEmpty() {
			continue
		}
		clobbers := c.Pat.Clobbers

		// Find vregs defined before this cell.
		defsBefore := make(map[int]bool) // vreg → true
		for j := 0; j < ci; j++ {
			if s.Cells[j].VRegDst >= 0 {
				defsBefore[s.Cells[j].VRegDst] = true
			}
		}

		// Find vregs used after this cell.
		usesAfter := make(map[int]bool)
		for j := ci + 1; j < len(s.Cells); j++ {
			for src := 0; src < 2; src++ {
				if s.Cells[j].VRegSrc[src] >= 0 {
					usesAfter[s.Cells[j].VRegSrc[src]] = true
				}
			}
		}

		// For vregs live across this call, restrict to non-clobbered locs.
		// Non-clobbered 8-bit locs include IX/IY halves (survive calls).
		// We expand the allowed set to include call-safe alternatives.
		callSafe := s.callSafeLocs()
		for vreg := range defsBefore {
			if !usesAfter[vreg] {
				continue
			}
			// Narrow all appearances of this vreg to non-clobbered + call-safe.
			safeSet := (^clobbers).Or(callSafe)
			for j := range s.Cells {
				cell := &s.Cells[j]
				if cell.VRegDst == vreg && !cell.DstLocs.IsEmpty() {
					narrowed := cell.DstLocs.And(safeSet)
					if narrowed.IsEmpty() {
						// No safe location in current set — widen to include call-safe.
						narrowed = callSafe
					}
					if narrowed != cell.DstLocs {
						cell.DstLocs = narrowed
						changed = true
					}
				}
				for src := 0; src < 2; src++ {
					if cell.VRegSrc[src] == vreg && !cell.SrcLocs[src].IsEmpty() {
						narrowed := cell.SrcLocs[src].And(safeSet)
						if narrowed.IsEmpty() {
							narrowed = callSafe
						}
						if narrowed != cell.SrcLocs[src] {
							cell.SrcLocs[src] = narrowed
							changed = true
						}
					}
				}
			}
		}
	}

	return changed
}

// destructiveWritePass handles the case where an instruction's dst physically
// overlaps with its src0 (e.g. ADD A, r: dst=A, src0=A). If another vreg
// is live at this point and currently constrained to the same location,
// it must be moved to a non-conflicting location.
//
// Example: %r3 = add %r1, %r2 (dst=A, src0=A, src1=gpr8)
// If %r1 is still used after this instruction, %r1 and %r3 can't both be in A.
// We narrow %r1's def-site to exclude A → it goes to IXH or another safe loc.
func (s *WFCState) destructiveWritePass() bool {
	changed := false

	for ci := range s.Cells {
		c := &s.Cells[ci]
		if c.Pat == nil || c.VRegDst < 0 {
			continue
		}

		// This instruction writes to DstLocs. Any vreg that's:
		// - defined BEFORE this cell
		// - used AFTER this cell
		// - AND its current allowed set is a SUBSET of DstLocs
		// needs to be widened to include alternative locations.
		dstSet := c.DstLocs
		if dstSet.IsEmpty() || dstSet.Count() > 4 {
			continue // too broad to cause conflicts
		}

		// Find vregs used after this cell (post-uses).
		usesAfter := make(map[int]bool)
		for j := ci + 1; j < len(s.Cells); j++ {
			for src := 0; src < 2; src++ {
				if s.Cells[j].VRegSrc[src] >= 0 {
					usesAfter[s.Cells[j].VRegSrc[src]] = true
				}
			}
		}

		// Find vregs defined before that are still used after AND overlap with dstSet.
		for j := 0; j < ci; j++ {
			defCell := &s.Cells[j]
			vreg := defCell.VRegDst
			if vreg < 0 || vreg == c.VRegDst {
				continue
			}
			if !usesAfter[vreg] {
				continue
			}
			// Check if this vreg's def is constrained to the same loc as our dst.
			if defCell.DstLocs.IsEmpty() {
				continue
			}
			overlap := defCell.DstLocs.And(dstSet)
			if overlap.IsEmpty() {
				continue // no conflict
			}
			// Conflict: this vreg would be overwritten by our dst.
			// Remove the conflicting locs and add alternatives.
			safe := s.alternativeLocs(defCell.DstLocs, dstSet)
			if safe != defCell.DstLocs {
				defCell.DstLocs = safe
				changed = true
			}
			// Also update source appearances of this vreg.
			for k := range s.Cells {
				for src := 0; src < 2; src++ {
					if s.Cells[k].VRegSrc[src] == vreg {
						srcSafe := s.alternativeLocs(s.Cells[k].SrcLocs[src], dstSet)
						if srcSafe != s.Cells[k].SrcLocs[src] {
							s.Cells[k].SrcLocs[src] = srcSafe
							changed = true
						}
					}
				}
			}
		}
	}

	return changed
}

// alternativeLocs removes conflicting locs from current and adds IX halves
// as alternatives. Returns the widened set.
func (s *WFCState) alternativeLocs(current, conflict LocSet) LocSet {
	// Remove conflicting locations.
	safe := current.And(^conflict)
	if !safe.IsEmpty() {
		return safe
	}
	// All current locs conflict — widen to IX halves + spill.
	for i, loc := range s.Desc.Locs {
		if loc.Kind == LocIndex && loc.Width == 8 {
			safe = safe.Set(i)
		}
		if loc.Kind == LocMem {
			safe = safe.Set(i)
		}
	}
	return safe
}

// callSafeLocs returns locations that survive across function calls.
// On Z80: IX/IY halves (IXH, IXL, IYH, IYL) and spill slots.
func (s *WFCState) callSafeLocs() LocSet {
	var safe LocSet
	for i, loc := range s.Desc.Locs {
		// IX/IY halves survive calls (not part of standard ABI clobbers)
		if loc.Kind == LocIndex && loc.Width == 8 {
			safe = safe.Set(i)
		}
		// Memory spill slots survive calls
		if loc.Kind == LocMem {
			safe = safe.Set(i)
		}
	}
	return safe
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
// Synthetic cells (param defs, term uses) with nil Pat are excluded.
func (s *WFCState) ToInsts() []Inst {
	insts := make([]Inst, 0, len(s.Cells))
	for _, c := range s.Cells {
		if c.Pat == nil {
			continue // skip synthetic param/use cells
		}
		insts = append(insts, Inst{
			Pat: c.Pat,
			Imm: c.Imm,
			Sym: c.Sym,
			Dst: Operand{VReg: c.VRegDst, Allowed: c.DstLocs, Phys: PhysOf(c.DstLocs)},
			Srcs: [2]Operand{
				{VReg: c.VRegSrc[0], Allowed: c.SrcLocs[0], Phys: PhysOf(c.SrcLocs[0])},
				{VReg: c.VRegSrc[1], Allowed: c.SrcLocs[1], Phys: PhysOf(c.SrcLocs[1])},
			},
		})
	}
	return insts
}
