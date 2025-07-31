package z80testing

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// CorpusTest represents a single test in the corpus
type CorpusTest struct {
	Name             string                 `json:"name"`
	SourceFile       string                 `json:"source_file"`
	Category         string                 `json:"category"`
	Description      string                 `json:"description"`
	ExpectedResult   ExpectedResult         `json:"expected_result"`
	Performance      PerformanceExpectation `json:"performance"`
	CompilerFlags    []string               `json:"compiler_flags,omitempty"`
	KnownIssues      []string               `json:"known_issues,omitempty"`
	Tags             []string               `json:"tags,omitempty"`
	FunctionTests    []FunctionTest         `json:"function_tests,omitempty"`
	MemoryChecks     []MemoryCheck          `json:"memory_checks,omitempty"`
	PortChecks       []PortCheck            `json:"port_checks,omitempty"`
}

// ExpectedResult defines what we expect from compilation and execution
type ExpectedResult struct {
	CompileSuccess bool   `json:"compile_success"`
	CompileError   string `json:"compile_error,omitempty"`
	RunSuccess     bool   `json:"run_success"`
	RuntimeError   string `json:"runtime_error,omitempty"`
	ExitCode       int    `json:"exit_code"`
}

// PerformanceExpectation defines performance expectations
type PerformanceExpectation struct {
	MaxCycles        int     `json:"max_cycles,omitempty"`
	TSMCImprovement  float64 `json:"tsmc_improvement,omitempty"` // Expected % improvement
	MemoryUsage      int     `json:"memory_usage,omitempty"`     // Max bytes
	CodeSize         int     `json:"code_size,omitempty"`        // Max bytes
}

// FunctionTest tests a specific function
type FunctionTest struct {
	FunctionName string   `json:"function_name"`
	Arguments    []uint16 `json:"arguments"`
	Expected     uint16   `json:"expected"`
	MaxCycles    int      `json:"max_cycles,omitempty"`
}

// MemoryCheck verifies memory state after execution
type MemoryCheck struct {
	Address  uint16 `json:"address"`
	Expected []byte `json:"expected"`
	Mask     []byte `json:"mask,omitempty"` // For partial checks
}

// PortCheck verifies port interactions
type PortCheck struct {
	Port     byte   `json:"port"`
	Expected []byte `json:"expected"`
}

// CorpusManifest is the top-level manifest structure
type CorpusManifest struct {
	Version     string                       `json:"version"`
	Description string                       `json:"description"`
	Categories  map[string]CategoryInfo      `json:"categories"`
	Tests       []CorpusTest                 `json:"tests"`
}

// CategoryInfo describes a test category
type CategoryInfo struct {
	Description string   `json:"description"`
	Tests       []string `json:"tests"` // Test names in this category
}

// CorpusTestRunner runs all corpus tests
type CorpusTestRunner struct {
	manifest      *CorpusManifest
	rootDir       string
	resultsDir    string
	enableVerbose bool
}

// NewCorpusTestRunner creates a new corpus test runner
func NewCorpusTestRunner(rootDir string, verbose bool) (*CorpusTestRunner, error) {
	manifestPath := filepath.Join(rootDir, "tests", "corpus", "manifest.json")
	data, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest CorpusManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	resultsDir := filepath.Join(rootDir, "tests", "corpus", "results")
	os.MkdirAll(resultsDir, 0755)

	return &CorpusTestRunner{
		manifest:      &manifest,
		rootDir:       rootDir,
		resultsDir:    resultsDir,
		enableVerbose: verbose,
	}, nil
}

// RunAll runs all tests in the corpus
func (r *CorpusTestRunner) RunAll(t *testing.T) {
	startTime := time.Now()
	totalTests := len(r.manifest.Tests)
	passed := 0
	failed := 0
	skipped := 0

	t.Logf("Running MinZ corpus tests: %d tests across %d categories", 
		totalTests, len(r.manifest.Categories))

	// Run tests by category
	for categoryName, category := range r.manifest.Categories {
		t.Run(categoryName, func(t *testing.T) {
			categoryTests := r.getTestsByCategory(categoryName)
			t.Logf("Category '%s': %s (%d tests)", 
				categoryName, category.Description, len(categoryTests))

			for _, test := range categoryTests {
				t.Run(test.Name, func(t *testing.T) {
					result := r.runSingleTest(t, &test)
					switch result {
					case TestPassed:
						passed++
					case TestFailed:
						failed++
					case TestSkipped:
						skipped++
					}
				})
			}
		})
	}

	duration := time.Since(startTime)
	t.Logf("\nCorpus test summary:")
	t.Logf("  Total:   %d tests", totalTests)
	t.Logf("  Passed:  %d (%.1f%%)", passed, float64(passed)/float64(totalTests)*100)
	t.Logf("  Failed:  %d (%.1f%%)", failed, float64(failed)/float64(totalTests)*100)
	t.Logf("  Skipped: %d (%.1f%%)", skipped, float64(skipped)/float64(totalTests)*100)
	t.Logf("  Duration: %v", duration)

	// Save results summary
	r.saveResultsSummary(passed, failed, skipped, duration)
}

// TestResult represents the outcome of a test
type TestResult int

const (
	TestPassed TestResult = iota
	TestFailed
	TestSkipped
)

// runSingleTest runs a single corpus test
func (r *CorpusTestRunner) runSingleTest(t *testing.T, test *CorpusTest) TestResult {
	// Skip if there are known blocking issues
	if len(test.KnownIssues) > 0 {
		for _, issue := range test.KnownIssues {
			if strings.Contains(issue, "BLOCKING") {
				t.Skipf("Skipping due to known issue: %s", issue)
				return TestSkipped
			}
		}
	}

	// Create test harness
	h, err := NewE2ETestHarness(t)
	if err != nil {
		t.Fatalf("Failed to create test harness: %v", err)
		return TestFailed
	}
	defer h.Cleanup()

	// Get source file path
	sourceFile := filepath.Join(r.rootDir, test.SourceFile)
	if _, err := os.Stat(sourceFile); err != nil {
		t.Errorf("Source file not found: %s", sourceFile)
		return TestFailed
	}

	// Test compilation
	if test.ExpectedResult.CompileSuccess {
		result := r.testCompilation(t, h, test, sourceFile)
		if result != TestPassed {
			return result
		}
	} else {
		// Expect compilation to fail
		_, err := h.CompileMinZ(sourceFile, false)
		if err == nil {
			t.Errorf("Expected compilation to fail, but it succeeded")
			return TestFailed
		}
		if test.ExpectedResult.CompileError != "" && 
		   !strings.Contains(err.Error(), test.ExpectedResult.CompileError) {
			t.Errorf("Expected error containing '%s', got: %v", 
				test.ExpectedResult.CompileError, err)
			return TestFailed
		}
		return TestPassed
	}

	// If we have function tests, run them
	if len(test.FunctionTests) > 0 {
		for _, funcTest := range test.FunctionTests {
			if !r.testFunction(t, h, test, sourceFile, &funcTest) {
				return TestFailed
			}
		}
	}

	// Test TSMC performance if expected
	if test.Performance.TSMCImprovement > 0 {
		if !r.testTSMCPerformance(t, h, test, sourceFile) {
			return TestFailed
		}
	}

	return TestPassed
}

// testCompilation tests that a file compiles successfully
func (r *CorpusTestRunner) testCompilation(t *testing.T, h *E2ETestHarness, 
	test *CorpusTest, sourceFile string) TestResult {
	
	// Compile without TSMC first
	a80File, err := h.CompileMinZ(sourceFile, false)
	if err != nil {
		t.Errorf("Compilation failed: %v", err)
		return TestFailed
	}

	// Assemble to binary
	binary, symbols, err := h.AssembleA80(a80File)
	if err != nil {
		t.Errorf("Assembly failed: %v", err)
		return TestFailed
	}

	// Check code size if specified
	if test.Performance.CodeSize > 0 && len(binary) > test.Performance.CodeSize {
		t.Errorf("Code size %d exceeds limit %d", len(binary), test.Performance.CodeSize)
		return TestFailed
	}

	if r.enableVerbose {
		t.Logf("Compiled successfully: %d bytes, %d symbols", len(binary), len(symbols))
	}

	return TestPassed
}

// testFunction tests a specific function
func (r *CorpusTestRunner) testFunction(t *testing.T, h *E2ETestHarness, 
	test *CorpusTest, sourceFile string, funcTest *FunctionTest) bool {
	
	// Compile and load
	a80File, err := h.CompileMinZ(sourceFile, false)
	if err != nil {
		t.Errorf("Compilation failed: %v", err)
		return false
	}

	binary, symbols, err := h.AssembleA80(a80File)
	if err != nil {
		t.Errorf("Assembly failed: %v", err)
		return false
	}

	h.LoadBinary(binary, 0x8000)

	// Find function
	funcAddr, ok := symbols[funcTest.FunctionName]
	if !ok {
		t.Errorf("Function %s not found in symbols", funcTest.FunctionName)
		return false
	}

	// Call function
	if err := h.CallFunction(funcAddr, funcTest.Arguments...); err != nil {
		t.Errorf("Function execution failed: %v", err)
		return false
	}

	// Check result
	result := h.GetResult()
	if result != funcTest.Expected {
		t.Errorf("Function %s: expected 0x%04X, got 0x%04X", 
			funcTest.FunctionName, funcTest.Expected, result)
		return false
	}

	// Check cycle count if specified
	cycles := h.GetCycles()
	if funcTest.MaxCycles > 0 && cycles > funcTest.MaxCycles {
		t.Errorf("Function %s: exceeded cycle limit %d (used %d)", 
			funcTest.FunctionName, funcTest.MaxCycles, cycles)
		return false
	}

	if r.enableVerbose {
		t.Logf("Function %s: OK (result=0x%04X, cycles=%d)", 
			funcTest.FunctionName, result, cycles)
	}

	return true
}

// testTSMCPerformance tests TSMC optimization
func (r *CorpusTestRunner) testTSMCPerformance(t *testing.T, h *E2ETestHarness, 
	test *CorpusTest, sourceFile string) bool {
	
	if len(test.FunctionTests) == 0 {
		t.Logf("No function tests defined for TSMC performance testing")
		return true
	}

	// Use the first function test for performance comparison
	funcTest := test.FunctionTests[0]
	
	comparison, err := h.ComparePerformance(sourceFile, funcTest.FunctionName, 
		funcTest.Arguments...)
	if err != nil {
		t.Errorf("Performance comparison failed: %v", err)
		return false
	}

	// Log the comparison
	t.Log(comparison.String())

	// Check improvement
	comparison.AssertPerformanceImprovement(t, test.Performance.TSMCImprovement)
	
	return true
}

// getTestsByCategory returns all tests in a category
func (r *CorpusTestRunner) getTestsByCategory(category string) []CorpusTest {
	var tests []CorpusTest
	for _, test := range r.manifest.Tests {
		if test.Category == category {
			tests = append(tests, test)
		}
	}
	return tests
}

// saveResultsSummary saves test results to a file
func (r *CorpusTestRunner) saveResultsSummary(passed, failed, skipped int, duration time.Duration) {
	summary := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"duration":  duration.String(),
		"total":     passed + failed + skipped,
		"passed":    passed,
		"failed":    failed,
		"skipped":   skipped,
	}

	data, _ := json.MarshalIndent(summary, "", "  ")
	summaryPath := filepath.Join(r.resultsDir, "summary.json")
	ioutil.WriteFile(summaryPath, data, 0644)
}

// TestCorpus is the main entry point for corpus testing
func TestCorpus(t *testing.T) {
	// Get project root
	rootDir := os.Getenv("MINZ_ROOT")
	if rootDir == "" {
		// Try to find it relative to current directory
		rootDir = "../.."
		if _, err := os.Stat(filepath.Join(rootDir, "tests", "corpus", "manifest.json")); err != nil {
			t.Skip("Cannot find MinZ root directory. Set MINZ_ROOT environment variable.")
		}
	}

	verbose := os.Getenv("VERBOSE") == "1"
	runner, err := NewCorpusTestRunner(rootDir, verbose)
	if err != nil {
		t.Fatalf("Failed to create corpus runner: %v", err)
	}

	runner.RunAll(t)
}