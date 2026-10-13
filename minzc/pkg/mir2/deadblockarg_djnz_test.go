package mir2

import "testing"

func TestDeadBlockArgElimDJNZEdges(t *testing.T) {
	for _, grace := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "grace"}[grace], func(t *testing.T) {
			f := &Func{Name: "test", nextReg: 100}
			entry := &Block{Label: "entry", Insts: []*Inst{
				{Op: OpConst, Dst: 1, Imm: 1, Ty: TyU8},
				{Op: OpConst, Dst: 2, Imm: 7, Ty: TyU8},
				{Op: OpConst, Dst: 3, Imm: 9, Ty: TyU8},
			}, Term: &TermJmp{Target: "body", Args: []Reg{1, 2, 3}}}
			// Counter parameter is implicit in the back edge, even though this body
			// deliberately uses the dominating constant as its DJNZ operand.
			body := &Block{Label: "body", Params: []BlockParam{{Dst: 10, Ty: TyU8}, {Dst: 11, Ty: TyU8}, {Dst: 12, Ty: TyU8}}, Term: &TermDJNZ{Counter: 1, Body: "body", BodyArgs: []Reg{2, 12}, Exit: "exit", ExitArgs: []Reg{2, 12}}}
			exit := &Block{Label: "exit", Params: []BlockParam{{Dst: 20, Ty: TyU8}, {Dst: 21, Ty: TyU8}}, Term: &TermRet{Vals: []Reg{21}}}
			f.Blocks = []*Block{entry, body, exit}
			m := &Module{Funcs: []*Func{f}}
			if err := Verify(m); err != nil {
				t.Fatal(err)
			}
			if grace {
				RunGracePasses(f, NewGraceStats())
			} else {
				DeadBlockArgElim(f)
			}
			if err := Verify(m); err != nil {
				t.Fatal(err)
			}
			if len(body.Params) != 2 || body.Params[0].Dst != 10 || body.Params[1].Dst != 12 {
				t.Fatalf("body params: %v", body.Params)
			}
			term := body.Term.(*TermDJNZ)
			if len(term.BodyArgs) != 1 || term.BodyArgs[0] != 12 || len(term.ExitArgs) != 1 || term.ExitArgs[0] != 12 {
				t.Fatalf("edges: %+v", term)
			}
			got, err := NewVM(m).Call("test", nil)
			if err != nil || len(got) != 1 || got[0].I != 9 {
				t.Fatalf("got=%v err=%v", got, err)
			}
		})
	}
}
