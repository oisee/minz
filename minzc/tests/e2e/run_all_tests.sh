#!/bin/bash

# Comprehensive test runner to get actual statistics
# This will test CTIE and general compilation across all examples

set -e

echo "================================================"
echo "     MINZ COMPREHENSIVE TEST STATISTICS        "
echo "================================================"
echo

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Counters
TOTAL=0
PASSED=0
FAILED=0
CTIE_OPTIMIZED=0
WARNINGS=0

# Lists for tracking
FAILED_FILES=""
OPTIMIZED_FILES=""
WARNING_FILES=""

# Test a single file
test_file() {
    local file="$1"
    local name=$(basename "$file" .minz)
    
    TOTAL=$((TOTAL + 1))
    echo -n "Testing $name... "
    
    # Try normal compilation first
    if ../../mz "$file" -o "/tmp/${name}.a80" 2>/dev/null; then
        # Normal compilation succeeded
        
        # Now try with CTIE
        if ../../mz "$file" --enable-ctie -o "/tmp/${name}_ctie.a80" 2>/dev/null; then
            # Check if CTIE actually optimized anything
            if grep -q "CTIE:" "/tmp/${name}_ctie.a80" 2>/dev/null; then
                echo -e "${GREEN}✓${NC} (CTIE optimized!)"
                PASSED=$((PASSED + 1))
                CTIE_OPTIMIZED=$((CTIE_OPTIMIZED + 1))
                OPTIMIZED_FILES="$OPTIMIZED_FILES\n  - $name"
            else
                echo -e "${GREEN}✓${NC}"
                PASSED=$((PASSED + 1))
            fi
        else
            # CTIE compilation failed but normal worked
            echo -e "${YELLOW}⚠${NC} (CTIE failed)"
            PASSED=$((PASSED + 1))
            WARNINGS=$((WARNINGS + 1))
            WARNING_FILES="$WARNING_FILES\n  - $name (CTIE compilation failed)"
        fi
    else
        # Compilation failed
        echo -e "${RED}✗${NC}"
        FAILED=$((FAILED + 1))
        FAILED_FILES="$FAILED_FILES\n  - $name"
    fi
    
    # Clean up
    rm -f "/tmp/${name}.a80" "/tmp/${name}_ctie.a80"
}

echo -e "${BLUE}=== Testing Examples Directory ===${NC}"
for file in ../../examples/*.minz; do
    if [ -f "$file" ]; then
        test_file "$file"
    fi
done

echo
echo -e "${BLUE}=== Testing Test Files ===${NC}"
for file in ../*.minz; do
    if [ -f "$file" ]; then
        test_file "$file"
    fi
done

echo
echo -e "${BLUE}=== Testing MinZ Test Suite ===${NC}"
for file in ../minz/*.minz; do
    if [ -f "$file" ]; then
        test_file "$file"
    fi
done

# Calculate percentages
if [ $TOTAL -gt 0 ]; then
    PASS_RATE=$((PASSED * 100 / TOTAL))
    CTIE_RATE=$((CTIE_OPTIMIZED * 100 / TOTAL))
else
    PASS_RATE=0
    CTIE_RATE=0
fi

echo
echo "================================================"
echo "              TEST STATISTICS                  "
echo "================================================"
echo

echo -e "${CYAN}Total Files Tested:${NC}     $TOTAL"
echo -e "${GREEN}Successfully Compiled:${NC}  $PASSED ($PASS_RATE%)"
echo -e "${RED}Failed to Compile:${NC}      $FAILED"
echo -e "${YELLOW}Warnings:${NC}               $WARNINGS"
echo

echo -e "${CYAN}=== CTIE Statistics ===${NC}"
echo -e "Files with CTIE optimizations: $CTIE_OPTIMIZED ($CTIE_RATE%)"

if [ -n "$OPTIMIZED_FILES" ]; then
    echo -e "\n${GREEN}Files optimized by CTIE:${NC}$OPTIMIZED_FILES"
fi

if [ -n "$FAILED_FILES" ]; then
    echo -e "\n${RED}Failed files:${NC}$FAILED_FILES"
fi

if [ -n "$WARNING_FILES" ]; then
    echo -e "\n${YELLOW}Files with warnings:${NC}$WARNING_FILES"
fi

echo
echo "================================================"
echo "                  SUMMARY                      "
echo "================================================"
echo

if [ $PASS_RATE -ge 70 ]; then
    echo -e "${GREEN}✅ PASS RATE: $PASS_RATE% - Production Ready!${NC}"
elif [ $PASS_RATE -ge 50 ]; then
    echo -e "${YELLOW}⚠️  PASS RATE: $PASS_RATE% - Needs improvement${NC}"
else
    echo -e "${RED}❌ PASS RATE: $PASS_RATE% - Critical issues${NC}"
fi

echo -e "\n${CYAN}CTIE is optimizing $CTIE_RATE% of compiled files${NC}"

exit 0