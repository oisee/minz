#!/bin/bash

# Test all MinZ backends with the comprehensive test suite
# This script compiles and reports on each backend's capabilities

echo "=== MinZ Backend Test Suite ==="
echo "Testing all available backends..."
echo

# Color codes for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test file
TEST_FILE="../tests/backend_test.minz"

# Function to test a backend
test_backend() {
    local backend=$1
    local extension=$2
    
    echo -n "Testing $backend backend... "
    
    # Try to compile
    if ./minzc "$TEST_FILE" -b "$backend" -o "test_$backend$extension" 2>test_$backend.err; then
        echo -e "${GREEN}✓ Compilation successful${NC}"
        
        # Check file size
        if [ -f "test_$backend$extension" ]; then
            size=$(wc -c < "test_$backend$extension")
            echo "  Output size: $size bytes"
        fi
        
        # Check for specific features
        echo -n "  Features: "
        
        # Check if it handles globals
        if grep -q "test_global" "test_$backend$extension" 2>/dev/null; then
            echo -n "globals "
        fi
        
        # Check if it handles recursion
        if grep -q "factorial" "test_$backend$extension" 2>/dev/null; then
            echo -n "recursion "
        fi
        
        # Check if SMC is mentioned
        if grep -q "SMC" "test_$backend$extension" 2>/dev/null; then
            echo -n "SMC "
        fi
        
        echo
    else
        echo -e "${RED}✗ Compilation failed${NC}"
        echo "  Error: $(head -n 1 test_$backend.err)"
    fi
    
    echo
}

# List of backends to test
BACKENDS=(
    "z80:.a80"
    "6502:.s"
    "68k:.s"
    "m68k:.s"
    "i8080:.s"
    "gb:.s"
    "wasm:.wat"
    "c:.c"
    "llvm:.ll"
)

# Test each backend
for backend_spec in "${BACKENDS[@]}"; do
    IFS=':' read -r backend extension <<< "$backend_spec"
    test_backend "$backend" "$extension"
done

# Summary
echo "=== Test Summary ==="
echo

# Count successful compilations
success_count=0
total_count=${#BACKENDS[@]}

for backend_spec in "${BACKENDS[@]}"; do
    IFS=':' read -r backend extension <<< "$backend_spec"
    if [ -f "test_$backend$extension" ]; then
        ((success_count++))
    fi
done

echo "Successful compilations: $success_count/$total_count"

# Generate feature comparison
echo
echo "=== Feature Support Matrix ==="
echo
echo "Backend | Compiles | SMC | Globals | Recursion | File Size"
echo "--------|----------|-----|---------|-----------|----------"

for backend_spec in "${BACKENDS[@]}"; do
    IFS=':' read -r backend extension <<< "$backend_spec"
    printf "%-7s | " "$backend"
    
    if [ -f "test_$backend$extension" ]; then
        printf "%-8s | " "Yes"
        
        # Check features
        if grep -q "SMC" "test_$backend$extension" 2>/dev/null; then
            printf "%-3s | " "Yes"
        else
            printf "%-3s | " "No"
        fi
        
        if grep -q "test_global" "test_$backend$extension" 2>/dev/null; then
            printf "%-7s | " "Yes"
        else
            printf "%-7s | " "No"
        fi
        
        if grep -q "factorial" "test_$backend$extension" 2>/dev/null; then
            printf "%-9s | " "Yes"
        else
            printf "%-9s | " "No"
        fi
        
        size=$(wc -c < "test_$backend$extension" 2>/dev/null || echo "0")
        printf "%9s" "$size"
    else
        printf "%-8s | %-3s | %-7s | %-9s | %9s" "No" "-" "-" "-" "-"
    fi
    
    echo
done

# Cleanup
echo
echo -n "Cleaning up test files... "
rm -f test_*.a80 test_*.s test_*.wat test_*.c test_*.ll test_*.err test_*.mir
echo "done."

echo
echo "Backend testing complete!"