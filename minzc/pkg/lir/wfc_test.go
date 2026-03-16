package lir

import "testing"

// TestWFC_Propagation verifies that forward+backward propagation
// narrows LocSets correctly on CISC (asymmetric) machine.
func TestWFC_Propagation(t *testing.T) {
	m := CISC

	// MIR: r0 = 10; r1 = 32; r2 = r0 + r1
	ops := []MIROp{
		{Op: OpConst, Dst: 0, Src: [2]int{-1, -1}, Imm: 10, Width: 8},
		{Op: OpConst, Dst: 1, Src: [2]int{-1, -1}, Imm: 32, Width: 8},
		{Op: OpAdd, Dst: 2, Src: [2]int{0, 1}, Width: 8},
	}

	sel, err := SelectInstructions(m, ops)
	if err != nil {
		t.Fatal(err)
	}

	wfc := NewWFCState(m, sel.Insts)

	// Before propagation: ADD's src0 allows {A,B,C,D,E} (from pattern union).
	addCell := &wfc.Cells[len(wfc.Cells)-1]
	t.Logf("Before propagation: ADD dst=%064b src0=%064b src1=%064b",
		addCell.DstLocs, addCell.SrcLocs[0], addCell.SrcLocs[1])

	iters := wfc.Propagate()
	t.Logf("Propagation converged in %d iterations", iters)

	// After propagation: ADD's dst should be narrowed to {A} (acc-only).
	t.Logf("After propagation: ADD dst=%064b src0=%064b src1=%064b",
		addCell.DstLocs, addCell.SrcLocs[0], addCell.SrcLocs[1])

	// ADD dst must include A (bit 0 in CISC).
	aIdx := m.LocByName("A")
	if !addCell.DstLocs.Has(aIdx) {
		t.Errorf("ADD dst should include A (bit %d), got %064b", aIdx, addCell.DstLocs)
	}
}

// TestWFC_CollapseConvergence runs WFC collapse on all 4 machines.
func TestWFC_CollapseConvergence(t *testing.T) {
	machines := []*MachineDesc{RISC32, RISC8, CISC, MICRO}

	// r0 = 10; r1 = 32; r2 = r0 + r1
	ops := []MIROp{
		{Op: OpConst, Dst: 0, Src: [2]int{-1, -1}, Imm: 10, Width: 8},
		{Op: OpConst, Dst: 1, Src: [2]int{-1, -1}, Imm: 32, Width: 8},
		{Op: OpAdd, Dst: 2, Src: [2]int{0, 1}, Width: 8},
	}

	for _, m := range machines {
		sel, err := SelectInstructions(m, ops)
		if err != nil {
			t.Fatalf("%s isel: %v", m.Name, err)
		}

		wfc := NewWFCState(m, sel.Insts)
		iters := wfc.Propagate()
		if err := wfc.Collapse(); err != nil {
			t.Fatalf("%s collapse: %v", m.Name, err)
		}

		// Execute on VM.
		insts := wfc.ToInsts()
		vm := NewVM(m)
		for i := range insts {
			vm.ExecInst(&insts[i])
		}

		// Find result.
		lastInst := insts[len(insts)-1]
		result := vm.Get(lastInst.Dst.Phys)

		t.Logf("%s: WFC(%d iters) → r2@%s = %d",
			m.Name, iters, m.Locs[lastInst.Dst.Phys].Name, result)

		if result != 42 {
			t.Errorf("%s: expected 42, got %d", m.Name, result)
		}
	}
}

// TestWFC_BackwardPropagation verifies that backward constraints
// influence earlier instructions' allocation.
func TestWFC_BackwardPropagation(t *testing.T) {
	m := CISC

	// r0 = 10; r1 = 32; r2 = r0 + r1; r3 = r2 - r1
	// The SUB needs src0 in A. If ADD already put r2 in A,
	// backward propagation should notice and keep r2 in A.
	ops := []MIROp{
		{Op: OpConst, Dst: 0, Src: [2]int{-1, -1}, Imm: 10, Width: 8},
		{Op: OpConst, Dst: 1, Src: [2]int{-1, -1}, Imm: 32, Width: 8},
		{Op: OpAdd, Dst: 2, Src: [2]int{0, 1}, Width: 8},
		{Op: OpSub, Dst: 3, Src: [2]int{2, 1}, Width: 8},
	}

	sel, err := SelectInstructions(m, ops)
	if err != nil {
		t.Fatal(err)
	}

	wfc := NewWFCState(m, sel.Insts)
	iters := wfc.Propagate()
	if err := wfc.Collapse(); err != nil {
		t.Fatalf("collapse: %v", err)
	}

	insts := wfc.ToInsts()
	vm := NewVM(m)
	for i := range insts {
		vm.ExecInst(&insts[i])
	}

	// r3 = (10+32) - 32 = 10
	lastInst := insts[len(insts)-1]
	result := vm.Get(lastInst.Dst.Phys)
	t.Logf("CISC: WFC(%d iters) → (10+32)-32 = %d @%s",
		iters, result, m.Locs[lastInst.Dst.Phys].Name)

	if result != 10 {
		t.Errorf("expected 10, got %d", result)
	}
}
