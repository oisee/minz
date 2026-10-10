package mir2

import (
	"github.com/minz/minzc/pkg/rewrite/grace"
	"testing"
)

func joinRetCollisionFunc() *Func {
	f := &Func{Name: "pick", nextReg: 100, Contract: Contract{Params: []Param{{Name: "path", Reg: 1, Ty: TyBool}, {Name: "other", Reg: 2, Ty: TyBool}}, Returns: []Return{{Ty: TyU8}}}}
	f.Blocks = []*Block{
		{Label: "entry", Insts: []*Inst{{Op: OpConst, Dst: 3, Imm: 11, Ty: TyU8}, {Op: OpConst, Dst: 4, Imm: 22, Ty: TyU8}, {Op: OpConst, Dst: 5, Imm: 99, Ty: TyU8}}, Term: &TermBrIf{Cond: 1, Then: "a", Else: "b"}},
		{Label: "a", Term: &TermBrIf{Cond: 2, Then: "other", Else: "join", ElseArgs: []Reg{3}}},
		{Label: "b", Term: &TermBrIf{Cond: 2, Then: "other", Else: "join", ElseArgs: []Reg{4}}},
		{Label: "other", Term: &TermRet{Vals: []Reg{5}}},
		{Label: "join", Params: []BlockParam{{Dst: 6, Ty: TyU8}}, Term: &TermRet{Vals: []Reg{6}}},
		{Label: "join_ret_else", Term: &TermRet{Vals: []Reg{5}}},
	}
	return f
}

func TestSplitJoinRetDistinctConditionalPredecessors(t *testing.T) {
	for _, mode := range []string{"go", "grace_rule", "grace_pipeline"} {
		t.Run(mode, func(t *testing.T) {
			f := joinRetCollisionFunc()
			m := &Module{Funcs: []*Func{f}}
			if err := Verify(m); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "go":
				if !SplitJoinRet(f) {
					t.Fatal("did not split")
				}
			case "grace_rule":
				rules, err := grace.ParseRules(graceSplitJoinRetRule)
				if err != nil {
					t.Fatal(err)
				}
				if grace.ApplyRules(FuncAsMutGraph(f), rules, buildRegistry(f), 1).Applied != 1 {
					t.Fatal("did not split")
				}
			case "grace_pipeline":
				RunGracePasses(f, nil)
			}
			// Check BEFORE cleanup: duplicate labels can otherwise be hidden by a
			// label->block map dropping one of the differing return blocks.
			if err := Verify(m); err != nil {
				t.Fatal(err)
			}
			EliminateDeadBlocks(f)
			if err := Verify(m); err != nil {
				t.Fatal(err)
			}
			for path := int64(0); path < 2; path++ {
				for other := int64(0); other < 2; other++ {
					want := int64(22)
					if path == 1 {
						want = 11
					}
					if other == 1 {
						want = 99
					}
					got, err := NewVM(m).Call("pick", []Value{{I: path}, {I: other}})
					if err != nil || len(got) != 1 || got[0].I != want {
						t.Fatalf("path%d other%d: %v %v want%d", path, other, got, err, want)
					}
				}
			}
		})
	}
}
