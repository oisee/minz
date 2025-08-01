#!/bin/bash

# MinZ Examples Comprehensive Compilation Test
# Tests all example files for 100% compilation success

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MINZ_ROOT="$(dirname "$SCRIPT_DIR")"
MINZC="$MINZ_ROOT/minzc/minzc"
EXAMPLES_DIR="$MINZ_ROOT/examples"
TEST_OUTPUT_DIR="$MINZ_ROOT/test_results"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Counters
TOTAL_FILES=0
SUCCESS_COUNT=0
FAILURE_COUNT=0
OPTIMIZATION_SUCCESS=0
OPTIMIZATION_FAILURE=0

# Arrays to store results
declare -a FAILED_FILES
declare -a SUCCESS_FILES
declare -a OPTIMIZATION_FAILED_FILES

echo -e "${BLUE}=== MinZ Examples Compilation Test ===${NC}"
echo "Testing all .minz files in examples directory..."
echo

# Create test output directory
mkdir -p "$TEST_OUTPUT_DIR"
rm -f "$TEST_OUTPUT_DIR"/*.log

# Ensure compiler is built
if [ ! -f "$MINZC" ]; then
    echo -e "${YELLOW}Building MinZ compiler...${NC}"
    cd "$MINZ_ROOT/minzc"
    make build
    cd "$MINZ_ROOT"
fi

# Function to test compilation
test_compilation() {
    local file="$1"
    local base_name=$(basename "$file" .minz)
    local rel_path=$(python3 -c "import os; print(os.path.relpath('$file', '$EXAMPLES_DIR'))")
    
    echo -n "Testing $rel_path... "
    
    # Test normal compilation
    if "$MINZC" "$file" -o "$TEST_OUTPUT_DIR/${base_name}.a80" >/dev/null 2>"$TEST_OUTPUT_DIR/${base_name}_error.log"; then
        echo -e "${GREEN}✓ PASS${NC}"
        SUCCESS_FILES+=("$rel_path")
        ((SUCCESS_COUNT++))
        
        # Test with optimizations
        echo -n "  → Testing with optimizations... "
        if "$MINZC" "$file" -O --enable-smc -o "$TEST_OUTPUT_DIR/${base_name}_opt.a80" >/dev/null 2>"$TEST_OUTPUT_DIR/${base_name}_opt_error.log"; then
            echo -e "${GREEN}✓ OPT PASS${NC}"
            ((OPTIMIZATION_SUCCESS++))
        else
            echo -e "${RED}✗ OPT FAIL${NC}"
            OPTIMIZATION_FAILED_FILES+=("$rel_path")
            ((OPTIMIZATION_FAILURE++))
        fi
    else
        echo -e "${RED}✗ FAIL${NC}"
        FAILED_FILES+=("$rel_path")
        ((FAILURE_COUNT++))
        
        # Show error preview
        echo -e "    ${RED}Error:${NC} $(head -1 "$TEST_OUTPUT_DIR/${base_name}_error.log")"
    fi
    
    ((TOTAL_FILES++))
}

# Find and test all .minz files
echo -e "${BLUE}Searching for .minz files...${NC}"
while IFS= read -r -d '' file; do
    # Skip files in subdirectories that are outputs or compiled results
    if [[ "$file" == */compiled/* ]] || [[ "$file" == */output/* ]]; then
        continue
    fi
    
    test_compilation "$file"
done < <(find "$EXAMPLES_DIR" -name "*.minz" -print0)

echo
echo -e "${BLUE}=== COMPILATION RESULTS ===${NC}"
echo "Total files tested: $TOTAL_FILES"
echo -e "Successful compilations: ${GREEN}$SUCCESS_COUNT${NC}"
echo -e "Failed compilations: ${RED}$FAILURE_COUNT${NC}"
echo -e "Success rate: ${GREEN}$(( SUCCESS_COUNT * 100 / TOTAL_FILES ))%${NC}"

echo
echo -e "${BLUE}=== OPTIMIZATION RESULTS ===${NC}"
echo -e "Successful optimized compilations: ${GREEN}$OPTIMIZATION_SUCCESS${NC}"
echo -e "Failed optimized compilations: ${RED}$OPTIMIZATION_FAILURE${NC}"
if [ $SUCCESS_COUNT -gt 0 ]; then
    echo -e "Optimization success rate: ${GREEN}$(( OPTIMIZATION_SUCCESS * 100 / SUCCESS_COUNT ))%${NC}"
fi

# Report failures
if [ ${#FAILED_FILES[@]} -gt 0 ]; then
    echo
    echo -e "${RED}=== FAILED COMPILATIONS ===${NC}"
    for file in "${FAILED_FILES[@]}"; do
        echo -e "  ${RED}✗${NC} $file"
        local base_name=$(basename "$file" .minz)
        echo "    Error: $(head -1 "$TEST_OUTPUT_DIR/${base_name}_error.log")"
    done
fi

if [ ${#OPTIMIZATION_FAILED_FILES[@]} -gt 0 ]; then
    echo
    echo -e "${YELLOW}=== OPTIMIZATION FAILURES ===${NC}"
    for file in "${OPTIMIZATION_FAILED_FILES[@]}"; do
        echo -e "  ${YELLOW}!${NC} $file"
        local base_name=$(basename "$file" .minz)
        if [ -f "$TEST_OUTPUT_DIR/${base_name}_opt_error.log" ]; then
            echo "    Error: $(head -1 "$TEST_OUTPUT_DIR/${base_name}_opt_error.log")"
        fi
    done
fi

# Generate detailed report
echo
echo -e "${BLUE}=== GENERATING DETAILED REPORT ===${NC}"
REPORT_FILE="$TEST_OUTPUT_DIR/compilation_report.md"

cat > "$REPORT_FILE" << EOF
# MinZ Examples Compilation Report

**Date:** $(date)
**Total Files Tested:** $TOTAL_FILES
**Compilation Success Rate:** $(( SUCCESS_COUNT * 100 / TOTAL_FILES ))%
**Optimization Success Rate:** $(( OPTIMIZATION_SUCCESS * 100 / SUCCESS_COUNT ))%

## Summary

- ✅ **Successful compilations:** $SUCCESS_COUNT
- ❌ **Failed compilations:** $FAILURE_COUNT
- 🚀 **Successful optimized compilations:** $OPTIMIZATION_SUCCESS
- ⚠️ **Failed optimized compilations:** $OPTIMIZATION_FAILURE

## Successful Files

EOF

for file in "${SUCCESS_FILES[@]}"; do
    echo "- ✅ \`$file\`" >> "$REPORT_FILE"
done

if [ ${#FAILED_FILES[@]} -gt 0 ]; then
    echo "" >> "$REPORT_FILE"
    echo "## Failed Compilations" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    for file in "${FAILED_FILES[@]}"; do
        local base_name=$(basename "$file" .minz)
        echo "- ❌ \`$file\`" >> "$REPORT_FILE"
        echo "  - Error: \`$(head -1 "$TEST_OUTPUT_DIR/${base_name}_error.log")\`" >> "$REPORT_FILE"
    done
fi

if [ ${#OPTIMIZATION_FAILED_FILES[@]} -gt 0 ]; then
    echo "" >> "$REPORT_FILE"
    echo "## Optimization Failures" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    for file in "${OPTIMIZATION_FAILED_FILES[@]}"; do
        local base_name=$(basename "$file" .minz)
        echo "- ⚠️ \`$file\`" >> "$REPORT_FILE"
        if [ -f "$TEST_OUTPUT_DIR/${base_name}_opt_error.log" ]; then
            echo "  - Error: \`$(head -1 "$TEST_OUTPUT_DIR/${base_name}_opt_error.log")\`" >> "$REPORT_FILE"
        fi
    done
fi

echo "Detailed report saved to: $REPORT_FILE"

# Exit with appropriate code
if [ $FAILURE_COUNT -eq 0 ]; then
    echo -e "\n${GREEN}🎉 ALL EXAMPLES COMPILE SUCCESSFULLY! 🎉${NC}"
    exit 0
else
    echo -e "\n${RED}❌ Some examples failed to compile${NC}"
    exit 1
fi