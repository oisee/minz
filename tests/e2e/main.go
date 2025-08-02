package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go [performance|pipeline|regression|all]")
		os.Exit(1)
	}

	testType := os.Args[1]
	workingDir := "/Users/alice/dev/minz-ts"

	// Change to working directory
	if err := os.Chdir(workingDir); err != nil {
		log.Fatalf("Failed to change directory: %v", err)
	}

	fmt.Println("=== MinZ E2E Testing Framework ===")
	fmt.Printf("Running test type: %s\n", testType)
	fmt.Printf("Working directory: %s\n", workingDir)

	switch testType {
	case "performance":
		runPerformanceTests()
	case "pipeline":
		runPipelineTests()
	case "regression":
		runRegressionTests()
	case "all":
		runPerformanceTests()
		runPipelineTests()
		runRegressionTests()
	default:
		fmt.Printf("Unknown test type: %s\n", testType)
		os.Exit(1)
	}
}

func runPerformanceTests() {
	fmt.Println("\n=== Performance Benchmarks ===")
	
	// Test lambda vs traditional performance
	testLambdaPerformance()
	
	// Test interface vs direct call performance
	testInterfacePerformance()
}

func runPipelineTests() {
	fmt.Println("\n=== Pipeline Verification ===")
	
	// Test AST-MIR-A80 pipeline
	testCompilationPipeline()
}

func runRegressionTests() {
	fmt.Println("\n=== Regression Tests ===")
	
	// Run all regression tests
	testZeroCostAbstractions()
}

func testLambdaPerformance() {
	fmt.Println("Testing Lambda vs Traditional Performance...")
	
	// Compile lambda test
	lambdaFile := "tests/e2e/testdata/lambda_zero_cost_test.minz"
	lambdaOutput := "lambda_test.a80"
	
	cmd := exec.Command("./minzc/minzc", lambdaFile, "-o", lambdaOutput, "-O", "--enable-smc")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Lambda compilation failed: %v\nOutput: %s\n", err, output)
		return
	}
	
	// Analyze assembly output
	assemblyContent, err := os.ReadFile(lambdaOutput)
	if err != nil {
		fmt.Printf("Failed to read assembly: %v\n", err)
		return
	}
	
	// Count instructions and analyze
	instructionCount := countInstructions(string(assemblyContent))
	fmt.Printf("Lambda test compiled successfully - %d instructions\n", instructionCount)
	
	// Look for zero-cost patterns
	if containsOptimizedPatterns(string(assemblyContent)) {
		fmt.Println("✅ Lambda optimization patterns detected")
	} else {
		fmt.Println("⚠️  Lambda optimization patterns not found")
	}
}

func testInterfacePerformance() {
	fmt.Println("Testing Interface vs Direct Call Performance...")
	
	// Compile interface test
	interfaceFile := "tests/e2e/testdata/interface_zero_cost_test.minz"
	interfaceOutput := "interface_test.a80"
	
	cmd := exec.Command("./minzc/minzc", interfaceFile, "-o", interfaceOutput, "-O", "--enable-smc")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Interface compilation failed: %v\nOutput: %s\n", err, output)
		return
	}
	
	// Analyze assembly output
	assemblyContent, err := os.ReadFile(interfaceOutput)
	if err != nil {
		fmt.Printf("Failed to read assembly: %v\n", err)
		return
	}
	
	// Look for direct calls (no vtables)
	if containsDirectCalls(string(assemblyContent)) {
		fmt.Println("✅ Interface calls optimized to direct calls")
	} else {
		fmt.Println("⚠️  Interface vtable overhead detected")
	}
}

func testCompilationPipeline() {
	fmt.Println("Testing AST-MIR-A80 Pipeline...")
	
	// Test with combined abstractions file
	testFile := "tests/e2e/testdata/combined_zero_cost_test.minz"
	outputFile := "pipeline_test.a80"
	
	cmd := exec.Command("./minzc/minzc", testFile, "-o", outputFile, "-O", "--enable-smc", "--verbose")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Pipeline test failed: %v\nOutput: %s\n", err, output)
		return
	}
	
	fmt.Println("✅ Complete AST-MIR-A80 pipeline functional")
}

func testZeroCostAbstractions() {
	fmt.Println("Running Zero-Cost Abstraction Regression Tests...")
	
	testFiles := []string{
		"examples/zero_cost_test.minz",
		"examples/interface_simple.minz", 
		"examples/lambda_transform_test.minz",
	}
	
	for _, file := range testFiles {
		fmt.Printf("Testing %s...\n", filepath.Base(file))
		
		outputFile := strings.TrimSuffix(filepath.Base(file), ".minz") + "_test.a80"
		cmd := exec.Command("./minzc/minzc", file, "-o", outputFile, "-O", "--enable-smc")
		
		if err := cmd.Run(); err != nil {
			fmt.Printf("❌ %s failed to compile\n", file)
		} else {
			fmt.Printf("✅ %s compiled successfully\n", file)
		}
	}
}

func countInstructions(assembly string) int {
	lines := strings.Split(assembly, "\n")
	count := 0
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, ";") && !strings.HasPrefix(line, ".") && strings.Contains(line, " ") {
			count++
		}
	}
	
	return count
}

func containsOptimizedPatterns(assembly string) bool {
	// Look for SMC patterns and direct calls
	return strings.Contains(assembly, "CALL") && 
		   strings.Contains(assembly, "$") && // SMC labels
		   !strings.Contains(assembly, "vtable") // No vtables
}

func containsDirectCalls(assembly string) bool {
	// Interface methods should compile to direct CALL instructions
	return strings.Contains(assembly, "CALL") && 
		   !strings.Contains(assembly, "JP (HL)") && // No indirect jumps
		   !strings.Contains(assembly, "vtable")     // No vtables
}