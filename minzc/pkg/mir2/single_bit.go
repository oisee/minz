package mir2

import "reflect"

// CanonicalizeSingleBitTests maps equality with the sole set bit to a zero
// test. For a one-bit mask, (x&m) has exactly the two values 0 and m,
// independently of signedness. The AND and its other consumers stay intact.
// Bare integer conditions already lower to CmpNe against zero in HIR.
func CanonicalizeSingleBitTests(f *Func) bool {
	if f == nil || f.Attrs.FallibleCY || f.Attrs.HasAsm {
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
			for _, r := range i.ExtraRets {
				counts[r]++
			}
		}
	}
	integer := func(t Ty) bool { return t == TyU8 || t == TyI8 || t == TyU16 || t == TyI16 }
	plain := func(i *Inst, constant bool) bool {
		c := *i
		c.Op, c.Dst, c.Src, c.Ty, c.Cls = 0, 0, [2]Reg{}, nil, 0
		c.SrcLine, c.SrcFile = 0, ""
		if constant {
			c.Imm = 0
		}
		return reflect.DeepEqual(c, Inst{})
	}
	changed := false
	for _, b := range f.Blocks {
		defs := map[Reg]*Inst{}
		var out []*Inst
		for _, i := range b.Insts {
			if i.Op == OpCmp && i.Ty == TyBool && (i.Cond == CmpEq || i.Cond == CmpNe) && counts[i.Dst] == 1 {
				clean := *i
				clean.Cond, clean.SrcTy = 0, nil
				if plain(&clean, false) {
					for side := 0; side < 2; side++ {
						a, k := defs[i.Src[side]], defs[i.Src[1-side]]
						if a == nil || k == nil || a.Op != OpAnd || !integer(a.Ty) || counts[a.Dst] != 1 || !plain(a, false) || k.Op != OpConst || k.Ty != a.Ty || counts[k.Dst] != 1 || !plain(k, true) {
							continue
						}
						if i.SrcTy != nil && i.SrcTy != a.Ty {
							continue
						}
						var mask *Inst
						for _, r := range a.Src {
							m := defs[r]
							if m != nil && m.Ty == a.Ty && counts[m.Dst] == 1 && plain(m, true) {
								if _, ok := shiftCarryMaskBit(m); ok {
									mask = m
									break
								}
							}
						}
						if mask == nil {
							continue
						}
						value := maskToWidth(k.Imm, k.Ty)
						if value != 0 && value != maskToWidth(mask.Imm, mask.Ty) {
							continue
						}
						if value != 0 {
							zero := f.AllocReg()
							out = append(out, &Inst{Op: OpConst, Dst: zero, Ty: a.Ty, Cls: ClassGeneral, SrcLine: i.SrcLine, SrcFile: i.SrcFile})
							i.Src = [2]Reg{a.Dst, zero}
							if i.Cond == CmpEq {
								i.Cond = CmpNe
							} else {
								i.Cond = CmpEq
							}
							changed = true
						} else if side == 1 {
							i.Src[0], i.Src[1] = i.Src[1], i.Src[0]
							changed = true
						}
						break
					}
				}
			}
			out = append(out, i)
			if i.Dst != NoReg {
				defs[i.Dst] = i
			}
		}
		b.Insts = out
	}
	return changed
}
