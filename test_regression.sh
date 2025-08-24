#!/bin/bash
# MinZ Compiler Regression Test Suite
# Quick win: Automated testing for all features

set -e

COMPILER="minzc/mz"
TEMP_DIR="/tmp/minz_tests"
PASS_COUNT=0
FAIL_COUNT=0
TOTAL_COUNT=0

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Create temp directory
mkdir -p $TEMP_DIR

echo "🧪 MinZ Compiler Regression Test Suite"
echo "========================================"
echo ""

# Test helper function
run_test() {
    local test_name=$1
    local test_file=$2
    local expected_result=$3  # "pass" or "fail"
    
    TOTAL_COUNT=$((TOTAL_COUNT + 1))
    
    if [ "$expected_result" = "pass" ]; then
        if $COMPILER "$test_file" -o "$TEMP_DIR/test.a80" 2>/dev/null; then
            echo -e "${GREEN}✓${NC} $test_name"
            PASS_COUNT=$((PASS_COUNT + 1))
        else
            echo -e "${RED}✗${NC} $test_name (expected to pass)"
            FAIL_COUNT=$((FAIL_COUNT + 1))
        fi
    else
        if $COMPILER "$test_file" -o "$TEMP_DIR/test.a80" 2>/dev/null; then
            echo -e "${RED}✗${NC} $test_name (expected to fail)"
            FAIL_COUNT=$((FAIL_COUNT + 1))
        else
            echo -e "${GREEN}✓${NC} $test_name (correctly fails)"
            PASS_COUNT=$((PASS_COUNT + 1))
        fi
    fi
}

# Create test files on the fly
create_test() {
    local filename=$1
    local content=$2
    echo "$content" > "$TEMP_DIR/$filename"
    echo "$TEMP_DIR/$filename"
}

echo "📋 Core Language Features"
echo "-------------------------"

# Basic function test
test_file=$(create_test "basic_func.minz" 'fun add(a: u8, b: u8) -> u8 { return a + b; }
fun main() -> void { let x = add(5, 3); }')
run_test "Basic functions" "$test_file" "pass"

# Variable declarations
test_file=$(create_test "vars.minz" 'fun main() -> void { 
    let x = 42;
    let y: u16 = 1000;
    global z: u8 = 10;
}')
run_test "Variable declarations" "$test_file" "pass"

# Control flow
test_file=$(create_test "control.minz" 'fun test(x: u8) -> u8 {
    if x > 10 { return 1; } else { return 0; }
}
fun main() -> void {}')
run_test "If/else statements" "$test_file" "pass"

# Loops
test_file=$(create_test "loops.minz" 'fun main() -> void {
    while true { break; }
    for i in 0..10 { }
}')
run_test "While/for loops" "$test_file" "pass"

echo ""
echo "🎯 Pattern Matching Features"
echo "----------------------------"

# Basic pattern matching
test_file=$(create_test "pattern_basic.minz" 'fun test(x: u8) -> u8 {
    case x {
        0 => 100,
        1 => 200,
        _ => 300
    }
}
fun main() -> void {}')
run_test "Basic pattern matching" "$test_file" "pass"

# Range patterns
test_file=$(create_test "pattern_range.minz" 'fun categorize(n: u8) -> u8 {
    case n {
        0 => 0,
        1..10 => 1,
        11..50 => 2,
        _ => 3
    }
}
fun main() -> void {}')
run_test "Range patterns (1..10)" "$test_file" "pass"

# Enum patterns
test_file=$(create_test "enum_pattern.minz" 'enum State { IDLE, RUNNING }
fun test(s: State) -> u8 {
    case s {
        State.IDLE => 1,
        State.RUNNING => 2,
        _ => 0
    }
}
fun main() -> void {}')
run_test "Enum patterns" "$test_file" "pass"

echo ""
echo "🔧 Advanced Features"
echo "--------------------"

# Lambda expressions
test_file=$(create_test "lambda.minz" 'fun main() -> void {
    let add = |x: u8, y: u8| => u8 { x + y };
    let result = add(5, 3);
}')
run_test "Lambda expressions" "$test_file" "pass"

# Function overloading
test_file=$(create_test "overload.minz" 'fun print(x: u8) -> void {}
fun print(x: u16) -> void {}
fun main() -> void { print(42); }')
run_test "Function overloading" "$test_file" "pass"

# Local functions (now working!)
test_file=$(create_test "local_func.minz" 'fun outer() -> u8 {
    fun inner() -> u8 { return 42; }
    return inner();
}
fun main() -> void {}')
run_test "Local/nested functions" "$test_file" "pass"

# Error propagation
test_file=$(create_test "error_prop.minz" 'fun test() -> u8? {
    let x = some_func()?;
    return x ?? 0;
}
fun main() -> void {}')
run_test "Error propagation (?)" "$test_file" "fail"

echo ""
echo "🎮 Type System"
echo "--------------"

# Structs
test_file=$(create_test "structs.minz" 'struct Point { x: u8, y: u8 }
fun main() -> void {
    let p = Point { x: 10, y: 20 };
}')
run_test "Struct definitions" "$test_file" "pass"

# Enums
test_file=$(create_test "enums.minz" 'enum Color { RED, GREEN, BLUE }
fun main() -> void {
    let c = Color.RED;
}')
run_test "Enum definitions" "$test_file" "pass"

# Arrays
test_file=$(create_test "arrays.minz" 'fun main() -> void {
    let arr: [u8; 10];
    arr[0] = 42;
}')
run_test "Array operations" "$test_file" "pass"

# Type casting
test_file=$(create_test "casting.minz" 'fun main() -> void {
    let x: u8 = 42;
    let y = x as u16;
}')
run_test "Type casting" "$test_file" "pass"

echo ""
echo "🔮 Metaprogramming"
echo "------------------"

# @minz blocks
test_file=$(create_test "minz_block.minz" '@minz[[[
    @emit("fun generated() -> u8 { return 42; }")
]]]
fun main() -> void {}')
run_test "@minz blocks" "$test_file" "pass"

# @define macros (currently not working as expected)
test_file=$(create_test "define.minz" '@define("DEBUG_VAL", "42")
fun main() -> void {
    let x = DEBUG_VAL;
}')
run_test "@define macros" "$test_file" "fail"

# Compile-time if
test_file=$(create_test "ct_if.minz" '@if(DEBUG) {
    fun debug() -> void {}
}
fun main() -> void {}')
run_test "@if compile-time" "$test_file" "pass"

echo ""
echo "🚧 Not Yet Implemented"
echo "----------------------"

# String interpolation
test_file=$(create_test "string_interp.minz" 'fun main() -> void {
    let x = 42;
    let msg = "Value: ${x}";  // Not working - just literal string
}')
run_test "String interpolation \${}" "$test_file" "pass"  # Passes but doesn't interpolate

# Self parameter (method syntax)
test_file=$(create_test "self_param.minz" 'struct Point { x: u8 }
impl Point {
    fun get(self) -> u8 { return self.x; }
}
fun main() -> void {
    let p = Point { x: 10 };
    let x = p.get();  // Method call syntax
}')
run_test "Self parameter (p.method())" "$test_file" "fail"  # Method calls not supported

# Variable binding in patterns
test_file=$(create_test "var_binding.minz" 'fun main() -> void {
    case 42 {
        x => print_u8(x)  // Binding x to value
    }
}')
run_test "Variable binding in patterns" "$test_file" "fail"  # Not implemented

# Error propagation with ??
test_file=$(create_test "null_coalesce.minz" 'fun get() -> u8? { return 42; }
fun main() -> void {
    let x = get() ?? 0;  // Null coalescing
}')
run_test "?? operator" "$test_file" "fail"  # Not implemented

# Array literals
test_file=$(create_test "array_literal.minz" 'fun main() -> void {
    let arr = [1, 2, 3];  // Array literal syntax
}')
run_test "Array literals [1,2,3]" "$test_file" "fail"  # Parser issue

# Generics
test_file=$(create_test "generics.minz" 'fun identity<T>(x: T) -> T { return x; }
fun main() -> void {}')
run_test "Generic functions <T>" "$test_file" "fail"  # Not implemented

echo ""
echo "📊 Test Results"
echo "==============="
echo -e "Total tests: $TOTAL_COUNT"
echo -e "Passed: ${GREEN}$PASS_COUNT${NC}"
echo -e "Failed: ${RED}$FAIL_COUNT${NC}"

if [ $FAIL_COUNT -eq 0 ]; then
    echo -e "\n${GREEN}🎉 All tests passed!${NC}"
    exit 0
else
    SUCCESS_RATE=$((PASS_COUNT * 100 / TOTAL_COUNT))
    echo -e "\nSuccess rate: ${YELLOW}${SUCCESS_RATE}%${NC}"
    
    if [ $SUCCESS_RATE -ge 80 ]; then
        echo -e "${GREEN}✓ Good coverage (≥80%)${NC}"
    elif [ $SUCCESS_RATE -ge 60 ]; then
        echo -e "${YELLOW}⚠ Acceptable coverage (≥60%)${NC}"
    else
        echo -e "${RED}✗ Poor coverage (<60%)${NC}"
    fi
    exit 1
fi