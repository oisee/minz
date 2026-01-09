// Package semantic tests for MIR code generation
package semantic

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/ir"
	"github.com/minz/minzc/pkg/mirvm"
	"github.com/minz/minzc/pkg/parser"
)

// compileToMIR compiles MinZ source to MIR module
func compileToMIR(t *testing.T, source string) *ir.Module {
	t.Helper()

	// Parse
	p := parser.New()
	decls, err := p.ParseString(source, "test.minz")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Create AST file
	astFile := &ast.File{
		Name:         "test.minz",
		Declarations: decls,
	}

	// Analyze with MIR backend
	analyzer := NewAnalyzer()
	analyzer.SetTargetBackend("mir")
	module, err := analyzer.Analyze(astFile)
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}

	return module
}

// findInstruction finds the first instruction matching the predicate
func findInstruction(fn *ir.Function, predicate func(ir.Instruction) bool) (int, *ir.Instruction) {
	for i, inst := range fn.Instructions {
		if predicate(inst) {
			return i, &fn.Instructions[i]
		}
	}
	return -1, nil
}

// findLabel finds the index where a label is defined
func findLabel(fn *ir.Function, label string) int {
	for i, inst := range fn.Instructions {
		if inst.Op == ir.OpLabel && inst.Label == label {
			return i
		}
	}
	return -1
}

// TestWhileLoopLabelPlacement tests that while loop labels are placed correctly
// The label should come AFTER variable initialization, not before
func TestWhileLoopLabelPlacement(t *testing.T) {
	source := `
fun main() -> void {
    let x: u8 = 0;
    while x < 10 {
        x = x + 1;
    }
}
`
	module := compileToMIR(t, source)

	// Find main function
	var mainFn *ir.Function
	for _, fn := range module.Functions {
		if strings.HasSuffix(fn.Name, ".main") || fn.Name == "main" {
			mainFn = fn
			break
		}
	}
	if mainFn == nil {
		t.Fatal("Could not find main function")
	}

	// Find the first loop label
	labelIdx := -1
	for i, inst := range mainFn.Instructions {
		if inst.Op == ir.OpLabel && strings.HasPrefix(inst.Label, "loop") {
			labelIdx = i
			break
		}
	}
	if labelIdx < 0 {
		t.Fatal("Could not find loop label")
	}

	// Find the store instruction for x (initialization)
	storeIdx := -1
	for i, inst := range mainFn.Instructions {
		if inst.Op == ir.OpStoreVar && inst.Symbol == "x" {
			storeIdx = i
			break
		}
	}
	if storeIdx < 0 {
		t.Fatal("Could not find store instruction for x")
	}

	// Print instructions for debugging
	t.Log("Instructions:")
	for i, inst := range mainFn.Instructions {
		t.Logf("  %d: Op=%v Label=%s Symbol=%s %s", i, inst.Op, inst.Label, inst.Symbol, inst.String())
	}
	t.Logf("labelIdx=%d, storeIdx=%d", labelIdx, storeIdx)

	// The store should come BEFORE the loop label
	// BUG: Currently the label comes before the store
	if storeIdx > labelIdx {
		t.Errorf("Store for x (idx %d) should come before loop label (idx %d)\n"+
			"This indicates the loop label is incorrectly placed before variable initialization",
			storeIdx, labelIdx)
	}
}

// TestWhileLoopExecution tests that while loops execute correctly
func TestWhileLoopExecution(t *testing.T) {
	source := `
fun main() -> void {
    let count: u8 = 0;
    while count < 10 {
        count = count + 1;
    }
}
`
	module := compileToMIR(t, source)

	// Run in VM
	config := mirvm.Config{
		MemorySize: 4096,
		StackSize:  1024,
		MaxSteps:   10000,
	}
	vm := mirvm.New(config)
	if err := vm.LoadModule(module); err != nil {
		t.Fatalf("Failed to load module: %v", err)
	}

	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	// Check that count = 10 after loop completes
	stats := vm.GetStatistics()
	t.Logf("Instructions executed: %d", stats.InstructionsExecuted)

	// The loop should have executed ~10 times, not infinitely
	// If it ran more than 1000 instructions, something is wrong
	if stats.InstructionsExecuted > 1000 {
		t.Errorf("Loop executed %d instructions, expected < 1000. Likely infinite loop bug.",
			stats.InstructionsExecuted)
	}
}

// TestNestedWhileLoopExecution tests nested while loops
func TestNestedWhileLoopExecution(t *testing.T) {
	source := `
fun main() -> void {
    let total: u8 = 0;
    let i: u8 = 0;
    while i < 3 {
        let j: u8 = 0;
        while j < 4 {
            total = total + 1;
            j = j + 1;
        }
        i = i + 1;
    }
}
`
	module := compileToMIR(t, source)

	// Run in VM
	config := mirvm.Config{
		MemorySize: 4096,
		StackSize:  1024,
		MaxSteps:   100000, // Higher limit for nested loops
	}
	vm := mirvm.New(config)
	if err := vm.LoadModule(module); err != nil {
		t.Fatalf("Failed to load module: %v", err)
	}

	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	stats := vm.GetStatistics()
	t.Logf("Instructions executed: %d", stats.InstructionsExecuted)

	// 3 * 4 = 12 iterations of inner loop
	// Should complete reasonably quickly, not run into limit
	if stats.InstructionsExecuted > 5000 {
		t.Errorf("Nested loops executed %d instructions, expected < 5000. Likely bug.",
			stats.InstructionsExecuted)
	}
}

// TestForLoopExecution tests for-range loops
func TestForLoopExecution(t *testing.T) {
	source := `
fun main() -> void {
    let sum: u8 = 0;
    for i in 0..10 {
        sum = sum + 1;
    }
}
`
	module := compileToMIR(t, source)

	// Run in VM
	config := mirvm.Config{
		MemorySize: 4096,
		StackSize:  1024,
		MaxSteps:   10000,
	}
	vm := mirvm.New(config)
	if err := vm.LoadModule(module); err != nil {
		t.Fatalf("Failed to load module: %v", err)
	}

	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	stats := vm.GetStatistics()
	t.Logf("Instructions executed: %d", stats.InstructionsExecuted)

	if stats.InstructionsExecuted > 1000 {
		t.Errorf("For loop executed %d instructions, expected < 1000. Likely bug.",
			stats.InstructionsExecuted)
	}
}

// TestVariableInitialization tests that variables are properly initialized
func TestVariableInitialization(t *testing.T) {
	source := `
fun main() -> void {
    let a: u8 = 42;
    let b: u8 = a + 1;
}
`
	module := compileToMIR(t, source)

	// Run in VM
	config := mirvm.Config{
		MemorySize: 4096,
		StackSize:  1024,
		MaxSteps:   1000,
	}
	vm := mirvm.New(config)
	if err := vm.LoadModule(module); err != nil {
		t.Fatalf("Failed to load module: %v", err)
	}

	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}
}

// TestConditionalExecution tests if-else execution
func TestConditionalExecution(t *testing.T) {
	source := `
fun main() -> void {
    let result: u8 = 0;
    let cond: u8 = 1;
    if cond > 0 {
        result = 42;
    } else {
        result = 99;
    }
}
`
	module := compileToMIR(t, source)

	// Run in VM
	config := mirvm.Config{
		MemorySize: 4096,
		StackSize:  1024,
		MaxSteps:   1000,
	}
	vm := mirvm.New(config)
	if err := vm.LoadModule(module); err != nil {
		t.Fatalf("Failed to load module: %v", err)
	}

	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}
}

// TestSetPixelSyscall tests the set_pixel syscall generation
func TestSetPixelSyscall(t *testing.T) {
	source := `
fun main() -> void {
    set_pixel(10, 20, 255);
}
`
	module := compileToMIR(t, source)

	// Check that syscall instruction is generated
	var mainFn *ir.Function
	for _, fn := range module.Functions {
		if strings.HasSuffix(fn.Name, ".main") || fn.Name == "main" {
			mainFn = fn
			break
		}
	}
	if mainFn == nil {
		t.Fatal("Could not find main function")
	}

	// Find syscall instruction
	foundSyscall := false
	for _, inst := range mainFn.Instructions {
		if inst.Op == ir.OpSyscall && inst.Imm == 10 { // syscall 10 = set_pixel
			foundSyscall = true
			break
		}
	}
	if !foundSyscall {
		t.Error("Expected syscall 10 (set_pixel) instruction")
		t.Log("Instructions:")
		for i, inst := range mainFn.Instructions {
			t.Logf("  %d: %s", i, inst.String())
		}
	}
}

// TestArithmetic tests basic arithmetic operations
func TestArithmetic(t *testing.T) {
	source := `
fun main() -> void {
    let a: u8 = 10;
    let b: u8 = 3;
    let sum: u8 = a + b;
    let diff: u8 = a - b;
    let prod: u8 = a * b;
    let quot: u8 = a / b;
}
`
	module := compileToMIR(t, source)

	// Run in VM
	config := mirvm.Config{
		MemorySize: 4096,
		StackSize:  1024,
		MaxSteps:   1000,
	}
	vm := mirvm.New(config)
	if err := vm.LoadModule(module); err != nil {
		t.Fatalf("Failed to load module: %v", err)
	}

	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}
}

// TestBitwise tests bitwise operations
func TestBitwise(t *testing.T) {
	source := `
fun main() -> void {
    let a: u8 = 0xFF;
    let b: u8 = 0x0F;
    let and_result: u8 = a & b;
    let or_result: u8 = a | b;
    let xor_result: u8 = a ^ b;
    let shl_result: u8 = b << 4;
    let shr_result: u8 = a >> 4;
}
`
	module := compileToMIR(t, source)

	// Run in VM
	config := mirvm.Config{
		MemorySize: 4096,
		StackSize:  1024,
		MaxSteps:   1000,
	}
	vm := mirvm.New(config)
	if err := vm.LoadModule(module); err != nil {
		t.Fatalf("Failed to load module: %v", err)
	}

	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}
}
