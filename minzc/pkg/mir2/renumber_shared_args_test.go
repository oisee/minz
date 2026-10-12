package mir2

import "testing"

func TestRenumberRegsSharedAndOverlappingEdgeArguments(t *testing.T) {
	for _, overlap := range []bool{false, true} {
		m := &Module{}
		f := m.AddFunc("shared_edges")
		// Reverse original IDs make repeat-remapping observable: old2→1, old1→2.
		a, b := Reg(2), Reg(1)
		shared := []Reg{a, b}
		one, two := shared, shared
		if overlap {
			one = shared[:1]
			two = shared[1:]
		}
		f.Contract.Params = []Param{{Reg: a, Ty: TyU8}, {Reg: b, Ty: TyU8}}
		f.Blocks = []*Block{{Label: "entry", Term: &TermBrIf{Cond: a, Then: "left", Else: "right", ThenArgs: one, ElseArgs: two}}, {Label: "left", Term: &TermRet{Vals: shared}}, {Label: "right", Term: &TermRet{Vals: shared}}}
		m.RenumberRegs()
		br := f.Blocks[0].Term.(*TermBrIf)
		expectedOne, expectedTwo := []Reg{1, 2}, []Reg{1, 2}
		if overlap {
			expectedOne = []Reg{1}
			expectedTwo = []Reg{2}
		}
		for i, r := range br.ThenArgs {
			if r != expectedOne[i] {
				t.Fatalf("overlap%v then%d got%d want%d", overlap, i, r, expectedOne[i])
			}
		}
		for i, r := range br.ElseArgs {
			if r != expectedTwo[i] {
				t.Fatalf("overlap%v else%d got%d want%d", overlap, i, r, expectedTwo[i])
			}
		}
		for _, block := range f.Blocks[1:] {
			r := block.Term.(*TermRet)
			if r.Vals[0] != 1 || r.Vals[1] != 2 {
				t.Fatalf("overlap%v ret%v", overlap, r.Vals)
			}
		}
	}
}

func TestRenumberRegsSharedArgumentsAcrossFunctions(t *testing.T) {
	m := &Module{}
	shared := []Reg{2, 1}
	for _, name := range []string{"first", "second"} {
		f := m.AddFunc(name)
		f.Contract.Params = []Param{{Reg: 2, Ty: TyU8}, {Reg: 1, Ty: TyU8}}
		f.Blocks = []*Block{{Label: "entry", Term: &TermRet{Vals: shared}}}
	}
	m.RenumberRegs()
	for i, f := range m.Funcs {
		r := f.Blocks[0].Term.(*TermRet)
		want := Reg(1 + i*2)
		if r.Vals[0] != want || r.Vals[1] != want+1 {
			t.Fatalf("function%d vals%v want%d,%d", i, r.Vals, want, want+1)
		}
	}
	if shared[0] != 2 || shared[1] != 1 {
		t.Fatal("external slice mutated", shared)
	}
}
