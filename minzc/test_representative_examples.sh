#!/bin/bash

# Test representative examples to understand success/failure patterns

COMPILER="/Users/alice/dev/minz-ts/minzc/minzc"
EXAMPLES_DIR="/Users/alice/dev/minz-ts/examples"
OUTPUT_DIR="/Users/alice/dev/minz-ts/minzc/representative_test_results"
mkdir -p "$OUTPUT_DIR"

# Representative test cases
TEST_CASES=(
    "simple_add.minz"
    "fibonacci.minz" 
    "arithmetic_demo.minz"
    "basic_functions.minz"
    "arrays.minz"
    "control_flow.minz"
    "enums.minz"
    "simple_abi_demo.minz"
    "lua_constants.minz"
    "tail_recursive.minz"
    "lambda_basic_test.minz"
    "mnist_simple.minz"
)

SUCCESS_COUNT=0
FAILURE_COUNT=0
declare -a SUCCESS_FILES=()
declare -a FAILURE_FILES=()
declare -a ERROR_PATTERNS=()

echo "Testing representative MinZ examples..."
echo "======================================="

for test_case in "${TEST_CASES[@]}"; do
    file="$EXAMPLES_DIR/$test_case"
    if [ ! -f "$file" ]; then
        echo "⚠️  File not found: $file"
        continue
    fi
    
    base_name=$(basename "$file" .minz)
    output_normal="$OUTPUT_DIR/${base_name}_normal.a80"
    output_optimized="$OUTPUT_DIR/${base_name}_optimized.a80"
    error_log="$OUTPUT_DIR/${base_name}_errors.log"
    
    echo ""
    echo "Testing: $test_case"
    echo "----------------------------------------"
    
    # Test normal compilation
    echo -n "  Normal compilation:  "
    if "$COMPILER" "$file" -o "$output_normal" 2>"$error_log"; then
        echo "✅ SUCCESS"
        normal_success=true
    else
        echo "❌ FAILED"
        normal_success=false
        error=$(head -1 "$error_log" 2>/dev/null || echo 'Unknown error')
        echo "    Error: $error"
        ERROR_PATTERNS+=("$error")
    fi
    
    # Test optimized compilation  
    echo -n "  Optimized compilation: "
    if "$COMPILER" "$file" -O --enable-smc -o "$output_optimized" 2>>"$error_log"; then
        echo "✅ SUCCESS"
        optimized_success=true
        
        # Check for SMC usage
        if grep -q "SMC\|patch\|self.modifying" "$output_optimized" 2>/dev/null; then
            echo "    📊 SMC optimization detected"
        fi
    else
        echo "❌ FAILED"
        optimized_success=false
        error=$(tail -1 "$error_log" 2>/dev/null || echo 'Unknown optimization error')
        echo "    Error: $error"
        ERROR_PATTERNS+=("$error")
    fi
    
    # Performance comparison
    if [ "$normal_success" = true ] && [ "$optimized_success" = true ]; then
        normal_size=$(wc -l < "$output_normal" 2>/dev/null || echo 0)
        optimized_size=$(wc -l < "$output_optimized" 2>/dev/null || echo 0)
        if [ "$normal_size" -gt 0 ] && [ "$optimized_size" -gt 0 ]; then
            if [ "$optimized_size" -lt "$normal_size" ]; then
                improvement=$(( (normal_size - optimized_size) * 100 / normal_size ))
                echo "    📈 Code size improvement: $improvement% ($normal_size → $optimized_size lines)"
            elif [ "$optimized_size" -eq "$normal_size" ]; then
                echo "    📊 Code size unchanged: $normal_size lines"
            else
                increase=$(( (optimized_size - normal_size) * 100 / normal_size ))
                echo "    📉 Code size increase: $increase% ($normal_size → $optimized_size lines)"
            fi
        fi
    fi
    
    # Track overall success
    if [ "$normal_success" = true ] || [ "$optimized_success" = true ]; then
        SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
        SUCCESS_FILES+=("$test_case")
    else
        FAILURE_COUNT=$((FAILURE_COUNT + 1))
        FAILURE_FILES+=("$test_case")
    fi
done

echo ""
echo "======================================="
echo "SUMMARY RESULTS"
echo "======================================="
echo "Total tests: $((SUCCESS_COUNT + FAILURE_COUNT))"
echo "Successful: $SUCCESS_COUNT"
echo "Failed: $FAILURE_COUNT"
echo "Success rate: $(( SUCCESS_COUNT * 100 / (SUCCESS_COUNT + FAILURE_COUNT) ))%"

if [ ${#SUCCESS_FILES[@]} -gt 0 ]; then
    echo ""
    echo "✅ SUCCESSFUL COMPILATIONS:"
    for file in "${SUCCESS_FILES[@]}"; do
        echo "   • $file"
    done
fi

if [ ${#FAILURE_FILES[@]} -gt 0 ]; then
    echo ""
    echo "❌ FAILED COMPILATIONS:"
    for file in "${FAILURE_FILES[@]}"; do
        echo "   • $file"
    done
    
    echo ""
    echo "🔍 COMMON ERROR PATTERNS:"
    printf '%s\n' "${ERROR_PATTERNS[@]}" | sort | uniq -c | sort -nr | head -5 | while read count pattern; do
        echo "   • $pattern ($count occurrences)"
    done
fi

echo ""
echo "Results saved to: $OUTPUT_DIR"