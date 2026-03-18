package lir

import "testing"

// TestBranchToNext: two blocks, first JP targets second → JP becomes TermNone.
func TestBranchToNext(t *testing.T) {
	m := RISC32
	constPat := findConstPat(m, 0)

	prog := &Prog{
		Name: "branch_to_next",
		Desc: m,
		Blocks: []Block{
			{
				Label: "a",
				Insts: []Inst{
					{Pat: constPat, Dst: Operand{Phys: 0}, Imm: 42},
				},
				Term: Term{Kind: TermJump, Targets: []string{"b"}},
			},
			{
				Label: "b",
				Term:  Term{Kind: TermReturn, RetVals: []Operand{{Phys: 0}}},
			},
		},
	}

	rules := []BlockRewriteRule{branchToNextRule()}
	result := ApplyBlockRules(prog, rules, 5)

	if result.Applied != 1 {
		t.Fatalf("expected 1 application, got %d", result.Applied)
	}

	a := findBlock(prog, "a")
	if a == nil {
		t.Fatal("block a not found")
	}
	if a.Term.Kind != TermNone {
		t.Errorf("expected TermNone, got %d", a.Term.Kind)
	}

	t.Logf("branch_to_next: JP a→b eliminated (fallthrough) ✓")
}

// TestEmptyBlockElim: A→B→C where B is empty → A targets C, B gone.
func TestEmptyBlockElim(t *testing.T) {
	m := RISC32
	constPat := findConstPat(m, 0)

	prog := &Prog{
		Name: "empty_elim",
		Desc: m,
		Blocks: []Block{
			{
				Label: "a",
				Insts: []Inst{
					{Pat: constPat, Dst: Operand{Phys: 0}, Imm: 99},
				},
				Term: Term{Kind: TermJump, Targets: []string{"b"}},
			},
			{
				Label: "b",
				// Empty: no insts, no params, just a jump
				Term: Term{Kind: TermJump, Targets: []string{"c"}},
			},
			{
				Label: "c",
				Term:  Term{Kind: TermReturn, RetVals: []Operand{{Phys: 0}}},
			},
		},
	}

	rules := []BlockRewriteRule{emptyBlockElimRule()}
	result := ApplyBlockRules(prog, rules, 5)

	if result.Applied == 0 {
		t.Fatal("expected at least 1 application")
	}

	// B should be gone
	if findBlock(prog, "b") != nil {
		t.Error("block b should have been deleted")
	}

	// A should now target C
	a := findBlock(prog, "a")
	if a == nil {
		t.Fatal("block a not found")
	}
	if len(a.Term.Targets) == 0 || a.Term.Targets[0] != "c" {
		t.Errorf("expected a→c, got a→%v", a.Term.Targets)
	}

	t.Logf("empty_block_elim: A→B→C collapsed to A→C ✓")
}

// TestBranchInversion: Branch then=next block → targets swap.
func TestBranchInversion(t *testing.T) {
	m := RISC32
	constPat := findConstPat(m, 0)
	regs := pickNFromSet(m.LocsOfWidth(m.WordSize), 2)

	prog := &Prog{
		Name: "branch_inv",
		Desc: m,
		Blocks: []Block{
			{
				Label: "cond",
				Insts: []Inst{
					{Pat: constPat, Dst: Operand{Phys: regs[0]}, Imm: 1},
				},
				// then=next (which is "then_block"), else="else_block"
				Term: Term{
					Kind:    TermBranch,
					Cond:    Operand{Phys: regs[0]},
					Targets: []string{"then_block", "else_block"},
				},
			},
			{
				Label: "then_block",
				Insts: []Inst{
					{Pat: constPat, Dst: Operand{Phys: regs[1]}, Imm: 10},
				},
				Term: Term{Kind: TermReturn, RetVals: []Operand{{Phys: regs[1]}}},
			},
			{
				Label: "else_block",
				Insts: []Inst{
					{Pat: constPat, Dst: Operand{Phys: regs[1]}, Imm: 20},
				},
				Term: Term{Kind: TermReturn, RetVals: []Operand{{Phys: regs[1]}}},
			},
		},
	}

	rules := []BlockRewriteRule{branchInversionRule()}
	result := ApplyBlockRules(prog, rules, 5)

	if result.Applied != 1 {
		t.Fatalf("expected 1 application, got %d", result.Applied)
	}

	cond := findBlock(prog, "cond")
	if cond == nil {
		t.Fatal("cond block not found")
	}
	// After inversion: Targets[0] should be "else_block", Targets[1] should be "then_block"
	if cond.Term.Targets[0] != "else_block" || cond.Term.Targets[1] != "then_block" {
		t.Errorf("expected [else_block, then_block], got %v", cond.Term.Targets)
	}

	t.Logf("branch_inversion: swapped then/else targets ✓")
}

// TestBlockMerge: P→S (single pred) → S merged into P.
func TestBlockMerge(t *testing.T) {
	m := RISC32
	constPat := findConstPat(m, 0)
	addPat := findAddPat(m, 0)
	regs := pickNFromSet(m.LocsOfWidth(m.WordSize), 2)

	prog := &Prog{
		Name: "block_merge",
		Desc: m,
		Blocks: []Block{
			{
				Label: "p",
				Insts: []Inst{
					{Pat: constPat, Dst: Operand{Phys: regs[0]}, Imm: 5},
				},
				Term: Term{Kind: TermJump, Targets: []string{"s"}},
			},
			{
				Label: "s",
				Insts: []Inst{
					{Pat: addPat, Dst: Operand{Phys: regs[0]},
						Srcs: [2]Operand{{Phys: regs[0]}, {Phys: regs[1]}}},
				},
				Term: Term{Kind: TermReturn, RetVals: []Operand{{Phys: regs[0]}}},
			},
		},
	}

	rules := []BlockRewriteRule{blockMergeRule()}
	result := ApplyBlockRules(prog, rules, 5)

	if result.Applied != 1 {
		t.Fatalf("expected 1 application, got %d", result.Applied)
	}

	// S should be gone
	if findBlock(prog, "s") != nil {
		t.Error("block s should have been merged into p")
	}

	// P should now have both instructions and return terminator
	p := findBlock(prog, "p")
	if p == nil {
		t.Fatal("block p not found")
	}
	if len(p.Insts) != 2 {
		t.Errorf("expected 2 insts in merged block, got %d", len(p.Insts))
	}
	if p.Term.Kind != TermReturn {
		t.Errorf("expected TermReturn, got %d", p.Term.Kind)
	}

	t.Logf("block_merge: P+S merged into P (%d insts, TermReturn) ✓", len(p.Insts))
}

// TestLoopRotateViaDSL: While loop (sum 1..5) → DJNZ, VM gives 15.
func TestLoopRotateViaDSL(t *testing.T) {
	machines := []*MachineDesc{RISC32, RISC8}

	for _, m := range machines {
		t.Run(m.Name, func(t *testing.T) {
			prog := makeSumProg_While(m, 5)

			// Execute before rotation
			vm1 := NewVM(m)
			r1, err := vm1.ExecProg(prog)
			if err != nil {
				t.Fatal("before:", err)
			}

			// Rotate via DSL rules
			rules := []BlockRewriteRule{loopRotateDJNZRule(), loopRotateRule()}
			result := ApplyBlockRules(prog, rules, 5)

			if result.Applied == 0 {
				t.Fatal("expected loop rotation")
			}

			// Verify DJNZ on body
			body := findBlock(prog, "body")
			if body == nil {
				t.Fatal("body block not found")
			}
			if body.Term.Kind != TermDJNZ {
				t.Errorf("expected TermDJNZ, got %d", body.Term.Kind)
			}

			// Verify sub absorbed
			for _, inst := range body.Insts {
				if inst.Pat != nil && inst.Pat.MIROp == OpSub {
					t.Error("sub should have been absorbed by DJNZ")
				}
			}

			// Execute after rotation
			vm2 := NewVM(m)
			r2, err := vm2.ExecProg(prog)
			if err != nil {
				t.Fatal("after:", err)
			}

			if r1[0] != r2[0] {
				t.Errorf("DIVERGENCE: before=%d, after=%d", r1[0], r2[0])
			}
			if r2[0] != 15 {
				t.Errorf("expected 15, got %d", r2[0])
			}

			t.Logf("DSL loop rotation: sum(1..5) = %d ✓ (DJNZ, rule=%q, rounds=%d)",
				r2[0], "loop_rotate_djnz", result.Rounds)
		})
	}
}

// TestFixpoint: chain of 3 empty blocks → all collapse to single redirect.
func TestFixpoint(t *testing.T) {
	m := RISC32
	constPat := findConstPat(m, 0)

	prog := &Prog{
		Name: "fixpoint",
		Desc: m,
		Blocks: []Block{
			{
				Label: "entry",
				Insts: []Inst{
					{Pat: constPat, Dst: Operand{Phys: 0}, Imm: 77},
				},
				Term: Term{Kind: TermJump, Targets: []string{"e1"}},
			},
			{
				Label: "e1",
				Term:  Term{Kind: TermJump, Targets: []string{"e2"}},
			},
			{
				Label: "e2",
				Term:  Term{Kind: TermJump, Targets: []string{"e3"}},
			},
			{
				Label: "e3",
				Term:  Term{Kind: TermJump, Targets: []string{"done"}},
			},
			{
				Label: "done",
				Term:  Term{Kind: TermReturn, RetVals: []Operand{{Phys: 0}}},
			},
		},
	}

	rules := []BlockRewriteRule{emptyBlockElimRule()}
	result := ApplyBlockRules(prog, rules, 10)

	if result.Applied != 3 {
		t.Errorf("expected 3 eliminations, got %d", result.Applied)
	}

	// All empty blocks should be gone
	for _, label := range []string{"e1", "e2", "e3"} {
		if findBlock(prog, label) != nil {
			t.Errorf("block %s should have been eliminated", label)
		}
	}

	// entry should now target done directly
	entry := findBlock(prog, "entry")
	if entry == nil {
		t.Fatal("entry not found")
	}
	if len(entry.Term.Targets) == 0 || entry.Term.Targets[0] != "done" {
		t.Errorf("expected entry→done, got entry→%v", entry.Term.Targets)
	}

	t.Logf("fixpoint: 3 empty blocks collapsed in %d rounds ✓", result.Rounds)
}

// TestConvergence: DSL rotation on RISC32 + RISC8 → same result as old LoopRotateDJNZ.
func TestConvergence(t *testing.T) {
	machines := []*MachineDesc{RISC32, RISC8}
	var dslResults []uint64
	var oldResults []uint64

	for _, m := range machines {
		// DSL path
		prog1 := makeSumProg_While(m, 10)
		rules := []BlockRewriteRule{loopRotateDJNZRule(), loopRotateRule()}
		ApplyBlockRules(prog1, rules, 5)

		vm1 := NewVM(m)
		r1, err := vm1.ExecProg(prog1)
		if err != nil {
			t.Fatalf("DSL %s: %v", m.Name, err)
		}
		dslResults = append(dslResults, r1[0])

		// Old imperative path (recreated inline to avoid circular dependency)
		prog2 := makeSumProg_While(m, 10)
		oldRotate(prog2)

		vm2 := NewVM(m)
		r2, err := vm2.ExecProg(prog2)
		if err != nil {
			t.Fatalf("old %s: %v", m.Name, err)
		}
		oldResults = append(oldResults, r2[0])

		if r1[0] != r2[0] {
			t.Errorf("%s DIVERGENCE: DSL=%d, old=%d", m.Name, r1[0], r2[0])
		} else {
			t.Logf("%s: DSL=%d == old=%d ✓", m.Name, r1[0], r2[0])
		}
	}

	// Cross-machine convergence
	for i := 1; i < len(dslResults); i++ {
		if dslResults[i] != dslResults[0] {
			t.Errorf("cross-machine DIVERGENCE: %s=%d, %s=%d",
				machines[0].Name, dslResults[0],
				machines[i].Name, dslResults[i])
		}
	}

	if dslResults[0] != 55 {
		t.Errorf("expected 55, got %d", dslResults[0])
	} else {
		t.Logf("all machines converge: sum(1..10) = 55 ✓")
	}
}

// oldRotate is a direct reimplementation of the original LoopRotateDJNZ
// (before the Layer 4 refactor) for convergence testing.
func oldRotate(prog *Prog) {
	blockIdx := make(map[string]int, len(prog.Blocks))
	for i, b := range prog.Blocks {
		blockIdx[b.Label] = i
	}

	for hi := range prog.Blocks {
		head := &prog.Blocks[hi]
		if head.Term.Kind != TermBranch || len(head.Term.Targets) < 2 {
			continue
		}

		bodyLabel := head.Term.Targets[0]
		exitLabel := head.Term.Targets[1]
		cntPhys := head.Term.Cond.Phys

		bi, ok := blockIdx[bodyLabel]
		if !ok {
			continue
		}
		body := &prog.Blocks[bi]

		if body.Term.Kind != TermJump || len(body.Term.Targets) == 0 {
			continue
		}
		if body.Term.Targets[0] != head.Label {
			continue
		}
		if len(head.Insts) > 0 {
			continue
		}

		subIdx := -1
		for i, inst := range body.Insts {
			if inst.Pat != nil && inst.Pat.MIROp == OpSub && inst.Dst.Phys == cntPhys {
				subIdx = i
			}
		}

		if subIdx >= 0 {
			body.Insts = append(body.Insts[:subIdx], body.Insts[subIdx+1:]...)
			body.Term = Term{
				Kind:    TermDJNZ,
				Counter: Operand{Phys: cntPhys},
				Targets: []string{bodyLabel, exitLabel},
			}
		} else {
			body.Term = Term{
				Kind:    TermBranch,
				Cond:    head.Term.Cond,
				Targets: []string{bodyLabel, exitLabel},
			}
		}
	}
}
