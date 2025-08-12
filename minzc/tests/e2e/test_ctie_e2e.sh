#!/bin/bash

# End-to-End Test for Compile-Time Interface Execution (CTIE)
# This test verifies that functions are executed at compile-time
# and calls are replaced with computed values

set -e

echo "=== CTIE End-to-End Test Suite ==="
echo "Testing compile-time function execution..."
echo

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counter
TOTAL=0
PASSED=0
FAILED=0

# Function to run a test
run_test() {
    local name="$1"
    local source="$2"
    local expected_pattern="$3"
    local should_not_contain="$4"
    
    TOTAL=$((TOTAL + 1))
    echo -n "Testing $name... "
    
    # Create temp file
    local temp_minz="/tmp/test_ctie_$$.minz"
    local temp_a80="/tmp/test_ctie_$$.a80"
    
    # Write source code
    echo "$source" > "$temp_minz"
    
    # Compile with CTIE
    if ../../mz "$temp_minz" --enable-ctie -o "$temp_a80" 2>/dev/null; then
        # Check if expected pattern exists
        if grep -q "$expected_pattern" "$temp_a80"; then
            # Check that unwanted pattern doesn't exist
            if [ -n "$should_not_contain" ]; then
                if grep -q "$should_not_contain" "$temp_a80"; then
                    echo -e "${RED}✗${NC} (still contains CALL)"
                    FAILED=$((FAILED + 1))
                    echo "  Expected to not find: $should_not_contain"
                    grep "$should_not_contain" "$temp_a80" | head -2
                else
                    echo -e "${GREEN}✓${NC}"
                    PASSED=$((PASSED + 1))
                fi
            else
                echo -e "${GREEN}✓${NC}"
                PASSED=$((PASSED + 1))
            fi
        else
            echo -e "${RED}✗${NC} (pattern not found)"
            FAILED=$((FAILED + 1))
            echo "  Expected: $expected_pattern"
        fi
    else
        echo -e "${RED}✗${NC} (compilation failed)"
        FAILED=$((FAILED + 1))
    fi
    
    # Clean up
    rm -f "$temp_minz" "$temp_a80"
}

# Test 1: Simple addition
run_test "Simple Addition" '
fun add_const(a: u8, b: u8) -> u8 {
    return a + b;
}
fun main() -> void {
    let result = add_const(5, 3);
    print_u8(result);
}' "LD A, 8" "CALL.*add_const"

# Test 2: Multiplication
run_test "Multiplication" '
fun multiply(x: u8, y: u8) -> u8 {
    return x * y;
}
fun main() -> void {
    let result = multiply(6, 7);
    print_u8(result);
}' "LD A, 42" "CALL.*multiply"

# Test 3: No-parameter function
run_test "No Parameters" '
fun get_magic_number() -> u8 {
    return 42;
}
fun main() -> void {
    let magic = get_magic_number();
    print_u8(magic);
}' "LD A, 42" "CALL.*get_magic"

# Test 4: Multiple calls
run_test "Multiple Calls" '
fun double(x: u8) -> u8 {
    return x * 2;
}
fun main() -> void {
    let a = double(5);
    let b = double(10);
    print_u8(a);
    print_u8(b);
}' "LD A, 10" "CALL.*double"

# Test 5: Conditional evaluation
run_test "Conditional" '
fun max_const(a: u8, b: u8) -> u8 {
    if a > b {
        return a;
    }
    return b;
}
fun main() -> void {
    let result = max_const(10, 20);
    print_u8(result);
}' "LD A, 20" "CALL.*max_const"

# Test 6: Nested calls
run_test "Nested Calls" '
fun add(a: u8, b: u8) -> u8 {
    return a + b;
}
fun add_three(a: u8, b: u8, c: u8) -> u8 {
    return add(add(a, b), c);
}
fun main() -> void {
    let result = add_three(1, 2, 3);
    print_u8(result);
}' "LD A, 6" "CALL.*add_three"

# Test 7: Complex expression
run_test "Complex Expression" '
fun calculate() -> u8 {
    return (10 + 20) * 2 - 15;
}
fun main() -> void {
    let result = calculate();
    print_u8(result);
}' "LD A, 45" "CALL.*calculate"

# Test 8: Chain of operations
run_test "Operation Chain" '
fun inc(x: u8) -> u8 { return x + 1; }
fun dec(x: u8) -> u8 { return x - 1; }
fun process(x: u8) -> u8 {
    return dec(inc(inc(x)));
}
fun main() -> void {
    let result = process(10);
    print_u8(result);
}' "LD A, 11" "CALL.*process"

# Test 9: Boolean operations
run_test "Boolean Ops" '
fun is_even(x: u8) -> bool {
    return x % 2 == 0;
}
fun main() -> void {
    let result = is_even(42);
    print_bool(result);
}' "LD A, 1" "CALL.*is_even"

# Test 10: Verify CTIE comment
echo -n "Testing CTIE comments... "
temp_minz="/tmp/test_ctie_comment.minz"
temp_a80="/tmp/test_ctie_comment.a80"
cat > "$temp_minz" << 'EOF'
fun add(a: u8, b: u8) -> u8 { return a + b; }
fun main() -> void {
    let x = add(2, 3);
}
EOF

if ../../mz "$temp_minz" --enable-ctie -o "$temp_a80" 2>/dev/null; then
    if grep -q "CTIE: Computed at compile-time" "$temp_a80"; then
        echo -e "${GREEN}✓${NC}"
        PASSED=$((PASSED + 1))
    else
        echo -e "${YELLOW}⚠${NC} (no CTIE comment found)"
    fi
else
    echo -e "${RED}✗${NC}"
    FAILED=$((FAILED + 1))
fi
TOTAL=$((TOTAL + 1))
rm -f "$temp_minz" "$temp_a80"

# Summary
echo
echo "=== Test Summary ==="
echo -e "Total:  $TOTAL"
echo -e "Passed: ${GREEN}$PASSED${NC}"
echo -e "Failed: ${RED}$FAILED${NC}"

if [ $FAILED -eq 0 ]; then
    echo -e "\n${GREEN}✨ All CTIE tests passed! Functions are disappearing!${NC}"
    exit 0
else
    echo -e "\n${RED}Some tests failed. CTIE needs investigation.${NC}"
    exit 1
fi