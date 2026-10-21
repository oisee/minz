package mir2

// narrowZeroExtendedByteComparisons applies the exact embedding identity
// compare16(zext8(x), zext8(y)) == unsigned_compare8(x, y).
// It also narrows constants in [0,255] and folds constants outside that
// interval, interpreting signed predicates at the original word width.
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
		out := make([]*Inst, 0, len(b.Insts))
		for _, i := range b.Insts {
			if i.Op == OpCmp && !i.ClsHard && !i.FlagsOnly {
				left, right := defs[i.Src[0]], defs[i.Src[1]]
				byteExt := func(x *Inst) bool {
					return x != nil && x.Op == OpExt && !x.ClsHard && BaseOf(x.SrcTy) == TyU8 &&
						(BaseOf(x.Ty) == TyI16 || BaseOf(x.Ty) == TyU16) &&
						definitions[x.Dst] == 1 && definitions[x.Src[0]] == 1
				}
				cond := i.Cond
				valid := true
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
					valid = false
				}
				if valid && byteExt(left) && byteExt(right) && left.Ty == right.Ty {
					i.Src = [2]Reg{left.Src[0], right.Src[0]}
					i.Cond, i.SrcTy = cond, TyU8
					changed = true
				} else if valid && i.SrcTy != nil && i.SrcTy.Width() == 16 {
					for side := 0; side < 2; side++ {
						ext, k := left, right
						if side == 1 {
							ext, k = right, left
						}
						if !byteExt(ext) || k == nil || k.Op != OpConst || k.ClsHard ||
							k.Ty != ext.Ty || definitions[k.Dst] != 1 {
							continue
						}
						// Constants are word bit patterns: -1 and 65535 denote
						// the same word, with ordering set by the predicate.
						value := maskToWidth(k.Imm, k.Ty)
						if value <= 255 {
							// Keep the word constant intact for other consumers.
							byteReg := f.AllocReg()
							out = append(out, &Inst{Op: OpConst, Dst: byteReg, Ty: TyU8,
								Imm: value, Cls: ClassGeneral, SrcFile: i.SrcFile, SrcLine: i.SrcLine})
							i.Src[side], i.Src[1-side] = ext.Src[0], byteReg
							i.Cond, i.SrcTy = cond, TyU8
						} else {
							// Outside [0,255], every admitted predicate has the
							// same result throughout the byte domain. Evaluate
							// at zero with the original word signedness.
							a, b := int64(0), value
							if side == 1 {
								a, b = b, a
							}
							result := foldCmpCond(i.Cond, a, b, ext.Ty)
							*i = Inst{Op: OpConst, Dst: i.Dst, Ty: i.Ty, Cls: i.Cls,
								Imm: result, SrcFile: i.SrcFile, SrcLine: i.SrcLine}
						}
						changed = true
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
