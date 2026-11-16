package mir2

// FoldFlagReturnTests removes the zero test of a bool returned in flags.
// For %r = call f(...) where f's contract returns ClassFlag:
//
//	%c = cmp.ne %r, 0   (every use a br_if / cond_ret condition) → %r
//	%c = cmp.eq %r, 0   (every use a br_if condition) → %r, arms swapped
//
// A bool is 0 or 1, so both rewrites keep the MIR meaning; the branch then
// tests the callee's flags directly. The result never becomes a block
// argument: a comparison with any other use is left alone, as is every
// comparison not of this form. It reports whether a function changed.
func FoldFlagReturnTests(m *Module) bool {
	changed := false
	for _, f := range m.Funcs {
		if foldFlagReturnTests(m, f) {
			changed = true
		}
	}
	return changed
}

func foldFlagReturnTests(m *Module, f *Func) bool {
	results := map[Reg]bool{}
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			if inst.Op != OpCall || inst.Dst == NoReg || inst.Ty != TyBool {
				continue
			}
			if callee := m.FuncByName(inst.Sym); callee != nil && flagReturn(callee) {
				results[inst.Dst] = true
			}
		}
	}
	if len(results) == 0 {
		return false
	}
	uses := map[Reg]int{}
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			for _, r := range inst.Uses() {
				uses[r]++
			}
		}
		for _, r := range b.Term.termUses() {
			uses[r]++
		}
	}
	// condUses counts the uses of r as a branch condition; brOnly is false
	// when one of them is a cond_ret, whose arms cannot be swapped.
	condUses := func(r Reg) (n int, brOnly bool) {
		brOnly = true
		for _, b := range f.Blocks {
			switch t := b.Term.(type) {
			case *TermBrIf:
				if t.Cond == r {
					n++
				}
			case *TermCondRet:
				if t.Cond == r {
					n++
					brOnly = false
				}
			}
		}
		return n, brOnly
	}
	zero := func(r Reg) bool {
		v, ok := flagReturnConst(f, r)
		return ok && v == 0
	}
	changed := false
	dropped := map[Reg]bool{}
	for _, b := range f.Blocks {
		kept := b.Insts[:0]
		for _, inst := range b.Insts {
			if inst.Op != OpCmp || inst.Cond != CmpNe && inst.Cond != CmpEq {
				kept = append(kept, inst)
				continue
			}
			r, k := inst.Src[0], inst.Src[1]
			if !results[r] {
				r, k = k, r
			}
			n, brOnly := condUses(inst.Dst)
			if !results[r] || !zero(k) || n == 0 || n != uses[inst.Dst] || inst.Cond == CmpEq && !brOnly {
				kept = append(kept, inst)
				continue
			}
			for _, bb := range f.Blocks {
				switch t := bb.Term.(type) {
				case *TermBrIf:
					if t.Cond == inst.Dst {
						t.Cond = r
						if inst.Cond == CmpEq {
							t.Then, t.Else = t.Else, t.Then
							t.ThenArgs, t.ElseArgs = t.ElseArgs, t.ThenArgs
						}
					}
				case *TermCondRet:
					if t.Cond == inst.Dst {
						t.Cond = r
					}
				}
			}
			uses[r] += n - 1
			uses[k]--
			dropped[k] = true
			changed = true
		}
		b.Insts = kept
	}
	if changed {
		// Drop zero constants the removed comparisons were the last use of.
		for _, b := range f.Blocks {
			kept := b.Insts[:0]
			for _, inst := range b.Insts {
				if inst.Op == OpConst && dropped[inst.Dst] && uses[inst.Dst] == 0 {
					continue
				}
				kept = append(kept, inst)
			}
			b.Insts = kept
		}
	}
	return changed
}
