#!/bin/bash
cd /Users/alice/dev/minz-ts

# Test our simplified metaprogramming example
echo "Testing simplified metaprogramming..."
if minzc/mz examples/metaprogramming_simple.minz -o /tmp/test.a80 2>&1; then
    echo "✓ metaprogramming_simple.minz compiles!"
else
    echo "✗ metaprogramming_simple.minz failed"
fi

# Count successes before and after
echo ""
echo "Checking overall success rate..."
success=0
failed=0
for file in examples/*.minz; do
    if [ -f "$file" ]; then
        if minzc/mz "$file" -o /tmp/test.a80 2>/dev/null; then
            success=$((success + 1))
        else
            failed=$((failed + 1))
        fi
    fi
done
total=$((success + failed))
rate=$((success * 100 / total))
echo "Current: $success/$total ($rate%)"
