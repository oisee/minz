#!/bin/bash

# Comprehensive MinZ Examples Test Script
# Tests compilation of all examples and categorizes results

echo "MinZ Examples Compilation Analysis Report"
echo "========================================"
echo "Date: $(date)"
echo ""

# Arrays to store results
declare -a PASS_FILES=()
declare -a FAIL_FILES=()
declare -a HANG_FILES=()

# Known hanging files (from previous testing)
KNOWN_HANGS=(
    "test_abi_comparison.minz"
)

# Function to check if file is known to hang
is_known_hang() {
    local file=$1
    local basename=$(basename "$file")
    for hang in "${KNOWN_HANGS[@]}"; do
        if [[ "$basename" == "$hang" ]]; then
            return 0
        fi
    done
    return 1
}

# Test function with proper error capture
test_file() {
    local file=$1
    local basename=$(basename "$file" .minz)
    local output_file="/tmp/${basename}.a80"
    
    # Skip known hanging files
    if is_known_hang "$file"; then
        echo "⏱️  SKIP (known hang): $file"
        HANG_FILES+=("$file|Known to hang compiler")
        return
    fi
    
    # Try to compile
    ./minzc "$file" -o "$output_file" > /tmp/compile_stdout.txt 2> /tmp/compile_stderr.txt
    local exit_code=$?
    
    if [ $exit_code -eq 0 ]; then
        echo "✅ PASS: $file"
        PASS_FILES+=("$file")
    else
        # Extract error message
        local error_msg=$(grep -E "Error:|error:" /tmp/compile_stderr.txt | head -n 1)
        if [ -z "$error_msg" ]; then
            error_msg="Exit code: $exit_code"
        fi
        echo "❌ FAIL: $file"
        echo "   Error: $error_msg"
        FAIL_FILES+=("$file|$error_msg")
    fi
}

# Process all files
while IFS= read -r file; do
    test_file "$file"
done < /tmp/minz_examples.txt

echo ""
echo "========================================"
echo "DETAILED ANALYSIS"
echo "========================================"
echo ""

# 1. PASSING EXAMPLES
echo "1. PASSING EXAMPLES (${#PASS_FILES[@]} files)"
echo "----------------------------------------"
for file in "${PASS_FILES[@]}"; do
    echo "   ✅ $file"
done

echo ""

# 2. FAILING EXAMPLES WITH ERRORS
echo "2. FAILING EXAMPLES (${#FAIL_FILES[@]} files)"
echo "----------------------------------------"
IFS='|'
for entry in "${FAIL_FILES[@]}"; do
    read -r file error <<< "$entry"
    echo "   ❌ $file"
    echo "      Error: $error"
done
IFS=$' \t\n'

echo ""

# 3. HANGING EXAMPLES
echo "3. HANGING EXAMPLES (${#HANG_FILES[@]} files)"
echo "----------------------------------------"
IFS='|'
for entry in "${HANG_FILES[@]}"; do
    read -r file reason <<< "$entry"
    echo "   ⏱️  $file"
    echo "      Reason: $reason"
done
IFS=$' \t\n'

echo ""
echo "========================================"
echo "SUMMARY STATISTICS"
echo "========================================"
echo "✅ Passed:  ${#PASS_FILES[@]}"
echo "❌ Failed:  ${#FAIL_FILES[@]}"
echo "⏱️  Hanging: ${#HANG_FILES[@]}"
echo "📊 Total:   $((${#PASS_FILES[@]} + ${#FAIL_FILES[@]} + ${#HANG_FILES[@]}))"
if [ $((${#PASS_FILES[@]} + ${#FAIL_FILES[@]})) -gt 0 ]; then
    echo "🎯 Success Rate (excluding hangs): $(( ${#PASS_FILES[@]} * 100 / (${#PASS_FILES[@]} + ${#FAIL_FILES[@]}) ))%"
fi