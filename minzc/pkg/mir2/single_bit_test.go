package mir2

import "testing"

func singleBitFixture(ty Ty, mask, rhs int64, cond CmpCond, reverse bool) (*Module, *Func, *Inst) {
	m := &Module{}
	f := m.AddFunc("probe")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Param("x", ty, ClassGeneral)
	k := b.Const(mask, ty, ClassGeneral)
	a := b.And(x, k, ty, ClassGeneral)
	r := b.Const(rhs, ty, ClassGeneral)
	l := a
	if reverse {
		l, r = r, l
	}
	c := b.CmpWithSrcTy(cond, l, r, ClassAcc, false, ty)
	cmp := f.Entry().Insts[len(f.Entry().Insts)-1]
	b.Ret(c, a)
	f.Contract.Returns = []Return{{Ty: TyBool, Class: ClassAcc}, {Ty: ty, Class: ClassGeneral}}
	return m, f, cmp
}

func TestSingleBitCanonicalAllWidths(t *testing.T) {
	for _, ty := range []Ty{TyU8, TyI8, TyU16, TyI16} {
		for bit := 0; bit < ty.Width(); bit++ {
			mask := int64(1 << bit)
			if (ty == TyI8 || ty == TyI16) && bit == ty.Width()-1 {
				mask = -mask
			}
			for _, cond := range []CmpCond{CmpEq, CmpNe} {
				for _, reverse := range []bool{false, true} {
					m, f, cmp := singleBitFixture(ty, mask, mask, cond, reverse)
					if !CanonicalizeSingleBitTests(f) || cmp.Cond == cond {
						t.Fatal("not canonicalized", ty, mask, cond, reverse)
					}
					if err := Verify(m); err != nil {
						t.Fatal(err)
					}
					if CanonicalizeSingleBitTests(f) {
						t.Fatal("not idempotent")
					}
					vm := NewVM(m)
					for v := 0; v < 256; v++ {
						x := v
						if ty.Width() == 16 {
							x = ((v*73+37)&255)<<8 | v
						}
						masked := int64(x) & maskToWidth(mask, ty)
						want := int64(0)
						if (masked != 0) == (cond == CmpEq) {
							want = 1
						}
						got, err := vm.Call("probe", []Value{{I: int64(x)}})
						if err != nil || len(got) != 2 || got[0].I != want || got[1].I != masked {
							t.Fatal(ty, mask, cond, reverse, x, got, err, want, masked)
						}
					}
				}
			}
		}
	}
}

func TestSingleBitCanonicalRefusals(t *testing.T) {
	for _, kind := range []string{"zero-mask", "multiple-bits", "other-rhs", "ordering", "hard-compare", "hard-and", "hidden-constant", "hidden-and", "hidden-compare", "redefined-and", "asm", "flag-return", "fallible", "different-width"} {
		t.Run(kind, func(t *testing.T) {
			m, f, c := singleBitFixture(TyU8, 1, 1, CmpEq, false)
			b := f.Entry()
			switch kind {
			case "zero-mask":
				b.Insts[0].Imm = 256
			case "multiple-bits":
				b.Insts[0].Imm = 3
			case "other-rhs":
				b.Insts[2].Imm = 2
			case "ordering":
				c.Cond = CmpUlt
			case "hard-compare":
				c.ClsHard = true
			case "hard-and":
				b.Insts[1].ClsHard = true
			case "hidden-constant":
				b.Insts[0].Sym = "unknown"
			case "hidden-and":
				b.Insts[1].Sym = "unknown"
			case "hidden-compare":
				c.Sym = "unknown"
			case "redefined-and":
				b.Insts = append(b.Insts, &Inst{Op: OpConst, Dst: b.Insts[1].Dst, Imm: 7, Ty: TyU8})
			case "asm":
				b.Insts = append(b.Insts, &Inst{Op: OpAsm})
			case "flag-return":
				f.Contract.Returns[0].Class = ClassFlag
			case "fallible":
				f.Attrs.FallibleCY = true
			case "different-width":
				c.SrcTy = TyU16
			}
			before := m.Dump()
			if CanonicalizeSingleBitTests(f) || m.Dump() != before {
				t.Fatal("refusal changed IR")
			}
		})
	}
}
