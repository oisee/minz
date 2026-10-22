package mir2

// foldTruncOfExtension cancels a widening conversion followed by truncation
// back to exactly the original width. Both zero and sign extension preserve
// those low bits. Other users keep their extension until dead-store cleanup.
func foldTruncOfExtension(f *Func) bool {
	// MIR can also contain coalesced, multiply-defined registers. Keep the
	// proof local and require unique definitions, including the original source.
	counts := map[Reg]int{}
	for _, p := range f.Contract.Params {
		counts[p.Reg]++
	}
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			counts[p.Dst]++
		}
		for _, i := range b.Insts {
			if i.Dst != NoReg {
				counts[i.Dst]++
			}
		}
	}
	changed := false
	for _, b := range f.Blocks {
		defs := map[Reg]*Inst{}
		for _, i := range b.Insts {
			if i.Op == OpTrunc && !i.ClsHard && i.Ty != nil && i.SrcTy != nil && counts[i.Dst] == 1 {
				e := defs[i.Src[0]]
				if e != nil && (e.Op == OpExt || e.Op == OpSext) && !e.ClsHard && e.Ty != nil && e.SrcTy != nil &&
					counts[e.Dst] == 1 && counts[e.Src[0]] == 1 && e.SrcTy.Width() == i.Ty.Width() &&
					e.Ty.Width() == i.SrcTy.Width() && e.Ty.Width() > e.SrcTy.Width() {
					i.Op, i.Src, i.SrcTy = OpMove, [2]Reg{e.Src[0]}, e.SrcTy
					changed = true
				}
			}
			if i.Dst != NoReg {
				defs[i.Dst] = i
			}
		}
	}
	return changed
}
