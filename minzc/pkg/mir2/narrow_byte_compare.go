package mir2

// narrowZeroExtendedByteComparisons applies the exact embedding identity
// compare16(zext8(x), zext8(y)) == unsigned_compare8(x, y).
// Both widened values are in [0,255], including for signed word predicates.
// It keeps the extensions and all their other consumers intact. Ordinary DSE
// may later remove extensions that no longer have users.
func narrowZeroExtendedByteComparisons(f *Func) bool {
	if f.Attrs.FallibleCY {
		return false
	}
	for _, r := range f.Contract.Returns {
		if r.Class == ClassFlag {
			return false
		}
	}
	definitions := map[Reg]int{}
	for _, p := range f.Contract.Params {
		definitions[p.Reg]++
	}
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			definitions[p.Dst]++
		}
		for _, i := range b.Insts {
			// Raw flag observations are outside this value-level identity.
			if i.Op == OpAsm {
				return false
			}
			if i.Dst != NoReg {
				definitions[i.Dst]++
			}
		}
	}
	changed := false
	for _, b := range f.Blocks {
		defs := map[Reg]*Inst{}
		for _, i := range b.Insts {
			if i.Op == OpCmp && !i.ClsHard {
				left, right := defs[i.Src[0]], defs[i.Src[1]]
				byteExt := func(x *Inst) bool {
					return x != nil && x.Op == OpExt && !x.ClsHard && BaseOf(x.SrcTy) == TyU8 &&
						(BaseOf(x.Ty) == TyI16 || BaseOf(x.Ty) == TyU16) &&
						definitions[x.Dst] == 1 && definitions[x.Src[0]] == 1
				}
				if byteExt(left) && byteExt(right) && left.Ty == right.Ty {
					cond := i.Cond
					switch cond {
					case CmpEq, CmpNe, CmpUlt, CmpUle, CmpUgt, CmpUge:
					case CmpLt:
						cond = CmpUlt
					case CmpLe:
						cond = CmpUle
					case CmpGt:
						cond = CmpUgt
					case CmpGe:
						cond = CmpUge
					default:
						defs[i.Dst] = i
						continue
					}
					i.Src = [2]Reg{left.Src[0], right.Src[0]}
					i.Cond = cond
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
