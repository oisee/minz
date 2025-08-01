package testing

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// ComprehensiveTestResult contains all test data for an example
type ComprehensiveTestResult struct {
	ExampleName string
	SourceFile  string
	
	// Compilation results
	CompileNormal     CompilationResult
	CompileOptimized  CompilationResult
	
	// Execution results
	ExecutionNormal    ExecutionResult
	ExecutionOptimized ExecutionResult
	
	// Verification
	OutputMatches     bool
	MemoryMatches     bool
	CorrectResult     bool
	
	// Performance metrics
	CycleReduction    float64
	SpeedupFactor     float64
	CodeSizeReduction float64
	
	// Test metadata
	TestTime          time.Duration
	ErrorDetails      string
}

// CompilationResult contains compilation metrics
type CompilationResult struct {
	Success       bool
	CompileTime   time.Duration
	AssemblySize  int
	BinarySize    int
	ErrorMessage  string
	SMCDetected   bool
	Optimizations []string
}

// ExecutionResult contains emulation results
type ExecutionResult struct {
	Success        bool
	Cycles         int
	ExecutionTime  time.Duration
	FinalRegisters RegisterState
	MemoryDump     []byte
	OutputCapture  string
	SMCEvents      int
}

// RegisterState captures Z80 register values
type RegisterState struct {
	A, F, B, C, D, E, H, L byte
	IX, IY, SP, PC         uint16
}

// TestSuite manages comprehensive testing
type TestSuite struct {
	examplesDir string
	outputDir   string
	emulator    *Z80Emulator
	compiler    *MinzCompiler
	results     []ComprehensiveTestResult
}

// NewTestSuite creates a new comprehensive test suite
func NewTestSuite(examplesDir, outputDir string) *TestSuite {
	return &TestSuite{
		examplesDir: examplesDir,
		outputDir:   outputDir,
		results:     []ComprehensiveTestResult{},
	}
}

// RunComprehensiveTests executes all tests
func (ts *TestSuite) RunComprehensiveTests() error {
	// Find all .minz files
	examples, err := ts.findAllExamples()
	if err != nil {
		return fmt.Errorf("failed to find examples: %v", err)
	}
	
	fmt.Printf("🧪 Running comprehensive tests on %d examples\n", len(examples))
	fmt.Println("=" + strings.Repeat("=", 60))
	
	successCount := 0
	for i, example := range examples {
		fmt.Printf("[%d/%d] Testing %s... ", i+1, len(examples), example)
		
		result := ts.testExample(example)
		ts.results = append(ts.results, result)
		
		if result.CorrectResult {
			fmt.Printf("✅ PASS (%.1f%% faster)\n", result.CycleReduction)
			successCount++
		} else {
			fmt.Printf("❌ FAIL: %s\n", result.ErrorDetails)
		}
	}
	
	// Generate reports
	ts.generateReports()
	
	fmt.Printf("\n📊 Test Summary: %d/%d passed (%.1f%%)\n", 
		successCount, len(examples), 
		float64(successCount)/float64(len(examples))*100)
	
	return nil
}

// testExample runs comprehensive test on a single example
func (ts *TestSuite) testExample(exampleFile string) ComprehensiveTestResult {
	result := ComprehensiveTestResult{
		ExampleName: filepath.Base(exampleFile),
		SourceFile:  exampleFile,
	}
	
	startTime := time.Now()
	
	// 1. Compile without optimization
	result.CompileNormal = ts.compileExample(exampleFile, false)
	if !result.CompileNormal.Success {
		result.ErrorDetails = "Normal compilation failed: " + result.CompileNormal.ErrorMessage
		result.TestTime = time.Since(startTime)
		return result
	}
	
	// 2. Compile with optimization
	result.CompileOptimized = ts.compileExample(exampleFile, true)
	if !result.CompileOptimized.Success {
		result.ErrorDetails = "Optimized compilation failed: " + result.CompileOptimized.ErrorMessage
		result.TestTime = time.Since(startTime)
		return result
	}
	
	// 3. Execute both versions and capture results
	result.ExecutionNormal = ts.executeExample(result.CompileNormal)
	result.ExecutionOptimized = ts.executeExample(result.CompileOptimized)
	
	// 4. Verify correctness
	result.OutputMatches = (result.ExecutionNormal.OutputCapture == result.ExecutionOptimized.OutputCapture)
	result.MemoryMatches = ts.compareMemoryStates(result.ExecutionNormal, result.ExecutionOptimized)
	result.CorrectResult = result.OutputMatches && result.MemoryMatches
	
	// 5. Calculate performance metrics
	if result.ExecutionNormal.Cycles > 0 {
		result.CycleReduction = float64(result.ExecutionNormal.Cycles - result.ExecutionOptimized.Cycles) / 
		                      float64(result.ExecutionNormal.Cycles) * 100.0
		result.SpeedupFactor = float64(result.ExecutionNormal.Cycles) / float64(result.ExecutionOptimized.Cycles)
	}
	
	if result.CompileNormal.AssemblySize > 0 {
		result.CodeSizeReduction = float64(result.CompileNormal.AssemblySize - result.CompileOptimized.AssemblySize) /
		                          float64(result.CompileNormal.AssemblySize) * 100.0
	}
	
	result.TestTime = time.Since(startTime)
	return result
}

// generateReports creates various output reports
func (ts *TestSuite) generateReports() {
	// 1. JSON report for CI/CD
	ts.generateJSONReport()
	
	// 2. Markdown summary report
	ts.generateMarkdownReport()
	
	// 3. HTML visualization report
	ts.generateHTMLReport()
	
	// 4. Performance comparison CSV
	ts.generatePerformanceCSV()
}

// generateMarkdownReport creates a detailed markdown report
func (ts *TestSuite) generateMarkdownReport() {
	report := strings.Builder{}
	
	report.WriteString("# MinZ Comprehensive Test Report\n\n")
	report.WriteString(fmt.Sprintf("**Generated:** %s\n\n", time.Now().Format("2006-01-02 15:04:05")))
	
	// Summary statistics
	totalExamples := len(ts.results)
	passedExamples := 0
	totalCycleReduction := 0.0
	maxSpeedup := 0.0
	minSpeedup := 999.0
	
	for _, result := range ts.results {
		if result.CorrectResult {
			passedExamples++
			totalCycleReduction += result.CycleReduction
			if result.SpeedupFactor > maxSpeedup {
				maxSpeedup = result.SpeedupFactor
			}
			if result.SpeedupFactor < minSpeedup && result.SpeedupFactor > 0 {
				minSpeedup = result.SpeedupFactor
			}
		}
	}
	
	avgCycleReduction := totalCycleReduction / float64(passedExamples)
	
	report.WriteString("## 📊 Summary Statistics\n\n")
	report.WriteString(fmt.Sprintf("- **Total Examples Tested:** %d\n", totalExamples))
	report.WriteString(fmt.Sprintf("- **Passed:** %d (%.1f%%)\n", passedExamples, 
		float64(passedExamples)/float64(totalExamples)*100))
	report.WriteString(fmt.Sprintf("- **Average Cycle Reduction:** %.1f%%\n", avgCycleReduction))
	report.WriteString(fmt.Sprintf("- **Maximum Speedup:** %.2fx\n", maxSpeedup))
	report.WriteString(fmt.Sprintf("- **Minimum Speedup:** %.2fx\n", minSpeedup))
	report.WriteString("\n")
	
	// Detailed results table
	report.WriteString("## 📋 Detailed Results\n\n")
	report.WriteString("| Example | Compile | Execute | Cycles Normal | Cycles Opt | Reduction | Speedup | Result |\n")
	report.WriteString("|---------|---------|---------|---------------|------------|-----------|---------|--------|\n")
	
	for _, result := range ts.results {
		compileStatus := "✅"
		if !result.CompileNormal.Success || !result.CompileOptimized.Success {
			compileStatus = "❌"
		}
		
		executeStatus := "✅"
		if !result.ExecutionNormal.Success || !result.ExecutionOptimized.Success {
			executeStatus = "❌"
		}
		
		testResult := "✅ PASS"
		if !result.CorrectResult {
			testResult = "❌ FAIL"
		}
		
		report.WriteString(fmt.Sprintf("| %s | %s | %s | %d | %d | %.1f%% | %.2fx | %s |\n",
			result.ExampleName,
			compileStatus,
			executeStatus,
			result.ExecutionNormal.Cycles,
			result.ExecutionOptimized.Cycles,
			result.CycleReduction,
			result.SpeedupFactor,
			testResult,
		))
	}
	
	// Performance distribution
	report.WriteString("\n## 📈 Performance Distribution\n\n")
	report.WriteString("```\n")
	report.WriteString(ts.generateASCIIHistogram())
	report.WriteString("```\n")
	
	// Failed tests details
	failedTests := []ComprehensiveTestResult{}
	for _, result := range ts.results {
		if !result.CorrectResult {
			failedTests = append(failedTests, result)
		}
	}
	
	if len(failedTests) > 0 {
		report.WriteString("\n## ❌ Failed Tests\n\n")
		for _, failed := range failedTests {
			report.WriteString(fmt.Sprintf("### %s\n", failed.ExampleName))
			report.WriteString(fmt.Sprintf("**Error:** %s\n\n", failed.ErrorDetails))
		}
	}
	
	// Top performers
	report.WriteString("\n## 🏆 Top Performers (Highest Speedup)\n\n")
	topPerformers := ts.getTopPerformers(5)
	for i, result := range topPerformers {
		report.WriteString(fmt.Sprintf("%d. **%s** - %.2fx speedup (%.1f%% cycle reduction)\n",
			i+1, result.ExampleName, result.SpeedupFactor, result.CycleReduction))
	}
	
	// Save report
	reportPath := filepath.Join(ts.outputDir, "comprehensive_test_report.md")
	ioutil.WriteFile(reportPath, []byte(report.String()), 0644)
}

// generateASCIIHistogram creates a text-based performance histogram
func (ts *TestSuite) generateASCIIHistogram() string {
	// Group results by speedup ranges
	ranges := map[string]int{
		"0.0-1.0x": 0,
		"1.0-1.2x": 0,
		"1.2-1.5x": 0,
		"1.5-2.0x": 0,
		"2.0-3.0x": 0,
		"3.0x+":    0,
	}
	
	for _, result := range ts.results {
		if !result.CorrectResult {
			continue
		}
		
		switch {
		case result.SpeedupFactor < 1.0:
			ranges["0.0-1.0x"]++
		case result.SpeedupFactor < 1.2:
			ranges["1.0-1.2x"]++
		case result.SpeedupFactor < 1.5:
			ranges["1.2-1.5x"]++
		case result.SpeedupFactor < 2.0:
			ranges["1.5-2.0x"]++
		case result.SpeedupFactor < 3.0:
			ranges["2.0-3.0x"]++
		default:
			ranges["3.0x+"]++
		}
	}
	
	// Create histogram
	histogram := "Speedup Distribution:\n"
	keys := []string{"0.0-1.0x", "1.0-1.2x", "1.2-1.5x", "1.5-2.0x", "2.0-3.0x", "3.0x+"}
	
	for _, key := range keys {
		count := ranges[key]
		bar := strings.Repeat("█", count)
		histogram += fmt.Sprintf("%-10s |%-30s| %d\n", key, bar, count)
	}
	
	return histogram
}

// getTopPerformers returns the N best performing examples
func (ts *TestSuite) getTopPerformers(n int) []ComprehensiveTestResult {
	// Filter passed tests
	passed := []ComprehensiveTestResult{}
	for _, result := range ts.results {
		if result.CorrectResult {
			passed = append(passed, result)
		}
	}
	
	// Sort by speedup factor
	sort.Slice(passed, func(i, j int) bool {
		return passed[i].SpeedupFactor > passed[j].SpeedupFactor
	})
	
	// Return top N
	if len(passed) < n {
		n = len(passed)
	}
	return passed[:n]
}

// Helper functions

func (ts *TestSuite) findAllExamples() ([]string, error) {
	examples := []string{}
	
	err := filepath.Walk(ts.examplesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Skip directories and non-.minz files
		if info.IsDir() || !strings.HasSuffix(path, ".minz") {
			return nil
		}
		
		// Skip files in output directories
		if strings.Contains(path, "/output/") || strings.Contains(path, "/compiled/") {
			return nil
		}
		
		examples = append(examples, path)
		return nil
	})
	
	return examples, err
}

func (ts *TestSuite) compileExample(sourceFile string, optimize bool) CompilationResult {
	// Implementation would call actual MinZ compiler
	// This is a placeholder structure
	return CompilationResult{
		Success: true,
		// ... other fields
	}
}

func (ts *TestSuite) executeExample(compilation CompilationResult) ExecutionResult {
	// Implementation would use Z80 emulator
	// This is a placeholder structure
	return ExecutionResult{
		Success: true,
		// ... other fields
	}
}

func (ts *TestSuite) compareMemoryStates(exec1, exec2 ExecutionResult) bool {
	// Compare relevant memory regions
	// Skip areas that might differ (like temporary variables)
	return true // Placeholder
}

func (ts *TestSuite) generateJSONReport() {
	jsonData, _ := json.MarshalIndent(ts.results, "", "  ")
	jsonPath := filepath.Join(ts.outputDir, "test_results.json")
	ioutil.WriteFile(jsonPath, jsonData, 0644)
}

func (ts *TestSuite) generateHTMLReport() {
	// Generate interactive HTML report with charts
	// Using Chart.js or similar for visualization
}

func (ts *TestSuite) generatePerformanceCSV() {
	// Export performance data as CSV for further analysis
}