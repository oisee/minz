#\!/bin/bash
# Comprehensive E2E Test Suite for MinZ

echo "================================================"
echo "MinZ Comprehensive E2E Test Suite"
echo "================================================"
echo "Date: $(date)"
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

# Counters
total=0
success=0

# Test all examples
for file in examples/*.minz; do
    if [ -f "$file" ]; then
        name=$(basename "$file")
        total=$((total + 1))
        
        if ./minzc/mz "$file" -o /tmp/test.a80 2>/dev/null; then
            echo -e "${GREEN}✅${NC} $name"
            success=$((success + 1))
        else
            echo -e "${RED}❌${NC} $name"
        fi
    fi
done

# Calculate rate
if [ $total -gt 0 ]; then
    rate=$((success * 100 / total))
else
    rate=0
fi

echo ""
echo "================================================"
echo "RESULTS: $success/$total successful (${rate}%)"
echo "================================================"
echo ""
echo "Historical:"
echo "  v0.14.0: 75% (111/148)"
echo "  Current: ${rate}% ($success/$total)"

improvement=$((rate - 75))
if [ $improvement -gt 0 ]; then
    echo -e "  ${GREEN}↑ +${improvement}%${NC}"
else
    echo -e "  ${RED}↓ ${improvement}%${NC}"
fi
