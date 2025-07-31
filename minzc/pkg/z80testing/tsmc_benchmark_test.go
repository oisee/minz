package z80testing

import (
	"testing"
)

// TestTSMCBenchmarkSuite runs the complete TSMC benchmark suite
func TestTSMCBenchmarkSuite(t *testing.T) {
	suite := NewTSMCBenchmarkSuite()
	
	// Run all benchmarks
	if err := suite.Run(t); err != nil {
		t.Fatalf("Benchmark suite failed: %v", err)
	}
	
	// Save report
	reportFile := "tsmc_benchmark_report.txt"
	if err := suite.SaveReport(reportFile); err != nil {
		t.Errorf("Failed to save report: %v", err)
	} else {
		t.Logf("Report saved to: %s", reportFile)
	}
	
	// Log summary
	t.Logf("\nBenchmark Summary:")
	t.Logf("==================")
	
	totalPassed := 0
	totalImprovement := 0.0
	results := suite.GetResults()
	
	for _, result := range results {
		if result.Passed && result.Error == nil {
			totalPassed++
			totalImprovement += result.Improvement
		}
	}
	
	avgImprovement := 0.0
	if totalPassed > 0 {
		avgImprovement = totalImprovement / float64(totalPassed)
	}
	
	t.Logf("Total benchmarks: %d", len(results))
	t.Logf("Passed: %d", totalPassed)
	t.Logf("Average improvement: %.1f%%", avgImprovement)
	
	// Check if we meet our performance goals
	if avgImprovement < 30.0 {
		t.Errorf("TSMC did not achieve target 30%% improvement (got %.1f%%)", avgImprovement)
	} else {
		t.Logf("✓ TSMC achieved %.1f%% average improvement!", avgImprovement)
	}
}

// Individual benchmark tests for debugging
func TestTSMCBenchmark_Fibonacci(t *testing.T) {
	runSingleBenchmark(t, "fibonacci_recursive")
}

func TestTSMCBenchmark_StringOps(t *testing.T) {
	runSingleBenchmark(t, "string_length")
}

func TestTSMCBenchmark_ArrayOps(t *testing.T) {
	runSingleBenchmark(t, "array_sum")
}

func TestTSMCBenchmark_Sorting(t *testing.T) {
	runSingleBenchmark(t, "bubble_sort")
}

// Helper to run a single benchmark by name
func runSingleBenchmark(t *testing.T, benchmarkName string) {
	suite := NewTSMCBenchmarkSuite()
	
	// Find the benchmark
	var benchmark *TSMCBenchmark
	for _, b := range suite.benchmarks {
		if b.Name == benchmarkName {
			benchmark = &b
			break
		}
	}
	
	if benchmark == nil {
		t.Fatalf("Benchmark '%s' not found", benchmarkName)
	}
	
	// Run just this benchmark
	result := suite.runBenchmark(t, *benchmark)
	
	// Report results
	if result.Error != nil {
		t.Fatalf("Benchmark failed: %v", result.Error)
	}
	
	t.Logf("Results for %s:", benchmark.Name)
	t.Logf("  No TSMC: %d cycles", result.NoTSMC.Cycles)
	t.Logf("  With TSMC: %d cycles", result.WithTSMC.Cycles)
	t.Logf("  Improvement: %.1f%% (%.2fx speedup)", result.Improvement, result.SpeedupFactor)
	t.Logf("  SMC Events: %d", result.WithTSMC.SMCEvents)
	
	if !result.Passed {
		t.Errorf("Benchmark did not meet minimum improvement of %.1f%%", benchmark.MinImprovement)
	}
}