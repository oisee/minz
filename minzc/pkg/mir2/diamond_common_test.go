package mir2

import "testing"

func diamondCommonFixture() (*Module, *Func) {
	m := &Module{}
	f := m.AddFunc("renamed_step")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Param("x", TyU8, ClassGeneral)
	mask := b.Const(128, TyU8, ClassGeneral)
	test := b.And(x, mask, TyU8, ClassGeneral)
	zero := b.Const(0, TyU8, ClassGeneral)
	cond := b.Cmp(CmpNe, test, zero, ClassFlag, false)
	b.BrIf(cond, "yes", nil, "no", nil)
	b.SwitchToNewBlock("yes")
	one := b.Const(1, TyU8, ClassGeneral)
	a := b.Shl(x, one, TyU8, ClassGeneral)
	b.Ret(a)
	b.SwitchToNewBlock("no")
	otherOne := b.Const(1, TyU8, ClassGeneral)
	c := b.Shl(x, otherOne, TyU8, ClassGeneral)
	b.Ret(c)
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	return m, f
}

func TestHoistDiamondCommon(t *testing.T) {
	m, f := diamondCommonFixture()
	if !HoistDiamondCommon(f) {
		t.Fatal("did not common shift")
	}
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
	if len(f.Entry().Insts) != 6 || f.Entry().Insts[4].Op != OpShl || len(f.Blocks[1].Insts) != 0 || len(f.Blocks[2].Insts) != 0 {
		t.Fatal(m.Dump())
	}
	for x := 0; x < 256; x++ {
		got, e := NewVM(m).Call(f.Name, []Value{{I: int64(x)}})
		if e != nil || got[0].I != int64(byte(x<<1)) {
			t.Fatal(x, got, e)
		}
	}
	if HoistDiamondCommon(f) {
		t.Fatal("not idempotent")
	}
}

func TestHoistDiamondCommonRefusals(t *testing.T) {
	for _, kind := range []string{"different-op", "different-count", "different-type", "different-source", "hard", "volatile", "load", "call", "division", "shared-successor", "block-param", "non-dominating", "duplicate-source", "duplicate-result", "effect-before", "implicit-carry"} {
		t.Run(kind, func(t *testing.T) {
			m, f := diamondCommonFixture()
			a, b := f.Blocks[1], f.Blocks[2]
			switch kind {
			case "implicit-carry":
				f.Entry().Insts[len(f.Entry().Insts)-1].Cond = CmpSubCarry
			case "different-op":
				b.Insts[1].Op = OpShr
			case "different-count":
				b.Insts[0].Imm = 2
			case "different-type":
				b.Insts[1].Ty = TyU16
			case "different-source":
				b.Insts[1].Src[0] = b.Insts[0].Dst
			case "hard":
				a.Insts[1].ClsHard = true
			case "volatile":
				a.Insts[1].Volatile = true
			case "load":
				a.Insts[1].Op = OpLoad
				b.Insts[1].Op = OpLoad
			case "call":
				a.Insts[1].Op = OpCall
				b.Insts[1].Op = OpCall
			case "division":
				a.Insts[1].Op = OpDiv
				b.Insts[1].Op = OpDiv
			case "shared-successor":
				f.NewBlock("extra").Term = &TermJmp{Target: a.Label}
			case "block-param":
				a.Params = []BlockParam{{Dst: f.AllocReg(), Ty: TyU8}}
			case "non-dominating":
				a.Insts[1].Src[0] = b.Insts[0].Dst
				b.Insts[1].Src[0] = b.Insts[0].Dst
			case "duplicate-source":
				a.Insts[0].Dst = f.Contract.Params[0].Reg
			case "duplicate-result":
				b.Insts[1].Dst = a.Insts[1].Dst
			case "effect-before":
				a.Insts = append([]*Inst{{Op: OpStore, Src: [2]Reg{1, 1}, Ty: TyU8}}, a.Insts...)
			}
			before := m.Dump()
			if HoistDiamondCommon(f) || m.Dump() != before {
				t.Fatal("unsafe or partial rewrite", kind)
			}
		})
	}
}
