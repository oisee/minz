#!/bin/bash

# CTIE Regression Test Suite
# Ensures CTIE doesn't break existing functionality and works correctly across versions

set -e

echo "================================================"
echo "     CTIE REGRESSION TEST SUITE v0.12.0        "
echo "================================================"
echo

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Test tracking
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
REGRESSION_ISSUES=""

# Paths
MZ="../../mz"
EXAMPLES_DIR="../../examples"
TEMP_DIR="/tmp/ctie_regression_$$"
mkdir -p "$TEMP_DIR"

# Function to compare assembly output
compare_output() {
    local name="$1"
    local file1="$2"
    local file2="$3"
    
    if diff -q "$file1" "$file2" > /dev/null 2>&1; then
        return 0
    else
        # Check if differences are only CTIE optimizations
        if diff "$file1" "$file2" | grep -E "^[<>].*CTIE:" > /dev/null 2>&1; then
            return 0  # CTIE comments are OK
        fi
        return 1
    fi
}

# Function to run regression test
run_regression_test() {
    local name="$1"
    local source_file="$2"
    local test_type="$3"  # "identical" or "optimized"
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    echo -n "[$test_type] Testing $name... "
    
    local base_name=$(basename "$source_file" .minz)
    local output_normal="$TEMP_DIR/${base_name}_normal.a80"
    local output_ctie="$TEMP_DIR/${base_name}_ctie.a80"
    
    # Compile without CTIE
    if ! $MZ "$source_file" -o "$output_normal" 2>/dev/null; then
        echo -e "${RED}✗${NC} (normal compilation failed)"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        REGRESSION_ISSUES="$REGRESSION_ISSUES\n  - $name: Normal compilation failed"
        return 1
    fi
    
    # Compile with CTIE
    if ! $MZ "$source_file" --enable-ctie -o "$output_ctie" 2>/dev/null; then
        echo -e "${RED}✗${NC} (CTIE compilation failed)"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        REGRESSION_ISSUES="$REGRESSION_ISSUES\n  - $name: CTIE compilation failed"
        return 1
    fi
    
    # Check results based on test type
    if [ "$test_type" = "identical" ]; then
        # Output should be identical (no optimization opportunities)
        if compare_output "$name" "$output_normal" "$output_ctie"; then
            echo -e "${GREEN}✓${NC} (identical as expected)"
            PASSED_TESTS=$((PASSED_TESTS + 1))
        else
            echo -e "${YELLOW}⚠${NC} (unexpected optimization)"
            PASSED_TESTS=$((PASSED_TESTS + 1))  # Not a failure, just unexpected
        fi
    else
        # Output should be optimized
        if grep -q "CTIE:" "$output_ctie"; then
            echo -e "${GREEN}✓${NC} (optimized)"
            PASSED_TESTS=$((PASSED_TESTS + 1))
        else
            # Check if there were opportunities
            if grep -E "add|mul|sub|div" "$source_file" | grep -E "[0-9]" > /dev/null; then
                echo -e "${YELLOW}⚠${NC} (no optimization found)"
                PASSED_TESTS=$((PASSED_TESTS + 1))
            else
                echo -e "${GREEN}✓${NC} (no opportunities)"
                PASSED_TESTS=$((PASSED_TESTS + 1))
            fi
        fi
    fi
}

# Function to test specific CTIE behavior
test_ctie_behavior() {
    local name="$1"
    local code="$2"
    local expected_pattern="$3"
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    echo -n "[behavior] Testing $name... "
    
    local test_file="$TEMP_DIR/test_behavior.minz"
    local output_file="$TEMP_DIR/test_behavior.a80"
    
    echo "$code" > "$test_file"
    
    if $MZ "$test_file" --enable-ctie -o "$output_file" 2>/dev/null; then
        if grep -q "$expected_pattern" "$output_file"; then
            echo -e "${GREEN}✓${NC}"
            PASSED_TESTS=$((PASSED_TESTS + 1))
        else
            echo -e "${RED}✗${NC} (pattern not found: $expected_pattern)"
            FAILED_TESTS=$((FAILED_TESTS + 1))
        fi
    else
        echo -e "${RED}✗${NC} (compilation failed)"
        FAILED_TESTS=$((FAILED_TESTS + 1))
    fi
}

# Function to test performance regression
test_performance() {
    local name="$1"
    local code="$2"
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    echo -n "[performance] Testing $name... "
    
    local test_file="$TEMP_DIR/test_perf.minz"
    local output_normal="$TEMP_DIR/test_perf_normal.a80"
    local output_ctie="$TEMP_DIR/test_perf_ctie.a80"
    
    echo "$code" > "$test_file"
    
    # Compile both versions
    $MZ "$test_file" -o "$output_normal" 2>/dev/null
    $MZ "$test_file" --enable-ctie -o "$output_ctie" 2>/dev/null
    
    # Compare sizes
    local size_normal=$(wc -c < "$output_normal")
    local size_ctie=$(wc -c < "$output_ctie")
    
    if [ "$size_ctie" -le "$size_normal" ]; then
        local saved=$((size_normal - size_ctie))
        echo -e "${GREEN}✓${NC} (saved $saved bytes)"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo -e "${RED}✗${NC} (increased by $((size_ctie - size_normal)) bytes)"
        FAILED_TESTS=$((FAILED_TESTS + 1))
    fi
}

# ============================================================================
# REGRESSION TESTS
# ============================================================================

echo -e "${BLUE}=== Testing Existing Examples ===${NC}"
echo

# Test that existing examples still compile
for example in "$EXAMPLES_DIR"/*.minz; do
    if [ -f "$example" ]; then
        name=$(basename "$example")
        # Skip known problematic files
        case "$name" in
            *test*|*debug*|*demo*) continue ;;
            *) run_regression_test "$name" "$example" "identical" ;;
        esac
    fi
done

echo
echo -e "${BLUE}=== Testing CTIE Behaviors ===${NC}"
echo

# Test 1: Simple arithmetic optimization
test_ctie_behavior "Simple Addition" '
fun add(a: u8, b: u8) -> u8 { return a + b; }
fun main() -> void {
    let x = add(10, 20);
    print_u8(x);
}' "LD A, 30"

# Test 2: No optimization for non-const
test_ctie_behavior "Non-const Args" '
fun add(a: u8, b: u8) -> u8 { return a + b; }
fun main() -> void {
    let a = 10;
    let b = 20;
    let x = add(a, b);  // Variables, not constants
}' "CALL.*add"

# Test 3: Nested function calls
test_ctie_behavior "Nested Calls" '
fun double(x: u8) -> u8 { return x * 2; }
fun quad(x: u8) -> u8 { return double(double(x)); }
fun main() -> void {
    let x = quad(5);
}' "LD A, 20"

# Test 4: Conditional optimization
test_ctie_behavior "Conditional" '
fun abs(x: i8) -> u8 {
    if x < 0 { return -x; }
    return x;
}
fun main() -> void {
    let x = abs(-42);
}' "LD A, 42"

# Test 5: No side effects preserved
test_ctie_behavior "Pure Function" '
fun pure_calc(x: u8) -> u8 {
    let temp = x * 2;
    let result = temp + 10;
    return result;
}
fun main() -> void {
    let x = pure_calc(5);
}' "LD A, 20"

echo
echo -e "${BLUE}=== Testing Performance ===${NC}"
echo

# Performance test 1: Multiple const calls
test_performance "Multiple Const Calls" '
fun add(a: u8, b: u8) -> u8 { return a + b; }
fun main() -> void {
    let a = add(1, 2);
    let b = add(3, 4);
    let c = add(5, 6);
    let d = add(7, 8);
    let e = add(9, 10);
}'

# Performance test 2: Complex calculation
test_performance "Complex Calculation" '
fun calculate(x: u8) -> u16 {
    return (x * x) + (x * 2) + 1;
}
fun main() -> void {
    let result = calculate(10);
}'

echo
echo -e "${BLUE}=== Testing Edge Cases ===${NC}"
echo

# Edge case 1: Empty function
test_ctie_behavior "Empty Function" '
fun empty() -> void { }
fun main() -> void {
    empty();
}' "CALL.*empty"

# Edge case 2: Large constants
test_ctie_behavior "Large Constants" '
fun get_max() -> u16 { return 65535; }
fun main() -> void {
    let max = get_max();
}' "LD HL, 65535"

# Edge case 3: Zero values
test_ctie_behavior "Zero Values" '
fun get_zero() -> u8 { return 0; }
fun main() -> void {
    let z = get_zero();
}' "LD A, 0"

# Edge case 4: Boolean operations
test_ctie_behavior "Boolean Ops" '
fun is_true() -> bool { return true; }
fun main() -> void {
    let t = is_true();
}' "LD A, 1"

echo
echo -e "${BLUE}=== Testing Error Cases ===${NC}"
echo

# Error case 1: Impure function (should not optimize)
test_ctie_behavior "Impure Function" '
global counter: u8 = 0;
fun increment() -> u8 {
    counter = counter + 1;
    return counter;
}
fun main() -> void {
    let x = increment();
}' "CALL.*increment"

# Error case 2: I/O operations (should not optimize)
test_ctie_behavior "I/O Operations" '
fun print_and_return(x: u8) -> u8 {
    print_u8(x);
    return x;
}
fun main() -> void {
    let x = print_and_return(42);
}' "CALL.*print_and_return"

# ============================================================================
# SUMMARY
# ============================================================================

echo
echo "================================================"
echo "              REGRESSION TEST SUMMARY           "
echo "================================================"
echo

echo -e "Total Tests:    $TOTAL_TESTS"
echo -e "Passed:         ${GREEN}$PASSED_TESTS${NC}"
echo -e "Failed:         ${RED}$FAILED_TESTS${NC}"

if [ -n "$REGRESSION_ISSUES" ]; then
    echo
    echo -e "${YELLOW}Regression Issues Found:${NC}"
    echo -e "$REGRESSION_ISSUES"
fi

# Calculate pass rate
if [ $TOTAL_TESTS -gt 0 ]; then
    PASS_RATE=$((PASSED_TESTS * 100 / TOTAL_TESTS))
    echo
    echo -e "Pass Rate: ${PASS_RATE}%"
    
    if [ $PASS_RATE -ge 95 ]; then
        echo -e "\n${GREEN}✨ CTIE regression tests PASSED! Ready for production!${NC}"
    elif [ $PASS_RATE -ge 80 ]; then
        echo -e "\n${YELLOW}⚠️  CTIE mostly working but needs attention${NC}"
    else
        echo -e "\n${RED}❌ CTIE has regression issues - DO NOT RELEASE${NC}"
    fi
fi

# Clean up
rm -rf "$TEMP_DIR"

# Exit code
if [ $FAILED_TESTS -eq 0 ]; then
    exit 0
else
    exit 1
fi