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
	pressure, _ := accumulatorPressureAndCredit(f, lr)
	return pressure
}

// accumulatorPressureAndCredit also returns, per value, what homing it outside
// A costs the A-form operations that take it as their left operand.
func accumulatorPressureAndCredit(f *Func, lr *LivenessResult) (map[Reg]int, map[Reg]int) {
	if lr == nil {
		return nil, nil
	}
	info := collectRegInfo(f)
	depth := map[string]int{}
	// Only loop induction variables (counters such as i and i+1) are priced
	// out of A: they are read by little more than an INC and a compare, so A
	// buys them almost nothing while it costs every byte load and ALU step of
	// the body. Accumulators such as a running CRC keep their A preference.
	counters := map[Reg]bool{}
	consts := map[Reg]int64{}
	for _, b := range f.Blocks {
		for _, i := range b.Insts {
			if i.Op == OpConst {
				consts[i.Dst] = i.Imm
			}
		}
	}
	for _, loop := range AnalyzeNaturalLoops(f) {
		for label := range loop.Blocks {
			depth[label]++
		}
		for _, ind := range loop.Inductions {
			counters[ind.Param.Dst] = true
			counters[ind.Next] = true
		}
	}
	const perUse = 8 // EX AF,AF' twice, or LD r,A / LD A,r
	pressure := map[Reg]int{}
	credit := map[Reg]int{}
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
			// Credit: an A-form operation whose left operand is r wants r in A;
			// elsewhere it pays LD A,r (and LD r,A) instead.
			if (needsAccumulator(inst) || isByteShift(inst)) && !byteConstStep(inst, consts) {
				if r := inst.Src[0]; r != NoReg {
					credit[r] += perUse * weight
				}
			}
			if needsAccumulator(inst) {
				read := map[Reg]bool{}
				for _, r := range inst.Uses() {
					read[r] = true
				}
				live.Each(func(r Reg) {
					if ri, ok := info[r]; (counters[r] || allBytesCharged) && !read[r] && ok && ri.Ty != nil && ri.Ty.Width() <= 8 {
						pressure[r] += perUse * weight
					}
				})
			}
			for _, r := range inst.Uses() {
				live.Add(r)
			}
		}
	}
	net := map[Reg]int{}
	for r, c := range credit {
		if left := c - pressure[r]; left > 0 {
			net[r] = left
		}
		if pressure[r] -= c; pressure[r] <= 0 {
			delete(pressure, r)
		}
	}
	return pressure, net
}

// isByteShift reports byte shifts, which the emitter prefers in A (ADD A,A).
func isByteShift(inst *Inst) bool {
	return (inst.Op == OpShl || inst.Op == OpShr) && inst.Ty != nil && inst.Ty.Width() == 8
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

// byteConstStep reports a byte add/sub of 1..3, emitted as INC/DEC on any
// register: it gives its operand no reason to live in A.
func byteConstStep(inst *Inst, consts map[Reg]int64) bool {
	if (inst.Op != OpAdd && inst.Op != OpSub) || inst.Ty == nil || inst.Ty.Width() != 8 {
		return false
	}
	cv, ok := consts[inst.Src[1]]
	return ok && cv >= 1 && cv <= 3
}

const allBytesCharged = true
