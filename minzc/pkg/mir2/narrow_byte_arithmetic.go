package mir2

// narrowTruncatedByteArithmetic applies
// trunc8(zext8(x) +/- zext8(y)) == add/sub8(x,y).
// For unsigned-byte sources, both promoted C sums and differences fit int16.
// The original word operation and extensions retain all other consumers.
func narrowTruncatedByteArithmetic(f *Func) bool {
	if f.Attrs.FallibleCY {
		return false
	}
	for _, r := range f.Contract.Returns {
		if r.Class == ClassFlag {
			return false
		}
	}
	counts := map[Reg]int{}
	for _, p := range f.Contract.Params {
		counts[p.Reg]++
	}
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			counts[p.Dst]++
		}
		for _, i := range b.Insts {
			if i.Op == OpAsm {
				return false
			}
			if i.Dst != NoReg {
				counts[i.Dst]++
			}
		}
	}
	changed := false
	for _, b := range f.Blocks {
		defs := map[Reg]*Inst{}
		for _, i := range b.Insts {
			if i.Op == OpTrunc && !i.ClsHard && BaseOf(i.Ty) == TyU8 &&
				(BaseOf(i.SrcTy) == TyI16 || BaseOf(i.SrcTy) == TyU16) {
				wide := defs[i.Src[0]]
				if wide != nil && (wide.Op == OpAdd || wide.Op == OpSub) && !wide.ClsHard &&
					wide.Ty == i.SrcTy && counts[wide.Dst] == 1 {
					left, right := defs[wide.Src[0]], defs[wide.Src[1]]
					ext := func(v *Inst) bool {
						return v != nil && v.Op == OpExt && !v.ClsHard && BaseOf(v.SrcTy) == TyU8 &&
							v.Ty == wide.Ty && counts[v.Dst] == 1 && counts[v.Src[0]] == 1
					}
					if ext(left) && ext(right) {
						i.Op = wide.Op
						i.Src = [2]Reg{left.Src[0], right.Src[0]}
						i.SrcTy = TyU8
						changed = true
					}
				}
			}
			if i.Dst != NoReg {
				defs[i.Dst] = i
			}
		}
	}
	return changed
}
