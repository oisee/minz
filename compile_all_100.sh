#!/bin/bash
# Script to compile all 100 MinZ examples and gather statistics

set -e

echo "=== Compiling All 100 MinZ Examples ==="
echo "Started at: $(date)"
echo

cd minzc

# Clean previous outputs
rm -rf output
mkdir -p output

# Initialize counters
TOTAL=0
SUCCESS=0
FAILED=0

# Create results file
RESULTS_FILE="../compilation_results_100.txt"
echo "MinZ Compilation Results - $(date)" > "$RESULTS_FILE"
echo "=====================================" >> "$RESULTS_FILE"
echo "" >> "$RESULTS_FILE"

# Function to compile a single file
compile_file() {
    local file=$1
    local basename=$(basename "$file" .minz)
    local output="output/${basename}.a80"
    
    TOTAL=$((TOTAL + 1))
    
    printf "%-40s ... " "$basename"
    
    if ./minzc "$file" -o "$output" 2>&1 > /dev/null; then
        echo "✅ SUCCESS"
        echo "✅ $basename" >> "$RESULTS_FILE"
        SUCCESS=$((SUCCESS + 1))
    else
        echo "❌ FAILED"
        echo "❌ $basename" >> "$RESULTS_FILE"
        FAILED=$((FAILED + 1))
    fi
}

# Compile all examples
echo "Compiling examples..." | tee -a "$RESULTS_FILE"
echo "" >> "$RESULTS_FILE"

for file in ../examples/*.minz; do
    if [ -f "$file" ]; then
        compile_file "$file"
    fi
done

# Calculate statistics
PERCENTAGE=$(echo "scale=2; $SUCCESS * 100 / $TOTAL" | bc)

echo
echo "=== COMPILATION STATISTICS ===" | tee -a "$RESULTS_FILE"
echo "Total examples:     $TOTAL" | tee -a "$RESULTS_FILE"
echo "Successful:         $SUCCESS" | tee -a "$RESULTS_FILE"
echo "Failed:             $FAILED" | tee -a "$RESULTS_FILE"
echo "Success rate:       ${PERCENTAGE}%" | tee -a "$RESULTS_FILE"
echo "" | tee -a "$RESULTS_FILE"

# List failed examples
if [ $FAILED -gt 0 ]; then
    echo "=== FAILED EXAMPLES ===" | tee -a "$RESULTS_FILE"
    grep "❌" "$RESULTS_FILE" | while read line; do
        echo "$line" | tee -a "$RESULTS_FILE"
    done
fi

echo
echo "Completed at: $(date)"
echo "Results saved to: $RESULTS_FILE"

# Create category statistics
echo
echo "=== CATEGORY BREAKDOWN ===" | tee -a "$RESULTS_FILE"

echo "Core Language Features:" | tee -a "$RESULTS_FILE"
grep -E "^(✅|❌) (arithmetic|arrays|basic|control|enums|fibonacci|functions|global|structs|types)" "$RESULTS_FILE" | wc -l | xargs printf "  Total: %d\n" | tee -a "$RESULTS_FILE"
grep -E "^✅ (arithmetic|arrays|basic|control|enums|fibonacci|functions|global|structs|types)" "$RESULTS_FILE" | wc -l | xargs printf "  Success: %d\n" | tee -a "$RESULTS_FILE"

echo "@abi & Assembly Integration:" | tee -a "$RESULTS_FILE"
grep -E "^(✅|❌) (abi_|asm_|inline_assembly)" "$RESULTS_FILE" | wc -l | xargs printf "  Total: %d\n" | tee -a "$RESULTS_FILE"
grep -E "^✅ (abi_|asm_|inline_assembly)" "$RESULTS_FILE" | wc -l | xargs printf "  Success: %d\n" | tee -a "$RESULTS_FILE"

echo "Optimizations & Advanced:" | tee -a "$RESULTS_FILE"
grep -E "^(✅|❌) (smc_|true_smc|tail_|shadow|register)" "$RESULTS_FILE" | wc -l | xargs printf "  Total: %d\n" | tee -a "$RESULTS_FILE"
grep -E "^✅ (smc_|true_smc|tail_|shadow|register)" "$RESULTS_FILE" | wc -l | xargs printf "  Success: %d\n" | tee -a "$RESULTS_FILE"

echo "Applications & Demos:" | tee -a "$RESULTS_FILE"
grep -E "^(✅|❌) (editor|game|mnist)" "$RESULTS_FILE" | wc -l | xargs printf "  Total: %d\n" | tee -a "$RESULTS_FILE"
grep -E "^✅ (editor|game|mnist)" "$RESULTS_FILE" | wc -l | xargs printf "  Success: %d\n" | tee -a "$RESULTS_FILE"

exit 0