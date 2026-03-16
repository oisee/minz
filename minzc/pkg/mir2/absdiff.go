package mir2

// FuseAbsDiff recognizes the abs_diff pattern and rewrites it for optimal Z80 codegen.
//
// Before:
//
//	%f = cmp.ugt %a, %b      ; separate comparison
//	%d = sub %b, %a           ; separate subtraction (reversed operands)
//	cond_ret %f, [%d], @then  ; branch
//	@then: neg %d; ret        ; negate
//
// After:
//
//	%d = sub %a, %b           ; sub + implicit carry (a < b)
//	%f = cmp.sub_carry        ; reuse carry from sub
//	cond_ret %f, [%d], @then  ; if a < b → negate
//	@then: neg %d; ret        ; negate
//
// The rewrite enables Sub+CmpLt flag fusion in Z80 codegen:
// SUB C sets carry when a < b, so no separate CP instruction needed.
// Result: 6B (SUB C; RET NC; NEG; RET) instead of 9B.
func FuseAbsDiff(f *Func) bool {
	changed := false
	for _, b := range f.Blocks {
		if len(b.Insts) < 2 {
			continue
		}
		for i := 0; i < len(b.Insts)-1; i++ {
			inst0 := b.Insts[i]
			inst1 := b.Insts[i+1]

			// Pattern A: cmp.ugt(a,b) followed by sub(b,a)
			// Pattern B: sub(a,b) followed by cmp.ult(a,b)
			var cmpInst, subInst *Inst
			var a, bReg Reg
			patternA := inst0.Op == OpCmp &&
				(inst0.Cond == CmpUgt || inst0.Cond == CmpUge) &&
				inst1.Op == OpSub &&
				inst1.Src[0] == inst0.Src[1] && inst1.Src[1] == inst0.Src[0]
			patternB := inst0.Op == OpSub &&
				inst1.Op == OpCmp &&
				(inst1.Cond == CmpUlt || inst1.Cond == CmpUle) &&
				inst0.Src[0] == inst1.Src[0] && inst0.Src[1] == inst1.Src[1]

			if patternA {
				cmpInst, subInst = inst0, inst1
				a, bReg = cmpInst.Src[0], cmpInst.Src[1]
			} else if patternB {
				subInst, cmpInst = inst0, inst1
				a, bReg = subInst.Src[0], subInst.Src[1]
			} else {
				continue
			}

			// Pattern A: swap sub operands to sub(a, b)
			if patternA {
				subInst.Src[0] = a
				subInst.Src[1] = bReg
			}

			// Rewrite cmp to use CmpSubCarry — the carry flag from the sub
			// already encodes the comparison result. No separate CP needed.
			// CmpSubCarry means "carry set by preceding SUB" — the codegen
			// recognizes this and emits zero instructions for the cmp.
			cmpInst.Cond = CmpSubCarry
			// Src[0] = sub result (a-b), Src[1] = b (needed by VM for carry check).
			// Using sub's dst for Src[0] removes the false live range on param `a`,
			// letting PBQP assign sub result to A (same reg as consumed param).
			cmpInst.Src = [2]Reg{subInst.Dst, bReg}
			cmpInst.SrcTy = subInst.Ty

			// Swap instruction order: sub first, then cmp.
			b.Insts[i] = subInst
			b.Insts[i+1] = cmpInst

			// Also check the cond_ret target block: if it contains sub(b, a)
			// (the reverse subtraction), replace with neg(sub_dst). This turns
			// the "compute b-a from scratch" path into "negate a-b" (2B NEG
			// instead of 3B NEG+ADD or a full sub).
			if cr, ok := b.Term.(*TermCondRet); ok {
				targetLabel := cr.Then
				for _, tb := range f.Blocks {
					if tb.Label != targetLabel {
						continue
					}
					for ti, tInst := range tb.Insts {
						if tInst.Op == OpSub &&
							tInst.Src[0] == bReg && tInst.Src[1] == a {
							// sub(b, a) → neg(sub_result)
							tb.Insts[ti] = &Inst{
								Op:  OpNeg,
								Dst: tInst.Dst,
								Src: [2]Reg{subInst.Dst},
								Ty:  tInst.Ty,
								Cls: tInst.Cls,
							}
						}
					}
				}
			}

			changed = true
		}
	}
	return changed
}
