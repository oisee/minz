package mir2

import (
	"testing"
)

func TestGraceRunner_DSE(t *testing.T) {
	// Build a function with a dead instruction
	f := &Func{Name: "test_dse", nextReg: 100}
	entry := &Block{
		Label: "entry",
		Insts: []*Inst{
			{Op: OpConst, Dst: Reg(1), Imm: 42, Ty: TyU8},   // used
			{Op: OpConst, Dst: Reg(2), Imm: 99, Ty: TyU8},   // dead — not used anywhere
			{Op: OpAdd, Dst: Reg(3), Src: [2]Reg{Reg(1), Reg(1)}, Ty: TyU8}, // dead
		},
		Term: &TermRet{Vals: []Reg{Reg(1)}},
	}
	f.Blocks = []*Block{entry}

	stats := NewGraceStats()
	changed := RunGracePasses(f, stats)
	if !changed {
		t.Fatal("expected Grace DSE to fire")
	}

	// Should have removed the dead const and dead add
	if len(entry.Insts) != 1 {
		t.Errorf("expected 1 instruction after DSE, got %d", len(entry.Insts))
		for _, inst := range entry.Insts {
			t.Logf("  %s dst=%d", inst.Op, inst.Dst)
		}
	}

	t.Logf("Stats: %s", stats)
}

func TestGraceRunner_CondRetSink(t *testing.T) {
	// Build the classic cond-ret-sink pattern:
	// entry: br_if %cond, @then, @else
	// else: sub %r3-%r1; ret %result
	f := &Func{Name: "test_condret", nextReg: 100}
	entry := &Block{
		Label: "entry",
		Insts: []*Inst{
			{Op: OpCmp, Dst: Reg(2), Src: [2]Reg{Reg(1), Reg(3)}, Ty: TyBool, Cond: CmpLt},
		},
		Term: &TermBrIf{
			Cond: Reg(2),
			Then: "then", ThenArgs: nil,
			Else: "else", ElseArgs: nil,
		},
	}
	then := &Block{
		Label: "then",
		Term:  &TermRet{Vals: []Reg{Reg(1)}},
	}
	els := &Block{
		Label: "else",
		Insts: []*Inst{
			{Op: OpSub, Dst: Reg(4), Src: [2]Reg{Reg(3), Reg(1)}, Ty: TyU8},
		},
		Term: &TermRet{Vals: []Reg{Reg(4)}},
	}
	f.Blocks = []*Block{entry, then, els}

	stats := NewGraceStats()
	changed := RunGracePasses(f, stats)
	if !changed {
		t.Fatal("expected Grace CondRetSink to fire")
	}

	// Entry should now have TermCondRet
	if _, ok := entry.Term.(*TermCondRet); !ok {
		t.Errorf("expected TermCondRet on entry, got %T", entry.Term)
	}

	// Entry should have hoisted the sub instruction
	foundSub := false
	for _, inst := range entry.Insts {
		if inst.Op == OpSub {
			foundSub = true
		}
	}
	if !foundSub {
		t.Error("expected hoisted sub instruction in entry block")
	}

	t.Logf("Stats: %s", stats)
}

func TestGraceRunner_SplitJoinRet(t *testing.T) {
	// Build join-ret pattern:
	// entry: br_if %cond, @then, @join(%r1)
	// then: jmp @join(%r2)
	// join(%result): ret %result
	f := &Func{Name: "test_joinret", nextReg: 100}
	entry := &Block{
		Label: "entry",
		Insts: []*Inst{
			{Op: OpCmp, Dst: Reg(3), Src: [2]Reg{Reg(1), Reg(2)}, Ty: TyBool, Cond: CmpEq},
		},
		Term: &TermBrIf{
			Cond:     Reg(3),
			Then:     "then",
			Else:     "join",
			ElseArgs: []Reg{Reg(1)},
		},
	}
	then := &Block{
		Label: "then",
		Term:  &TermJmp{Target: "join", Args: []Reg{Reg(2)}},
	}
	join := &Block{
		Label:  "join",
		Params: []BlockParam{{Dst: Reg(10), Ty: TyU8}},
		Term:   &TermRet{Vals: []Reg{Reg(10)}},
	}
	f.Blocks = []*Block{entry, then, join}

	stats := NewGraceStats()
	changed := RunGracePasses(f, stats)
	if !changed {
		t.Fatal("expected Grace SplitJoinRet to fire")
	}

	// Verify the split happened: the "then" block should no longer jump to "join".
	// Additional rules (cond-ret-sink, empty-block-elim) may further transform,
	// so we just check that splitJoinRet fired in the stats.
	if stats.ByRule["split-join-ret"] == 0 {
		t.Error("expected split-join-ret rule to fire")
	}

	t.Logf("Stats: %s", stats)
}

func TestGraceRunner_DeadBlockArg(t *testing.T) {
	// Build dead block arg pattern:
	// entry: jmp @target(%r1, %r2)
	// target(%p1, %p2): ret %p1  ← %p2 is dead
	f := &Func{Name: "test_deadarg", nextReg: 100}
	entry := &Block{
		Label: "entry",
		Term:  &TermJmp{Target: "target", Args: []Reg{Reg(1), Reg(2)}},
	}
	target := &Block{
		Label: "target",
		Params: []BlockParam{
			{Dst: Reg(10), Ty: TyU8},
			{Dst: Reg(11), Ty: TyU8}, // dead
		},
		Term: &TermRet{Vals: []Reg{Reg(10)}},
	}
	f.Blocks = []*Block{entry, target}

	stats := NewGraceStats()
	changed := RunGracePasses(f, stats)
	if !changed {
		t.Fatal("expected Grace DeadBlockArgElim to fire")
	}

	// target should have 1 param now
	if len(target.Params) != 1 {
		t.Errorf("expected 1 param after elim, got %d", len(target.Params))
	}

	// entry's jmp should have 1 arg
	if jmp, ok := entry.Term.(*TermJmp); ok {
		if len(jmp.Args) != 1 {
			t.Errorf("expected 1 jmp arg after elim, got %d", len(jmp.Args))
		}
	}

	t.Logf("Stats: %s", stats)
}

func TestGraceRunner_Stats(t *testing.T) {
	stats := NewGraceStats()

	// Run on a simple function with dead code
	f := &Func{Name: "stats_test", nextReg: 100}
	f.Blocks = []*Block{{
		Label: "entry",
		Insts: []*Inst{
			{Op: OpConst, Dst: Reg(1), Imm: 42, Ty: TyU8},
			{Op: OpConst, Dst: Reg(2), Imm: 99, Ty: TyU8}, // dead
		},
		Term: &TermRet{Vals: []Reg{Reg(1)}},
	}}

	RunGracePasses(f, stats)

	if stats.Total == 0 {
		t.Error("expected at least 1 rule application")
	}
	if stats.Funcs != 1 {
		t.Errorf("expected 1 func processed, got %d", stats.Funcs)
	}
	t.Logf("Stats: %s", stats)
}
