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
	Hints map[int]int // vreg → preferred phys loc (from PBQP)
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
	MIROp   *MIROp   // original MIR op (for pattern-level retry in validate-reject)
	IsMeta  bool     // true for meta-instructions (skip in emit/propagate)
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
	// Collect meta-pin constraints: vreg → forced physical location.
	pins := map[int]int{} // vreg → loc index
	for _, inst := range insts {
		if inst.Meta != nil && inst.Meta.Kind == MetaPin {
			pins[inst.Meta.PinVReg] = inst.Meta.PinLoc
		}
	}

	cells := make([]WFCCell, len(insts))
	for i, inst := range insts {
		// Skip meta instructions — they don't become real cells.
		// But we still create a placeholder cell to keep indices aligned.
		if inst.Meta != nil {
			cells[i] = WFCCell{IsMeta: true}
			continue
		}
		cells[i] = WFCCell{
			Pat:     inst.Pat,
			DstLocs: inst.Dst.Allowed,
			SrcLocs: [2]LocSet{inst.Srcs[0].Allowed, inst.Srcs[1].Allowed},
			VRegDst: inst.Dst.VReg,
			VRegSrc: [2]int{inst.Srcs[0].VReg, inst.Srcs[1].VReg},
			Imm:     inst.Imm,
			Sym:     inst.Sym,
		}
		// Apply pin constraints: narrow Allowed to single location.
		if loc, ok := pins[cells[i].VRegDst]; ok {
			cells[i].DstLocs = 1 << uint(loc)
		}
		for s := 0; s < 2; s++ {
			if loc, ok := pins[cells[i].VRegSrc[s]]; ok {
				cells[i].SrcLocs[s] = 1 << uint(loc)
			}
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
		if s.operandInterference() {
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

// operandInterference ensures that different vregs used as operands of the
// same instruction don't get the same physical register. For stores, this
// prevents ptr and val from collapsing to the same pair (LD (HL), HL).
// Also handles dst vs src interference for accumulator-destination ops.
func (s *WFCState) operandInterference() bool {
	changed := false
	for i := range s.Cells {
		c := &s.Cells[i]

		// src0 vs src1: if different vregs, must not overlap.
		if c.VRegSrc[0] >= 0 && c.VRegSrc[1] >= 0 && c.VRegSrc[0] != c.VRegSrc[1] {
			// If src0 is collapsed to singleton, remove from src1's domain.
			if c.SrcLocs[0].Count() == 1 {
				narrowed := c.SrcLocs[1].Subtract(c.SrcLocs[0])
				if !narrowed.IsEmpty() && narrowed != c.SrcLocs[1] {
					c.SrcLocs[1] = narrowed
					changed = true
				}
			}
			// Symmetric: if src1 collapsed, remove from src0.
			if c.SrcLocs[1].Count() == 1 {
				narrowed := c.SrcLocs[0].Subtract(c.SrcLocs[1])
				if !narrowed.IsEmpty() && narrowed != c.SrcLocs[0] {
					c.SrcLocs[0] = narrowed
					changed = true
				}
			}
		}

		// dst vs src: dst must not overlap with src (except for in-place ops
		// like INC/DEC where dst==src by design).
		if c.VRegDst >= 0 && c.DstLocs.Count() == 1 {
			for src := 0; src < 2; src++ {
				if c.VRegSrc[src] >= 0 && c.VRegSrc[src] != c.VRegDst {
					narrowed := c.SrcLocs[src].Subtract(c.DstLocs)
					if !narrowed.IsEmpty() && narrowed != c.SrcLocs[src] {
						c.SrcLocs[src] = narrowed
						changed = true
					}
				}
			}
		}
	}

	// Also propagate: if a vreg appears as src in one cell with a narrowed
	// domain, update all other cells that use the same vreg.
	vregNarrow := make(map[int]LocSet)
	for i := range s.Cells {
		c := &s.Cells[i]
		for src := 0; src < 2; src++ {
			vreg := c.VRegSrc[src]
			if vreg < 0 || c.SrcLocs[src].IsEmpty() {
				continue
			}
			if prev, ok := vregNarrow[vreg]; ok {
				inter := prev.And(c.SrcLocs[src])
				if !inter.IsEmpty() {
					vregNarrow[vreg] = inter
				}
			} else {
				vregNarrow[vreg] = c.SrcLocs[src]
			}
		}
	}
	for i := range s.Cells {
		c := &s.Cells[i]
		for src := 0; src < 2; src++ {
			vreg := c.VRegSrc[src]
			if vreg < 0 {
				continue
			}
			if narrow, ok := vregNarrow[vreg]; ok {
				inter := c.SrcLocs[src].And(narrow)
				if !inter.IsEmpty() && inter != c.SrcLocs[src] {
					c.SrcLocs[src] = inter
					changed = true
				}
			}
		}
		if c.VRegDst >= 0 {
			if narrow, ok := vregNarrow[c.VRegDst]; ok {
				inter := c.DstLocs.And(narrow)
				if !inter.IsEmpty() && inter != c.DstLocs {
					c.DstLocs = inter
					changed = true
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
				c.DstLocs = s.pickPreferred(free, c.VRegDst)
			} else {
				c.DstLocs = s.pickPreferred(c.DstLocs, c.VRegDst) // fallback
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

		// Collapse sources: use already-assigned phys for their vreg,
		// BUT respect the propagated domain (operandInterference etc.).
		for src := 0; src < 2; src++ {
			vreg := c.VRegSrc[src]
			if vreg < 0 {
				continue
			}
			if phys, ok := vregPhys[vreg]; ok {
				if c.SrcLocs[src].Has(phys) {
					// Assigned phys is in the narrowed domain — use it.
					c.SrcLocs[src] = Singleton(phys)
				} else if !c.SrcLocs[src].IsEmpty() {
					// Assigned phys was excluded by interference — pick from domain.
					if s.Desc != nil {
						c.SrcLocs[src] = s.Desc.PickCheapest(c.SrcLocs[src])
					} else {
						c.SrcLocs[src] = pickFirst(c.SrcLocs[src])
					}
				} else {
					c.SrcLocs[src] = Singleton(phys) // fallback
				}
			} else if c.SrcLocs[src].Count() > 1 {
				if s.Desc != nil {
					c.SrcLocs[src] = s.Desc.PickCheapest(c.SrcLocs[src])
				} else {
					c.SrcLocs[src] = pickFirst(c.SrcLocs[src])
				}
			}
		}

		// ── Z80 validate-reject: check if collapsed instruction is valid ──
		// Build a temporary Inst from the cell, expand template, validate.
		// If invalid (e.g. LD A, HL), try alternative dst/src assignments.
		if s.Desc != nil && s.Desc.Name == "z80" && c.Pat != nil {
			tmpInst := Inst{
				Pat: c.Pat,
				Imm: c.Imm,
				Sym: c.Sym,
				Dst: Operand{VReg: c.VRegDst, Allowed: c.DstLocs, Phys: PhysOf(c.DstLocs)},
				Srcs: [2]Operand{
					{VReg: c.VRegSrc[0], Allowed: c.SrcLocs[0], Phys: PhysOf(c.SrcLocs[0])},
					{VReg: c.VRegSrc[1], Allowed: c.SrcLocs[1], Phys: PhysOf(c.SrcLocs[1])},
				},
			}
			if !ValidateExpandedTemplate(tmpInst, s.Desc) {
				fixed := false
				for src := 0; src < 2 && !fixed; src++ {
					vreg := c.VRegSrc[src]
					if vreg < 0 {
						continue
					}
					origDomain := c.SrcLocs[src]
					if origDomain.Count() <= 1 {
						// Already singleton from constraint — try widening
						// to the pattern's SrcLocs for alternatives.
						if c.Pat != nil {
							origDomain = c.Pat.SrcLocs[src]
						}
					}
					for bit := 0; bit < MaxLocs; bit++ {
						if !origDomain.Has(bit) {
							continue
						}
						if Singleton(bit) == c.SrcLocs[src] {
							continue // already tried
						}
						// Try this alternative src assignment.
						c.SrcLocs[src] = Singleton(bit)
						alt := tmpInst
						alt.Srcs[src] = Operand{VReg: vreg, Allowed: c.SrcLocs[src], Phys: bit}
						if ValidateExpandedTemplate(alt, s.Desc) {
							// Valid! Update vreg tracking.
							if vreg >= 0 {
								vregPhys[vreg] = bit
							}
							fixed = true
							break
						}
					}
				}
				// If still not fixed, try alternative dst.
				if !fixed && c.VRegDst >= 0 {
					origDst := c.DstLocs
					if c.Pat != nil && !c.Pat.DstLocs.IsEmpty() {
						origDst = c.Pat.DstLocs
					}
					for bit := 0; bit < MaxLocs; bit++ {
						if !origDst.Has(bit) || Singleton(bit) == c.DstLocs {
							continue
						}
						c.DstLocs = Singleton(bit)
						alt := tmpInst
						alt.Dst = Operand{VReg: c.VRegDst, Allowed: c.DstLocs, Phys: bit}
						if ValidateExpandedTemplate(alt, s.Desc) {
							vregPhys[c.VRegDst] = bit
							fixed = true
							break
						}
					}
				}
				// Pattern-level retry: if all locs of current pattern are invalid,
				// try alternative patterns that match the same (Op, Width).
				if !fixed && c.Pat != nil {
					altPats := findAllPatterns(s.Desc, MIROp{
						Op:    c.Pat.MIROp,
						Dst:   c.VRegDst,
						Src:   [2]int{c.VRegSrc[0], c.VRegSrc[1]},
						Width: c.Pat.Width,
					})
					for _, altPat := range altPats {
						if altPat == c.Pat {
							continue // already tried
						}
						// Try this pattern with its own DstLocs/SrcLocs.
						savedPat := c.Pat
						savedDst := c.DstLocs
						savedSrc := c.SrcLocs

						c.Pat = altPat
						if !altPat.DstLocs.IsEmpty() && c.VRegDst >= 0 {
							c.DstLocs = s.pickPreferred(altPat.DstLocs, c.VRegDst)
						}
						// Assign srcs: reuse vregPhys only if compatible with alt pattern.
						for s2 := 0; s2 < 2; s2++ {
							if c.VRegSrc[s2] >= 0 && !altPat.SrcLocs[s2].IsEmpty() {
								if phys, ok := vregPhys[c.VRegSrc[s2]]; ok && altPat.SrcLocs[s2].Has(phys) {
									c.SrcLocs[s2] = Singleton(phys)
								} else {
									c.SrcLocs[s2] = s.Desc.PickCheapest(altPat.SrcLocs[s2])
								}
							}
						}
						// Self-conflict: if src0 and src1 ended up in the same loc,
						// force src1 to a different one (e.g. HL ptr + HL val → use DE val).
						if c.VRegSrc[0] >= 0 && c.VRegSrc[1] >= 0 {
							p0, p1 := PhysOf(c.SrcLocs[0]), PhysOf(c.SrcLocs[1])
							if p0 >= 0 && p0 == p1 && !altPat.SrcLocs[1].IsEmpty() {
								alt1 := altPat.SrcLocs[1].Clear(p0) // exclude conflicting loc
								if !alt1.IsEmpty() {
									c.SrcLocs[1] = s.Desc.PickCheapest(alt1)
								}
							}
						}

						alt := Inst{
							Pat: c.Pat,
							Imm: c.Imm,
							Sym: c.Sym,
							Dst: Operand{VReg: c.VRegDst, Allowed: c.DstLocs, Phys: PhysOf(c.DstLocs)},
							Srcs: [2]Operand{
								{VReg: c.VRegSrc[0], Allowed: c.SrcLocs[0], Phys: PhysOf(c.SrcLocs[0])},
								{VReg: c.VRegSrc[1], Allowed: c.SrcLocs[1], Phys: PhysOf(c.SrcLocs[1])},
							},
						}
						if ValidateExpandedTemplate(alt, s.Desc) {
							// Update vreg tracking for new assignments.
							if c.VRegDst >= 0 {
								if p := PhysOf(c.DstLocs); p >= 0 {
									vregPhys[c.VRegDst] = p
								}
							}
							for s2 := 0; s2 < 2; s2++ {
								if c.VRegSrc[s2] >= 0 {
									if p := PhysOf(c.SrcLocs[s2]); p >= 0 {
										vregPhys[c.VRegSrc[s2]] = p
									}
								}
							}
							fixed = true
							break
						}
						// Revert to original pattern.
						c.Pat = savedPat
						c.DstLocs = savedDst
						c.SrcLocs = savedSrc
					}
				}
				// If nothing worked, leave as-is (error will show in final validate).
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

// pickPreferred returns a singleton LocSet. If a PBQP hint exists for this vreg
// and the hinted loc is in the available set, prefer it. Otherwise pick the
// cheapest available loc using the machine descriptor's cost table.
func (st *WFCState) pickPreferred(available LocSet, vreg int) LocSet {
	if st.Hints != nil && vreg >= 0 {
		if hinted, ok := st.Hints[vreg]; ok && available.Has(hinted) {
			return Singleton(hinted)
		}
	}
	if st.Desc != nil {
		return st.Desc.PickCheapest(available)
	}
	return pickFirst(available)
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

	// Post-WFC fixup: fix self-stores where ptr and val collapsed to same phys.
	if s.Desc != nil && s.Desc.Name == "z80" {
		insts = fixSelfStores(insts, s.Desc)
	}

	return insts
}

// fixSelfStores detects store instructions where src0 (ptr) and src1 (val)
// ended up in the same physical register or where a width-8 store has src1
// in a 16-bit pair, and inserts fixup moves (EX DE,HL or LD A, low_byte).
func fixSelfStores(insts []Inst, desc *MachineDesc) []Inst {
	exPat := findExDEHLPat(desc)
	if exPat == nil {
		return insts
	}

	deIdx := desc.LocByName("DE")
	hlIdx := desc.LocByName("HL")
	if deIdx < 0 || hlIdx < 0 {
		return insts
	}

	// Also need the st16_hl_de pattern for the fixed store.
	var stPat *Pattern
	for i := range desc.Patterns {
		p := &desc.Patterns[i]
		if p.Name == "st16_hl_de" {
			stPat = p
			break
		}
	}

	// Find trunc patterns for 8-bit fixups.
	aIdx := desc.LocByName("A")
	var truncHL, truncDE, truncBC *Pattern
	for i := range desc.Patterns {
		p := &desc.Patterns[i]
		switch p.Name {
		case "trunc_hl_a":
			truncHL = p
		case "trunc_de_a":
			truncDE = p
		case "trunc_bc_a":
			truncBC = p
		}
	}
	// Also find ld_hl_a for the fixed 8-bit store.
	var stHLA *Pattern
	for i := range desc.Patterns {
		p := &desc.Patterns[i]
		if p.Name == "ld_hl_a" {
			stHLA = p
			break
		}
	}

	var result []Inst
	for _, inst := range insts {
		if inst.Pat == nil || inst.Pat.MIROp != OpStore {
			result = append(result, inst)
			continue
		}
		s0, s1 := inst.Srcs[0].Phys, inst.Srcs[1].Phys
		if s0 < 0 || s1 < 0 {
			result = append(result, inst)
			continue
		}

		// Case 2 FIRST: store where src1 (value) is a 16-bit pair.
		// This catches width mismatches (8-bit store with 16-bit value) AND
		// invalid templates like LD ({src0}), HL where src0=BC → LD (BC), HL.
		// Fix: trunc pair to low byte via A, then store A.
		bcIdx := desc.LocByName("BC")
		ixIdx := desc.LocByName("IX")
		iyIdx := desc.LocByName("IY")

		// Detect: src1 (value) is a 16-bit pair being stored via a non-HL pointer.
		// Z80 only supports LD (BC),A and LD (DE),A for non-HL indirect stores.
		isPairSrc1 := s1 == hlIdx || s1 == deIdx || s1 == bcIdx
		isPairPtr := s0 == hlIdx || s0 == deIdx || s0 == bcIdx || s0 == ixIdx || s0 == iyIdx

		if isPairSrc1 && isPairPtr {
			// Find trunc pattern for the value pair.
			var truncPat *Pattern
			if s1 == hlIdx && truncHL != nil {
				truncPat = truncHL
			} else if s1 == deIdx && truncDE != nil {
				truncPat = truncDE
			} else if s1 == bcIdx && truncBC != nil {
				truncPat = truncBC
			}
			if truncPat != nil && aIdx >= 0 {
				// Find store-via-A pattern matching the pointer register.
				var storePat *Pattern
				for pi := range desc.Patterns {
					p := &desc.Patterns[pi]
					if p.MIROp == OpStore && p.Width == 8 &&
						!p.SrcLocs[0].IsEmpty() && p.SrcLocs[0].Has(s0) &&
						!p.SrcLocs[1].IsEmpty() && p.SrcLocs[1].Has(aIdx) {
						storePat = p
						break
					}
				}
				if storePat == nil {
					storePat = stHLA // fallback
				}
				if storePat != nil {
					if inst.Pat.Width <= 8 || inst.Pat.Width == 0 || s0 == s1 {
						// 8-bit store or self-store: single trunc + store.
						result = append(result, Inst{
							Pat: truncPat,
							Dst: Operand{Phys: aIdx, Allowed: Singleton(aIdx)},
							Srcs: [2]Operand{{Phys: s1, Allowed: Singleton(s1)}},
						})
						fixed := inst
						fixed.Srcs[1] = Operand{VReg: inst.Srcs[1].VReg, Phys: aIdx, Allowed: Singleton(aIdx)}
						fixed.Pat = storePat
						result = append(result, fixed)
					} else {
						// 16-bit store via non-HL pointer: byte-by-byte.
						// LD A, low_byte(val) / LD (ptr), A / INC ptr / LD A, high_byte(val) / LD (ptr), A / DEC ptr
						// Use trunc (LD A, L/E/C) for low byte, then synthesize high byte.
						result = append(result, Inst{
							Pat: truncPat,
							Dst: Operand{Phys: aIdx, Allowed: Singleton(aIdx)},
							Srcs: [2]Operand{{Phys: s1, Allowed: Singleton(s1)}},
						})
						fixed := inst
						fixed.Srcs[1] = Operand{VReg: inst.Srcs[1].VReg, Phys: aIdx, Allowed: Singleton(aIdx)}
						fixed.Pat = storePat
						result = append(result, fixed)
						// TODO: emit INC ptr, LD A, high_byte, LD (ptr), A, DEC ptr
						// For now, emit just the low byte store (partial fix).
					}
					continue
				}
			}
		}

		// Case 1: self-store (same phys, 16-bit, no trunc pattern matched). EX DE,HL.
		if s0 == s1 && s0 == hlIdx && stPat != nil {
			result = append(result, Inst{
				Pat: exPat,
				Dst: Operand{Phys: deIdx, Allowed: Singleton(deIdx)},
				Srcs: [2]Operand{{Phys: hlIdx, Allowed: Singleton(hlIdx)}},
			})
			fixed := inst
			fixed.Srcs[1] = Operand{VReg: inst.Srcs[1].VReg, Phys: deIdx, Allowed: Singleton(deIdx)}
			fixed.Pat = stPat
			result = append(result, fixed)
			continue
		}

		result = append(result, inst)
	}
	return result
}

// findExDEHLPat finds the EX DE,HL pattern (OpMove, HL→DE or DE→HL).
func findExDEHLPat(desc *MachineDesc) *Pattern {
	for i := range desc.Patterns {
		p := &desc.Patterns[i]
		if p.Name == "ex_de_hl" || p.Name == "ex_hl_de" {
			return p
		}
	}
	return nil
}
