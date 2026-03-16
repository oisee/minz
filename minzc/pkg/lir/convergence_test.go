package lir

import (
	"testing"
)

// TestConvergence_AddConst verifies that the same LIR program produces
// identical results on all machine descriptors.
func TestConvergence_AddConst(t *testing.T) {
	machines := []*MachineDesc{RISC32, RISC8}

	for _, m := range machines {
		vm := NewVM(m)

		// Program: r0 = 10; r1 = 32; r2 = r0 + r1; return r2
		constPat := findPat(m, OpConst)
		addPat := findPat(m, OpAdd)
		if constPat == nil || addPat == nil {
			t.Fatalf("%s: missing patterns", m.Name)
		}

		prog := []Inst{
			{Pat: constPat, Dst: op(0), Imm: 10},
			{Pat: constPat, Dst: op(1), Imm: 32},
			{Pat: addPat, Dst: op(2), Srcs: [2]Operand{op(0), op(1)}},
		}

		for _, inst := range prog {
			vm.ExecInst(&inst)
		}

		result := vm.Get(2)
		if result != 42 {
			t.Errorf("%s: r2 = %d, want 42", m.Name, result)
		} else {
			t.Logf("%s: r2 = %d ✓", m.Name, result)
		}
	}
}

// TestConvergence_SubAndCarry tests subtraction with borrow across machines.
func TestConvergence_SubAndCarry(t *testing.T) {
	machines := []*MachineDesc{RISC32, RISC8}

	for _, m := range machines {
		vm := NewVM(m)

		constPat := findPat(m, OpConst)
		subPat := findPat(m, OpSub)
		if constPat == nil || subPat == nil {
			t.Fatalf("%s: missing patterns", m.Name)
		}

		// r0 = 5; r1 = 10; r2 = r0 - r1 (borrow)
		prog := []Inst{
			{Pat: constPat, Dst: op(0), Imm: 5},
			{Pat: constPat, Dst: op(1), Imm: 10},
			{Pat: subPat, Dst: op(2), Srcs: [2]Operand{op(0), op(1)}},
		}

		for _, inst := range prog {
			vm.ExecInst(&inst)
		}

		result := vm.Get(2)
		// 5 - 10 = -5 = 65531 (u16) or 251 (u8)
		w := uint64(m.Locs[2].Width)
		mask := uint64((1 << w) - 1)
		expected := uint64((int64(5) - int64(10) + int64(1<<w))) & mask

		if result != expected {
			t.Errorf("%s: r2 = %d, want %d (width=%d)", m.Name, result, expected, m.Locs[2].Width)
		} else {
			t.Logf("%s: r2 = %d (%d-bit) ✓", m.Name, result, m.Locs[2].Width)
		}

		// Carry flag should be set (borrow)
		if vm.Flags&1 == 0 {
			t.Errorf("%s: carry not set after 5-10", m.Name)
		}
	}
}

// TestConvergence_LoadStore tests memory operations across machines.
func TestConvergence_LoadStore(t *testing.T) {
	machines := []*MachineDesc{RISC32, RISC8}

	for _, m := range machines {
		vm := NewVM(m)

		constPat := findPat(m, OpConst)
		storePat := findPat(m, OpStore)
		loadPat := findPat(m, OpLoad)
		if constPat == nil || storePat == nil || loadPat == nil {
			t.Fatalf("%s: missing patterns", m.Name)
		}

		// r0 = 0x1000 (address); r1 = 42; store [r0], r1; r2 = load [r0]
		prog := []Inst{
			{Pat: constPat, Dst: op(0), Imm: 0x1000},
			{Pat: constPat, Dst: op(1), Imm: 42},
			{Pat: storePat, Srcs: [2]Operand{op(0), op(1)}},
			{Pat: loadPat, Dst: op(2), Srcs: [2]Operand{op(0)}},
		}

		for _, inst := range prog {
			vm.ExecInst(&inst)
		}

		result := vm.Get(2)
		if result != 42 {
			t.Errorf("%s: load result = %d, want 42", m.Name, result)
		} else {
			t.Logf("%s: store/load = %d ✓", m.Name, result)
		}
	}
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func findPat(m *MachineDesc, mirOp int) *Pattern {
	for i := range m.Patterns {
		if m.Patterns[i].MIROp == mirOp {
			return &m.Patterns[i]
		}
	}
	return nil
}

func op(phys int) Operand {
	return Operand{VReg: phys, Phys: phys}
}
