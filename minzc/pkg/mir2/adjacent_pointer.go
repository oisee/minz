package mir2

// ReuseAdjacentPointers turns base+(idx+k) into (base+idx)+k. The offset
// addition must wrap at the pointer's own width, never at a narrow byte width.
// Only short same-block distances are considered; memory operations stay put.
func ReuseAdjacentPointers(f *Func) int {
	if f == nil || verifyFunc(f) != nil || !wordMaskFlagsUnobserved(f, NoReg) {
		return 0
	}
	info := collectRegInfo(f)
	changed := 0
	for _, b := range f.Blocks {
		type key struct{ base, index Reg }
		type previous struct {
			reg      Reg
			position int
		}
		prior := map[key]previous{}
		defs := map[Reg]*Inst{}
		for position, x := range b.Insts {
			if x.Op == OpPtrAdd && x.Ty != nil && x.Ty.Width() == 16 && !x.ClsHard {
				k := key{x.Src[0], x.Src[1]}
				off := defs[x.Src[1]]
				if off != nil && off.Op == OpAdd && off.Ty != nil && off.Ty.Width() == 16 && !off.ClsHard {
					for n := 0; n < 2; n++ {
						c := defs[off.Src[n]]
						idx := off.Src[1-n]
						if c == nil || c.Op != OpConst || c.Ty == nil || info[idx].Ty == nil || info[idx].Ty.Width() != 16 {
							continue
						}
						step := truncate(Value{I: c.Imm}, c.Ty).I
						old, ok := prior[key{k.base, idx}]
						if !ok || step < 1 || step > 3 || position-old.position > 16 {
							continue
						}
						x.Op = OpField
						x.Src = [2]Reg{old.reg}
						x.Imm = step
						changed++
						break
					}
				}
				prior[k] = previous{x.Dst, position}
			}
			if x.Dst != NoReg {
				defs[x.Dst] = x
			}
		}
	}
	if changed > 0 {
		DeadStoreElim(f)
	}
	return changed
}
