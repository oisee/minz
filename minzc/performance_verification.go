// Performance Verification System for MinZ String/LString Types
// This tool measures and compares performance between different string implementations

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type BenchmarkResult struct {
	Name          string
	CompileTime   time.Duration
	CodeSize      int64
	Success       bool
	ErrorMessage  string
	Optimizations []string
}

type PerformanceReport struct {
	Timestamp    time.Time
	BaselineResults []BenchmarkResult
	OptimizedResults []BenchmarkResult
	Improvements map[string]float64
}

// Test programs for benchmarking
var benchmarkPrograms = map[string]string{
	"string_simple": `
fun test() -> void {
    let msg: String = "Hello, World!";
    print_string(msg);
}

fun main() -> void {
    test();
}`,
	
	"string_long": `
fun test() -> void {
    let article: LString = l"This is a long string that uses u16 length prefix for efficient storage.";
    print_string(article);
}

fun main() -> void {
    test();
}`,
	
	"string_mixed": `
fun process() -> void {
    let short: String = "Short";
    let long: LString = l"This is a much longer string that demonstrates LString usage";
    
    print_string(short);
    print_string(long);
}

fun main() -> void {
    process();
}`,
	
	"string_loop": `
fun main() -> void {
    let messages: [String; 3] = ["First", "Second", "Third"];
    
    for i in 0..3 {
        print_string(messages[i]);
    }
}`,
}

func main() {
	fmt.Println("=== MinZ String/LString Performance Verification ===")
	fmt.Println()
	
	report := &PerformanceReport{
		Timestamp:    time.Now(),
		Improvements: make(map[string]float64),
	}
	
	// Run baseline tests (without optimizations)
	fmt.Println("📊 Running BASELINE tests (no optimizations)...")
	report.BaselineResults = runBenchmarks(false)
	
	// Run optimized tests (with optimizations)
	fmt.Println("\n⚡ Running OPTIMIZED tests (with -O flag)...")
	report.OptimizedResults = runBenchmarks(true)
	
	// Calculate improvements
	calculateImprovements(report)
	
	// Generate report
	generateReport(report)
	
	// Create visual dashboard
	createDashboard(report)
}

func runBenchmarks(optimize bool) []BenchmarkResult {
	var results []BenchmarkResult
	
	for name, code := range benchmarkPrograms {
		fmt.Printf("  Testing %s...", name)
		
		// Write test file
		testFile := fmt.Sprintf("bench_%s.minz", name)
		outFile := fmt.Sprintf("bench_%s.a80", name)
		
		err := os.WriteFile(testFile, []byte(code), 0644)
		if err != nil {
			fmt.Printf(" ❌ (failed to write: %v)\n", err)
			continue
		}
		
		// Compile with or without optimizations
		start := time.Now()
		
		args := []string{testFile, "-o", outFile}
		if optimize {
			args = append(args, "-O")
		}
		
		cmd := exec.Command("./minzc", args...)
		output, err := cmd.CombinedOutput()
		
		compileTime := time.Since(start)
		
		result := BenchmarkResult{
			Name:        name,
			CompileTime: compileTime,
			Success:     err == nil,
		}
		
		if err != nil {
			result.ErrorMessage = string(output)
			fmt.Printf(" ❌ (compilation failed)\n")
		} else {
			// Get code size
			if info, err := os.Stat(outFile); err == nil {
				result.CodeSize = info.Size()
			}
			
			// Extract optimization info if available
			if optimize && strings.Contains(string(output), "Optimizations:") {
				// Parse optimization messages
				result.Optimizations = extractOptimizations(string(output))
			}
			
			fmt.Printf(" ✅ (%.2fms, %d bytes)\n", compileTime.Seconds()*1000, result.CodeSize)
		}
		
		results = append(results, result)
		
		// Clean up
		os.Remove(testFile)
		os.Remove(outFile)
	}
	
	return results
}

func extractOptimizations(output string) []string {
	var opts []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Optimization:") {
			opts = append(opts, strings.TrimSpace(strings.TrimPrefix(line, "Optimization:")))
		}
	}
	return opts
}

func calculateImprovements(report *PerformanceReport) {
	for i, baseline := range report.BaselineResults {
		if i < len(report.OptimizedResults) {
			optimized := report.OptimizedResults[i]
			
			if baseline.Success && optimized.Success {
				// Calculate compile time improvement
				compileImprovement := float64(baseline.CompileTime-optimized.CompileTime) / float64(baseline.CompileTime) * 100
				
				// Calculate code size improvement
				sizeImprovement := float64(baseline.CodeSize-optimized.CodeSize) / float64(baseline.CodeSize) * 100
				
				key := baseline.Name
				report.Improvements[key+"_compile"] = compileImprovement
				report.Improvements[key+"_size"] = sizeImprovement
			}
		}
	}
}

func generateReport(report *PerformanceReport) {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("                    PERFORMANCE VERIFICATION REPORT")
	fmt.Println(strings.Repeat("=", 80))
	
	fmt.Printf("Generated: %s\n\n", report.Timestamp.Format("2006-01-02 15:04:05"))
	
	// Summary table
	fmt.Println("📊 BENCHMARK RESULTS")
	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("%-20s %15s %15s %15s\n", "Test", "Baseline (ms)", "Optimized (ms)", "Improvement")
	fmt.Println(strings.Repeat("-", 80))
	
	totalBaselineTime := time.Duration(0)
	totalOptimizedTime := time.Duration(0)
	successCount := 0
	
	for i, baseline := range report.BaselineResults {
		if i < len(report.OptimizedResults) && baseline.Success {
			optimized := report.OptimizedResults[i]
			if optimized.Success {
				improvement := report.Improvements[baseline.Name+"_compile"]
				fmt.Printf("%-20s %15.2f %15.2f %14.1f%%\n",
					baseline.Name,
					baseline.CompileTime.Seconds()*1000,
					optimized.CompileTime.Seconds()*1000,
					improvement)
				
				totalBaselineTime += baseline.CompileTime
				totalOptimizedTime += optimized.CompileTime
				successCount++
			}
		}
	}
	
	if successCount > 0 {
		fmt.Println(strings.Repeat("-", 80))
		avgImprovement := float64(totalBaselineTime-totalOptimizedTime) / float64(totalBaselineTime) * 100
		fmt.Printf("%-20s %15.2f %15.2f %14.1f%%\n",
			"AVERAGE",
			totalBaselineTime.Seconds()*1000/float64(successCount),
			totalOptimizedTime.Seconds()*1000/float64(successCount),
			avgImprovement)
	}
	
	// Code size comparison
	fmt.Println("\n📦 CODE SIZE COMPARISON")
	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("%-20s %15s %15s %15s\n", "Test", "Baseline", "Optimized", "Reduction")
	fmt.Println(strings.Repeat("-", 80))
	
	totalBaselineSize := int64(0)
	totalOptimizedSize := int64(0)
	
	for i, baseline := range report.BaselineResults {
		if i < len(report.OptimizedResults) && baseline.Success {
			optimized := report.OptimizedResults[i]
			if optimized.Success && baseline.CodeSize > 0 {
				reduction := report.Improvements[baseline.Name+"_size"]
				fmt.Printf("%-20s %15d %15d %14.1f%%\n",
					baseline.Name,
					baseline.CodeSize,
					optimized.CodeSize,
					reduction)
				
				totalBaselineSize += baseline.CodeSize
				totalOptimizedSize += optimized.CodeSize
			}
		}
	}
	
	if totalBaselineSize > 0 {
		fmt.Println(strings.Repeat("-", 80))
		avgReduction := float64(totalBaselineSize-totalOptimizedSize) / float64(totalBaselineSize) * 100
		fmt.Printf("%-20s %15d %15d %14.1f%%\n",
			"TOTAL",
			totalBaselineSize,
			totalOptimizedSize,
			avgReduction)
	}
	
	// Success rate
	fmt.Println("\n✅ SUCCESS METRICS")
	fmt.Println(strings.Repeat("-", 80))
	
	baselineSuccess := 0
	optimizedSuccess := 0
	
	for _, r := range report.BaselineResults {
		if r.Success {
			baselineSuccess++
		}
	}
	
	for _, r := range report.OptimizedResults {
		if r.Success {
			optimizedSuccess++
		}
	}
	
	fmt.Printf("Baseline Success Rate:  %d/%d (%.1f%%)\n", 
		baselineSuccess, len(report.BaselineResults),
		float64(baselineSuccess)/float64(len(report.BaselineResults))*100)
	
	fmt.Printf("Optimized Success Rate: %d/%d (%.1f%%)\n",
		optimizedSuccess, len(report.OptimizedResults),
		float64(optimizedSuccess)/float64(len(report.OptimizedResults))*100)
	
	// Save to file
	saveReportToFile(report)
}

func createDashboard(report *PerformanceReport) {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("                      PERFORMANCE DASHBOARD")
	fmt.Println(strings.Repeat("=", 80))
	
	// ASCII bar chart for compile time improvements
	fmt.Println("\n📊 Compile Time Improvements")
	fmt.Println()
	
	maxNameLen := 0
	for _, r := range report.BaselineResults {
		if len(r.Name) > maxNameLen {
			maxNameLen = len(r.Name)
		}
	}
	
	for _, baseline := range report.BaselineResults {
		if improvement, ok := report.Improvements[baseline.Name+"_compile"]; ok {
			// Create bar
			barLen := int(improvement / 2) // Scale to fit
			if barLen < 0 {
				barLen = 0
			}
			if barLen > 40 {
				barLen = 40
			}
			
			bar := strings.Repeat("█", barLen)
			fmt.Printf("%-*s |%-40s| %6.1f%%\n", 
				maxNameLen, baseline.Name, bar, improvement)
		}
	}
	
	// ASCII bar chart for size reductions
	fmt.Println("\n📦 Code Size Reductions")
	fmt.Println()
	
	for _, baseline := range report.BaselineResults {
		if reduction, ok := report.Improvements[baseline.Name+"_size"]; ok {
			// Create bar
			barLen := int(reduction / 2) // Scale to fit
			if barLen < 0 {
				barLen = 0
			}
			if barLen > 40 {
				barLen = 40
			}
			
			bar := strings.Repeat("▓", barLen)
			fmt.Printf("%-*s |%-40s| %6.1f%%\n",
				maxNameLen, baseline.Name, bar, reduction)
		}
	}
	
	// Summary verdict
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("                           VERDICT")
	fmt.Println(strings.Repeat("=", 80))
	
	avgCompileImprovement := 0.0
	avgSizeReduction := 0.0
	count := 0
	
	for name, improvement := range report.Improvements {
		if strings.HasSuffix(name, "_compile") {
			avgCompileImprovement += improvement
			count++
		} else if strings.HasSuffix(name, "_size") {
			avgSizeReduction += improvement
		}
	}
	
	if count > 0 {
		avgCompileImprovement /= float64(count)
		avgSizeReduction /= float64(count)
	}
	
	fmt.Printf("\n🎯 Average Compile Time Improvement: %.1f%%\n", avgCompileImprovement)
	fmt.Printf("📉 Average Code Size Reduction:      %.1f%%\n", avgSizeReduction)
	
	if avgCompileImprovement > 30 {
		fmt.Println("\n✅ PERFORMANCE TARGET ACHIEVED! (>30% improvement)")
	} else if avgCompileImprovement > 20 {
		fmt.Println("\n⚡ Good performance improvement (>20%)")
	} else if avgCompileImprovement > 10 {
		fmt.Println("\n📈 Moderate performance improvement (>10%)")
	} else {
		fmt.Println("\n📊 Minimal performance impact")
	}
	
	fmt.Println("\n" + strings.Repeat("=", 80))
}

func saveReportToFile(report *PerformanceReport) {
	filename := fmt.Sprintf("performance_report_%s.md", 
		report.Timestamp.Format("20060102_150405"))
	
	var buf bytes.Buffer
	
	buf.WriteString("# MinZ String/LString Performance Report\n\n")
	buf.WriteString(fmt.Sprintf("Generated: %s\n\n", report.Timestamp.Format("2006-01-02 15:04:05")))
	
	buf.WriteString("## Summary\n\n")
	
	avgImprovement := 0.0
	count := 0
	for name, imp := range report.Improvements {
		if strings.HasSuffix(name, "_compile") {
			avgImprovement += imp
			count++
		}
	}
	if count > 0 {
		avgImprovement /= float64(count)
	}
	
	buf.WriteString(fmt.Sprintf("- **Average Performance Improvement**: %.1f%%\n", avgImprovement))
	buf.WriteString(fmt.Sprintf("- **Tests Run**: %d\n", len(report.BaselineResults)))
	buf.WriteString(fmt.Sprintf("- **Compiler**: MinZ v0.9.4\n\n"))
	
	buf.WriteString("## Detailed Results\n\n")
	buf.WriteString("| Test | Baseline (ms) | Optimized (ms) | Improvement |\n")
	buf.WriteString("|------|---------------|----------------|-------------|\n")
	
	for i, baseline := range report.BaselineResults {
		if i < len(report.OptimizedResults) && baseline.Success {
			optimized := report.OptimizedResults[i]
			if optimized.Success {
				improvement := report.Improvements[baseline.Name+"_compile"]
				buf.WriteString(fmt.Sprintf("| %s | %.2f | %.2f | %.1f%% |\n",
					baseline.Name,
					baseline.CompileTime.Seconds()*1000,
					optimized.CompileTime.Seconds()*1000,
					improvement))
			}
		}
	}
	
	buf.WriteString("\n## Code Size Analysis\n\n")
	buf.WriteString("| Test | Baseline | Optimized | Reduction |\n")
	buf.WriteString("|------|----------|-----------|----------|\n")
	
	for i, baseline := range report.BaselineResults {
		if i < len(report.OptimizedResults) && baseline.Success {
			optimized := report.OptimizedResults[i]
			if optimized.Success && baseline.CodeSize > 0 {
				reduction := report.Improvements[baseline.Name+"_size"]
				buf.WriteString(fmt.Sprintf("| %s | %d | %d | %.1f%% |\n",
					baseline.Name,
					baseline.CodeSize,
					optimized.CodeSize,
					reduction))
			}
		}
	}
	
	buf.WriteString("\n## Conclusion\n\n")
	if avgImprovement > 30 {
		buf.WriteString("✅ **Performance target achieved!** The String/LString implementation demonstrates >30% improvement with optimizations enabled.\n")
	} else {
		buf.WriteString(fmt.Sprintf("📊 The String/LString implementation shows %.1f%% average improvement.\n", avgImprovement))
	}
	
	err := os.WriteFile(filename, buf.Bytes(), 0644)
	if err == nil {
		fmt.Printf("\n📄 Report saved to: %s\n", filename)
	}
}