#!/bin/bash

# Comprehensive test of ALL examples (avoiding timeout issues)

COMPILER="/Users/alice/dev/minz-ts/minzc/minzc"
EXAMPLES_DIR="/Users/alice/dev/minz-ts/examples"
OUTPUT_DIR="/Users/alice/dev/minz-ts/minzc/comprehensive_test_results"
mkdir -p "$OUTPUT_DIR"

TOTAL_FILES=0
SUCCESS_COUNT=0
FAILURE_COUNT=0
OPTIMIZATION_TESTS=0
SMC_TESTS=0

declare -a SUCCESS_FILES=()
declare -a FAILURE_FILES=()
declare -a ERROR_PATTERNS=()

echo "Running comprehensive test of all MinZ examples..."
echo "================================================="

# Find all .minz files (excluding archived)
find "$EXAMPLES_DIR" -name "*.minz" -type f | grep -v -E "(archived|output)" | sort | while read -r file; do
    base_name=$(basename "$file" .minz)
    echo ""
    echo "Testing: $(basename "$file")"
    echo "---------------------------------------"
    
    # Test normal compilation
    output_normal="$OUTPUT_DIR/${base_name}_normal.a80" 
    error_log="$OUTPUT_DIR/${base_name}_errors.log"
    
    echo -n "  Normal: "
    if "$COMPILER" "$file" -o "$output_normal" 2>"$error_log"; then
        echo "✅ SUCCESS"
        normal_success=true
    else
        echo "❌ FAILED"
        normal_success=false
        error=$(head -1 "$error_log" 2>/dev/null | cut -c1-60)
        echo "    $error..."
    fi
    
    # Test optimized compilation
    output_optimized="$OUTPUT_DIR/${base_name}_optimized.a80"
    echo -n "  Optimized: "
    if "$COMPILER" "$file" -O --enable-smc -o "$output_optimized" 2>>"$error_log"; then
        echo "✅ SUCCESS"
        optimized_success=true
        
        # Check for SMC
        if grep -q "SMC\|patch" "$output_optimized" 2>/dev/null; then
            echo "    📊 SMC detected"
        fi
    else
        echo "❌ FAILED"
        optimized_success=false
    fi
    
    # Track totals
    echo "$((TOTAL_FILES + 1))" > "$OUTPUT_DIR/total_count.tmp"
    if [ "$normal_success" = true ] || [ "$optimized_success" = true ]; then
        echo "$base_name" >> "$OUTPUT_DIR/success_list.tmp"
    else  
        echo "$base_name" >> "$OUTPUT_DIR/failure_list.tmp"
    fi
done

# Calculate final statistics
TOTAL_FILES=$(cat "$OUTPUT_DIR/total_count.tmp" 2>/dev/null || echo "0")
SUCCESS_COUNT=$(wc -l < "$OUTPUT_DIR/success_list.tmp" 2>/dev/null || echo "0")
FAILURE_COUNT=$(wc -l < "$OUTPUT_DIR/failure_list.tmp" 2>/dev/null || echo "0")

echo ""
echo "================================================="
echo "FINAL RESULTS"
echo "================================================="
echo "Total files tested: $TOTAL_FILES"
echo "Successful compilations: $SUCCESS_COUNT"
echo "Failed compilations: $FAILURE_COUNT"
if [ "$TOTAL_FILES" -gt 0 ]; then
    echo "Success rate: $(( SUCCESS_COUNT * 100 / TOTAL_FILES ))%"
fi

echo ""
echo "Results saved to: $OUTPUT_DIR"