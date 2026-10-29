package mir2

// Inline-asm clobber constraints.
//
// An inline-asm template writes whatever physical registers its text names;
// the lowerer reduces that to Asm.Clobbers.  A value that has to survive the
// block therefore cannot be homed in a register the template overwrites, or
// the template silently destroys it.  Stack and memory slots are never
// constrained — a template writes registers, and they are what keeps a
// function that clobbers everything allocatable.

// asmClobberAvoid returns, per virtual register, the register classes whose
// locations cannot hold it because it is live across an inline-asm block that
// clobbers them.  "Live across" means read after the block: the backward walk
// below holds exactly that set when it reaches the instruction.
//
// A register defined *by* the block is not live across it, so the walk removes
// the definition before recording — an asm result must keep the home its
// (ret REG) class asks for.
//
// A block that produces no result register can only affect the program through
// the registers it is given, so its inputs are in-out: the template's write to
// the input's own register is how the value escapes, not a clobber of it.  A
// block with a declared result has a declared output, and every other register
// it writes is a clobber its inputs must survive too.
func asmClobberAvoid(f *Func, lr *LivenessResult) map[Reg]ClassSet {
	if lr == nil || !funcHasAsm(f) {
		return nil
	}
	avoid := make(map[Reg]ClassSet)
	record := func(r Reg, clobbers ClassSet) {
		if s := avoid[r]; s == 0 {
			avoid[r] = clobbers
		} else {
			avoid[r] = s.Union(clobbers)
		}
	}
	for bi, b := range f.Blocks {
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
			if inst.Asm != nil && !inst.Asm.Clobbers.Empty() {
				inOut := inst.Dst == NoReg
				for _, r := range inst.Asm.Ins {
					if inOut {
						live.Remove(r)
					}
				}
				live.Each(func(r Reg) { record(r, inst.Asm.Clobbers) })
				for _, r := range inst.Asm.Ins {
					if inOut {
						live.Add(r)
					}
				}
			}
			for _, r := range inst.Uses() {
				live.Add(r)
			}
		}
	}
	if len(avoid) == 0 {
		return nil
	}
	return avoid
}

// locHomesClobbers reports whether loc can hold a value of a class in
// clobbers.  A clobber set names register classes, so a location is unsafe
// when it is a legal home for any of them.
func locHomesClobbers(ct CostTable, clobbers ClassSet, loc PhysLoc) bool {
	switch loc.Kind {
	case LocStack, LocMem:
		return false
	}
	for c := RegClass(0); c < classCount; c++ {
		if clobbers.Has(c) && ct.Cost(c, loc) < InfCost {
			return true
		}
	}
	return false
}

// applyAsmClobberCosts excludes every clobbered location from the cost vector
// of each register that is live across the block, so PBQP prefers — and, when
// nothing else is legal, spills — rather than assigning a register the
// template will overwrite.
func applyAsmClobberCosts(f *Func, lr *LivenessResult, states map[Reg]*regState, locs []PhysLoc, ct CostTable) {
	avoid := asmClobberAvoid(f, lr)
	if avoid == nil {
		return
	}
	for r, clobbers := range avoid {
		state := states[r]
		if state == nil {
			continue
		}
		for i, loc := range locs {
			if locHomesClobbers(ct, clobbers, loc) {
				state.costs[i] = InfCost
			}
		}
	}
}
