package mir2

// affineCounter replaces an up-counting index whose only remaining job, once
// its addresses are pointer walks, is the exit test `i < bound`, by the
// down-counter of the iterator lowering: initial value bound, header test
// `counter != 0`, `counter - 1` on the backedge. The plan is computed before
// any edit and applied unchanged, so the profitability decision that relies on
// it and the rewrite cannot disagree.
type affineCounter struct {
	iv   LoopInduction
	cmp  *Inst // header exit test, rewritten in place
	init Reg   // trip count available in the preheader; NoReg when hoisted
	// hoist is the header bound load and its in-loop address DAG, in
	// dependency order, moved to the preheader to become the initial count.
	hoist []*Inst
}

// affineCounterPlan proves the conversion for iv. Every condition is required
// for the trip count `bound - i` to stand in for `i < bound`:
//
//   - the loop has a separate header and one unconditional latch, and the
//     header's only in-loop successor is the exit test's true edge, so the
//     counter is decremented exactly once per completed iteration and the test
//     is evaluated on every header entry (checked by the caller);
//   - the index is a u8/u16 header parameter starting at the constant 0 and
//     advanced by `add i, 1` of its own width on the backedge;
//   - the exit test is `cmp.ult i, bound` (unsigned, operands in this order) in
//     the header, used only by the header terminator, and bound has the
//     index's width. Then 0 <= i <= bound holds at every header entry, i never
//     wraps, and `i < bound` is exactly `bound - i != 0`; the trip count is
//     bound in both forms, including zero trips;
//   - bound has one value for the whole loop: an SSA value defined outside the
//     loop, an invariant header parameter, or a load in the header that
//     staticGlobalLoad admits in a loop that writes no memory and has no
//     unknown effects (loopObscuresMemory). The header runs on every entry, so
//     moving that load to the preheader executes it no more often than before
//     on the entering path; the static-global proof makes the extra read on a
//     guarded zero-trip path unobservable.
//
// Two further conditions keep the conversion a saving: the index has no use
// left once the replaced addresses are gone, and a hoisted load is used only
// by the exit test. Otherwise the index or the bound would stay live next to
// the counter, adding a register pair instead of saving one. apply relies on
// the first: it deletes the index together with every pure value computed
// from it (affineCounterRemoved), so none of those may be needed inside the
// loop (affineNeeded) or live after it (LiveOut).
func affineCounterPlan(f *Func, l *NaturalLoop, iv LoopInduction, replaced map[Reg]bool, defs map[Reg]*Inst, types map[Reg]Ty) *affineCounter {
	ty := iv.Param.Ty
	if l.Header == l.Latch || l.Back == nil || iv.Step != 1 || (ty != TyU8 && ty != TyU16) {
		return nil
	}
	if x := defs[iv.Initial]; x == nil || x.Op != OpConst || x.ClsHard || maskToWidth(x.Imm, ty) != 0 {
		return nil
	}
	if next := defs[iv.Next]; next == nil || next.Op != OpAdd || next.ClsHard || next.Src[0] != iv.Param.Dst {
		return nil
	}
	var cond Reg
	switch t := l.Header.Term.(type) {
	case *TermCondRet:
		cond = t.Cond
	case *TermBrIf:
		if l.Blocks[t.Else] || !l.Blocks[t.Then] {
			return nil
		}
		cond = t.Cond
	default:
		return nil
	}
	inHeader := map[Reg]*Inst{}
	for _, x := range l.Header.Insts {
		if x.Dst != NoReg {
			inHeader[x.Dst] = x
		}
	}
	cmp := inHeader[cond]
	if cmp == nil || cmp.Op != OpCmp || cmp.ClsHard || cmp.Cond != CmpUlt || cmp.Src[0] != iv.Param.Dst {
		return nil
	}
	bound := cmp.Src[1]
	if cmp.SrcTy != nil && cmp.SrcTy.Width() != ty.Width() || types[bound] == nil || types[bound].Width() != ty.Width() {
		return nil
	}
	plan := &affineCounter{iv: iv, cmp: cmp}
	if root, ok := l.Invariant[bound]; ok {
		if types[root] == nil || types[root].Width() != ty.Width() {
			return nil
		}
		plan.init = root
	} else {
		load := inHeader[bound]
		if load == nil || load.Op != OpLoad || load.ClsHard || loopObscuresMemory(f, l.Blocks) || countRegUses(f)[bound] != 1 {
			return nil
		}
		if _, _, ok := staticGlobalLoad(f, load, defs); !ok {
			return nil
		}
		moving := map[*Inst]bool{}
		var emit func(*Inst)
		emit = func(x *Inst) {
			if moving[x] {
				return
			}
			moving[x] = true
			for _, u := range x.Uses() {
				if d := inHeader[u]; d != nil {
					emit(d)
				}
			}
			plan.hoist = append(plan.hoist, x)
		}
		emit(load)
	}
	for r := range affineCounterRemoved(f, l, iv.Param.Dst) {
		if l.LiveOut[r] {
			return nil
		}
	}
	needed := affineNeeded(f, l, replaced, defs, plan)
	if needed[iv.Param.Dst] || needed[iv.Next] {
		return nil
	}
	return plan
}

// apply performs the planned conversion. The types and classes are those of
// the iterator lowering (hir lowerForEach): a byte counter prefers B, a word
// counter DE. The old index is removed here, not left to the cleanup passes:
// DeadBlockArgElim treats every instruction input as a use and DSE keeps an
// increment that feeds a backedge, so `i' = add i, 1` would keep both alive.
// The plan proved that every remaining user of the index is a pure
// instruction inside the loop whose result is not needed, so deleting those
// users and the header parameter preserves the program.
func (c *affineCounter) apply(f *Func, l *NaturalLoop) {
	ty := c.iv.Param.Ty
	cls, zeroCls := ClassCounter, ClassAcc
	if ty.Width() > TyU8.Width() {
		cls, zeroCls = ClassIndex, ClassIndex
	}
	init := c.init
	if init == NoReg {
		moved := map[*Inst]bool{}
		for _, x := range c.hoist {
			moved[x] = true
		}
		out := l.Header.Insts[:0]
		for _, x := range l.Header.Insts {
			if !moved[x] {
				out = append(out, x)
			}
		}
		l.Header.Insts = out
		l.Preheader.Insts = append(l.Preheader.Insts, c.hoist...)
		init = c.hoist[len(c.hoist)-1].Dst
	}
	zero := &Inst{Op: OpConst, Dst: f.AllocReg(), Ty: ty, Cls: zeroCls}
	one := &Inst{Op: OpConst, Dst: f.AllocReg(), Imm: 1, Ty: ty, Cls: cls}
	l.Preheader.Insts = append(l.Preheader.Insts, zero, one)
	counter, next := f.AllocReg(), f.AllocReg()
	l.Header.Params = append(l.Header.Params, BlockParam{Dst: counter, Ty: ty, Class: cls})
	appendAffinePointerEdge(l.Preheader.Term, l.Header.Label, init)
	l.Latch.Insts = append(l.Latch.Insts, &Inst{Op: OpSub, Dst: next, Src: [2]Reg{counter, one.Dst}, Ty: ty, Cls: cls, SrcFile: c.cmp.SrcFile, SrcLine: c.cmp.SrcLine})
	appendAffinePointerEdge(l.Latch.Term, l.Header.Label, next)
	c.cmp.Cond = CmpNe
	c.cmp.Src = [2]Reg{counter, zero.Dst}
	c.cmp.SrcTy = nil

	keep := make([]bool, len(l.Header.Params))
	params := l.Header.Params[:0]
	for i, p := range l.Header.Params {
		keep[i] = p.Dst != c.iv.Param.Dst
		if keep[i] {
			params = append(params, p)
		}
	}
	l.Header.Params = params
	compactEdgeArgs(l.Preheader.Term, l.Header.Label, keep)
	compactEdgeArgs(l.Latch.Term, l.Header.Label, keep)
	dead := affineCounterRemoved(f, l, c.iv.Param.Dst)
	for _, b := range f.Blocks {
		if !l.Blocks[b.Label] {
			continue
		}
		out := b.Insts[:0]
		for _, x := range b.Insts {
			if !dead[x.Dst] || !isDSEPure(x.Op) {
				out = append(out, x)
			}
		}
		b.Insts = out
	}
}

// affineCounterRemoved returns the index and every value a pure instruction
// of the loop computes from it, directly or through other such values: the
// registers that have no definition once the index parameter is gone.
func affineCounterRemoved(f *Func, l *NaturalLoop, index Reg) map[Reg]bool {
	dead := map[Reg]bool{index: true}
	for progress := true; progress; {
		progress = false
		for _, b := range f.Blocks {
			if !l.Blocks[b.Label] {
				continue
			}
			for _, x := range b.Insts {
				if x.Dst == NoReg || dead[x.Dst] || !isDSEPure(x.Op) {
					continue
				}
				for _, r := range x.Uses() {
					if dead[r] {
						dead[x.Dst] = true
						progress = true
						break
					}
				}
			}
		}
	}
	return dead
}
