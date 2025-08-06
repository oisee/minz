package codegen

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	
	"github.com/minz/minzc/pkg/ir"
)

// BackendDevKit provides comprehensive tools for backend development
type BackendDevKit struct {
	*BackendToolkit
	
	// Test generation
	TestGen *BackendTestGenerator
	
	// Documentation generation
	DocGen *BackendDocGenerator
	
	// Validation tools
	Validator *BackendValidator
	
	// Common optimizations
	Optimizer *BackendOptimizer
	
	// Debugging helpers
	Debugger *BackendDebugger
}

// NewBackendDevKit creates a complete development kit
func NewBackendDevKit() *BackendDevKit {
	toolkit := NewBackendToolkit()
	return &BackendDevKit{
		BackendToolkit: toolkit,
		TestGen:        NewBackendTestGenerator(),
		DocGen:         NewBackendDocGenerator(),
		Validator:      NewBackendValidator(),
		Optimizer:      NewBackendOptimizer(),
		Debugger:       NewBackendDebugger(),
	}
}

// BackendTestGenerator generates test cases for backends
type BackendTestGenerator struct {
	testTemplates map[string]string
}

func NewBackendTestGenerator() *BackendTestGenerator {
	return &BackendTestGenerator{
		testTemplates: map[string]string{
			"arithmetic": `
// Test basic arithmetic
fun test_arithmetic() -> u8 {
    let a: u8 = 10;
    let b: u8 = 20;
    let c: u8 = a + b;
    let d: u8 = b - a;
    let e: u8 = a * 2;
    return e;
}`,
			"control_flow": `
// Test control flow
fun test_control_flow(x: u8) -> u8 {
    if x > 10 {
        return x * 2;
    } else {
        return x + 5;
    }
}`,
			"loops": `
// Test loops
fun test_loops() -> u8 {
    let sum: u8 = 0;
    for i in 0..10 {
        sum = sum + i;
    }
    return sum;
}`,
			"memory": `
// Test memory operations
fun test_memory() -> u8 {
    let arr: [u8; 5] = [1, 2, 3, 4, 5];
    arr[2] = 10;
    return arr[2];
}`,
			"functions": `
// Test function calls
fun helper(x: u8) -> u8 {
    return x * 2;
}

fun test_functions() -> u8 {
    let result = helper(21);
    return result;
}`,
		},
	}
}

// GenerateTestSuite generates a comprehensive test suite for a backend
func (tg *BackendTestGenerator) GenerateTestSuite(backendName string) string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("// Test suite for %s backend\n\n", backendName))
	
	for name, code := range tg.testTemplates {
		buf.WriteString(fmt.Sprintf("// === %s test ===\n", name))
		buf.WriteString(code)
		buf.WriteString("\n\n")
	}
	
	buf.WriteString(`fun main() -> void {
    // Run all tests
    let r1 = test_arithmetic();
    let r2 = test_control_flow(15);
    let r3 = test_loops();
    let r4 = test_memory();
    let r5 = test_functions();
}`)
	
	return buf.String()
}

// GenerateExpectedOutput generates expected assembly patterns
func (tg *BackendTestGenerator) GenerateExpectedOutput(backend Backend) map[string][]string {
	expected := make(map[string][]string)
	
	// Expected patterns for each test
	expected["arithmetic"] = []string{
		"load.*10",  // Load constant 10
		"load.*20",  // Load constant 20
		"add",       // Addition
		"sub",       // Subtraction
		"mul|shift", // Multiplication (might be shifts)
	}
	
	expected["control_flow"] = []string{
		"cmp|test",  // Comparison
		"j[gnlze]",  // Conditional jump
		"ret",       // Return
	}
	
	expected["loops"] = []string{
		"loop|djnz|dec.*j", // Loop pattern
	}
	
	return expected
}

// BackendDocGenerator generates documentation for backends
type BackendDocGenerator struct {
	templates map[string]*template.Template
}

func NewBackendDocGenerator() *BackendDocGenerator {
	dg := &BackendDocGenerator{
		templates: make(map[string]*template.Template),
	}
	
	// README template
	readmeTmpl := `# {{.Name}} Backend for MinZ

## Overview
The {{.Name}} backend generates code for {{.Description}}.

## Features
{{range .Features}}
- {{.}}{{end}}

## Supported Instructions
{{range $op, $asm := .Instructions}}
- {{$op}}: {{$asm}}{{end}}

## Register Mapping
| Virtual | Physical |
|---------|----------|
{{range $v, $p := .Registers}}| r{{$v}} | {{$p}} |
{{end}}

## Calling Convention
- Parameter Passing: {{.CallingConvention.Style}}
- Return Register: {{.CallingConvention.ReturnReg}}

## Usage
` + "```" + `bash
minzc program.minz -b {{.ShortName}} -o program{{.Extension}}
` + "```" + `

## Examples
See test files in examples/backend_tests/{{.ShortName}}/
`
	
	dg.templates["readme"] = template.Must(template.New("readme").Parse(readmeTmpl))
	
	return dg
}

// GenerateREADME generates a README for a backend
func (dg *BackendDocGenerator) GenerateREADME(backend Backend, toolkit *BackendToolkit) string {
	data := struct {
		Name              string
		ShortName         string
		Description       string
		Features          []string
		Instructions      map[ir.Opcode]string
		Registers         map[ir.Register]string
		CallingConvention struct {
			Style     string
			ReturnReg string
		}
		Extension string
	}{
		Name:         strings.ToUpper(backend.Name()),
		ShortName:    backend.Name(),
		Description:  "the " + backend.Name() + " architecture",
		Features:     dg.getBackendFeatures(backend),
		Instructions: toolkit.OpToAsm,
		Registers:    toolkit.VirtualToPhysical,
		CallingConvention: struct {
			Style     string
			ReturnReg string
		}{
			Style:     toolkit.Patterns.ParamPassingStyle,
			ReturnReg: toolkit.Patterns.ReturnValueReg,
		},
		Extension: backend.GetFileExtension(),
	}
	
	var buf bytes.Buffer
	dg.templates["readme"].Execute(&buf, data)
	return buf.String()
}

func (dg *BackendDocGenerator) getBackendFeatures(backend Backend) []string {
	features := []string{}
	featureNames := map[string]string{
		FeatureSelfModifyingCode: "Self-modifying code optimization",
		Feature16BitPointers:     "16-bit pointers",
		FeatureZeroPage:          "Zero-page optimization",
		FeatureInterrupts:        "Interrupt support",
		FeatureShadowRegisters:   "Shadow registers",
	}
	
	for feature, name := range featureNames {
		if backend.SupportsFeature(feature) {
			features = append(features, name)
		}
	}
	
	return features
}

// BackendValidator validates backend implementations
type BackendValidator struct {
	requiredOps []ir.Opcode
}

func NewBackendValidator() *BackendValidator {
	return &BackendValidator{
		requiredOps: []ir.Opcode{
			ir.OpLoadConst,
			ir.OpLoadVar,
			ir.OpStoreVar,
			ir.OpAdd,
			ir.OpSub,
			ir.OpCall,
			ir.OpReturn,
			ir.OpJump,
			ir.OpJumpIf,
			ir.OpLabel,
		},
	}
}

// ValidateBackend checks if a backend implements required functionality
func (v *BackendValidator) ValidateBackend(backend Backend, toolkit *BackendToolkit) []string {
	errors := []string{}
	
	// Check required opcodes
	for _, op := range v.requiredOps {
		if _, hasMapping := toolkit.OpToAsm[op]; !hasMapping {
			// Check if pattern exists
			hasPattern := false
			switch op {
			case ir.OpLoadConst, ir.OpLoadVar:
				hasPattern = toolkit.Patterns.LoadPattern != ""
			case ir.OpStoreVar:
				hasPattern = toolkit.Patterns.StorePattern != ""
			case ir.OpAdd:
				hasPattern = toolkit.Patterns.AddPattern != ""
			case ir.OpSub:
				hasPattern = toolkit.Patterns.SubPattern != ""
			}
			
			if !hasPattern {
				errors = append(errors, fmt.Sprintf("Missing implementation for %s", op))
			}
		}
	}
	
	// Check calling convention
	if toolkit.Patterns.ParamPassingStyle == "" {
		errors = append(errors, "No calling convention specified")
	}
	
	// Check register mappings
	if len(toolkit.VirtualToPhysical) == 0 {
		errors = append(errors, "No register mappings defined")
	}
	
	return errors
}

// BackendOptimizer provides common optimization patterns
type BackendOptimizer struct {
	patterns []OptimizationPattern
}

type OptimizationPattern struct {
	Name        string
	Description string
	Match       func([]ir.Instruction) (bool, int) // Returns match and length
	Transform   func([]ir.Instruction) []ir.Instruction
}

func NewBackendOptimizer() *BackendOptimizer {
	return &BackendOptimizer{
		patterns: []OptimizationPattern{
			{
				Name:        "constant_folding",
				Description: "Fold constant arithmetic at compile time",
				Match: func(insts []ir.Instruction) (bool, int) {
					if len(insts) >= 3 &&
						insts[0].Op == ir.OpLoadConst &&
						insts[1].Op == ir.OpLoadConst &&
						(insts[2].Op == ir.OpAdd || insts[2].Op == ir.OpSub) &&
						insts[2].Src1 == insts[0].Dest &&
						insts[2].Src2 == insts[1].Dest {
						return true, 3
					}
					return false, 0
				},
				Transform: func(insts []ir.Instruction) []ir.Instruction {
					val1 := insts[0].Imm
					val2 := insts[1].Imm
					var result int64
					
					if insts[2].Op == ir.OpAdd {
						result = val1 + val2
					} else {
						result = val1 - val2
					}
					
					return []ir.Instruction{
						{
							Op:   ir.OpLoadConst,
							Dest: insts[2].Dest,
							Imm:  result,
							Type: insts[2].Type,
						},
					}
				},
			},
			{
				Name:        "dead_store_elimination",
				Description: "Remove stores that are immediately overwritten",
				Match: func(insts []ir.Instruction) (bool, int) {
					if len(insts) >= 2 &&
						insts[0].Op == ir.OpStoreVar &&
						insts[1].Op == ir.OpStoreVar &&
						insts[0].Symbol == insts[1].Symbol {
						return true, 2
					}
					return false, 0
				},
				Transform: func(insts []ir.Instruction) []ir.Instruction {
					// Keep only the second store
					return []ir.Instruction{insts[1]}
				},
			},
		},
	}
}

// BackendDebugger provides debugging helpers
type BackendDebugger struct {
	traceEnabled bool
	breakpoints  map[string]bool
}

func NewBackendDebugger() *BackendDebugger {
	return &BackendDebugger{
		breakpoints: make(map[string]bool),
	}
}

// GenerateDebugCode adds debug instrumentation to generated code
func (d *BackendDebugger) GenerateDebugCode(inst *ir.Instruction, backend string) string {
	if !d.traceEnabled {
		return ""
	}
	
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("; DEBUG: %s\n", inst.Op))
	
	// Add breakpoint if requested based on comment
	if inst.Comment != "" && d.breakpoints[inst.Comment] {
		switch backend {
		case "z80":
			buf.WriteString("    RST 38h  ; Breakpoint\n")
		case "6502":
			buf.WriteString("    BRK      ; Breakpoint\n")
		case "68000":
			buf.WriteString("    TRAP #15 ; Breakpoint\n")
		default:
			buf.WriteString("    ; BREAKPOINT\n")
		}
	}
	
	return buf.String()
}

// CreateBackendScaffold creates a new backend from scratch
func CreateBackendScaffold(name, description string) string {
	tmpl := `package codegen

import (
	"fmt"
	"github.com/minz/minzc/pkg/ir"
)

// {{.Name}}Backend implements code generation for {{.Description}}
type {{.Name}}Backend struct {
	options *BackendOptions
	toolkit *BackendToolkit
}

// New{{.Name}}Backend creates a new {{.Description}} backend
func New{{.Name}}Backend(options *BackendOptions) Backend {
	toolkit := NewBackendBuilder().
		// TODO: Add instruction mappings
		WithInstruction(ir.OpNop, "nop").
		
		// TODO: Define patterns
		WithPattern("load", "    ; TODO: load %reg%, %addr%").
		WithPattern("store", "    ; TODO: store %reg%, %addr%").
		
		// TODO: Set calling convention
		WithCallConvention("stack", "").
		
		// TODO: Map registers
		WithRegisterMapping(1, "r1").
		Build()
	
	return &{{.Name}}Backend{
		options: options,
		toolkit: toolkit,
	}
}

func (b *{{.Name}}Backend) Name() string {
	return "{{.ShortName}}"
}

func (b *{{.Name}}Backend) Generate(module *ir.Module) (string, error) {
	gen := NewBaseGenerator(b, module, b.toolkit)
	return gen.Generate()
}

func (b *{{.Name}}Backend) GetFileExtension() string {
	return ".s"
}

func (b *{{.Name}}Backend) SupportsFeature(feature string) bool {
	switch feature {
	case FeatureSelfModifyingCode:
		return false
	case Feature16BitPointers:
		return true
	default:
		return false
	}
}
`

	t := template.Must(template.New("scaffold").Parse(tmpl))
	
	data := struct {
		Name        string
		ShortName   string
		Description string
	}{
		Name:        strings.Title(name),
		ShortName:   strings.ToLower(name),
		Description: description,
	}
	
	var buf bytes.Buffer
	t.Execute(&buf, data)
	return buf.String()
}