package mir2

import (
	"testing"

	"github.com/minz/minzc/pkg/rewrite/grace"
)

func TestFuncAsGraph_Basic(t *testing.T) {
	f := buildTestFunc()
	g := FuncAsGraph(f)

	blocks := g.Blocks()
	if len(blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(blocks))
	}
	if blocks[0] != "entry" {
		t.Errorf("first block: got %s, want entry", blocks[0])
	}

	b := g.Block("entry")
	if b == nil {
		t.Fatal("Block('entry') returned nil")
	}
	if b.TermKind() != "br_if" {
		t.Errorf("entry term kind: got %s, want br_if", b.TermKind())
	}
	if b.InstCount() != 2 {
		t.Errorf("entry inst count: got %d, want 2", b.InstCount())
	}

	preds := g.Predecessors("then")
	if len(preds) != 1 || preds[0] != "entry" {
		t.Errorf("predecessors of 'then': got %v, want [entry]", preds)
	}
}

func TestFuncAsGraph_CondRetPattern(t *testing.T) {
	f := buildCondRetFunc()
	g := FuncAsGraph(f)

	// Parse the cond-ret-sink Grace rule
	src := `(grace cond-ret-sink 15
		(match
			(block ?cur (term-kind "br_if"))
			(block ?else (term-kind "ret"))
			(edge ?cur ?else "else"))
		(where
			(no-params ?else)
			(all-pure ?else)
			(pred-count ?else == 1))
		(action
			(hoist-insts ?else ?cur)))`

	rules, err := grace.ParseRules(src)
	if err != nil {
		t.Fatal(err)
	}

	matches := grace.FindAllMatches(g, &rules[0], nil)
	if len(matches) != 1 {
		t.Fatalf("expected 1 cond-ret-sink match, got %d", len(matches))
	}
	if matches[0]["cur"] != "entry" || matches[0]["else"] != "else" {
		t.Errorf("unexpected bindings: %v", matches[0])
	}
}

func TestFuncAsMutGraph_HoistInsts(t *testing.T) {
	f := buildCondRetFunc()
	g := FuncAsMutGraph(f)

	elseBlock := f.BlockByLabel("else")
	origCount := len(elseBlock.Insts)
	if origCount == 0 {
		t.Fatal("else block should have instructions")
	}

	entryBlock := f.BlockByLabel("entry")
	entryCount := len(entryBlock.Insts)

	g.HoistInsts("else", "entry")

	if len(entryBlock.Insts) != entryCount+origCount {
		t.Errorf("entry insts after hoist: got %d, want %d", len(entryBlock.Insts), entryCount+origCount)
	}
	if len(elseBlock.Insts) != 0 {
		t.Errorf("else insts after hoist: got %d, want 0", len(elseBlock.Insts))
	}
}

// ── Test helpers ────────────────────────────────────────────────────────────

func buildTestFunc() *Func {
	f := &Func{Name: "test", nextReg: 100}
	entry := &Block{
		Label: "entry",
		Insts: []*Inst{
			{Op: OpConst, Dst: Reg(1), Imm: 42, Ty: TyU8},
			{Op: OpCmp, Dst: Reg(2), Src: [2]Reg{Reg(1), Reg(3)}, Ty: TyBool},
		},
		Term: &TermBrIf{Cond: Reg(2), Then: "then", Else: "else"},
	}
	then := &Block{
		Label: "then",
		Term:  &TermRet{Vals: []Reg{Reg(1)}},
	}
	els := &Block{
		Label: "else",
		Term:  &TermRet{Vals: []Reg{Reg(3)}},
	}
	f.Blocks = []*Block{entry, then, els}
	return f
}

func buildCondRetFunc() *Func {
	f := &Func{Name: "condret_test", nextReg: 100}
	entry := &Block{
		Label: "entry",
		Insts: []*Inst{
			{Op: OpConst, Dst: Reg(1), Imm: 42, Ty: TyU8},
			{Op: OpCmp, Dst: Reg(2), Src: [2]Reg{Reg(1), Reg(3)}, Ty: TyBool, Cond: CmpLt},
		},
		Term: &TermBrIf{Cond: Reg(2), Then: "then", Else: "else"},
	}
	then := &Block{
		Label: "then",
		Term:  &TermRet{Vals: []Reg{Reg(1)}},
	}
	els := &Block{
		Label: "else",
		// Pure instructions that could be hoisted
		Insts: []*Inst{
			{Op: OpSub, Dst: Reg(4), Src: [2]Reg{Reg(3), Reg(1)}, Ty: TyU8},
		},
		Term: &TermRet{Vals: []Reg{Reg(4)}},
	}
	f.Blocks = []*Block{entry, then, els}
	return f
}
