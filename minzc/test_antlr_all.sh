#!/bin/bash
# Test ANTLR parser against all MinZ examples

echo "Testing ANTLR parser against all examples..."
echo "============================================"

success=0
failed=0
failed_files=""

# Test all examples
for file in ../examples/*.minz; do
    if [ -f "$file" ]; then
        filename=$(basename "$file")
        
        # Skip known problematic files
        if [[ "$filename" == *"@"* ]] || [[ "$filename" == *"meta"* ]]; then
            echo -n "S"  # Skip
            continue
        fi
        
        # Try to compile with ANTLR
        if MINZ_USE_ANTLR=1 ./mz "$file" -o /tmp/test.a80 2>/dev/null; then
            echo -n "✓"
            success=$((success + 1))
        else
            echo -n "✗"
            failed=$((failed + 1))
            failed_files="$failed_files $filename"
        fi
    fi
done

echo ""
echo ""
echo "Results:"
echo "  Success: $success"
echo "  Failed:  $failed"
total=$((success + failed))
if [ $total -gt 0 ]; then
    rate=$((success * 100 / total))
    echo "  Success rate: $rate%"
fi

if [ -n "$failed_files" ]; then
    echo ""
    echo "Failed files:"
    for f in $failed_files; do
        echo "  - $f"
    done
fi

rm -f /tmp/test.a80 /tmp/test.mir