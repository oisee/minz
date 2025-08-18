#!/bin/bash
# E2E Regression Test Suite for MinZ Compiler v0.14.1+
# Tests all examples and tracks success rate

echo "=========================================="
echo "MinZ E2E Regression Test Suite"
echo "Date: $(date)"
echo "Version: $(./minzc/mz --version)"
echo "=========================================="
echo

# Statistics
total=0
success=0
failed=0
failed_files=""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test function
test_file() {
    local file=$1
    local name=$(basename "$file")
    printf "Testing %-50s " "$name..."
    
    if ./minzc/mz "$file" -o /tmp/test.a80 2>/dev/null; then
        echo -e "${GREEN}✓${NC}"
        ((success++))
        return 0
    else
        echo -e "${RED}✗${NC}"
        ((failed++))
        failed_files="$failed_files\n  - $name"
        return 1
    fi
}

# Test all examples
echo "=== Testing Examples Directory ==="
for file in examples/*.minz; do
    if [ -f "$file" ]; then
        ((total++))
        test_file "$file"
    fi
done

# Test feature tests
echo
echo "=== Testing Feature Tests ==="
for file in examples/feature_tests/*.minz; do
    if [ -f "$file" ]; then
        ((total++))
        test_file "$file"
    fi
done

# Test stdlib files
echo
echo "=== Testing Standard Library ==="
for file in stdlib/std/*.minz stdlib/zx/*.minz; do
    if [ -f "$file" ]; then
        ((total++))
        test_file "$file"
    fi
done

# Test games
echo
echo "=== Testing Games ==="
for file in games/*.minz; do
    if [ -f "$file" ]; then
        ((total++))
        test_file "$file"
    fi
done

# Test specific regression cases
echo
echo "=== Testing Regression Cases ==="
# Test pointer arithmetic (fixed in this session)
echo 'fun test(data: *u8) -> u16 { return (*data as u16); }' > /tmp/test_cast.minz
((total++))
test_file "/tmp/test_cast.minz"

# Test self parameter (fixed in this session)
echo 'struct Point { x: u8 } impl Point { fun move(self: *Point, dx: u8) { self.x = self.x + dx; } }' > /tmp/test_self.minz
((total++))
test_file "/tmp/test_self.minz"

# Test inline assembly
echo 'fun test() -> u8 { let r: u8 = 0; @asm { LD A, 42; LD (r), A } return r; }' > /tmp/test_asm.minz
((total++))
test_file "/tmp/test_asm.minz"

# Calculate percentage
if [ $total -gt 0 ]; then
    percentage=$((success * 100 / total))
else
    percentage=0
fi

# Summary
echo
echo "=========================================="
echo "RESULTS SUMMARY"
echo "=========================================="
echo -e "Total files tested: $total"
echo -e "Successful:        ${GREEN}$success${NC}"
echo -e "Failed:            ${RED}$failed${NC}"
echo -e "Success rate:      ${YELLOW}${percentage}%${NC}"

if [ $failed -gt 0 ]; then
    echo
    echo "Failed files:"
    echo -e "$failed_files"
fi

# Compare with baseline
baseline_percentage=77  # Our previous success rate
echo
echo "=========================================="
echo "COMPARISON WITH BASELINE"
echo "=========================================="
echo "Baseline success rate: ${baseline_percentage}%"
echo "Current success rate:  ${percentage}%"

if [ $percentage -gt $baseline_percentage ]; then
    improvement=$((percentage - baseline_percentage))
    echo -e "${GREEN}✅ IMPROVEMENT: +${improvement}%${NC}"
elif [ $percentage -eq $baseline_percentage ]; then
    echo -e "${YELLOW}➖ NO CHANGE${NC}"
else
    regression=$((baseline_percentage - percentage))
    echo -e "${RED}⚠️  REGRESSION: -${regression}%${NC}"
fi

# Clean up temp files
rm -f /tmp/test.a80 /tmp/test.mir /tmp/test_*.minz

# Exit with appropriate code
if [ $percentage -ge $baseline_percentage ]; then
    exit 0
else
    exit 1
fi