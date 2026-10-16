package mir2

import "testing"

func lowMaskFixture(ty Ty, mask int64, cond CmpCond, reverse bool, zeroTy Ty) (*Module, *Func, *Inst, *Inst) {
	m := &Module{}
	f := m.AddFunc("test")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Param("word", ty, ClassPair)
	k := b.Const(mask, ty, ClassPair)
	r := b.And(x, k, ty, ClassPair)
	z := b.Const(0, zeroTy, ClassGeneral)
	left, right := r, z
	if reverse {
		left, right = right, left
	}
	c := b.CmpWithSrcTy(cond, left, right, ClassAcc, false, ty)
	b.Ret(c, r)
	f.Contract.Returns = []Return{{Ty: TyBool, Class: ClassAcc}, {Ty: ty, Class: ClassPair}}
	return m, f, f.Blocks[0].Insts[3], f.Blocks[0].Insts[1]
}

func TestLowByteMaskAllWordsSharedConsumer(t *testing.T) {
	for _, ty := range []Ty{TyU16, TyI16} {
		for _, mask := range []int64{1, 90, 128, 255} {
			for _, cond := range []CmpCond{CmpEq, CmpNe} {
				for _, reverse := range []bool{false, true} {
					zty := TyU16
					if reverse {
						zty = TyU8
					}
					raw, _, _, _ := lowMaskFixture(ty, mask, cond, reverse, zty)
					opt, f, cmp, original := lowMaskFixture(ty, mask, cond, reverse, zty)
					if !NarrowLowByteMaskTests(f) || cmp.SrcTy != TyU8 || original.Ty != ty {
						t.Fatal("not narrowed or other consumer changed")
					}
					if err := Verify(opt); err != nil {
						t.Fatal(err)
					}
					vms := []*VM{NewVM(raw), NewVM(opt)}
					for word := 0; word < 65536; word++ {
						masked := int64(word) & mask
						want := int64(0)
						if (masked == 0) == (cond == CmpEq) {
							want = 1
						}
						for _, vm := range vms {
							got, err := vm.Call("test", []Value{{I: int64(word)}})
							if err != nil || len(got) != 2 || got[0].I != want || got[1].I != masked {
								t.Fatal(ty, mask, cond, reverse, word, got, err)
							}
						}
					}
				}
			}
		}
	}
}

func TestLowByteMaskRefusals(t *testing.T) {
	for _, mode := range []string{"high-mask", "negative-mask", "zero-mask", "ordering", "nonzero-rhs", "wide-source", "hard-mask", "hard-compare", "hidden-mask", "hidden-constant", "hidden-compare", "flags-only", "flag-return", "asm", "fallible", "redefined-source", "redefined-mask", "late-mask"} {
		t.Run(mode, func(t *testing.T) {
			m, f, cmp, a := lowMaskFixture(TyU16, 1, CmpNe, false, TyU16)
			b := f.Blocks[0]
			switch mode {
			case "high-mask":
				b.Insts[0].Imm = 256
			case "negative-mask":
				b.Insts[0].Imm = -1
			case "zero-mask":
				b.Insts[0].Imm = 0
			case "ordering":
				cmp.Cond = CmpUlt
			case "nonzero-rhs":
				b.Insts[2].Imm = 1
			case "wide-source":
				f.Contract.Params[0].Ty = TyU32
			case "hard-mask":
				a.ClsHard = true
			case "hard-compare":
				cmp.ClsHard = true
			case "hidden-mask":
				a.Sym = "unknown"
			case "hidden-constant":
				b.Insts[0].Sym = "unknown"
			case "hidden-compare":
				cmp.Sym = "unknown"
			case "flags-only":
				cmp.FlagsOnly = true
			case "flag-return":
				f.Contract.Returns[0].Class = ClassFlag
			case "asm":
				b.Insts = append(b.Insts, &Inst{Op: OpAsm})
			case "fallible":
				f.Attrs.FallibleCY = true
			case "redefined-source":
				b.Insts = append(b.Insts, &Inst{Op: OpConst, Dst: f.Contract.Params[0].Reg, Imm: 123, Ty: TyU16})
			case "redefined-mask":
				b.Insts = append(b.Insts, &Inst{Op: OpConst, Dst: a.Dst, Imm: 123, Ty: TyU16})
			case "late-mask":
				b.Insts = []*Inst{b.Insts[0], b.Insts[2], cmp, a}
			}
			before := m.Dump()
			if NarrowLowByteMaskTests(f) || m.Dump() != before {
				t.Fatal("refusal modified input")
			}
		})
	}
}

func TestLowByteMaskKeepsExistingMemoryBitPattern(t *testing.T) {
	for _, volatile := range []bool{false, true} {
		m := &Module{}
		f := m.AddFunc("read_bit")
		b := NewBuilder(f)
		b.SwitchToNewBlock("entry")
		p := b.Param("pointer", TyPtr, ClassPointer)
		value := b.Load(p, TyU16, ClassPair)
		f.Blocks[0].Insts[0].Volatile = volatile
		k := b.Const(1, TyU16, ClassPair)
		r := b.And(value, k, TyU16, ClassPair)
		z := b.Const(0, TyU16, ClassPair)
		c := b.CmpWithSrcTy(CmpNe, r, z, ClassFlag, false, TyU16)
		b.Ret(c)
		before := m.Dump()
		got := NarrowLowByteMaskTests(f)
		if got != volatile {
			t.Fatal("ordinary memory BIT must keep original form", volatile, got)
		}
		if !volatile && m.Dump() != before {
			t.Fatal("refusal modified memory BIT")
		}
	}
}
