package z80testing

import (
	"strings"
	"testing"
)

// TestTSMCOptimizationVerification ensures TSMC optimizations are applied correctly
func TestTSMCOptimizationVerification(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		function   string
		args       []uint16
		minSMC     int    // Minimum expected SMC events
		maxCycles  int    // Maximum cycles with TSMC
	}{
		{
			name:     "simple_function_call",
			function: "test_func",
			args:     []uint16{42},
			minSMC:   2, // At least 2 parameter patches
			maxCycles: 100,
			source: `
module test;

export fn test_func(x: u16) -> u16 {
    return helper(x) + helper(x + 1);
}

fn helper(n: u16) -> u16 {
    return n * 2;
}
`,
		},
		{
			name:     "recursive_with_tsmc",
			function: "factorial",
			args:     []uint16{5},
			minSMC:   10, // Multiple recursive calls
			maxCycles: 500,
			source: `
module test;

export fn factorial(n: u16) -> u16 {
    if (n <= 1) {
        return 1;
    }
    return n * factorial(n - 1);
}
`,
		},
		{
			name:     "loop_with_function_calls",
			function: "sum_loop",
			args:     []uint16{10},
			minSMC:   20, // Function called in loop
			maxCycles: 800,
			source: `
module test;

export fn sum_loop(n: u16) -> u16 {
    let sum: u16 = 0;
    let i: u16 = 0;
    while (i < n) {
        sum = add(sum, i);
        i = inc(i);
    }
    return sum;
}

fn add(a: u16, b: u16) -> u16 {
    return a + b;
}

fn inc(x: u16) -> u16 {
    return x + 1;
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create harness
			harness, err := NewE2ETestHarness(t)
			if err != nil {
				t.Fatalf("Failed to create harness: %v", err)
			}
			defer harness.Cleanup()

			// Write source file
			sourceFile := harness.workDir + "/" + tt.name + ".minz"
			if err := WriteTestFile(sourceFile, tt.source); err != nil {
				t.Fatalf("Failed to write source: %v", err)
			}

			// Test with TSMC enabled
			a80File, err := harness.CompileMinZ(sourceFile, true)
			if err != nil {
				t.Fatalf("Compilation with TSMC failed: %v", err)
			}

			// Verify TSMC markers in assembly
			a80Content, err := ReadTestFile(a80File)
			if err != nil {
				t.Fatalf("Failed to read assembly: %v", err)
			}

			// Check for TSMC patterns
			if !strings.Contains(a80Content, "$imm") {
				t.Error("No TSMC immediate markers found in assembly")
			}
			if !strings.Contains(a80Content, "$immOP") {
				t.Error("No TSMC operation markers found in assembly")
			}

			// Assemble and run
			binary, symbols, err := harness.AssembleA80(a80File)
			if err != nil {
				t.Fatalf("Assembly failed: %v", err)
			}

			harness.LoadBinary(binary, 0x8000)

			funcAddr, ok := symbols[tt.function]
			if !ok {
				t.Fatalf("Function %s not found", tt.function)
			}

			// Clear and enable SMC tracking
			harness.smcTracker.Clear()
			harness.smcTracker.Enable()

			// Execute function
			if err := harness.CallFunction(funcAddr, tt.args...); err != nil {
				t.Fatalf("Execution failed: %v", err)
			}

			// Verify SMC events occurred
			smcEvents := harness.smcTracker.CodeEventCount()
			if smcEvents < tt.minSMC {
				t.Errorf("Insufficient SMC events: got %d, want at least %d", 
					smcEvents, tt.minSMC)
			}

			// Verify performance improvement
			cycles := harness.GetCycles()
			if cycles > tt.maxCycles {
				t.Errorf("Too many cycles with TSMC: got %d, want <= %d", 
					cycles, tt.maxCycles)
			}

			// Log details
			t.Logf("Function: %s(%v)", tt.function, tt.args)
			t.Logf("Cycles: %d (max: %d)", cycles, tt.maxCycles)
			t.Logf("SMC Events: %d (min: %d)", smcEvents, tt.minSMC)
			t.Logf("SMC Summary:\n%s", harness.GetSMCSummary())
		})
	}
}

// TestTSMCPatternGeneration verifies correct TSMC pattern generation
func TestTSMCPatternGeneration(t *testing.T) {
	source := `
module patterns;

// Function with multiple parameters
export fn multi_param(a: u16, b: u16, c: u16) -> u16 {
    return a + b + c;
}

// Function called multiple times
export fn caller() -> u16 {
    let sum: u16 = 0;
    sum = sum + multi_param(1, 2, 3);
    sum = sum + multi_param(4, 5, 6);
    sum = sum + multi_param(7, 8, 9);
    return sum;
}
`

	harness, err := NewE2ETestHarness(t)
	if err != nil {
		t.Fatalf("Failed to create harness: %v", err)
	}
	defer harness.Cleanup()

	sourceFile := harness.workDir + "/patterns.minz"
	if err := WriteTestFile(sourceFile, source); err != nil {
		t.Fatalf("Failed to write source: %v", err)
	}

	// Compile with TSMC
	a80File, err := harness.CompileMinZ(sourceFile, true)
	if err != nil {
		t.Fatalf("Compilation failed: %v", err)
	}

	// Read and analyze assembly
	content, err := ReadTestFile(a80File)
	if err != nil {
		t.Fatalf("Failed to read assembly: %v", err)
	}

	// Verify TSMC patterns for multi-parameter function
	expectedPatterns := []string{
		"multi_param$imm0:",      // First parameter immediate
		"multi_param$imm1:",      // Second parameter immediate
		"multi_param$imm2:",      // Third parameter immediate
		"multi_param$immOP",      // Operation marker
		"LD HL, 0000",           // Immediate load for parameter
		"LD DE, 0000",           // Immediate load for parameter
	}

	for _, pattern := range expectedPatterns {
		if !strings.Contains(content, pattern) {
			t.Errorf("Expected TSMC pattern not found: %s", pattern)
		}
	}

	// Count TSMC call sites
	callSites := strings.Count(content, "multi_param$imm0 EQU")
	if callSites == 0 {
		t.Error("No TSMC call site definitions found")
	}

	t.Logf("Found %d TSMC call sites", callSites)
}

// TestTSMCCorrectness ensures TSMC doesn't break program correctness
func TestTSMCCorrectness(t *testing.T) {
	source := `
module correctness;

// Complex calculation to verify correctness
export fn complex_calc(n: u16) -> u16 {
    let result: u16 = 0;
    
    // Multiple operations
    result = add(result, multiply(n, 2));
    result = add(result, multiply(n, 3));
    result = subtract(result, divide(n, 2));
    
    // Conditional operations
    if (n > 10) {
        result = add(result, bonus(n));
    }
    
    return result;
}

fn add(a: u16, b: u16) -> u16 {
    return a + b;
}

fn multiply(a: u16, b: u16) -> u16 {
    return a * b;
}

fn subtract(a: u16, b: u16) -> u16 {
    return a - b;
}

fn divide(a: u16, b: u16) -> u16 {
    if (b == 0) {
        return 0;
    }
    return a / b;
}

fn bonus(n: u16) -> u16 {
    return n * 10;
}
`

	testCases := []struct {
		input    uint16
		expected uint16
	}{
		{5, 23},    // 5*2 + 5*3 - 5/2 = 10 + 15 - 2 = 23
		{10, 48},   // 10*2 + 10*3 - 10/2 = 20 + 30 - 5 = 45
		{20, 290},  // 20*2 + 20*3 - 20/2 + 20*10 = 40 + 60 - 10 + 200 = 290
	}

	harness, err := NewE2ETestHarness(t)
	if err != nil {
		t.Fatalf("Failed to create harness: %v", err)
	}
	defer harness.Cleanup()

	sourceFile := harness.workDir + "/correctness.minz"
	if err := WriteTestFile(sourceFile, source); err != nil {
		t.Fatalf("Failed to write source: %v", err)
	}

	// Test both with and without TSMC
	for _, enableTSMC := range []bool{false, true} {
		mode := "without TSMC"
		if enableTSMC {
			mode = "with TSMC"
		}

		t.Run(mode, func(t *testing.T) {
			a80File, err := harness.CompileMinZ(sourceFile, enableTSMC)
			if err != nil {
				t.Fatalf("Compilation failed: %v", err)
			}

			binary, symbols, err := harness.AssembleA80(a80File)
			if err != nil {
				t.Fatalf("Assembly failed: %v", err)
			}

			funcAddr := symbols["complex_calc"]

			for _, tc := range testCases {
				harness.LoadBinary(binary, 0x8000)
				
				if err := harness.CallFunction(funcAddr, tc.input); err != nil {
					t.Fatalf("Execution failed for input %d: %v", tc.input, err)
				}

				result := harness.GetResult()
				if result != tc.expected {
					t.Errorf("Incorrect result for input %d: got %d, want %d",
						tc.input, result, tc.expected)
				}
			}
		})
	}
}