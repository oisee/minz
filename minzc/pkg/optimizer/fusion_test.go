package optimizer

import (
	"testing"

	"github.com/minz/minzc/pkg/ir"
)

// buildDJNZLoop creates a standard DJNZ iterator loop MIR pattern
// with an OpCall to the given function inside the loop body.
func buildDJNZLoop(callSymbol string, callArgs []ir.Register, callDest ir.Register) []ir.Instruction {
	return []ir.Instruction{
		{Op: ir.OpNop, Comment: "DJNZ OPTIMIZED LOOP for array[5]"},
		{Op: ir.OpLoadConst, Dest: 9, Imm: 5, Comment: "DJNZ counter = 5"},
		{Op: ir.OpMove, Dest: 10, Src1: 8, Comment: "Pointer to array start"},
		{Op: ir.OpLabel, Label: "djnz_loop_1"},
		{Op: ir.OpLoad, Dest: 11, Src1: 10, Comment: "Load element via pointer"},
		{Op: ir.OpPush, Src1: 10, Comment: "Save array pointer before operations"},
		{Op: ir.OpCall, Dest: callDest, Symbol: callSymbol, Args: callArgs, Comment: "Call " + callSymbol},
		{Op: ir.OpPop, Dest: 10, Comment: "Restore array pointer after operations"},
		{Op: ir.OpInc, Dest: 10, Src1: 10, Comment: "Advance to next element"},
		{Op: ir.OpDJNZ, Src1: 9, Label: "djnz_loop_1", Comment: "DJNZ - decrement and loop"},
	}
}

func TestFusionDetectsDJNZLoop(t *testing.T) {
	module := &ir.Module{
		Functions: []*ir.Function{
			{
				Name:         "double",
				NumParams:    1,
				Params:       []ir.Parameter{{Name: "x", Reg: 2}},
				NextRegister: 5,
				Instructions: []ir.Instruction{
					{Op: ir.OpTrueSMCLoad, Dest: 2, Symbol: "x_imm0", Comment: "Load from anchor x_imm0"},
					{Op: ir.OpShl, Dest: 4, Src1: 2, Imm: 1},
					{Op: ir.OpReturn, Src1: 4},
				},
			},
			{
				Name:         "main",
				NextRegister: 15,
				Instructions: buildDJNZLoop("double", []ir.Register{11}, 12),
			},
		},
	}

	f := NewFusionOptimizer()
	changed, err := f.Run(module)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !changed {
		t.Fatal("Expected fusion to make changes")
	}

	// Verify no OpCall to "double" remains in main
	main := module.Functions[1]
	for _, inst := range main.Instructions {
		if inst.Op == ir.OpCall && inst.Symbol == "double" {
			t.Error("OpCall to 'double' should have been inlined")
		}
	}

	// Verify the inlined body contains a shift operation
	foundShift := false
	for _, inst := range main.Instructions {
		if inst.Op == ir.OpShl {
			foundShift = true
			break
		}
	}
	if !foundShift {
		t.Error("Expected inlined OpShl from 'double' function")
	}
}

func TestFusionSkipsAsmFunctions(t *testing.T) {
	module := &ir.Module{
		Functions: []*ir.Function{
			{
				Name:         "console_log",
				NumParams:    1,
				Params:       []ir.Parameter{{Name: "c", Reg: 2}},
				NextRegister: 3,
				Instructions: []ir.Instruction{
					{Op: ir.OpAsm, AsmCode: "OUT ($23), A"},
					{Op: ir.OpReturn},
				},
			},
			{
				Name:         "main",
				NextRegister: 15,
				Instructions: buildDJNZLoop("console_log", []ir.Register{11}, 12),
			},
		},
	}

	f := NewFusionOptimizer()
	changed, err := f.Run(module)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if changed {
		t.Error("Fusion should NOT inline asm functions")
	}

	// Verify the OpCall to console_log still exists
	main := module.Functions[1]
	foundCall := false
	for _, inst := range main.Instructions {
		if inst.Op == ir.OpCall && inst.Symbol == "console_log" {
			foundCall = true
			break
		}
	}
	if !foundCall {
		t.Error("OpCall to 'console_log' should remain (asm function)")
	}
}

func TestFusionSkipsMultiParamFunctions(t *testing.T) {
	module := &ir.Module{
		Functions: []*ir.Function{
			{
				Name:         "add",
				NumParams:    2,
				Params:       []ir.Parameter{{Name: "x", Reg: 2}, {Name: "y", Reg: 3}},
				NextRegister: 5,
				Instructions: []ir.Instruction{
					{Op: ir.OpLoadParam, Dest: 2, Symbol: "x"},
					{Op: ir.OpLoadParam, Dest: 3, Symbol: "y"},
					{Op: ir.OpAdd, Dest: 4, Src1: 2, Src2: 3},
					{Op: ir.OpReturn, Src1: 4},
				},
			},
			{
				Name:         "main",
				NextRegister: 15,
				Instructions: buildDJNZLoop("add", []ir.Register{11}, 12),
			},
		},
	}

	f := NewFusionOptimizer()
	changed, _ := f.Run(module)
	if changed {
		t.Error("Fusion should NOT inline multi-parameter functions")
	}
}

func TestFusionHandlesLoadParam(t *testing.T) {
	module := &ir.Module{
		Functions: []*ir.Function{
			{
				Name:         "inc_one",
				NumParams:    1,
				Params:       []ir.Parameter{{Name: "x", Reg: 2}},
				NextRegister: 5,
				Instructions: []ir.Instruction{
					{Op: ir.OpLoadParam, Dest: 2, Symbol: "x"},
					{Op: ir.OpLoadConst, Dest: 3, Imm: 1},
					{Op: ir.OpAdd, Dest: 4, Src1: 2, Src2: 3},
					{Op: ir.OpReturn, Src1: 4},
				},
			},
			{
				Name:         "main",
				NextRegister: 15,
				Instructions: buildDJNZLoop("inc_one", []ir.Register{11}, 12),
			},
		},
	}

	f := NewFusionOptimizer()
	changed, err := f.Run(module)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !changed {
		t.Fatal("Expected fusion to inline inc_one (uses OpLoadParam)")
	}

	// Verify inlined: OpMove (param) + OpLoadConst + OpAdd + OpMove (return)
	main := module.Functions[1]
	foundAdd := false
	for _, inst := range main.Instructions {
		if inst.Op == ir.OpAdd {
			foundAdd = true
			break
		}
	}
	if !foundAdd {
		t.Error("Expected inlined OpAdd from 'inc_one' function")
	}
}

func TestFusionReturnValueMapping(t *testing.T) {
	module := &ir.Module{
		Functions: []*ir.Function{
			{
				Name:         "double",
				NumParams:    1,
				Params:       []ir.Parameter{{Name: "x", Reg: 2}},
				NextRegister: 5,
				Instructions: []ir.Instruction{
					{Op: ir.OpTrueSMCLoad, Dest: 2, Symbol: "x_imm0"},
					{Op: ir.OpShl, Dest: 4, Src1: 2, Imm: 1},
					{Op: ir.OpReturn, Src1: 4},
				},
			},
			{
				Name:         "main",
				NextRegister: 15,
				Instructions: buildDJNZLoop("double", []ir.Register{11}, 12),
			},
		},
	}

	f := NewFusionOptimizer()
	f.Run(module)

	// Find the return value move: should map to call dest (r12)
	main := module.Functions[1]
	foundReturnMove := false
	for _, inst := range main.Instructions {
		if inst.Op == ir.OpMove && inst.Dest == 12 && inst.Comment == "Fused: return value" {
			foundReturnMove = true
			break
		}
	}
	if !foundReturnMove {
		t.Error("Expected OpMove mapping return value to r12 (call dest)")
	}
}

func TestFusionPreservesNonDJNZCode(t *testing.T) {
	// A function with no DJNZ loops should not be modified
	module := &ir.Module{
		Functions: []*ir.Function{
			{
				Name:         "helper",
				NumParams:    1,
				Params:       []ir.Parameter{{Name: "x", Reg: 2}},
				NextRegister: 3,
				Instructions: []ir.Instruction{
					{Op: ir.OpLoadConst, Dest: 2, Imm: 42},
					{Op: ir.OpReturn, Src1: 2},
				},
			},
			{
				Name:         "main",
				NextRegister: 5,
				Instructions: []ir.Instruction{
					{Op: ir.OpCall, Dest: 3, Symbol: "helper", Args: []ir.Register{1}},
					{Op: ir.OpReturn, Src1: 3},
				},
			},
		},
	}

	f := NewFusionOptimizer()
	changed, _ := f.Run(module)
	if changed {
		t.Error("Fusion should not modify code without DJNZ iterator loops")
	}
}

func TestFusionSkipsLargeFunctions(t *testing.T) {
	// A function with > 8 instructions should not be inlined
	bigFunc := &ir.Function{
		Name:         "big",
		NumParams:    1,
		Params:       []ir.Parameter{{Name: "x", Reg: 2}},
		NextRegister: 12,
	}
	for i := 0; i < 10; i++ {
		bigFunc.Instructions = append(bigFunc.Instructions, ir.Instruction{
			Op: ir.OpLoadConst, Dest: ir.Register(i + 2), Imm: int64(i),
		})
	}
	bigFunc.Instructions = append(bigFunc.Instructions, ir.Instruction{
		Op: ir.OpReturn, Src1: 2,
	})

	module := &ir.Module{
		Functions: []*ir.Function{
			bigFunc,
			{
				Name:         "main",
				NextRegister: 15,
				Instructions: buildDJNZLoop("big", []ir.Register{11}, 12),
			},
		},
	}

	f := NewFusionOptimizer()
	changed, _ := f.Run(module)
	if changed {
		t.Error("Fusion should not inline functions with > 8 instructions")
	}
}
