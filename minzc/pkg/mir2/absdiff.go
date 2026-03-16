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
		// Look for pattern: cmp.ugt(a,b) followed by sub(b,a)
		// where cond_ret uses the cmp result and sub result.
		for i := 0; i < len(b.Insts)-1; i++ {
			cmpInst := b.Insts[i]
			subInst := b.Insts[i+1]

			// Match: cmp.ugt(a, b) or cmp.uge(a, b)
			if cmpInst.Op != OpCmp || (cmpInst.Cond != CmpUgt && cmpInst.Cond != CmpUge) {
				continue
			}
			// Match: sub(b, a) — operands reversed from cmp
			if subInst.Op != OpSub {
				continue
			}
			a, bReg := cmpInst.Src[0], cmpInst.Src[1]
			if subInst.Src[0] != bReg || subInst.Src[1] != a {
				continue
			}

			// Rewrite: swap sub operands to sub(a, b)
			subInst.Src[0] = a
			subInst.Src[1] = bReg

			// Rewrite cmp to use CmpSubCarry — the carry flag from the sub
			// already encodes the comparison result. No separate CP needed.
			// CmpSubCarry means "carry set by preceding SUB" — the codegen
			// recognizes this and emits zero instructions for the cmp.
			cmpInst.Cond = CmpSubCarry
			// Point cmp sources at the sub's dst so that the original params
			// (%r1, %r2) are no longer live across the sub → PBQP can assign
			// sub result to the same register as param (A for ClassAcc).
			cmpInst.Src = [2]Reg{subInst.Dst, NoReg}

			// Swap instruction order: sub first, then cmp.
			b.Insts[i] = subInst
			b.Insts[i+1] = cmpInst

			changed = true
		}
	}
	return changed
}
