#!/bin/bash

# MinZ v0.9.0 Release Validation Script
# Compiles all examples and gathers statistics

echo "=== MinZ v0.9.0 Release Validation ==="
echo "Date: $(date)"
echo ""

# Create output directory
mkdir -p release_validation
cd release_validation

# Initialize counters
TOTAL=0
SUCCESS=0
FAILED=0

# Initialize statistics file
echo "# MinZ v0.9.0 Compilation Statistics" > compilation_stats.md
echo "" >> compilation_stats.md
echo "| Example | Status | Functions | Instructions | Code Size | String Opts | SMC |" >> compilation_stats.md
echo "|---------|--------|-----------|--------------|-----------|-------------|-----|" >> compilation_stats.md

# Function to compile and analyze
compile_example() {
    local src=$1
    local name=$(basename "$src" .minz)
    
    echo "Compiling $name..."
    TOTAL=$((TOTAL + 1))
    
    # Compile with full output
    if ../minzc "$src" -o "${name}.a80" -O --enable-smc > "${name}.log" 2>&1; then
        SUCCESS=$((SUCCESS + 1))
        STATUS="✅"
        
        # Extract statistics from log
        FUNCTIONS=$(grep -c "Function .* IsRecursive=" "${name}.log" || echo "0")
        
        # Count instructions in generated assembly
        INSTRUCTIONS=$(grep -E "^\s*(LD|ADD|SUB|CALL|RET|RST|JP|JR|CP|INC|DEC|PUSH|POP)" "${name}.a80" | wc -l | tr -d ' ')
        
        # Get file size
        SIZE=$(wc -c < "${name}.a80" | tr -d ' ')
        
        # Count string optimizations
        DIRECT_STRINGS=$(grep -c "Direct print" "${name}.a80" || echo "0")
        LOOP_STRINGS=$(grep -c "CALL print_string" "${name}.a80" || echo "0")
        STRING_OPTS="${DIRECT_STRINGS}D/${LOOP_STRINGS}L"
        
        # Check for SMC usage
        SMC_COUNT=$(grep -c "SMC" "${name}.a80" || echo "0")
        if [ "$SMC_COUNT" -gt 0 ]; then
            SMC="Yes($SMC_COUNT)"
        else
            SMC="No"
        fi
        
    else
        FAILED=$((FAILED + 1))
        STATUS="❌"
        FUNCTIONS="-"
        INSTRUCTIONS="-"
        SIZE="-"
        STRING_OPTS="-"
        SMC="-"
    fi
    
    # Add to statistics
    echo "| $name | $STATUS | $FUNCTIONS | $INSTRUCTIONS | $SIZE | $STRING_OPTS | $SMC |" >> compilation_stats.md
}

# Compile all examples
echo "=== Compiling Examples ==="
for example in ../../examples/*.minz; do
    if [ -f "$example" ]; then
        compile_example "$example"
    fi
done

# Summary
echo "" >> compilation_stats.md
echo "## Summary" >> compilation_stats.md
echo "- Total Examples: $TOTAL" >> compilation_stats.md
echo "- Successful: $SUCCESS" >> compilation_stats.md
echo "- Failed: $FAILED" >> compilation_stats.md
echo "- Success Rate: $(( SUCCESS * 100 / TOTAL ))%" >> compilation_stats.md

echo ""
echo "=== Compilation Summary ==="
echo "Total: $TOTAL"
echo "Success: $SUCCESS"
echo "Failed: $FAILED"
echo "Success Rate: $(( SUCCESS * 100 / TOTAL ))%"

# Show any failures
if [ $FAILED -gt 0 ]; then
    echo ""
    echo "=== Failed Compilations ==="
    grep -l "Error:" *.log 2>/dev/null | while read log; do
        echo "- $(basename $log .log):"
        grep "Error:" "$log" | head -3
    done
fi

echo ""
echo "Statistics saved to release_validation/compilation_stats.md"