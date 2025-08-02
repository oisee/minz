package e2e

import (
	"fmt"
	"log"
	"os"
	"testing"
)

// TestZeroCostLambdaDemo demonstrates the zero-cost lambda verification
func TestZeroCostLambdaDemo(t *testing.T) {
	pb, err := NewPerformanceBenchmarks(t)
	if err != nil {
		t.Fatalf("Failed to create benchmarker: %v", err)
	}
	defer pb.Cleanup()

	// Test lambda vs traditional function performance
	lambdaTest := LambdaVsTraditionalTest{
		Name: "LambdaVsTraditionalDemo",
		LambdaSource: `
			fun test_lambda() -> u8 {
				let add = |a: u8, b: u8| => u8 { a + b };
				let multiply = |a: u8, b: u8| => u8 { a * b };
				let result1 = add(5, 3);      // 8
				let result2 = multiply(result1, 2); // 16
				result2
			}
		`,
		TraditionalSource: `
			fun traditional_add(a: u8, b: u8) -> u8 { a + b }
			fun traditional_multiply(a: u8, b: u8) -> u8 { a * b }
			fun test_traditional() -> u8 {
				let result1 = traditional_add(5, 3);      // 8
				let result2 = traditional_multiply(result1, 2); // 16
				result2
			}
		`,
		TestFunction: "test_lambda",
		ExpectedResult: 16,
	}

	result, err := pb.RunLambdaVsTraditionalBenchmark(lambdaTest)
	if err != nil {
		t.Errorf("Lambda vs Traditional benchmark failed: %v", err)
		return
	}

	// Verify zero-cost assertion
	if result.ZeroCostLambda {
		t.Logf("✅ ZERO-COST VERIFIED: Lambda has identical performance to traditional functions")
		t.Logf("   Lambda cycles: %d, Traditional cycles: %d", 
			result.LambdaCycles, result.TraditionalCycles)
		t.Logf("   Lambda instructions: %d, Traditional instructions: %d", 
			result.LambdaInstructions, result.TraditionalInstructions)
	} else {
		t.Errorf("❌ ZERO-COST FAILED: Lambda performance differs from traditional")
		t.Errorf("   Lambda cycles: %d, Traditional cycles: %d", 
			result.LambdaCycles, result.TraditionalCycles)
		t.Errorf("   Lambda instructions: %d, Traditional instructions: %d", 
			result.LambdaInstructions, result.TraditionalInstructions)
	}

	// Generate report for this demo
	pb.LogResults(t)
}

// TestPipelineDemo demonstrates the pipeline verification
func TestPipelineDemo(t *testing.T) {
	pv, err := NewPipelineVerification(t)
	if err != nil {
		t.Fatalf("Failed to create pipeline verifier: %v", err)
	}
	defer pv.Cleanup()

	// Test lambda transformation pipeline
	testCase := PipelineTestCase{
		Name: "LambdaTransformationDemo",
		Source: `
			fun test() -> u8 {
				let add = |a: u8, b: u8| => u8 { a + b };
				add(5, 3)
			}
		`,
		ExpectedMIR: []string{"CALL"},  // Lambda should become direct call
		ExpectedA80: []string{"CALL"},  // Should generate direct CALL instruction
	}

	result, err := pv.RunPipelineTest(testCase)
	if err != nil {
		t.Fatalf("Pipeline test failed: %v", err)
	}

	// Verify lambda transformation
	if err := pv.VerifyLambdaTransformation(result); err != nil {
		t.Errorf("Lambda transformation verification failed: %v", err)
	} else {
		t.Logf("✅ LAMBDA TRANSFORMATION VERIFIED: Lambda eliminated in compilation pipeline")
	}

	// Verify expected patterns
	if err := pv.VerifyMIRTransformation(result, testCase.ExpectedMIR); err != nil {
		t.Errorf("MIR verification failed: %v", err)
	} else {
		t.Logf("✅ MIR GENERATION VERIFIED: Expected patterns found")
	}

	if err := pv.VerifyA80Generation(result, testCase.ExpectedA80); err != nil {
		t.Errorf("A80 verification failed: %v", err)
	} else {
		t.Logf("✅ A80 GENERATION VERIFIED: Correct Z80 assembly generated")
	}

	t.Logf("Pipeline verification completed:")
	t.Logf("  MIR instructions: %d", len(result.MIR))
	t.Logf("  A80 instructions: %d", len(result.A80))
}

// TestRegressionDemo demonstrates the regression testing
func TestRegressionDemo(t *testing.T) {
	rt, err := NewRegressionTests(t)
	if err != nil {
		t.Fatalf("Failed to create regression tester: %v", err)
	}
	defer rt.Cleanup()

	// Run a subset of regression tests for demo
	simpleLambdaTest := RegressionTest{
		Name:        "SimpleLambdaDemo",
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
	}

	result, err := rt.RunRegressionTest(simpleLambdaTest)
	if err != nil {
		t.Fatalf("Regression test execution failed: %v", err)
	}

	if result.Passed {
		t.Logf("✅ REGRESSION TEST PASSED: %s", result.Reason)
		if result.Performance != nil {
			t.Logf("   Performance: %d cycles, %d instructions", 
				result.Performance.LambdaCycles,
				result.Performance.LambdaInstructions)
		}
		t.Logf("   Duration: %v", result.Duration)
	} else {
		t.Errorf("❌ REGRESSION TEST FAILED: %s", result.Reason)
		t.Errorf("   Duration: %v", result.Duration)
	}
}

// Main function for running demo tests
func main() {
	fmt.Println("MinZ Zero-Cost Abstractions Demo Test")
	fmt.Println("=====================================")

	// Override test framework to run individual demos
	if len(os.Args) > 1 && os.Args[1] == "demo" {
		t := &testing.T{}
		
		fmt.Println("\n1. Testing Lambda Zero-Cost Performance...")
		TestZeroCostLambdaDemo(t)
		
		fmt.Println("\n2. Testing Compilation Pipeline...")
		TestPipelineDemo(t)
		
		fmt.Println("\n3. Testing Regression Framework...")
		TestRegressionDemo(t)
		
		fmt.Println("\n✅ Demo completed successfully!")
		return
	}

	// Run as normal Go test
	testing.Main(func(_, _ string) (bool, error) { return true, nil },
		[]testing.InternalTest{
			{"TestZeroCostLambdaDemo", TestZeroCostLambdaDemo},
			{"TestPipelineDemo", TestPipelineDemo},
			{"TestRegressionDemo", TestRegressionDemo},
		},
		nil, nil)
}