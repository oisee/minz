package mir2

import "testing"

func TestGraceDJNZExitRedirectKeepsTarget(t *testing.T) {
	for _, param := range []bool{false, true} {
		f := &Func{Name: "djnz_redirect", nextReg: 20}
		entry := &Block{Label: "entry", Insts: []*Inst{{Op: OpConst, Dst: 1, Ty: TyU8, Imm: 3}}, Term: &TermJmp{Target: "body", Args: []Reg{1}}}
		body := &Block{Label: "body", Params: []BlockParam{{Dst: 2, Ty: TyU8, Class: ClassCounter}}, Term: &TermDJNZ{Counter: 2, Body: "body", Exit: "forward"}}
		forward := &Block{Label: "forward", Term: &TermJmp{Target: "exit"}}
		exit := &Block{Label: "exit", Term: &TermRet{}}
		if param {
			body.Term.(*TermDJNZ).ExitArgs = []Reg{2}
			forward.Params = []BlockParam{{Dst: 3, Ty: TyU8}}
			forward.Term = &TermJmp{Target: "exit", Args: []Reg{3}}
			exit.Params = []BlockParam{{Dst: 4, Ty: TyU8}}
			exit.Term = &TermRet{Vals: []Reg{4}}
			f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
		}
		f.Blocks = []*Block{entry, body, forward, exit}
		module := &Module{Funcs: []*Func{f}}
		before, err := NewVM(module).Call(f.Name, nil)
		if err != nil {
			t.Fatal(err)
		}
		RunGracePasses(f, NewGraceStats())
		if err := Verify(module); err != nil {
			t.Fatal("redirect deleted live DJNZ target", param, err)
		}
		after, err := NewVM(module).Call(f.Name, nil)
		if err != nil || len(after) != len(before) {
			t.Fatal("redirect result", param, before, after, err)
		}
		if param && (before[0].I != 1 || after[0].I != 1) {
			t.Fatal("DJNZ explicit exit operand changed", before, after)
		}
	}
}
