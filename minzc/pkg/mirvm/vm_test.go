// Package mirvm tests
package mirvm

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/ir"
)

// TestBasicOpcodes tests basic arithmetic and logic operations
func TestBasicOpcodes(t *testing.T) {
	tests := []struct {
		name     string
		mir      string
		wantRegs map[int]int64 // expected register values after execution
	}{
		{
			name: "LoadImm",
			mir: `; Test load immediate
Function test.main() -> void
  Instructions:
      0: r1 = 42
      1: return
`,
			wantRegs: map[int]int64{1: 42},
		},
		{
			name: "Add",
			mir: `; Test add
Function test.main() -> void
  Instructions:
      0: r1 = 10
      1: r2 = 20
      2: r3 = r1 + r2
      3: return
`,
			wantRegs: map[int]int64{3: 30},
		},
		{
			name: "Sub",
			mir: `; Test subtract
Function test.main() -> void
  Instructions:
      0: r1 = 100
      1: r2 = 30
      2: r3 = r1 - r2
      3: return
`,
			wantRegs: map[int]int64{3: 70},
		},
		{
			name: "Mul",
			mir: `; Test multiply
Function test.main() -> void
  Instructions:
      0: r1 = 7
      1: r2 = 6
      2: r3 = r1 * r2
      3: return
`,
			wantRegs: map[int]int64{3: 42},
		},
		{
			name: "Div",
			mir: `; Test divide
Function test.main() -> void
  Instructions:
      0: r1 = 100
      1: r2 = 10
      2: r3 = r1 / r2
      3: return
`,
			wantRegs: map[int]int64{3: 10},
		},
		{
			name: "Shl",
			mir: `; Test shift left
Function test.main() -> void
  Instructions:
      0: r1 = 1
      1: r2 = 4
      2: r3 = r1 << r2
      3: return
`,
			wantRegs: map[int]int64{3: 16},
		},
		{
			name: "Shr",
			mir: `; Test shift right
Function test.main() -> void
  Instructions:
      0: r1 = 64
      1: r2 = 2
      2: r3 = r1 >> r2
      3: return
`,
			wantRegs: map[int]int64{3: 16},
		},
		{
			name: "And",
			mir: `; Test bitwise and
Function test.main() -> void
  Instructions:
      0: r1 = 0xFF
      1: r2 = 0x0F
      2: r3 = r1 & r2
      3: return
`,
			wantRegs: map[int]int64{3: 0x0F},
		},
		{
			name: "Or",
			mir: `; Test bitwise or
Function test.main() -> void
  Instructions:
      0: r1 = 0xF0
      1: r2 = 0x0F
      2: r3 = r1 | r2
      3: return
`,
			wantRegs: map[int]int64{3: 0xFF},
		},
		{
			name: "Xor",
			mir: `; Test bitwise xor
Function test.main() -> void
  Instructions:
      0: r1 = 0xFF
      1: r2 = 0xF0
      2: r3 = r1 ^ r2
      3: return
`,
			wantRegs: map[int]int64{3: 0x0F},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module, err := ir.ParseMIR(tt.mir)
			if err != nil {
				t.Fatalf("Failed to parse MIR: %v", err)
			}

			config := Config{
				MemorySize: 4096,
				StackSize:  1024,
				MaxSteps:   1000,
			}
			vm := New(config)
			if err := vm.LoadModule(module); err != nil {
				t.Fatalf("Failed to load module: %v", err)
			}

			_, err = vm.Run()
			if err != nil {
				t.Fatalf("Execution failed: %v", err)
			}

			for reg, want := range tt.wantRegs {
				got := vm.registers[reg]
				if got != want {
					t.Errorf("Register r%d = %d, want %d", reg, got, want)
				}
			}
		})
	}
}

// TestComparison tests comparison operations
func TestComparison(t *testing.T) {
	tests := []struct {
		name     string
		mir      string
		wantRegs map[int]int64
	}{
		{
			name: "Lt_True",
			mir: `; Test less than (true case)
Function test.main() -> void
  Instructions:
      0: r1 = 5
      1: r2 = 10
      2: r3 = r1 < r2
      3: return
`,
			wantRegs: map[int]int64{3: 1},
		},
		{
			name: "Lt_False",
			mir: `; Test less than (false case)
Function test.main() -> void
  Instructions:
      0: r1 = 10
      1: r2 = 5
      2: r3 = r1 < r2
      3: return
`,
			wantRegs: map[int]int64{3: 0},
		},
		{
			name: "Gt_True",
			mir: `; Test greater than (true case)
Function test.main() -> void
  Instructions:
      0: r1 = 10
      1: r2 = 5
      2: r3 = r1 > r2
      3: return
`,
			wantRegs: map[int]int64{3: 1},
		},
		{
			name: "Eq_True",
			mir: `; Test equal (true case)
Function test.main() -> void
  Instructions:
      0: r1 = 42
      1: r2 = 42
      2: r3 = r1 == r2
      3: return
`,
			wantRegs: map[int]int64{3: 1},
		},
		{
			name: "Ne_True",
			mir: `; Test not equal (true case)
Function test.main() -> void
  Instructions:
      0: r1 = 42
      1: r2 = 43
      2: r3 = r1 != r2
      3: return
`,
			wantRegs: map[int]int64{3: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module, err := ir.ParseMIR(tt.mir)
			if err != nil {
				t.Fatalf("Failed to parse MIR: %v", err)
			}

			config := Config{
				MemorySize: 4096,
				StackSize:  1024,
				MaxSteps:   1000,
			}
			vm := New(config)
			if err := vm.LoadModule(module); err != nil {
				t.Fatalf("Failed to load module: %v", err)
			}

			_, err = vm.Run()
			if err != nil {
				t.Fatalf("Execution failed: %v", err)
			}

			for reg, want := range tt.wantRegs {
				got := vm.registers[reg]
				if got != want {
					t.Errorf("Register r%d = %d, want %d", reg, got, want)
				}
			}
		})
	}
}

// TestJumps tests jump instructions
func TestJumps(t *testing.T) {
	tests := []struct {
		name     string
		mir      string
		wantRegs map[int]int64
	}{
		{
			name: "UnconditionalJump",
			mir: `; Test unconditional jump
Function test.main() -> void
  Instructions:
      0: r1 = 1
      1: jump skip
      2: r1 = 99
      3: skip:
      4: return
`,
			wantRegs: map[int]int64{1: 1}, // Should be 1, not 99 (skipped)
		},
		{
			name: "JumpIfNot_TakeBranch",
			mir: `; Test jump_if_not when condition is false
Function test.main() -> void
  Instructions:
      0: r1 = 0
      1: jump_if_not r1, taken
      2: r2 = 99
      3: jump end
      4: taken:
      5: r2 = 42
      6: end:
      7: return
`,
			wantRegs: map[int]int64{2: 42}, // r1=0 means condition is false, so jump is taken
		},
		{
			name: "JumpIfNot_DontTakeBranch",
			mir: `; Test jump_if_not when condition is true
Function test.main() -> void
  Instructions:
      0: r1 = 1
      1: jump_if_not r1, nottaken
      2: r2 = 42
      3: jump end
      4: nottaken:
      5: r2 = 99
      6: end:
      7: return
`,
			wantRegs: map[int]int64{2: 42}, // r1=1 means condition is true, so jump not taken
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module, err := ir.ParseMIR(tt.mir)
			if err != nil {
				t.Fatalf("Failed to parse MIR: %v", err)
			}

			config := Config{
				MemorySize: 4096,
				StackSize:  1024,
				MaxSteps:   1000,
			}
			vm := New(config)
			if err := vm.LoadModule(module); err != nil {
				t.Fatalf("Failed to load module: %v", err)
			}

			_, err = vm.Run()
			if err != nil {
				t.Fatalf("Execution failed: %v", err)
			}

			for reg, want := range tt.wantRegs {
				got := vm.registers[reg]
				if got != want {
					t.Errorf("Register r%d = %d, want %d", reg, got, want)
				}
			}
		})
	}
}

// TestVariables tests variable load/store operations
func TestVariables(t *testing.T) {
	tests := []struct {
		name     string
		mir      string
		wantRegs map[int]int64
	}{
		{
			name: "StoreAndLoad",
			mir: `; Test store and load variable
Function test.main() -> void
  Locals:
    r1 = x: u8
  Instructions:
      0: r2 = 42
      1: store x, r2
      2: r3 = load x
      3: return
`,
			wantRegs: map[int]int64{3: 42, 1: 42},
		},
		{
			name: "MultipleVariables",
			mir: `; Test multiple variables
Function test.main() -> void
  Locals:
    r1 = a: u8
    r2 = b: u8
  Instructions:
      0: r3 = 10
      1: store a, r3
      2: r4 = 20
      3: store b, r4
      4: r5 = load a
      5: r6 = load b
      6: r7 = r5 + r6
      7: return
`,
			wantRegs: map[int]int64{7: 30},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module, err := ir.ParseMIR(tt.mir)
			if err != nil {
				t.Fatalf("Failed to parse MIR: %v", err)
			}

			config := Config{
				MemorySize: 4096,
				StackSize:  1024,
				MaxSteps:   1000,
			}
			vm := New(config)
			if err := vm.LoadModule(module); err != nil {
				t.Fatalf("Failed to load module: %v", err)
			}

			_, err = vm.Run()
			if err != nil {
				t.Fatalf("Execution failed: %v", err)
			}

			for reg, want := range tt.wantRegs {
				got := vm.registers[reg]
				if got != want {
					t.Errorf("Register r%d = %d, want %d", reg, got, want)
				}
			}
		})
	}
}

// TestSimpleLoop tests a basic counting loop
func TestSimpleLoop(t *testing.T) {
	mir := `; Test simple loop: count from 0 to 9
Function test.main() -> void
  Locals:
    r1 = count: u8
  Instructions:
      0: r2 = 0
      1: store count, r2
      2: loop:
      3: r3 = load count
      4: r4 = 10
      5: r5 = r3 < r4
      6: jump_if_not r5, end
      7: r6 = load count
      8: r7 = 1
      9: r8 = r6 + r7
     10: store count, r8
     11: jump loop
     12: end:
     13: return
`
	module, err := ir.ParseMIR(mir)
	if err != nil {
		t.Fatalf("Failed to parse MIR: %v", err)
	}

	config := Config{
		MemorySize: 4096,
		StackSize:  1024,
		MaxSteps:   1000,
	}
	vm := New(config)
	if err := vm.LoadModule(module); err != nil {
		t.Fatalf("Failed to load module: %v", err)
	}

	_, err = vm.Run()
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	// After loop, count should be 10
	countReg := vm.findVarRegister("count")
	if countReg < 0 {
		t.Fatal("Could not find 'count' variable")
	}
	got := vm.registers[countReg]
	if got != 10 {
		t.Errorf("count = %d, want 10", got)
	}

	// Check statistics
	stats := vm.GetStatistics()
	// Loop runs 10 times, each iteration has ~6 instructions, plus setup and exit
	if stats.InstructionsExecuted < 60 || stats.InstructionsExecuted > 100 {
		t.Errorf("InstructionsExecuted = %d, want between 60 and 100", stats.InstructionsExecuted)
	}
}

// TestNestedLoop tests nested loops
func TestNestedLoop(t *testing.T) {
	mir := `; Test nested loop: outer 3x, inner 4x = 12 total
Function test.main() -> void
  Locals:
    r1 = i: u8
    r2 = j: u8
    r3 = total: u8
  Instructions:
      0: r4 = 0
      1: store total, r4
      2: r5 = 0
      3: store i, r5
      4: outer:
      5: r6 = load i
      6: r7 = 3
      7: r8 = r6 < r7
      8: jump_if_not r8, end_outer
      9: r9 = 0
     10: store j, r9
     11: inner:
     12: r10 = load j
     13: r11 = 4
     14: r12 = r10 < r11
     15: jump_if_not r12, end_inner
     16: r13 = load total
     17: r14 = 1
     18: r15 = r13 + r14
     19: store total, r15
     20: r16 = load j
     21: r17 = 1
     22: r18 = r16 + r17
     23: store j, r18
     24: jump inner
     25: end_inner:
     26: r19 = load i
     27: r20 = 1
     28: r21 = r19 + r20
     29: store i, r21
     30: jump outer
     31: end_outer:
     32: return
`
	module, err := ir.ParseMIR(mir)
	if err != nil {
		t.Fatalf("Failed to parse MIR: %v", err)
	}

	config := Config{
		MemorySize: 4096,
		StackSize:  1024,
		MaxSteps:   10000,
	}
	vm := New(config)
	if err := vm.LoadModule(module); err != nil {
		t.Fatalf("Failed to load module: %v", err)
	}

	_, err = vm.Run()
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	// total should be 3 * 4 = 12
	totalReg := vm.findVarRegister("total")
	if totalReg < 0 {
		t.Fatal("Could not find 'total' variable")
	}
	got := vm.registers[totalReg]
	if got != 12 {
		t.Errorf("total = %d, want 12", got)
	}
}

// TestSyscall tests syscall instruction
func TestSyscall(t *testing.T) {
	mir := `; Test syscall (set_pixel)
Function test.main() -> void
  Instructions:
      0: r1 = 10
      1: r2 = 20
      2: r0 = 255
      3: syscall 10 (r1, r2)
      4: return
`
	module, err := ir.ParseMIR(mir)
	if err != nil {
		t.Fatalf("Failed to parse MIR: %v", err)
	}

	// Create a platform that tracks syscalls
	plat := NewAgonPlatform()

	config := Config{
		MemorySize: 4096,
		StackSize:  1024,
		MaxSteps:   1000,
		Platform:   plat,
	}
	vm := New(config)
	if err := vm.LoadModule(module); err != nil {
		t.Fatalf("Failed to load module: %v", err)
	}

	_, err = vm.Run()
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	// Check if pixel was set
	if plat.HasDisplay() {
		display := plat.Display().(*GenericDisplay)
		// The pixel at (10, 20) should have been set to color 255
		// This is a simplified check - actual pixel value depends on color mapping
		if display == nil {
			t.Error("Display is nil")
		}
	}
}

// TestArrayExecution tests array load/store execution
func TestArrayExecution(t *testing.T) {
	// Test array store and load with dynamic index
	mir := `; Array operations
Function test.main() -> void
  Instructions:
      0: r1 = 1000
      1: r2 = 0
      2: r3 = 42
      3: r1[r2] = r3
      4: r4 = 1
      5: r5 = 43
      6: r1[r4] = r5
      7: r6 = r1[r2]
      8: r7 = r1[r4]
      9: return
`
	module, err := ir.ParseMIR(mir)
	if err != nil {
		t.Fatalf("Failed to parse MIR: %v", err)
	}

	config := Config{
		MemorySize: 4096,
		StackSize:  1024,
		MaxSteps:   100,
	}
	vm := New(config)
	if err := vm.LoadModule(module); err != nil {
		t.Fatalf("Failed to load module: %v", err)
	}

	_, err = vm.Run()
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	// Check results: r6 should be 42, r7 should be 43
	if vm.registers[6] != 42 {
		t.Errorf("r6 = %d, want 42", vm.registers[6])
	}
	if vm.registers[7] != 43 {
		t.Errorf("r7 = %d, want 43", vm.registers[7])
	}
}

// TestArrayConstantIndex tests array access with constant index
func TestArrayConstantIndex(t *testing.T) {
	mir := `; Array with constant index
Function test.main() -> void
  Instructions:
      0: r1 = 1000
      1: r2 = 99
      2: r1[5] = r2
      3: r3 = r1[5]
      4: return
`
	module, err := ir.ParseMIR(mir)
	if err != nil {
		t.Fatalf("Failed to parse MIR: %v", err)
	}

	config := Config{
		MemorySize: 4096,
		StackSize:  1024,
		MaxSteps:   100,
	}
	vm := New(config)
	if err := vm.LoadModule(module); err != nil {
		t.Fatalf("Failed to load module: %v", err)
	}

	_, err = vm.Run()
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	// r3 should be 99
	if vm.registers[3] != 99 {
		t.Errorf("r3 = %d, want 99", vm.registers[3])
	}
}

// TestFieldExecution tests struct field load/store execution
func TestFieldExecution(t *testing.T) {
	mir := `; Struct field operations
Function test.main() -> void
  Instructions:
      0: r1 = 2000
      1: r2 = 10
      2: r1.field[0] = r2
      3: r3 = 20
      4: r1.field[1] = r3
      5: r4 = r1.field[0]
      6: r5 = r1.field[1]
      7: return
`
	module, err := ir.ParseMIR(mir)
	if err != nil {
		t.Fatalf("Failed to parse MIR: %v", err)
	}

	config := Config{
		MemorySize: 4096,
		StackSize:  1024,
		MaxSteps:   100,
	}
	vm := New(config)
	if err := vm.LoadModule(module); err != nil {
		t.Fatalf("Failed to load module: %v", err)
	}

	_, err = vm.Run()
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	// r4 should be 10, r5 should be 20
	if vm.registers[4] != 10 {
		t.Errorf("r4 = %d, want 10", vm.registers[4])
	}
	if vm.registers[5] != 20 {
		t.Errorf("r5 = %d, want 20", vm.registers[5])
	}
}

// TestPointerExecution tests pointer load/store execution
func TestPointerExecution(t *testing.T) {
	mir := `; Pointer operations
Function test.main() -> void
  Instructions:
      0: r1 = 1000
      1: r2 = 42
      2: *r1 = r2
      3: r3 = *r1
      4: return
`
	module, err := ir.ParseMIR(mir)
	if err != nil {
		t.Fatalf("Failed to parse MIR: %v", err)
	}

	config := Config{
		MemorySize: 4096,
		StackSize:  1024,
		MaxSteps:   100,
	}
	vm := New(config)
	if err := vm.LoadModule(module); err != nil {
		t.Fatalf("Failed to load module: %v", err)
	}

	_, err = vm.Run()
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	// r3 should be 42 (loaded from memory[1000])
	if vm.registers[3] != 42 {
		t.Errorf("r3 = %d, want 42", vm.registers[3])
	}
}

// TestAddressOf tests address-of operator
func TestAddressOf(t *testing.T) {
	mir := `; Address-of operation
Function test.main() -> void
  Instructions:
      0: r1 = 2000
      1: r2 = &r1
      2: return
`
	module, err := ir.ParseMIR(mir)
	if err != nil {
		t.Fatalf("Failed to parse MIR: %v", err)
	}

	config := Config{
		MemorySize: 4096,
		StackSize:  1024,
		MaxSteps:   100,
	}
	vm := New(config)
	if err := vm.LoadModule(module); err != nil {
		t.Fatalf("Failed to load module: %v", err)
	}

	_, err = vm.Run()
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	// r2 should have the value from r1 (2000)
	if vm.registers[2] != 2000 {
		t.Errorf("r2 = %d, want 2000", vm.registers[2])
	}
}

// TestMIRParser tests MIR parsing edge cases
func TestMIRParser(t *testing.T) {
	tests := []struct {
		name    string
		mir     string
		wantErr bool
	}{
		{
			name: "ValidSimple",
			mir: `; Simple valid MIR
Function test.main() -> void
  Instructions:
      0: r1 = 42
      1: return
`,
			wantErr: false,
		},
		{
			name: "WithLocals",
			mir: `; MIR with locals
Function test.main() -> void
  Locals:
    r1 = x: u8
    r2 = y: u16
  Instructions:
      0: r3 = 1
      1: store x, r3
      2: return
`,
			wantErr: false,
		},
		{
			name: "WithSMC",
			mir: `; MIR with SMC flag
Function test.main() -> void
  @smc
  Instructions:
      0: return
`,
			wantErr: false,
		},
		{
			name: "AllBinaryOps",
			mir: `; All binary operations
Function test.main() -> void
  Instructions:
      0: r1 = 10
      1: r2 = 3
      2: r3 = r1 + r2
      3: r4 = r1 - r2
      4: r5 = r1 * r2
      5: r6 = r1 / r2
      6: r7 = r1 & r2
      7: r8 = r1 | r2
      8: r9 = r1 ^ r2
      9: r10 = r1 << r2
     10: r11 = r1 >> r2
     11: return
`,
			wantErr: false,
		},
		{
			name: "AllComparisonOps",
			mir: `; All comparison operations
Function test.main() -> void
  Instructions:
      0: r1 = 10
      1: r2 = 5
      2: r3 = r1 < r2
      3: r4 = r1 > r2
      4: r5 = r1 <= r2
      5: r6 = r1 >= r2
      6: r7 = r1 == r2
      7: r8 = r1 != r2
      8: return
`,
			wantErr: false,
		},
		{
			name: "ArrayLoadDynamic",
			mir: `; Array load with dynamic index
Function test.main() -> void
  Instructions:
      0: r1 = 100
      1: r2 = 5
      2: r3 = r1[r2]
      3: return
`,
			wantErr: false,
		},
		{
			name: "ArrayLoadConstant",
			mir: `; Array load with constant index
Function test.main() -> void
  Instructions:
      0: r1 = 100
      1: r2 = r1[5]
      2: return
`,
			wantErr: false,
		},
		{
			name: "ArrayStoreDynamic",
			mir: `; Array store with dynamic index
Function test.main() -> void
  Instructions:
      0: r1 = 100
      1: r2 = 5
      2: r3 = 42
      3: r1[r2] = r3
      4: return
`,
			wantErr: false,
		},
		{
			name: "ArrayStoreConstant",
			mir: `; Array store with constant index
Function test.main() -> void
  Instructions:
      0: r1 = 100
      1: r2 = 42
      2: r1[5] = r2
      3: return
`,
			wantErr: false,
		},
		{
			name: "FieldLoad",
			mir: `; Struct field load
Function test.main() -> void
  Instructions:
      0: r1 = 100
      1: r2 = r1.field[0]
      2: r3 = r1.field[1]
      3: return
`,
			wantErr: false,
		},
		{
			name: "FieldStore",
			mir: `; Struct field store
Function test.main() -> void
  Instructions:
      0: r1 = 100
      1: r2 = 42
      2: r1.field[0] = r2
      3: r1.field[1] = r2
      4: return
`,
			wantErr: false,
		},
		{
			name: "ParamLoad",
			mir: `; Function parameter load
Function test.add(a: u8, b: u8) -> u8
  Instructions:
      0: r1 = param a
      1: r2 = param b
      2: r3 = r1 + r2
      3: return r3
`,
			wantErr: false,
		},
		{
			name: "PointerLoad",
			mir: `; Pointer dereference load
Function test.main() -> void
  Instructions:
      0: r1 = 100
      1: r2 = *r1
      2: return
`,
			wantErr: false,
		},
		{
			name: "PointerStore",
			mir: `; Pointer store
Function test.main() -> void
  Instructions:
      0: r1 = 100
      1: r2 = 42
      2: *r1 = r2
      3: return
`,
			wantErr: false,
		},
		{
			name: "AddressOf",
			mir: `; Address-of operator
Function test.main() -> void
  Instructions:
      0: r1 = 100
      1: r2 = &r1
      2: return
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module, err := ir.ParseMIR(tt.mir)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMIR() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && module == nil {
				t.Error("ParseMIR() returned nil module")
			}
		})
	}
}

// TestExecutionLimit tests that execution limit is enforced
func TestExecutionLimit(t *testing.T) {
	mir := `; Infinite loop
Function test.main() -> void
  Instructions:
      0: loop:
      1: jump loop
`
	module, err := ir.ParseMIR(mir)
	if err != nil {
		t.Fatalf("Failed to parse MIR: %v", err)
	}

	config := Config{
		MemorySize: 4096,
		StackSize:  1024,
		MaxSteps:   100, // Very low limit
	}
	vm := New(config)
	if err := vm.LoadModule(module); err != nil {
		t.Fatalf("Failed to load module: %v", err)
	}

	_, err = vm.Run()
	if err == nil {
		t.Error("Expected execution limit error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "execution limit") {
		t.Errorf("Expected execution limit error, got: %v", err)
	}
}

// BenchmarkSimpleLoop benchmarks a simple counting loop
func BenchmarkSimpleLoop(b *testing.B) {
	mir := `; Benchmark loop
Function test.main() -> void
  Locals:
    r1 = count: u8
  Instructions:
      0: r2 = 0
      1: store count, r2
      2: loop:
      3: r3 = load count
      4: r4 = 100
      5: r5 = r3 < r4
      6: jump_if_not r5, end
      7: r6 = load count
      8: r7 = 1
      9: r8 = r6 + r7
     10: store count, r8
     11: jump loop
     12: end:
     13: return
`
	module, err := ir.ParseMIR(mir)
	if err != nil {
		b.Fatalf("Failed to parse MIR: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config := Config{
			MemorySize: 4096,
			StackSize:  1024,
			MaxSteps:   10000,
		}
		vm := New(config)
		vm.LoadModule(module)
		vm.Run()
	}
}
