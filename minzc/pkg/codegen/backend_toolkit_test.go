package codegen

import (
	"strings"
	"testing"
	
	"github.com/minz/minzc/pkg/ir"
)

// TestBackendToolkit tests the backend development toolkit
func TestBackendToolkit(t *testing.T) {
	// Create a simple test backend using the toolkit
	toolkit := NewBackendBuilder().
		WithInstruction(ir.OpNop, "nop").
		WithInstruction(ir.OpLoadConst, "li r%d, %d").
		WithPattern("load", "    lw %reg%, %addr%").
		WithPattern("store", "    sw %reg%, %addr%").
		WithPattern("add", "    add %dest%, %src1%, %src2%").
		WithCallConvention("registers", "r0").
		WithRegisterMapping(1, "r0").
		WithRegisterMapping(2, "r1").
		Build()
	
	// Create a simple module
	module := &ir.Module{
		Name: "test",
		Functions: []*ir.Function{
			{
				Name: "main",
				Instructions: []ir.Instruction{
					{Op: ir.OpLoadConst, Dest: 1, Imm: 42},
					{Op: ir.OpReturn, Src1: 1},
				},
			},
		},
	}
	
	// Create mock backend
	backend := &mockBackend{name: "test"}
	
	// Generate code
	gen := NewBaseGenerator(backend, module, toolkit)
	output, err := gen.Generate()
	if err != nil {
		t.Fatalf("Failed to generate code: %v", err)
	}
	
	// Verify output contains expected instructions
	if !strings.Contains(output, "main:") {
		t.Error("Output should contain main label")
	}
	
	// Note: The simple instruction mapping doesn't support the format string
	// For real use, we'd need a more sophisticated formatter
	t.Logf("Generated output:\n%s", output)
}

// TestBackendBuilder tests the fluent builder interface
func TestBackendBuilder(t *testing.T) {
	toolkit := NewBackendBuilder().
		WithInstruction(ir.OpAdd, "add").
		WithInstruction(ir.OpSub, "sub").
		WithPattern("mul", "mul %dest%, %src1%, %src2%").
		WithCallConvention("stack", "a0").
		Build()
	
	// Verify instructions were added
	if toolkit.OpToAsm[ir.OpAdd] != "add" {
		t.Error("Add instruction not mapped correctly")
	}
	
	if toolkit.OpToAsm[ir.OpSub] != "sub" {
		t.Error("Sub instruction not mapped correctly")
	}
	
	// Verify patterns
	if toolkit.Patterns.ParamPassingStyle != "stack" {
		t.Error("Call convention not set correctly")
	}
	
	if toolkit.Patterns.ReturnValueReg != "a0" {
		t.Error("Return register not set correctly")
	}
}

// TestRealWorldExample tests a more realistic backend configuration
func TestRealWorldExample(t *testing.T) {
	// Simulate creating an 8051-style backend
	toolkit := NewBackendBuilder().
		// 8051 uses accumulator for most operations
		WithInstruction(ir.OpLoadConst, "mov a, #%value%").
		WithPattern("add", "    add a, %src2%").
		WithPattern("store", "    mov %addr%, a").
		WithCallConvention("stack", "a").
		WithRegisterMapping(1, "a").
		WithRegisterMapping(2, "b").
		WithRegisterMapping(3, "r0").
		Build()
	
	// Verify accumulator-based architecture
	if toolkit.Patterns.ReturnValueReg != "a" {
		t.Error("8051 should return in accumulator")
	}
	
	if toolkit.VirtualToPhysical[1] != "a" {
		t.Error("Register 1 should map to accumulator")
	}
}

// mockBackend for testing
type mockBackend struct {
	name string
}

func (b *mockBackend) Name() string { return b.name }
func (b *mockBackend) Generate(module *ir.Module) (string, error) { return "", nil }
func (b *mockBackend) GetFileExtension() string { return ".s" }
func (b *mockBackend) SupportsFeature(feature string) bool { return false }