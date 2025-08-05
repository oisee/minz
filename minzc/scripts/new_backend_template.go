// Template for creating a new MinZ backend
// Copy this file and replace ARCH with your architecture name

package codegen

import (
	"fmt"
	"github.com/minz/minzc/pkg/ir"
)

// ARCHBackend implements the Backend interface for ARCH code generation
type ARCHBackend struct {
	options *BackendOptions
	toolkit *BackendToolkit
}

// NewARCHBackend creates a new ARCH backend
func NewARCHBackend(options *BackendOptions) Backend {
	// Configure your backend using the toolkit builder
	toolkit := NewBackendBuilder().
		// === BASIC INSTRUCTIONS ===
		// Map simple MIR opcodes to assembly instructions
		WithInstruction(ir.OpNop, "nop").
		// WithInstruction(ir.OpHalt, "halt").
		// WithInstruction(ir.OpBreak, "break").
		
		// === PATTERNS ===
		// Define patterns for complex operations
		// Use placeholders: %reg%, %dest%, %src1%, %src2%, %addr%, %value%, %label%
		
		// Memory operations
		WithPattern("load", "    LOAD_INSTRUCTION %reg%, %addr%").
		WithPattern("store", "    STORE_INSTRUCTION %reg%, %addr%").
		
		// Arithmetic operations
		WithPattern("add", "    ADD_INSTRUCTION %dest%, %src1%, %src2%").
		WithPattern("sub", "    SUB_INSTRUCTION %dest%, %src1%, %src2%").
		// WithPattern("mul", "    MUL_INSTRUCTION %dest%, %src1%, %src2%").
		// WithPattern("div", "    DIV_INSTRUCTION %dest%, %src1%, %src2%").
		
		// Function prologue/epilogue
		WithPattern("prologue", "    ; TODO: Save registers/setup frame").
		WithPattern("epilogue", "    ; TODO: Restore registers/cleanup").
		
		// === CALLING CONVENTION ===
		// Options: "stack", "registers", "zero-page"
		WithCallConvention("stack", "RETURN_REGISTER").
		
		// === REGISTER MAPPING ===
		// Map virtual registers (r1, r2, ...) to physical registers
		WithRegisterMapping(1, "REG0").
		WithRegisterMapping(2, "REG1").
		WithRegisterMapping(3, "REG2").
		WithRegisterMapping(4, "REG3").
		// Add more as needed...
		
		Build()
	
	// Customize type sizes if different from defaults
	// toolkit.TypeSizes["ptr"] = 3  // For 24-bit pointers
	// toolkit.TypeSizes["u32"] = 4  // Should be 4 anyway
	
	return &ARCHBackend{
		options: options,
		toolkit: toolkit,
	}
}

// Name returns the name of this backend
func (b *ARCHBackend) Name() string {
	return "ARCH"
}

// Generate generates ARCH assembly code
func (b *ARCHBackend) Generate(module *ir.Module) (string, error) {
	// Option 1: Use the base generator (recommended for simple backends)
	gen := NewBaseGenerator(b, module, b.toolkit)
	return gen.Generate()
	
	// Option 2: Use a custom generator (for complex backends)
	// gen := NewARCHGenerator(b, module, b.toolkit)
	// return gen.Generate()
}

// GetFileExtension returns the file extension for ARCH assembly
func (b *ARCHBackend) GetFileExtension() string {
	return ".s"  // Change to your platform's convention (.asm, .a51, etc.)
}

// SupportsFeature returns whether this backend supports a specific feature
func (b *ARCHBackend) SupportsFeature(feature string) bool {
	switch feature {
	case FeatureSelfModifyingCode:
		return false  // Set to true if ARCH supports SMC
	case FeatureInterrupts:
		return false  // Set to true if ARCH has interrupts
	case FeatureShadowRegisters:
		return false  // Set to true if ARCH has shadow registers
	case Feature16BitPointers:
		return true   // Most common
	case Feature24BitPointers:
		return false  // Set to true for 24-bit address space
	case Feature32BitPointers:
		return false  // Set to true for 32-bit address space
	case FeatureFloatingPoint:
		return false  // Set to true if ARCH has FPU
	case FeatureFixedPoint:
		return false  // Set to true if ARCH has fixed-point
	// Add custom features:
	// case "custom_feature":
	//     return true
	default:
		return false
	}
}

// Register the backend
func init() {
	RegisterBackend("ARCH", func(options *BackendOptions) Backend {
		return NewARCHBackend(options)
	})
}

// === OPTIONAL: Custom Generator ===
// Use this if you need more control over code generation

/*
type ARCHGenerator struct {
	*BaseGenerator
	// Add custom state here
}

func NewARCHGenerator(backend Backend, module *ir.Module, toolkit *BackendToolkit) *ARCHGenerator {
	return &ARCHGenerator{
		BaseGenerator: NewBaseGenerator(backend, module, toolkit),
	}
}

// Override specific methods as needed
func (g *ARCHGenerator) GenerateFunction(fn *ir.Function) error {
	// Custom function generation
	g.EmitComment(fmt.Sprintf("Function: %s", fn.Name))
	
	// Add custom logic here
	
	// Call base implementation
	return g.BaseGenerator.GenerateFunction(fn)
}

// Example: Handle special ARCH instructions
func (g *ARCHGenerator) GenerateInstruction(inst *ir.Instruction) error {
	switch inst.Op {
	case ir.OpCustom:
		// Handle ARCH-specific operation
		g.EmitLine("    SPECIAL_INSTRUCTION")
		return nil
	default:
		// Use base implementation
		return g.BaseGenerator.GenerateInstruction(inst)
	}
}
*/

// === QUICK START GUIDE ===
//
// 1. Copy this file to pkg/codegen/ARCH_backend.go
// 2. Replace all instances of "ARCH" with your architecture name
// 3. Update instruction mappings and patterns
// 4. Set correct feature support flags
// 5. Test with: ./minzc -b ARCH test.minz -o test.s
//
// === EXAMPLES ===
//
// For 8-bit accumulator-based CPU:
//   WithPattern("add", "    ADD %src2%")  // Implicit accumulator
//   WithCallConvention("stack", "A")      // Return in accumulator
//
// For RISC CPU:
//   WithPattern("add", "    add %dest%, %src1%, %src2%")
//   WithCallConvention("registers", "r0")  // Return in r0
//
// For CISC CPU:
//   WithPattern("add", "    add %dest%, %src2%")  // Dest is also src1
//   WithCallConvention("stack", "eax")     // x86 style
//