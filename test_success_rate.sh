#!/bin/bash
cd /Users/alice/dev/minz-ts
success=0
failed=0
echo "Testing compilation success rate..."
for file in examples/*.minz; do
    if [ -f "$file" ]; then
        filename=$(basename "$file")
        # Skip archive files
        if [[ "$file" == *"archive"* ]]; then
            continue
        fi
        if minzc/mz "$file" -o /tmp/test.a80 2>/dev/null; then
            success=$((success + 1))
            echo -n "✓"
        else
            failed=$((failed + 1))
            echo -n "✗"
        fi
    fi
done
echo ""
total=$((success + failed))
if [ $total -gt 0 ]; then
    rate=$((success * 100 / total))
else
    rate=0
fi
echo "=== Updated Results ==="
echo "Success: $success/$total ($rate%)"
echo ""
echo "Previous: 64% (57/88)"
echo "Current:  $rate% ($success/$total)"
