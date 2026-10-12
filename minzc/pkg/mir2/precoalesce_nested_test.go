package mir2

import "testing"

// Outer y is passed by value to a mutable inner sy, then used after sy reaches
// zero. Edge coalescing must not turn y into sy. Expected result is independent
// of the inner countdown's final value.
func TestPreallocCoalescePreservesLiveOuterLoopValue(t *testing.T) {
	for _, n := range []int64{1, 4, 19, 255} {
		m := &Module{}
		f := m.AddFunc("nested")
		f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
		entry := f.NewBlock("entry")
		header := f.NewBlock("inner")
		body := f.NewBlock("body")
		exit := f.NewBlock("exit")
		entry.Insts = []*Inst{{Op: OpConst, Dst: 1, Ty: TyU8, Imm: n}}
		entry.Term = &TermJmp{Target: "inner", Args: []Reg{1}}
		header.Params = []BlockParam{{Dst: 2, Ty: TyU8, Class: ClassGeneral}}
		header.Insts = []*Inst{{Op: OpConst, Dst: 3, Ty: TyU8, Imm: 0}, {Op: OpCmp, Dst: 4, Ty: TyBool, SrcTy: TyU8, Src: [2]Reg{2, 3}, Cond: CmpUgt}}
		header.Term = &TermBrIf{Cond: 4, Then: "body", Else: "exit"}
		body.Insts = []*Inst{{Op: OpConst, Dst: 5, Ty: TyU8, Imm: 1}, {Op: OpSub, Dst: 6, Ty: TyU8, Src: [2]Reg{2, 5}}}
		body.Term = &TermJmp{Target: "inner", Args: []Reg{6}}
		exit.Term = &TermRet{Vals: []Reg{1}}
		verify := func() {
			got, e := NewVM(m).Call("nested", nil)
			if e != nil || got[0].I != n {
				t.Fatalf("n%d got%v err%v", n, got, e)
			}
		}
		verify()
		PreallocCoalesce(f)
		verify()
	}
}

// A chain must never absorb conflicting members, regardless of which earlier
// legal unions selected its representative. Check the entire returned partition.
func TestPreallocCoalesceChecksTransitiveMembers(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("transitive")
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	e := f.NewBlock("entry")
	one := f.NewBlock("one")
	two := f.NewBlock("two")
	e.Insts = []*Inst{{Op: OpConst, Dst: 1, Ty: TyU8, Imm: 37}}
	e.Term = &TermJmp{Target: "one", Args: []Reg{1}}
	one.Params = []BlockParam{{Dst: 2, Ty: TyU8, Class: ClassGeneral}}
	one.Term = &TermJmp{Target: "two", Args: []Reg{2}}
	two.Params = []BlockParam{{Dst: 3, Ty: TyU8, Class: ClassGeneral}}
	two.Insts = []*Inst{{Op: OpConst, Dst: 4, Ty: TyU8, Imm: 1}, {Op: OpAdd, Dst: 5, Ty: TyU8, Src: [2]Reg{1, 3}}}
	two.Term = &TermRet{Vals: []Reg{5}}
	ig := BuildInterferenceGraph(f, ComputeLiveness(f))
	if !ig.Neighbors(1).Has(3) {
		t.Fatal("fixture requires member conflict")
	}
	remap := PreallocCoalesce(f)
	for x, rx := range remap {
		for y, ry := range remap {
			if rx == ry && ig.Neighbors(x).Has(y) {
				t.Fatal("conflicting original members merged", x, y, remap)
			}
		}
	}
	got, err := NewVM(m).Call("transitive", nil)
	if err != nil || got[0].I != 74 {
		t.Fatal(got, err)
	}
}
