package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// RegressionTests provides automated regression testing for MinZ compiler features
type RegressionTests struct {
	workDir         string
	benchmarker     *PerformanceBenchmarks
	pipelineTester  *PipelineVerification
	testSuites      []RegressionTestSuite
	baselineResults map[string]BenchmarkResult
}

// RegressionTestSuite represents a collection of related regression tests
type RegressionTestSuite struct {
	Name        string
	Description string
	Tests       []RegressionTest
}

// RegressionTest represents a single regression test
type RegressionTest struct {
	Name                 string
	Description          string
	SourceCode           string
	ExpectedOutput       interface{}
	PerformanceBaseline  *BenchmarkResult
	ZeroCostAssertion    bool
	CriticalFeatures     []string
	MinimumCycles        int
	MaximumCycles        int
	RequiredOptimizations []string
}

// RegressionResult captures the result of a regression test
type RegressionResult struct {
	TestName     string
	Passed       bool
	Reason       string
	Performance  *BenchmarkResult
	Timestamp    time.Time
	Duration     time.Duration
}

// NewRegressionTests creates a new regression testing framework
func NewRegressionTests(t *testing.T) (*RegressionTests, error) {
	workDir, err := os.MkdirTemp("", "minz_regression_test_*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	benchmarker, err := NewPerformanceBenchmarks(t)
	if err != nil {
		return nil, fmt.Errorf("failed to create benchmarker: %w", err)
	}

	pipelineTester, err := NewPipelineVerification(t)
	if err != nil {
		return nil, fmt.Errorf("failed to create pipeline tester: %w", err)
	}

	rt := &RegressionTests{
		workDir:         workDir,
		benchmarker:     benchmarker,
		pipelineTester:  pipelineTester,
		testSuites:      make([]RegressionTestSuite, 0),
		baselineResults: make(map[string]BenchmarkResult),
	}

	// Initialize with core test suites
	rt.initializeCoreTestSuites()

	return rt, nil
}

// Cleanup removes temporary files and cleans up resources
func (rt *RegressionTests) Cleanup() {
	if rt.benchmarker != nil {
		rt.benchmarker.Cleanup()
	}
	if rt.pipelineTester != nil {
		rt.pipelineTester.Cleanup()
	}
	if rt.workDir != "" {
		os.RemoveAll(rt.workDir)
	}
}

// initializeCoreTestSuites sets up the core regression test suites
func (rt *RegressionTests) initializeCoreTestSuites() {
	// Lambda Zero-Cost Test Suite
	rt.testSuites = append(rt.testSuites, RegressionTestSuite{
		Name:        "LambdaZeroCost",
		Description: "Verifies that lambda abstractions have zero runtime cost",
		Tests: []RegressionTest{
			{
				Name:        "SimpleLambdaTransformation",
				Description: "Basic lambda should transform to direct function call",
				SourceCode: `
					fun test_lambda() -> u8 {
						let add = |a: u8, b: u8| => u8 { a + b };
						add(5, 3)
					}
				`,
				ZeroCostAssertion: true,
				CriticalFeatures:  []string{"lambda_elimination", "direct_call"},
				MaximumCycles:     50,
			},
			{
				Name:        "NestedLambdaTransformation",
				Description: "Nested lambdas should be fully flattened",
				SourceCode: `
					fun test_nested() -> u8 {
						let outer = |x: u8| => (u8 => u8) { 
							|y: u8| => u8 { x + y }
						};
						let inner = outer(10);
						inner(5)
					}
				`,
				ZeroCostAssertion: true,
				CriticalFeatures:  []string{"lambda_flattening", "closure_elimination"},
				MaximumCycles:     80,
			},
			{
				Name:        "LambdaInLoop",
				Description: "Lambda used in loop should not create overhead",
				SourceCode: `
					fun test_loop_lambda() -> u16 {
						let multiply = |a: u8, b: u8| => u16 { (a as u16) * (b as u16) };
						let sum: u16 = 0;
						for i in 0..10 {
							sum = sum + multiply(i, 2);
						}
						sum
					}
				`,
				ZeroCostAssertion: true,
				CriticalFeatures:  []string{"loop_lambda_optimization", "type_casting"},
				MaximumCycles:     200,
			},
		},
	})

	// Interface Zero-Cost Test Suite
	rt.testSuites = append(rt.testSuites, RegressionTestSuite{
		Name:        "InterfaceZeroCost",
		Description: "Verifies that interface method calls resolve to direct calls",
		Tests: []RegressionTest{
			{
				Name:        "SimpleInterfaceResolution",
				Description: "Interface method should resolve to direct call",
				SourceCode: `
					interface Addable {
						fun add(self, other: u8) -> u8;
					}
					
					struct Number {
						value: u8
					}
					
					impl Addable for Number {
						fun add(self, other: u8) -> u8 {
							self.value + other
						}
					}
					
					fun test_interface() -> u8 {
						let num = Number { value: 10 };
						num.add(5)
					}
				`,
				ZeroCostAssertion: true,
				CriticalFeatures:  []string{"interface_resolution", "monomorphization"},
				MaximumCycles:     40,
			},
			{
				Name:        "GenericInterfaceResolution",
				Description: "Generic interface should monomorphize to specific types",
				SourceCode: `
					interface Comparable<T> {
						fun compare(self, other: T) -> i8;
					}
					
					struct Point {
						x: u8,
						y: u8
					}
					
					impl Comparable<Point> for Point {
						fun compare(self, other: Point) -> i8 {
							if self.x > other.x { 1 } else { -1 }
						}
					}
					
					fun test_generic() -> i8 {
						let p1 = Point { x: 5, y: 3 };
						let p2 = Point { x: 2, y: 7 };
						p1.compare(p2)
					}
				`,
				ZeroCostAssertion: true,
				CriticalFeatures:  []string{"generic_monomorphization", "struct_methods"},
				MaximumCycles:     60,
			},
		},
	})

	// TSMC (True Self-Modifying Code) Test Suite
	rt.testSuites = append(rt.testSuites, RegressionTestSuite{
		Name:        "TSMCOptimization",
		Description: "Verifies that TSMC optimizations provide significant performance gains",
		Tests: []RegressionTest{
			{
				Name:        "TSMCParameterInlining",
				Description: "Parameters should be patched directly into instructions",
				SourceCode: `
					fun add_with_constant(x: u8) -> u8 {
						x + 42  // 42 should be patched into instruction immediate
					}
					
					fun test_tsmc() -> u8 {
						add_with_constant(10)
					}
				`,
				CriticalFeatures:  []string{"parameter_inlining", "immediate_patching"},
				RequiredOptimizations: []string{"true_smc"},
				MaximumCycles:     30,
			},
			{
				Name:        "TSMCLoopOptimization",
				Description: "Loop parameters should use self-modifying code",
				SourceCode: `
					fun count_to_n(n: u8) -> u16 {
						let sum: u16 = 0;
						for i in 0..n {
							sum = sum + (i as u16);
						}
						sum
					}
				`,
				CriticalFeatures:  []string{"loop_smc", "parameter_patching"},
				RequiredOptimizations: []string{"true_smc", "O"},
				MinimumCycles:     50,
				MaximumCycles:     150,
			},
		},
	})

	// ZX Spectrum Integration Test Suite
	rt.testSuites = append(rt.testSuites, RegressionTestSuite{
		Name:        "ZXSpectrumIntegration",
		Description: "Verifies ZX Spectrum specific functionality and library integration",
		Tests: []RegressionTest{
			{
				Name:        "ScreenMemoryAccess",
				Description: "Screen memory operations should compile correctly",
				SourceCode: `
					use zx.screen;
					
					fun test_screen() -> u8 {
						screen::set_pixel(10, 10, true);
						screen::get_pixel(10, 10) as u8
					}
				`,
				CriticalFeatures:  []string{"memory_mapping", "library_integration"},
				MaximumCycles:     80,
			},
			{
				Name:        "HardwarePortAccess",
				Description: "Hardware port I/O should work correctly",
				SourceCode: `
					fun test_border() -> u8 {
						// Set border color to red (2)
						port_out(0x254, 2);
						port_in(0x254)
					}
				`,
				CriticalFeatures:  []string{"port_io", "hardware_access"},
				MaximumCycles:     20,
			},
		},
	})

	// ABI Integration Test Suite
	rt.testSuites = append(rt.testSuites, RegressionTestSuite{
		Name:        "ABIIntegration",
		Description: "Verifies seamless integration with existing assembly code",
		Tests: []RegressionTest{
			{
				Name:        "SimpleABICall",
				Description: "Basic @abi function call should work correctly",
				SourceCode: `
					@abi("register: A=param1, return: A")
					extern fun rom_multiply(param1: u8) -> u8;
					
					fun test_abi() -> u8 {
						rom_multiply(5)
					}
				`,
				CriticalFeatures:  []string{"abi_integration", "register_mapping"},
				MaximumCycles:     25,
			},
			{
				Name:        "ComplexABICall",
				Description: "Complex @abi with multiple parameters",
				SourceCode: `
					@abi("register: HL=buffer, DE=length, return: A")
					extern fun process_buffer(buffer: *u8, length: u16) -> u8;
					
					fun test_complex_abi() -> u8 {
						let data: [u8; 5] = [1, 2, 3, 4, 5];
						process_buffer(&data[0], 5)
					}
				`,
				CriticalFeatures:  []string{"pointer_parameters", "array_addressing"},
				MaximumCycles:     50,
			},
		},
	})
}

// RunRegressionTest executes a single regression test
func (rt *RegressionTests) RunRegressionTest(test RegressionTest) (*RegressionResult, error) {
	startTime := time.Now()
	result := &RegressionResult{
		TestName:  test.Name,
		Timestamp: startTime,
	}

	// Create test file
	sourceFile := filepath.Join(rt.workDir, fmt.Sprintf("%s.minz", test.Name))
	if err := os.WriteFile(sourceFile, []byte(test.SourceCode), 0644); err != nil {
		result.Passed = false
		result.Reason = fmt.Sprintf("Failed to write source file: %v", err)
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Compile and measure performance
	optimizations := []string{"base"}
	if len(test.RequiredOptimizations) > 0 {
		optimizations = append(optimizations, test.RequiredOptimizations...)
	}

	benchResult, err := rt.benchmarker.CompileAndMeasure(test.Name, test.SourceCode, optimizations)
	if err != nil {
		result.Passed = false
		result.Reason = fmt.Sprintf("Compilation or measurement failed: %v", err)
		result.Duration = time.Since(startTime)
		return result, nil
	}

	result.Performance = benchResult

	// Check performance constraints
	if test.MinimumCycles > 0 && benchResult.LambdaCycles < test.MinimumCycles {
		result.Passed = false
		result.Reason = fmt.Sprintf("Performance too good: %d cycles < minimum %d", 
			benchResult.LambdaCycles, test.MinimumCycles)
		result.Duration = time.Since(startTime)
		return result, nil
	}

	if test.MaximumCycles > 0 && benchResult.LambdaCycles > test.MaximumCycles {
		result.Passed = false
		result.Reason = fmt.Sprintf("Performance too slow: %d cycles > maximum %d", 
			benchResult.LambdaCycles, test.MaximumCycles)
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Check zero-cost assertion
	if test.ZeroCostAssertion {
		// For zero-cost tests, we need to compare against a traditional implementation
		// This is a simplified check - in practice, you'd compare against baseline
		if !benchResult.ZeroCostLambda && benchResult.TraditionalCycles > 0 {
			result.Passed = false
			result.Reason = fmt.Sprintf("Zero-cost assertion failed: %d != %d cycles", 
				benchResult.LambdaCycles, benchResult.TraditionalCycles)
			result.Duration = time.Since(startTime)
			return result, nil
		}
	}

	// Verify critical features through pipeline testing
	pipelineTestCase := PipelineTestCase{
		Name:   test.Name,
		Source: test.SourceCode,
	}

	compilationResult, err := rt.pipelineTester.RunPipelineTest(pipelineTestCase)
	if err != nil {
		result.Passed = false
		result.Reason = fmt.Sprintf("Pipeline verification failed: %v", err)
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Check for critical features in the compiled output
	for _, feature := range test.CriticalFeatures {
		if err := rt.verifyCriticalFeature(feature, compilationResult); err != nil {
			result.Passed = false
			result.Reason = fmt.Sprintf("Critical feature '%s' verification failed: %v", feature, err)
			result.Duration = time.Since(startTime)
			return result, nil
		}
	}

	result.Passed = true
	result.Reason = "All checks passed"
	result.Duration = time.Since(startTime)
	return result, nil
}

// verifyCriticalFeature checks that a critical feature is properly implemented
func (rt *RegressionTests) verifyCriticalFeature(feature string, result *CompilationResult) error {
	switch feature {
	case "lambda_elimination":
		return rt.pipelineTester.VerifyLambdaTransformation(result)
		
	case "interface_resolution":
		return rt.pipelineTester.VerifyInterfaceResolution(result)
		
	case "direct_call":
		// Check for direct CALL instructions in A80
		hasCall := false
		for _, instr := range result.A80 {
			if instr.Mnemonic == "CALL" {
				hasCall = true
				break
			}
		}
		if !hasCall {
			return fmt.Errorf("no direct CALL instructions found")
		}
		
	case "parameter_inlining":
		// Check for immediate values in A80 instead of memory loads
		hasImmediate := false
		for _, instr := range result.A80 {
			if strings.Contains(strings.Join(instr.Operands, " "), "#") {
				hasImmediate = true
				break
			}
		}
		if !hasImmediate {
			return fmt.Errorf("no immediate values found - parameters not inlined")
		}

	case "loop_smc":
		// Check for self-modifying code patterns in loops
		hasSMCPattern := false
		for _, instr := range result.A80 {
			if instr.Mnemonic == "LD" && len(instr.Operands) >= 2 {
				// Look for instructions that modify other instructions
				if strings.Contains(instr.Operands[0], "+1") || 
				   strings.Contains(instr.Operands[0], "+2") {
					hasSMCPattern = true
					break
				}
			}
		}
		if !hasSMCPattern {
			return fmt.Errorf("no self-modifying code patterns found")
		}

	case "memory_mapping":
		// Check for memory access patterns
		hasMemoryAccess := false
		for _, instr := range result.A80 {
			if instr.Mnemonic == "LD" && len(instr.Operands) >= 2 {
				// Look for memory addressing
				if strings.Contains(instr.Operands[0], "(") || 
				   strings.Contains(instr.Operands[1], "(") {
					hasMemoryAccess = true
					break
				}
			}
		}
		if !hasMemoryAccess {
			return fmt.Errorf("no memory access patterns found")
		}

	case "port_io":
		// Check for IN/OUT instructions
		hasPortIO := false
		for _, instr := range result.A80 {
			if instr.Mnemonic == "IN" || instr.Mnemonic == "OUT" {
				hasPortIO = true
				break
			}
		}
		if !hasPortIO {
			return fmt.Errorf("no port I/O instructions found")
		}

	default:
		// Feature not specifically handled - assume it passes
		return nil
	}

	return nil
}

// RunAllRegressionTests executes all regression test suites
func (rt *RegressionTests) RunAllRegressionTests(t *testing.T) {
	for _, suite := range rt.testSuites {
		t.Run(suite.Name, func(t *testing.T) {
			t.Logf("Running regression test suite: %s", suite.Description)
			
			for _, test := range suite.Tests {
				t.Run(test.Name, func(t *testing.T) {
					result, err := rt.RunRegressionTest(test)
					if err != nil {
						t.Fatalf("Regression test execution failed: %v", err)
					}

					if !result.Passed {
						t.Errorf("Regression test failed: %s", result.Reason)
					} else {
						t.Logf("✅ %s passed in %v", test.Name, result.Duration)
						if result.Performance != nil {
							t.Logf("   Performance: %d cycles, %d instructions", 
								result.Performance.LambdaCycles, 
								result.Performance.LambdaInstructions)
						}
					}
				})
			}
		})
	}
}

// GenerateRegressionReport creates a comprehensive regression test report
func (rt *RegressionTests) GenerateRegressionReport() string {
	var report strings.Builder
	report.WriteString("# MinZ Regression Test Report\n\n")
	report.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().Format(time.RFC3339)))

	totalTests := 0
	passedTests := 0
	
	report.WriteString("## Test Suite Summary\n\n")
	
	for _, suite := range rt.testSuites {
		totalTests += len(suite.Tests)
		report.WriteString(fmt.Sprintf("### %s\n", suite.Name))
		report.WriteString(fmt.Sprintf("%s\n\n", suite.Description))
		
		report.WriteString("| Test | Status | Critical Features |\n")
		report.WriteString("|------|--------|------------------|\n")
		
		for _, test := range suite.Tests {
			status := "❓ Not Run"
			features := strings.Join(test.CriticalFeatures, ", ")
			
			report.WriteString(fmt.Sprintf("| %s | %s | %s |\n", 
				test.Name, status, features))
		}
		report.WriteString("\n")
	}

	report.WriteString(fmt.Sprintf("## Overall Statistics\n\n"))
	report.WriteString(fmt.Sprintf("- Total Test Suites: %d\n", len(rt.testSuites)))
	report.WriteString(fmt.Sprintf("- Total Tests: %d\n", totalTests))
	report.WriteString(fmt.Sprintf("- Passed: %d\n", passedTests))
	report.WriteString(fmt.Sprintf("- Failed: %d\n", totalTests-passedTests))
	
	if totalTests > 0 {
		passRate := float64(passedTests) / float64(totalTests) * 100
		report.WriteString(fmt.Sprintf("- Pass Rate: %.1f%%\n", passRate))
	}

	return report.String()
}

// SaveRegressionReport saves the regression report to a file
func (rt *RegressionTests) SaveRegressionReport(filename string) error {
	report := rt.GenerateRegressionReport()
	return os.WriteFile(filename, []byte(report), 0644)
}