package lir

import (
	"strings"
	"testing"
)

// ── GCD test helpers ─────────────────────────────────────────────────────────

// opv creates an Operand with a specific vreg and pre-assigned physical register.
func opv(phys int, vreg int) Operand {
	return Operand{VReg: vreg, Allowed: Singleton(phys), Phys: phys}
}

// findPatByOp finds the first pattern for the given op code on a descriptor.
func findPatByOp(desc *MachineDesc, opcode int) *Pattern {
	for i := range desc.Patterns {
		if desc.Patterns[i].MIROp == opcode {
			return &desc.Patterns[i]
		}
	}
	return nil
}

// buildSimpleGCDProg builds a simplified GCD using Euclid's subtraction algorithm:
//
//	while a != b: if a > b then a = a - b else b = b - a
//	return a
//
// Uses RISC32 with pre-assigned registers.
// Equality test uses sub(a,b)==0 instead of cmp (cmp Z-flag is nonzero).
// Direction test uses cmp: a>b → flags=0, a<b → flags=1(C).
func buildSimpleGCDProg(initA, initB uint64) *Prog {
	desc := RISC32
	r0 := 0 // a
	r1 := 1 // b
	r2 := 2 // scratch/flags
	r3 := 3 // a-b
	r4 := 4 // b-a

	constPat := findPatByOp(desc, OpConst)
	cmpPat := findPatByOp(desc, OpCmp)
	subPat := findPatByOp(desc, OpSub)

	return &Prog{
		Name: "gcd",
		Desc: desc,
		Blocks: []Block{
			{
				// Block 0: entry — load a, b and jump to loop
				Label: "entry",
				Insts: []Inst{
					{Pat: constPat, Dst: opv(r0, 10), Imm: int64(initA)},
					{Pat: constPat, Dst: opv(r1, 11), Imm: int64(initB)},
				},
				Term: Term{
					Kind:    TermJump,
					Targets: []string{"loop"},
					Args:    [][]Operand{{opv(r0, 10), opv(r1, 11)}},
				},
			},
			{
				// Block 1: loop — check equality via sub(a,b)
				// sub gives 0 when a==b (branch takes "else" = done).
				Label: "loop",
				Params: []BlockParam{
					{VReg: 20, Allowed: Singleton(r0), Phys: r0}, // a
					{VReg: 21, Allowed: Singleton(r1), Phys: r1}, // b
				},
				Insts: []Inst{
					// sub(a, b) → 0 if equal, nonzero if not
					{Pat: subPat, Dst: opv(r2, 22), Srcs: [2]Operand{opv(r0, 20), opv(r1, 21)}},
				},
				Term: Term{
					Kind:    TermBranch,
					Cond:    opv(r2, 22), // nonzero = not equal → continue
					Targets: []string{"sub_a_b", "done"},
					Args: [][]Operand{
						{opv(r0, 20), opv(r1, 21)}, // to sub_a_b
						{opv(r0, 20)},               // to done (a == b, return a)
					},
				},
			},
			{
				// Block 2: sub_a_b — compute both subtractions and branch on direction
				Label: "sub_a_b",
				Params: []BlockParam{
					{VReg: 30, Allowed: Singleton(r0), Phys: r0}, // a
					{VReg: 31, Allowed: Singleton(r1), Phys: r1}, // b
				},
				Insts: []Inst{
					// a - b
					{Pat: subPat, Dst: opv(r3, 33), Srcs: [2]Operand{opv(r0, 30), opv(r1, 31)}},
					// b - a
					{Pat: subPat, Dst: opv(r4, 34), Srcs: [2]Operand{opv(r1, 31), opv(r0, 30)}},
					// cmp(a, b): a>b → 0, a<b → 1(C)
					{Pat: cmpPat, Dst: opv(r2, 32), Srcs: [2]Operand{opv(r0, 30), opv(r1, 31)}},
				},
				Term: Term{
					// cmp: a<b → flags=1 (nonzero) → take "then" (take_b_minus_a)
					//      a>b → flags=0           → take "else" (take_a_minus_b)
					Kind:    TermBranch,
					Cond:    opv(r2, 32),
					Targets: []string{"take_b_minus_a", "take_a_minus_b"},
					Args: [][]Operand{
						{opv(r0, 30), opv(r4, 34)}, // a<b: loop(a, b-a)
						{opv(r3, 33), opv(r1, 31)}, // a>b: loop(a-b, b)
					},
				},
			},
			{
				// Block 3: take_b_minus_a — jump back to loop with (a, b-a)
				Label: "take_b_minus_a",
				Params: []BlockParam{
					{VReg: 40, Allowed: Singleton(r0), Phys: r0},
					{VReg: 41, Allowed: Singleton(r1), Phys: r1},
				},
				Term: Term{
					Kind:    TermJump,
					Targets: []string{"loop"},
					Args:    [][]Operand{{opv(r0, 40), opv(r1, 41)}},
				},
			},
			{
				// Block 4: take_a_minus_b — jump back to loop with (a-b, b)
				Label: "take_a_minus_b",
				Params: []BlockParam{
					{VReg: 50, Allowed: Singleton(r0), Phys: r0},
					{VReg: 51, Allowed: Singleton(r1), Phys: r1},
				},
				Term: Term{
					Kind:    TermJump,
					Targets: []string{"loop"},
					Args:    [][]Operand{{opv(r0, 50), opv(r1, 51)}},
				},
			},
			{
				// Block 5: done — return a
				Label: "done",
				Params: []BlockParam{
					{VReg: 60, Allowed: Singleton(r0), Phys: r0},
				},
				Term: Term{
					Kind:    TermReturn,
					RetVals: []Operand{opv(r0, 60)},
				},
			},
		},
	}
}

// ── Test: GCD VM with pre-assigned registers ─────────────────────────────────

func TestGCD_VM(t *testing.T) {
	tests := []struct {
		a, b     uint64
		expected uint64
	}{
		{12, 8, 4},
		{48, 18, 6},
		{7, 7, 7},
		{15, 5, 5},
		{100, 75, 25},
		{17, 13, 1},
	}

	for _, tt := range tests {
		prog := buildSimpleGCDProg(tt.a, tt.b)
		vm := NewVM(prog.Desc)

		result, err := vm.ExecProg(prog)
		if err != nil {
			t.Fatalf("gcd(%d,%d): VM error: %v", tt.a, tt.b, err)
		}
		if len(result) == 0 {
			t.Fatalf("gcd(%d,%d): no return value", tt.a, tt.b)
		}
		if result[0] != tt.expected {
			t.Errorf("gcd(%d,%d) = %d, want %d", tt.a, tt.b, result[0], tt.expected)
		}
	}
}

// ── Test: GCD WFC — virtual registers, verify zero parallel copies ───────────

func buildGCDProg_Virtual() *Prog {
	desc := RISC32
	allRegs := desc.LocsOfWidth(16)

	constPat := findPatByOp(desc, OpConst)
	cmpPat := findPatByOp(desc, OpCmp)
	subPat := findPatByOp(desc, OpSub)

	vop := func(vreg int) Operand {
		return Operand{VReg: vreg, Allowed: allRegs, Phys: -1}
	}

	return &Prog{
		Name: "gcd",
		Desc: desc,
		Blocks: []Block{
			{
				Label: "entry",
				Insts: []Inst{
					{Pat: constPat, Dst: vop(10), Imm: 12},
					{Pat: constPat, Dst: vop(11), Imm: 8},
				},
				Term: Term{
					Kind:    TermJump,
					Targets: []string{"loop"},
					Args:    [][]Operand{{vop(10), vop(11)}},
				},
			},
			{
				// loop: sub(a,b) for equality test (0 = equal)
				Label: "loop",
				Params: []BlockParam{
					{VReg: 20, Allowed: allRegs, Phys: -1},
					{VReg: 21, Allowed: allRegs, Phys: -1},
				},
				Insts: []Inst{
					{Pat: subPat, Dst: vop(22), Srcs: [2]Operand{vop(20), vop(21)}},
				},
				Term: Term{
					Kind:    TermBranch,
					Cond:    vop(22), // nonzero = not equal
					Targets: []string{"sub_block", "done"},
					Args: [][]Operand{
						{vop(20), vop(21)},
						{vop(20)},
					},
				},
			},
			{
				// sub_block: compute both subs, then cmp for direction
				Label: "sub_block",
				Params: []BlockParam{
					{VReg: 30, Allowed: allRegs, Phys: -1},
					{VReg: 31, Allowed: allRegs, Phys: -1},
				},
				Insts: []Inst{
					{Pat: subPat, Dst: vop(33), Srcs: [2]Operand{vop(30), vop(31)}}, // a-b
					{Pat: subPat, Dst: vop(34), Srcs: [2]Operand{vop(31), vop(30)}}, // b-a
					{Pat: cmpPat, Dst: vop(32), Srcs: [2]Operand{vop(30), vop(31)}}, // cmp last
				},
				Term: Term{
					// cmp: a<b → 1 (nonzero, take then=take_bma)
					//      a>b → 0 (take else=take_amb)
					Kind:    TermBranch,
					Cond:    vop(32),
					Targets: []string{"take_bma", "take_amb"},
					Args: [][]Operand{
						{vop(30), vop(34)}, // a<b: loop(a, b-a)
						{vop(33), vop(31)}, // a>b: loop(a-b, b)
					},
				},
			},
			{
				Label: "take_bma",
				Params: []BlockParam{
					{VReg: 40, Allowed: allRegs, Phys: -1},
					{VReg: 41, Allowed: allRegs, Phys: -1},
				},
				Term: Term{
					Kind:    TermJump,
					Targets: []string{"loop"},
					Args:    [][]Operand{{vop(40), vop(41)}},
				},
			},
			{
				Label: "take_amb",
				Params: []BlockParam{
					{VReg: 50, Allowed: allRegs, Phys: -1},
					{VReg: 51, Allowed: allRegs, Phys: -1},
				},
				Term: Term{
					Kind:    TermJump,
					Targets: []string{"loop"},
					Args:    [][]Operand{{vop(50), vop(51)}},
				},
			},
			{
				Label: "done",
				Params: []BlockParam{
					{VReg: 60, Allowed: allRegs, Phys: -1},
				},
				Term: Term{
					Kind:    TermReturn,
					RetVals: []Operand{vop(60)},
				},
			},
		},
	}
}

func TestGCD_WFC(t *testing.T) {
	prog := buildGCDProg_Virtual()

	pw := NewProgWFC(prog)
	pw.Propagate()
	if err := pw.Collapse(); err != nil {
		t.Fatalf("WFC collapse: %v", err)
	}

	// Verify: zero parallel copies
	copies := pw.ParallelCopyCount()
	t.Logf("parallel copies: %d", copies)
	if copies != 0 {
		t.Errorf("expected 0 parallel copies, got %d", copies)
	}

	// Execute and verify gcd(12, 8) == 4
	vm := NewVM(prog.Desc)
	result, err := vm.ExecProg(prog)
	if err != nil {
		t.Fatalf("VM exec: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("no return value")
	}
	if result[0] != 4 {
		t.Errorf("gcd(12,8) = %d, want 4", result[0])
	}
}

// ── Test: ProgWFC propagation on simple two-block program ────────────────────

func TestProgWFC_SimplePropagate(t *testing.T) {
	desc := RISC32
	allRegs := desc.LocsOfWidth(16)
	constPat := findPatByOp(desc, OpConst)
	addPat := findPatByOp(desc, OpAdd)

	vop := func(vreg int) Operand {
		return Operand{VReg: vreg, Allowed: allRegs, Phys: -1}
	}

	prog := &Prog{
		Name: "add_blocks",
		Desc: desc,
		Blocks: []Block{
			{
				Label: "entry",
				Insts: []Inst{
					{Pat: constPat, Dst: vop(1), Imm: 10},
					{Pat: constPat, Dst: vop(2), Imm: 20},
				},
				Term: Term{
					Kind:    TermJump,
					Targets: []string{"add_block"},
					Args:    [][]Operand{{vop(1), vop(2)}},
				},
			},
			{
				Label: "add_block",
				Params: []BlockParam{
					{VReg: 10, Allowed: allRegs, Phys: -1},
					{VReg: 11, Allowed: allRegs, Phys: -1},
				},
				Insts: []Inst{
					{Pat: addPat, Dst: vop(12), Srcs: [2]Operand{vop(10), vop(11)}},
				},
				Term: Term{
					Kind:    TermReturn,
					RetVals: []Operand{vop(12)},
				},
			},
		},
	}

	pw := NewProgWFC(prog)
	iters := pw.Propagate()
	t.Logf("propagation iterations: %d", iters)

	if err := pw.Collapse(); err != nil {
		t.Fatalf("collapse: %v", err)
	}

	// Execute: should return 30
	vm := NewVM(desc)
	result, err := vm.ExecProg(prog)
	if err != nil {
		t.Fatalf("VM exec: %v", err)
	}
	if len(result) == 0 || result[0] != 30 {
		t.Errorf("expected 30, got %v", result)
	}
}

// ── Test: RPO ordering ───────────────────────────────────────────────────────

func TestProgWFC_RPO(t *testing.T) {
	desc := RISC32

	prog := &Prog{
		Name: "rpo_test",
		Desc: desc,
		Blocks: []Block{
			{Label: "entry", Term: Term{Kind: TermJump, Targets: []string{"middle"}}},
			{Label: "middle", Term: Term{Kind: TermBranch, Targets: []string{"left", "right"}, Cond: Operand{Phys: 0}}},
			{Label: "left", Term: Term{Kind: TermJump, Targets: []string{"exit"}}},
			{Label: "right", Term: Term{Kind: TermJump, Targets: []string{"exit"}}},
			{Label: "exit", Term: Term{Kind: TermReturn}},
		},
	}

	pw := NewProgWFC(prog)
	order := pw.rpo()

	if len(order) != 5 {
		t.Fatalf("expected 5 blocks in RPO, got %d: %v", len(order), order)
	}

	// entry must come first
	if order[0] != "entry" {
		t.Errorf("RPO[0] = %s, want entry", order[0])
	}
	// exit must come last
	if order[len(order)-1] != "exit" {
		t.Errorf("RPO[last] = %s, want exit", order[len(order)-1])
	}

	t.Logf("RPO order: %v", order)
}

// ── Test: Multi-block assembly emission ──────────────────────────────────────

func TestGCD_Asm(t *testing.T) {
	prog := buildSimpleGCDProg(12, 8)

	// Since this is pre-assigned, we can directly emit
	var sb strings.Builder
	for bi, b := range prog.Blocks {
		if bi == 0 {
			sb.WriteString("gcd:\n")
		} else {
			sb.WriteString(b.Label + ":\n")
		}
		for _, inst := range b.Insts {
			if inst.Pat == nil {
				continue
			}
			line := ExpandTemplateNamed(inst, prog.Desc)
			sb.WriteString("    " + line + "\n")
		}
		emitTerminator(&sb, &b.Term, bi, len(prog.Blocks))
	}

	asm := sb.String()
	t.Logf("GCD assembly:\n%s", asm)

	// Verify: no "parallel-copy" comments (all pre-assigned to same regs)
	if strings.Contains(asm, "parallel-copy") {
		t.Error("unexpected parallel-copy in pre-assigned GCD")
	}

	// Verify structure
	if !strings.Contains(asm, "gcd:") {
		t.Error("missing gcd: label")
	}
	if !strings.Contains(asm, "loop:") {
		t.Error("missing loop: label")
	}
	if !strings.Contains(asm, "done:") {
		t.Error("missing done: label")
	}
	if !strings.Contains(asm, "RET") {
		t.Error("missing RET")
	}
}

// ── Test: Block params are preserved through WFC ─────────────────────────────

func TestBlockParams_WFC(t *testing.T) {
	desc := RISC32
	allRegs := desc.LocsOfWidth(16)
	constPat := findPatByOp(desc, OpConst)
	addPat := findPatByOp(desc, OpAdd)

	vop := func(vreg int) Operand {
		return Operand{VReg: vreg, Allowed: allRegs, Phys: -1}
	}

	prog := &Prog{
		Name: "param_test",
		Desc: desc,
		Blocks: []Block{
			{
				Label: "entry",
				Insts: []Inst{
					{Pat: constPat, Dst: vop(1), Imm: 5},
				},
				Term: Term{
					Kind:    TermJump,
					Targets: []string{"body"},
					Args:    [][]Operand{{vop(1)}},
				},
			},
			{
				Label: "body",
				Params: []BlockParam{
					{VReg: 10, Allowed: allRegs, Phys: -1},
				},
				Insts: []Inst{
					{Pat: addPat, Dst: vop(11), Srcs: [2]Operand{vop(10), vop(10)}},
				},
				Term: Term{
					Kind:    TermReturn,
					RetVals: []Operand{vop(11)},
				},
			},
		},
	}

	pw := NewProgWFC(prog)
	pw.Propagate()
	if err := pw.Collapse(); err != nil {
		t.Fatalf("collapse: %v", err)
	}

	// Verify param got assigned a phys
	body := &prog.Blocks[1]
	if len(body.Params) == 0 {
		t.Fatal("body has no params")
	}
	if body.Params[0].Phys < 0 {
		t.Error("body param[0] not assigned a physical register")
	}

	// Execute: 5 + 5 = 10
	vm := NewVM(desc)
	result, err := vm.ExecProg(prog)
	if err != nil {
		t.Fatalf("VM: %v", err)
	}
	if len(result) == 0 || result[0] != 10 {
		t.Errorf("expected 10, got %v", result)
	}
}

// ── Test: SelectBlockInstructions with params ────────────────────────────────

func TestSelectBlockInstructions_Params(t *testing.T) {
	desc := RISC32
	allRegs := desc.LocsOfWidth(16)

	params := []BlockParam{
		{VReg: 1, Allowed: allRegs, Phys: -1},
		{VReg: 2, Allowed: allRegs, Phys: -1},
	}

	ops := []MIROp{
		{Op: OpAdd, Dst: 3, Src: [2]int{1, 2}, Width: 16},
	}

	result, err := SelectBlockInstructions(desc, ops, params)
	if err != nil {
		t.Fatalf("isel: %v", err)
	}

	// Verify params were pre-seeded
	if _, ok := result.VRegAllowed[1]; !ok {
		t.Error("vreg 1 (param) not in VRegAllowed")
	}
	if _, ok := result.VRegAllowed[2]; !ok {
		t.Error("vreg 2 (param) not in VRegAllowed")
	}
	if _, ok := result.VRegAllowed[3]; !ok {
		t.Error("vreg 3 (result) not in VRegAllowed")
	}
}
