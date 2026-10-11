package mir2

import "testing"

func TestVMConditionalCallTakenAndInverted(t *testing.T) {
	for _, predicate := range []CmpCond{CmpEq, CmpNe, CmpLt, CmpLe, CmpGt, CmpGe} {
		for _, invert := range []bool{false, true} {
			for _, value := range []int64{0, 1, 127, 128, 255} {
				m := &Module{Name: "conditional effects"}
				f := m.AddFunc("entry")
				b := NewBuilder(f)
				b.SwitchToNewBlock("entry")
				x := b.Param("x", TyU8, ClassAcc)
				y := b.Const(127, TyU8, ClassGeneral)
				cond := b.Cmp(predicate, x, y, ClassFlag, false)
				cc := predicate
				if invert {
					cc = invertCmpCond(cc)
				}
				b.Cur.Insts = append(b.Cur.Insts, &Inst{Op: OpCallCond, Src: [2]Reg{cond, NoReg}, Sym: "@observer", Ty: TyVoid, Cond: cc})
				b.Ret(x)
				f.Contract.Returns = []Return{{Ty: TyU8}}
				calls := 0
				vm := NewVM(m)
				vm.Hosts["@observer"] = func([]Value) ([]Value, error) { calls++; return nil, nil }
				v, e := vm.Call("entry", []Value{{I: value}})
				if e != nil {
					t.Fatal(e)
				}
				want := false
				switch predicate {
				case CmpEq:
					want = value == 127
				case CmpNe:
					want = value != 127
				case CmpLt:
					want = value < 127
				case CmpLe:
					want = value <= 127
				case CmpGt:
					want = value > 127
				case CmpGe:
					want = value >= 127
				}
				if invert {
					want = !want
				}
				n := 0
				if want {
					n = 1
				}
				if calls != n || len(v) != 1 || v[0].I != value {
					t.Fatal(predicate, invert, value, calls, v)
				}
			}
		}
	}
}
func TestVMConditionalCallRejectsUnrelatedPredicate(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("entry")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Const(1, TyU8, ClassGeneral)
	y := b.Const(2, TyU8, ClassGeneral)
	c := b.Cmp(CmpEq, x, y, ClassFlag, false)
	b.Cur.Insts = append(b.Cur.Insts, &Inst{Op: OpCallCond, Src: [2]Reg{c, NoReg}, Sym: "@observer", Ty: TyVoid, Cond: CmpLt})
	b.Ret()
	vm := NewVM(m)
	calls := 0
	vm.Hosts["@observer"] = func([]Value) ([]Value, error) { calls++; return nil, nil }
	if _, e := vm.Call("entry", nil); e == nil || calls != 0 {
		t.Fatal("ambiguous predicate executed", e, calls)
	}
}
