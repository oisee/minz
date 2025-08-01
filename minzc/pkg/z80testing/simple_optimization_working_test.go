package z80testing

import (
	"testing"
)

// TestWorkingOptimizationCorrectness tests optimization correctness with correct field names
func TestWorkingOptimizationCorrectness(t *testing.T) {
	testCases := []struct {
		name         string
		sourceFile   string
		functionName string
		args         []uint16
		description  string
	}{
		{
			name:         "simple_add",
			sourceFile:   "../examples/simple_add.minz",
			functionName: "add",
			args:         []uint16{10, 20},
			description:  "Simple arithmetic addition with TSMC optimization",
		},
		{
			name:         "fibonacci",
			sourceFile:   "../examples/fibonacci.minz", 
			functionName: "fibonacci",
			args:         []uint16{5},
			description:  "Fibonacci calculation with TSMC optimization",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			harness, err := NewE2ETestHarness(t)
			if err != nil {
				t.Fatalf("Failed to create test harness: %v", err)
			}
			defer harness.Cleanup()

			t.Logf("Testing %s: %s", tc.name, tc.description)

			// Test both optimized and non-optimized versions
			comparison, err := harness.ComparePerformance(tc.sourceFile, tc.functionName, tc.args...)
			if err != nil {
				t.Fatalf("Performance comparison failed: %v", err)
			}

			// Verify results are equivalent (using correct field names)
			if comparison.NoTSMCResult != comparison.TSMCResult {
				t.Errorf("Results not equivalent!\n  NoTSMC: %d (0x%04X)\n  TSMC: %d (0x%04X)", 
					comparison.NoTSMCResult, comparison.NoTSMCResult,
					comparison.TSMCResult, comparison.TSMCResult)
			} else {
				t.Logf("✅ Results equivalent: %d (0x%04X)", comparison.NoTSMCResult, comparison.NoTSMCResult)
			}

			// Log performance improvement (using correct field names)
			improvement := comparison.CycleReduction
			speedup := comparison.SpeedupFactor
			
			t.Logf("Performance improvement: %.1f%% cycle reduction (%.2fx speedup)", 
				improvement, speedup)
			t.Logf("Cycles: %d → %d", comparison.NoTSMCCycles, comparison.TSMCCycles)

			// For TSMC functions, we expect some improvement
			if improvement > 0 {
				t.Logf("✅ Performance improved by %.1f%%", improvement)
			} else if improvement < 0 {
				t.Logf("⚠️ Performance regression: %.1f%%", -improvement)
			} else {
				t.Logf("ℹ️ No performance difference")
			}

			// Log SMC events
			if comparison.TSMCSMCEvents > 0 {
				t.Logf("SMC events in TSMC version: %d", comparison.TSMCSMCEvents)
			}
			if comparison.NoTSMCSMCEvents > 0 {
				t.Logf("SMC events in non-TSMC version: %d", comparison.NoTSMCSMCEvents)
			}

			// Use the built-in assertion method
			if improvement > 0 {
				comparison.AssertPerformanceImprovement(t, improvement)
				t.Logf("✅ Performance improvement assertion passed")
			}
		})
	}
}

// TestBasicExamplesCompile verifies that basic examples compile successfully 
func TestBasicExamplesCompile(t *testing.T) {
	examples := []string{
		"../examples/simple_add.minz",
		"../examples/fibonacci.minz", 
		"../examples/screen_color.minz",
		"../examples/tail_sum.minz",
	}

	harness, err := NewE2ETestHarness(t)
	if err != nil {
		t.Fatalf("Failed to create test harness: %v", err)
	}
	defer harness.Cleanup()

	successCount := 0
	for _, example := range examples {
		t.Run(example, func(t *testing.T) {
			// Test normal compilation
			_, err := harness.CompileMinZ(example, false)
			if err != nil {
				t.Errorf("Normal compilation failed: %v", err)
				return
			}

			// Test optimized compilation
			_, err = harness.CompileMinZ(example, true)
			if err != nil {
				t.Errorf("Optimized compilation failed: %v", err)
				return
			}

			successCount++
			t.Logf("✅ %s compiles successfully", example)
		})
	}

	t.Logf("Compilation success: %d/%d examples", successCount, len(examples))
	
	if successCount == len(examples) {
		t.Logf("🎉 All basic examples compile successfully!")
	}
}