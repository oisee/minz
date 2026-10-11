package mir2

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/rewrite/grace"
)

var testLoopOrigin = LoopOrigin{SiteID: "source:loop:17", Role: "main", Rule: "peel"}

func verifyOriginsFunc(t *testing.T, f *Func) {
	t.Helper()
	if err := Verify(&Module{Funcs: []*Func{f}}); err != nil {
		t.Fatal(err)
	}
}

func applyOriginRule(t *testing.T, f *Func, rule string) int {
	t.Helper()
	rules, err := grace.ParseRules(rule)
	if err != nil {
		t.Fatal(err)
	}
	return grace.ApplyRules(FuncAsMutGraph(f), rules, buildRegistry(f), 20).Applied
}

func TestLoopOriginsSplitJoinRet(t *testing.T) {
	for _, pass := range []struct {
		name string
		run  func(*Func) bool
	}{{"go", SplitJoinRet}, {"grace", RunGraceSplitJoinRet}} {
		t.Run(pass.name, func(t *testing.T) {
			f := joinRetCollisionFunc()
			f.LoopOrigins = map[string]LoopOrigin{"join": testLoopOrigin}
			verifyOriginsFunc(t, f)
			if !pass.run(f) {
				t.Fatal("did not split")
			}
			for _, label := range []string{"a", "b"} {
				retLabel := f.BlockByLabel(label).Term.(*TermBrIf).Else
				if got := f.LoopOrigins[retLabel]; got != testLoopOrigin {
					t.Fatalf("clone %s: got %+v", retLabel, got)
				}
			}
			verifyOriginsFunc(t, f)
			EliminateDeadBlocks(f)
			if _, ok := f.LoopOrigins["join"]; ok {
				t.Fatal("removed join retains attribution")
			}
			ReorderBlocks(f)
			verifyOriginsFunc(t, f)
		})
	}
}

func originChain() *Func {
	return &Func{Name: "chain", Blocks: []*Block{
		{Label: "entry", Term: &TermJmp{Target: "body"}},
		{Label: "body", Insts: []*Inst{{Op: OpConst, Dst: 1, Ty: TyU8, Imm: 42}}, Term: &TermRet{Vals: []Reg{1}}},
	}, LoopOrigins: map[string]LoopOrigin{"body": testLoopOrigin}}
}

func TestLoopOriginsGraceMerge(t *testing.T) {
	for _, mode := range []string{"rule", "production"} {
		t.Run(mode, func(t *testing.T) {
			f := originChain()
			verifyOriginsFunc(t, f)
			if mode == "rule" {
				if applyOriginRule(t, f, graceBlockMergeRule) != 1 {
					t.Fatal("did not merge")
				}
			} else if !RunGracePasses(f, nil) {
				t.Fatal("production passes did not transform")
			}
			if len(f.Blocks) != 1 || f.LoopOrigins["entry"] != testLoopOrigin || len(f.LoopOrigins) != 1 {
				t.Fatalf("lost merged lineage: %+v", f.LoopOrigins)
			}
			verifyOriginsFunc(t, f)
			got, err := NewVM(&Module{Funcs: []*Func{f}}).Call("chain", nil)
			if err != nil || len(got) != 1 || got[0].I != 42 {
				t.Fatalf("merged function: %v %v", got, err)
			}
		})
	}
	for _, other := range []LoopOrigin{
		{SiteID: "another-site", Role: "main", Rule: "peel"},
		{SiteID: testLoopOrigin.SiteID, Role: "remainder", Rule: "peel"},
		{SiteID: testLoopOrigin.SiteID, Role: "main", Rule: "another-rule"},
	} {
		f := originChain()
		f.LoopOrigins["entry"] = other
		if RunGracePasses(f, nil) || len(f.Blocks) != 2 {
			t.Fatal("fused distinct attributions")
		}
		if f.LoopOrigins["entry"] != other || f.LoopOrigins["body"] != testLoopOrigin {
			t.Fatal("changed attribution on rejected fusion")
		}
		verifyOriginsFunc(t, f)
	}
}

func TestLoopOriginsRemovalAndVerification(t *testing.T) {
	f := originChain()
	f.Blocks = append(f.Blocks, &Block{Label: "dead", Term: &TermRet{}})
	f.LoopOrigins["dead"] = LoopOrigin{SiteID: "dead-site", Role: "remainder"}
	EliminateDeadBlocks(f)
	if _, ok := f.LoopOrigins["dead"]; ok {
		t.Fatal("dead block retains origin")
	}
	if f.LoopOrigins["body"] != testLoopOrigin {
		t.Fatal("live origin lost")
	}
	verifyOriginsFunc(t, f)
	FuncAsMutGraph(f).RemoveBlock("body")
	if len(f.LoopOrigins) != 0 {
		t.Fatal("adapter removal retained origin")
	}
	// Repair the edge explicitly: removal alone does not repair the CFG.
	f.Blocks[0].Term = &TermRet{}
	verifyOriginsFunc(t, f)
	f.LoopOrigins["missing"] = testLoopOrigin
	EliminateDeadBlocks(f)
	if err := Verify(&Module{Funcs: []*Func{f}}); err == nil || !strings.Contains(err.Error(), "missing block @missing") {
		t.Fatalf("invalid remnant must be reported: %v", err)
	}
	delete(f.LoopOrigins, "missing")
	for _, invalid := range []LoopOrigin{{Role: "main"}, {SiteID: "site"}} {
		f.LoopOrigins["entry"] = invalid
		if err := Verify(&Module{Funcs: []*Func{f}}); err == nil {
			t.Fatal("accepted incomplete attribution")
		}
	}
}

func TestLoopOriginsBoundedUnrollClones(t *testing.T) {
	f := &Func{Name: "bounded", nextReg: 10, Contract: Contract{Params: []Param{{Reg: 1, Ty: TyU8}}}, Blocks: []*Block{
		{Label: "entry", Term: &TermJmp{Target: "head", Args: []Reg{1}}},
		{Label: "head", Params: []BlockParam{{Dst: 2, Ty: TyU8}}, Insts: []*Inst{
			{Op: OpConst, Dst: 3, Ty: TyU8, Imm: 100},
			{Op: OpCmp, Dst: 4, Ty: TyBool, SrcTy: TyU8, Cond: CmpUge, Src: [2]Reg{2, 3}},
		}, Term: &TermBrIf{Cond: 4, Then: "body", Else: "exit"}},
		{Label: "body", Insts: []*Inst{{Op: OpSub, Dst: 5, Ty: TyU8, Src: [2]Reg{2, 3}}}, Term: &TermJmp{Target: "head", Args: []Reg{5}}},
		{Label: "exit", Term: &TermRet{Vals: []Reg{1}}},
		// Existing labels exercise clone-name collisions.
		{Label: "unroll_sub1", Term: &TermRet{}},
		{Label: "unroll_cmp1", Term: &TermRet{}},
	}, LoopOrigins: map[string]LoopOrigin{
		"head": {SiteID: "site", Role: "header", Rule: "bounded"},
		"body": {SiteID: "site", Role: "main", Rule: "bounded"},
	}}
	verifyOriginsFunc(t, f)
	if applyOriginRule(t, f, graceBoundedLoopUnrollRule) != 1 {
		t.Fatal("did not unroll")
	}
	for _, pair := range [][2]string{{"head", "unroll_cmp1_1"}, {"body", "unroll_sub1_1"}} {
		if f.BlockByLabel(pair[1]) == nil || f.LoopOrigins[pair[1]] != f.LoopOrigins[pair[0]] {
			t.Fatalf("clone lost lineage: %v %+v", pair, f.LoopOrigins)
		}
	}
	verifyOriginsFunc(t, f)
	EliminateDeadBlocks(f)
	verifyOriginsFunc(t, f)
}

func TestLoopOriginsDirectReturnSplit(t *testing.T) {
	for _, pass := range []func(*Func) bool{SplitJoinRet, RunGraceSplitJoinRet} {
		for _, conflict := range []bool{false, true} {
			f := &Func{Name: "direct", Blocks: []*Block{
				{Label: "entry", Insts: []*Inst{{Op: OpConst, Dst: 1, Ty: TyU8, Imm: 7}}, Term: &TermJmp{Target: "join", Args: []Reg{1}}},
				{Label: "join", Params: []BlockParam{{Dst: 2, Ty: TyU8}}, Term: &TermRet{Vals: []Reg{2}}},
			}, LoopOrigins: map[string]LoopOrigin{"join": testLoopOrigin}}
			other := LoopOrigin{SiteID: "other", Role: "remainder"}
			if conflict {
				f.LoopOrigins["entry"] = other
			}
			if changed := pass(f); changed == conflict {
				t.Fatalf("conflict=%v changed=%v", conflict, changed)
			}
			EliminateDeadBlocks(f)
			if conflict {
				if len(f.Blocks) != 2 || f.LoopOrigins["entry"] != other || f.LoopOrigins["join"] != testLoopOrigin {
					t.Fatal("distinct origins fused")
				}
			} else if len(f.Blocks) != 1 || f.LoopOrigins["entry"] != testLoopOrigin || len(f.LoopOrigins) != 1 {
				t.Fatal("direct return lost join lineage")
			}
			verifyOriginsFunc(t, f)
		}
	}
}

func TestLoopOriginsCondRetSink(t *testing.T) {
	for _, pass := range []func(*Func) bool{CondRetSink, RunGraceCondRetSink} {
		for _, conflict := range []bool{false, true} {
			f := &Func{Name: "sink", Contract: Contract{Params: []Param{{Reg: 1, Ty: TyBool}}}, Blocks: []*Block{
				{Label: "entry", Term: &TermBrIf{Cond: 1, Then: "yes", Else: "no"}},
				{Label: "yes", Insts: []*Inst{{Op: OpConst, Dst: 2, Ty: TyU8, Imm: 11}}, Term: &TermRet{Vals: []Reg{2}}},
				{Label: "no", Insts: []*Inst{{Op: OpConst, Dst: 3, Ty: TyU8, Imm: 22}}, Term: &TermRet{Vals: []Reg{3}}},
			}, LoopOrigins: map[string]LoopOrigin{"no": testLoopOrigin}}
			other := LoopOrigin{SiteID: "other", Role: "main"}
			if conflict {
				f.LoopOrigins["entry"] = other
			}
			verifyOriginsFunc(t, f)
			if changed := pass(f); changed == conflict {
				t.Fatalf("conflict=%v changed=%v", conflict, changed)
			}
			EliminateDeadBlocks(f)
			if conflict {
				if f.LoopOrigins["entry"] != other || f.LoopOrigins["no"] != testLoopOrigin {
					t.Fatal("conflicting sink changed metadata")
				}
			} else if f.LoopOrigins["entry"] != testLoopOrigin || len(f.LoopOrigins) != 1 {
				t.Fatal("sunk return lost lineage")
			}
			verifyOriginsFunc(t, f)
			for x := int64(0); x < 2; x++ {
				want := int64(22)
				if x == 1 {
					want = 11
				}
				got, err := NewVM(&Module{Funcs: []*Func{f}}).Call("sink", []Value{{I: x}})
				if err != nil || len(got) != 1 || got[0].I != want {
					t.Fatalf("sunk function: %v %v", got, err)
				}
			}
		}
	}
}

func TestLoopOriginsTailRecursionMove(t *testing.T) {
	f := &Func{Name: "countdown", nextReg: 10, Contract: Contract{Params: []Param{{Reg: 1, Ty: TyU8}}, Returns: []Return{{Ty: TyU8, Class: ClassAcc}}}, Blocks: []*Block{
		{Label: "entry", Insts: []*Inst{
			{Op: OpConst, Dst: 2, Ty: TyU8, Imm: 0},
			{Op: OpCmp, Dst: 3, Ty: TyBool, SrcTy: TyU8, Cond: CmpNe, Src: [2]Reg{1, 2}},
		}, Term: &TermBrIf{Cond: 3, Then: "recurse", Else: "exit"}},
		{Label: "recurse", Insts: []*Inst{
			{Op: OpConst, Dst: 4, Ty: TyU8, Imm: 1},
			{Op: OpSub, Dst: 5, Ty: TyU8, Src: [2]Reg{1, 4}},
			{Op: OpCall, Dst: 6, Ty: TyU8, Sym: "countdown", Args: []Reg{5}},
		}, Term: &TermRet{Vals: []Reg{6}}},
		{Label: "exit", Term: &TermRet{Vals: []Reg{1}}},
	}, LoopOrigins: map[string]LoopOrigin{"entry": testLoopOrigin, "recurse": {SiteID: "other-site", Role: "remainder"}}}
	verifyOriginsFunc(t, f)
	if applyOriginRule(t, f, graceTailRecursionRule) != 1 {
		t.Fatal("did not eliminate tail recursion")
	}
	if _, ok := f.LoopOrigins["entry"]; ok {
		t.Fatal("empty entry retains moved lineage")
	}
	if f.LoopOrigins["countdown_tail_loop"] != testLoopOrigin || f.LoopOrigins["recurse"].SiteID != "other-site" {
		t.Fatalf("lost moved lineage: %+v", f.LoopOrigins)
	}
	verifyOriginsFunc(t, f)
	got, err := NewVM(&Module{Funcs: []*Func{f}}).Call("countdown", []Value{{I: 4}})
	if err != nil || len(got) != 1 || got[0].I != 0 {
		t.Fatalf("tail loop: %v %v", got, err)
	}
}

func TestLoopOriginsGraceForwardRemoval(t *testing.T) {
	for _, params := range []bool{false, true} {
		f := &Func{Name: "forward", Blocks: []*Block{
			{Label: "entry", Insts: []*Inst{{Op: OpConst, Dst: 1, Ty: TyU8, Imm: 7}}, Term: &TermJmp{Target: "tramp"}},
			{Label: "tramp", Term: &TermJmp{Target: "exit"}},
			{Label: "exit", Term: &TermRet{Vals: []Reg{1}}},
		}, LoopOrigins: map[string]LoopOrigin{"tramp": testLoopOrigin}}
		rule := graceEmptyBlockElimRule
		if params {
			f.Blocks[0].Term.(*TermJmp).Args = []Reg{1}
			f.Blocks[1].Params = []BlockParam{{Dst: 2, Ty: TyU8}}
			f.Blocks[1].Term.(*TermJmp).Args = []Reg{2}
			f.Blocks[2].Params = []BlockParam{{Dst: 3, Ty: TyU8}}
			f.Blocks[2].Term = &TermRet{Vals: []Reg{3}}
			rule = graceParamForwardRule
		}
		verifyOriginsFunc(t, f)
		if applyOriginRule(t, f, rule) != 1 {
			t.Fatal("did not remove forwarding block")
		}
		if len(f.LoopOrigins) != 0 || f.BlockByLabel("tramp") != nil {
			t.Fatal("eliminated forwarding block retains lineage")
		}
		verifyOriginsFunc(t, f)
	}
}

func TestLoopOriginsLUTReplacement(t *testing.T) {
	f := originChain()
	f.Contract = Contract{Params: []Param{{Name: "x", Reg: 2, Ty: NewRanged(TyU8, 0, 4), Class: ClassAcc}}, Returns: []Return{{Ty: TyU8, Class: ClassAcc}}}
	f.nextReg = 10
	// Even a reused entry label must not retain the old body's attribution.
	f.LoopOrigins["entry"] = testLoopOrigin
	m := &Module{Funcs: []*Func{f}}
	if !LUTGen(m) {
		t.Fatal("did not generate lookup")
	}
	if len(f.LoopOrigins) != 0 {
		t.Fatal("lookup inherited loop attribution")
	}
	verifyOriginsFunc(t, f)
}

func TestLoopOriginsCondCallFold(t *testing.T) {
	for _, mode := range []string{"go", "grace"} {
		for _, conflict := range []bool{false, true} {
			f := &Func{Name: "call", Contract: Contract{Params: []Param{{Reg: 1, Ty: TyBool}}}, Blocks: []*Block{
				{Label: "entry", Term: &TermBrIf{Cond: 1, Then: "call_body", Else: "exit"}},
				{Label: "call_body", Insts: []*Inst{{Op: OpCall, Sym: "work"}}, Term: &TermJmp{Target: "exit"}},
				{Label: "exit", Term: &TermRet{}},
			}, LoopOrigins: map[string]LoopOrigin{"call_body": testLoopOrigin}}
			other := LoopOrigin{SiteID: "other", Role: "main"}
			if conflict {
				f.LoopOrigins["entry"] = other
			}
			verifyOriginsFunc(t, f)
			var applied int
			if mode == "go" {
				applied = FoldConditionalCalls(f)
			} else {
				applied = applyOriginRule(t, f, graceCondCallRule)
			}
			if (applied > 0) == conflict {
				t.Fatalf("mode=%s conflict=%v applied=%d", mode, conflict, applied)
			}
			EliminateDeadBlocks(f)
			if conflict {
				if f.LoopOrigins["entry"] != other || f.LoopOrigins["call_body"] != testLoopOrigin {
					t.Fatal("conflicting call fusion changed lineage")
				}
			} else if f.LoopOrigins["entry"] != testLoopOrigin || len(f.LoopOrigins) != 1 || f.Blocks[0].Insts[0].Op != OpCallCond {
				t.Fatal("conditional call lost lineage")
			}
			verifyOriginsFunc(t, f)
		}
	}
}

func TestLoopOriginsCompareFusion(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		f := &Func{Name: "max", nextReg: 10, Contract: Contract{Params: []Param{{Reg: 1, Ty: TyU8}, {Reg: 2, Ty: TyU8}}}, Blocks: []*Block{
			{Label: "entry", Insts: []*Inst{{Op: OpCmp, Dst: 3, Ty: TyBool, SrcTy: TyU8, Cond: CmpNe, Src: [2]Reg{1, 2}}}, Term: &TermCondRet{Cond: 3, Vals: []Reg{1}, Then: "body"}},
			{Label: "body", Insts: []*Inst{{Op: OpCmp, Dst: 4, Ty: TyBool, SrcTy: TyU8, Cond: CmpUgt, Src: [2]Reg{1, 2}}}, Term: &TermBrIf{Cond: 4, Then: "greater", Else: "less"}},
			{Label: "greater", Term: &TermRet{Vals: []Reg{1}}},
			{Label: "less", Term: &TermRet{Vals: []Reg{2}}},
		}, LoopOrigins: map[string]LoopOrigin{"body": testLoopOrigin}}
		other := LoopOrigin{SiteID: "other", Role: "main"}
		if conflict {
			f.LoopOrigins["entry"] = other
		}
		verifyOriginsFunc(t, f)
		if applied := applyOriginRule(t, f, graceFuseCmpBrIf2Rule); (applied > 0) == conflict {
			t.Fatalf("conflict=%v applied=%d", conflict, applied)
		}
		if !conflict {
			retLabel := f.Blocks[0].Term.(*TermBrIf2).Eq
			if _, ok := f.LoopOrigins[retLabel]; ok {
				t.Fatal("equality return inherited unrelated body attribution")
			}
		}
		EliminateDeadBlocks(f)
		if !conflict && (f.LoopOrigins["entry"] != testLoopOrigin || len(f.LoopOrigins) != 1) {
			t.Fatal("compare fusion lost body lineage")
		}
		verifyOriginsFunc(t, f)
		for _, args := range [][2]int64{{4, 4}, {3, 8}, {8, 3}} {
			want := args[0]
			if args[1] > want {
				want = args[1]
			}
			got, err := NewVM(&Module{Funcs: []*Func{f}}).Call("max", []Value{{I: args[0]}, {I: args[1]}})
			if err != nil || len(got) != 1 || got[0].I != want {
				t.Fatalf("comparison fusion: %v %v", got, err)
			}
		}
	}
}
