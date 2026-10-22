package mir2

// PackBytePairs exposes disjoint byte lanes before allocation. Only pure,
// single-use word arithmetic is retired; byte loads/calls stay at their sites.
func PackBytePairs(f *Func) int {
	if f == nil || verifyFunc(f) != nil {
		return 0
	}
	info := collectRegInfo(f)
	uses := countRegUses(f)
	changed := 0
	for _, b := range f.Blocks {
		defs := map[Reg]*Inst{}
		for _, x := range b.Insts {
			if (x.Op == OpOr || x.Op == OpAdd) && x.Ty != nil && x.Ty.Width() == 16 && !x.ClsHard && wordMaskFlagsUnobserved(f, x.Dst) {
				byteOf := func(r Reg) Reg {
					d := defs[r]
					if d != nil && d.Op == OpExt && d.SrcTy == TyU8 && byteLaneType(info[d.Src[0]].Ty) && d.Ty != nil && d.Ty.Width() == 16 && !d.ClsHard && uses[r] == 1 {
						return d.Src[0]
					}
					return NoReg
				}
				highOf := func(r Reg) Reg {
					d := defs[r]
					if d == nil || d.Ty != x.Ty || d.ClsHard || uses[r] != 1 {
						return NoReg
					}
					if d.Op == OpShl {
						if c := defs[d.Src[1]]; c != nil && c.Op == OpConst && c.Imm == 8 {
							return byteOf(d.Src[0])
						}
					}
					if d.Op == OpMul {
						for k := 0; k < 2; k++ {
							if c := defs[d.Src[k]]; c != nil && c.Op == OpConst && c.Imm == 256 {
								return byteOf(d.Src[1-k])
							}
						}
					}
					return NoReg
				}
				for k := 0; k < 2; k++ {
					lo, hi := byteOf(x.Src[k]), highOf(x.Src[1-k])
					if lo != NoReg && hi != NoReg {
						x.Op = OpPackBytes
						x.Src = [2]Reg{lo, hi}
						changed++
						break
					}
				}
			}
			if x.Dst != NoReg {
				defs[x.Dst] = x
			}
		}
	}
	if changed != 0 {
		DeadStoreElim(f)
	}
	return changed
}

func (g *z80cg) genPackBytes(inst *Inst) {
	g.materializePendingFlag(inst)
	dst := g.loc(inst.Dst)
	lo, hi := lowByte(dst), highByte(dst)
	if isSpill(dst) {
		lo, hi = dst, dst+"+1"
	}
	source := func(r Reg) string {
		s := g.loc(r)
		if isPairReg(s) {
			return lowByte(s)
		}
		return s
	}
	g.comment("pack disjoint byte lanes")
	// Parallel byte copies handle swapped halves, A, index halves and spills.
	g.emitParallelCopy([]parallelCopy{{srcName: source(inst.Src[0]), dstName: lo, ty: TyU8}, {srcName: source(inst.Src[1]), dstName: hi, ty: TyU8}})
	g.invalidate(dst)
}

// Signed byte values are stored as their eight-bit representation too. An
// explicit unsigned extension can consume such a value after a same-width
// C reinterpretation cast; the extension, not the register type, proves zext.
func byteLaneType(ty Ty) bool { return IsInt(BaseOf(ty)) && ty.Width() == 8 }
