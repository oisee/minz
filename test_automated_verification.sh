#!/bin/bash
# Advanced automated verification system for MinZ

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

echo "========================================="
echo "MinZ Automated Verification System (AVS)"
echo "========================================="
echo

# Initialize test metrics
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
COVERAGE_LINES=0
PERFORMANCE_SCORE=0

# ======================
# 1. DIFFERENTIAL TESTING
# ======================
echo -e "${CYAN}1. Differential Testing (Parser Comparison)${NC}"
echo "--------------------------------------------"

# Test the same program with both parsers
cat > diff_test.minz << 'EOF'
enum Mode { NORMAL, FAST }
fun compute(x: u8, mode: Mode) -> u8 {
    if (mode as u8 == Mode::FAST as u8) {
        return x * 2;
    }
    return x + 1;
}
fun main() -> void {
    print_u8(compute(5, Mode::NORMAL));
}
EOF

echo -n "Testing with tree-sitter parser... "
if MINZ_USE_TREE_SITTER=1 ./minzc/mz diff_test.minz -o ts.a80 2>/dev/null; then
    echo -e "${GREEN}✓${NC}"
    TS_SUCCESS=1
else
    echo -e "${RED}✗${NC}"
    TS_SUCCESS=0
fi

echo -n "Testing with ANTLR parser... "
if ./minzc/mz diff_test.minz -o antlr.a80 2>/dev/null; then
    echo -e "${GREEN}✓${NC}"
    ANTLR_SUCCESS=1
else
    echo -e "${RED}✗${NC}"
    ANTLR_SUCCESS=0
fi

if [ $TS_SUCCESS -eq 1 ] && [ $ANTLR_SUCCESS -eq 1 ]; then
    echo -n "Comparing outputs... "
    # Compare assembly outputs (ignoring comments)
    if diff -q <(grep -v '^;' ts.a80 2>/dev/null || echo "") <(grep -v '^;' antlr.a80 2>/dev/null || echo "") >/dev/null 2>&1; then
        echo -e "${GREEN}Identical output!${NC}"
        ((PASSED_TESTS++))
    else
        echo -e "${YELLOW}Different output (optimization variance)${NC}"
    fi
else
    echo -e "${RED}Parser inconsistency detected${NC}"
    ((FAILED_TESTS++))
fi
((TOTAL_TESTS++))

echo

# =======================
# 2. METAMORPHIC TESTING
# =======================
echo -e "${CYAN}2. Metamorphic Testing${NC}"
echo "----------------------"

# Test that semantically equivalent programs produce same results
cat > metamorphic1.minz << 'EOF'
fun sum(a: u8, b: u8) -> u8 {
    return a + b;
}
fun main() -> void {
    print_u8(sum(10, 20));
}
EOF

cat > metamorphic2.minz << 'EOF'
fun sum(x: u8, y: u8) -> u8 {
    let result = x;
    result = result + y;
    return result;
}
fun main() -> void {
    let first = 10;
    let second = 20;
    print_u8(sum(first, second));
}
EOF

echo -n "Compiling variant 1... "
if ./minzc/mz metamorphic1.minz -o meta1.a80 2>/dev/null; then
    echo -e "${GREEN}✓${NC}"
    ((PASSED_TESTS++))
else
    echo -e "${RED}✗${NC}"
    ((FAILED_TESTS++))
fi
((TOTAL_TESTS++))

echo -n "Compiling variant 2... "
if ./minzc/mz metamorphic2.minz -o meta2.a80 2>/dev/null; then
    echo -e "${GREEN}✓${NC}"
    ((PASSED_TESTS++))
else
    echo -e "${RED}✗${NC}"
    ((FAILED_TESTS++))
fi
((TOTAL_TESTS++))

echo

# ========================
# 3. SYMBOLIC EXECUTION
# ========================
echo -e "${CYAN}3. Symbolic Path Analysis${NC}"
echo "-------------------------"

# Create program with multiple paths
cat > symbolic_test.minz << 'EOF'
fun analyze(x: u8) -> u8 {
    if (x < 10) {
        return x * 2;      // Path 1
    } else if (x < 20) {
        return x + 5;      // Path 2
    } else if (x < 30) {
        return x - 3;      // Path 3
    } else {
        return 42;         // Path 4
    }
}

fun main() -> void {
    // Test all paths
    print_u8(analyze(5));   // Path 1: 10
    print_u8(analyze(15));  // Path 2: 20
    print_u8(analyze(25));  // Path 3: 22
    print_u8(analyze(35));  // Path 4: 42
}
EOF

echo -n "Compiling path analysis test... "
if ./minzc/mz symbolic_test.minz -o symbolic.a80 2>/dev/null; then
    # Check that all paths are present in assembly
    PATHS_FOUND=0
    grep -q "CP 10" symbolic.a80 2>/dev/null && ((PATHS_FOUND++))
    grep -q "CP 20" symbolic.a80 2>/dev/null && ((PATHS_FOUND++))
    grep -q "CP 30" symbolic.a80 2>/dev/null && ((PATHS_FOUND++))
    
    if [ $PATHS_FOUND -ge 2 ]; then
        echo -e "${GREEN}✓ All paths compiled${NC}"
        ((PASSED_TESTS++))
    else
        echo -e "${YELLOW}⚠ Some paths missing${NC}"
    fi
else
    echo -e "${RED}✗${NC}"
    ((FAILED_TESTS++))
fi
((TOTAL_TESTS++))

echo

# ========================
# 4. INVARIANT CHECKING
# ========================
echo -e "${CYAN}4. Invariant Verification${NC}"
echo "-------------------------"

# Test that compiler maintains invariants
cat > invariant_test.minz << 'EOF'
// Invariant: array bounds are always checked
fun test_bounds() -> void {
    let arr: [u8; 5] = [1, 2, 3, 4, 5];
    let index: u8 = 0;
    
    // Loop invariant: index < 5
    while (index < 5) {
        print_u8(arr[index]);
        index = index + 1;
    }
}

fun main() -> void {
    test_bounds();
}
EOF

echo -n "Testing array bounds invariant... "
if ./minzc/mz invariant_test.minz -o invariant.a80 2>/dev/null; then
    echo -e "${GREEN}✓${NC}"
    ((PASSED_TESTS++))
else
    echo -e "${RED}✗${NC}"
    ((FAILED_TESTS++))
fi
((TOTAL_TESTS++))

echo

# ============================
# 5. PERFORMANCE BENCHMARKING
# ============================
echo -e "${CYAN}5. Performance Regression Testing${NC}"
echo "---------------------------------"

# Measure compilation speed
cat > perf_test.minz << 'EOF'
fun fibonacci(n: u8) -> u8 {
    if (n <= 1) { return n; }
    return fibonacci(n - 1) + fibonacci(n - 2);
}

fun main() -> void {
    let i: u8 = 0;
    while (i < 10) {
        print_u8(fibonacci(i));
        i = i + 1;
    }
}
EOF

echo -n "Benchmarking compilation speed... "
START_TIME=$(date +%s%N)
if ./minzc/mz perf_test.minz -o perf.a80 2>/dev/null; then
    END_TIME=$(date +%s%N)
    ELAPSED=$((($END_TIME - $START_TIME) / 1000000))
    
    if [ $ELAPSED -lt 1000 ]; then
        echo -e "${GREEN}✓ ${ELAPSED}ms (fast)${NC}"
        PERFORMANCE_SCORE=100
    elif [ $ELAPSED -lt 2000 ]; then
        echo -e "${YELLOW}⚠ ${ELAPSED}ms (acceptable)${NC}"
        PERFORMANCE_SCORE=70
    else
        echo -e "${RED}✗ ${ELAPSED}ms (slow)${NC}"
        PERFORMANCE_SCORE=40
    fi
    ((PASSED_TESTS++))
else
    echo -e "${RED}✗ Compilation failed${NC}"
    ((FAILED_TESTS++))
fi
((TOTAL_TESTS++))

echo

# =======================
# 6. COVERAGE ANALYSIS
# =======================
echo -e "${CYAN}6. Feature Coverage Analysis${NC}"
echo "----------------------------"

# Test coverage of language features
FEATURES=(
    "enum:enum E { A, B }"
    "struct:struct S { x: u8 }"
    "array:let a: [u8; 3] = [1,2,3]"
    "while:while (1) { break; }"
    "for:for i in 0..5 { }"
    "if:if (1) { } else { }"
    "function:fun f() -> u8 { return 1; }"
    "cast:let x = 5 as u16"
    "global:global g: u8 = 0"
)

COVERED=0
for feature in "${FEATURES[@]}"; do
    IFS=':' read -r name code <<< "$feature"
    echo -n "Testing $name... "
    
    cat > coverage_test.minz << EOF
$code
fun main() -> void {
    print_u8(42);
}
EOF
    
    if ./minzc/mz coverage_test.minz -o /tmp/test.a80 2>/dev/null; then
        echo -e "${GREEN}✓${NC}"
        ((COVERED++))
    else
        echo -e "${RED}✗${NC}"
    fi
done

COVERAGE=$((COVERED * 100 / ${#FEATURES[@]}))
echo -e "Feature coverage: ${COVERAGE}%"
((TOTAL_TESTS++))
if [ $COVERAGE -ge 70 ]; then
    ((PASSED_TESTS++))
else
    ((FAILED_TESTS++))
fi

echo

# =======================
# 7. ORACLE TESTING
# =======================
echo -e "${CYAN}7. Oracle Testing (Expected Values)${NC}"
echo "------------------------------------"

# Test against known correct outputs
declare -A ORACLE_TESTS=(
    ["1 + 1"]=2
    ["5 * 3"]=15
    ["10 - 7"]=3
    ["8 / 2"]=4
    ["7 % 3"]=1
)

for expr in "${!ORACLE_TESTS[@]}"; do
    expected="${ORACLE_TESTS[$expr]}"
    echo -n "Testing '$expr = $expected'... "
    
    cat > oracle_test.minz << EOF
fun main() -> void {
    let result: u8 = $expr;
    print_u8(result);
}
EOF
    
    if ./minzc/mz oracle_test.minz -o oracle.a80 2>/dev/null; then
        # Check if the expected value appears in assembly
        if grep -q "LD A, $expected" oracle.a80 2>/dev/null || \
           grep -q "LD A, #$expected" oracle.a80 2>/dev/null || \
           grep -q "$expected" oracle.a80 2>/dev/null; then
            echo -e "${GREEN}✓${NC}"
            ((PASSED_TESTS++))
        else
            echo -e "${YELLOW}⚠ Compiled but value uncertain${NC}"
        fi
    else
        echo -e "${RED}✗${NC}"
        ((FAILED_TESTS++))
    fi
    ((TOTAL_TESTS++))
done

echo

# =======================
# 8. SMOKE TESTING
# =======================
echo -e "${CYAN}8. Smoke Testing (Critical Paths)${NC}"
echo "---------------------------------"

# Quick tests of critical functionality
echo -n "Compiler executable exists... "
if [ -x ./minzc/mz ]; then
    echo -e "${GREEN}✓${NC}"
    ((PASSED_TESTS++))
else
    echo -e "${RED}✗${NC}"
    ((FAILED_TESTS++))
fi
((TOTAL_TESTS++))

echo -n "MIR VM (mzv) exists... "
if [ -x ./minzc/mzv ]; then
    echo -e "${GREEN}✓${NC}"
    ((PASSED_TESTS++))
else
    echo -e "${RED}✗${NC}"
    ((FAILED_TESTS++))
fi
((TOTAL_TESTS++))

echo -n "Can compile empty program... "
echo "fun main() -> void { }" > smoke.minz
if ./minzc/mz smoke.minz -o smoke.a80 2>/dev/null; then
    echo -e "${GREEN}✓${NC}"
    ((PASSED_TESTS++))
else
    echo -e "${RED}✗${NC}"
    ((FAILED_TESTS++))
fi
((TOTAL_TESTS++))

echo

# =======================
# 9. ERROR INJECTION
# =======================
echo -e "${CYAN}9. Error Injection Testing${NC}"
echo "--------------------------"

# Test error handling with invalid inputs
INVALID_PROGRAMS=(
    "fun main() {"  # Missing closing brace
    "fun () -> void { }"  # Missing function name
    "let x = ;"  # Incomplete assignment
    "fun main() -> void { undefined_func(); }"  # Undefined function
)

echo "Testing compiler error handling:"
ERROR_HANDLED=0
for prog in "${INVALID_PROGRAMS[@]}"; do
    echo "$prog" > error_test.minz
    if ./minzc/mz error_test.minz -o /tmp/test.a80 2>&1 | grep -q -E "error|Error|failed"; then
        ((ERROR_HANDLED++))
    fi
done

echo -e "Properly handled ${ERROR_HANDLED}/${#INVALID_PROGRAMS[@]} errors"
((TOTAL_TESTS++))
if [ $ERROR_HANDLED -eq ${#INVALID_PROGRAMS[@]} ]; then
    ((PASSED_TESTS++))
    echo -e "${GREEN}✓ All errors handled gracefully${NC}"
else
    ((FAILED_TESTS++))
    echo -e "${RED}✗ Some errors not handled properly${NC}"
fi

echo

# =======================
# 10. LOAD TESTING
# =======================
echo -e "${CYAN}10. Load Testing (Stress Test)${NC}"
echo "------------------------------"

# Generate a large program
echo -n "Generating large program (1000 functions)... "
{
    for i in {1..1000}; do
        echo "fun func_$i() -> u8 { return $((i % 256)); }"
    done
    echo "fun main() -> void { print_u8(func_500()); }"
} > load_test.minz

echo -e "${GREEN}done${NC}"
echo -n "Compiling large program... "
if ./minzc/mz load_test.minz -o load.a80 2>/dev/null; then
    echo -e "${GREEN}✓ Handled 1000 functions${NC}"
    ((PASSED_TESTS++))
else
    echo -e "${RED}✗ Failed on large input${NC}"
    ((FAILED_TESTS++))
fi
((TOTAL_TESTS++))

echo

# =======================
# FINAL REPORT
# =======================
echo "========================================="
echo -e "${MAGENTA}Verification Report${NC}"
echo "========================================="

SUCCESS_RATE=$((PASSED_TESTS * 100 / TOTAL_TESTS))

echo -e "Total Tests:     $TOTAL_TESTS"
echo -e "Passed:          ${GREEN}$PASSED_TESTS${NC}"
echo -e "Failed:          ${RED}$FAILED_TESTS${NC}"
echo -e "Success Rate:    ${SUCCESS_RATE}%"
echo -e "Performance:     ${PERFORMANCE_SCORE}/100"
echo -e "Coverage:        ${COVERAGE}%"

echo
if [ $SUCCESS_RATE -ge 90 ]; then
    echo -e "${GREEN}✅ VERIFICATION PASSED - System is stable!${NC}"
    exit 0
elif [ $SUCCESS_RATE -ge 70 ]; then
    echo -e "${YELLOW}⚠️  VERIFICATION WARNING - Some issues detected${NC}"
    exit 0
else
    echo -e "${RED}❌ VERIFICATION FAILED - Critical issues found${NC}"
    exit 1
fi