#!/bin/bash

# Function to check for pattern in assembly
check_pattern() {
    local file=$1
    local pattern=$2
    local description=$3
    
    if grep -q "$pattern" "$file"; then
        echo "✅ $description"
        return 1
    else
        echo "❌ $description"
        return 0
    fi
}

# Test a MinZ file
test_file() {
    local input=$1
    local name=$(basename $input .minz)
    
    echo "=== Testing $name ==="
    
    # Compile with TRUE SMC
    ./minzc "$input" -o "${name}_true_smc.a80" -O --enable-true-smc
    
    if [ $? -ne 0 ]; then
        echo "❌ Compilation failed"
        return 0
    fi
    
    # Check patterns
    local score=0
    
    # Check for anchors
    check_pattern "${name}_true_smc.a80" "\$imm0:" "Has parameter anchors"
    score=$((score + $?))
    
    # Check for anchor reuse
    check_pattern "${name}_true_smc.a80" "LD.*(\w\+\$imm0)" "Has anchor reuse"
    score=$((score + $?))
    
    # Check for immediate loads in anchors
    check_pattern "${name}_true_smc.a80" "LD.*0.*anchor" "Has immediate anchors"
    score=$((score + $?))
    
    echo "Score: $score/3"
    echo ""
    
    return $score
}

# Run all tests
echo "TRUE SMC Validation Test Suite"
echo "=============================="
echo ""

total_score=0
test_count=0

for test in ../examples/test*.minz; do
    if [ -f "$test" ]; then
        test_file "$test"
        score=$?
        total_score=$((total_score + score))
        test_count=$((test_count + 1))
    fi
done

echo "=============================="
echo "TOTAL SCORE: $total_score / $((test_count * 3))"
echo ""

# Calculate grade
if [ $test_count -gt 0 ]; then
    percentage=$(( (total_score * 100) / (test_count * 3) ))
    echo -n "Grade: "
    if [ $percentage -ge 90 ]; then
        echo "A - Excellent TRUE SMC implementation"
    elif [ $percentage -ge 80 ]; then
        echo "B - Good TRUE SMC implementation"
    elif [ $percentage -ge 70 ]; then
        echo "C - Acceptable TRUE SMC implementation"
    else
        echo "D - Needs improvement"
    fi
fi