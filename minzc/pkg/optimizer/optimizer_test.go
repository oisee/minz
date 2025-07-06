package optimizer

import (
	"testing"

	"github.com/minz/minzc/pkg/ir"
)

// Test constant folding
func TestConstantFolding(t *testing.T) {
	tests := []struct {
		name     string
		input    []ir.Instruction
		expected []ir.Instruction
	}{
		{
			name: "fold addition",
			input: []ir.Instruction{
				{Op: ir.OpLoadConst, Dest: 1, Imm: 10},
				{Op: ir.OpLoadConst, Dest: 2, Imm: 20},
				{Op: ir.OpAdd, Dest: 3, Src1: 1, Src2: 2},
			},
			expected: []ir.Instruction{
				{Op: ir.OpLoadConst, Dest: 1, Imm: 10},
				{Op: ir.OpLoadConst, Dest: 2, Imm: 20},
				{Op: ir.OpLoadConst, Dest: 3, Imm: 30, Comment: "Folded: "},
			},
		},
		{
			name: "fold multiplication",
			input: []ir.Instruction{
				{Op: ir.OpLoadConst, Dest: 1, Imm: 5},
				{Op: ir.OpLoadConst, Dest: 2, Imm: 6},
				{Op: ir.OpMul, Dest: 3, Src1: 1, Src2: 2},
			},
			expected: []ir.Instruction{
				{Op: ir.OpLoadConst, Dest: 1, Imm: 5},
				{Op: ir.OpLoadConst, Dest: 2, Imm: 6},
				{Op: ir.OpLoadConst, Dest: 3, Imm: 30, Comment: "Folded: "},
			},
		},
		{
			name: "fold comparison",
			input: []ir.Instruction{
				{Op: ir.OpLoadConst, Dest: 1, Imm: 10},
				{Op: ir.OpLoadConst, Dest: 2, Imm: 5},
				{Op: ir.OpGt, Dest: 3, Src1: 1, Src2: 2},
			},
			expected: []ir.Instruction{
				{Op: ir.OpLoadConst, Dest: 1, Imm: 10},
				{Op: ir.OpLoadConst, Dest: 2, Imm: 5},
				{Op: ir.OpLoadConst, Dest: 3, Imm: 1, Comment: "Folded: "},
			},
		},
		{
			name: "fold conditional jump with constant true",
			input: []ir.Instruction{
				{Op: ir.OpLoadConst, Dest: 1, Imm: 1},
				{Op: ir.OpJumpIfNot, Src1: 1, Label: "else"},
				{Op: ir.OpLoadConst, Dest: 2, Imm: 42},
			},
			expected: []ir.Instruction{
				{Op: ir.OpLoadConst, Dest: 1, Imm: 1},
				// Jump removed because condition is always true
				{Op: ir.OpLoadConst, Dest: 2, Imm: 42},
			},
		},
		{
			name: "fold conditional jump with constant false",
			input: []ir.Instruction{
				{Op: ir.OpLoadConst, Dest: 1, Imm: 0},
				{Op: ir.OpJumpIfNot, Src1: 1, Label: "else"},
				{Op: ir.OpLoadConst, Dest: 2, Imm: 42},
			},
			expected: []ir.Instruction{
				{Op: ir.OpLoadConst, Dest: 1, Imm: 0},
				{Op: ir.OpJump, Label: "else", Comment: "Always false: "},
				{Op: ir.OpLoadConst, Dest: 2, Imm: 42},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := &ir.Function{
				Name:         "test",
				Instructions: tt.input,
			}
			module := &ir.Module{
				Name:      "test",
				Functions: []*ir.Function{fn},
			}

			pass := NewConstantFoldingPass()
			changed, err := pass.Run(module)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !changed {
				t.Error("expected changes but none were made")
			}

			if len(fn.Instructions) != len(tt.expected) {
				t.Fatalf("expected %d instructions, got %d", len(tt.expected), len(fn.Instructions))
			}

			for i, inst := range fn.Instructions {
				exp := tt.expected[i]
				if inst.Op != exp.Op || inst.Dest != exp.Dest || inst.Imm != exp.Imm {
					t.Errorf("instruction %d mismatch: got %v, expected %v", i, inst, exp)
				}
			}
		})
	}
}

// Test dead code elimination
func TestDeadCodeElimination(t *testing.T) {
	tests := []struct {
		name     string
		input    []ir.Instruction
		expected []ir.Instruction
	}{
		{
			name: "remove unused computation",
			input: []ir.Instruction{
				{Op: ir.OpLoadConst, Dest: 1, Imm: 10},
				{Op: ir.OpLoadConst, Dest: 2, Imm: 20},
				{Op: ir.OpAdd, Dest: 3, Src1: 1, Src2: 2}, // Result never used
				{Op: ir.OpReturn},
			},
			expected: []ir.Instruction{
				{Op: ir.OpReturn},
			},
		},
		{
			name: "remove code after return",
			input: []ir.Instruction{
				{Op: ir.OpLoadConst, Dest: 1, Imm: 42},
				{Op: ir.OpReturn, Src1: 1},
				{Op: ir.OpLoadConst, Dest: 2, Imm: 99}, // Unreachable
			},
			expected: []ir.Instruction{
				{Op: ir.OpLoadConst, Dest: 1, Imm: 42},
				{Op: ir.OpReturn, Src1: 1},
			},
		},
		{
			name: "remove redundant jump",
			input: []ir.Instruction{
				{Op: ir.OpJump, Label: "next"},
				{Op: ir.OpLabel, Label: "next"},
				{Op: ir.OpReturn},
			},
			expected: []ir.Instruction{
				{Op: ir.OpLabel, Label: "next"},
				{Op: ir.OpReturn},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := &ir.Function{
				Name:         "test",
				Instructions: tt.input,
			}
			module := &ir.Module{
				Name:      "test",
				Functions: []*ir.Function{fn},
			}

			pass := NewDeadCodeEliminationPass()
			changed, err := pass.Run(module)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !changed && len(tt.input) != len(tt.expected) {
				t.Error("expected changes but none were made")
			}

			if len(fn.Instructions) != len(tt.expected) {
				t.Fatalf("expected %d instructions, got %d", len(tt.expected), len(fn.Instructions))
			}

			for i, inst := range fn.Instructions {
				exp := tt.expected[i]
				if inst.Op != exp.Op || inst.Dest != exp.Dest || inst.Src1 != exp.Src1 {
					t.Errorf("instruction %d mismatch: got %v, expected %v", i, inst, exp)
				}
			}
		})
	}
}

// Test peephole optimization
func TestPeepholeOptimization(t *testing.T) {
	tests := []struct {
		name     string
		input    []ir.Instruction
		expected []ir.Instruction
	}{
		{
			name: "load zero to xor",
			input: []ir.Instruction{
				{Op: ir.OpLoadConst, Dest: 1, Imm: 0},
			},
			expected: []ir.Instruction{
				{Op: ir.OpXor, Dest: 1, Src1: 1, Src2: 1, Comment: "XOR A,A (optimized from LD A,0)"},
			},
		},
		{
			name: "add one to inc",
			input: []ir.Instruction{
				{Op: ir.OpLoadConst, Dest: 1, Imm: 1},
				{Op: ir.OpAdd, Dest: 2, Src1: 3, Src2: 1},
			},
			expected: []ir.Instruction{
				{Op: ir.OpInc, Dest: 2, Src1: 3, Comment: "INC (optimized from ADD 1)"},
			},
		},
		{
			name: "multiply by power of 2 to shift",
			input: []ir.Instruction{
				{Op: ir.OpLoadConst, Dest: 1, Imm: 8},
				{Op: ir.OpMul, Dest: 2, Src1: 3, Src2: 1},
			},
			expected: []ir.Instruction{
				{Op: ir.OpLoadConst, Dest: 1, Imm: 3}, // 8 = 2^3
				{Op: ir.OpShl, Dest: 2, Src1: 3, Src2: 1, Comment: "SHL (optimized from MUL by power of 2)"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := &ir.Function{
				Name:         "test",
				Instructions: tt.input,
			}
			module := &ir.Module{
				Name:      "test",
				Functions: []*ir.Function{fn},
			}

			pass := NewPeepholeOptimizationPass()
			changed, err := pass.Run(module)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !changed {
				t.Error("expected changes but none were made")
			}

			if len(fn.Instructions) != len(tt.expected) {
				t.Fatalf("expected %d instructions, got %d", len(tt.expected), len(fn.Instructions))
			}

			for i, inst := range fn.Instructions {
				exp := tt.expected[i]
				if inst.Op != exp.Op {
					t.Errorf("instruction %d op mismatch: got %v, expected %v", i, inst.Op, exp.Op)
				}
			}
		})
	}
}