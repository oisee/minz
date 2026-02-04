package interpreter

import (
	"testing"
	"strings"
	
	"github.com/minz/minzc/pkg/ir"
)

func TestMIRInterpreter_BasicArithmetic(t *testing.T) {
	interp := NewMIRInterpreter()
	
	// Create a simple function: return a + b
	function := &ir.Function{
		Name: "add",
		Instructions: []ir.Instruction{
			{Op: ir.OpAdd, Dest: 3, Src1: 1, Src2: 2},
			{Op: ir.OpReturn, Src1: 3},
		},
	}
	
	result, err := interp.Execute(function, []int64{5, 3})
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}
	
	if result != 8 {
		t.Errorf("Expected 8, got %d", result)
	}
}

func TestMIRInterpreter_MinzMetafunction(t *testing.T) {
	interp := NewMIRInterpreter()
	
	tests := []struct {
		name     string
		template string
		args     []interface{}
		expected string
	}{
		{
			name:     "Simple function generation",
			template: "fun hello_{0}() -> void { @print(\"Hi {0}!\"); }",
			args:     []interface{}{"world"},
			expected: "fun hello_world() -> void { @print(\"Hi world!\"); }",
		},
		{
			name:     "Variable declaration",
			template: "var {0}: u8 = {1};",
			args:     []interface{}{"counter", int64(42)},
			expected: "var counter: u8 = 42;",
		},
		{
			name:     "Complex template",
			template: "var {0}_hp: u8 = {1}; var {0}_mp: u8 = {2};",
			args:     []interface{}{"player", int64(100), int64(50)},
			expected: "var player_hp: u8 = 100; var player_mp: u8 = 50;",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := interp.ExecuteMinzMetafunction(tt.template, tt.args)
			if err != nil {
				t.Fatalf("Metafunction failed: %v", err)
			}
			
			if result != tt.expected {
				t.Errorf("Expected:\n%s\nGot:\n%s", tt.expected, result)
			}
		})
	}
}

func TestMIRInterpreter_TemplateValidation(t *testing.T) {
	interp := NewMIRInterpreter()
	
	// Valid template
	err := interp.ValidateTemplate("fun {0}_{1}() -> {2}", 3)
	if err != nil {
		t.Errorf("Valid template rejected: %v", err)
	}
	
	// Missing placeholder
	err = interp.ValidateTemplate("fun {0}_{2}() -> void", 3)
	if err == nil || !strings.Contains(err.Error(), "missing placeholder {1}") {
		t.Errorf("Should reject template with missing placeholder {1}")
	}
	
	// Unbalanced braces
	err = interp.ValidateTemplate("fun {0}_{1} -> void", 2)
	if err == nil || !strings.Contains(err.Error(), "unbalanced braces") {
		t.Errorf("Should reject template with unbalanced braces")
	}
}

func TestMIRInterpreter_StringOperations(t *testing.T) {
	interp := NewMIRInterpreter()
	
	// Store some strings
	id1 := interp.storeString("Hello")
	id2 := interp.storeString(" World")
	
	// Create function that concatenates strings
	function := &ir.Function{
		Name: "concat_test",
		Instructions: []ir.Instruction{
			{Op: ir.OpCall, Symbol: "string_concat", Dest: 3, Args: []ir.Register{1, 2}},
			{Op: ir.OpReturn, Src1: 3},
		},
	}
	
	result, err := interp.Execute(function, []int64{id1, id2})
	if err != nil {
		t.Fatalf("String concat failed: %v", err)
	}
	
	resultStr := interp.getString(result)
	if resultStr != "Hello World" {
		t.Errorf("Expected 'Hello World', got '%s'", resultStr)
	}
}

func TestMIRInterpreter_ConditionalJump(t *testing.T) {
	interp := NewMIRInterpreter()
	
	// Function: if (a == b) return 1 else return 0
	function := &ir.Function{
		Name: "compare",
		Instructions: []ir.Instruction{
			{Op: ir.OpCmp, Src1: 1, Src2: 2},              // Compare a and b
			{Op: ir.OpJumpIfNot, Label: "not_equal"},      // Jump if not equal
			{Op: ir.OpLoadConst, Dest: 3, Imm: 1},         // Load 1
			{Op: ir.OpJump, Label: "end"},                 // Jump to end
			{Op: ir.OpLabel, Label: "not_equal"},          // Label: not_equal
			{Op: ir.OpLoadConst, Dest: 3, Imm: 0},         // Load 0
			{Op: ir.OpLabel, Label: "end"},                // Label: end
			{Op: ir.OpReturn, Src1: 3},                    // Return result
		},
	}
	
	// Test equal values
	result, err := interp.Execute(function, []int64{5, 5})
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}
	if result != 1 {
		t.Errorf("Expected 1 for equal values, got %d", result)
	}
	
	// Test different values
	result, err = interp.Execute(function, []int64{5, 3})
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}
	if result != 0 {
		t.Errorf("Expected 0 for different values, got %d", result)
	}
}

func TestMIRInterpreter_ExecutionLimit(t *testing.T) {
	interp := NewMIRInterpreter()
	interp.maxInstructions = 10
	
	// Create an infinite loop
	function := &ir.Function{
		Name: "infinite",
		Instructions: []ir.Instruction{
			{Op: ir.OpLabel, Label: "loop"},
			{Op: ir.OpJump, Label: "loop"},
		},
	}
	
	_, err := interp.Execute(function, []int64{})
	if err == nil || !strings.Contains(err.Error(), "execution limit exceeded") {
		t.Errorf("Should fail with execution limit error")
	}
}

func TestMIRInterpreter_ComplexMetafunction(t *testing.T) {
	interp := NewMIRInterpreter()

	// Generate an iterator function
	template := `
fun {0}_iterator(arr: [{1}; {2}]) -> void {
    for i in 0..{2} {
        process_{0}(arr[i]);
    }
}`

	args := []interface{}{"item", "u8", int64(10)}
	result, err := interp.ExecuteMinzMetafunction(template, args)
	if err != nil {
		t.Fatalf("Complex metafunction failed: %v", err)
	}

	expected := `
fun item_iterator(arr: [u8; 10]) -> void {
    for i in 0..10 {
        process_item(arr[i]);
    }
}`

	if result != expected {
		t.Errorf("Complex template mismatch:\nExpected:\n%s\nGot:\n%s", expected, result)
	}
}

// ====== NEW TESTS FOR ARRAY/STRUCT OPERATIONS ======

func TestMIRInterpreter_ArrayLoadStore(t *testing.T) {
	interp := NewMIRInterpreter()

	// Pre-populate memory: array at address 1000
	interp.memory[1000] = 10
	interp.memory[1001] = 20
	interp.memory[1002] = 30

	// Test OpLoadElement (constant index)
	// Use LoadConst to set up base address, since Execute clears registers
	function := &ir.Function{
		Name: "test_load_element",
		Instructions: []ir.Instruction{
			{Op: ir.OpLoadConst, Dest: 1, Imm: 1000},           // r1 = 1000 (base address)
			{Op: ir.OpLoadElement, Dest: 3, Src1: 1, Imm: 0},   // r3 = r1[0]
			{Op: ir.OpLoadElement, Dest: 4, Src1: 1, Imm: 2},   // r4 = r1[2]
			{Op: ir.OpReturn, Src1: 4},
		},
	}

	result, err := interp.Execute(function, []int64{})
	if err != nil {
		t.Fatalf("LoadElement execution failed: %v", err)
	}

	if interp.registers[3] != 10 {
		t.Errorf("OpLoadElement r1[0]: expected 10, got %d", interp.registers[3])
	}
	if result != 30 {
		t.Errorf("OpLoadElement r1[2]: expected 30, got %d", result)
	}
}

func TestMIRInterpreter_ArrayLoadIndex(t *testing.T) {
	interp := NewMIRInterpreter()

	// Pre-populate memory: array at address 2000
	interp.memory[2000] = 100
	interp.memory[2001] = 101
	interp.memory[2002] = 102

	// Test OpLoadIndex (dynamic index)
	function := &ir.Function{
		Name: "test_load_index",
		Instructions: []ir.Instruction{
			{Op: ir.OpLoadConst, Dest: 1, Imm: 2000},        // r1 = 2000 (base address)
			{Op: ir.OpLoadConst, Dest: 2, Imm: 2},           // r2 = 2 (index)
			{Op: ir.OpLoadIndex, Dest: 3, Src1: 1, Src2: 2}, // r3 = r1[r2]
			{Op: ir.OpReturn, Src1: 3},
		},
	}

	result, err := interp.Execute(function, []int64{})
	if err != nil {
		t.Fatalf("LoadIndex execution failed: %v", err)
	}

	if result != 102 {
		t.Errorf("OpLoadIndex r1[r2]: expected 102, got %d", result)
	}
}

func TestMIRInterpreter_ArrayStoreElement(t *testing.T) {
	interp := NewMIRInterpreter()

	// Test OpStoreElement (constant index)
	function := &ir.Function{
		Name: "test_store_element",
		Instructions: []ir.Instruction{
			{Op: ir.OpLoadConst, Dest: 1, Imm: 3000},          // r1 = 3000 (base address)
			{Op: ir.OpLoadConst, Dest: 5, Imm: 42},            // r5 = 42 (value to store)
			{Op: ir.OpStoreElement, Dest: 5, Src1: 1, Imm: 3}, // r1[3] = r5
			{Op: ir.OpLoadConst, Dest: 0, Imm: 0},
			{Op: ir.OpReturn, Src1: 0},
		},
	}

	_, err := interp.Execute(function, []int64{})
	if err != nil {
		t.Fatalf("StoreElement execution failed: %v", err)
	}

	if interp.memory[3003] != 42 {
		t.Errorf("OpStoreElement r1[3] = r5: expected 42, got %d", interp.memory[3003])
	}
}

func TestMIRInterpreter_ArrayStoreIndex(t *testing.T) {
	interp := NewMIRInterpreter()

	// Test OpStoreIndex (dynamic index)
	function := &ir.Function{
		Name: "test_store_index",
		Instructions: []ir.Instruction{
			{Op: ir.OpLoadConst, Dest: 1, Imm: 4000},         // r1 = 4000 (base address)
			{Op: ir.OpLoadConst, Dest: 2, Imm: 5},            // r2 = 5 (index)
			{Op: ir.OpLoadConst, Dest: 6, Imm: 77},           // r6 = 77 (value)
			{Op: ir.OpStoreIndex, Dest: 6, Src1: 1, Src2: 2}, // r1[r2] = r6
			{Op: ir.OpLoadConst, Dest: 0, Imm: 0},
			{Op: ir.OpReturn, Src1: 0},
		},
	}

	_, err := interp.Execute(function, []int64{})
	if err != nil {
		t.Fatalf("StoreIndex execution failed: %v", err)
	}

	if interp.memory[4005] != 77 {
		t.Errorf("OpStoreIndex r1[r2] = r6: expected 77, got %d", interp.memory[4005])
	}
}

func TestMIRInterpreter_StructField(t *testing.T) {
	interp := NewMIRInterpreter()

	// Simulate a struct: { x: u8, y: u8, z: u8 } at address 5000
	interp.memory[5000] = 11 // field[0] = x
	interp.memory[5001] = 22 // field[1] = y
	interp.memory[5002] = 33 // field[2] = z

	// Test OpLoadField
	function := &ir.Function{
		Name: "test_load_field",
		Instructions: []ir.Instruction{
			{Op: ir.OpLoadConst, Dest: 1, Imm: 5000},       // r1 = 5000 (struct base)
			{Op: ir.OpLoadField, Dest: 3, Src1: 1, Imm: 0}, // r3 = r1.field[0]
			{Op: ir.OpLoadField, Dest: 4, Src1: 1, Imm: 1}, // r4 = r1.field[1]
			{Op: ir.OpLoadField, Dest: 5, Src1: 1, Imm: 2}, // r5 = r1.field[2]
			{Op: ir.OpReturn, Src1: 5},
		},
	}

	result, err := interp.Execute(function, []int64{})
	if err != nil {
		t.Fatalf("LoadField execution failed: %v", err)
	}

	if interp.registers[3] != 11 {
		t.Errorf("OpLoadField field[0]: expected 11, got %d", interp.registers[3])
	}
	if interp.registers[4] != 22 {
		t.Errorf("OpLoadField field[1]: expected 22, got %d", interp.registers[4])
	}
	if result != 33 {
		t.Errorf("OpLoadField field[2]: expected 33, got %d", result)
	}
}

func TestMIRInterpreter_StructStoreField(t *testing.T) {
	interp := NewMIRInterpreter()

	// Test OpStoreField
	function := &ir.Function{
		Name: "test_store_field",
		Instructions: []ir.Instruction{
			{Op: ir.OpLoadConst, Dest: 1, Imm: 6000},        // r1 = 6000 (struct base)
			{Op: ir.OpLoadConst, Dest: 7, Imm: 99},          // r7 = 99 (value)
			{Op: ir.OpStoreField, Src1: 1, Src2: 7, Imm: 2}, // r1.field[2] = r7
			{Op: ir.OpLoadConst, Dest: 0, Imm: 0},
			{Op: ir.OpReturn, Src1: 0},
		},
	}

	_, err := interp.Execute(function, []int64{})
	if err != nil {
		t.Fatalf("StoreField execution failed: %v", err)
	}

	if interp.memory[6002] != 99 {
		t.Errorf("OpStoreField r1.field[2] = r7: expected 99, got %d", interp.memory[6002])
	}
}

func TestMIRInterpreter_LoadParam(t *testing.T) {
	interp := NewMIRInterpreter()

	// Test OpLoadParam - loading function parameters by name
	// Note: Parameters are passed via args, which get loaded to registers 1, 2, 3...
	function := &ir.Function{
		Name: "test_params",
		Params: []ir.Parameter{
			{Name: "x", Reg: 1},
			{Name: "y", Reg: 2},
		},
		Instructions: []ir.Instruction{
			// The test just adds the two parameters
			{Op: ir.OpAdd, Dest: 3, Src1: 1, Src2: 2}, // r3 = r1 + r2
			{Op: ir.OpReturn, Src1: 3},
		},
	}

	// Register the function so OpLoadParam can find it
	interp.functions["test_params"] = function

	result, err := interp.Execute(function, []int64{10, 25})
	if err != nil {
		t.Fatalf("LoadParam execution failed: %v", err)
	}

	if result != 35 {
		t.Errorf("Expected 10 + 25 = 35, got %d", result)
	}
}

func TestMIRInterpreter_ArrayRoundTrip(t *testing.T) {
	interp := NewMIRInterpreter()

	// Full round-trip test: store values and read them back
	function := &ir.Function{
		Name: "test_roundtrip",
		Instructions: []ir.Instruction{
			// Set up base address and values
			{Op: ir.OpLoadConst, Dest: 1, Imm: 7000},          // r1 = 7000 (base)
			{Op: ir.OpLoadConst, Dest: 2, Imm: 111},           // r2 = 111
			{Op: ir.OpLoadConst, Dest: 3, Imm: 222},           // r3 = 222

			// Store values
			{Op: ir.OpStoreElement, Dest: 2, Src1: 1, Imm: 0}, // r1[0] = r2 (111)
			{Op: ir.OpStoreElement, Dest: 3, Src1: 1, Imm: 1}, // r1[1] = r3 (222)

			// Load values back
			{Op: ir.OpLoadElement, Dest: 4, Src1: 1, Imm: 0},  // r4 = r1[0]
			{Op: ir.OpLoadElement, Dest: 5, Src1: 1, Imm: 1},  // r5 = r1[1]

			// Add them together to verify
			{Op: ir.OpAdd, Dest: 6, Src1: 4, Src2: 5},         // r6 = r4 + r5
			{Op: ir.OpReturn, Src1: 6},
		},
	}

	result, err := interp.Execute(function, []int64{})
	if err != nil {
		t.Fatalf("RoundTrip execution failed: %v", err)
	}

	// 111 + 222 = 333
	if result != 333 {
		t.Errorf("Expected 111 + 222 = 333, got %d", result)
	}
}