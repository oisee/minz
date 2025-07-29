#!/bin/bash

# Test all MinZ examples
echo "Testing all MinZ examples..."
echo "=========================="

success=0
failed=0
total=0

for file in ../examples/*.minz; do
    if [ -f "$file" ]; then
        total=$((total + 1))
        filename=$(basename "$file")
        printf "%-40s" "$filename"
        
        if ./minzc "$file" -o /tmp/test.a80 >/dev/null 2>&1; then
            echo "✓ OK"
            success=$((success + 1))
        else
            echo "✗ FAILED"
            failed=$((failed + 1))
        fi
    fi
done

echo "=========================="
echo "Total: $total"
echo "Success: $success ($((success * 100 / total))%)"
echo "Failed: $failed"