package mir2

// Direct-store address constants.
//
// `p = addr_of @g` whose only use in the whole function is the address of a
// following `store p, v : u16` in the same block is never read out of a home
// when v lives in HL, DE or BC: the Z80 store names the symbol itself
// (`LD (g),HL` / `LD (g),DE` / `LD (g),BC`, z80codegen_inst.go genStore).  The
// address therefore has no register lifetime.  Giving it one anyway makes it
// interfere with v — v is live at p's definition or p is live at v's — so the
// allocator keeps the two in different pairs, and a call result already in HL
// is copied into DE only so the address can sit in HL unread:
//
//	CALL f / LD D,H / LD E,L / LD (result),DE      (main, 2 copies, 8 T)
//	CALL f / LD (result),HL                        (address held no home)
//
// computeDeadConsts reports these addresses dead (their LD is not emitted) and
// constRematCandidates admits them for rematerialisation, which removes them
// from the interference graph.  PBQPAllocate re-checks the set against the
// final allocation, so a re-solve that moves v out of the three pairs drops
// the address again and it gets a home as before.

// directStoreAddressConstants returns the addr_of vregs of f whose single use
// is the address operand of a non-volatile u16 store in the defining block
// whose stored value ar places in HL, DE or BC.  The symbol must be a module
// global (the same test the codegen's direct-store path applies).  With
// ar == nil the value-location condition cannot be checked and nothing is
// returned.
func directStoreAddressConstants(f *Func, ar *AllocResult) map[Reg]bool {
	out := map[Reg]bool{}
	for r := range directStoreAddressValues(f, ar) {
		out[r] = true
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// contestedDirectStores returns the direct-store addresses whose trial home
// under ar is a location the stored value's cost vector prices strictly
// below the value's own trial home.
func contestedDirectStores(f *Func, ar *AllocResult, costs map[Reg][]int, locs []PhysLoc) map[Reg]bool {
	idx := locIndex(locs)
	var out map[Reg]bool
	for addr, value := range directStoreAddressValues(f, ar) {
		cv := costs[value]
		i, ok1 := idx[ar.Locs[addr]]
		j, ok2 := idx[ar.Locs[value]]
		if !ok1 || !ok2 || i >= len(cv) || j >= len(cv) || cv[i] >= cv[j] {
			continue
		}
		if out == nil {
			out = map[Reg]bool{}
		}
		out[addr] = true
	}
	return out
}

// directStoreAddressValues maps each address directStoreAddressConstants
// reports to the value its store writes.
func directStoreAddressValues(f *Func, ar *AllocResult) map[Reg]Reg {
	if f == nil || f.mod == nil || ar == nil {
		return nil
	}
	globals := make(map[string]bool, len(f.mod.Globals))
	for _, gl := range f.mod.Globals {
		globals[gl.Name] = true
	}
	if len(globals) == 0 {
		return nil
	}
	defs := map[Reg]int{}
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			defs[p.Dst] += 2 // never a single definition
		}
		for _, inst := range b.Insts {
			if inst.Dst != NoReg {
				defs[inst.Dst]++
			}
			for _, r := range inst.ExtraRets {
				defs[r]++
			}
		}
	}
	uses := map[Reg]int{}
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			for _, r := range inst.Uses() {
				uses[r]++
			}
		}
		if b.Term != nil {
			for _, r := range b.Term.termUses() {
				uses[r]++
			}
		}
	}
	var out map[Reg]Reg
	for _, b := range f.Blocks {
		local := map[Reg]*Inst{}
		for _, inst := range b.Insts {
			if inst.Op == OpAddrOf && inst.Dst != NoReg {
				local[inst.Dst] = inst
			}
			if inst.Op != OpStore || inst.Volatile || inst.Ty == nil || inst.Ty.Width() != 16 || len(inst.Src) < 2 {
				continue
			}
			addr, value := inst.Src[0], inst.Src[1]
			def := local[addr]
			if def == nil || def.Volatile || !globals[def.Sym] || addr == value {
				continue
			}
			if defs[addr] != 1 || uses[addr] != 1 {
				continue
			}
			switch ar.Locs[value].Name {
			case "HL", "DE", "BC":
			default:
				continue
			}
			if ar.Locs[value].Kind != LocReg {
				continue
			}
			if out == nil {
				out = map[Reg]Reg{}
			}
			out[addr] = value
		}
	}
	return out
}

// directStoreViaHL reports whether a byte store of val (a register name) to a
// direct global address may go through HL: val is a main byte register other
// than H/L and nothing live after inst is in HL.
func (g *z80cg) directStoreViaHL(inst *Inst, val string) bool {
	switch val {
	case "A", "B", "C", "D", "E":
	default:
		return false
	}
	for _, loc := range g.physOverride {
		if loc == "HL" || loc == "H" || loc == "L" {
			return false
		}
	}
	for r := range g.regsLiveAfterInst(inst) {
		switch g.loc(r) {
		case "HL", "H", "L":
			return false
		}
	}
	return true
}

// constStoreBetterThroughHome reports whether a direct constant byte store
// should keep the ordinary path through its address register's home: the
// direct path (LD HL,sym / LD (HL),n, z80codegen_inst.go) must save HL when
// a value is live there (PUSH/POP, 41 T), while an address already homed in
// DE or BC stores with LD rr,sym / LD A,n / LD (rr),A (24 T).
func (g *z80cg) constStoreBetterThroughHome(store *Inst, addr Reg, defs map[Reg]*Inst) bool {
	if d := defs[store.Src[1]]; d == nil || d.Op != OpConst {
		return false
	}
	if g.ar == nil || g.curBlock == nil {
		return false
	}
	switch home := g.ar.Loc(addr); {
	case home.Kind == LocReg && (home.Name == "DE" || home.Name == "BC"):
	default:
		return false
	}
	for r := range g.regsLiveAfterInst(store) {
		switch g.ar.Loc(r).Name {
		case "HL", "H", "L":
			return true
		}
	}
	return false
}
