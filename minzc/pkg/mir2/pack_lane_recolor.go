package mir2

// Byte-lane homes for pack_bytes.
//
// `w = pack_bytes lo, hi` is w = lo | hi<<8.  When w lives in a register pair
// P, the pack is free if lo already sits in P's low byte and hi in its high
// byte; any other placement makes genPackBytes emit a parallel copy, and the
// worst case — the two bytes in P but crossed — is a byte swap:
//
//	LD D,(HL) / INC HL / LD E,(HL) / PUSH AF / LD A,D / LD D,E / LD E,A / POP AF
//
// PBQP prices each vreg on its own and has no pairwise preference, so nothing
// tells it which half of the pair a byte should take.  recolorPackLanes runs
// after the solve: it moves lo to low(P) and hi to high(P) when no interfering
// vreg holds those bytes and the move keeps every constraint coalescing keeps
// (hard classes, location compatibility, inline-asm clobbers, finite class
// cost).  The codegen's parallel copy stays the fallback for every case this
// does not reach.

// recolorPackLanes applies the lane preference above to result in place.
func recolorPackLanes(f *Func, result *AllocResult, lr *LivenessResult, ct CostTable) {
	if f == nil || result == nil {
		return
	}
	type pack struct{ lo, hi, dst Reg }
	var packs []pack
	for _, b := range f.Blocks {
		for _, x := range b.Insts {
			if x.Op == OpPackBytes && x.Dst != NoReg && x.Src[0] != x.Src[1] {
				packs = append(packs, pack{x.Src[0], x.Src[1], x.Dst})
			}
			// A zero-extended byte is a pack with a zero high lane.
			if x.Op == OpExt && x.Dst != NoReg && x.SrcTy != nil && x.SrcTy.Width() == 8 && x.Ty != nil && x.Ty.Width() == 16 {
				packs = append(packs, pack{x.Src[0], NoReg, x.Dst})
			}
		}
	}
	if len(packs) == 0 {
		return
	}
	hard := map[Reg]bool{}
	for _, p := range f.Contract.Params {
		hard[p.Reg] = true // ABI home
	}
	defs := map[Reg]int{}
	cls := map[Reg]RegClass{}
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			defs[p.Dst] += 2 // a block parameter takes its home from every edge
		}
		for _, x := range b.Insts {
			if x.Dst != NoReg {
				defs[x.Dst]++
				cls[x.Dst] = x.Cls
				hard[x.Dst] = hard[x.Dst] || x.ClsHard
			}
			for _, r := range x.ExtraRets {
				defs[r] += 2
			}
		}
	}
	if f.Attrs.HasAsm {
		return
	}
	info := collectRegInfo(f)
	_, z80Table := ct.(Z80CostTable)
	asmAvoid := asmClobberAvoid(f, lr)
	ig := BuildInterferenceGraph(f, lr)
	// A byte held across a call took a home the callee preserves; the lane
	// preference must not trade that for a PUSH/POP.  Otherwise a move must
	// not raise the vreg's own modelled cost.
	acrossCall := liveAcrossCallRegs(f, lr)
	locs := ct.Locs()
	idx := locIndex(locs)
	costs := nodeCosts(f, info, ct, locs)
	notWorse := func(r Reg, to PhysLoc) bool {
		cv := costs[r]
		i, ok1 := idx[to]
		j, ok2 := idx[result.Locs[r]]
		return ok1 && ok2 && i < len(cv) && j < len(cv) && cv[i] <= cv[j]
	}
	mainByte := func(loc PhysLoc) bool {
		if loc.Kind != LocReg {
			return false
		}
		switch loc.Name {
		case "B", "C", "D", "E", "H", "L":
			return true
		}
		return false
	}
	movable := func(r Reg, to PhysLoc) bool {
		if hard[r] || defs[r] != 1 || !mainByte(result.Locs[r]) {
			return false
		}
		if ri, ok := info[r]; !ok || ri.Ty == nil || ri.Ty.Width() != 8 || !locCompatibleFor(ct, ri.Ty, to) {
			return false
		}
		if z80Table && locHomesClobbers(ct, asmAvoid[r], to) {
			return false
		}
		if acrossCall[r] || !notWorse(r, to) {
			return false
		}
		return ct.Cost(cls[r], to) < InfCost
	}
	// free reports whether r may take to while the vregs in moving take their
	// proposed homes: no other neighbour of r overlaps to.
	free := func(r Reg, to PhysLoc, moving map[Reg]PhysLoc) bool {
		ok := true
		ig.Neighbors(r).Each(func(n Reg) {
			if !ok {
				return
			}
			loc := result.Locs[n]
			if m, moved := moving[n]; moved {
				loc = m
			}
			if physicallyConflicts(loc, to) || physicallyConflicts(to, loc) {
				ok = false
			}
		})
		return ok
	}
	for _, p := range packs {
		home := result.Locs[p.dst]
		if home.Kind != LocReg {
			continue
		}
		switch home.Name {
		case "BC", "DE", "HL":
		default:
			continue
		}
		low := PhysLoc{Kind: LocReg, Name: lowByte(home.Name)}
		high := PhysLoc{Kind: LocReg, Name: highByte(home.Name)}
		moving := map[Reg]PhysLoc{}
		if result.Locs[p.lo] != low && movable(p.lo, low) {
			moving[p.lo] = low
		}
		if p.hi != NoReg && result.Locs[p.hi] != high && movable(p.hi, high) {
			moving[p.hi] = high
		}
		// Try both lanes together first (a crossed pair can only be fixed
		// that way), then each lane on its own.
		try := func(m map[Reg]PhysLoc) bool {
			if len(m) == 0 {
				return false
			}
			for r, to := range m {
				if !free(r, to, m) {
					return false
				}
			}
			for r, to := range m {
				result.Locs[r] = to
			}
			return true
		}
		if try(moving) {
			continue
		}
		for _, r := range []Reg{p.lo, p.hi} {
			if to, ok := moving[r]; ok && r != NoReg {
				try(map[Reg]PhysLoc{r: to})
			}
		}
	}
}
