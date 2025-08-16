#!/bin/bash
# Quick success rate test for current MZA state

cd /Users/alice/dev/minz-ts/minzc

echo "=== Phase 1 Success Rate Test ==="
echo "Testing sample of corpus..."

success=0
failed=0
total=0

# Test a representative sample (first 50 files)
for file in ../sanitized_corpus/*.a80; do
    if [ $total -ge 50 ]; then break; fi
    
    if [ -f "$file" ]; then
        total=$((total + 1))
        if ./mza "$file" -o /tmp/test.bin 2>/dev/null; then
            success=$((success + 1))
            echo -n "✓"
        else
            failed=$((failed + 1))
            echo -n "✗"
        fi
    fi
done

echo ""
echo ""
echo "=== Phase 1 Results ==="
echo "Success: $success/$total files"
if [ $total -gt 0 ]; then
    rate=$((success * 100 / total))
    echo "Success Rate: $rate%"
else
    echo "Success Rate: 0%"
fi
echo ""
echo "Ready for Phase 2: Table-driven encoder"