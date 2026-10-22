package mir2

import "testing"

func TestDeadBlockArgElimForwardedExitFixpoint(t *testing.T) {
	f := &Func{Name: "forwarded", Blocks: []*Block{
		{Label: "entry", Insts: []*Inst{{Op: OpConst, Dst: 1, Ty: TyU8, Imm: 42}}, Term: &TermJmp{Target: "head", Args: []Reg{1}}},
		{Label: "head", Params: []BlockParam{{Dst: 2, Ty: TyU8}}, Term: &TermJmp{Target: "exit", Args: []Reg{2}}},
		{Label: "exit", Params: []BlockParam{{Dst: 3, Ty: TyU8}}, Term: &TermRet{Vals: []Reg{1}}},
	}}
	m := &Module{Funcs: []*Func{f}}
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
	if !DeadBlockArgElim(f) {
		t.Fatal("no change")
	}
	for _, b := range f.Blocks[1:] {
		if len(b.Params) != 0 {
			t.Fatalf("%s retains forwarded dead parameter: %v", b.Label, b.Params)
		}
	}
	if DeadBlockArgElim(f) {
		t.Fatal("not at fixpoint")
	}
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
	got, err := NewVM(m).Call(f.Name, nil)
	if err != nil || len(got) != 1 || got[0].I != 42 {
		t.Fatal(got, err)
	}
}
