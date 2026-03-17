package lir

import "testing"

// TestLoop_DJNZ tests a counted loop using the DJNZ terminator.
// Program: sum = 0; for i = N downto 1: sum += i; return sum
// For N=5: sum = 5+4+3+2+1 = 15
//
// LIR blocks:
//
//	entry:
//	    r0 = const 0     ; sum = 0
//	    r1 = const 5     ; counter = N
//	    jump @loop
//
//	loop:
//	    r0 = add r0, r1  ; sum += counter
//	    djnz r1, @loop, @exit
//
//	exit:
//	    ret r0
func TestLoop_DJNZ(t *testing.T) {
	machines := []*MachineDesc{RISC32, RISC8, CISC}

	for _, m := range machines {
		t.Run(m.Name, func(t *testing.T) {
			allRegs := m.LocsOfWidth(m.WordSize)
			if allRegs.IsEmpty() {
				allRegs = m.LocsOfWidth(8)
			}

			// We need two distinct physical registers for sum and counter.
			// Pick the first two available.
			var sumPhys, cntPhys int
			found := 0
			for i := 0; i < len(m.Locs) && found < 2; i++ {
				if allRegs.Has(i) {
					if found == 0 {
						sumPhys = i
					} else {
						cntPhys = i
					}
					found++
				}
			}
			if found < 2 {
				t.Skip("need at least 2 registers")
			}

			// For CISC: ADD A,r requires dst=A, src0=A, src1=r
			// So sum must be in A (index 0) for CISC
			addPat := findAddPat(m, 8)
			if addPat == nil {
				addPat = findAddPat(m, 0)
			}
			if addPat == nil {
				t.Fatal("no add pattern")
			}

			// Find appropriate registers for add: dst ∩ src0 ∩ pattern constraints
			sumPhys = pickFromSet(addPat.DstLocs, -1)
			cntPhys = pickFromSet(addPat.SrcLocs[1], sumPhys)
			if sumPhys < 0 || cntPhys < 0 {
				t.Fatal("can't find suitable registers for add pattern")
			}

			constPat := findConstPat(m, 8)
			if constPat == nil {
				constPat = findConstPat(m, 0)
			}
			if constPat == nil {
				t.Fatal("no const pattern")
			}

			t.Logf("sum=@%s(%d), cnt=@%s(%d), add=%s",
				m.Locs[sumPhys].Name, sumPhys,
				m.Locs[cntPhys].Name, cntPhys,
				addPat.Name)

			prog := &Prog{
				Name: "sum_1_to_n",
				Desc: m,
				Blocks: []Block{
					{
						Label: "entry",
						Insts: []Inst{
							{Pat: constPat, Dst: Operand{Phys: sumPhys}, Imm: 0},
							{Pat: constPat, Dst: Operand{Phys: cntPhys}, Imm: 5},
						},
						Term: Term{Kind: TermJump, Targets: []string{"loop"}},
					},
					{
						Label: "loop",
						Insts: []Inst{
							{
								Pat:  addPat,
								Dst:  Operand{Phys: sumPhys},
								Srcs: [2]Operand{{Phys: sumPhys}, {Phys: cntPhys}},
							},
						},
						Term: Term{
							Kind:    TermDJNZ,
							Counter: Operand{Phys: cntPhys},
							Targets: []string{"loop", "exit"},
						},
					},
					{
						Label: "exit",
						Term: Term{
							Kind:    TermReturn,
							RetVals: []Operand{{Phys: sumPhys}},
						},
					},
				},
			}

			vm := NewVM(m)
			result, err := vm.ExecProg(prog)
			if err != nil {
				t.Fatal(err)
			}

			if len(result) == 0 {
				t.Fatal("no return value")
			}

			expected := uint64(15) // 5+4+3+2+1
			if result[0] != expected {
				t.Errorf("sum(1..5) = %d, want %d", result[0], expected)
			} else {
				t.Logf("DJNZ loop: sum(1..5) = %d ✓ (counter@%s)",
					result[0], m.Locs[cntPhys].Name)
			}
		})
	}
}

// TestLoop_BranchWhile tests a header-tested while loop (br_if pattern).
// Same program: sum(1..5) = 15, but using TermBranch instead of TermDJNZ.
//
//	entry:
//	    sum = 0; cnt = 5; jump @head
//	head:
//	    cmp cnt, 0; branch (cnt!=0) @body, @exit
//	body:
//	    sum += cnt; cnt -= 1; jump @head
//	exit:
//	    ret sum
// TestLoop_BranchWhile tests a header-tested while loop using TermBranch.
// Uses counter value directly as branch condition (nonzero = continue).
//
//	entry: sum=0; cnt=5; one=1; jump @head
//	head:  branch cnt!=0 @body, @exit
//	body:  sum+=cnt; cnt-=1; jump @head
//	exit:  ret sum
func TestLoop_BranchWhile(t *testing.T) {
	machines := []*MachineDesc{RISC32, RISC8}

	for _, m := range machines {
		t.Run(m.Name, func(t *testing.T) {
			allRegs := m.LocsOfWidth(m.WordSize)
			if allRegs.IsEmpty() {
				allRegs = m.LocsOfWidth(8)
			}

			constPat := findConstPat(m, 0)
			addPat := findAddPat(m, 0)
			subPat := findSubPat(m, 0)
			if constPat == nil || addPat == nil || subPat == nil {
				t.Fatal("missing patterns")
			}

			regs := pickNFromSet(allRegs, 3)
			if len(regs) < 3 {
				t.Skip("need 3 registers")
			}
			sumR, cntR, oneR := regs[0], regs[1], regs[2]

			prog := &Prog{
				Name: "sum_while",
				Desc: m,
				Blocks: []Block{
					{
						Label: "entry",
						Insts: []Inst{
							{Pat: constPat, Dst: Operand{Phys: sumR}, Imm: 0},
							{Pat: constPat, Dst: Operand{Phys: cntR}, Imm: 5},
							{Pat: constPat, Dst: Operand{Phys: oneR}, Imm: 1},
						},
						Term: Term{Kind: TermJump, Targets: []string{"head"}},
					},
					{
						Label: "head",
						// Use counter directly as branch condition: cnt!=0 → body
						Term: Term{
							Kind:    TermBranch,
							Cond:    Operand{Phys: cntR},
							Targets: []string{"body", "exit"},
						},
					},
					{
						Label: "body",
						Insts: []Inst{
							{Pat: addPat, Dst: Operand{Phys: sumR},
								Srcs: [2]Operand{{Phys: sumR}, {Phys: cntR}}},
							{Pat: subPat, Dst: Operand{Phys: cntR},
								Srcs: [2]Operand{{Phys: cntR}, {Phys: oneR}}},
						},
						Term: Term{Kind: TermJump, Targets: []string{"head"}},
					},
					{
						Label: "exit",
						Term: Term{
							Kind:    TermReturn,
							RetVals: []Operand{{Phys: sumR}},
						},
					},
				},
			}

			vm := NewVM(m)
			result, err := vm.ExecProg(prog)
			if err != nil {
				t.Fatal(err)
			}

			if len(result) == 0 {
				t.Fatal("no return value")
			}

			expected := uint64(15)
			if result[0] != expected {
				t.Errorf("sum(1..5) = %d, want %d", result[0], expected)
			} else {
				t.Logf("while loop: sum(1..5) = %d ✓", result[0])
			}
		})
	}
}

// TestLoop_DJNZ_vs_While verifies DJNZ and While produce the same result.
// This is the convergence test: two different loop encodings, same semantics.
func TestLoop_DJNZ_vs_While(t *testing.T) {
	m := RISC32

	// Run DJNZ version
	djnzProg := makeSumProg_DJNZ(m, 10) // sum(1..10) = 55
	vm1 := NewVM(m)
	r1, err := vm1.ExecProg(djnzProg)
	if err != nil {
		t.Fatal("djnz:", err)
	}

	// Run While version
	whileProg := makeSumProg_While(m, 10)
	vm2 := NewVM(m)
	r2, err := vm2.ExecProg(whileProg)
	if err != nil {
		t.Fatal("while:", err)
	}

	if len(r1) == 0 || len(r2) == 0 {
		t.Fatal("no return values")
	}

	if r1[0] != r2[0] {
		t.Errorf("DIVERGENCE: DJNZ=%d, While=%d", r1[0], r2[0])
	} else {
		t.Logf("DJNZ=%d == While=%d ✓ (sum 1..10)", r1[0], r2[0])
	}

	if r1[0] != 55 {
		t.Errorf("expected 55, got %d", r1[0])
	}
}

// ── Helpers ────────────────────────────────────────────────────────────────

func findAddPat(m *MachineDesc, width int) *Pattern {
	for i := range m.Patterns {
		p := &m.Patterns[i]
		if p.MIROp == OpAdd && (width == 0 || p.Width == 0 || p.Width == width) {
			return p
		}
	}
	return nil
}

func findSubPat(m *MachineDesc, width int) *Pattern {
	for i := range m.Patterns {
		p := &m.Patterns[i]
		if p.MIROp == OpSub && (width == 0 || p.Width == 0 || p.Width == width) {
			return p
		}
	}
	return nil
}

func findCmpPat(m *MachineDesc, width int) *Pattern {
	for i := range m.Patterns {
		p := &m.Patterns[i]
		if p.MIROp == OpCmp && (width == 0 || p.Width == 0 || p.Width == width) {
			return p
		}
	}
	return nil
}

func findConstPat(m *MachineDesc, width int) *Pattern {
	for i := range m.Patterns {
		p := &m.Patterns[i]
		if p.MIROp == OpConst && (width == 0 || p.Width == 0 || p.Width == width) {
			return p
		}
	}
	return nil
}

func pickFromSet(s LocSet, avoid int) int {
	for i := 0; i < MaxLocs; i++ {
		if s.Has(i) && i != avoid {
			return i
		}
	}
	return -1
}

func pickNFromSet(s LocSet, n int) []int {
	var result []int
	for i := 0; i < MaxLocs && len(result) < n; i++ {
		if s.Has(i) {
			result = append(result, i)
		}
	}
	return result
}

// makeSumProg_DJNZ builds: sum(1..N) using DJNZ loop
func makeSumProg_DJNZ(m *MachineDesc, n int) *Prog {
	constPat := findConstPat(m, 0)
	addPat := findAddPat(m, 0)
	regs := pickNFromSet(m.LocsOfWidth(m.WordSize), 2)
	sumR, cntR := regs[0], regs[1]

	return &Prog{
		Name: "sum_djnz",
		Desc: m,
		Blocks: []Block{
			{
				Label: "entry",
				Insts: []Inst{
					{Pat: constPat, Dst: Operand{Phys: sumR}, Imm: 0},
					{Pat: constPat, Dst: Operand{Phys: cntR}, Imm: int64(n)},
				},
				Term: Term{Kind: TermJump, Targets: []string{"loop"}},
			},
			{
				Label: "loop",
				Insts: []Inst{
					{Pat: addPat, Dst: Operand{Phys: sumR},
						Srcs: [2]Operand{{Phys: sumR}, {Phys: cntR}}},
				},
				Term: Term{
					Kind:    TermDJNZ,
					Counter: Operand{Phys: cntR},
					Targets: []string{"loop", "exit"},
				},
			},
			{
				Label: "exit",
				Term:  Term{Kind: TermReturn, RetVals: []Operand{{Phys: sumR}}},
			},
		},
	}
}

// makeSumProg_While builds: sum(1..N) using while(cnt!=0) loop
func makeSumProg_While(m *MachineDesc, n int) *Prog {
	constPat := findConstPat(m, 0)
	addPat := findAddPat(m, 0)
	subPat := findSubPat(m, 0)
	regs := pickNFromSet(m.LocsOfWidth(m.WordSize), 3)
	sumR, cntR, oneR := regs[0], regs[1], regs[2]

	return &Prog{
		Name: "sum_while",
		Desc: m,
		Blocks: []Block{
			{
				Label: "entry",
				Insts: []Inst{
					{Pat: constPat, Dst: Operand{Phys: sumR}, Imm: 0},
					{Pat: constPat, Dst: Operand{Phys: cntR}, Imm: int64(n)},
					{Pat: constPat, Dst: Operand{Phys: oneR}, Imm: 1},
				},
				Term: Term{Kind: TermJump, Targets: []string{"head"}},
			},
			{
				Label: "head",
				// Branch on counter value directly: cnt!=0 → body
				Term: Term{
					Kind:    TermBranch,
					Cond:    Operand{Phys: cntR},
					Targets: []string{"body", "exit"},
				},
			},
			{
				Label: "body",
				Insts: []Inst{
					{Pat: addPat, Dst: Operand{Phys: sumR},
						Srcs: [2]Operand{{Phys: sumR}, {Phys: cntR}}},
					{Pat: subPat, Dst: Operand{Phys: cntR},
						Srcs: [2]Operand{{Phys: cntR}, {Phys: oneR}}},
				},
				Term: Term{Kind: TermJump, Targets: []string{"head"}},
			},
			{
				Label: "exit",
				Term:  Term{Kind: TermReturn, RetVals: []Operand{{Phys: sumR}}},
			},
		},
	}
}
