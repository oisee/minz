package codegen

import (
	"fmt"
	
	"github.com/minz/minzc/pkg/ir"
)

// ExampleBackend demonstrates how to use the BackendToolkit
// This example creates a simple RISC-style backend
type ExampleBackend struct {
	options *BackendOptions
	toolkit *BackendToolkit
}

// NewExampleBackend creates a new example backend using the toolkit
func NewExampleBackend(options *BackendOptions) Backend {
	// Build the backend configuration using the fluent builder
	toolkit := NewBackendBuilder().
		// Map common instructions
		WithInstruction(ir.OpNop, "nop").
		
		// Define patterns for complex operations
		WithPattern("load", "    load %reg%, %addr%").
		WithPattern("store", "    store %reg%, %addr%").
		WithPattern("add", "    add %dest%, %src1%, %src2%").
		WithPattern("sub", "    sub %dest%, %src1%, %src2%").
		
		// Set up function conventions
		WithPattern("prologue", "    ; function prologue").
		WithPattern("epilogue", "    ; function epilogue").
		WithCallConvention("registers", "r0").
		
		// Map virtual registers to physical ones
		WithRegisterMapping(1, "r0").
		WithRegisterMapping(2, "r1").
		WithRegisterMapping(3, "r2").
		WithRegisterMapping(4, "r3").
		Build()
	
	return &ExampleBackend{
		options: options,
		toolkit: toolkit,
	}
}

func (b *ExampleBackend) Name() string {
	return "example"
}

func (b *ExampleBackend) Generate(module *ir.Module) (string, error) {
	// Use the base generator with our toolkit
	gen := NewBaseGenerator(b, module, b.toolkit)
	return gen.Generate()
}

func (b *ExampleBackend) GetFileExtension() string {
	return ".s"
}

func (b *ExampleBackend) SupportsFeature(feature string) bool {
	switch feature {
	case FeatureSelfModifyingCode:
		return false
	case Feature16BitPointers:
		return true
	default:
		return false
	}
}

// More complex example: 8051 microcontroller backend
type Intel8051Backend struct {
	options *BackendOptions
	toolkit *BackendToolkit
}

func NewIntel8051Backend(options *BackendOptions) Backend {
	toolkit := NewBackendBuilder().
		// 8051 specific instructions
		WithInstruction(ir.OpLoadConst, "mov a, #%value%").
		
		// Patterns for memory access (8051 style)
		WithPattern("load", "    mov a, %addr%").
		WithPattern("store", "    mov %addr%, a").
		WithPattern("add", "    add a, %src2%").
		WithPattern("sub", "    subb a, %src2%").
		
		// 8051 calling convention (accumulator-based)
		WithCallConvention("stack", "a").
		
		// Register mappings (8051 has limited registers)
		WithRegisterMapping(1, "a").    // Accumulator
		WithRegisterMapping(2, "b").    // B register
		WithRegisterMapping(3, "r0").   // General purpose
		WithRegisterMapping(4, "r1").
		WithRegisterMapping(5, "r2").
		WithRegisterMapping(6, "r3").
		WithRegisterMapping(7, "r4").
		WithRegisterMapping(8, "r5").
		Build()
	
	// Customize type sizes for 8051
	toolkit.TypeSizes["ptr"] = 2  // 16-bit pointers on 8051
	
	return &Intel8051Backend{
		options: options,
		toolkit: toolkit,
	}
}

func (b *Intel8051Backend) Name() string {
	return "8051"
}

func (b *Intel8051Backend) Generate(module *ir.Module) (string, error) {
	gen := NewIntel8051Generator(b, module, b.toolkit)
	return gen.Generate()
}

func (b *Intel8051Backend) GetFileExtension() string {
	return ".a51"
}

func (b *Intel8051Backend) SupportsFeature(feature string) bool {
	switch feature {
	case FeatureSelfModifyingCode:
		return false // 8051 has Harvard architecture
	case Feature16BitPointers:
		return true
	case FeatureInterrupts:
		return true
	case "bit_addressable":
		return true // 8051 special feature
	default:
		return false
	}
}

// Custom generator for 8051 to handle its unique features
type Intel8051Generator struct {
	*BaseGenerator
}

func NewIntel8051Generator(backend Backend, module *ir.Module, toolkit *BackendToolkit) *Intel8051Generator {
	return &Intel8051Generator{
		BaseGenerator: NewBaseGenerator(backend, module, toolkit),
	}
}

// Override to add 8051-specific header
func (g *Intel8051Generator) EmitHeader() {
	g.BaseGenerator.EmitHeader()
	g.EmitLine("    .area CODE")
	g.EmitLine("    .area DATA")
	g.EmitLine("")
}

// Override to handle 8051's accumulator-centric architecture
func (g *Intel8051Generator) GenerateArithmetic(inst *ir.Instruction, pattern string) error {
	// 8051 arithmetic always uses accumulator
	// Load first operand to accumulator
	g.EmitLine(fmt.Sprintf("    mov a, %s", g.GetPhysicalReg(inst.Src1)))
	
	// Apply operation with second operand
	g.EmitLine(fmt.Sprintf(pattern, g.GetPhysicalReg(inst.Src2)))
	
	// Store result
	if inst.Dest != 1 { // If dest is not accumulator
		g.EmitLine(fmt.Sprintf("    mov %s, a", g.GetPhysicalReg(inst.Dest)))
	}
	
	return nil
}