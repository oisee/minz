package mir2

// FuseAbsDiff lets a compare reuse the borrow of an adjacent subtraction.
//
// The only fact used is the borrow of r = sub(x, y): it is set exactly when
// x <u y (unsigned, at the width of the sub). A compare is fused only when it
// is that predicate or its complement on the SAME subtraction:
//
//	cmp.ult(x, y) == x <u y  == borrow(sub x,y)  -> cmp.sub_carry(r, y)
//	cmp.ugt(y, x) == x <u y  == borrow(sub x,y)  -> cmp.sub_carry(r, y)
//	cmp.uge(x, y) == x >=u y == !borrow          -> cmp.sub_carry_not(r, y)
//	cmp.ule(y, x) == x >=u y == !borrow          -> cmp.sub_carry_not(r, y)
//
// cmp.ule(x, y), cmp.ugt(x, y), cmp.ult(y, x) and cmp.uge(y, x) are the borrow
// (or its complement) of sub(y, x), a different subtraction, and differ from
// every predicate above when x == y; they are never fused. Signed orderings
// and equality are never fused either.
//
// The sub is moved before the compare when the compare came first, and in a
// cond_ret whose Then block has this block as its only predecessor, a
// subtraction there with the operands reversed becomes neg(r)
// (sub(y, x) == neg(sub(x, y)) modulo 2^width). Example (abs_diff):
//
//	%d = sub %a, %b
//	%f = cmp.sub_carry %d, %b   ; a < b
//	cond_ret %f, [%d], @then    ; SUB C / RET NC
//	@then: %n = neg %d; ret %n  ; NEG / RET
func FuseAbsDiff(f *Func) bool {
	changed := false
	for _, b := range f.Blocks {
		if fuseAbsDiffBlock(f, b) {
			changed = true
		}
	}
	return changed
}

// matchAbsDiffPair reports whether the adjacent instructions i0, i1 are a sub
// and a compare whose result is the borrow of that sub (or its complement),
// according to the table on FuseAbsDiff.
func matchAbsDiffPair(i0, i1 *Inst) (sub, cmp *Inst, cond CmpCond, ok bool) {
	switch {
	case i0.Op == OpSub && i1.Op == OpCmp:
		sub, cmp = i0, i1
	case i0.Op == OpCmp && i1.Op == OpSub:
		cmp, sub = i0, i1
	default:
		return nil, nil, 0, false
	}
	if sub.Dst == NoReg || cmp.Dst == NoReg {
		return nil, nil, 0, false
	}
	x, y := sub.Src[0], sub.Src[1]
	same := cmp.Src == [2]Reg{x, y}
	swapped := cmp.Src == [2]Reg{y, x}
	switch {
	case cmp.Cond == CmpUlt && same, cmp.Cond == CmpUgt && swapped:
		return sub, cmp, CmpSubCarry, true
	case cmp.Cond == CmpUge && same, cmp.Cond == CmpUle && swapped:
		return sub, cmp, CmpSubCarryNot, true
	}
	return nil, nil, 0, false
}

func hasAbsDiffPair(b *Block) bool {
	for i := 0; i+1 < len(b.Insts); i++ {
		if _, _, _, ok := matchAbsDiffPair(b.Insts[i], b.Insts[i+1]); ok {
			return true
		}
	}
	return false
}

func fuseAbsDiffBlock(f *Func, b *Block) bool {
	changed := false
	for i := 0; i+1 < len(b.Insts); i++ {
		sub, cmp, cond, ok := matchAbsDiffPair(b.Insts[i], b.Insts[i+1])
		if !ok {
			continue
		}
		x, y := sub.Src[0], sub.Src[1]
		cmp.Cond = cond
		// Src[0] = sub result, Src[1] = y (the VM recomputes the borrow from them).
		// Dropping x from the compare lets the allocator give the sub result x's register.
		cmp.Src = [2]Reg{sub.Dst, y}
		cmp.SrcTy = sub.Ty
		b.Insts[i], b.Insts[i+1] = sub, cmp

		if cr, ok := b.Term.(*TermCondRet); ok && countPredecessors(f, cr.Then) == 1 {
			if tb := f.BlockByLabel(cr.Then); tb != nil && tb != b {
				for ti, t := range tb.Insts {
					if t.Op == OpSub && t.Src == [2]Reg{y, x} && t.Ty == sub.Ty {
						tb.Insts[ti] = &Inst{Op: OpNeg, Dst: t.Dst, Src: [2]Reg{sub.Dst}, Ty: t.Ty, Cls: t.Cls}
					}
				}
			}
		}
		changed = true
	}
	return changed
}
