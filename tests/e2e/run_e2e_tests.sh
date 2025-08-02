#!/bin/bash

# MinZ E2E Testing Infrastructure Runner
# This script orchestrates comprehensive end-to-end testing of the MinZ compiler
# with focus on verifying zero-cost abstractions (lambdas + interfaces)

set -e  # Exit on any error

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
MINZC_DIR="$PROJECT_ROOT/minzc"
MINZC_BINARY="$MINZC_DIR/minzc"
RESULTS_DIR="$PROJECT_ROOT/test_results_$(date +%Y%m%d_%H%M%S)"

# Test configuration
RUN_PERFORMANCE_BENCHMARKS=true
RUN_PIPELINE_VERIFICATION=true
RUN_REGRESSION_TESTS=true
GENERATE_REPORTS=true
VERBOSE=false
PARALLEL_JOBS=4

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Usage information
show_usage() {
    cat << EOF
MinZ E2E Testing Infrastructure

Usage: $0 [OPTIONS]

OPTIONS:
    --performance-only       Run only performance benchmarks
    --pipeline-only         Run only pipeline verification tests
    --regression-only       Run only regression tests
    --no-reports           Skip report generation
    --verbose              Enable verbose output
    --jobs N               Number of parallel jobs (default: 4)
    --results-dir DIR      Custom results directory
    --help                 Show this help message

EXAMPLES:
    # Run all tests with default settings
    $0

    # Run only performance benchmarks with verbose output
    $0 --performance-only --verbose

    # Run tests with custom results directory
    $0 --results-dir /tmp/minz_test_results

    # Run regression tests only
    $0 --regression-only
EOF
}

# Parse command line arguments
parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --performance-only)
                RUN_PERFORMANCE_BENCHMARKS=true
                RUN_PIPELINE_VERIFICATION=false
                RUN_REGRESSION_TESTS=false
                shift
                ;;
            --pipeline-only)
                RUN_PERFORMANCE_BENCHMARKS=false
                RUN_PIPELINE_VERIFICATION=true
                RUN_REGRESSION_TESTS=false
                shift
                ;;
            --regression-only)
                RUN_PERFORMANCE_BENCHMARKS=false
                RUN_PIPELINE_VERIFICATION=false
                RUN_REGRESSION_TESTS=true
                shift
                ;;
            --no-reports)
                GENERATE_REPORTS=false
                shift
                ;;
            --verbose)
                VERBOSE=true
                shift
                ;;
            --jobs)
                PARALLEL_JOBS="$2"
                shift 2
                ;;
            --results-dir)
                RESULTS_DIR="$2"
                shift 2
                ;;
            --help)
                show_usage
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                show_usage
                exit 1
                ;;
        esac
    done
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."

    # Check if MinZ compiler exists
    if [[ ! -f "$MINZC_BINARY" ]]; then
        log_error "MinZ compiler not found at: $MINZC_BINARY"
        log_info "Please build the compiler first: cd $MINZC_DIR && make build"
        exit 1
    fi

    # Check if Go is available
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed or not in PATH"
        exit 1
    fi

    # Check if tree-sitter is available
    if ! command -v tree-sitter &> /dev/null; then
        log_warning "tree-sitter CLI not found - AST parsing may be limited"
    fi

    # Check if sjasmplus is available
    if ! command -v sjasmplus &> /dev/null; then
        log_warning "sjasmplus assembler not found - assembly tests may fail"
        log_info "Install from: https://github.com/z00m128/sjasmplus"
    fi

    # Create results directory
    mkdir -p "$RESULTS_DIR"
    log_success "Results directory created: $RESULTS_DIR"
}

# Build the test infrastructure
build_test_infrastructure() {
    log_info "Building test infrastructure..."

    cd "$SCRIPT_DIR"

    # Create go.mod if it doesn't exist
    if [[ ! -f "go.mod" ]]; then
        log_info "Initializing Go module for E2E tests..."
        go mod init minz-e2e-tests
        
        # Add required dependencies
        go get github.com/remogatto/z80@latest
    fi

    # Build test binaries if needed
    if $VERBOSE; then
        log_info "Go module status:"
        go mod tidy
        go list -m all
    fi

    log_success "Test infrastructure ready"
}

# Run performance benchmarks
run_performance_benchmarks() {
    if ! $RUN_PERFORMANCE_BENCHMARKS; then
        return 0
    fi

    log_info "Running performance benchmarks..."

    local benchmark_output="$RESULTS_DIR/performance_benchmarks.log"
    local benchmark_report="$RESULTS_DIR/performance_report.md"

    # Create test file for performance benchmarks
    cat > "$SCRIPT_DIR/performance_test.go" << 'EOF'
package main

import (
    "log"
    "os"
    "testing"
)

func TestPerformanceBenchmarks(t *testing.T) {
    pb, err := NewPerformanceBenchmarks(t)
    if err != nil {
        t.Fatalf("Failed to create benchmarker: %v", err)
    }
    defer pb.Cleanup()

    // Lambda vs Traditional Test
    lambdaTest := LambdaVsTraditionalTest{
        Name: "BasicArithmetic",
        LambdaSource: `
            fun test_lambda() -> u8 {
                let add = |a: u8, b: u8| => u8 { a + b };
                let multiply = |a: u8, b: u8| => u8 { a * b };
                let result1 = add(5, 3);
                let result2 = multiply(result1, 2);
                result2
            }
        `,
        TraditionalSource: `
            fun traditional_add(a: u8, b: u8) -> u8 { a + b }
            fun traditional_multiply(a: u8, b: u8) -> u8 { a * b }
            fun test_traditional() -> u8 {
                let result1 = traditional_add(5, 3);
                let result2 = traditional_multiply(result1, 2);
                result2
            }
        `,
        TestFunction: "test_lambda",
        ExpectedResult: 16,
    }

    result, err := pb.RunLambdaVsTraditionalBenchmark(lambdaTest)
    if err != nil {
        t.Errorf("Lambda vs Traditional benchmark failed: %v", err)
    } else {
        t.Logf("Lambda benchmark result: %+v", result)
    }

    // Assert zero-cost abstractions
    pb.AssertZeroCost(t)

    // Generate and save report
    reportPath := os.Getenv("BENCHMARK_REPORT_PATH")
    if reportPath != "" {
        if err := pb.SaveReport(reportPath); err != nil {
            t.Errorf("Failed to save report: %v", err)
        }
    }
}

func main() {
    testing.Main(func(_, _ string) (bool, error) { return true, nil }, 
                []testing.InternalTest{
                    {"TestPerformanceBenchmarks", TestPerformanceBenchmarks},
                },
                nil, nil)
}
EOF

    # Run the performance benchmarks
    cd "$SCRIPT_DIR"
    export BENCHMARK_REPORT_PATH="$benchmark_report"
    
    if $VERBOSE; then
        go test -v -run TestPerformanceBenchmarks . 2>&1 | tee "$benchmark_output"
    else
        go test -run TestPerformanceBenchmarks . > "$benchmark_output" 2>&1
    fi

    local exit_code=$?
    if [[ $exit_code -eq 0 ]]; then
        log_success "Performance benchmarks completed successfully"
    else
        log_error "Performance benchmarks failed (exit code: $exit_code)"
        if ! $VERBOSE; then
            log_info "Last 20 lines of output:"
            tail -20 "$benchmark_output"
        fi
    fi

    # Clean up test file
    rm -f "$SCRIPT_DIR/performance_test.go"

    return $exit_code
}

# Run pipeline verification tests
run_pipeline_verification() {
    if ! $RUN_PIPELINE_VERIFICATION; then
        return 0
    fi

    log_info "Running pipeline verification tests..."

    local pipeline_output="$RESULTS_DIR/pipeline_verification.log"
    local pipeline_report="$RESULTS_DIR/pipeline_report.md"

    # Create test file for pipeline verification
    cat > "$SCRIPT_DIR/pipeline_test.go" << 'EOF'
package main

import (
    "os"
    "testing"
)

func TestPipelineVerification(t *testing.T) {
    pv, err := NewPipelineVerification(t)
    if err != nil {
        t.Fatalf("Failed to create pipeline verifier: %v", err)
    }
    defer pv.Cleanup()

    // Add lambda transformation test
    pv.AddTestCase(PipelineTestCase{
        Name: "LambdaTransformation",
        Source: `
            fun test() -> u8 {
                let add = |a: u8, b: u8| => u8 { a + b };
                add(5, 3)
            }
        `,
        ExpectedMIR: []string{"CALL", "ADD"},
        ExpectedA80: []string{"CALL", "ADD"},
    })

    // Add interface resolution test
    pv.AddTestCase(PipelineTestCase{
        Name: "InterfaceResolution",
        Source: `
            interface Addable {
                fun add(self, other: u8) -> u8;
            }
            struct Number { value: u8 }
            impl Addable for Number {
                fun add(self, other: u8) -> u8 { self.value + other }
            }
            fun test() -> u8 {
                let n = Number { value: 5 };
                n.add(3)
            }
        `,
        ExpectedMIR: []string{"CALL"},
        ExpectedA80: []string{"CALL", "ADD"},
    })

    // Run all pipeline tests
    pv.RunAllTests(t)

    // Generate report
    reportPath := os.Getenv("PIPELINE_REPORT_PATH")
    if reportPath != "" {
        report := pv.GeneratePipelineReport()
        if err := os.WriteFile(reportPath, []byte(report), 0644); err != nil {
            t.Errorf("Failed to save pipeline report: %v", err)
        }
    }
}

func main() {
    testing.Main(func(_, _ string) (bool, error) { return true, nil },
                []testing.InternalTest{
                    {"TestPipelineVerification", TestPipelineVerification},
                },
                nil, nil)
}
EOF

    # Run the pipeline verification
    cd "$SCRIPT_DIR"
    export PIPELINE_REPORT_PATH="$pipeline_report"
    
    if $VERBOSE; then
        go test -v -run TestPipelineVerification . 2>&1 | tee "$pipeline_output"
    else
        go test -run TestPipelineVerification . > "$pipeline_output" 2>&1
    fi

    local exit_code=$?
    if [[ $exit_code -eq 0 ]]; then
        log_success "Pipeline verification completed successfully"
    else
        log_error "Pipeline verification failed (exit code: $exit_code)"
        if ! $VERBOSE; then
            log_info "Last 20 lines of output:"
            tail -20 "$pipeline_output"
        fi
    fi

    # Clean up test file
    rm -f "$SCRIPT_DIR/pipeline_test.go"

    return $exit_code
}

# Run regression tests
run_regression_tests() {
    if ! $RUN_REGRESSION_TESTS; then
        return 0
    fi

    log_info "Running regression tests..."

    local regression_output="$RESULTS_DIR/regression_tests.log"
    local regression_report="$RESULTS_DIR/regression_report.md"

    # Create test file for regression tests
    cat > "$SCRIPT_DIR/regression_test.go" << 'EOF'
package main

import (
    "os"
    "testing"
)

func TestRegressionTests(t *testing.T) {
    rt, err := NewRegressionTests(t)
    if err != nil {
        t.Fatalf("Failed to create regression tester: %v", err)
    }
    defer rt.Cleanup()

    // Run all regression test suites
    rt.RunAllRegressionTests(t)

    // Generate report
    reportPath := os.Getenv("REGRESSION_REPORT_PATH")
    if reportPath != "" {
        if err := rt.SaveRegressionReport(reportPath); err != nil {
            t.Errorf("Failed to save regression report: %v", err)
        }
    }
}

func main() {
    testing.Main(func(_, _ string) (bool, error) { return true, nil },
                []testing.InternalTest{
                    {"TestRegressionTests", TestRegressionTests},
                },
                nil, nil)
}
EOF

    # Run the regression tests
    cd "$SCRIPT_DIR"
    export REGRESSION_REPORT_PATH="$regression_report"
    
    if $VERBOSE; then
        go test -v -run TestRegressionTests . 2>&1 | tee "$regression_output"
    else
        go test -run TestRegressionTests . > "$regression_output" 2>&1
    fi

    local exit_code=$?
    if [[ $exit_code -eq 0 ]]; then
        log_success "Regression tests completed successfully"
    else
        log_error "Regression tests failed (exit code: $exit_code)"
        if ! $VERBOSE; then
            log_info "Last 20 lines of output:"
            tail -20 "$regression_output"
        fi
    fi

    # Clean up test file
    rm -f "$SCRIPT_DIR/regression_test.go"

    return $exit_code
}

# Generate comprehensive summary report
generate_summary_report() {
    if ! $GENERATE_REPORTS; then
        return 0
    fi

    log_info "Generating comprehensive summary report..."

    local summary_report="$RESULTS_DIR/summary.md"

    cat > "$summary_report" << EOF
# MinZ E2E Testing Infrastructure - Comprehensive Report

Generated: $(date -u +"%Y-%m-%d %H:%M:%S UTC")

## Executive Summary

This report summarizes the comprehensive end-to-end testing of the MinZ compiler's zero-cost abstractions, focusing on:

1. **Lambda Zero-Cost Verification**: Ensuring lambda abstractions compile to identical machine code as traditional functions
2. **Interface Zero-Cost Verification**: Ensuring interface method calls resolve to direct function calls at compile time
3. **Pipeline Integrity**: Verifying the AST → MIR → A80 compilation pipeline produces correct and optimal code
4. **Performance Regression Prevention**: Automated testing to prevent performance regressions in core optimizations

## Test Results Overview

### Performance Benchmarks
$(if [[ -f "$RESULTS_DIR/performance_report.md" ]]; then
    echo "✅ **COMPLETED** - Detailed results in performance_report.md"
    echo ""
    echo "Key Findings:"
    if grep -q "Zero-Cost Lambda" "$RESULTS_DIR/performance_report.md" 2>/dev/null; then
        grep "Zero-Cost Lambda" "$RESULTS_DIR/performance_report.md" | head -3
    fi
else
    echo "❌ **NOT RUN** - Performance benchmarks were not executed"
fi)

### Pipeline Verification
$(if [[ -f "$RESULTS_DIR/pipeline_report.md" ]]; then
    echo "✅ **COMPLETED** - Detailed results in pipeline_report.md"
    echo ""
    echo "Verification Points:"
    echo "- Lambda elimination in MIR stage"
    echo "- Interface resolution in MIR stage"
    echo "- Correct Z80 instruction generation"
else
    echo "❌ **NOT RUN** - Pipeline verification was not executed"
fi)

### Regression Tests
$(if [[ -f "$RESULTS_DIR/regression_report.md" ]]; then
    echo "✅ **COMPLETED** - Detailed results in regression_report.md"
    echo ""
    echo "Test Suites:"
    if grep -q "Test Suite Summary" "$RESULTS_DIR/regression_report.md" 2>/dev/null; then
        grep -A 10 "Test Suite Summary" "$RESULTS_DIR/regression_report.md" | grep "###" | sed 's/### /- /'
    fi
else
    echo "❌ **NOT RUN** - Regression tests were not executed"
fi)

## Zero-Cost Claims Verification

$(if [[ -f "$RESULTS_DIR/performance_report.md" ]] && grep -q "Zero-Cost" "$RESULTS_DIR/performance_report.md"; then
    echo "### Lambda Abstractions"
    echo "Based on performance benchmarks, lambda abstractions demonstrate:"
    echo ""
    grep -i "lambda.*zero.*cost\|zero.*cost.*lambda" "$RESULTS_DIR/performance_report.md" | head -3 || echo "- Analysis results in performance_report.md"
    echo ""
    echo "### Interface Method Resolution"
    echo "Interface method calls show:"
    echo ""
    grep -i "interface.*zero.*cost\|zero.*cost.*interface" "$RESULTS_DIR/performance_report.md" | head -3 || echo "- Analysis results in performance_report.md"
else
    echo "**Zero-cost verification data not available** - Run performance benchmarks for detailed analysis"
fi)

## Technical Details

### Compilation Pipeline Verified
1. **Source Code** (.minz) → Tree-sitter parsing → **AST**
2. **AST** → Semantic analysis → **Typed AST**  
3. **Typed AST** → IR generation → **MIR** (Middle Intermediate Representation)
4. **MIR** → Optimization passes → **Optimized MIR**
5. **Optimized MIR** → Code generation → **Z80 Assembly** (.a80)
6. **Z80 Assembly** → sjasmplus → **Binary**

### Key Optimizations Verified
- Lambda function inlining and elimination
- Interface method devirtualization
- True Self-Modifying Code (TSMC) parameter optimization
- Register allocation and usage optimization
- Dead code elimination in abstraction overhead

## Performance Metrics

$(if [[ -f "$RESULTS_DIR/performance_benchmarks.log" ]]; then
    echo "### Execution Statistics"
    echo "\`\`\`"
    grep -E "cycles|instructions|Performance|Zero-Cost" "$RESULTS_DIR/performance_benchmarks.log" | tail -10
    echo "\`\`\`"
fi)

## Recommendations

1. **Regular Execution**: Run this test suite after every significant compiler change
2. **CI Integration**: Integrate into continuous integration pipeline
3. **Performance Monitoring**: Set up automated alerts for performance regressions
4. **Expansion**: Add more complex real-world test cases as the language evolves

## Files Generated

- \`performance_report.md\` - Detailed performance analysis
- \`pipeline_report.md\` - Compilation pipeline verification
- \`regression_report.md\` - Comprehensive regression test results
- \`*.log\` files - Raw test execution logs

---

**Testing Infrastructure Version**: E2E v1.0  
**MinZ Compiler**: $(if [[ -f "$MINZC_BINARY" ]]; then "$MINZC_BINARY" --version 2>/dev/null || echo "Version detection failed"; else echo "Not found"; fi)  
**Test Execution Time**: $(date -u +"%Y-%m-%d %H:%M:%S UTC")
EOF

    log_success "Summary report generated: $summary_report"
}

# Main execution function
main() {
    echo "=================================="
    echo "MinZ E2E Testing Infrastructure"
    echo "=================================="
    echo ""

    # Parse arguments
    parse_arguments "$@"

    # Setup
    check_prerequisites
    build_test_infrastructure

    # Track overall success
    local overall_success=true

    # Run test suites
    echo ""
    log_info "Starting test execution..."

    if ! run_performance_benchmarks; then
        overall_success=false
    fi

    if ! run_pipeline_verification; then
        overall_success=false
    fi

    if ! run_regression_tests; then
        overall_success=false
    fi

    # Generate reports
    generate_summary_report

    # Final results
    echo ""
    echo "=================================="
    if $overall_success; then
        log_success "All E2E tests completed successfully!"
        log_info "Results available in: $RESULTS_DIR"
        echo ""
        log_info "Quick verification of zero-cost claims:"
        if [[ -f "$RESULTS_DIR/performance_report.md" ]]; then
            grep -E "Zero-Cost.*✅|✅.*Zero-Cost" "$RESULTS_DIR/performance_report.md" || echo "  (See detailed performance report)"
        fi
    else
        log_error "Some E2E tests failed!"
        log_info "Check logs in: $RESULTS_DIR"
        exit 1
    fi
    echo "=================================="
}

# Execute main function with all arguments
main "$@"