package lir

import "fmt"

// ISel performs instruction selection: MIR2-like ops → LIR instructions
// using pattern matching against the machine descriptor.
//
// The isel is deliberately simple for now (greedy forward pass).
// Future: bidirectional constraint propagation (ADR-0027 WFC style).

// MIROp is a simplified MIR2 operation for LIR testing.
// Decoupled from the mir2 package to keep lir self-contained.
type MIROp struct {
	Op   int   // OpAdd, OpSub, etc.
	Dst  int   // virtual register for result (-1 = no result)
	Src  [2]int // virtual register sources (-1 = unused)
	Imm  int64 // immediate value (for OpConst)
	Width int  // operand width in bits
}

// ISelResult is the output of instruction selection.
type ISelResult struct {
	Insts []Inst
	VRegAllowed map[int]LocSet // vreg → allowed physical locations (narrowed by isel)
}

// SelectInstructions performs greedy isel: for each MIR op, find the best
// matching pattern and emit LIR instructions. Inserts setup moves when
// operand constraints require values in specific locations.
func SelectInstructions(desc *MachineDesc, ops []MIROp) (*ISelResult, error) {
	result := &ISelResult{
		VRegAllowed: make(map[int]LocSet),
	}

	// Track where each vreg currently lives (after const/move).
	vregLoc := make(map[int]LocSet)

	movePat := findMovePat(desc, 8)

	for i, op := range ops {
		pat, err := findBestPattern(desc, op)
		if err != nil {
			return nil, fmt.Errorf("isel: op %d (%d): %w", i, op.Op, err)
		}

		// Insert setup moves: if pattern requires src in a specific set
		// and the vreg isn't there yet, emit a move first.
		for s := 0; s < 2; s++ {
			if op.Src[s] < 0 {
				continue
			}
			srcAllowed := pat.SrcLocs[s]
			if srcAllowed.IsEmpty() {
				continue
			}
			curLoc := vregLoc[op.Src[s]]
			if !curLoc.IsEmpty() && curLoc.And(srcAllowed).IsEmpty() && movePat != nil {
				// Value not in allowed set → insert move to bring it there.
				// Create a new vreg for the moved value.
				newVReg := 1000 + len(result.Insts) // synthetic vreg
				moveInst := Inst{
					Pat:  movePat,
					Dst:  Operand{VReg: newVReg, Allowed: srcAllowed, Phys: -1},
					Srcs: [2]Operand{{VReg: op.Src[s], Allowed: curLoc, Phys: -1}},
				}
				result.Insts = append(result.Insts, moveInst)
				result.VRegAllowed[newVReg] = srcAllowed
				vregLoc[newVReg] = srcAllowed
				op.Src[s] = newVReg // patch source to use moved value
			}
		}

		inst := Inst{
			Pat: pat,
			Imm: op.Imm,
		}

		if op.Dst >= 0 {
			allowed := pat.DstLocs
			inst.Dst = Operand{VReg: op.Dst, Allowed: allowed, Phys: -1}
			result.VRegAllowed[op.Dst] = allowed
			vregLoc[op.Dst] = allowed
		}

		for s := 0; s < 2; s++ {
			if op.Src[s] >= 0 {
				allowed := pat.SrcLocs[s]
				inst.Srcs[s] = Operand{VReg: op.Src[s], Allowed: allowed, Phys: -1}
				if _, ok := result.VRegAllowed[op.Src[s]]; !ok {
					result.VRegAllowed[op.Src[s]] = allowed
				}
			}
		}

		result.Insts = append(result.Insts, inst)
	}

	return result, nil
}

func findMovePat(desc *MachineDesc, width int) *Pattern {
	for i := range desc.Patterns {
		p := &desc.Patterns[i]
		if p.MIROp == OpMove && (p.Width == 0 || p.Width == width) {
			return p
		}
	}
	return nil
}

// findBestPattern selects the lowest-cost pattern that matches the MIR op.
// For ops like OpConst that have multiple patterns with different DstLocs,
// it creates a synthetic "union" pattern combining all alternatives.
func findBestPattern(desc *MachineDesc, op MIROp) (*Pattern, error) {
	var candidates []*Pattern

	for i := range desc.Patterns {
		p := &desc.Patterns[i]
		if p.MIROp != op.Op {
			continue
		}
		if p.Width != 0 && p.Width != op.Width {
			continue
		}
		if op.Dst >= 0 && p.DstLocs.IsEmpty() {
			continue
		}
		candidates = append(candidates, p)
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no pattern for op=%d width=%d", op.Op, op.Width)
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}

	// Multiple patterns: pick best cost, but UNION all DstLocs so the
	// allocator has freedom to choose among alternatives.
	best := candidates[0]
	unionDst := best.DstLocs
	unionSrc0 := best.SrcLocs[0]
	unionSrc1 := best.SrcLocs[1]
	for _, c := range candidates[1:] {
		unionDst = unionDst.Or(c.DstLocs)
		unionSrc0 = unionSrc0.Or(c.SrcLocs[0])
		unionSrc1 = unionSrc1.Or(c.SrcLocs[1])
		if c.Cost < best.Cost {
			best = c
		}
	}

	// Return a synthetic pattern with unioned location sets.
	synth := *best
	synth.DstLocs = unionDst
	synth.SrcLocs = [2]LocSet{unionSrc0, unionSrc1}
	return &synth, nil
}

// GreedyAlloc assigns physical locations to virtual registers using a
// simple greedy strategy: first-come, first-served from the allowed set.
// For convergence testing only — real backends will use PBQP/LocSet-aware alloc.
func GreedyAlloc(result *ISelResult, desc *MachineDesc) error {
	assigned := make(map[int]int) // vreg → phys loc index
	used := LocSet(0)

	for vreg, allowed := range result.VRegAllowed {
		if allowed.IsEmpty() {
			return fmt.Errorf("alloc: vreg %d has empty allowed set", vreg)
		}
		// Find first available location in allowed set.
		found := false
		for i := 0; i < len(desc.Locs); i++ {
			if allowed.Has(i) && !used.Has(i) {
				assigned[vreg] = i
				used = used.Set(i)
				found = true
				break
			}
		}
		if !found {
			// All preferred locs taken — spill to first available.
			for i := 0; i < len(desc.Locs); i++ {
				if allowed.Has(i) {
					assigned[vreg] = i
					found = true
					break
				}
			}
		}
		if !found {
			return fmt.Errorf("alloc: vreg %d: no location available (allowed=%064b)", vreg, allowed)
		}
	}

	// Patch instructions with physical assignments.
	for i := range result.Insts {
		inst := &result.Insts[i]
		if inst.Dst.VReg >= 0 {
			inst.Dst.Phys = assigned[inst.Dst.VReg]
		}
		for s := 0; s < 2; s++ {
			if inst.Srcs[s].VReg >= 0 {
				inst.Srcs[s].Phys = assigned[inst.Srcs[s].VReg]
			}
		}
	}

	return nil
}
