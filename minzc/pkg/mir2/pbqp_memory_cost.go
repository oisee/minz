package mir2

// applyByteMemoryCosts models a bounded byte-copy pattern. HL can store an
// arbitrary byte directly; BC/DE require A. When a byte accumulator is live,
// the current emitter saves AF around that store. These are candidate costs,
// not a guarantee of optimal allocation: final locations remain constrained
// by interference, ABI and the ordinary PBQP reductions.
func applyByteMemoryCosts(f *Func, lr *LivenessResult, states map[Reg]*regState, locs []PhysLoc) {
	for bi, b := range f.Blocks {
		definitions := make(map[Reg]*Inst, len(b.Insts))
		for _, inst := range b.Insts {
			if inst.Dst != NoReg {
				definitions[inst.Dst] = inst
			}
		}
		live := lr.LiveOut[bi].Clone()
		if b.Term != nil {
			for _, r := range b.Term.termUses() {
				live.Add(r)
			}
		}
		for ii := len(b.Insts) - 1; ii >= 0; ii-- {
			inst := b.Insts[ii]
			if inst.Op == OpStore && inst.Ty == TyU8 {
				// Only a direct byte load feeding this store is modelled. Constant
				// fills, calls and arbitrary expressions keep their existing costs.
				definition := definitions[inst.Src[1]]
				loaded := definition != nil && definition.Op == OpLoad && definition.Ty == TyU8
				if loaded {
					penalty := 4 // LD A,r before LD (BC/DE),A
					for r, state := range states {
						if r != inst.Src[1] && live.Has(r) && state.info.Ty == TyU8 && state.info.Cls == ClassAcc {
							penalty += 21 // PUSH AF / POP AF under current emitter
							break
						}
					}
					if state := states[inst.Src[0]]; state != nil {
						for i, loc := range locs {
							if (loc.Name == "BC" || loc.Name == "DE") && state.costs[i] < InfCost {
								state.costs[i] += penalty
							}
						}
					}
				}
			}
			live.Remove(inst.Dst)
			for _, r := range inst.ExtraRets {
				live.Remove(r)
			}
			for _, r := range inst.Uses() {
				live.Add(r)
			}
		}
	}
}
