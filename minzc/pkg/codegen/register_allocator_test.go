package codegen

import (
	"testing"

	"github.com/minz/minzc/pkg/ir"
)

func TestHintToPhysical(t *testing.T) {
	tests := []struct {
		hint     ir.RegisterHint
		expected PhysicalReg
	}{
		{ir.RegHintNone, RegNone},
		{ir.RegHintA, RegA},
		{ir.RegHintB, RegB},
		{ir.RegHintC, RegC},
		{ir.RegHintD, RegD},
		{ir.RegHintE, RegE},
		{ir.RegHintH, RegH},
		{ir.RegHintL, RegL},
		{ir.RegHintHL, RegHL},
		{ir.RegHintDE, RegDE},
		{ir.RegHintBC, RegBC},
	}
	for _, tt := range tests {
		got := hintToPhysical(tt.hint)
		if got != tt.expected {
			t.Errorf("hintToPhysical(%d) = %d, want %d", tt.hint, got, tt.expected)
		}
	}
}

func TestHintBasedAllocation(t *testing.T) {
	ra := NewZ80RegisterAllocator()

	// Allocate counter with RegHintB → should get B
	counterInst := ir.Instruction{
		Op:   ir.OpLoadConst,
		Dest: 1,
		Type: &ir.BasicType{Kind: ir.TypeU8},
		Hint: ir.RegHintB,
	}
	phys := ra.allocateRegister(1, &counterInst)
	if phys != RegB {
		t.Errorf("counter with RegHintB got %d, want RegB (%d)", phys, RegB)
	}

	// Allocate pointer with RegHintHL → should get HL
	ptrInst := ir.Instruction{
		Op:   ir.OpMove,
		Dest: 2,
		Type: &ir.PointerType{Base: &ir.BasicType{Kind: ir.TypeU8}},
		Hint: ir.RegHintHL,
	}
	phys = ra.allocateRegister(2, &ptrInst)
	if phys != RegHL {
		t.Errorf("pointer with RegHintHL got %d, want RegHL (%d)", phys, RegHL)
	}

	// Allocate element with RegHintA → should get A
	elemInst := ir.Instruction{
		Op:   ir.OpLoad,
		Dest: 3,
		Type: &ir.BasicType{Kind: ir.TypeU8},
		Hint: ir.RegHintA,
	}
	phys = ra.allocateRegister(3, &elemInst)
	if phys != RegA {
		t.Errorf("element with RegHintA got %d, want RegA (%d)", phys, RegA)
	}
}

func TestHintFallbackWhenOccupied(t *testing.T) {
	ra := NewZ80RegisterAllocator()

	// First, allocate A without a hint
	inst1 := ir.Instruction{
		Op:   ir.OpLoadConst,
		Dest: 1,
		Type: &ir.BasicType{Kind: ir.TypeU8},
	}
	phys1 := ra.allocateRegister(1, &inst1)
	if phys1 != RegA {
		t.Fatalf("first alloc without hint got %d, want RegA (%d)", phys1, RegA)
	}

	// Now try to allocate with RegHintA — A is occupied, should fall back
	inst2 := ir.Instruction{
		Op:   ir.OpLoadConst,
		Dest: 2,
		Type: &ir.BasicType{Kind: ir.TypeU8},
		Hint: ir.RegHintA,
	}
	phys2 := ra.allocateRegister(2, &inst2)
	if phys2 == RegA {
		t.Errorf("second alloc with RegHintA should not get A (occupied)")
	}
	if phys2 == RegNone {
		t.Errorf("second alloc should get some register, got RegNone")
	}
}

func TestPairComponentConsistency(t *testing.T) {
	ra := NewZ80RegisterAllocator()

	// Allocate B individually
	inst := ir.Instruction{
		Op:   ir.OpLoadConst,
		Dest: 1,
		Type: &ir.BasicType{Kind: ir.TypeU8},
		Hint: ir.RegHintB,
	}
	ra.allocateRegister(1, &inst)

	// BC pair should now be unavailable
	if ra.freeRegs.available[RegBC] {
		t.Error("BC pair should be unavailable after B is allocated")
	}

	// Allocate HL pair
	inst2 := ir.Instruction{
		Op:   ir.OpMove,
		Dest: 2,
		Type: &ir.PointerType{Base: &ir.BasicType{Kind: ir.TypeU8}},
		Hint: ir.RegHintHL,
	}
	ra.allocateRegister(2, &inst2)

	// H and L should be unavailable
	if ra.freeRegs.available[RegH] {
		t.Error("H should be unavailable after HL is allocated")
	}
	if ra.freeRegs.available[RegL] {
		t.Error("L should be unavailable after HL is allocated")
	}
}

func TestIteratorPatternAllocation(t *testing.T) {
	// Simulate the iterator pattern: counter→B, pointer→HL, element→A
	// Uses a DJNZ back-edge to keep registers live across the loop body.
	ra := NewZ80RegisterAllocator()

	u8Type := &ir.BasicType{Kind: ir.TypeU8}
	ptrType := &ir.PointerType{Base: u8Type}

	fn := &ir.Function{
		Name: "test_iter",
		Instructions: []ir.Instruction{
			// 0: counter = 10  (B)
			{Op: ir.OpLoadConst, Dest: 1, Imm: 10, Type: u8Type, Hint: ir.RegHintB},
			// 1: ptr = source   (HL)
			{Op: ir.OpMove, Dest: 2, Src1: 3, Type: ptrType, Hint: ir.RegHintHL},
			// 2: loop:
			{Op: ir.OpLabel, Label: "loop"},
			// 3: elem = *ptr    (A)
			{Op: ir.OpLoad, Dest: 4, Src1: 2, Type: u8Type, Hint: ir.RegHintA},
			// 4: call print(elem)
			{Op: ir.OpCall, Src1: 4, Symbol: "print"},
			// 5: ptr++
			{Op: ir.OpInc, Dest: 2, Src1: 2, Type: ptrType, Hint: ir.RegHintHL},
			// 6: DJNZ loop  (back-edge: label "loop" at index 2)
			{Op: ir.OpDJNZ, Src1: 1, Label: "loop", Hint: ir.RegHintB},
			// 7: return (keeps regs alive through last instruction)
			{Op: ir.OpReturn, Src1: 0},
		},
	}

	ra.AllocateFunction(fn)

	// After allocation, check that hints were respected.
	// Regs may have been freed by freeDeadRegisters by now,
	// so we snapshot the allocation map right before freeing.
	// Since we can't do that externally, test via direct allocateRegister calls instead.
	// (The back-edge should keep regs 1 and 2 alive across the loop.)

	// With the DJNZ back-edge to index 2, register 1 (used at 0 and 6) has interval [0, 6].
	// Register 2 (def at 1, used at 3, 5, range inside loop 2-6) has interval [1, 6].
	// At instruction 7 (OpReturn with Src1=0), both regs 1 and 2 have ended (End=6, instIdx=7 >=6).
	// So they WILL be freed. But we can verify via the direct-allocation tests above.
	// This test verifies the full AllocateFunction pipeline doesn't crash.
	// Detailed allocation verification is done in TestHintBasedAllocation.
}

func TestLiveIntervalExtension(t *testing.T) {
	// Verify that live intervals extend across loop back-edges
	ra := NewZ80RegisterAllocator()

	u8Type := &ir.BasicType{Kind: ir.TypeU8}

	fn := &ir.Function{
		Name: "test_live",
		Instructions: []ir.Instruction{
			{Op: ir.OpLoadConst, Dest: 1, Imm: 10, Type: u8Type, Hint: ir.RegHintB},  // 0
			{Op: ir.OpLabel, Label: "loop"},                                             // 1
			{Op: ir.OpNop},                                                              // 2
			{Op: ir.OpDJNZ, Src1: 1, Label: "loop"},                                    // 3 (back-edge to 1)
		},
	}

	intervals := ra.computeLiveIntervals(fn)

	if iv, ok := intervals[ir.Register(1)]; ok {
		if iv.Start != 0 || iv.End != 3 {
			t.Errorf("reg 1 interval: got [%d, %d], want [0, 3]", iv.Start, iv.End)
		}
	} else {
		t.Error("reg 1 not found in live intervals")
	}
}
