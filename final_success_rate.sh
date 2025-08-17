#!/bin/bash
cd /Users/alice/dev/minz-ts
success=0
failed=0
echo "Testing final compilation success rate..."
for file in examples/*.minz; do
    if [ -f "$file" ]; then
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
rate=$((success * 100 / total))
echo ""
echo "=== Final Results ==="
echo "Success: $success/$total ($rate%)"
echo ""
echo "Progress:"
echo "- Initial: 64% (57/88)"
echo "- Current: $rate% ($success/$total)"
improvement=$((rate - 64))
echo "- Improvement: +$improvement%"
