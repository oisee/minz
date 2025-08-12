#!/bin/bash

# Comprehensive E2E test to analyze the 85% success rate
# Tests all examples and categorizes failures

echo "=== MinZ v0.13.0 E2E Comprehensive Test ==="
echo "Testing all examples to analyze 85% success rate"
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Counters
total=0
success=0
failures=0

# Failure categories
declare -A failure_types
failure_types["semantic"]=0
failure_types["codegen"]=0
failure_types["parse"]=0
failure_types["other"]=0

# Lists to track
success_files=""
failed_files=""
semantic_failures=""
codegen_failures=""
parse_failures=""

# Test function
test_file() {
    local file=$1
    local filename=$(basename "$file")
    
    # Skip non-.minz files
    if [[ ! "$file" == *.minz ]]; then
        return
    fi
    
    total=$((total + 1))
    
    # Try to compile
    output=$(./minzc/mz "$file" -o /tmp/test.a80 2>&1)
    exit_code=$?
    
    if [ $exit_code -eq 0 ]; then
        success=$((success + 1))
        success_files="$success_files\n  ✅ $filename"
        echo -n "."
    else
        failures=$((failures + 1))
        failed_files="$failed_files\n  ❌ $filename"
        echo -n "F"
        
        # Categorize failure
        if echo "$output" | grep -q "semantic error"; then
            failure_types["semantic"]=$((failure_types["semantic"] + 1))
            semantic_failures="$semantic_failures\n    - $filename"
        elif echo "$output" | grep -q "code generation failed"; then
            failure_types["codegen"]=$((failure_types["codegen"] + 1))
            codegen_failures="$codegen_failures\n    - $filename"
        elif echo "$output" | grep -q "parse error\|parsing failed"; then
            failure_types["parse"]=$((failure_types["parse"] + 1))
            parse_failures="$parse_failures\n    - $filename"
        else
            failure_types["other"]=$((failure_types["other"] + 1))
        fi
    fi
}

# Test all example files
echo "Testing examples/ directory..."
for file in examples/*.minz; do
    if [ -f "$file" ]; then
        test_file "$file"
    fi
done

echo ""
echo "Testing test files..."
for file in test_*.minz; do
    if [ -f "$file" ]; then
        test_file "$file"
    fi
done

echo ""
echo "Testing stdlib modules..."
for file in stdlib/*.minz; do
    if [ -f "$file" ]; then
        test_file "$file"
    fi
done

echo ""
echo ""
echo "══════════════════════════════════════════════"
echo "                 TEST RESULTS                  "
echo "══════════════════════════════════════════════"
echo ""

# Calculate success rate
if [ $total -gt 0 ]; then
    success_rate=$(( success * 100 / total ))
else
    success_rate=0
fi

# Summary
echo -e "${GREEN}✅ Successful: $success/$total ($success_rate%)${NC}"
echo -e "${RED}❌ Failed: $failures/$total${NC}"
echo ""

# Failure breakdown
echo "📊 FAILURE BREAKDOWN:"
echo "  Semantic errors: ${failure_types["semantic"]}"
echo "  Code generation: ${failure_types["codegen"]}"
echo "  Parse errors: ${failure_types["parse"]}"
echo "  Other errors: ${failure_types["other"]}"
echo ""

# Detailed lists
if [ $failures -gt 0 ]; then
    echo "❌ FAILED FILES BY CATEGORY:"
    
    if [ ${failure_types["semantic"]} -gt 0 ]; then
        echo ""
        echo "  Semantic Errors (${failure_types["semantic"]}):"
        echo -e "$semantic_failures"
    fi
    
    if [ ${failure_types["codegen"]} -gt 0 ]; then
        echo ""
        echo "  Code Generation Errors (${failure_types["codegen"]}):"
        echo -e "$codegen_failures"
    fi
    
    if [ ${failure_types["parse"]} -gt 0 ]; then
        echo ""
        echo "  Parse Errors (${failure_types["parse"]}):"
        echo -e "$parse_failures"
    fi
fi

echo ""
echo "══════════════════════════════════════════════"
echo ""

# Analysis
echo "📈 ANALYSIS:"
if [ $success_rate -ge 85 ]; then
    echo -e "  ${GREEN}✅ Target 85% success rate ACHIEVED!${NC}"
elif [ $success_rate -ge 80 ]; then
    echo -e "  ${YELLOW}⚠️ Close to 85% target (currently $success_rate%)${NC}"
else
    echo -e "  ${RED}❌ Below target (currently $success_rate%, target 85%)${NC}"
fi

echo ""
echo "🎯 NEXT PRIORITIES (based on failures):"
if [ ${failure_types["semantic"]} -gt 0 ]; then
    echo "  1. Fix semantic analysis issues (${failure_types["semantic"]} failures)"
fi
if [ ${failure_types["codegen"]} -gt 0 ]; then
    echo "  2. Fix code generation issues (${failure_types["codegen"]} failures)"
fi
if [ ${failure_types["parse"]} -gt 0 ]; then
    echo "  3. Fix parsing issues (${failure_types["parse"]} failures)"
fi

echo ""
echo "Test completed at $(date)"