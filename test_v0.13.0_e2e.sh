#!/bin/bash

# MinZ v0.13.0 End-to-End Release Test
# Tests the complete release package functionality

echo "═══════════════════════════════════════════════════════"
echo "    MinZ v0.13.0 'Module Revolution' E2E Test          "
echo "═══════════════════════════════════════════════════════"
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Test counter
tests_passed=0
tests_failed=0

# Test function
run_test() {
    local test_name=$1
    local command=$2
    
    echo -n "Testing $test_name... "
    if eval "$command" > /dev/null 2>&1; then
        echo -e "${GREEN}✅ PASSED${NC}"
        tests_passed=$((tests_passed + 1))
    else
        echo -e "${RED}❌ FAILED${NC}"
        tests_failed=$((tests_failed + 1))
    fi
}

# Create temp directory for testing
TEMP_DIR=$(mktemp -d)
cd $TEMP_DIR
echo "Test directory: $TEMP_DIR"
echo ""

# Download and extract release
echo "📦 Downloading release package..."
curl -sL https://github.com/oisee/minz/releases/download/v0.13.0/minz-v0.13.0-darwin-arm64.tar.gz | tar xz
cd darwin-arm64

echo ""
echo "🧪 Running tests..."
echo ""

# Test 1: Binary exists and runs
run_test "Binary execution" "./bin/mz --version"

# Test 2: Compile basic example
run_test "Basic compilation" "./bin/mz examples/fibonacci.minz -o test.a80"

# Test 3: Module system - std
cat > test_std.minz << 'EOF'
import std;

fun main() -> void {
    std.print("Hello from v0.13.0!");
}
EOF
run_test "Standard library import" "./bin/mz test_std.minz -o test.a80"

# Test 4: Module aliasing
cat > test_alias.minz << 'EOF'
import std as io;

fun main() -> void {
    io.print("Aliasing works!");
}
EOF
run_test "Module aliasing" "./bin/mz test_alias.minz -o test.a80"

# Test 5: File-based module (math)
cat > test_math.minz << 'EOF'
import std;
import math;

fun main() -> void {
    let x: u8 = 5;
    let result = math.square(x);
    std.print(result);
}
EOF
run_test "File-based module" "./bin/mz test_math.minz -o test.a80"

# Test 6: Platform modules
cat > test_platform.minz << 'EOF'
import std;
import zx.screen;

fun main() -> void {
    zx.screen.set_border(2);
    std.print("Platform module works!");
}
EOF
run_test "Platform modules" "./bin/mz test_platform.minz -o test.a80"

# Test 7: Complex aliases
cat > test_complex.minz << 'EOF'
import std as io;
import math as m;
import zx.screen as gfx;

fun main() -> void {
    io.cls();
    let val = m.square(3);
    gfx.set_border(val);
}
EOF
run_test "Complex module usage" "./bin/mz test_complex.minz -o test.a80"

# Test 8: Lambda support
cat > test_lambda.minz << 'EOF'
fun main() -> void {
    let add = |x: u8, y: u8| => u8 { x + y };
    let result = add(5, 3);
}
EOF
run_test "Lambda expressions" "./bin/mz test_lambda.minz -o test.a80"

# Test 9: Interface methods
run_test "Interface example" "./bin/mz examples/interface_simple.minz -o test.a80"

# Test 10: Error propagation
cat > test_error.minz << 'EOF'
fun risky() -> u8? {
    return 42;
}

fun main() -> void {
    let val = risky() ?? 0;
}
EOF
run_test "Error propagation" "./bin/mz test_error.minz -o test.a80"

# Test 11: Enums
run_test "Enum support" "./bin/mz examples/enums.minz -o test.a80"

# Test 12: SMC optimization
run_test "SMC optimization" "./bin/mz examples/fibonacci.minz -O --enable-smc -o test.a80"

# Test 13: CTIE
run_test "CTIE compilation" "./bin/mz examples/fibonacci.minz --enable-ctie -o test.a80"

# Test 14: Examples compilation
echo ""
echo "📚 Testing all included examples..."
example_count=0
example_success=0
for example in examples/*.minz; do
    if [ -f "$example" ]; then
        example_count=$((example_count + 1))
        if ./bin/mz "$example" -o test.a80 > /dev/null 2>&1; then
            example_success=$((example_success + 1))
            echo -n "✓"
        else
            echo -n "✗"
        fi
    fi
done
echo ""
echo "Examples: $example_success/$example_count successful"

# Final summary
echo ""
echo "═══════════════════════════════════════════════════════"
echo "                    TEST RESULTS                        "
echo "═══════════════════════════════════════════════════════"
echo ""
echo -e "Core tests: ${GREEN}$tests_passed passed${NC}, ${RED}$tests_failed failed${NC}"
echo "Examples: $example_success/$example_count compiled successfully"
echo ""

total_tests=$((tests_passed + tests_failed))
success_rate=$((tests_passed * 100 / total_tests))

if [ $tests_failed -eq 0 ]; then
    echo -e "${GREEN}🎉 ALL TESTS PASSED! MinZ v0.13.0 is working perfectly!${NC}"
elif [ $success_rate -ge 80 ]; then
    echo -e "${YELLOW}⚠️ Most tests passed ($success_rate% success rate)${NC}"
else
    echo -e "${RED}❌ Tests need attention ($success_rate% success rate)${NC}"
fi

echo ""
echo "📦 Release URL: https://github.com/oisee/minz/releases/tag/v0.13.0"
echo ""

# Cleanup
cd /
rm -rf $TEMP_DIR
echo "Test environment cleaned up."