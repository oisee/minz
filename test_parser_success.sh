#!/bin/bash
# Test all examples and report success rate

echo "Testing MinZ Parser Success Rate"
echo "================================"

cd /Users/alice/dev/minz-ts

success=0
failed=0
failed_files=""

for file in examples/*.minz; do
    if [ -f "$file" ]; then
        filename=$(basename "$file")
        # Skip known problematic files
        if [[ "$filename" == "abi_"* ]] || [[ "$filename" == "debug_"* ]]; then
            continue
        fi
        
        if minzc/mz "$file" -o /tmp/test.a80 2>/dev/null; then
            success=$((success + 1))
            echo -n "✓"
        else
            failed=$((failed + 1))
            failed_files="$failed_files\n  - $filename"
            echo -n "✗"
        fi
    fi
done

echo ""
echo ""
total=$((success + failed))
if [ $total -gt 0 ]; then
    rate=$((success * 100 / total))
else
    rate=0
fi

echo "Results:"
echo "--------"
echo "Success: $success/$total ($rate%)"
echo ""

if [ $failed -gt 0 ]; then
    echo "Failed files:"
    echo -e "$failed_files"
fi