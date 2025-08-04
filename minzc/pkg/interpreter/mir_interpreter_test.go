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