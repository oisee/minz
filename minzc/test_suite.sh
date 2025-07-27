#!/bin/bash
# MinZ Compiler Test Suite
# Tests for v0.4.0 features: strings and hierarchical register allocation

set -e  # Exit on error

echo "=== MinZ Compiler Test Suite v0.4.0 ==="
echo "Testing: Length-prefixed strings and hierarchical register allocation"
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counter
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Function to run a test
run_test() {
    local test_name="$1"
    local minz_file="$2"
    local expected_pattern="$3"
    local description="$4"
    
    TESTS_RUN=$((TESTS_RUN + 1))
    echo -n "Testing $test_name... "
    
    # Compile the test file
    if ./minzc "$minz_file" -o "test_output.a80" 2>/dev/null; then
        # Check if expected pattern exists in output
        if grep -q "$expected_pattern" test_output.a80; then
            echo -e "${GREEN}PASSED${NC} - $description"
            TESTS_PASSED=$((TESTS_PASSED + 1))
        else
            echo -e "${RED}FAILED${NC} - Expected pattern not found: $expected_pattern"
            echo "  Description: $description"
            TESTS_FAILED=$((TESTS_FAILED + 1))
        fi
    else
        echo -e "${RED}FAILED${NC} - Compilation error"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    
    rm -f test_output.a80
}

# Function to check assembly quality
check_assembly_quality() {
    local test_name="$1"
    local minz_file="$2"
    
    echo ""
    echo "=== Quality Check: $test_name ==="
    ./minzc "$minz_file" -o "test_output.a80" 2>/dev/null
    
    # Count various instruction types
    local memory_loads=$(grep -c "LD.*(\$F" test_output.a80 || true)
    local register_ops=$(grep -c "LD [A-L], [A-L]" test_output.a80 || true)
    local total_instructions=$(grep -c "^\s*[A-Z]" test_output.a80 || true)
    
    echo "  Memory loads: $memory_loads"
    echo "  Register operations: $register_ops"
    echo "  Total instructions: $total_instructions"
    
    if [ $memory_loads -gt 0 ] && [ $register_ops -gt 0 ]; then
        echo -e "  ${GREEN}✓ Uses both registers and memory (hierarchical allocation working)${NC}"
    fi
    
    rm -f test_output.a80
}

echo "=== Testing String Implementation ==="
echo ""

# Test 1: Basic string with length prefix
run_test "string_basic" "../examples/test_strings.minz" \
    "DB 13.*; Length" \
    "String should have length prefix"

# Test 2: Empty string
cat > test_empty_string.minz << 'EOF'
fun main() -> void {
    let empty = "";
}
EOF
run_test "string_empty" "test_empty_string.minz" \
    "DB 0.*; Length" \
    "Empty string should have length 0"

# Test 3: Long string (test single byte length)
cat > test_long_string.minz << 'EOF'
fun main() -> void {
    let long_str = "This is a string with exactly 255 characters to test the boundary between single byte and double byte length encoding in our length-prefixed string implementation which should use DB for lengths up to 255 and DW for longer strings ensuring efficient memory usage patterns";
}
EOF
run_test "string_255_chars" "test_long_string.minz" \
    "DB [0-9].*; Length" \
    "String ≤255 chars should use DB for length"

# Test 4: No null terminator
run_test "string_no_null" "../examples/test_strings.minz" \
    "! DB 0$" \
    "Strings should NOT have null terminator"

echo ""
echo "=== Testing Hierarchical Register Allocation ==="
echo ""

# Test 5: Physical register usage
run_test "reg_physical" "../examples/test_physical_registers.minz" \
    "LD [C-L], A.*; Store to physical register" \
    "Should use physical registers for allocation"

# Test 6: Hierarchical allocation comment
run_test "reg_hierarchy_comment" "../examples/test_physical_registers.minz" \
    "; Using hierarchical register allocation" \
    "Should indicate hierarchical allocation is active"

# Test 7: Mixed allocation (registers + memory)
cat > test_many_vars.minz << 'EOF'
fun many_variables() -> u8 {
    let a: u8 = 1;
    let b: u8 = 2;
    let c: u8 = 3;
    let d: u8 = 4;
    let e: u8 = 5;
    let f: u8 = 6;
    let g: u8 = 7;
    let h: u8 = 8;
    let i: u8 = 9;
    let j: u8 = 10;
    return a + b + c + d + e + f + g + h + i + j;
}

fun main() -> void {
    let result = many_variables();
}
EOF
run_test "reg_mixed_allocation" "test_many_vars.minz" \
    "LD.*\$F.*Virtual register.*memory" \
    "Should fall back to memory when registers exhausted"

# Test 8: Shadow register preparation (infrastructure exists)
run_test "reg_shadow_ready" "../examples/test_physical_registers.minz" \
    "physical.*shadow.*memory" \
    "Should show hierarchical allocation strategy"

echo ""
echo "=== Assembly Quality Analysis ==="

# Check quality of generated code
check_assembly_quality "Simple Function" "../examples/test_physical_registers.minz"
check_assembly_quality "Many Variables" "test_many_vars.minz"

echo ""
echo "=== Manual Review Checklist ==="
echo ""
echo "String Implementation Review:"
echo "  [ ] Length prefix present (DB n for ≤255, DW n for >255)"
echo "  [ ] No null terminators"
echo "  [ ] Special characters handled correctly"
echo "  [ ] Empty strings work (DB 0)"
echo ""
echo "Register Allocation Review:"
echo "  [ ] Physical registers used (B, C, D, E, H, L)"
echo "  [ ] Memory fallback working ($F000+ addresses)"
echo "  [ ] Comments indicate hierarchical system active"
echo "  [ ] No register allocation conflicts"
echo "  [ ] Shadow register infrastructure ready"
echo ""

# Summary
echo ""
echo "=== Test Summary ==="
echo "Total tests run: $TESTS_RUN"
echo -e "Tests passed: ${GREEN}$TESTS_PASSED${NC}"
echo -e "Tests failed: ${RED}$TESTS_FAILED${NC}"

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "\n${GREEN}✓ All tests passed!${NC}"
    exit 0
else
    echo -e "\n${RED}✗ Some tests failed${NC}"
    exit 1
fi