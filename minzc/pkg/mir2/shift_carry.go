package mir2

import "math/bits"

// Use the typed constant value: a byte literal's high bits do not constitute
// a word mask after truncation, and negative signed masks retain their bits.
func shiftCarryMaskBit(i *Inst) (int, bool) {
	if i == nil || i.Op != OpConst || i.Ty == nil || i.Ty.Width() < 1 || i.Ty.Width() > 16 {
		return 0, false
	}
	value := uint16(maskToWidth(i.Imm, i.Ty))
	if bits.OnesCount16(value) != 1 {
		return 0, false
	}
	return bits.TrailingZeros16(value), true
}

// PrepareShiftCarry moves a discarded-bit shift from the mandatory join of a
// small pure diamond/triangle to its test block. This covers shift-add loops:
// the optional addition consumes other values, then every path shifts x. The
// one-bit shift is total and its input must already dominate the test. Arm
// values, memory, calls and trapping computations are never speculated.
func PrepareShiftCarry(f *Func) bool {
	if f == nil || len(f.Blocks) == 0 {
		return false
	}
	dom := reachableDominators(f)
	defs := map[Reg]*Inst{}
	where := map[Reg]*Block{}
	counts := map[Reg]int{}
	for _, p := range f.Contract.Params {
		counts[p.Reg]++
	}
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			counts[p.Dst]++
			where[p.Dst] = b
		}
		for _, i := range b.Insts {
			if i.Dst != NoReg {
				counts[i.Dst]++
				defs[i.Dst] = i
				where[i.Dst] = b
			}
			for _, r := range i.ExtraRets {
				counts[r]++
			}
		}
	}
	available := func(r Reg, h *Block) bool { return counts[r] == 1 && (where[r] == nil || dom[h.Label][where[r].Label]) }
	root := func(r Reg, bit int) Reg {
		for n := 0; n < len(defs); n++ {
			i := defs[r]
			if i == nil || i.Ty == nil || bit >= i.Ty.Width() {
				break
			}
			if i.Op != OpMove && !((i.Op == OpTrunc || i.Op == OpExt) && i.SrcTy != nil && bit < i.SrcTy.Width()) {
				break
			}
			r = i.Src[0]
		}
		return r
	}
	pureArm := func(b *Block) bool {
		for _, i := range b.Insts {
			if i.ClsHard || i.Volatile || !isDSEPure(i.Op) || i.Op == OpCmp || i.Op == OpAddrOf {
				return false
			}
		}
		return true
	}
	changed := false
	for _, head := range f.Blocks {
		br, ok := head.Term.(*TermBrIf)
		if !ok || br.Then == br.Else || len(head.Insts) == 0 {
			continue
		}
		cmp := head.Insts[len(head.Insts)-1]
		if cmp.Op != OpCmp || cmp.Dst != br.Cond || (cmp.Cond != CmpNe && cmp.Cond != CmpEq) {
			continue
		}
		test, zero := defs[cmp.Src[0]], defs[cmp.Src[1]]
		if zero == nil || zero.Op != OpConst || zero.Imm != 0 {
			test, zero = zero, test
		}
		if test == nil || zero == nil || zero.Op != OpConst || zero.Imm != 0 {
			continue
		}
		base, bit := NoReg, -1
		if test.Op == OpAnd {
			for k, r := range test.Src {
				c := defs[r]
				if bitIndex, ok := shiftCarryMaskBit(c); ok {
					base, bit = test.Src[1-k], bitIndex
					break
				}
			}
		}
		if test.Op == OpBitGet {
			base, bit = test.Src[0], int(test.Imm)
		}
		if base == NoReg || bit < 0 || test.Op == OpAnd && (test.Ty == nil || bit >= test.Ty.Width()) {
			continue
		}
		a, b := f.BlockByLabel(br.Then), f.BlockByLabel(br.Else)
		if a == nil || b == nil {
			continue
		}
		var join *Block
		arms := []*Block{}
		aj, aok := a.Term.(*TermJmp)
		bj, bok := b.Term.(*TermJmp)
		switch {
		case aok && aj.Target == b.Label:
			join = b
			arms = []*Block{a}
		case bok && bj.Target == a.Label:
			join = a
			arms = []*Block{b}
		case aok && bok && aj.Target == bj.Target:
			join = f.BlockByLabel(aj.Target)
			arms = []*Block{a, b}
		}
		if join == nil || join == head || join == f.Entry() || !dom[join.Label][head.Label] {
			continue
		}
		legal := true
		for _, arm := range arms {
			if arm == head || arm == join || len(arm.Params) != 0 || countPredecessors(f, arm.Label) != 1 || !pureArm(arm) {
				legal = false
			}
		}
		if countPredecessors(f, join.Label) != 2 {
			legal = false
		}
		if !legal {
			continue
		}
		for index, shift := range join.Insts {
			if (shift.Op != OpShl && shift.Op != OpShr) || shift.ClsHard || shift.Volatile || counts[shift.Dst] != 1 || shift.Ty == nil {
				continue
			}
			w := shift.Ty.Width()
			if w != 8 && w != 16 || shift.Op == OpShl && bit != w-1 || shift.Op == OpShr && bit != 0 || root(shift.Src[0], bit) != root(base, bit) || !available(shift.Src[0], head) {
				continue
			}
			count := defs[shift.Src[1]]
			if count == nil || count.Op != OpConst || count.Imm != 1 || count.ClsHard || counts[count.Dst] != 1 {
				continue
			}
			if !available(count.Dst, head) && where[count.Dst] != join {
				continue
			}
			// If local, move the original count definition too; any other join uses
			// remain dominated by the new location. The shift precedes the predicate.
			out := head.Insts[:len(head.Insts)-1]
			if where[count.Dst] == join {
				out = append(out, count)
			}
			out = append(out, shift, cmp)
			head.Insts = out
			kept := join.Insts[:0]
			for n, i := range join.Insts {
				if n == index || i == count && where[count.Dst] == join {
					continue
				}
				kept = append(kept, i)
			}
			join.Insts = kept
			where[shift.Dst] = head
			where[count.Dst] = head
			changed = true
			break
		}
	}
	// End the old masked value's live range before the shift. Codegen can
	// defer this single-use predicate to the shift's carry; unsupported homes
	// use materializePendingFlag and an ordinary boolean branch instead.
	for _, b := range f.Blocks {
		if len(b.Insts) < 2 {
			continue
		}
		n := len(b.Insts)
		cmp, shift := b.Insts[n-1], b.Insts[n-2]
		if cmp.Op != OpCmp || (cmp.Cond != CmpEq && cmp.Cond != CmpNe) || (shift.Op != OpShl && shift.Op != OpShr) || shift.Ty == nil || shift.ClsHard || shift.Volatile {
			continue
		}
		cond := NoReg
		switch t := b.Term.(type) {
		case *TermBrIf:
			cond = t.Cond
		case *TermCondRet:
			cond = t.Cond
		}
		if cmp.Dst != cond || countRegUses(f)[cond] != 1 {
			continue
		}
		test, zero := defs[cmp.Src[0]], defs[cmp.Src[1]]
		if zero == nil || zero.Op != OpConst || zero.Imm != 0 {
			test, zero = zero, test
		}
		if test == nil || zero == nil || zero.Op != OpConst || zero.Imm != 0 {
			continue
		}
		base, bit := NoReg, -1
		if test.Op == OpAnd {
			for k, r := range test.Src {
				c := defs[r]
				if bitIndex, ok := shiftCarryMaskBit(c); ok {
					base, bit = test.Src[1-k], bitIndex
					break
				}
			}
		}
		if test.Op == OpBitGet {
			base, bit = test.Src[0], int(test.Imm)
		}
		c := defs[shift.Src[1]]
		w := shift.Ty.Width()
		if base == NoReg || bit < 0 || test.Op == OpAnd && (test.Ty == nil || bit >= test.Ty.Width()) || w != 8 && w != 16 || shift.Op == OpShl && bit != w-1 || shift.Op == OpShr && bit != 0 || c == nil || c.Op != OpConst || c.Imm != 1 || root(base, bit) != root(shift.Src[0], bit) {
			continue
		}
		if test.Op == OpAnd && countRegUses(f)[test.Dst] == 1 {
			// Only zero/nonzero is observed. BitGet has the same predicate
			// without an accumulator-only AND result that would force the
			// still-live shift source out of A during allocation.
			test.Op, test.Src, test.Imm = OpBitGet, [2]Reg{base}, int64(bit)
			test.SrcTy = collectRegInfo(f)[base].Ty
			test.Ty = TyU8
		}
		b.Insts[n-2], b.Insts[n-1] = cmp, shift
		changed = true
	}
	if changed {
		DeadStoreElim(f)
		// Commoning leaves a zero-instruction arm. Bypass only its unique
		// branch edge, preserving the exact arguments to the join and loop
		// attribution. This removes the extra jump on the no-polynomial path.
		var removed []string
		for _, head := range f.Blocks {
			br, ok := head.Term.(*TermBrIf)
			if !ok {
				continue
			}
			bypass := func(label *string, args *[]Reg) {
				arm := f.BlockByLabel(*label)
				if arm == nil || arm == head || arm == f.Entry() || len(arm.Insts) != 0 || len(arm.Params) != 0 || len(*args) != 0 || countPredecessors(f, arm.Label) != 1 {
					return
				}
				jmp, ok := arm.Term.(*TermJmp)
				if !ok || jmp.Target == arm.Label || !f.TransferLoopOrigin(arm.Label, head.Label) {
					return
				}
				*label = jmp.Target
				*args = append([]Reg(nil), jmp.Args...)
				removed = append(removed, arm.Label)
			}
			bypass(&br.Then, &br.ThenArgs)
			bypass(&br.Else, &br.ElseArgs)
		}
		for _, label := range removed {
			f.removeBlock(label)
		}
	}
	return changed
}
