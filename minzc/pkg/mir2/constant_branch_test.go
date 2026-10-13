package mir2

import "testing"

func TestFoldConstantBranchPreservesEdgeValues(t *testing.T) {
	for _, condRet := range []bool{false, true} {
		for _, flag := range []int64{0, 1} {
			m := &Module{}
			f := m.AddFunc("branch")
			b := NewBuilder(f)
			entry := b.SwitchToNewBlock("entry")
			x := b.Const(flag, TyU8, ClassGeneral)
			zero := b.Const(0, TyU8, ClassGeneral)
			// Condition becomes constant through folding, not a literal bool.
			cond := b.Cmp(CmpNe, x, zero, ClassFlag, false)
			chosen := b.Const(17, TyU8, ClassGeneral)
			other := b.Const(29, TyU8, ClassGeneral)
			if condRet {
				entry.Term = &TermCondRet{Cond: cond, Vals: []Reg{other}, Then: "then", ThenArgs: []Reg{chosen}}
			} else {
				b.BrIf(cond, "then", []Reg{chosen}, "else", []Reg{other})
			}
			then := b.SwitchToNewBlock("then")
			p := b.BlockParam(then, TyU8, ClassGeneral)
			b.Ret(p)
			if !condRet {
				otherwise := b.SwitchToNewBlock("else")
				q := b.BlockParam(otherwise, TyU8, ClassGeneral)
				b.Ret(q)
			}
			check := func() {
				t.Helper()
				if err := Verify(m); err != nil {
					t.Fatal(err)
				}
				got, err := NewVM(m).Call(f.Name, nil)
				want := int64(29)
				if flag != 0 {
					want = 17
				}
				if err != nil || len(got) != 1 || got[0].I != want {
					t.Fatalf("condRet=%v flag=%d got %v %v want %d", condRet, flag, got, err, want)
				}
			}
			check()
			if !FoldConstants(f) {
				t.Fatal("expected fold")
			}
			DeadStoreElim(f)
			check()
			if _, ok := entry.Term.(*TermBrIf); ok {
				t.Fatal("constant branch retained")
			}
			if _, ok := entry.Term.(*TermCondRet); ok {
				t.Fatal("constant cond_ret retained")
			}
			if FoldConstants(f) {
				t.Fatal("not idempotent")
			}
		}
	}
}
func TestFoldConstantBranchKeepsUnknownCondition(t *testing.T) {
	f := &Func{Name: "unknown", Contract: Contract{Params: []Param{{Reg: 1, Ty: TyBool}}}, Blocks: []*Block{{Label: "entry", Term: &TermBrIf{Cond: 1, Then: "yes", Else: "no"}}, {Label: "yes", Term: &TermRet{}}, {Label: "no", Term: &TermRet{}}}}
	if FoldConstants(f) {
		t.Fatal("unknown condition folded")
	}
}
