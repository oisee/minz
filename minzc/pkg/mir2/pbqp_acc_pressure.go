package mir2

// applyAccumulatorPressureCosts charges the A home of a byte value that stays
// live across an instruction which needs A as its own working register: a byte
// ALU operation or comparison (the Z80 has them only as A-forms) and a byte
// load (BC/DE loads go through A). Keeping the value in A there costs the
// emitter an EX AF,AF' pair or a save/restore move, about 8T, every time the
// instruction runs, so the charge is scaled by loop nesting (×8 per level, at
// most three levels). Values the instruction itself reads are not charged:
// A is where an ALU operand or comparison lhs wants to be.
//
// These are candidate costs like applyByteMemoryCosts; interference, ABI pins
// and the ordinary PBQP reductions still decide the final homes.
func applyAccumulatorPressureCosts(f *Func, lr *LivenessResult, states map[Reg]*regState, locs []PhysLoc) {
	aIdx := -1
	for i, loc := range locs {
		if loc.Kind == LocReg && loc.Name == "A" {
			aIdx = i
		}
	}
	if aIdx < 0 {
		return
	}
	for r, cost := range accumulatorPressure(f, lr) {
		if state := states[r]; state != nil && state.costs[aIdx] < InfCost {
			state.costs[aIdx] += cost
		}
	}
}

// accumulatorPressure returns, per byte value, the modelled cost of homing it
// in A (see applyAccumulatorPressureCosts).
func accumulatorPressure(f *Func, lr *LivenessResult) map[Reg]int {
	if lr == nil {
		return nil
	}
	info := collectRegInfo(f)
	depth := map[string]int{}
	for _, loop := range AnalyzeNaturalLoops(f) {
		for label := range loop.Blocks {
			depth[label]++
		}
	}
	defs := map[Reg]*Inst{}
	consts := map[Reg]int64{}
	for _, b := range f.Blocks {
		for _, i := range b.Insts {
			if i.Dst != NoReg {
				defs[i.Dst] = i
			}
			if i.Op == OpConst {
				consts[i.Dst] = i.Imm
			}
		}
	}
	const perUse = 8 // EX AF,AF' twice, or LD r,A / LD A,r
	pressure := map[Reg]int{}
	for bi, b := range f.Blocks {
		weight := 1
		for d := 0; d < depth[b.Label] && d < 3; d++ {
			weight *= 8
		}
		live := lr.LiveOut[bi].Clone()
		if b.Term != nil {
			for _, r := range b.Term.termUses() {
				live.Add(r)
			}
		}
		for ii := len(b.Insts) - 1; ii >= 0; ii-- {
			inst := b.Insts[ii]
			live.Remove(inst.Dst)
			for _, r := range inst.ExtraRets {
				live.Remove(r)
			}
			if needsAccumulator(inst) && !bitTestable(inst, defs, consts) {
				read := map[Reg]bool{}
				for _, r := range inst.Uses() {
					read[r] = true
				}
				live.Each(func(r Reg) {
					if ri, ok := info[r]; !read[r] && ok && ri.Ty != nil && ri.Ty.Width() <= 8 {
						pressure[r] += perUse * weight
					}
				})
			}
			for _, r := range inst.Uses() {
				live.Add(r)
			}
		}
	}
	return pressure
}

// loopAccumulatorPressure is the pressure at which a value is kept out of A by
// the post-allocation coalescer too: one A-working instruction inside a loop.
const loopAccumulatorPressure = 64

// needsAccumulator reports byte operations whose Z80 forms work through A.
func needsAccumulator(inst *Inst) bool {
	byteTy := func(t Ty) bool { return t != nil && t.Width() <= 8 && t.Width() > 1 }
	switch inst.Op {
	case OpLoad:
		return byteTy(inst.Ty)
	case OpAnd, OpOr, OpXor, OpAdd, OpSub:
		return byteTy(inst.Ty)
	case OpCmp:
		return byteTy(inst.SrcTy)
	}
	return false
}

// bitTestable reports a single-bit mask and its zero test, which the emitter
// fuses into BIT n,r (or a shift carry) without touching A.
func bitTestable(inst *Inst, defs map[Reg]*Inst, consts map[Reg]int64) bool {
	single := func(r Reg) bool {
		v, ok := consts[r]
		return ok && v > 0 && v&(v-1) == 0
	}
	switch inst.Op {
	case OpAnd:
		return single(inst.Src[0]) || single(inst.Src[1])
	case OpCmp:
		if inst.Cond != CmpEq && inst.Cond != CmpNe {
			return false
		}
		if v, ok := consts[inst.Src[1]]; !ok || v != 0 {
			return false
		}
		d := defs[inst.Src[0]]
		return d != nil && d.Op == OpAnd && (single(d.Src[0]) || single(d.Src[1]))
	}
	return false
}

// loopCounters returns the induction variables of f's natural loops and
// their step results: the values the coalescer keeps out of A.
func loopCounters(f *Func) map[Reg]bool {
	counters := map[Reg]bool{}
	for _, loop := range AnalyzeNaturalLoops(f) {
		for _, ind := range loop.Inductions {
			counters[ind.Param.Dst] = true
			if ind.Next != NoReg {
				counters[ind.Next] = true
			}
		}
	}
	return counters
}
