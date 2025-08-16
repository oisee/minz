#!/bin/bash

echo "Testing MZA with Table-Driven Encoder"
echo "======================================"

# Test directory
TEST_DIR="corpus_test_table_driven"
mkdir -p "$TEST_DIR"

# Statistics
total=0
success=0
failed=0
ld_failures=0

# Test each file in sanitized_corpus
for file in sanitized_corpus/*.a80 sanitized_corpus/*/*.a80; do
    if [ -f "$file" ]; then
        filename=$(basename "$file")
        total=$((total + 1))
        
        # Try to assemble
        if ./minzc/mza -o "$TEST_DIR/${filename%.a80}.bin" "$file" 2>/dev/null; then
            success=$((success + 1))
            echo -n "✓"
        else
            failed=$((failed + 1))
            echo -n "✗"
            # Capture specific error for LD instructions
            error_msg=$(./minzc/mza "$file" 2>&1)
            if echo "$error_msg" | grep -q "LD"; then
                ld_failures=$((ld_failures + 1))
                echo "$filename: $error_msg" >> "$TEST_DIR/ld_failures.txt"
            fi
        fi
        
        # Progress indicator
        if [ $((total % 50)) -eq 0 ]; then
            echo " [$total]"
        fi
    fi
done

echo ""
echo ""
echo "Results:"
echo "--------"
echo "Total files: $total"
echo "Successful: $success"
echo "Failed: $failed"
if [ $total -gt 0 ]; then
    success_rate=$((success * 100 / total))
    echo "Success rate: ${success_rate}%"
else
    echo "Success rate: N/A (no files)"
fi

echo ""
echo "LD-related failures: $ld_failures"

# Compare with previous result
echo ""
echo "Previous result: 12% success rate"
if [ $total -gt 0 ]; then
    improvement=$((success_rate - 12))
    if [ $improvement -gt 0 ]; then
        echo "✅ Improvement: +${improvement}%"
    else
        echo "⚠️ No improvement yet: ${improvement}%"
    fi
fi

# Sample of LD failures for debugging
if [ -f "$TEST_DIR/ld_failures.txt" ]; then
    echo ""
    echo "Sample LD failures (first 5):"
    head -5 "$TEST_DIR/ld_failures.txt"
fi

# Clean up
rm -rf "$TEST_DIR"