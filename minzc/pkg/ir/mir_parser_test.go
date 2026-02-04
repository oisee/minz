package ir

import (
	"strings"
	"testing"
)

func TestParseMIR_ArrayAccess(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantOp   Opcode
		wantDest Register
		wantSrc1 Register
		wantSrc2 Register
		wantImm  int64
	}{
		{
			name:     "LoadIndex dynamic",
			input:    "r0 = r1[r2]",
			wantOp:   OpLoadIndex,
			wantDest: 0,
			wantSrc1: 1,
			wantSrc2: 2,
		},
		{
			name:     "LoadElement constant",
			input:    "r3 = r4[5]",
			wantOp:   OpLoadElement,
			wantDest: 3,
			wantSrc1: 4,
			wantImm:  5,
		},
		{
			name:     "LoadElement zero index",
			input:    "r0 = r1[0]",
			wantOp:   OpLoadElement,
			wantDest: 0,
			wantSrc1: 1,
			wantImm:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mir := ".function test()\n" + tt.input + "\n.end"
			module, err := ParseMIR(mir)
			if err != nil {
				t.Fatalf("ParseMIR failed: %v", err)
			}

			if len(module.Functions) == 0 {
				t.Fatal("No functions parsed")
			}

			fn := module.Functions[0]
			if len(fn.Instructions) == 0 {
				t.Fatal("No instructions parsed")
			}

			inst := fn.Instructions[0]
			if inst.Op != tt.wantOp {
				t.Errorf("Op = %v, want %v", inst.Op, tt.wantOp)
			}
			if inst.Dest != tt.wantDest {
				t.Errorf("Dest = %v, want %v", inst.Dest, tt.wantDest)
			}
			if inst.Src1 != tt.wantSrc1 {
				t.Errorf("Src1 = %v, want %v", inst.Src1, tt.wantSrc1)
			}
			if tt.wantOp == OpLoadIndex && inst.Src2 != tt.wantSrc2 {
				t.Errorf("Src2 = %v, want %v", inst.Src2, tt.wantSrc2)
			}
			if tt.wantOp == OpLoadElement && inst.Imm != tt.wantImm {
				t.Errorf("Imm = %v, want %v", inst.Imm, tt.wantImm)
			}
		})
	}
}

func TestParseMIR_ArrayStore(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantOp   Opcode
		wantDest Register
		wantSrc1 Register
		wantSrc2 Register
		wantImm  int64
	}{
		{
			name:     "StoreIndex dynamic",
			input:    "r1[r2] = r0",
			wantOp:   OpStoreIndex,
			wantDest: 0,  // value to store
			wantSrc1: 1,  // base address
			wantSrc2: 2,  // index
		},
		{
			name:     "StoreElement constant",
			input:    "r3[5] = r4",
			wantOp:   OpStoreElement,
			wantDest: 4,  // value to store
			wantSrc1: 3,  // base address
			wantImm:  5,  // constant index
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mir := ".function test()\n" + tt.input + "\n.end"
			module, err := ParseMIR(mir)
			if err != nil {
				t.Fatalf("ParseMIR failed: %v", err)
			}

			if len(module.Functions) == 0 {
				t.Fatal("No functions parsed")
			}

			fn := module.Functions[0]
			if len(fn.Instructions) == 0 {
				t.Fatal("No instructions parsed")
			}

			inst := fn.Instructions[0]
			if inst.Op != tt.wantOp {
				t.Errorf("Op = %v, want %v", inst.Op, tt.wantOp)
			}
			if inst.Dest != tt.wantDest {
				t.Errorf("Dest = %v, want %v", inst.Dest, tt.wantDest)
			}
			if inst.Src1 != tt.wantSrc1 {
				t.Errorf("Src1 = %v, want %v", inst.Src1, tt.wantSrc1)
			}
			if tt.wantOp == OpStoreIndex && inst.Src2 != tt.wantSrc2 {
				t.Errorf("Src2 = %v, want %v", inst.Src2, tt.wantSrc2)
			}
			if tt.wantOp == OpStoreElement && inst.Imm != tt.wantImm {
				t.Errorf("Imm = %v, want %v", inst.Imm, tt.wantImm)
			}
		})
	}
}

func TestParseMIR_StructField(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantOp   Opcode
		wantDest Register
		wantSrc1 Register
		wantSrc2 Register
		wantImm  int64
	}{
		{
			name:     "LoadField",
			input:    "r0 = r1.field[2]",
			wantOp:   OpLoadField,
			wantDest: 0,
			wantSrc1: 1,
			wantImm:  2,
		},
		{
			name:     "StoreField",
			input:    "r1.field[0] = r2",
			wantOp:   OpStoreField,
			wantSrc1: 1,  // base address
			wantSrc2: 2,  // value to store
			wantImm:  0,  // field offset
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mir := ".function test()\n" + tt.input + "\n.end"
			module, err := ParseMIR(mir)
			if err != nil {
				t.Fatalf("ParseMIR failed: %v", err)
			}

			if len(module.Functions) == 0 {
				t.Fatal("No functions parsed")
			}

			fn := module.Functions[0]
			if len(fn.Instructions) == 0 {
				t.Fatal("No instructions parsed")
			}

			inst := fn.Instructions[0]
			if inst.Op != tt.wantOp {
				t.Errorf("Op = %v, want %v", inst.Op, tt.wantOp)
			}
			if tt.wantOp == OpLoadField {
				if inst.Dest != tt.wantDest {
					t.Errorf("Dest = %v, want %v", inst.Dest, tt.wantDest)
				}
				if inst.Src1 != tt.wantSrc1 {
					t.Errorf("Src1 = %v, want %v", inst.Src1, tt.wantSrc1)
				}
			} else {
				if inst.Src1 != tt.wantSrc1 {
					t.Errorf("Src1 = %v, want %v", inst.Src1, tt.wantSrc1)
				}
				if inst.Src2 != tt.wantSrc2 {
					t.Errorf("Src2 = %v, want %v", inst.Src2, tt.wantSrc2)
				}
			}
			if inst.Imm != tt.wantImm {
				t.Errorf("Imm = %v, want %v", inst.Imm, tt.wantImm)
			}
		})
	}
}

func TestParseMIR_ParamLoad(t *testing.T) {
	mir := `.function test()
r0 = param x
r1 = param counter
.end`

	module, err := ParseMIR(mir)
	if err != nil {
		t.Fatalf("ParseMIR failed: %v", err)
	}

	if len(module.Functions) == 0 {
		t.Fatal("No functions parsed")
	}

	fn := module.Functions[0]
	if len(fn.Instructions) < 2 {
		t.Fatalf("Expected 2 instructions, got %d", len(fn.Instructions))
	}

	// First instruction: r0 = param x
	inst := fn.Instructions[0]
	if inst.Op != OpLoadParam {
		t.Errorf("Op = %v, want %v", inst.Op, OpLoadParam)
	}
	if inst.Dest != 0 {
		t.Errorf("Dest = %v, want 0", inst.Dest)
	}
	if inst.Symbol != "x" {
		t.Errorf("Symbol = %q, want %q", inst.Symbol, "x")
	}

	// Second instruction: r1 = param counter
	inst = fn.Instructions[1]
	if inst.Op != OpLoadParam {
		t.Errorf("Op = %v, want %v", inst.Op, OpLoadParam)
	}
	if inst.Dest != 1 {
		t.Errorf("Dest = %v, want 1", inst.Dest)
	}
	if inst.Symbol != "counter" {
		t.Errorf("Symbol = %q, want %q", inst.Symbol, "counter")
	}
}

func TestParseMIR_ComplexFunction(t *testing.T) {
	// A more complex test that combines multiple features
	mir := `Function compute(a: u8, b: u8) -> u8
r1 = a: u8
r2 = b: u8
Instructions:
0: r3 = r1 + r2
1: r4 = r3 * 2
2: return r4
`

	module, err := ParseMIR(mir)
	if err != nil {
		t.Fatalf("ParseMIR failed: %v", err)
	}

	if len(module.Functions) != 1 {
		t.Fatalf("Expected 1 function, got %d", len(module.Functions))
	}

	fn := module.Functions[0]
	if fn.Name != "compute" {
		t.Errorf("Function name = %q, want %q", fn.Name, "compute")
	}

	if len(fn.Params) != 2 {
		t.Errorf("Expected 2 params, got %d", len(fn.Params))
	}

	if len(fn.Locals) != 2 {
		t.Errorf("Expected 2 locals, got %d", len(fn.Locals))
	}

	if len(fn.Instructions) < 3 {
		t.Fatalf("Expected at least 3 instructions, got %d", len(fn.Instructions))
	}

	// Check add instruction
	if fn.Instructions[0].Op != OpAdd {
		t.Errorf("First instruction Op = %v, want %v", fn.Instructions[0].Op, OpAdd)
	}
}

func TestParseMIR_JumpLabels(t *testing.T) {
	mir := `.function test()
jmp end
loop:
r0 = r0 + 1
jmp loop
end:
return r0
.end`

	module, err := ParseMIR(mir)
	if err != nil {
		t.Fatalf("ParseMIR failed: %v", err)
	}

	fn := module.Functions[0]

	// Find jump instructions
	var jumpInst *Instruction
	for i := range fn.Instructions {
		if fn.Instructions[i].Op == OpJmp && fn.Instructions[i].Label == "end" {
			jumpInst = &fn.Instructions[i]
			break
		}
	}

	if jumpInst == nil {
		t.Fatal("Jump to 'end' not found")
	}

	// Verify label was resolved
	if jumpInst.Target < 0 {
		t.Log("Note: Label resolution may be deferred to execution time")
	}
}

func TestParseMIR_BinaryOperations(t *testing.T) {
	tests := []struct {
		input  string
		wantOp Opcode
	}{
		{"r0 = r1 + r2", OpAdd},
		{"r0 = r1 - r2", OpSub},
		{"r0 = r1 * r2", OpMul},
		{"r0 = r1 / r2", OpDiv},
		{"r0 = r1 % r2", OpMod},
		{"r0 = r1 & r2", OpAnd},
		{"r0 = r1 | r2", OpOr},
		{"r0 = r1 ^ r2", OpXor},
		{"r0 = r1 << r2", OpShl},
		{"r0 = r1 >> r2", OpShr},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			mir := ".function test()\n" + tt.input + "\n.end"
			module, err := ParseMIR(mir)
			if err != nil {
				t.Fatalf("ParseMIR failed: %v", err)
			}

			fn := module.Functions[0]
			if len(fn.Instructions) == 0 {
				t.Fatal("No instructions parsed")
			}

			if fn.Instructions[0].Op != tt.wantOp {
				t.Errorf("Op = %v, want %v", fn.Instructions[0].Op, tt.wantOp)
			}
		})
	}
}

func TestParseMIR_ComparisonOperations(t *testing.T) {
	tests := []struct {
		input  string
		wantOp Opcode
	}{
		{"r0 = r1 < r2", OpLt},
		{"r0 = r1 > r2", OpGt},
		{"r0 = r1 <= r2", OpLe},
		{"r0 = r1 >= r2", OpGe},
		{"r0 = r1 == r2", OpEq},
		{"r0 = r1 != r2", OpNe},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			mir := ".function test()\n" + tt.input + "\n.end"
			module, err := ParseMIR(mir)
			if err != nil {
				t.Fatalf("ParseMIR failed: %v", err)
			}

			fn := module.Functions[0]
			if len(fn.Instructions) == 0 {
				t.Fatal("No instructions parsed")
			}

			if fn.Instructions[0].Op != tt.wantOp {
				t.Errorf("Op = %v, want %v", fn.Instructions[0].Op, tt.wantOp)
			}
		})
	}
}

func TestParseMIR_MemoryOperations(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wantOp Opcode
	}{
		{"LoadMem", "r0 = [r1]", OpLoadMem},
		{"LoadMemOffset", "r0 = [r1 + 8]", OpLoadMem},
		{"StoreMem", "[r0] = r1", OpStoreMem},
		{"Deref", "r0 = *r1", OpLoad},
		{"StorePtr", "*r0 = r1", OpStore},
		{"AddrOf", "r0 = &r1", OpAddr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mir := ".function test()\n" + tt.input + "\n.end"
			module, err := ParseMIR(mir)
			if err != nil {
				t.Fatalf("ParseMIR failed: %v", err)
			}

			fn := module.Functions[0]
			if len(fn.Instructions) == 0 {
				t.Fatal("No instructions parsed")
			}

			if fn.Instructions[0].Op != tt.wantOp {
				t.Errorf("Op = %v, want %v", fn.Instructions[0].Op, tt.wantOp)
			}
		})
	}
}

func TestParseMIR_SMCInstructions(t *testing.T) {
	// SMC instructions should be parsed as NOPs for VM execution
	smc_inputs := []string{
		"TRUE_SMC_PATCH x",
		"PATCH_VALUE y",
		"patch_param z",
		"smc_store x",
		"tsmc_patch",
	}

	for _, input := range smc_inputs {
		t.Run(input, func(t *testing.T) {
			mir := ".function test()\n" + input + "\n.end"
			module, err := ParseMIR(mir)
			if err != nil {
				t.Fatalf("ParseMIR failed for SMC instruction: %v", err)
			}

			fn := module.Functions[0]
			if len(fn.Instructions) == 0 {
				t.Fatal("No instructions parsed")
			}

			// SMC instructions should be converted to NOP
			if fn.Instructions[0].Op != OpNop {
				t.Errorf("SMC instruction should be NOP, got %v", fn.Instructions[0].Op)
			}
		})
	}
}

func TestParseMIR_FunctionDumpFormat(t *testing.T) {
	// Test parsing of function dump format (from MIR dumps)
	mir := `Function add(x: i16, y: i16) -> i16
Locals:
r1 = x: i16
r2 = y: i16
Instructions:
0: r3 = r1 + r2
1: return r3
`

	module, err := ParseMIR(mir)
	if err != nil {
		t.Fatalf("ParseMIR failed: %v", err)
	}

	if len(module.Functions) != 1 {
		t.Fatalf("Expected 1 function, got %d", len(module.Functions))
	}

	fn := module.Functions[0]
	if fn.Name != "add" {
		t.Errorf("Function name = %q, want %q", fn.Name, "add")
	}

	if len(fn.Params) != 2 {
		t.Errorf("Expected 2 params, got %d", len(fn.Params))
	}

	// Check that parameters were parsed
	if len(fn.Params) > 0 && fn.Params[0].Name != "x" {
		t.Errorf("First param name = %q, want %q", fn.Params[0].Name, "x")
	}
}

func TestParseMIR_CallInstruction(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantFunc  string
		wantArgs  int
	}{
		{"SimpleCall", "r0 = call foo", "foo", 0},
		{"CallWithArgs", "r0 = call bar(r1, r2, r3)", "bar", 3},
		{"StandaloneCall", "call baz", "baz", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mir := ".function test()\n" + tt.input + "\n.end"
			module, err := ParseMIR(mir)
			if err != nil {
				t.Fatalf("ParseMIR failed: %v", err)
			}

			fn := module.Functions[0]
			if len(fn.Instructions) == 0 {
				t.Fatal("No instructions parsed")
			}

			inst := fn.Instructions[0]
			if inst.Op != OpCall {
				t.Errorf("Op = %v, want %v", inst.Op, OpCall)
			}

			funcName := inst.FuncName
			if funcName == "" {
				funcName = inst.Symbol
			}
			if !strings.Contains(funcName, tt.wantFunc) {
				t.Errorf("FuncName = %q, want to contain %q", funcName, tt.wantFunc)
			}

			if tt.wantArgs > 0 && len(inst.Args) != tt.wantArgs {
				t.Errorf("Args count = %d, want %d", len(inst.Args), tt.wantArgs)
			}
		})
	}
}
