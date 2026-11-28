package mir2

import "testing"

// FuseAbsDiff must leave every compare's value unchanged: each unsigned
// condition, both operand orders, sub before or after the compare, u8 on all
// byte pairs and u16 on boundary values. Only the four rows that are the
// borrow of the sub (or its complement) may be rewritten.
func TestFuseAbsDiffTruthTable(t *testing.T) {
	type row struct {
		cond    CmpCond
		swapped bool
	}
	fused := map[row]CmpCond{
		{CmpUlt, false}: CmpSubCarry, {CmpUgt, true}: CmpSubCarry,
		{CmpUge, false}: CmpSubCarryNot, {CmpUle, true}: CmpSubCarryNot,
	}
	build := func(ty Ty, cond CmpCond, swapped, cmpFirst bool) (*Module, *Func) {
		m := &Module{}
		f := m.AddFunc("f")
		f.Contract.Params = []Param{{Reg: 1, Ty: ty, Class: ClassGeneral}, {Reg: 2, Ty: ty, Class: ClassGeneral}}
		f.Contract.Returns = []Return{{Ty: ty, Class: ClassGeneral}, {Ty: TyBool, Class: ClassGeneral}}
		src := [2]Reg{1, 2}
		if swapped {
			src = [2]Reg{2, 1}
		}
		sub := &Inst{Op: OpSub, Dst: 3, Src: [2]Reg{1, 2}, Ty: ty, Cls: ClassGeneral}
		cmp := &Inst{Op: OpCmp, Dst: 4, Src: src, Ty: TyBool, SrcTy: ty, Cond: cond, Cls: ClassFlag}
		insts := []*Inst{sub, cmp}
		if cmpFirst {
			insts = []*Inst{cmp, sub}
		}
		f.Blocks = []*Block{{Label: "entry", Insts: insts, Term: &TermRet{Vals: []Reg{3, 4}}}}
		return m, f
	}
	var words []int64
	for _, v := range []int64{0, 1, 2, 0x7f, 0x80, 0xff, 0x100, 0x7fff, 0x8000, 0xfffe, 0xffff} {
		words = append(words, v)
	}
	for _, ty := range []Ty{TyU8, TyU16} {
		var vals []int64
		if ty == TyU8 {
			for v := int64(0); v < 256; v++ {
				vals = append(vals, v)
			}
		} else {
			vals = words
		}
		for _, cond := range []CmpCond{CmpUlt, CmpUle, CmpUgt, CmpUge, CmpEq, CmpNe, CmpLt, CmpGe} {
			for _, swapped := range []bool{false, true} {
				for _, cmpFirst := range []bool{false, true} {
					before, _ := build(ty, cond, swapped, cmpFirst)
					m, f := build(ty, cond, swapped, cmpFirst)
					changed := FuseAbsDiff(f)
					want, ok := fused[row{cond, swapped}]
					cmp := f.Blocks[0].Insts[1]
					if changed != ok || ok && (cmp.Cond != want || cmp.Src != [2]Reg{3, 2} || f.Blocks[0].Insts[0].Op != OpSub) {
						t.Fatalf("%v cond=%v swapped=%v cmpFirst=%v: changed=%v\n%s", ty, cond, swapped, cmpFirst, changed, m.Dump())
					}
					if err := Verify(m); err != nil {
						t.Fatal(err)
					}
					v0, v1 := NewVM(before), NewVM(m)
					for _, x := range vals {
						for _, y := range vals {
							a, err0 := v0.Call("f", []Value{{I: x}, {I: y}})
							b, err1 := v1.Call("f", []Value{{I: x}, {I: y}})
							if err0 != nil || err1 != nil || len(a) != 2 || len(b) != 2 || a[0].I != b[0].I || a[1].I != b[1].I {
								t.Fatalf("%v cond=%v swapped=%v cmpFirst=%v x=%d y=%d: %v %v %v %v", ty, cond, swapped, cmpFirst, x, y, a, b, err0, err1)
							}
						}
					}
				}
			}
		}
	}
}
