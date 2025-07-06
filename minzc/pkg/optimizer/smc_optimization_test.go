package optimizer

import (
	"testing"

	"github.com/minz/minzc/pkg/ir"
)

// Test self-modifying code optimization
func TestSelfModifyingCodeOptimization(t *testing.T) {
	tests := []struct {
		name         string
		input        []ir.Instruction
		isSMCEnabled bool
		isRecursive  bool
		expectChange bool
		checkSMC     bool
	}{
		{
			name: "simple constant to SMC",
			input: []ir.Instruction{
				{Op: ir.OpLoadConst, Dest: 1, Imm: 42},
				{Op: ir.OpLoadVar, Dest: 2},
				{Op: ir.OpAdd, Dest: 3, Src1: 1, Src2: 2},
				{Op: ir.OpReturn, Src1: 3},
			},
			isSMCEnabled: true,
			isRecursive:  false,
			expectChange: true,
			checkSMC:     true,
		},
		{
			name: "parameter optimization",
			input: []ir.Instruction{
				{Op: ir.OpLoadParam, Dest: 1, Src1: 0}, // First parameter
				{Op: ir.OpLoadConst, Dest: 2, Imm: 10},
				{Op: ir.OpAdd, Dest: 3, Src1: 1, Src2: 2},
				{Op: ir.OpReturn, Src1: 3},
			},
			isSMCEnabled: true,
			isRecursive:  false,
			expectChange: true,
			checkSMC:     true,
		},
		{
			name: "recursive function not optimized",
			input: []ir.Instruction{
				{Op: ir.OpLoadConst, Dest: 1, Imm: 42},
				{Op: ir.OpCall, Symbol: "test"}, // Recursive call
				{Op: ir.OpReturn, Src1: 1},
			},
			isSMCEnabled: true,
			isRecursive:  true,
			expectChange: false,
			checkSMC:     false,
		},
		{
			name: "SMC disabled",
			input: []ir.Instruction{
				{Op: ir.OpLoadConst, Dest: 1, Imm: 42},
				{Op: ir.OpReturn, Src1: 1},
			},
			isSMCEnabled: false,
			isRecursive:  false,
			expectChange: false,
			checkSMC:     false,
		},
		{
			name: "modifiable constant",
			input: []ir.Instruction{
				{Op: ir.OpLoadConst, Dest: 1, Imm: 100},
				{Op: ir.OpLoadVar, Dest: 2},
				{Op: ir.OpAdd, Dest: 1, Src1: 1, Src2: 2}, // Modifies the constant register
				{Op: ir.OpReturn, Src1: 1},
			},
			isSMCEnabled: true,
			isRecursive:  false,
			expectChange: true,
			checkSMC:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := &ir.Function{
				Name:         "test",
				Instructions: tt.input,
				IsSMCEnabled: tt.isSMCEnabled,
				IsRecursive:  tt.isRecursive,
				NumParams:    1,
			}
			module := &ir.Module{
				Name:      "test",
				Functions: []*ir.Function{fn},
			}

			pass := NewSelfModifyingCodePass()
			changed, err := pass.Run(module)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if changed != tt.expectChange {
				t.Errorf("expected changed=%v, got %v", tt.expectChange, changed)
			}

			if tt.checkSMC {
				// Check if SMC instructions were generated
				foundSMC := false
				for _, inst := range fn.Instructions {
					if inst.Op == ir.OpSMCLoadConst {
						foundSMC = true
						if inst.SMCLabel == "" {
							t.Error("SMC load instruction missing label")
						}
						break
					}
				}
				
				if !foundSMC {
					t.Error("expected SMC instructions but none found")
				}
			}
		})
	}
}

// Test recursion detection
func TestRecursionDetection(t *testing.T) {
	tests := []struct {
		name        string
		functions   []*ir.Function
		recursive   map[string]bool
	}{
		{
			name: "direct recursion",
			functions: []*ir.Function{
				{
					Name: "factorial",
					Instructions: []ir.Instruction{
						{Op: ir.OpLoadParam, Dest: 1, Src1: 0},
						{Op: ir.OpLoadConst, Dest: 2, Imm: 1},
						{Op: ir.OpLe, Dest: 3, Src1: 1, Src2: 2},
						{Op: ir.OpJumpIfNot, Src1: 3, Label: "recurse"},
						{Op: ir.OpReturn, Src1: 2},
						{Op: ir.OpLabel, Label: "recurse"},
						{Op: ir.OpCall, Symbol: "factorial"}, // Direct recursion
						{Op: ir.OpReturn},
					},
				},
			},
			recursive: map[string]bool{
				"factorial": true,
			},
		},
		{
			name: "indirect recursion",
			functions: []*ir.Function{
				{
					Name: "foo",
					Instructions: []ir.Instruction{
						{Op: ir.OpCall, Symbol: "bar"},
						{Op: ir.OpReturn},
					},
				},
				{
					Name: "bar",
					Instructions: []ir.Instruction{
						{Op: ir.OpCall, Symbol: "baz"},
						{Op: ir.OpReturn},
					},
				},
				{
					Name: "baz",
					Instructions: []ir.Instruction{
						{Op: ir.OpCall, Symbol: "foo"}, // Indirect recursion
						{Op: ir.OpReturn},
					},
				},
			},
			recursive: map[string]bool{
				"foo": true,
				"bar": true,
				"baz": true,
			},
		},
		{
			name: "no recursion",
			functions: []*ir.Function{
				{
					Name: "main",
					Instructions: []ir.Instruction{
						{Op: ir.OpCall, Symbol: "helper"},
						{Op: ir.OpReturn},
					},
				},
				{
					Name: "helper",
					Instructions: []ir.Instruction{
						{Op: ir.OpLoadConst, Dest: 1, Imm: 42},
						{Op: ir.OpReturn, Src1: 1},
					},
				},
			},
			recursive: map[string]bool{
				"main":   false,
				"helper": false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &ir.Module{
				Name:      "test",
				Functions: tt.functions,
			}

			detector := NewRecursionDetector()
			detector.AnalyzeModule(module)

			for _, fn := range module.Functions {
				expected := tt.recursive[fn.Name]
				if fn.IsRecursive != expected {
					t.Errorf("function %s: expected recursive=%v, got %v", 
						fn.Name, expected, fn.IsRecursive)
				}
			}
		})
	}
}