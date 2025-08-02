package e2e

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/remogatto/z80"
)

// PerformanceBenchmarks provides comprehensive performance testing framework
// for MinZ zero-cost abstractions
type PerformanceBenchmarks struct {
	workDir       string
	minzcPath     string
	sjasmplusPath string
	testResults   []BenchmarkResult
}

// BenchmarkResult captures performance metrics for a single test
type BenchmarkResult struct {
	TestName           string
	LambdaCycles       int
	TraditionalCycles  int
	InterfaceCycles    int
	DirectCycles       int
	LambdaInstructions int
	TraditionalInstructions int
	InterfaceInstructions   int
	DirectInstructions      int
	ZeroCostLambda     bool
	ZeroCostInterface  bool
	PerformanceGain    float64
	Timestamp          time.Time
}

// LambdaVsTraditionalTest compares lambda performance against traditional functions
type LambdaVsTraditionalTest struct {
	Name              string
	LambdaSource      string
	TraditionalSource string
	TestFunction      string
	ExpectedResult    interface{}
}

// InterfaceVsDirectTest compares interface method calls vs direct calls
type InterfaceVsDirectTest struct {
	Name           string
	InterfaceSource string
	DirectSource    string
	TestFunction    string
	ExpectedResult  interface{}
}

// NewPerformanceBenchmarks creates a new performance benchmarking framework
func NewPerformanceBenchmarks(t *testing.T) (*PerformanceBenchmarks, error) {
	workDir, err := os.MkdirTemp("", "minz_perf_test_*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Find minzc compiler
	minzcPath := "/Users/alice/dev/minz-ts/minzc/minzc"
	if _, err := os.Stat(minzcPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("minzc compiler not found at %s", minzcPath)
	}

	// Find sjasmplus assembler
	sjasmplusPath := "/Users/alice/dev/bin/sjasmplus"
	if _, err := os.Stat(sjasmplusPath); os.IsNotExist(err) {
		// Try alternative locations
		alternatives := []string{
			"/usr/local/bin/sjasmplus",
			"/opt/homebrew/bin/sjasmplus",
			"sjasmplus", // System PATH
		}
		found := false
		for _, alt := range alternatives {
			if _, err := exec.LookPath(alt); err == nil {
				sjasmplusPath = alt
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("sjasmplus assembler not found")
		}
	}

	return &PerformanceBenchmarks{
		workDir:       workDir,
		minzcPath:     minzcPath,
		sjasmplusPath: sjasmplusPath,
		testResults:   make([]BenchmarkResult, 0),
	}, nil
}

// Cleanup removes temporary files
func (pb *PerformanceBenchmarks) Cleanup() {
	if pb.workDir != "" {
		os.RemoveAll(pb.workDir)
	}
}

// CompileAndMeasure compiles MinZ source and measures performance
func (pb *PerformanceBenchmarks) CompileAndMeasure(name, source string, optimizations []string) (*BenchmarkResult, error) {
	// Create source file
	sourceFile := filepath.Join(pb.workDir, fmt.Sprintf("%s.minz", name))
	if err := os.WriteFile(sourceFile, []byte(source), 0644); err != nil {
		return nil, fmt.Errorf("failed to write source file: %w", err)
	}

	result := &BenchmarkResult{
		TestName:  name,
		Timestamp: time.Now(),
	}

	// Compile with different optimization levels
	for _, opt := range optimizations {
		a80File := filepath.Join(pb.workDir, fmt.Sprintf("%s_%s.a80", name, opt))
		
		// Build compiler command
		args := []string{sourceFile, "-o", a80File}
		if strings.Contains(opt, "smc") {
			args = append(args, "--enable-smc")
		}
		if strings.Contains(opt, "true_smc") {
			args = append(args, "--enable-true-smc")
		}
		if strings.Contains(opt, "O") {
			args = append(args, "-O")
		}

		// Compile
		cmd := exec.Command(pb.minzcPath, args...)
		cmd.Dir = pb.workDir
		if output, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("compilation failed for %s: %s\n%s", opt, err, output)
		}

		// Measure performance
		cycles, instructions, err := pb.measurePerformance(a80File)
		if err != nil {
			return nil, fmt.Errorf("performance measurement failed for %s: %w", opt, err)
		}

		// Store results based on optimization type
		switch {
		case strings.Contains(opt, "lambda"):
			result.LambdaCycles = cycles
			result.LambdaInstructions = instructions
		case strings.Contains(opt, "traditional"):
			result.TraditionalCycles = cycles
			result.TraditionalInstructions = instructions
		case strings.Contains(opt, "interface"):
			result.InterfaceCycles = cycles
			result.InterfaceInstructions = instructions
		case strings.Contains(opt, "direct"):
			result.DirectCycles = cycles
			result.DirectInstructions = instructions
		}
	}

	// Calculate zero-cost analysis
	pb.analyzeZeroCost(result)

	pb.testResults = append(pb.testResults, *result)
	return result, nil
}

// measurePerformance measures cycle count and instruction count for compiled assembly
func (pb *PerformanceBenchmarks) measurePerformance(a80File string) (cycles, instructions int, err error) {
	// Assemble to binary
	binFile := strings.Replace(a80File, ".a80", ".bin", 1)
	cmd := exec.Command(pb.sjasmplusPath, a80File, binFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		return 0, 0, fmt.Errorf("assembly failed: %s\n%s", err, output)
	}

	// Read binary
	binary, err := os.ReadFile(binFile)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read binary: %w", err)
	}

	// Count instructions by parsing assembly
	instructions, err = pb.countInstructions(a80File)
	if err != nil {
		return 0, 0, fmt.Errorf("instruction counting failed: %w", err)
	}

	// Emulate execution for cycle counting
	cycles, err = pb.emulateExecution(binary)
	if err != nil {
		return 0, 0, fmt.Errorf("emulation failed: %w", err)
	}

	return cycles, instructions, nil
}

// countInstructions counts the number of Z80 instructions in assembly file
func (pb *PerformanceBenchmarks) countInstructions(a80File string) (int, error) {
	content, err := os.ReadFile(a80File)
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(content), "\n")
	count := 0
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip comments, labels, and directives
		if line == "" || strings.HasPrefix(line, ";") || 
		   strings.HasPrefix(line, ".") || strings.HasSuffix(line, ":") {
			continue
		}
		// Count actual instructions
		if !strings.HasPrefix(line, "ORG") && !strings.HasPrefix(line, "END") {
			count++
		}
	}
	
	return count, nil
}

// emulateExecution runs the binary in Z80 emulator and counts cycles
func (pb *PerformanceBenchmarks) emulateExecution(binary []byte) (int, error) {
	// Create Z80 CPU
	cpu := z80.NewZ80(nil)
	
	// Load binary at standard location
	memory := make([]byte, 65536)
	copy(memory[0x8000:], binary)
	
	// Set up memory interface
	cpu.Init(func(port byte) byte { return 0 }, // Port input
		func(port, value byte) {}, // Port output
		func(address uint16) byte { return memory[address] }, // Memory read
		func(address uint16, value byte) { memory[address] = value }) // Memory write

	// Set PC to start of program
	cpu.SetPC(0x8000)
	initialStates := cpu.Tstates

	// Execute until RET or halt (max 10000 cycles for safety)
	maxCycles := 10000
	for i := 0; i < maxCycles; i++ {
		// Check if we hit a RET instruction (end of function)
		opcode := memory[cpu.PC()]
		if opcode == 0xC9 { // RET
			cpu.Tick()
			break
		}
		
		// Execute one instruction
		cpu.Tick()
		
		// Safety check for infinite loops
		if cpu.PC() == 0 {
			break
		}
	}

	cycles := cpu.Tstates - initialStates
	return cycles, nil
}

// analyzeZeroCost determines if abstractions truly have zero cost
func (pb *PerformanceBenchmarks) analyzeZeroCost(result *BenchmarkResult) {
	// Lambda zero-cost analysis
	if result.LambdaCycles > 0 && result.TraditionalCycles > 0 {
		result.ZeroCostLambda = result.LambdaCycles == result.TraditionalCycles &&
			result.LambdaInstructions == result.TraditionalInstructions
	}

	// Interface zero-cost analysis
	if result.InterfaceCycles > 0 && result.DirectCycles > 0 {
		result.ZeroCostInterface = result.InterfaceCycles == result.DirectCycles &&
			result.InterfaceInstructions == result.DirectInstructions
	}

	// Calculate overall performance gain
	if result.TraditionalCycles > 0 {
		baseline := float64(result.TraditionalCycles)
		optimized := float64(result.LambdaCycles)
		if optimized > 0 {
			result.PerformanceGain = ((baseline - optimized) / baseline) * 100
		}
	}
}

// RunLambdaVsTraditionalBenchmark performs lambda vs traditional function comparison
func (pb *PerformanceBenchmarks) RunLambdaVsTraditionalBenchmark(test LambdaVsTraditionalTest) (*BenchmarkResult, error) {
	// Test lambda version
	result, err := pb.CompileAndMeasure(
		fmt.Sprintf("%s_lambda", test.Name),
		test.LambdaSource,
		[]string{"lambda", "lambda_O", "lambda_smc"},
	)
	if err != nil {
		return nil, fmt.Errorf("lambda test failed: %w", err)
	}

	// Test traditional version
	tradResult, err := pb.CompileAndMeasure(
		fmt.Sprintf("%s_traditional", test.Name),
		test.TraditionalSource,
		[]string{"traditional", "traditional_O", "traditional_smc"},
	)
	if err != nil {
		return nil, fmt.Errorf("traditional test failed: %w", err)
	}

	// Merge results
	result.TraditionalCycles = tradResult.LambdaCycles // Traditional uses lambda slot
	result.TraditionalInstructions = tradResult.LambdaInstructions

	pb.analyzeZeroCost(result)
	return result, nil
}

// RunInterfaceVsDirectBenchmark performs interface vs direct call comparison
func (pb *PerformanceBenchmarks) RunInterfaceVsDirectBenchmark(test InterfaceVsDirectTest) (*BenchmarkResult, error) {
	// Test interface version
	result, err := pb.CompileAndMeasure(
		fmt.Sprintf("%s_interface", test.Name),
		test.InterfaceSource,
		[]string{"interface", "interface_O"},
	)
	if err != nil {
		return nil, fmt.Errorf("interface test failed: %w", err)
	}

	// Test direct version
	directResult, err := pb.CompileAndMeasure(
		fmt.Sprintf("%s_direct", test.Name),
		test.DirectSource,
		[]string{"direct", "direct_O"},
	)
	if err != nil {
		return nil, fmt.Errorf("direct test failed: %w", err)
	}

	// Merge results
	result.InterfaceCycles = result.LambdaCycles // Interface reuses lambda slot
	result.InterfaceInstructions = result.LambdaInstructions
	result.DirectCycles = directResult.LambdaCycles // Direct reuses lambda slot  
	result.DirectInstructions = directResult.LambdaInstructions

	pb.analyzeZeroCost(result)
	return result, nil
}

// GeneratePerformanceReport creates a comprehensive performance report
func (pb *PerformanceBenchmarks) GeneratePerformanceReport() string {
	var report strings.Builder
	report.WriteString("# MinZ Zero-Cost Abstractions Performance Report\n\n")
	report.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().Format(time.RFC3339)))

	// Summary statistics
	lambdaZeroCost := 0
	interfaceZeroCost := 0
	totalTests := len(pb.testResults)

	for _, result := range pb.testResults {
		if result.ZeroCostLambda {
			lambdaZeroCost++
		}
		if result.ZeroCostInterface {
			interfaceZeroCost++
		}
	}

	report.WriteString("## Summary\n\n")
	report.WriteString(fmt.Sprintf("- Total Tests: %d\n", totalTests))
	report.WriteString(fmt.Sprintf("- Lambda Zero-Cost: %d/%d (%.1f%%)\n", 
		lambdaZeroCost, totalTests, float64(lambdaZeroCost)/float64(totalTests)*100))
	report.WriteString(fmt.Sprintf("- Interface Zero-Cost: %d/%d (%.1f%%)\n\n", 
		interfaceZeroCost, totalTests, float64(interfaceZeroCost)/float64(totalTests)*100))

	// Detailed results
	report.WriteString("## Detailed Results\n\n")
	report.WriteString("| Test | Lambda Cycles | Traditional Cycles | Zero-Cost Lambda | Performance Gain |\n")
	report.WriteString("|------|---------------|-------------------|------------------|------------------|\n")

	for _, result := range pb.testResults {
		zeroCostStatus := "❌"
		if result.ZeroCostLambda {
			zeroCostStatus = "✅"
		}
		
		report.WriteString(fmt.Sprintf("| %s | %d | %d | %s | %.2f%% |\n",
			result.TestName,
			result.LambdaCycles,
			result.TraditionalCycles,
			zeroCostStatus,
			result.PerformanceGain))
	}

	report.WriteString("\n## Instruction Count Analysis\n\n")
	report.WriteString("| Test | Lambda Instructions | Traditional Instructions | Difference |\n")
	report.WriteString("|------|-------------------|------------------------|------------|\n")

	for _, result := range pb.testResults {
		diff := result.LambdaInstructions - result.TraditionalInstructions
		report.WriteString(fmt.Sprintf("| %s | %d | %d | %+d |\n",
			result.TestName,
			result.LambdaInstructions,
			result.TraditionalInstructions,
			diff))
	}

	return report.String()
}

// SaveReport saves the performance report to a file
func (pb *PerformanceBenchmarks) SaveReport(filename string) error {
	report := pb.GeneratePerformanceReport()
	return os.WriteFile(filename, []byte(report), 0644)
}

// LogResults logs benchmark results to the test logger
func (pb *PerformanceBenchmarks) LogResults(t *testing.T) {
	t.Logf("Performance Benchmark Results:")
	for _, result := range pb.testResults {
		t.Logf("  %s:", result.TestName)
		t.Logf("    Lambda: %d cycles, %d instructions", 
			result.LambdaCycles, result.LambdaInstructions)
		t.Logf("    Traditional: %d cycles, %d instructions", 
			result.TraditionalCycles, result.TraditionalInstructions)
		t.Logf("    Zero-Cost: %t, Gain: %.2f%%", 
			result.ZeroCostLambda, result.PerformanceGain)
	}
}

// AssertZeroCost asserts that all tested abstractions have zero cost
func (pb *PerformanceBenchmarks) AssertZeroCost(t *testing.T) {
	for _, result := range pb.testResults {
		if result.LambdaCycles > 0 && result.TraditionalCycles > 0 {
			if !result.ZeroCostLambda {
				t.Errorf("Lambda abstraction in %s is not zero-cost: %d vs %d cycles",
					result.TestName, result.LambdaCycles, result.TraditionalCycles)
			}
		}
		
		if result.InterfaceCycles > 0 && result.DirectCycles > 0 {
			if !result.ZeroCostInterface {
				t.Errorf("Interface abstraction in %s is not zero-cost: %d vs %d cycles",
					result.TestName, result.InterfaceCycles, result.DirectCycles)
			}
		}
	}
}