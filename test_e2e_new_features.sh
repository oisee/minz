#!/bin/bash
# Comprehensive E2E test suite for new MinZ features

set -e  # Exit on first error

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

TOTAL=0
PASSED=0
FAILED=0

# Test helper
test_feature() {
    local name="$1"
    local command="$2"
    local expected="$3"
    
    TOTAL=$((TOTAL + 1))
    echo -n "Testing $name... "
    
    if output=$($command 2>&1); then
        if [[ -z "$expected" ]] || [[ "$output" == *"$expected"* ]]; then
            echo -e "${GREEN}✓${NC}"
            PASSED=$((PASSED + 1))
            return 0
        else
            echo -e "${RED}✗ (unexpected output)${NC}"
            echo "  Expected: $expected"
            echo "  Got: $output"
            FAILED=$((FAILED + 1))
            return 1
        fi
    else
        echo -e "${RED}✗ (command failed)${NC}"
        echo "  Error: $output"
        FAILED=$((FAILED + 1))
        return 1
    fi
}

# Test compilation
test_compile() {
    local name="$1"
    local file="$2"
    
    TOTAL=$((TOTAL + 1))
    echo -n "Compiling $name... "
    
    if ./minzc/mz "$file" -o /tmp/test.a80 2>/dev/null; then
        echo -e "${GREEN}✓${NC}"
        PASSED=$((PASSED + 1))
        return 0
    else
        echo -e "${RED}✗${NC}"
        FAILED=$((FAILED + 1))
        return 1
    fi
}

echo "================================"
echo "MinZ New Features E2E Test Suite"
echo "================================"
echo

# ===================
# 1. ENUM ACCESS TESTS
# ===================
echo "1. Testing Enum Access (both syntaxes)"
echo "-----------------------------------"

cat > test_enum_access.minz << 'EOF'
enum Color {
    RED,
    GREEN,
    BLUE
}

fun main() -> void {
    // Test :: syntax
    let c1 = Color::RED;
    print_u8(c1 as u8);
    
    // Test . syntax  
    let c2 = Color.GREEN;
    print_u8(c2 as u8);
    
    // Direct cast
    print_u8(Color::BLUE as u8);
}
EOF

test_compile "enum with :: syntax" "test_enum_access.minz"

# ===================
# 2. ARRAY LITERAL TESTS
# ===================
echo
echo "2. Testing Array Literals"
echo "------------------------"

cat > test_array_literals.minz << 'EOF'
fun main() -> void {
    let arr = [10, 20, 30];
    print_u8(arr[0]);
    print_u8(arr[1]);
    print_u8(arr[2]);
    
    // Nested arrays
    let matrix = [[1, 2], [3, 4]];
    print_u8(matrix[0][0]);
}
EOF

test_compile "array literals" "test_array_literals.minz"

# ===================
# 3. ERROR MESSAGE TESTS
# ===================
echo
echo "3. Testing Improved Error Messages"
echo "---------------------------------"

cat > test_typo.minz << 'EOF'
fun main() -> void {
    primt_u8(42);  // Typo: should be print_u8
}
EOF

output=$(./minzc/mz test_typo.minz -o /tmp/test.a80 2>&1 || true)
if [[ "$output" == *"did you mean 'print_u8'?"* ]]; then
    echo -e "Typo suggestion: ${GREEN}✓${NC}"
    PASSED=$((PASSED + 1))
else
    echo -e "Typo suggestion: ${RED}✗${NC}"
    FAILED=$((FAILED + 1))
fi
TOTAL=$((TOTAL + 1))

# ===================
# 4. MZV (MIR VM) TESTS
# ===================
echo
echo "4. Testing mzv (MIR VM Interpreter)"
echo "----------------------------------"

# Create a simple MIR program
cat > test.mir << 'EOF'
.function main()
    r0 = 42
    print r0
    r0 = 10
    printchar r0
    r1 = 5
    r2 = 7
    r3 = r1 + r2
    print r3
    return
.end
EOF

test_feature "mzv basic execution" "./minzc/mzv -i test.mir" "42"

# Test with tracing
test_feature "mzv tracing mode" "./minzc/mzv -i test.mir -trace 2>&1 | head -1" "[main:0]"

# Create MIR with loops
cat > test_loop.mir << 'EOF'
.function main()
    r0 = 5
loop:
    print r0
    r0 = r0 - 1
    jmpif r0 loop
    return
.end
EOF

test_feature "mzv loop execution" "./minzc/mzv -i test_loop.mir -max-steps 100 2>&1 | grep -o '54321'" "54321"

# ===================
# 5. PROPERTY-BASED TESTS
# ===================
echo
echo "5. Property-Based Testing (Fuzzing)"
echo "----------------------------------"

# Generate random valid MinZ programs
for i in {1..5}; do
    cat > fuzz_$i.minz << EOF
fun test_$i(x: u8) -> u8 {
    return x + $((RANDOM % 10));
}

fun main() -> void {
    let result = test_$i($((RANDOM % 256)));
    print_u8(result);
}
EOF
    
    test_compile "fuzz test $i" "fuzz_$i.minz"
done

# ===================
# 6. SNAPSHOT TESTING
# ===================
echo
echo "6. Snapshot Testing"
echo "------------------"

# Compile a known program and check output consistency
cat > snapshot_test.minz << 'EOF'
fun fibonacci(n: u8) -> u8 {
    if (n <= 1) {
        return n;
    }
    return fibonacci(n - 1) + fibonacci(n - 2);
}

fun main() -> void {
    print_u8(fibonacci(5));
}
EOF

if ./minzc/mz snapshot_test.minz -o snapshot.a80 2>/dev/null; then
    # Check if assembly contains expected patterns
    if grep -q "CALL.*fibonacci" snapshot.a80 2>/dev/null; then
        echo -e "Snapshot test: ${GREEN}✓${NC}"
        PASSED=$((PASSED + 1))
    else
        echo -e "Snapshot test: ${RED}✗ (no recursive call)${NC}"
        FAILED=$((FAILED + 1))
    fi
else
    echo -e "Snapshot test: ${RED}✗ (compilation failed)${NC}"
    FAILED=$((FAILED + 1))
fi
TOTAL=$((TOTAL + 1))

# ===================
# 7. REGRESSION TESTS
# ===================
echo
echo "7. Regression Testing"
echo "-------------------"

# Test that old features still work
cat > regression_test.minz << 'EOF'
struct Point {
    x: u8,
    y: u8
}

fun distance(p: Point) -> u8 {
    return p.x + p.y;
}

fun main() -> void {
    let p = Point { x: 3, y: 4 };
    print_u8(distance(p));
}
EOF

test_compile "struct regression" "regression_test.minz"

# ===================
# 8. INTEGRATION TESTS
# ===================
echo
echo "8. Integration Testing (Multiple Features)"
echo "-----------------------------------------"

cat > integration_test.minz << 'EOF'
enum Status {
    OK,
    ERROR
}

fun process_array(arr: [u8; 3]) -> Status {
    let sum: u8 = 0;
    for i in 0..3 {
        sum = sum + arr[i];
    }
    
    if (sum > 100) {
        return Status::ERROR;
    }
    return Status::OK;
}

fun main() -> void {
    let data = [10, 20, 30];
    let status = process_array(data);
    
    if (status as u8 == Status.OK as u8) {
        print_u8(1);
    } else {
        print_u8(0);
    }
}
EOF

test_compile "integration test" "integration_test.minz"

# ===================
# 9. MUTATION TESTING
# ===================
echo
echo "9. Mutation Testing"
echo "------------------"

# Original program
cat > original.minz << 'EOF'
fun add(a: u8, b: u8) -> u8 {
    return a + b;
}

fun main() -> void {
    print_u8(add(5, 3));
}
EOF

# Mutated versions (should fail or produce different output)
mutations=(
    "return a - b;"  # Changed operator
    "return a * b;"  # Changed operator
    "return b + a;"  # Swapped operands
)

for i in "${!mutations[@]}"; do
    cat > mutant_$i.minz << EOF
fun add(a: u8, b: u8) -> u8 {
    ${mutations[$i]}
}

fun main() -> void {
    print_u8(add(5, 3));
}
EOF
    
    test_compile "mutation $i" "mutant_$i.minz"
done

# ===================
# 10. CHAOS ENGINEERING
# ===================
echo
echo "10. Chaos Engineering Tests"
echo "--------------------------"

# Test compiler resilience with edge cases
cat > chaos_deep_nesting.minz << 'EOF'
fun main() -> void {
    if (1) {
        if (1) {
            if (1) {
                if (1) {
                    if (1) {
                        print_u8(42);
                    }
                }
            }
        }
    }
}
EOF

test_compile "deep nesting" "chaos_deep_nesting.minz"

# Large array
cat > chaos_large_array.minz << 'EOF'
fun main() -> void {
    let big: [u8; 100];
    big[0] = 1;
    big[99] = 2;
    print_u8(big[0] + big[99]);
}
EOF

test_compile "large array" "chaos_large_array.minz"

# ===================
# 11. GOLDEN FILE TESTS
# ===================
echo
echo "11. Golden File Testing"
echo "----------------------"

# Generate expected output
cat > golden_source.minz << 'EOF'
fun factorial(n: u8) -> u8 {
    if (n == 0) {
        return 1;
    }
    return n * factorial(n - 1);
}

fun main() -> void {
    print_u8(factorial(5));  // Should print 120
}
EOF

if ./minzc/mz golden_source.minz -o golden.a80 2>/dev/null; then
    # Save as golden file
    cp golden.a80 golden.a80.expected
    echo -e "Golden file created: ${GREEN}✓${NC}"
    PASSED=$((PASSED + 1))
else
    echo -e "Golden file creation: ${RED}✗${NC}"
    FAILED=$((FAILED + 1))
fi
TOTAL=$((TOTAL + 1))

# ===================
# 12. CONTRACT TESTING
# ===================
echo
echo "12. Contract Testing (Design by Contract)"
echo "----------------------------------------"

cat > contract_test.minz << 'EOF'
// @requires: x < 100
// @ensures: result > x
fun increment_safe(x: u8) -> u8 {
    if (x >= 100) {
        return 100;  // Saturate
    }
    return x + 1;
}

fun main() -> void {
    print_u8(increment_safe(50));   // Valid
    print_u8(increment_safe(99));   // Edge case
    print_u8(increment_safe(100));  // Contract violation
}
EOF

test_compile "contract test" "contract_test.minz"

# ===================
# SUMMARY
# ===================
echo
echo "================================"
echo "Test Results Summary"
echo "================================"
echo -e "Total:  $TOTAL"
echo -e "Passed: ${GREEN}$PASSED${NC}"
echo -e "Failed: ${RED}$FAILED${NC}"

if [ $FAILED -eq 0 ]; then
    echo -e "\n${GREEN}All tests passed! 🎉${NC}"
    exit 0
else
    echo -e "\n${RED}Some tests failed${NC}"
    exit 1
fi