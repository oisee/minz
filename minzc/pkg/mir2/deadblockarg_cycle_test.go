package mir2

import "testing"

// The second head parameter feeds only itself or a forwarding latch. It must
// disappear without disturbing the counter/control flow or real return uses.
func blockArgCycleFixture(mode string) *Module {
	f := &Func{Name: "cycle", nextReg: 100}
	initial := int64(7)
	if mode == "branch_value" || mode == "condret_condition" || mode == "threeway_value" {
		initial = 0
	}
	entry := &Block{Label: "entry", Insts: []*Inst{
		{Op: OpConst, Dst: 1, Ty: TyU8, Imm: 3},
		{Op: OpConst, Dst: 2, Ty: TyU8, Imm: initial},
		{Op: OpConst, Dst: 3, Ty: TyU8, Imm: 42},
	}, Term: &TermJmp{Target: "head", Args: []Reg{1, 2}}}
	head := &Block{Label: "head", Params: []BlockParam{{Dst: 4, Ty: TyU8}, {Dst: 5, Ty: TyU8}}, Insts: []*Inst{
		{Op: OpConst, Dst: 6, Ty: TyU8, Imm: 0},
		{Op: OpCmp, Dst: 7, Src: [2]Reg{4, 6}, Ty: TyBool, Cond: CmpNe},
	}, Term: &TermBrIf{Cond: 7, Then: "body", ThenArgs: []Reg{5}, Else: "exit"}}
	body := &Block{Label: "body", Params: []BlockParam{{Dst: 10, Ty: TyU8}}, Insts: []*Inst{
		{Op: OpConst, Dst: 8, Ty: TyU8, Imm: 1},
		{Op: OpSub, Dst: 9, Src: [2]Reg{4, 8}, Ty: TyU8},
	}, Term: &TermJmp{Target: "head", Args: []Reg{9, 10}}}
	exit := &Block{Label: "exit", Term: &TermRet{Vals: []Reg{3}}}
	switch mode {
	case "self":
		head.Term.(*TermBrIf).ThenArgs = nil
		body.Params = nil
		body.Term.(*TermJmp).Args[1] = 5
	case "error":
		f.Attrs.FallibleCY = true
		exit.Term = &TermErrorRet{Code: 5}
	case "return":
		exit.Term = &TermRet{Vals: []Reg{5}}
	case "condret_dead":
		head.Term = &TermCondRet{Cond: 7, Vals: []Reg{3}, Then: "body", ThenArgs: []Reg{5}}
	case "condret":
		head.Term = &TermCondRet{Cond: 7, Vals: []Reg{5}, Then: "body", ThenArgs: []Reg{5}}
	case "condret_condition":
		head.Term = &TermCondRet{Cond: 5, Vals: []Reg{3}, Then: "body", ThenArgs: []Reg{5}}
	case "threeway_value":
		head.Term = &TermBrIf2{Lhs: 5, Rhs: 6, Eq: "exit", Lt: "exit", Gt: "body", GtArgs: []Reg{5}}
	case "threeway":
		head.Term = &TermBrIf2{Lhs: 4, Rhs: 6, Eq: "exit", Lt: "exit", Gt: "body", GtArgs: []Reg{5}}
	case "branch_value":
		head.Term.(*TermBrIf).Cond = 5
	}
	f.Blocks = []*Block{entry, head, body, exit}
	return &Module{Funcs: []*Func{f}}
}

func TestDeadBlockArgElimForwardingCycles(t *testing.T) {
	for _, mode := range []string{"self", "multi", "return", "condret", "threeway", "branch_value", "error", "condret_condition", "threeway_value", "condret_dead"} {
		for _, grace := range []bool{false, true} {
			name := mode + "/ordinary"
			if grace {
				name = mode + "/grace"
			}
			t.Run(name, func(t *testing.T) {
				m := blockArgCycleFixture(mode)
				f := m.Funcs[0]
				verify := func() {
					t.Helper()
					if e := Verify(m); e != nil {
						t.Fatal(e)
					}
				}
				verify()
				want := int64(42)
				live := mode == "return" || mode == "condret" || mode == "branch_value" || mode == "error" || mode == "condret_condition" || mode == "threeway_value"
				if mode == "return" || mode == "condret" {
					want = 7
				}
				execute := func() {
					t.Helper()
					if mode == "error" {
						out, e := NewVM(m).CallWithOutcome("cycle", nil)
						if e != nil || !out.Failed || out.ErrorCode != 7 {
							t.Fatalf("error outcome%+v err%v", out, e)
						}
						return
					}
					out, e := NewVM(m).Call("cycle", nil)
					if e != nil || len(out) != 1 || out[0].I != want {
						t.Fatalf("result%v error%v want%d", out, e, want)
					}
				}
				execute()
				run := func() bool {
					if grace {
						return RunGraceDeadBlockArgElim(f)
					}
					return DeadBlockArgElim(f)
				}
				if changed := run(); changed == live {
					t.Fatalf("changed=%v live-cycle=%v", changed, live)
				}
				verify()
				execute()
				wantHead := 1
				if live {
					wantHead = 2
				}
				if len(f.BlockByLabel("head").Params) != wantHead {
					t.Fatalf("head params%v", f.BlockByLabel("head").Params)
				}
				wantBody := 0
				if live {
					wantBody = 1
				}
				if len(f.BlockByLabel("body").Params) != wantBody {
					t.Fatalf("body params%v", f.BlockByLabel("body").Params)
				}
				if run() {
					t.Fatal("not at fixpoint")
				}
			})
		}
	}
}
