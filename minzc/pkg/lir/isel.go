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
	Sym  string // symbol name (OpCall target)
	Clobbers LocSet // registers clobbered (OpCall)
	DstAllowed LocSet // override dst allowed set (for call arg setup moves)
	SrcAllowed [2]LocSet // override src allowed sets (for e-graph variants)
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
	defMap := buildDefMap(ops)

	for i, op := range ops {
		// Immediate ALU folding
		op = foldConstIntoALU(op, ops, defMap)

		pat, err := findBestPattern(desc, op)
		if err != nil {
			// Fallback for machines without immediate ALU patterns
			if op.Op >= OpAddImm && op.Op <= OpCmpImm {
				op = ops[i]
				pat, err = findBestPattern(desc, op)
			}
			if err != nil {
				return nil, fmt.Errorf("isel: op %d (%d): %w", i, op.Op, err)
			}
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
			if !curLoc.IsEmpty() && curLoc.And(srcAllowed).IsEmpty() {
				// Value not in allowed set → insert move to bring it there.
				// Find the best move/trunc pattern that bridges curLoc → srcAllowed.
				bridgePat := findBridgePattern(desc, curLoc, srcAllowed)
				if bridgePat == nil {
					bridgePat = movePat // fallback
				}
				if bridgePat != nil {
					newVReg := 1000 + len(result.Insts)
					moveInst := Inst{
						Pat:  bridgePat,
						Dst:  Operand{VReg: newVReg, Allowed: srcAllowed, Phys: -1},
						Srcs: [2]Operand{{VReg: op.Src[s], Allowed: curLoc, Phys: -1}},
					}
					result.Insts = append(result.Insts, moveInst)
					result.VRegAllowed[newVReg] = srcAllowed
					vregLoc[newVReg] = srcAllowed
					op.Src[s] = newVReg
				}
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

// SelectBlockInstructions performs isel for one block, pre-seeding vreg
// constraints from block params. Params define vregs that aren't produced
// by any instruction in the block — they arrive from predecessor edges.
func SelectBlockInstructions(desc *MachineDesc, ops []MIROp, params []BlockParam) (*ISelResult, error) {
	result := &ISelResult{
		VRegAllowed: make(map[int]LocSet),
	}

	// Track where each vreg currently lives.
	vregLoc := make(map[int]LocSet)

	// Pre-seed from block params: these vregs are defined on block entry.
	for _, p := range params {
		allowed := p.Allowed
		if allowed.IsEmpty() {
			allowed = desc.LocsOfWidth(desc.WordSize)
		}
		result.VRegAllowed[p.VReg] = allowed
		vregLoc[p.VReg] = allowed
	}

	movePat := findMovePat(desc, 8)
	defMap := buildDefMap(ops)

	for i, op := range ops {
		// Pass through sentinel meta-ops (condret markers).
		if op.Op == OpCondRet {
			result.Insts = append(result.Insts, Inst{
				Pat: nil, // no pattern — emitter handles via Meta
				Meta: &MetaInst{Kind: MetaComment, Comment: fmt.Sprintf("condret:%d", op.Imm)},
				Imm: op.Imm,
			})
			continue
		}

		// Immediate ALU folding: if src1 is a const, fold into immediate op.
		op = foldConstIntoALU(op, ops, defMap)

		pat, err := findBestPattern(desc, op)
		if err != nil {
			// If immediate op has no pattern (e.g. RISC), fall back to original op.
			if op.Op >= OpAddImm && op.Op <= OpCmpImm {
				op = ops[i] // restore original
				pat, err = findBestPattern(desc, op)
			}
			if err != nil {
				return nil, fmt.Errorf("isel: op %d (%d): %w", i, op.Op, err)
			}
		}

		// Insert setup moves if needed (with width-bridging via findBridgePattern).
		for s := 0; s < 2; s++ {
			if op.Src[s] < 0 {
				continue
			}
			srcAllowed := pat.SrcLocs[s]
			if srcAllowed.IsEmpty() {
				continue
			}
			curLoc := vregLoc[op.Src[s]]
			if !curLoc.IsEmpty() && curLoc.And(srcAllowed).IsEmpty() {
				bridgePat := findBridgePattern(desc, curLoc, srcAllowed)
				if bridgePat == nil {
					bridgePat = movePat
				}
				if bridgePat != nil {
					newVReg := 1000 + len(result.Insts)
					moveInst := Inst{
						Pat:  bridgePat,
						Dst:  Operand{VReg: newVReg, Allowed: srcAllowed, Phys: -1},
						Srcs: [2]Operand{{VReg: op.Src[s], Allowed: curLoc, Phys: -1}},
					}
					result.Insts = append(result.Insts, moveInst)
					result.VRegAllowed[newVReg] = srcAllowed
					vregLoc[newVReg] = srcAllowed
					op.Src[s] = newVReg
				}
			}
		}

		inst := Inst{
			Pat: pat,
			Imm: op.Imm,
			Sym: op.Sym,
		}

		if op.Dst >= 0 {
			allowed := pat.DstLocs
			// DstAllowed override: narrow destination to a specific set
			// (used for call arg setup moves to target the callee's param class).
			if !op.DstAllowed.IsEmpty() {
				narrowed := allowed.And(op.DstAllowed)
				if !narrowed.IsEmpty() {
					allowed = narrowed
				} else {
					allowed = op.DstAllowed
				}
			}
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

// defMap builds a vreg→index mapping for constant definitions.
// Used by foldConstIntoALU to look up const values.
func buildDefMap(ops []MIROp) map[int]int {
	m := make(map[int]int)
	for i, op := range ops {
		if op.Dst >= 0 {
			m[op.Dst] = i
		}
	}
	return m
}

// foldConstIntoALU checks if an ALU op's src1 is a const and converts
// to an immediate op (OpAddImm etc.) to emit ADD A,n instead of LD+ADD.
func foldConstIntoALU(op MIROp, ops []MIROp, defMap map[int]int) MIROp {
	// Only fold 8-bit ALU ops (Z80 immediate ALU is 8-bit only)
	if op.Width != 8 {
		return op
	}

	// Map ALU ops to their immediate variants
	immOp := map[int]int{
		OpAdd: OpAddImm, OpSub: OpSubImm,
		OpAnd: OpAndImm, OpOr: OpOrImm,
		OpXor: OpXorImm, OpCmp: OpCmpImm,
	}

	newOp, ok := immOp[op.Op]
	if !ok {
		return op
	}

	// Check if src1 is a const (for binary ALU: src0=acc, src1=operand)
	if op.Src[1] >= 0 {
		if idx, exists := defMap[op.Src[1]]; exists && ops[idx].Op == OpConst {
			result := op
			result.Op = newOp
			imm := ops[idx].Imm
			// Mask to operand width to prevent overflow (CP 4294967295 → CP 255).
			if op.Width <= 8 {
				imm &= 0xFF
			} else if op.Width <= 16 {
				imm &= 0xFFFF
			}
			result.Imm = imm
			result.Src[1] = -1 // no longer need the const vreg
			return result
		}
	}

	return op
}

// findAllPatterns returns ALL matching patterns for a MIR op, sorted by cost.
// Used by WFC validate-reject to try alternative patterns when the best one
// produces invalid Z80 assembly for all loc assignments.
func findAllPatterns(desc *MachineDesc, op MIROp) []*Pattern {
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
		if !op.DstAllowed.IsEmpty() && !p.DstLocs.IsEmpty() {
			if p.DstLocs.And(op.DstAllowed).IsEmpty() {
				continue
			}
		}
		candidates = append(candidates, p)
	}
	// Sort by cost ascending.
	for i := 1; i < len(candidates); i++ {
		for j := i; j > 0 && candidates[j].Cost < candidates[j-1].Cost; j-- {
			candidates[j], candidates[j-1] = candidates[j-1], candidates[j]
		}
	}
	return candidates
}

// findBridgePattern finds an OpMove pattern whose SrcLocs overlaps curLoc
// and DstLocs overlaps dstAllowed. Handles width-bridging (pair→gpr via trunc).
func findBridgePattern(desc *MachineDesc, curLoc, dstAllowed LocSet) *Pattern {
	var best *Pattern
	bestCost := 1 << 30
	for i := range desc.Patterns {
		p := &desc.Patterns[i]
		if p.MIROp != OpMove {
			continue
		}
		if p.SrcLocs[0].IsEmpty() || p.DstLocs.IsEmpty() {
			continue
		}
		// Pattern's src must overlap where value currently is.
		if p.SrcLocs[0].And(curLoc).IsEmpty() {
			continue
		}
		// Pattern's dst must overlap where we need it.
		if p.DstLocs.And(dstAllowed).IsEmpty() {
			continue
		}
		if p.Cost < bestCost {
			bestCost = p.Cost
			best = p
		}
	}
	return best
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
		// If DstAllowed is set, only consider patterns whose DstLocs intersects it.
		if !op.DstAllowed.IsEmpty() && !p.DstLocs.IsEmpty() {
			if p.DstLocs.And(op.DstAllowed).IsEmpty() {
				continue
			}
		}
		// INC/DEC patterns: dst = src + 1 (implicit constant).
		// Match ONLY when dst == src0 == src1 (same vreg) AND the operation
		// is really increment (not double). If src0 == src1 but they're both
		// real vregs, it's x+x = 2*x (double), not x+1 (increment).
		if desc.Name == "z80" &&
			(p.Name == "inc_r" || p.Name == "dec_r" || p.Name == "inc_rr" || p.Name == "dec_rr") {
			if op.Src[0] >= 0 && op.Src[1] >= 0 {
				if op.Src[0] != op.Src[1] {
					continue // different srcs → not increment
				}
				// Same src for both → this is x+x (double), skip INC.
				if op.Op == OpAdd || op.Op == OpSub {
					continue
				}
			}
		}
		candidates = append(candidates, p)
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no pattern for op=%d width=%d", op.Op, op.Width)
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}

	// Multiple patterns: pick best cost, but UNION all DstLocs and SrcLocs
	// so the allocator has freedom to choose among alternatives.
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
