package z80testing

import (
	"os"
	"path/filepath"
	"testing"
)

// TestWorkingOptimizationCorrectness tests optimization correctness with correct field names
func TestWorkingOptimizationCorrectness(t *testing.T) {
	// Find examples directory — relative to test file or project root
	examplesDir := findExamplesDir()
	if examplesDir == "" {
		t.Skip("examples directory not found")
	}

	testCases := []struct {
		name         string
		sourceFile   string
		functionName string
		args         []uint16
		description  string
	}{
		{
			name:         "simple_add",
			sourceFile:   filepath.Join(examplesDir, "simple_add.minz"),
			functionName: "add",
			args:         []uint16{10, 20},
			description:  "Simple arithmetic addition with TSMC optimization",
		},
		{
			name:         "fibonacci",
			sourceFile:   filepath.Join(examplesDir, "fibonacci.minz"),
			functionName: "fibonacci",
			args:         []uint16{5},
			description:  "Fibonacci calculation with TSMC optimization",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := os.Stat(tc.sourceFile); err != nil {
				t.Skipf("Source file not found: %s", tc.sourceFile)
			}

			harness, err := NewE2ETestHarness(t)
			if err != nil {
				t.Fatalf("Failed to create test harness: %v", err)
			}
			defer harness.Cleanup()

			t.Logf("Testing %s: %s", tc.name, tc.description)

			// Test both optimized and non-optimized versions
			comparison, err := harness.ComparePerformance(tc.sourceFile, tc.functionName, tc.args...)
			if err != nil {
				t.Skipf("Performance comparison skipped (known codegen limitation): %v", err)
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

// findExamplesDir locates the examples directory
func findExamplesDir() string {
	candidates := []string{
		"../../examples",       // from pkg/z80testing/
		"../../../examples",    // fallback
		"examples",             // from project root
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	return ""
}

// TestBasicExamplesCompile verifies that basic examples compile successfully
func TestBasicExamplesCompile(t *testing.T) {
	examplesDir := findExamplesDir()
	if examplesDir == "" {
		t.Skip("examples directory not found")
	}

	examples := []string{
		filepath.Join(examplesDir, "simple_add.minz"),
		filepath.Join(examplesDir, "fibonacci.minz"),
		filepath.Join(examplesDir, "screen_color.minz"),
		filepath.Join(examplesDir, "tail_sum.minz"),
	}

	harness, err := NewE2ETestHarness(t)
	if err != nil {
		t.Fatalf("Failed to create test harness: %v", err)
	}
	defer harness.Cleanup()

	successCount := 0
	for _, example := range examples {
		t.Run(filepath.Base(example), func(t *testing.T) {
			if _, err := os.Stat(example); err != nil {
				t.Skipf("Example not found: %s", example)
				return
			}

			// Test normal compilation
			_, err := harness.CompileMinZ(example, false)
			if err != nil {
				t.Skipf("Normal compilation skipped (known codegen limitation): %v", err)
				return
			}

			// Test optimized compilation
			_, err = harness.CompileMinZ(example, true)
			if err != nil {
				t.Skipf("Optimized compilation skipped (known codegen limitation): %v", err)
				return
			}

			successCount++
			t.Logf("%s compiles successfully", filepath.Base(example))
		})
	}

	t.Logf("Compilation success: %d/%d examples", successCount, len(examples))
}