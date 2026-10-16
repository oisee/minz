package mir2

// fuseByteLoadMoves lets allocation place a byte load directly in its sole
// consumer's preferred register. The memory access stays at its original
// position, with its original volatility and source metadata.
func fuseByteLoadMoves(f *Func) bool {
	if f.Attrs.FallibleCY {
		return false
	}
	for _, r := range f.Contract.Returns {
		if r.Class == ClassFlag {
			return false
		}
	}
	defs := map[Reg]int{}
	for _, p := range f.Contract.Params {
		defs[p.Reg]++
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
	for _, b := range f.Blocks {
		for n := 0; n+1 < len(b.Insts); n++ {
			load, move := b.Insts[n], b.Insts[n+1]
			if load.Op != OpLoad || move.Op != OpMove || load.ClsHard || move.ClsHard ||
				load.Ty == nil || load.Ty.Width() != 8 || move.Ty != load.Ty ||
				load.Dst == NoReg || move.Dst == NoReg || move.Src[0] != load.Dst ||
				defs[load.Dst] != 1 || defs[move.Dst] != 1 || uses[load.Dst] != 1 {
				continue
			}
			load.Dst, load.Cls = move.Dst, move.Cls
			b.Insts = append(b.Insts[:n+1], b.Insts[n+2:]...)
			changed = true
		}
	}
	return changed
}
