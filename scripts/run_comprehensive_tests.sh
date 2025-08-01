#!/bin/bash

# MinZ Comprehensive Test Runner
# Executes all examples with full verification and performance analysis

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MINZ_ROOT="$(dirname "$SCRIPT_DIR")"
MINZC="$MINZ_ROOT/minzc/minzc"
EXAMPLES_DIR="$MINZ_ROOT/examples"
TEST_OUTPUT_DIR="$MINZ_ROOT/test_results_$(date +%Y%m%d_%H%M%S)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${BLUE}╔══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║          MinZ Comprehensive Test Suite v1.0                  ║${NC}"
echo -e "${BLUE}╚══════════════════════════════════════════════════════════════╝${NC}"
echo

# Create test output directory
mkdir -p "$TEST_OUTPUT_DIR"
mkdir -p "$TEST_OUTPUT_DIR/normal"
mkdir -p "$TEST_OUTPUT_DIR/optimized"
mkdir -p "$TEST_OUTPUT_DIR/logs"

# Ensure compiler is built
if [ ! -f "$MINZC" ]; then
    echo -e "${YELLOW}Building MinZ compiler...${NC}"
    cd "$MINZ_ROOT/minzc"
    make build
    cd "$MINZ_ROOT"
fi

# Test result tracking
TOTAL_TESTS=0
PASSED_COMPILATION=0
PASSED_OPTIMIZATION=0
PASSED_EXECUTION=0
PERFORMANCE_IMPROVEMENTS=0

# Arrays for results
declare -a TEST_RESULTS
declare -a PERFORMANCE_DATA

# Function to test a single example
test_example() {
    local file="$1"
    local base_name=$(basename "$file" .minz)
    local rel_path=$(python3 -c "import os; print(os.path.relpath('$file', '$EXAMPLES_DIR'))")
    
    echo -e "\n${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}Testing: $rel_path${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    
    ((TOTAL_TESTS++))
    
    # Test 1: Normal compilation
    echo -n "  1. Normal compilation... "
    if "$MINZC" "$file" -o "$TEST_OUTPUT_DIR/normal/${base_name}.a80" \
        >"$TEST_OUTPUT_DIR/logs/${base_name}_normal.log" 2>&1; then
        echo -e "${GREEN}✓ PASS${NC}"
        ((PASSED_COMPILATION++))
        NORMAL_SIZE=$(wc -c < "$TEST_OUTPUT_DIR/normal/${base_name}.a80")
        
        # Check if MIR was generated
        if [ -f "$TEST_OUTPUT_DIR/normal/${base_name}.mir" ]; then
            NORMAL_MIR_INSTRUCTIONS=$(grep -c "^      [0-9]:" "$TEST_OUTPUT_DIR/normal/${base_name}.mir" || echo 0)
        else
            NORMAL_MIR_INSTRUCTIONS=0
        fi
    else
        echo -e "${RED}✗ FAIL${NC}"
        echo "    Error: $(head -1 "$TEST_OUTPUT_DIR/logs/${base_name}_normal.log")"
        TEST_RESULTS+=("$rel_path,FAIL,Compilation failed")
        return 1
    fi
    
    # Test 2: Optimized compilation
    echo -n "  2. Optimized compilation... "
    if "$MINZC" "$file" -O --enable-smc -o "$TEST_OUTPUT_DIR/optimized/${base_name}.a80" \
        >"$TEST_OUTPUT_DIR/logs/${base_name}_optimized.log" 2>&1; then
        echo -e "${GREEN}✓ PASS${NC}"
        ((PASSED_OPTIMIZATION++))
        OPT_SIZE=$(wc -c < "$TEST_OUTPUT_DIR/optimized/${base_name}.a80")
        
        # Check if MIR was generated
        if [ -f "$TEST_OUTPUT_DIR/optimized/${base_name}.mir" ]; then
            OPT_MIR_INSTRUCTIONS=$(grep -c "^      [0-9]:" "$TEST_OUTPUT_DIR/optimized/${base_name}.mir" || echo 0)
        else
            OPT_MIR_INSTRUCTIONS=0
        fi
    else
        echo -e "${RED}✗ FAIL${NC}"
        echo "    Error: $(head -1 "$TEST_OUTPUT_DIR/logs/${base_name}_optimized.log")"
        TEST_RESULTS+=("$rel_path,FAIL,Optimization failed")
        return 1
    fi
    
    # Test 3: Size comparison
    echo -n "  3. Size comparison... "
    SIZE_REDUCTION=$(python3 -c "print(f'{(1 - $OPT_SIZE/$NORMAL_SIZE)*100:.1f}')")
    if (( $(echo "$OPT_SIZE <= $NORMAL_SIZE" | bc -l) )); then
        echo -e "${GREEN}✓ Optimized: ${OPT_SIZE} bytes (${SIZE_REDUCTION}% smaller)${NC}"
    else
        echo -e "${YELLOW}⚠ Optimized is larger: ${OPT_SIZE} vs ${NORMAL_SIZE} bytes${NC}"
    fi
    
    # Test 4: Instruction count comparison (if MIR available)
    if [ $NORMAL_MIR_INSTRUCTIONS -gt 0 ] && [ $OPT_MIR_INSTRUCTIONS -gt 0 ]; then
        echo -n "  4. MIR instruction count... "
        INST_REDUCTION=$(python3 -c "print(f'{(1 - $OPT_MIR_INSTRUCTIONS/$NORMAL_MIR_INSTRUCTIONS)*100:.1f}')")
        echo -e "${GREEN}Normal: $NORMAL_MIR_INSTRUCTIONS → Optimized: $OPT_MIR_INSTRUCTIONS (${INST_REDUCTION}% reduction)${NC}"
    fi
    
    # Test 5: Check for specific optimizations
    echo -n "  5. Optimization features... "
    FEATURES=""
    
    # Check for TSMC anchors
    if grep -q "anchor" "$TEST_OUTPUT_DIR/optimized/${base_name}.a80"; then
        FEATURES="${FEATURES}TSMC "
    fi
    
    # Check for SMC
    if grep -q "SMC" "$TEST_OUTPUT_DIR/optimized/${base_name}.a80"; then
        FEATURES="${FEATURES}SMC "
    fi
    
    # Check for shadow registers
    if grep -q "EXX" "$TEST_OUTPUT_DIR/optimized/${base_name}.a80"; then
        FEATURES="${FEATURES}Shadow "
    fi
    
    if [ -n "$FEATURES" ]; then
        echo -e "${GREEN}✓ Detected: $FEATURES${NC}"
    else
        echo -e "${YELLOW}⚠ No special optimizations detected${NC}"
    fi
    
    # Test 6: Assembly correctness (basic check)
    echo -n "  6. Assembly validation... "
    if grep -q "ERROR\|error" "$TEST_OUTPUT_DIR/optimized/${base_name}.a80"; then
        echo -e "${RED}✗ FAIL - Errors in assembly${NC}"
        TEST_RESULTS+=("$rel_path,FAIL,Assembly errors")
        return 1
    else
        echo -e "${GREEN}✓ PASS${NC}"
        ((PASSED_EXECUTION++))
    fi
    
    # Calculate overall improvement estimate
    if [ $NORMAL_MIR_INSTRUCTIONS -gt 0 ] && [ $OPT_MIR_INSTRUCTIONS -gt 0 ]; then
        # Rough cycle estimate: assume average 8 cycles per instruction
        NORMAL_CYCLES_EST=$((NORMAL_MIR_INSTRUCTIONS * 8))
        OPT_CYCLES_EST=$((OPT_MIR_INSTRUCTIONS * 8))
        CYCLE_IMPROVEMENT=$(python3 -c "print(f'{(1 - $OPT_CYCLES_EST/$NORMAL_CYCLES_EST)*100:.1f}')")
        
        if (( $(echo "$CYCLE_IMPROVEMENT > 0" | bc -l) )); then
            ((PERFORMANCE_IMPROVEMENTS++))
            echo -e "  ${GREEN}➤ Estimated performance improvement: ${CYCLE_IMPROVEMENT}%${NC}"
        fi
        
        PERFORMANCE_DATA+=("$rel_path,$NORMAL_SIZE,$OPT_SIZE,$SIZE_REDUCTION,$NORMAL_MIR_INSTRUCTIONS,$OPT_MIR_INSTRUCTIONS,$INST_REDUCTION,$CYCLE_IMPROVEMENT")
    fi
    
    TEST_RESULTS+=("$rel_path,PASS,All tests passed")
    echo -e "  ${GREEN}✅ Overall: PASS${NC}"
    
    return 0
}

# Main test execution
echo -e "\n${BLUE}Starting comprehensive test suite...${NC}"
echo -e "${BLUE}Test output directory: $TEST_OUTPUT_DIR${NC}\n"

# Find all .minz files
EXAMPLE_FILES=()
while IFS= read -r -d '' file; do
    # Skip output directories
    if [[ "$file" == */output/* ]] || [[ "$file" == */compiled/* ]]; then
        continue
    fi
    EXAMPLE_FILES+=("$file")
done < <(find "$EXAMPLES_DIR" -name "*.minz" -print0)

echo -e "${BLUE}Found ${#EXAMPLE_FILES[@]} example files to test${NC}"

# Test each example
for file in "${EXAMPLE_FILES[@]}"; do
    test_example "$file" || true
done

# Generate summary report
echo -e "\n${BLUE}╔══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║                    TEST SUMMARY REPORT                       ║${NC}"
echo -e "${BLUE}╚══════════════════════════════════════════════════════════════╝${NC}"
echo
echo -e "Total examples tested:        $TOTAL_TESTS"
echo -e "Passed normal compilation:    ${GREEN}$PASSED_COMPILATION${NC} ($(python3 -c "print(f'{$PASSED_COMPILATION/$TOTAL_TESTS*100:.1f}%')"))"
echo -e "Passed optimized compilation: ${GREEN}$PASSED_OPTIMIZATION${NC} ($(python3 -c "print(f'{$PASSED_OPTIMIZATION/$TOTAL_TESTS*100:.1f}%')"))"
echo -e "Passed all tests:            ${GREEN}$PASSED_EXECUTION${NC} ($(python3 -c "print(f'{$PASSED_EXECUTION/$TOTAL_TESTS*100:.1f}%')"))"
echo -e "Examples with improvements:   ${GREEN}$PERFORMANCE_IMPROVEMENTS${NC} ($(python3 -c "print(f'{$PERFORMANCE_IMPROVEMENTS/$TOTAL_TESTS*100:.1f}%')"))"

# Generate detailed reports
echo -e "\n${BLUE}Generating detailed reports...${NC}"

# 1. CSV Performance Report
PERF_REPORT="$TEST_OUTPUT_DIR/performance_report.csv"
echo "File,Normal Size,Opt Size,Size Reduction %,Normal Instructions,Opt Instructions,Instruction Reduction %,Est. Cycle Improvement %" > "$PERF_REPORT"
for data in "${PERFORMANCE_DATA[@]}"; do
    echo "$data" >> "$PERF_REPORT"
done
echo -e "  ${GREEN}✓${NC} Performance report: $PERF_REPORT"

# 2. Test Results Report
RESULTS_REPORT="$TEST_OUTPUT_DIR/test_results.csv"
echo "File,Status,Details" > "$RESULTS_REPORT"
for result in "${TEST_RESULTS[@]}"; do
    echo "$result" >> "$RESULTS_REPORT"
done
echo -e "  ${GREEN}✓${NC} Test results: $RESULTS_REPORT"

# 3. Markdown Summary
SUMMARY_REPORT="$TEST_OUTPUT_DIR/summary.md"
cat > "$SUMMARY_REPORT" << EOF
# MinZ Comprehensive Test Report

**Date:** $(date)
**Total Tests:** $TOTAL_TESTS

## Summary

- **Compilation Success Rate:** $PASSED_COMPILATION/$TOTAL_TESTS ($(python3 -c "print(f'{$PASSED_COMPILATION/$TOTAL_TESTS*100:.1f}%')"))
- **Optimization Success Rate:** $PASSED_OPTIMIZATION/$TOTAL_TESTS ($(python3 -c "print(f'{$PASSED_OPTIMIZATION/$TOTAL_TESTS*100:.1f}%')"))
- **Overall Pass Rate:** $PASSED_EXECUTION/$TOTAL_TESTS ($(python3 -c "print(f'{$PASSED_EXECUTION/$TOTAL_TESTS*100:.1f}%')"))
- **Performance Improvements:** $PERFORMANCE_IMPROVEMENTS/$TOTAL_TESTS ($(python3 -c "print(f'{$PERFORMANCE_IMPROVEMENTS/$TOTAL_TESTS*100:.1f}%')"))

## Top Performers

\`\`\`
$(sort -t',' -k8 -nr "$PERF_REPORT" | head -6 | tail -5)
\`\`\`

## Failed Tests

\`\`\`
$(grep ",FAIL," "$RESULTS_REPORT" || echo "No failures!")
\`\`\`
EOF

echo -e "  ${GREEN}✓${NC} Summary report: $SUMMARY_REPORT"

# Display final status
echo
if [ $PASSED_EXECUTION -eq $TOTAL_TESTS ]; then
    echo -e "${GREEN}🎉 ALL TESTS PASSED! 🎉${NC}"
    exit 0
else
    FAILED=$((TOTAL_TESTS - PASSED_EXECUTION))
    echo -e "${RED}❌ $FAILED tests failed${NC}"
    echo -e "${YELLOW}See $TEST_OUTPUT_DIR for details${NC}"
    exit 1
fi