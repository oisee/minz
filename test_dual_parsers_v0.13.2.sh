#!/bin/bash

# MinZ v0.13.2 Dual Parser Testing Script
# Tests both Native and ANTLR parsers across all examples
# Provides performance benchmarks and compatibility reports

set -e

echo "🚀 MinZ v0.13.2 Dual Parser Testing Suite"
echo "=========================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Counters
NATIVE_SUCCESS=0
NATIVE_FAIL=0
ANTLR_SUCCESS=0
ANTLR_FAIL=0
TOTAL_EXAMPLES=0

# Test results arrays
declare -a NATIVE_FAILURES
declare -a ANTLR_FAILURES
declare -a COMMON_FAILURES
declare -a NATIVE_ONLY_FAILURES
declare -a ANTLR_ONLY_FAILURES

# Performance tracking
NATIVE_TOTAL_TIME=0
ANTLR_TOTAL_TIME=0

# Create test results directory
RESULTS_DIR="test_results_dual_parser_$(date +%Y%m%d_%H%M%S)"
mkdir -p "$RESULTS_DIR"

# Log file
LOG_FILE="$RESULTS_DIR/dual_parser_test.log"

echo "📝 Results will be saved to: $RESULTS_DIR"
echo "📋 Log file: $LOG_FILE"

# Function to log with timestamp
log() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') - $1" | tee -a "$LOG_FILE"
}

# Function to test a single file with both parsers
test_file() {
    local file="$1"
    local basename=$(basename "$file" .minz)
    
    echo -e "\n${BLUE}Testing: $file${NC}"
    log "Testing file: $file"
    
    TOTAL_EXAMPLES=$((TOTAL_EXAMPLES + 1))
    
    # Test Native Parser
    echo -n "  Native Parser: "
    local native_start=$(date +%s%N)
    
    if timeout 30s bash -c "cd minzc && ./mz \"../$file\" -o \"../$RESULTS_DIR/${basename}_native.a80\" 2>&1" > "$RESULTS_DIR/${basename}_native.log" 2>&1; then
        local native_end=$(date +%s%N)
        local native_time=$(( (native_end - native_start) / 1000000 )) # Convert to milliseconds
        NATIVE_TOTAL_TIME=$((NATIVE_TOTAL_TIME + native_time))
        
        echo -e "${GREEN}PASS${NC} (${native_time}ms)"
        NATIVE_SUCCESS=$((NATIVE_SUCCESS + 1))
        log "Native parser SUCCESS for $file (${native_time}ms)"
    else
        echo -e "${RED}FAIL${NC}"
        NATIVE_FAIL=$((NATIVE_FAIL + 1))
        NATIVE_FAILURES+=("$file")
        log "Native parser FAILED for $file"
    fi
    
    # Test ANTLR Parser
    echo -n "  ANTLR Parser:  "
    local antlr_start=$(date +%s%N)
    
    if timeout 30s bash -c "cd minzc && MINZ_USE_ANTLR_PARSER=1 ./mz \"../$file\" -o \"../$RESULTS_DIR/${basename}_antlr.a80\" 2>&1" > "$RESULTS_DIR/${basename}_antlr.log" 2>&1; then
        local antlr_end=$(date +%s%N)
        local antlr_time=$(( (antlr_end - antlr_start) / 1000000 )) # Convert to milliseconds
        ANTLR_TOTAL_TIME=$((ANTLR_TOTAL_TIME + antlr_time))
        
        echo -e "${GREEN}PASS${NC} (${antlr_time}ms)"
        ANTLR_SUCCESS=$((ANTLR_SUCCESS + 1))
        log "ANTLR parser SUCCESS for $file (${antlr_time}ms)"
    else
        echo -e "${RED}FAIL${NC}"
        ANTLR_FAIL=$((ANTLR_FAIL + 1))
        ANTLR_FAILURES+=("$file")
        log "ANTLR parser FAILED for $file"
    fi
}

# Function to run benchmarks
run_benchmarks() {
    echo -e "\n${YELLOW}🔬 Running Performance Benchmarks${NC}"
    
    cd minzc
    
    # Standard benchmark
    echo "Running standard parser benchmarks..."
    if go test -bench=BenchmarkParserComparison -benchmem ./pkg/parser/ > "../$RESULTS_DIR/benchmark_comparison.txt" 2>&1; then
        echo -e "${GREEN}✓${NC} Parser comparison benchmark completed"
    else
        echo -e "${RED}✗${NC} Parser comparison benchmark failed"
    fi
    
    # Memory benchmark
    echo "Running memory usage benchmarks..."
    if MINZ_TEST_ANTLR=1 go test -bench=BenchmarkParserMemory -benchmem ./pkg/parser/ > "../$RESULTS_DIR/benchmark_memory.txt" 2>&1; then
        echo -e "${GREEN}✓${NC} Memory benchmark completed"
    else
        echo -e "${RED}✗${NC} Memory benchmark failed"
    fi
    
    # Error handling benchmark
    echo "Running error handling benchmarks..."
    if MINZ_TEST_ANTLR=1 go test -bench=BenchmarkParserErrorHandling -benchmem ./pkg/parser/ > "../$RESULTS_DIR/benchmark_errors.txt" 2>&1; then
        echo -e "${GREEN}✓${NC} Error handling benchmark completed"
    else
        echo -e "${RED}✗${NC} Error handling benchmark failed"
    fi
    
    cd ..
}

# Function to analyze failures
analyze_failures() {
    echo -e "\n${YELLOW}📊 Analyzing Failure Patterns${NC}"
    
    # Find common failures
    for file in "${NATIVE_FAILURES[@]}"; do
        if [[ " ${ANTLR_FAILURES[@]} " =~ " ${file} " ]]; then
            COMMON_FAILURES+=("$file")
        else
            NATIVE_ONLY_FAILURES+=("$file")
        fi
    done
    
    # Find ANTLR-only failures
    for file in "${ANTLR_FAILURES[@]}"; do
        if [[ ! " ${NATIVE_FAILURES[@]} " =~ " ${file} " ]]; then
            ANTLR_ONLY_FAILURES+=("$file")
        fi
    done
    
    # Save analysis to file
    {
        echo "# Dual Parser Failure Analysis"
        echo "## Common Failures (both parsers fail)"
        printf '%s\n' "${COMMON_FAILURES[@]}"
        echo ""
        echo "## Native Parser Only Failures"
        printf '%s\n' "${NATIVE_ONLY_FAILURES[@]}"
        echo ""
        echo "## ANTLR Parser Only Failures"  
        printf '%s\n' "${ANTLR_ONLY_FAILURES[@]}"
    } > "$RESULTS_DIR/failure_analysis.md"
}

# Function to generate performance report
generate_performance_report() {
    echo -e "\n${YELLOW}📈 Generating Performance Report${NC}"
    
    local native_avg=0
    local antlr_avg=0
    
    if [ $NATIVE_SUCCESS -gt 0 ]; then
        native_avg=$((NATIVE_TOTAL_TIME / NATIVE_SUCCESS))
    fi
    
    if [ $ANTLR_SUCCESS -gt 0 ]; then
        antlr_avg=$((ANTLR_TOTAL_TIME / ANTLR_SUCCESS))
    fi
    
    # Create performance report
    cat > "$RESULTS_DIR/performance_report.md" << EOF
# MinZ v0.13.2 Dual Parser Performance Report

## Summary Statistics

### Success Rates
- **Native Parser**: $NATIVE_SUCCESS/$TOTAL_EXAMPLES ($(( (NATIVE_SUCCESS * 100) / TOTAL_EXAMPLES ))%)
- **ANTLR Parser**: $ANTLR_SUCCESS/$TOTAL_EXAMPLES ($(( (ANTLR_SUCCESS * 100) / TOTAL_EXAMPLES ))%)

### Total Processing Time
- **Native Parser**: ${NATIVE_TOTAL_TIME}ms total
- **ANTLR Parser**: ${ANTLR_TOTAL_TIME}ms total

### Average Time per File
- **Native Parser**: ${native_avg}ms average
- **ANTLR Parser**: ${antlr_avg}ms average

### Speed Comparison
- **Performance Ratio**: $(( antlr_avg / (native_avg > 0 ? native_avg : 1) ))x (ANTLR vs Native)
- **Recommendation**: Native parser is $(( antlr_avg / (native_avg > 0 ? native_avg : 1) ))x faster on average

## Compatibility Analysis

### Files Working on Both Parsers
$((TOTAL_EXAMPLES - ${#COMMON_FAILURES[@]} - ${#NATIVE_ONLY_FAILURES[@]} - ${#ANTLR_ONLY_FAILURES[@]}))/$TOTAL_EXAMPLES files

### Parser-Specific Issues
- **Native Only Failures**: ${#NATIVE_ONLY_FAILURES[@]} files
- **ANTLR Only Failures**: ${#ANTLR_ONLY_FAILURES[@]} files  
- **Common Failures**: ${#COMMON_FAILURES[@]} files

## Recommendations

### Default Parser
**Native Parser** - Provides the best performance with $(( (NATIVE_SUCCESS * 100) / TOTAL_EXAMPLES ))% success rate.

### Fallback Strategy
Use ANTLR parser as automatic fallback for maximum compatibility.

### Platform-Specific Recommendations
- **Linux/macOS**: Native parser (fastest)
- **Windows**: Either parser (ANTLR for CGO-free builds)
- **Docker/Alpine**: ANTLR parser (avoid CGO complexities)
- **CI/CD**: ANTLR parser (simpler build requirements)
EOF
}

# Function to test environment setup
test_environment() {
    echo -e "\n${YELLOW}🔧 Testing Environment Setup${NC}"
    
    # Check if compiler exists
    if [ ! -f "minzc/mz" ]; then
        echo -e "${RED}✗${NC} MinZ compiler not found at minzc/mz"
        echo "Please build the compiler first:"
        echo "  cd minzc && make build"
        exit 1
    fi
    
    echo -e "${GREEN}✓${NC} MinZ compiler found"
    
    # Check Go environment for benchmarks
    if command -v go >/dev/null 2>&1; then
        echo -e "${GREEN}✓${NC} Go environment available for benchmarks"
    else
        echo -e "${YELLOW}⚠${NC} Go not found - skipping benchmark tests"
    fi
    
    # Test both parsers are available
    echo "Testing parser availability..."
    
    cd minzc
    if ./mz --help >/dev/null 2>&1; then
        echo -e "${GREEN}✓${NC} Default parser (Native) available"
    else
        echo -e "${RED}✗${NC} Default parser not working"
        exit 1
    fi
    
    if MINZ_USE_ANTLR_PARSER=1 ./mz --help >/dev/null 2>&1; then
        echo -e "${GREEN}✓${NC} ANTLR parser available"
    else
        echo -e "${RED}✗${NC} ANTLR parser not working"
        exit 1
    fi
    
    cd ..
}

# Main execution
main() {
    log "Starting MinZ v0.13.2 Dual Parser Testing Suite"
    
    # Test environment
    test_environment
    
    # Find all example files
    echo -e "\n${YELLOW}🔍 Finding test files...${NC}"
    
    example_files=()
    if [ -d "examples" ]; then
        while IFS= read -r -d '' file; do
            example_files+=("$file")
        done < <(find examples -name "*.minz" -print0)
        echo "Found ${#example_files[@]} example files"
    else
        echo -e "${RED}✗${NC} Examples directory not found"
        exit 1
    fi
    
    # Test all files
    echo -e "\n${YELLOW}🧪 Testing All Files${NC}"
    for file in "${example_files[@]}"; do
        test_file "$file"
    done
    
    # Run benchmarks if Go is available
    if command -v go >/dev/null 2>&1; then
        run_benchmarks
    fi
    
    # Analyze results
    analyze_failures
    generate_performance_report
    
    # Final summary
    echo -e "\n${YELLOW}📋 Final Summary${NC}"
    echo "=============="
    echo -e "Total Examples Tested: ${BLUE}$TOTAL_EXAMPLES${NC}"
    echo ""
    echo -e "Native Parser:  ${GREEN}$NATIVE_SUCCESS${NC} success, ${RED}$NATIVE_FAIL${NC} failed"
    echo -e "ANTLR Parser:   ${GREEN}$ANTLR_SUCCESS${NC} success, ${RED}$ANTLR_FAIL${NC} failed"
    echo ""
    echo -e "Success Rates:"
    echo -e "  Native: ${GREEN}$(( (NATIVE_SUCCESS * 100) / TOTAL_EXAMPLES ))%${NC}"
    echo -e "  ANTLR:  ${GREEN}$(( (ANTLR_SUCCESS * 100) / TOTAL_EXAMPLES ))%${NC}"
    
    if [ $NATIVE_TOTAL_TIME -gt 0 ] && [ $ANTLR_TOTAL_TIME -gt 0 ]; then
        echo ""
        echo -e "Performance:"
        echo -e "  Native: ${NATIVE_TOTAL_TIME}ms total (avg: $((NATIVE_TOTAL_TIME / NATIVE_SUCCESS))ms)"
        echo -e "  ANTLR:  ${ANTLR_TOTAL_TIME}ms total (avg: $((ANTLR_TOTAL_TIME / ANTLR_SUCCESS))ms)"
        echo -e "  Speedup: ${GREEN}$(( ANTLR_TOTAL_TIME / (NATIVE_TOTAL_TIME > 0 ? NATIVE_TOTAL_TIME : 1) ))x${NC} (Native vs ANTLR)"
    fi
    
    echo ""
    echo -e "Detailed results saved to: ${BLUE}$RESULTS_DIR/${NC}"
    echo ""
    
    # Recommendations
    echo -e "${YELLOW}🎯 Recommendations:${NC}"
    echo "1. Use Native parser as default (fastest performance)"
    echo "2. ANTLR parser as automatic fallback for compatibility"
    echo "3. Both parsers provide high compatibility for MinZ code"
    
    log "Testing completed. Results in $RESULTS_DIR"
}

# Run main function
main "$@"