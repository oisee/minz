#!/bin/bash
cd /Users/alice/dev/minz-ts
success=0
failed=0
echo "Testing final compilation success rate with all fixes..."

# Update the compiler in examples directory
cp mz minzc/mz

for file in examples/*.minz; do
    if [ -f "$file" ]; then
        if ./mz "$file" -o /tmp/test.a80 2>/dev/null; then
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
rate=$((success * 100 / total))
echo ""
echo "=== Final Results After Semantic Fixes ==="
echo "Success: $success/$total ($rate%)"
echo ""
echo "Progress:"
echo "- Initial:     64% (57/88)"
echo "- Quick fixes: 66% (59/89)" 
echo "- Semantic:    $rate% ($success/$total)"
improvement=$((rate - 66))
echo "- New improvement: +$improvement%"
