package mir2

import (
	"github.com/minz/minzc/pkg/rewrite/grace"
	"testing"
)

func TestGraceTrivialBranchRequiresEquivalentEdgeValues(t *testing.T) {
	rules, e := grace.ParseRules(graceTrivialBranchRule)
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name      string
		then, els []Reg
		fold      bool
	}{{"different-forward", []Reg{2}, []Reg{3}, false}, {"different-reverse", []Reg{3}, []Reg{2}, false}, {"different-order", []Reg{2, 3}, []Reg{3, 2}, false}, {"different-length", []Reg{2}, []Reg{2, 3}, false}, {"equal", []Reg{2, 3}, []Reg{2, 3}, true}, {"empty-nil", nil, []Reg{}, true}} {
		t.Run(tc.name, func(t *testing.T) {
			f := &Func{Name: "edge_args", Blocks: []*Block{{Label: "entry", Term: &TermBrIf{Cond: 1, Then: "join", Else: "join", ThenArgs: tc.then, ElseArgs: tc.els}}, {Label: "join", Term: &TermRet{}}}}
			r := grace.ApplyRules(FuncAsMutGraph(f), rules, buildRegistry(f), 5)
			if (r.Applied > 0) != tc.fold {
				t.Fatalf("fold%d want%v", r.Applied, tc.fold)
			}
			_, jump := f.Entry().Term.(*TermJmp)
			if jump != tc.fold {
				t.Fatal("branch retained/folded incorrectly")
			}
		})
	}
}
