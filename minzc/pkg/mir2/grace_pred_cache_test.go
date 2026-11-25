package mir2

import (
	"testing"

	"github.com/minz/minzc/pkg/rewrite"
	"github.com/minz/minzc/pkg/rewrite/grace"
)

// The per-block Grace predicates share countRegUses / liveBlockArguments
// results keyed on the registry generation. These cases mutate f between two
// predicate queries; a cache that survived the mutation would miss the
// second rewrite.

// DSE chain: removing r3 makes r2 dead, removing r2 makes r1 dead. Each step
// needs use counts recomputed after the previous elimDeadInsts action.
func TestGracePredicateUseCountsFollowMutation(t *testing.T) {
	f := &Func{Name: "chain", Blocks: []*Block{{Label: "entry", Insts: []*Inst{
		{Op: OpConst, Dst: 1, Ty: TyU8, Imm: 7},
		{Op: OpAdd, Dst: 2, Src: [2]Reg{1, 1}, Ty: TyU8},
		{Op: OpAdd, Dst: 3, Src: [2]Reg{2, 2}, Ty: TyU8},
	}, Term: &TermRet{}}}}
	rules, err := grace.ParseRules(graceDSERule)
	if err != nil {
		t.Fatal(err)
	}
	res := grace.ApplyRules(FuncAsMutGraph(f), rules, buildRegistry(f), 10)
	if res.Applied != 3 || len(f.Blocks[0].Insts) != 0 {
		t.Fatalf("applied %d, %d insts left; want 3 rewrites and an empty block", res.Applied, len(f.Blocks[0].Insts))
	}
}

// has-dead-params is evaluated (param live) in round 1, then a lower-priority
// test action removes the only use. Round 2 must see the parameter dead.
func TestGracePredicateLiveBlockArgsFollowMutation(t *testing.T) {
	f := &Func{Name: "args", Blocks: []*Block{
		{Label: "entry", Insts: []*Inst{{Op: OpConst, Dst: 1, Ty: TyU8, Imm: 7}}, Term: &TermJmp{Target: "b", Args: []Reg{1}}},
		{Label: "b", Params: []BlockParam{{Dst: 2, Ty: TyU8}}, Term: &TermRet{Vals: []Reg{2}}},
	}}
	src := graceDeadBlockArgRule + `
(grace test-drop-ret 1
  (match (block ?b (term-kind "ret")))
  (where (test-ret-has-vals ?b))
  (action (custom "testDropRet" ?b)))
`
	rules, err := grace.ParseRules(src)
	if err != nil {
		t.Fatal(err)
	}
	reg := buildRegistry(f)
	reg.RegisterPredicate("test-ret-has-vals", func(_ rewrite.IRGraph, b grace.BlockBindings, args []string) bool {
		ret, ok := f.BlockByLabel(b[args[0]]).Term.(*TermRet)
		return ok && len(ret.Vals) > 0
	})
	reg.RegisterAction("testDropRet", func(_ rewrite.IRGraphMut, b grace.BlockBindings) bool {
		f.BlockByLabel(b["b"]).Term = &TermRet{}
		return true
	})
	res := grace.ApplyRules(FuncAsMutGraph(f), rules, reg, 10)
	if res.ByRule["test-drop-ret"] != 1 || res.ByRule["dead-block-arg"] != 1 || len(f.Blocks[1].Params) != 0 {
		t.Fatalf("got %v, %d params left; want one drop then one dead-block-arg", res.ByRule, len(f.Blocks[1].Params))
	}
}
