#!/bin/bash

# Test actual compilation success rate for MinZ v0.13.0

echo "=== MinZ v0.13.0 Compilation Success Test ==="
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Counters
total=0
success=0
failed=0

# Arrays for tracking
declare -a success_files
declare -a failed_files

echo "Testing examples directory..."
for file in examples/*.minz; do
    if [ -f "$file" ]; then
        filename=$(basename "$file")
        total=$((total + 1))
        
        # Try to compile (suppress debug output)
        if ./minzc/mz "$file" -o test_output.a80 2>/dev/null; then
            success=$((success + 1))
            success_files+=("$filename")
            echo -n "✓"
        else
            failed=$((failed + 1))
            failed_files+=("$filename")
            echo -n "✗"
        fi
    fi
done

echo ""
echo "Testing test files..."
for file in test_*.minz; do
    if [ -f "$file" ]; then
        filename=$(basename "$file")
        total=$((total + 1))
        
        # Try to compile (suppress debug output)
        if ./minzc/mz "$file" -o test_output.a80 2>/dev/null; then
            success=$((success + 1))
            success_files+=("$filename")
            echo -n "✓"
        else
            failed=$((failed + 1))
            failed_files+=("$filename")
            echo -n "✗"
        fi
    fi
done

echo ""
echo ""
echo "══════════════════════════════════════════"
echo "           COMPILATION RESULTS             "
echo "══════════════════════════════════════════"
echo ""

# Calculate success rate
if [ $total -gt 0 ]; then
    success_rate=$(( success * 100 / total ))
else
    success_rate=0
fi

# Summary with colors
echo -e "${GREEN}✅ Successful: $success/$total ($success_rate%)${NC}"
echo -e "${RED}❌ Failed: $failed/$total${NC}"
echo ""

# Show some successful examples
if [ ${#success_files[@]} -gt 0 ]; then
    echo "✅ WORKING EXAMPLES (first 10):"
    count=0
    for file in "${success_files[@]}"; do
        if [ $count -lt 10 ]; then
            echo "  • $file"
            count=$((count + 1))
        fi
    done
    if [ ${#success_files[@]} -gt 10 ]; then
        echo "  ... and $((${#success_files[@]} - 10)) more"
    fi
    echo ""
fi

# Show some failures
if [ ${#failed_files[@]} -gt 0 ]; then
    echo "❌ FAILED EXAMPLES (first 10):"
    count=0
    for file in "${failed_files[@]}"; do
        if [ $count -lt 10 ]; then
            echo "  • $file"
            count=$((count + 1))
        fi
    done
    if [ ${#failed_files[@]} -gt 10 ]; then
        echo "  ... and $((${#failed_files[@]} - 10)) more"
    fi
    echo ""
fi

# Achievement check
echo "📊 STATUS:"
if [ $success_rate -ge 85 ]; then
    echo -e "  ${GREEN}🎉 TARGET 85% ACHIEVED!${NC}"
elif [ $success_rate -ge 80 ]; then
    echo -e "  ${YELLOW}⚠️ Close to 85% target (currently $success_rate%)${NC}"
elif [ $success_rate -ge 70 ]; then
    echo -e "  ${YELLOW}📈 Good progress at $success_rate%${NC}"
else
    echo -e "  ${RED}📉 Below expectations at $success_rate%${NC}"
fi

echo ""
echo "Clean up test file..."
rm -f test_output.a80 test_output.mir

echo "Test completed: $(date)"