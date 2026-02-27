package mirvm

import (
	"testing"

	"github.com/minz/minzc/pkg/ir"
)

// TestDJNZBasicLoop tests a simple DJNZ loop that counts iterations
func TestDJNZBasicLoop(t *testing.T) {
	module := &ir.Module{
		Functions: []*ir.Function{
			{
				Name: "test.main",
				Instructions: []ir.Instruction{
					{Op: ir.OpLoadConst, Dest: 1, Imm: 0},   // r1 = accumulator
					{Op: ir.OpLoadConst, Dest: 2, Imm: 5},   // r2 = counter
					{Op: ir.OpLabel, Label: "loop"},
					{Op: ir.OpLoadConst, Dest: 3, Imm: 1},
					{Op: ir.OpAdd, Dest: 1, Src1: 1, Src2: 3}, // r1 += 1
					{Op: ir.OpDJNZ, Src1: 2, Label: "loop"},   // r2--; if r2 != 0 goto loop
					{Op: ir.OpReturn},
				},
			},
		},
	}

	vm := New(Config{MemorySize: 4096, StackSize: 1024, MaxSteps: 1000})
	if err := vm.LoadModule(module); err != nil {
		t.Fatalf("LoadModule: %v", err)
	}
	if _, err := vm.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if vm.registers[1] != 5 {
		t.Errorf("accumulator r1 = %d, want 5", vm.registers[1])
	}
	if vm.registers[2] != 0 {
		t.Errorf("counter r2 = %d, want 0", vm.registers[2])
	}
}

// TestDJNZSingleIteration verifies DJNZ with count=1 executes exactly once
func TestDJNZSingleIteration(t *testing.T) {
	module := &ir.Module{
		Functions: []*ir.Function{
			{
				Name: "test.main",
				Instructions: []ir.Instruction{
					{Op: ir.OpLoadConst, Dest: 1, Imm: 0},   // accumulator
					{Op: ir.OpLoadConst, Dest: 2, Imm: 1},   // counter = 1
					{Op: ir.OpLabel, Label: "loop"},
					{Op: ir.OpLoadConst, Dest: 3, Imm: 10},
					{Op: ir.OpAdd, Dest: 1, Src1: 1, Src2: 3}, // r1 += 10
					{Op: ir.OpDJNZ, Src1: 2, Label: "loop"},
					{Op: ir.OpReturn},
				},
			},
		},
	}

	vm := New(Config{MemorySize: 4096, StackSize: 1024, MaxSteps: 100})
	if err := vm.LoadModule(module); err != nil {
		t.Fatalf("LoadModule: %v", err)
	}
	if _, err := vm.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if vm.registers[1] != 10 {
		t.Errorf("r1 = %d, want 10 (single iteration)", vm.registers[1])
	}
}

// TestDJNZWithStore tests DJNZ with memory store inside the body
func TestDJNZWithStore(t *testing.T) {
	module := &ir.Module{
		Functions: []*ir.Function{
			{
				Name: "test.main",
				Instructions: []ir.Instruction{
					{Op: ir.OpLoadConst, Dest: 1, Imm: 0},    // accumulator
					{Op: ir.OpLoadConst, Dest: 2, Imm: 3},    // counter = 3
					{Op: ir.OpLabel, Label: "loop"},
					{Op: ir.OpLoadConst, Dest: 3, Imm: 7},
					{Op: ir.OpAdd, Dest: 1, Src1: 1, Src2: 3}, // r1 += 7
					// Store intermediate result (simulating a side effect)
					{Op: ir.OpStoreMem, Src1: 1, Imm: 0x100},  // mem[0x100] = r1
					{Op: ir.OpDJNZ, Src1: 2, Label: "loop"},
					{Op: ir.OpReturn},
				},
			},
		},
	}

	vm := New(Config{MemorySize: 4096, StackSize: 1024, MaxSteps: 1000})
	if err := vm.LoadModule(module); err != nil {
		t.Fatalf("LoadModule: %v", err)
	}
	if _, err := vm.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if vm.registers[1] != 21 { // 7 * 3 = 21
		t.Errorf("r1 = %d, want 21", vm.registers[1])
	}
	if vm.registers[2] != 0 {
		t.Errorf("counter r2 = %d, want 0", vm.registers[2])
	}
}

// TestDJNZNestedLoops tests nested DJNZ loops
func TestDJNZNestedLoops(t *testing.T) {
	module := &ir.Module{
		Functions: []*ir.Function{
			{
				Name: "test.main",
				Instructions: []ir.Instruction{
					{Op: ir.OpLoadConst, Dest: 1, Imm: 0},   // accumulator
					{Op: ir.OpLoadConst, Dest: 2, Imm: 3},   // outer counter
					{Op: ir.OpLabel, Label: "outer"},
					// Inner loop: count 4 times
					{Op: ir.OpLoadConst, Dest: 3, Imm: 4},   // inner counter
					{Op: ir.OpLabel, Label: "inner"},
					{Op: ir.OpLoadConst, Dest: 4, Imm: 1},
					{Op: ir.OpAdd, Dest: 1, Src1: 1, Src2: 4}, // r1 += 1
					{Op: ir.OpDJNZ, Src1: 3, Label: "inner"},  // inner--
					// End inner loop
					{Op: ir.OpDJNZ, Src1: 2, Label: "outer"},  // outer--
					{Op: ir.OpReturn},
				},
			},
		},
	}

	vm := New(Config{MemorySize: 4096, StackSize: 1024, MaxSteps: 10000})
	if err := vm.LoadModule(module); err != nil {
		t.Fatalf("LoadModule: %v", err)
	}
	if _, err := vm.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// 3 outer * 4 inner = 12 total increments
	if vm.registers[1] != 12 {
		t.Errorf("r1 = %d, want 12 (3 outer * 4 inner)", vm.registers[1])
	}
}

// TestDJNZWithConditionalSkip tests DJNZ with filter-like conditional skip
func TestDJNZWithConditionalSkip(t *testing.T) {
	// Simulate: for i in [1,2,3,4,5] { if i > 3 { acc += i } }
	// Counter counts down from 5, we load from "array" positions
	module := &ir.Module{
		Functions: []*ir.Function{
			{
				Name: "test.main",
				Instructions: []ir.Instruction{
					{Op: ir.OpLoadConst, Dest: 1, Imm: 0},   // accumulator
					{Op: ir.OpLoadConst, Dest: 2, Imm: 5},   // counter
					{Op: ir.OpLoadConst, Dest: 5, Imm: 0},   // index
					{Op: ir.OpLabel, Label: "loop"},
					// Simulate loading element: r3 = index + 1
					{Op: ir.OpLoadConst, Dest: 6, Imm: 1},
					{Op: ir.OpAdd, Dest: 3, Src1: 5, Src2: 6}, // r3 = index + 1
					// Filter: if r3 <= 3 skip
					{Op: ir.OpLoadConst, Dest: 7, Imm: 3},
					{Op: ir.OpGt, Dest: 4, Src1: 3, Src2: 7},  // r4 = r3 > 3
					{Op: ir.OpJumpIfNot, Src1: 4, Label: "skip"},
					// Passed filter: accumulate
					{Op: ir.OpAdd, Dest: 1, Src1: 1, Src2: 3}, // acc += element
					{Op: ir.OpLabel, Label: "skip"},
					// Increment index
					{Op: ir.OpAdd, Dest: 5, Src1: 5, Src2: 6}, // index++
					{Op: ir.OpDJNZ, Src1: 2, Label: "loop"},
					{Op: ir.OpReturn},
				},
			},
		},
	}

	vm := New(Config{MemorySize: 4096, StackSize: 1024, MaxSteps: 10000})
	if err := vm.LoadModule(module); err != nil {
		t.Fatalf("LoadModule: %v", err)
	}
	if _, err := vm.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Elements > 3: 4 + 5 = 9
	if vm.registers[1] != 9 {
		t.Errorf("r1 = %d, want 9 (4 + 5, elements > 3)", vm.registers[1])
	}
}

// TestDJNZWithEarlyExit tests DJNZ loop with early exit (take N)
func TestDJNZWithEarlyExit(t *testing.T) {
	// Simulate: take first 2 from [1,2,3,4,5]
	module := &ir.Module{
		Functions: []*ir.Function{
			{
				Name: "test.main",
				Instructions: []ir.Instruction{
					{Op: ir.OpLoadConst, Dest: 1, Imm: 0},   // accumulator
					{Op: ir.OpLoadConst, Dest: 2, Imm: 5},   // loop counter
					{Op: ir.OpLoadConst, Dest: 5, Imm: 0},   // taken count
					{Op: ir.OpLoadConst, Dest: 6, Imm: 0},   // element index
					{Op: ir.OpLabel, Label: "loop"},
					// Element = index + 1
					{Op: ir.OpLoadConst, Dest: 7, Imm: 1},
					{Op: ir.OpAdd, Dest: 3, Src1: 6, Src2: 7}, // element
					// Check taken count >= 2 → exit
					{Op: ir.OpLoadConst, Dest: 8, Imm: 2},
					{Op: ir.OpGe, Dest: 4, Src1: 5, Src2: 8},
					{Op: ir.OpJumpIf, Src1: 4, Label: "done"},
					// Accumulate
					{Op: ir.OpAdd, Dest: 1, Src1: 1, Src2: 3},
					// taken++
					{Op: ir.OpAdd, Dest: 5, Src1: 5, Src2: 7},
					// index++
					{Op: ir.OpAdd, Dest: 6, Src1: 6, Src2: 7},
					{Op: ir.OpDJNZ, Src1: 2, Label: "loop"},
					{Op: ir.OpLabel, Label: "done"},
					{Op: ir.OpReturn},
				},
			},
		},
	}

	vm := New(Config{MemorySize: 4096, StackSize: 1024, MaxSteps: 10000})
	if err := vm.LoadModule(module); err != nil {
		t.Fatalf("LoadModule: %v", err)
	}
	if _, err := vm.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Take first 2: 1 + 2 = 3
	if vm.registers[1] != 3 {
		t.Errorf("r1 = %d, want 3 (take first 2: 1+2)", vm.registers[1])
	}
}

// TestDJNZCounterReachesZero verifies counter is exactly 0 after loop
func TestDJNZCounterReachesZero(t *testing.T) {
	for _, count := range []int64{1, 2, 5, 10, 255} {
		module := &ir.Module{
			Functions: []*ir.Function{
				{
					Name: "test.main",
					Instructions: []ir.Instruction{
						{Op: ir.OpLoadConst, Dest: 1, Imm: 0},
						{Op: ir.OpLoadConst, Dest: 2, Imm: count},
						{Op: ir.OpLabel, Label: "loop"},
						{Op: ir.OpLoadConst, Dest: 3, Imm: 1},
						{Op: ir.OpAdd, Dest: 1, Src1: 1, Src2: 3},
						{Op: ir.OpDJNZ, Src1: 2, Label: "loop"},
						{Op: ir.OpReturn},
					},
				},
			},
		}

		vm := New(Config{MemorySize: 4096, StackSize: 1024, MaxSteps: 100000})
		if err := vm.LoadModule(module); err != nil {
			t.Fatalf("count=%d LoadModule: %v", count, err)
		}
		if _, err := vm.Run(); err != nil {
			t.Fatalf("count=%d Run: %v", count, err)
		}

		if vm.registers[1] != count {
			t.Errorf("count=%d: accumulator = %d, want %d", count, vm.registers[1], count)
		}
		if vm.registers[2] != 0 {
			t.Errorf("count=%d: counter = %d, want 0", count, vm.registers[2])
		}
	}
}
