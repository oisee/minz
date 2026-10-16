package mir2

import "reflect"

// NarrowLowByteMaskTests uses the exact identity
// ((word & mask) ==/!= 0) == ((trunc8(word) & mask8) ==/!= 0),
// for 0 < mask <= 255. Word signedness is irrelevant to Eq/Ne. The
// original word AND stays intact for any additional value consumers.
func NarrowLowByteMaskTests(f *Func) bool {
	if f == nil || f.Attrs.FallibleCY || f.Attrs.HasAsm {
		return false
	}
	for _, r := range f.Contract.Returns {
		if r.Class == ClassFlag {
			return false
		}
	}
	counts := map[Reg]int{}
	types := map[Reg]Ty{}
	for _, p := range f.Contract.Params {
		counts[p.Reg]++
		types[p.Reg] = p.Ty
	}
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			counts[p.Dst]++
			types[p.Dst] = p.Ty
		}
		for _, x := range b.Insts {
			if x.Op == OpAsm {
				return false
			}
			if x.Dst != NoReg {
				counts[x.Dst]++
				types[x.Dst] = x.Ty
			}
		}
	}
	word := func(t Ty) bool { return t == TyU16 || t == TyI16 }
	plain := func(x *Inst, constant bool) bool {
		y := *x
		y.Op, y.Dst, y.Src, y.Ty, y.Cls = 0, 0, [2]Reg{}, nil, 0
		y.SrcLine, y.SrcFile = 0, ""
		if constant {
			y.Imm = 0
		}
		return reflect.DeepEqual(y, Inst{})
	}
	uses := countRegUses(f)
	changed := false
	for _, b := range f.Blocks {
		defs := map[Reg]*Inst{}
		var out []*Inst
		for _, x := range b.Insts {
			admit := x.Op == OpCmp && x.Ty == TyBool && (x.Cond == CmpEq || x.Cond == CmpNe) && !x.ClsHard && !x.FlagsOnly && counts[x.Dst] == 1
			if admit {
				clean := *x
				clean.Cond, clean.SrcTy = 0, nil
				admit = plain(&clean, false)
			}
			if admit {
				for side := 0; side < 2; side++ {
					a, z := defs[x.Src[side]], defs[x.Src[1-side]]
					if a == nil || z == nil || a.Op != OpAnd || !word(a.Ty) || !plain(a, false) || counts[a.Dst] != 1 || z.Op != OpConst || !(word(z.Ty) || z.Ty == TyU8 || z.Ty == TyI8) || z.Imm != 0 || !plain(z, true) || counts[z.Dst] != 1 {
						continue
					}
					if x.SrcTy != nil && x.SrcTy.Width() != 16 {
						continue
					}
					var k *Inst
					var value Reg
					for n := 0; n < 2; n++ {
						v := defs[a.Src[n]]
						if v != nil && v.Op == OpConst && word(v.Ty) && v.Imm > 0 && v.Imm <= 255 && plain(v, true) && counts[v.Dst] == 1 {
							k = v
							value = a.Src[1-n]
							break
						}
					}
					if k == nil || counts[value] != 1 || !word(types[value]) {
						continue
					}
					// Preserve the existing single-use memory BIT pattern. Adding a
					// trunc between Load and And would force a full word load and
					// defeat its checked byte-only native access.
					if load := defs[value]; load != nil && load.Op == OpLoad && !load.Volatile && uses[value] == 1 && uses[a.Dst] == 1 && k.Imm&(k.Imm-1) == 0 {
						continue
					}
					low, mask, test, zero := f.AllocReg(), f.AllocReg(), f.AllocReg(), f.AllocReg()
					out = append(out,
						&Inst{Op: OpTrunc, Dst: low, Src: [2]Reg{value}, SrcTy: a.Ty, Ty: TyU8, Cls: ClassGeneral, SrcLine: x.SrcLine, SrcFile: x.SrcFile},
						&Inst{Op: OpConst, Dst: mask, Imm: k.Imm, Ty: TyU8, Cls: ClassGeneral, SrcLine: x.SrcLine, SrcFile: x.SrcFile},
						&Inst{Op: OpAnd, Dst: test, Src: [2]Reg{low, mask}, Ty: TyU8, Cls: ClassGeneral, SrcLine: x.SrcLine, SrcFile: x.SrcFile},
						&Inst{Op: OpConst, Dst: zero, Imm: 0, Ty: TyU8, Cls: ClassGeneral, SrcLine: x.SrcLine, SrcFile: x.SrcFile})
					x.Src[side], x.Src[1-side] = test, zero
					x.SrcTy = TyU8
					changed = true
					break
				}
			}
			out = append(out, x)
			if x.Dst != NoReg {
				defs[x.Dst] = x
			}
		}
		b.Insts = out
	}
	return changed
}
