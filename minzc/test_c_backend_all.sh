#!/bin/bash

# Script to test C backend compilation of all MinZ examples
# This will compile all .minz files with the C backend and collect errors

set -e

OUTPUT_DIR="c_backend_test_results"
MINZC="./mz"
ERRORS_LOG="c_backend_errors.log"
SUCCESS_LOG="c_backend_success.log"
SUMMARY_LOG="c_backend_summary.log"

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Clear previous logs
> "$ERRORS_LOG"
> "$SUCCESS_LOG" 
> "$SUMMARY_LOG"

echo "=== MinZ C Backend Testing Suite ===" | tee "$SUMMARY_LOG"
echo "Started at: $(date)" | tee -a "$SUMMARY_LOG"
echo "" | tee -a "$SUMMARY_LOG"

total_files=0
successful_compilations=0
failed_compilations=0

# Function to test a single MinZ file
test_minz_file() {
    local minz_file="$1"
    local basename=$(basename "$minz_file" .minz)
    local c_output="$OUTPUT_DIR/${basename}.c"
    
    echo "Testing: $minz_file"
    total_files=$((total_files + 1))
    
    # Try to compile with C backend
    if "$MINZC" "$minz_file" -b c -o "$c_output" 2>&1; then
        echo "✅ SUCCESS: $minz_file -> $c_output" | tee -a "$SUCCESS_LOG"
        successful_compilations=$((successful_compilations + 1))
        
        # Try to compile the generated C with clang to verify validity
        if clang -std=c99 -o "$OUTPUT_DIR/${basename}" "$c_output" 2>&1; then
            echo "  ✅ C compilation with clang: SUCCESS" | tee -a "$SUCCESS_LOG"
        else
            echo "  ❌ C compilation with clang: FAILED" | tee -a "$ERRORS_LOG"
            echo "--- C compilation error for $minz_file ---" >> "$ERRORS_LOG"
            clang -std=c99 -o "$OUTPUT_DIR/${basename}" "$c_output" 2>&1 | head -20 >> "$ERRORS_LOG"
            echo "--- End C compilation error ---" >> "$ERRORS_LOG"
            echo "" >> "$ERRORS_LOG"
        fi
    else
        echo "❌ FAILED: $minz_file" | tee -a "$ERRORS_LOG"
        failed_compilations=$((failed_compilations + 1))
        echo "--- MinZ compilation error for $minz_file ---" >> "$ERRORS_LOG"
        "$MINZC" "$minz_file" -b c -o "$c_output" 2>&1 | head -20 >> "$ERRORS_LOG"
        echo "--- End MinZ compilation error ---" >> "$ERRORS_LOG"
        echo "" >> "$ERRORS_LOG"
    fi
}

# Test files in the examples directory
echo "Testing files in examples/ directory:"
if [ -d "examples" ]; then
    for minz_file in examples/*.minz; do
        if [ -f "$minz_file" ]; then
            test_minz_file "$minz_file"
        fi
    done
fi

# Test .minz files in current directory
echo ""
echo "Testing .minz files in current directory:"
for minz_file in *.minz; do
    if [ -f "$minz_file" ]; then
        test_minz_file "$minz_file"
    fi
done

# Generate summary
echo "" | tee -a "$SUMMARY_LOG"
echo "=== SUMMARY ===" | tee -a "$SUMMARY_LOG"
echo "Total files tested: $total_files" | tee -a "$SUMMARY_LOG"
echo "Successful MinZ compilations: $successful_compilations" | tee -a "$SUMMARY_LOG"
echo "Failed MinZ compilations: $failed_compilations" | tee -a "$SUMMARY_LOG"

if [ $total_files -gt 0 ]; then
    success_rate=$(echo "scale=1; $successful_compilations * 100 / $total_files" | bc -l)
    echo "Success rate: ${success_rate}%" | tee -a "$SUMMARY_LOG"
fi

echo "" | tee -a "$SUMMARY_LOG"
echo "Generated C files are in: $OUTPUT_DIR" | tee -a "$SUMMARY_LOG"
echo "Error details in: $ERRORS_LOG" | tee -a "$SUMMARY_LOG"
echo "Success details in: $SUCCESS_LOG" | tee -a "$SUMMARY_LOG"
echo "Completed at: $(date)" | tee -a "$SUMMARY_LOG"

# Show most common error patterns
echo "" | tee -a "$SUMMARY_LOG"
echo "=== COMMON ERROR PATTERNS ===" | tee -a "$SUMMARY_LOG"
if [ -s "$ERRORS_LOG" ]; then
    echo "Top error patterns:" | tee -a "$SUMMARY_LOG"
    grep -E "(Error:|error:|failed|FAILED)" "$ERRORS_LOG" | sort | uniq -c | sort -nr | head -10 | tee -a "$SUMMARY_LOG"
else
    echo "No errors found!" | tee -a "$SUMMARY_LOG"
fi