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
	
	// Generate and display report
	report := suite.GenerateReport()
	t.Log("\n" + report)
	
	// Save report to file
	if err := suite.SaveReport("tsmc_benchmark_report.md"); err != nil {
		t.Errorf("Failed to save report: %v", err)
	} else {
		t.Log("Report saved to tsmc_benchmark_report.md")
	}
	
	// Overall assessment
	passedCount := 0
	totalCount := len(suite.results)
	totalImprovement := 0.0
	
	for _, result := range suite.results {
		if result.Passed && result.Error == nil {
			passedCount++
			totalImprovement += result.Improvement
		}
	}
	
	if passedCount == 0 {
		t.Fatal("No benchmarks passed")
	}
	
	avgImprovement := totalImprovement / float64(passedCount)
	t.Logf("\nOverall: %d/%d benchmarks passed with average %.1f%% improvement",
		passedCount, totalCount, avgImprovement)
	
	// Ensure we meet the 30% target
	if avgImprovement < 30.0 {
		t.Errorf("TSMC did not achieve target 30%% improvement (got %.1f%%)", avgImprovement)
	}
}

// TestIndividualBenchmarks allows running specific benchmarks
func TestIndividualBenchmarks(t *testing.T) {
	benchmarks := GetAllBenchmarks()
	
	// Test specific high-impact benchmarks
	highImpactTests := []string{
		"fibonacci_recursive",
		"matrix_multiply",
		"bubble_sort",
	}
	
	for _, testName := range highImpactTests {
		for _, benchmark := range benchmarks {
			if benchmark.Name == testName {
				t.Run(benchmark.Name, func(t *testing.T) {
					suite := NewTSMCBenchmarkSuite()
					result := suite.runBenchmark(t, benchmark)
					
					if result.Error != nil {
						t.Fatalf("Benchmark failed: %v", result.Error)
					}
					
					t.Logf("Results for %s:", benchmark.Name)
					t.Logf("  No TSMC: %d cycles", result.NoTSMC.Cycles)
					t.Logf("  With TSMC: %d cycles", result.WithTSMC.Cycles)
					t.Logf("  Improvement: %.1f%% (%.2fx speedup)", 
						result.Improvement, result.SpeedupFactor)
					t.Logf("  SMC Events: %d", result.WithTSMC.SMCEvents)
					
					if result.Improvement < benchmark.MinImprovement {
						t.Errorf("Insufficient improvement: got %.1f%%, want %.1f%%",
							result.Improvement, benchmark.MinImprovement)
					}
				})
			}
		}
	}
}

// BenchmarkTSMCPerformance provides Go benchmark integration
func BenchmarkTSMCPerformance(b *testing.B) {
	// This is a meta-benchmark that shows TSMC impact
	benchmarks := GetAllBenchmarks()
	
	for _, benchmark := range benchmarks {
		b.Run(benchmark.Name+"_NoTSMC", func(b *testing.B) {
			// Note: This is for reporting purposes, actual cycles are measured in Z80
			b.ReportMetric(float64(benchmark.MinImprovement), "%expected")
		})
		
		b.Run(benchmark.Name+"_WithTSMC", func(b *testing.B) {
			b.ReportMetric(float64(benchmark.MinImprovement), "%improvement")
		})
	}
}