package mir2

// FuseByteAbsDiffCFG recognizes two opposite byte subtractions in returning
// branches. Unlike a generic hoist, it proves the entire diamond: equality
// returns zero on either path, and unsigned borrow selects the negation.
func FuseByteAbsDiffCFG(f *Func) bool {
	if f.Attrs.FallibleCY || len(f.Contract.Returns) != 1 || f.Contract.Returns[0].Ty != TyU8 || f.Contract.Returns[0].Class == ClassFlag {
		return false
	}
	defs := map[Reg]int{}
	paramTypes := map[Reg]Ty{}
	for _, p := range f.Contract.Params {
		defs[p.Reg]++
		if !p.ClassHard {
			paramTypes[p.Reg] = p.Ty
		}
	}
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			defs[p.Dst]++
		}
		for _, i := range b.Insts {
			if i.Op == OpAsm {
				return false
			}
			if i.Dst != NoReg {
				defs[i.Dst]++
			}
			for _, r := range i.ExtraRets {
				defs[r]++
			}
		}
	}
	uses := countRegUses(f)
	changed := false
	for _, head := range f.Blocks {
		br, ok := head.Term.(*TermBrIf)
		if !ok || len(br.ThenArgs) != 0 || len(br.ElseArgs) != 0 || len(head.Insts) == 0 || br.Then == br.Else {
			continue
		}
		cmp := head.Insts[len(head.Insts)-1]
		if cmp.Op != OpCmp || cmp.Dst != br.Cond || cmp.ClsHard || cmp.Ty != TyBool || (cmp.SrcTy != nil && cmp.SrcTy != TyU8) || defs[cmp.Dst] != 1 || uses[cmp.Dst] != 1 {
			continue
		}
		forwardLabel, reverseLabel := br.Then, br.Else
		switch cmp.Cond {
		case CmpUgt, CmpUge:
		case CmpUlt, CmpUle:
			forwardLabel, reverseLabel = br.Else, br.Then
		default:
			continue
		}
		forward, reverse := f.BlockByLabel(forwardLabel), f.BlockByLabel(reverseLabel)
		if forward == nil || reverse == nil || forward == head || reverse == head || len(forward.Params) != 0 || len(reverse.Params) != 0 || countPredecessors(f, forwardLabel) != 1 || countPredecessors(f, reverseLabel) != 1 {
			continue
		}
		match := func(b *Block, lhs, rhs Reg) *Inst {
			ret, ok := b.Term.(*TermRet)
			if !ok || len(ret.Vals) != 1 || len(b.Insts) != 1 {
				return nil
			}
			i := b.Insts[0]
			if i.Op != OpSub || i.Ty != TyU8 || i.ClsHard || i.Src != [2]Reg{lhs, rhs} || i.Dst == NoReg || i.Dst != ret.Vals[0] || defs[i.Dst] != 1 || uses[i.Dst] != 1 {
				return nil
			}
			return i
		}
		x, y := cmp.Src[0], cmp.Src[1]
		sub, opposite := match(forward, x, y), match(reverse, y, x)
		if sub == nil || opposite == nil || defs[x] != 1 || defs[y] != 1 || !f.canTransferLoopOrigin(forwardLabel, head.Label) {
			continue
		}
		// Both operands must be available before the comparison, with exact byte
		// types and no hard physical requirements. No cross-block guesswork.
		types := map[Reg]Ty{}
		for r, t := range paramTypes {
			types[r] = t
		}
		for _, p := range head.Params {
			types[p.Dst] = p.Ty
		}
		for _, i := range head.Insts[:len(head.Insts)-1] {
			if !i.ClsHard {
				types[i.Dst] = i.Ty
			}
		}
		if types[x] != TyU8 || types[y] != TyU8 {
			continue
		}
		f.TransferLoopOrigin(forwardLabel, head.Label)
		cmp.Cond, cmp.Src = CmpSubCarry, [2]Reg{sub.Dst, y}
		cmp.SrcTy = TyU8 // explicit synthetic carry width, including Nanz's nil source metadata
		head.Insts = append(head.Insts[:len(head.Insts)-1], sub, cmp)
		head.Term = &TermCondRet{Cond: cmp.Dst, Vals: []Reg{sub.Dst}, Then: reverseLabel}
		opposite.Op, opposite.Src, opposite.SrcTy = OpNeg, [2]Reg{sub.Dst}, TyU8
		f.removeBlock(forwardLabel)
		changed = true
	}
	return changed
}
